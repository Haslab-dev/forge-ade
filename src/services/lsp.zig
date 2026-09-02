// services.lsp — language server client (LSP over stdio with Content-Length
// framing). Port of src/server/lsp.ts (LSPClient). Supports the frontend
// contract: didOpen/didChange/didSave/didClose, completion, hover, definition,
// declaration, typeDefinition, implementation, diagnostics, listServers.

const std = @import("std");
const svc = @import("../services.zig");

pub const ServerInfo = struct {
    languageId: []const u8 = "",
    name: []const u8 = "",
    command: []const u8 = "",
    args: []const []const u8 = &.{},
    status: []const u8 = "stopped",
    workspaceRoot: []const u8 = "",
    openDocumentsCount: usize = 0,
    errorsCount: usize = 0,
    warningsCount: usize = 0,
};

const SpinLock = struct {
    locked: std.atomic.Value(bool) = std.atomic.Value(bool).init(false),
    fn lock(self: *SpinLock) void {
        while (self.locked.swap(true, .acquire)) std.atomic.spinLoopHint();
    }
    fn unlock(self: *SpinLock) void {
        self.locked.store(false, .release);
    }
};

const Timespec = extern struct { tv_sec: isize, tv_nsec: isize };
extern "c" fn nanosleep(req: *const Timespec, rem: ?*Timespec) c_int;

fn threadSleepMs(ms: u64) void {
    const sec: isize = @intCast(@divTrunc(ms, 1000));
    const nsec: isize = @intCast(@mod(ms, 1000) * 1_000_000);
    var ts = Timespec{ .tv_sec = sec, .tv_nsec = nsec };
    _ = nanosleep(&ts, null);
}

const c = struct {
    extern "c" fn fork() c_int;
    extern "c" fn pipe(fildes: *[2]c_int) c_int;
    extern "c" fn dup2(oldfd: c_int, newfd: c_int) c_int;
    extern "c" fn close(fd: c_int) c_int;
    extern "c" fn read(fd: c_int, buf: [*]u8, len: usize) isize;
    extern "c" fn write(fd: c_int, buf: [*]const u8, len: usize) isize;
    extern "c" fn waitpid(pid: c_int, status: ?*c_int, options: c_int) c_int;
    extern "c" fn kill(pid: c_int, sig: c_int) c_int;
    extern "c" fn execvp(file: [*:0]const u8, argv: [*:null]const ?[*:0]const u8) c_int;
    extern "c" fn _exit(code: c_int) noreturn;
    extern "c" fn access(path: [*:0]const u8, mode: c_int) c_int;
    extern "c" fn chdir(path: [*:0]const u8) c_int;
};

const F_OK: c_int = 0;
const SIGTERM: c_int = 15;

const Shared = struct {
    allocator: std.mem.Allocator,
    mutex: SpinLock = .{},
    /// Response values are DEEP COPIES owned by the map (never arena
    /// aliases — the frame's parse arena dies when handleFrame returns).
    responses: ?std.AutoHashMap(u64, std.json.Value) = null,
    errors: ?std.AutoHashMap(u64, []const u8) = null,
    alive: bool = true,
    /// Latest diagnostics per file uri (from publishDiagnostics).
    diagnostics: std.StringHashMap([]const u8) = undefined,
    diagnostics_init: bool = false,

    pub fn init(allocator: std.mem.Allocator) Shared {
        var s = Shared{
            .allocator = allocator,
            .responses = std.AutoHashMap(u64, std.json.Value).init(allocator),
            .errors = std.AutoHashMap(u64, []const u8).init(allocator),
        };
        s.diagnostics = std.StringHashMap([]const u8).init(allocator);
        s.diagnostics_init = true;
        return s;
    }

    pub fn deinit(self: *Shared) void {
        if (self.errors) |*e| {
            var e_it = e.iterator();
            while (e_it.next()) |entry| e.allocator.free(entry.value_ptr.*);
            e.deinit();
        }
        if (self.responses) |*r| {
            var r_it = r.iterator();
            while (r_it.next()) |entry| svc.deepFreeValue(r.allocator, entry.value_ptr.*);
            r.deinit();
        }
        if (self.diagnostics_init) {
            var d_it = self.diagnostics.iterator();
            while (d_it.next()) |e| {
                self.allocator.free(e.key_ptr.*);
                self.allocator.free(e.value_ptr.*);
            }
            self.diagnostics.deinit();
        }
    }
};

pub const Client = struct {
    allocator: std.mem.Allocator,
    language_id: []const u8 = "",
    workspace_root: []const u8 = "",
    command: []const u8 = "",
    args: []const []const u8 = &.{},
    status: []const u8 = "stopped",
    pid: c_int = -1,
    stdin_fd: c_int = -1,
    stdout_fd: c_int = -1,
    alive: bool = true,
    next_id: u64 = 1,
    shared: *Shared,
    initialized: bool = false,
    open_docs: std.ArrayList([]const u8) = undefined,
    open_init: bool = false,
    /// Set by the reader thread on exit; deinit waits for it before freeing
    /// anything the reader touches (prevents use-after-free on restart/stop).
    reader_done: std.atomic.Value(bool) = std.atomic.Value(bool).init(false),

    pub fn deinit(self: *Client) void {
        self.close();
        var waited: u32 = 0;
        while (!self.reader_done.load(.acquire) and waited < 3000) {
            threadSleepMs(2);
            waited += 2;
        }
        self.shared.deinit();
        self.allocator.destroy(self.shared);
        self.allocator.free(self.language_id);
        self.allocator.free(self.workspace_root);
        self.allocator.free(self.command);
        if (self.open_init) {
            for (self.open_docs.items) |p| self.allocator.free(p);
            self.open_docs.deinit(self.allocator);
        }
        self.allocator.destroy(self);
    }

    pub fn close(self: *Client) void {
        self.shared.mutex.lock();
        self.shared.alive = false;
        self.shared.mutex.unlock();
        if (self.alive) {
            self.alive = false;
            if (self.pid > 0) _ = c.kill(self.pid, SIGTERM);
            if (self.stdin_fd >= 0) _ = c.close(self.stdin_fd);
            if (self.stdout_fd >= 0) _ = c.close(self.stdout_fd);
            self.stdin_fd = -1;
            self.stdout_fd = -1;
            if (self.pid > 0) {
                _ = c.waitpid(self.pid, null, 1);
                self.pid = -1;
            }
        }
    }
};

fn spawnProcess(
    allocator: std.mem.Allocator,
    command: []const u8,
    args: []const []const u8,
    cwd: []const u8,
) !struct { pid: c_int, stdin_fd: c_int, stdout_fd: c_int } {
    var in_pipe: [2]c_int = undefined;
    var out_pipe: [2]c_int = undefined;
    if (c.pipe(&in_pipe) != 0) return error.PipeFailed;
    if (c.pipe(&out_pipe) != 0) {
        _ = c.close(in_pipe[0]);
        _ = c.close(in_pipe[1]);
        return error.PipeFailed;
    }
    const pid = c.fork();
    if (pid < 0) {
        _ = c.close(in_pipe[0]);
        _ = c.close(in_pipe[1]);
        _ = c.close(out_pipe[0]);
        _ = c.close(out_pipe[1]);
        return error.ForkFailed;
    }
    if (pid == 0) {
        _ = c.close(in_pipe[1]);
        _ = c.close(out_pipe[0]);
        _ = c.dup2(in_pipe[0], 0);
        _ = c.dup2(out_pipe[1], 1);
        _ = c.close(in_pipe[0]);
        _ = c.close(out_pipe[1]);
        if (cwd.len > 0) {
            const cwd_z = allocator.dupeZ(u8, cwd) catch c._exit(127);
            _ = c.chdir(cwd_z.ptr);
        }
        const argc = 1 + args.len + 1;
        const argv = allocator.alloc(?[*:0]const u8, argc) catch c._exit(127);
        const cmd_z = allocator.dupeZ(u8, command) catch c._exit(127);
        argv[0] = cmd_z.ptr;
        for (args, 1..) |a, i| {
            argv[i] = (allocator.dupeZ(u8, a) catch c._exit(127)).ptr;
        }
        argv[argc - 1] = null;
        _ = c.execvp(cmd_z.ptr, @ptrCast(argv.ptr));
        c._exit(127);
    }
    _ = c.close(in_pipe[0]);
    _ = c.close(out_pipe[1]);
    return .{ .pid = pid, .stdin_fd = in_pipe[1], .stdout_fd = out_pipe[0] };
}

fn resolveCommand(allocator: std.mem.Allocator, command: []const u8, env_map: *std.process.Environ.Map) []const u8 {
    if (std.mem.indexOfScalar(u8, command, '/') != null) {
        const z = allocator.dupeZ(u8, command) catch return command;
        defer allocator.free(z);
        if (c.access(z, F_OK) == 0) return allocator.dupe(u8, command) catch command;
    }
    const home = svc.homeDir(env_map);
    const candidates = [_][]const u8{
        std.fmt.allocPrint(allocator, "{s}/homebrew/bin/{s}", .{ home, command }) catch "",
        std.fmt.allocPrint(allocator, "/opt/homebrew/bin/{s}", .{command}) catch "",
        std.fmt.allocPrint(allocator, "/usr/local/bin/{s}", .{command}) catch "",
        std.fmt.allocPrint(allocator, "{s}/go/bin/{s}", .{ home, command }) catch "",
        std.fmt.allocPrint(allocator, "{s}/.cargo/bin/{s}", .{ home, command }) catch "",
    };
    for (candidates) |cand| {
        defer allocator.free(cand);
        if (cand.len == 0) continue;
        const z = allocator.dupeZ(u8, cand) catch continue;
        defer allocator.free(z);
        if (c.access(z, F_OK) == 0) return cand;
    }
    if (env_map.get("PATH")) |path_env| {
        var it = std.mem.splitScalar(u8, path_env, ':');
        while (it.next()) |dir| {
            if (dir.len == 0) continue;
            const cand = std.fmt.allocPrint(allocator, "{s}/{s}", .{ dir, command }) catch continue;
            defer allocator.free(cand);
            const z = allocator.dupeZ(u8, cand) catch continue;
            defer allocator.free(z);
            if (c.access(z, F_OK) == 0) return cand;
        }
    }
    return command;
}

/// Server config candidates per language id (port of KNOWN_LSP_SERVERS).
fn serverCandidates(lang: []const u8) []const struct { command: []const u8, args: []const []const u8 } {
    const ts = [_][]const u8{"--stdio"};
    const tsx = [_][]const u8{"-y", "typescript-language-server", "--stdio"};
    const noargs = [_][]const u8{};
    const clangd = [_][]const u8{ "--background-index", "--clang-tidy" };
    const vscode_html = [_][]const u8{"--stdio"};
    const vscode_css = [_][]const u8{"--stdio"};
    const vscode_json = [_][]const u8{"--stdio"};
    const pyright = [_][]const u8{"--stdio"};
    const npx_pyright = [_][]const u8{ "-y", "pyright", "--stdio" };
    const sourcekit = [_][]const u8{};
    const xcrun = [_][]const u8{"sourcekit-lsp"};
    if (std.mem.eql(u8, lang, "typescript") or std.mem.eql(u8, lang, "javascript") or
        std.mem.eql(u8, lang, "typescriptreact") or std.mem.eql(u8, lang, "javascriptreact"))
    {
        return &.{
            .{ .command = "typescript-language-server", .args = &ts },
            .{ .command = "vtsls", .args = &ts },
            .{ .command = "bunx", .args = &tsx },
            .{ .command = "npx", .args = &tsx },
        };
    }
    if (std.mem.eql(u8, lang, "go")) {
        return &.{ .{ .command = "gopls", .args = &noargs } };
    }
    if (std.mem.eql(u8, lang, "python")) {
        return &.{
            .{ .command = "pyright-langserver", .args = &pyright },
            .{ .command = "pylsp", .args = &noargs },
            .{ .command = "basedpyright-langserver", .args = &pyright },
            .{ .command = "npx", .args = &npx_pyright },
        };
    }
    if (std.mem.eql(u8, lang, "rust")) {
        return &.{ .{ .command = "rust-analyzer", .args = &noargs } };
    }
    if (std.mem.eql(u8, lang, "zig")) {
        return &.{ .{ .command = "zls", .args = &noargs } };
    }
    if (std.mem.eql(u8, lang, "cpp") or std.mem.eql(u8, lang, "c")) {
        return &.{ .{ .command = "clangd", .args = &clangd } };
    }
    if (std.mem.eql(u8, lang, "swift")) {
        return &.{
            .{ .command = "sourcekit-lsp", .args = &sourcekit },
            .{ .command = "xcrun", .args = &xcrun },
        };
    }
    if (std.mem.eql(u8, lang, "html") or std.mem.eql(u8, lang, "htm")) {
        return &.{ .{ .command = "vscode-html-language-server", .args = &vscode_html } };
    }
    if (std.mem.eql(u8, lang, "css") or std.mem.eql(u8, lang, "scss") or std.mem.eql(u8, lang, "less")) {
        return &.{ .{ .command = "vscode-css-language-server", .args = &vscode_css } };
    }
    if (std.mem.eql(u8, lang, "json")) {
        return &.{ .{ .command = "vscode-json-language-server", .args = &vscode_json } };
    }
    return &.{};
}

/// Language id from a file path.
pub fn languageIdFromPath(path: []const u8) []const u8 {
    const ext = std.fs.path.extension(path);
    if (std.mem.eql(u8, ext, ".ts")) return "typescript";
    if (std.mem.eql(u8, ext, ".tsx")) return "typescriptreact";
    if (std.mem.eql(u8, ext, ".js")) return "javascript";
    if (std.mem.eql(u8, ext, ".jsx")) return "javascriptreact";
    if (std.mem.eql(u8, ext, ".mts")) return "typescript";
    if (std.mem.eql(u8, ext, ".cts")) return "typescript";
    if (std.mem.eql(u8, ext, ".mjs")) return "javascript";
    if (std.mem.eql(u8, ext, ".cjs")) return "javascript";
    if (std.mem.eql(u8, ext, ".go")) return "go";
    if (std.mem.eql(u8, ext, ".py")) return "python";
    if (std.mem.eql(u8, ext, ".rs")) return "rust";
    if (std.mem.eql(u8, ext, ".zig")) return "zig";
    if (std.mem.eql(u8, ext, ".c")) return "c";
    if (std.mem.eql(u8, ext, ".h")) return "c";
    if (std.mem.eql(u8, ext, ".cpp") or std.mem.eql(u8, ext, ".cc") or std.mem.eql(u8, ext, ".hpp")) return "cpp";
    if (std.mem.eql(u8, ext, ".swift")) return "swift";
    if (std.mem.eql(u8, ext, ".html") or std.mem.eql(u8, ext, ".htm")) return "html";
    if (std.mem.eql(u8, ext, ".css") or std.mem.eql(u8, ext, ".scss") or std.mem.eql(u8, ext, ".less")) return "css";
    if (std.mem.eql(u8, ext, ".json")) return "json";
    if (std.mem.eql(u8, ext, ".java")) return "java";
    if (std.mem.eql(u8, ext, ".kt") or std.mem.eql(u8, ext, ".kts")) return "kotlin";
    return "";
}

/// Spawns the LSP server for a language and performs the initialize handshake.
pub fn start(
    allocator: std.mem.Allocator,
    language_id: []const u8,
    workspace_root: []const u8,
    env_map: *std.process.Environ.Map,
) !*Client {
    const candidates = serverCandidates(language_id);
    if (candidates.len == 0) return error.UnsupportedLanguage;

    var last_err: anyerror = error.NotFound;
    for (candidates) |cand| {
        const resolved = resolveCommand(allocator, cand.command, env_map);
        defer if (!std.mem.eql(u8, resolved, cand.command)) allocator.free(resolved);
        if (std.mem.indexOfScalar(u8, resolved, '/') == null and !std.mem.eql(u8, resolved, cand.command)) {
            // resolved stayed bare — command not found.
        }
        // Check existence: bare command that didn't resolve to a path.
        if (std.mem.indexOfScalar(u8, resolved, '/') == null and std.mem.eql(u8, resolved, cand.command)) {
            last_err = error.CommandNotFound;
            continue;
        }

        const procs = spawnProcess(allocator, resolved, cand.args, workspace_root) catch |err| {
            last_err = err;
            continue;
        };
        const shared = try allocator.create(Shared);
        shared.* = Shared.init(allocator);
        const client = try allocator.create(Client);
        client.* = .{
            .allocator = allocator,
            .language_id = allocator.dupe(u8, language_id) catch "",
            .workspace_root = allocator.dupe(u8, workspace_root) catch "",
            .command = allocator.dupe(u8, cand.command) catch "",
            .args = cand.args,
            .status = "starting",
            .pid = procs.pid,
            .stdin_fd = procs.stdin_fd,
            .stdout_fd = procs.stdout_fd,
            .shared = shared,
        };
        client.open_docs = std.ArrayList([]const u8).empty;
        client.open_init = true;
        errdefer client.deinit();

        const thread = try std.Thread.spawn(.{}, readerLoop, .{client});
        thread.detach();

        // Initialize handshake.
        const root_uri = try fileUri(allocator, workspace_root);
        defer allocator.free(root_uri);
        const init_params = .{
            .processId = null,
            .clientInfo = .{ .name = "forge-ade", .version = "0.1.0" },
            .rootUri = root_uri,
            .capabilities = .{
                .textDocument = .{
                    .completion = .{ .completionItem = .{ .snippetSupport = true } },
                    .hover = .{ .contentFormat = &.{"markdown"} },
                    .definition = .{},
                    .declaration = .{},
                    .typeDefinition = .{},
                    .implementation = .{},
                },
                .workspace = .{ .workspaceFolders = .{} },
            },
            .workspaceFolders = @as([]struct {}, &.{}),
        };
        const init_res = request(client, allocator, "initialize", init_params, 15_000) catch |err| {
            last_err = err;
            client.deinit();
            continue;
        };
        svc.deepFreeValue(allocator, init_res);
        try notify(client, allocator, "initialized", .{});
        client.initialized = true;
        client.status = "running";
        return client;
    }
    return last_err;
}

fn fileUri(allocator: std.mem.Allocator, path: []const u8) ![]const u8 {
    if (std.mem.startsWith(u8, path, "file://")) return allocator.dupe(u8, path) catch error.OutOfMemory;
    // Absolute path → file:///...
    var out = std.ArrayList(u8).empty;
    try out.appendSlice(allocator, "file://");
    for (path) |ch| {
        if (ch == ' ') {
            try out.appendSlice(allocator, "%20");
        } else {
            try out.append(allocator, ch);
        }
    }
    return out.toOwnedSlice(allocator);
}

fn uriToPath(allocator: std.mem.Allocator, uri: []const u8) []const u8 {
    if (std.mem.startsWith(u8, uri, "file://")) {
        return allocator.dupe(u8, uri["file://".len..]) catch "";
    }
    return allocator.dupe(u8, uri) catch "";
}

fn readerLoop(self: *Client) void {
    defer self.reader_done.store(true, .release);
    var buf: [65536]u8 = undefined;
    var recv = std.ArrayList(u8).empty;
    defer recv.deinit(self.allocator);
    while (true) {
        const n = c.read(self.stdout_fd, &buf, buf.len);
        if (n <= 0) {
            self.shared.mutex.lock();
            self.shared.alive = false;
            self.shared.mutex.unlock();
            return;
        }
        recv.appendSlice(self.allocator, buf[0..@intCast(n)]) catch {};
        // Parse Content-Length framed messages.
        while (true) {
            const header_end = std.mem.indexOf(u8, recv.items, "\r\n\r\n") orelse break;
            const header = recv.items[0..header_end];
            const len_marker = "Content-Length:";
            const len_idx = std.mem.indexOf(u8, header, len_marker) orelse {
                // Skip malformed header.
                var remaining = std.ArrayList(u8).empty;
                remaining.appendSlice(self.allocator, recv.items[header_end + 4 ..]) catch {};
                recv.deinit(self.allocator);
                recv = remaining;
                continue;
            };
            var rest = header[len_idx + len_marker.len ..];
            const rest_len = std.mem.indexOfAny(u8, rest, " \r\t") orelse rest.len;
            const len_str = std.mem.trim(u8, rest[0..rest_len], " \r\t");
            const content_length = std.fmt.parseInt(usize, len_str, 10) catch break;
            const total = header_end + 4 + content_length;
            if (recv.items.len < total) break;
            const body = recv.items[header_end + 4 .. total];
            handleFrame(self, self.allocator, body) catch {};
            // Remove consumed bytes.
            var remaining = std.ArrayList(u8).empty;
            remaining.appendSlice(self.allocator, recv.items[total..]) catch {};
            recv.deinit(self.allocator);
            recv = remaining;
        }
    }
}

fn handleFrame(self: *Client, allocator: std.mem.Allocator, body: []const u8) !void {
    const parsed = std.json.parseFromSlice(std.json.Value, allocator, body, .{}) catch return;
    defer parsed.deinit();
    const root = parsed.value;
    if (root != .object) return;
    const obj = root.object;

    // Response to a request.
    if (obj.get("id")) |idv| {
        if (idv != .integer) return;
        const id: u64 = @intCast(@max(idv.integer, 0));
        self.shared.mutex.lock();
        defer self.shared.mutex.unlock();
        if (obj.get("error")) |errv| {
            if (errv == .object) {
                var msg: []const u8 = "lsp error";
                if (errv.object.get("message")) |mv| {
                    if (mv == .string) msg = mv.string;
                }
                if (self.shared.errors) |*errs| {
                    if (errs.get(id)) |old| allocator.free(old);
                    errs.put(id, allocator.dupe(u8, msg) catch "") catch {};
                }
            }
        } else if (obj.get("result")) |rv| {
            if (self.shared.responses) |*resps| {
                // Deep-copy: `rv` aliases this frame's parse arena, which is
                // freed when handleFrame returns. The map owns the copy.
                const copy = svc.deepCopyValue(allocator, rv);
                if (resps.get(id)) |old| svc.deepFreeValue(allocator, old);
                resps.put(id, copy) catch svc.deepFreeValue(allocator, copy);
            }
        }
        return;
    }

    // Server notification (publishDiagnostics).
    if (obj.get("method")) |m| {
        if (m == .string and std.mem.eql(u8, m.string, "textDocument/publishDiagnostics")) {
            if (obj.get("params")) |pv| {
                if (pv == .object) {
                    var uri: []const u8 = "";
                    var diags_json: []const u8 = "[]";
                    if (pv.object.get("uri")) |uv| {
                        if (uv == .string) uri = uv.string;
                    }
                    if (pv.object.get("diagnostics")) |dv| {
                        var out = std.Io.Writer.Allocating.init(allocator);
                        defer out.deinit();
                        try std.json.Stringify.value(dv, .{}, &out.writer);
                        diags_json = out.toOwnedSlice() catch "[]";
                    }
                    if (uri.len > 0) {
                        const path = uriToPath(allocator, uri);
                        self.shared.mutex.lock();
                        if (self.shared.diagnostics.get(path)) |old| allocator.free(old);
                        self.shared.diagnostics.put(path, diags_json) catch {};
                        self.shared.mutex.unlock();
                    }
                }
            }
        }
    }
}

fn writeRaw(self: *Client, data: []const u8) !void {
    if (!self.alive or self.stdin_fd < 0) return error.NotConnected;
    var written: usize = 0;
    while (written < data.len) {
        const n = c.write(self.stdin_fd, data[written..].ptr, data.len - written);
        if (n <= 0) return error.WriteFailed;
        written += @intCast(n);
    }
}

fn sendFrame(self: *Client, allocator: std.mem.Allocator, value: anytype) !void {
    var out = std.Io.Writer.Allocating.init(allocator);
    defer out.deinit();
    try std.json.Stringify.value(value, .{}, &out.writer);
    const body = out.written();
    var header = std.ArrayList(u8).empty;
    defer header.deinit(allocator);
    try header.appendSlice(allocator, "Content-Length: ");
    try header.print(allocator, "{d}", .{body.len});
    try header.appendSlice(allocator, "\r\n\r\n");
    try writeRaw(self, header.items);
    try writeRaw(self, body);
}

pub fn request(self: *Client, allocator: std.mem.Allocator, method: []const u8, params: anytype, timeout_ms: u32) !std.json.Value {
    // OWNERSHIP: the returned Value is a deep copy owned by the CALLER
    // (free with svc.deepFreeValue).
    self.shared.mutex.lock();
    const id = self.next_id;
    self.next_id += 1;
    self.shared.mutex.unlock();

    var out = std.Io.Writer.Allocating.init(allocator);
    defer out.deinit();
    try std.json.Stringify.value(.{
        .jsonrpc = "2.0",
        .id = id,
        .method = method,
        .params = params,
    }, .{}, &out.writer);
    const body = out.written();
    var header = std.ArrayList(u8).empty;
    defer header.deinit(allocator);
    try header.appendSlice(allocator, "Content-Length: ");
    try header.print(allocator, "{d}", .{body.len});
    try header.appendSlice(allocator, "\r\n\r\n");
    try writeRaw(self, header.items);
    try writeRaw(self, body);

    const deadline = svc.nowMs() + @as(i64, @intCast(timeout_ms));
    while (true) {
        self.shared.mutex.lock();
        if (self.shared.errors) |*errs| {
            if (errs.get(id)) |err| {
                const owned = self.allocator.dupe(u8, err) catch "lsp request failed";
                _ = errs.remove(id);
                self.shared.mutex.unlock();
                defer self.allocator.free(owned);
                return error.LspRequestFailed;
            }
        }
        if (self.shared.responses) |*resps| {
            if (resps.get(id)) |val| {
                _ = resps.remove(id);
                self.shared.mutex.unlock();
                return val;
            }
        }
        const conn_alive = self.shared.alive;
        self.shared.mutex.unlock();
        if (!conn_alive) return error.NotConnected;
        const now = svc.nowMs();
        if (now >= deadline) return error.Timeout;
        threadSleepMs(5);
    }
}

pub fn notify(self: *Client, allocator: std.mem.Allocator, method: []const u8, params: anytype) !void {
    var out = std.Io.Writer.Allocating.init(allocator);
    defer out.deinit();
    try std.json.Stringify.value(.{
        .jsonrpc = "2.0",
        .method = method,
        .params = params,
    }, .{}, &out.writer);
    const body = out.written();
    var header = std.ArrayList(u8).empty;
    defer header.deinit(allocator);
    try header.appendSlice(allocator, "Content-Length: ");
    try header.print(allocator, "{d}", .{body.len});
    try header.appendSlice(allocator, "\r\n\r\n");
    try writeRaw(self, header.items);
    try writeRaw(self, body);
}

/// didOpen — register the document with the server.
pub fn didOpen(self: *Client, allocator: std.mem.Allocator, path: []const u8, content: []const u8, version: i64) !void {
    const uri = try fileUri(allocator, path);
    defer allocator.free(uri);
    try notify(self, allocator, "textDocument/didOpen", .{
        .textDocument = .{
            .uri = uri,
            .languageId = self.language_id,
            .version = version,
            .text = content,
        },
    });
    self.shared.mutex.lock();
    defer self.shared.mutex.unlock();
    if (!self.open_init) {
        self.open_docs = std.ArrayList([]const u8).empty;
        self.open_init = true;
    }
    var found = false;
    for (self.open_docs.items) |p| {
        if (std.mem.eql(u8, p, path)) { found = true; break; }
    }
    if (!found) self.open_docs.append(self.allocator, self.allocator.dupe(u8, path) catch "") catch {};
}

pub fn didChange(self: *Client, allocator: std.mem.Allocator, path: []const u8, content: []const u8, version: i64) !void {
    const uri = try fileUri(allocator, path);
    defer allocator.free(uri);
    try notify(self, allocator, "textDocument/didChange", .{
        .textDocument = .{ .uri = uri, .version = version },
        .contentChanges = &.{.{ .text = content }},
    });
}

pub fn didSave(self: *Client, allocator: std.mem.Allocator, path: []const u8, content: []const u8) !void {
    const uri = try fileUri(allocator, path);
    defer allocator.free(uri);
    try notify(self, allocator, "textDocument/didSave", .{
        .textDocument = .{ .uri = uri },
        .text = content,
    });
}

pub fn didClose(self: *Client, allocator: std.mem.Allocator, path: []const u8) !void {
    const uri = try fileUri(allocator, path);
    defer allocator.free(uri);
    try notify(self, allocator, "textDocument/didClose", .{
        .textDocument = .{ .uri = uri },
    });
}

pub fn completion(self: *Client, allocator: std.mem.Allocator, path: []const u8, line: i64, character: i64) !std.json.Value {
    const uri = try fileUri(allocator, path);
    defer allocator.free(uri);
    return try request(self, allocator, "textDocument/completion", .{
        .textDocument = .{ .uri = uri },
        .position = .{ .line = line, .character = character },
    }, 8_000);
}

pub fn hover(self: *Client, allocator: std.mem.Allocator, path: []const u8, line: i64, character: i64) !std.json.Value {
    const uri = try fileUri(allocator, path);
    defer allocator.free(uri);
    return try request(self, allocator, "textDocument/hover", .{
        .textDocument = .{ .uri = uri },
        .position = .{ .line = line, .character = character },
    }, 8_000);
}

pub fn definition(self: *Client, allocator: std.mem.Allocator, path: []const u8, line: i64, character: i64) !std.json.Value {
    const uri = try fileUri(allocator, path);
    defer allocator.free(uri);
    return try request(self, allocator, "textDocument/definition", .{
        .textDocument = .{ .uri = uri },
        .position = .{ .line = line, .character = character },
    }, 8_000);
}

pub fn declaration(self: *Client, allocator: std.mem.Allocator, path: []const u8, line: i64, character: i64) !std.json.Value {
    const uri = try fileUri(allocator, path);
    defer allocator.free(uri);
    return try request(self, allocator, "textDocument/declaration", .{
        .textDocument = .{ .uri = uri },
        .position = .{ .line = line, .character = character },
    }, 8_000);
}

pub fn typeDefinition(self: *Client, allocator: std.mem.Allocator, path: []const u8, line: i64, character: i64) !std.json.Value {
    const uri = try fileUri(allocator, path);
    defer allocator.free(uri);
    return try request(self, allocator, "textDocument/typeDefinition", .{
        .textDocument = .{ .uri = uri },
        .position = .{ .line = line, .character = character },
    }, 8_000);
}

pub fn implementation(self: *Client, allocator: std.mem.Allocator, path: []const u8, line: i64, character: i64) !std.json.Value {
    const uri = try fileUri(allocator, path);
    defer allocator.free(uri);
    return try request(self, allocator, "textDocument/implementation", .{
        .textDocument = .{ .uri = uri },
        .position = .{ .line = line, .character = character },
    }, 8_000);
}

/// Returns the accumulated diagnostics map {filePath: diagnosticsJson}.
pub fn allDiagnostics(self: *Client, allocator: std.mem.Allocator) !std.json.Value {
    self.shared.mutex.lock();
    defer self.shared.mutex.unlock();
    var map = std.json.ObjectMap.empty;
    var it = self.shared.diagnostics.iterator();
    while (it.next()) |e| {
        const parsed = std.json.parseFromSlice(std.json.Value, allocator, e.value_ptr.*, .{}) catch continue;
        map.put(allocator, e.key_ptr.*, parsed.value) catch {};
    }
    return std.json.Value{ .object = map };
}

pub fn serverInfo(self: *Client) ServerInfo {
    var args_list = std.ArrayList([]const u8).empty;
    for (self.args) |a| args_list.append(std.heap.c_allocator, a) catch {};
    return .{
        .languageId = self.language_id,
        .name = self.command,
        .command = self.command,
        .args = args_list.toOwnedSlice(std.heap.c_allocator) catch &.{},
        .status = self.status,
        .workspaceRoot = self.workspace_root,
        .openDocumentsCount = self.open_docs.items.len,
    };
}
