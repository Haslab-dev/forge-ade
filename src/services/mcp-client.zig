// services.mcp-client — minimal MCP stdio client (JSON-RPC 2.0 over
// newline-delimited frames on a subprocess's stdin/stdout).
// Port of src/server/mcp/client.ts (McpStdioClient).

const std = @import("std");
const svc = @import("../services.zig");

pub const ToolDef = struct {
    name: []const u8 = "",
    description: []const u8 = "",
    input_schema: ?std.json.Value = null,
};

pub const CallResult = struct {
    content: []const u8 = "",
    is_error: bool = false,
};

pub const ProtocolVersion = "2024-11-05";

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

/// Spinlock (Zig 0.16 removed std.Thread.Mutex; this mirrors main.zig's).
const SpinLock = struct {
    locked: std.atomic.Value(bool) = std.atomic.Value(bool).init(false),
    fn lock(self: *SpinLock) void {
        while (self.locked.swap(true, .acquire)) std.atomic.spinLoopHint();
    }
    fn unlock(self: *SpinLock) void {
        self.locked.store(false, .release);
    }
};

/// Portable nanosleep (Zig 0.16's std.Io.sleep needs an Io handle).
const Timespec = extern struct { tv_sec: isize, tv_nsec: isize };
extern "c" fn nanosleep(req: *const Timespec, rem: ?*Timespec) c_int;

fn threadSleepMs(ms: u64) void {
    const sec: isize = @intCast(@divTrunc(ms, 1000));
    const nsec: isize = @intCast(@mod(ms, 1000) * 1_000_000);
    var ts = Timespec{ .tv_sec = sec, .tv_nsec = nsec };
    _ = nanosleep(&ts, null);
}

/// Shared, thread-safe response state. The reader thread parses frames and
/// fills these; the worker thread polls with a bounded timeout.
///
/// Response values stored in `responses` are DEEP COPIES owned by the map
/// (never arena aliases — the parse arena dies at the end of handleFrame).
pub const Shared = struct {
    mutex: SpinLock = .{},
    responses: ?std.AutoHashMap(u64, std.json.Value) = null,
    errors: ?std.AutoHashMap(u64, []const u8) = null,
    alive: bool = true,

    pub fn init(allocator: std.mem.Allocator) Shared {
        return .{
            .responses = std.AutoHashMap(u64, std.json.Value).init(allocator),
            .errors = std.AutoHashMap(u64, []const u8).init(allocator),
        };
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
    }
};

/// A live stdio MCP connection. Owns the child pid + pipe fds.
pub const Connection = struct {
    allocator: std.mem.Allocator,
    server_name: []const u8 = "",
    pid: c_int = -1,
    stdin_fd: c_int = -1,
    stdout_fd: c_int = -1,
    alive: bool = true,
    next_id: u64 = 1,
    shared: *Shared,
    /// Set by the reader thread when it exits; deinit waits for it so the
    /// Connection is never freed under a live reader (use-after-free).
    reader_done: std.atomic.Value(bool) = std.atomic.Value(bool).init(false),

    pub fn deinit(self: *Connection) void {
        self.close();
        // Wait (bounded) for the reader thread to observe the closed pipe and
        // exit before freeing anything it touches.
        var waited: u32 = 0;
        while (!self.reader_done.load(.acquire) and waited < 3000) {
            threadSleepMs(2);
            waited += 2;
        }
        self.shared.deinit();
        self.allocator.destroy(self.shared);
        self.allocator.free(self.server_name);
        self.allocator.destroy(self);
    }

    pub fn close(self: *Connection) void {
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
        // Child: stdin ← in_pipe[0], stdout → out_pipe[1].
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

/// Spawns the server and performs the initialize handshake. Returns a
/// connected Connection (caller owns it; free with deinit()).
pub fn spawn(
    allocator: std.mem.Allocator,
    command: []const u8,
    args: []const []const u8,
    env_map: *std.process.Environ.Map,
    cwd: []const u8,
    server_name: []const u8,
) !*Connection {
    const resolved = resolveCommand(allocator, command, env_map);
    defer if (!std.mem.eql(u8, resolved, command)) allocator.free(resolved);

    const procs = try spawnProcess(allocator, resolved, args, cwd);
    const shared = try allocator.create(Shared);
    shared.* = Shared.init(allocator);
    const conn = try allocator.create(Connection);
    conn.* = .{
        .allocator = allocator,
        .server_name = allocator.dupe(u8, server_name) catch "",
        .pid = procs.pid,
        .stdin_fd = procs.stdin_fd,
        .stdout_fd = procs.stdout_fd,
        .shared = shared,
    };
    errdefer conn.deinit();

    // Start the reader thread.
    const thread = try std.Thread.spawn(.{}, readerLoop, .{conn});
    thread.detach();

    // Initialize handshake.
    const init_res = try request(conn, allocator, "initialize", .{
        .protocolVersion = ProtocolVersion,
        .capabilities = .{},
        .clientInfo = .{ .name = "forge-ade", .version = "1.0.0" },
    }, 25_000);
    svc.deepFreeValue(allocator, init_res);
    try notify(conn, allocator, "notifications/initialized", null);
    return conn;
}

fn readerLoop(self: *Connection) void {
    defer self.reader_done.store(true, .release);
    var buf: [16384]u8 = undefined;
    var line_buf = std.ArrayList(u8).empty;
    defer line_buf.deinit(self.allocator);
    while (true) {
        const n = c.read(self.stdout_fd, &buf, buf.len);
        if (n <= 0) {
            self.shared.mutex.lock();
            self.shared.alive = false;
            self.shared.mutex.unlock();
            return;
        }
        for (buf[0..@intCast(n)]) |ch| {
            if (ch == '\n') {
                const line = std.mem.trim(u8, line_buf.items, " \r\t");
                if (line.len > 0) handleFrame(self, self.allocator, line);
                line_buf.clearRetainingCapacity();
            } else {
                line_buf.append(self.allocator, ch) catch {};
            }
        }
    }
}

fn handleFrame(self: *Connection, allocator: std.mem.Allocator, line: []const u8) void {
    const parsed = std.json.parseFromSlice(std.json.Value, allocator, line, .{}) catch return;
    defer parsed.deinit();
    const root = parsed.value;
    if (root != .object) return;
    const obj = root.object;

    // Server-initiated request (ping / method not found).
    if (obj.get("method")) |m| {
        if (m == .string and obj.get("id") != null) {
            const idv = obj.get("id").?;
            if (idv == .integer) {
                if (std.mem.eql(u8, m.string, "ping")) {
                    sendJson(self, allocator, .{ .jsonrpc = "2.0", .id = idv.integer, .result = .{} }) catch {};
                } else {
                    // Build {"jsonrpc":"2.0","id":N,"error":{"code":-32601,"message":"..."}} manually
                    // — `.error` is a reserved word in struct literals.
                    var out2 = std.Io.Writer.Allocating.init(allocator);
                    defer out2.deinit();
                    std.json.Stringify.value(.{
                        .jsonrpc = "2.0",
                        .id = idv.integer,
                    }, .{}, &out2.writer) catch {};
                    out2.writer.writeAll(",\"error\":{\"code\":-32601,\"message\":") catch {};
                    std.json.Stringify.value("method not found", .{}, &out2.writer) catch {};
                    out2.writer.writeAll("}}") catch {};
                    writeRaw(self, out2.written()) catch {};
                    writeRaw(self, "\n") catch {};
                }
            }
        }
        return;
    }

    // Response to our request.
    if (obj.get("id")) |idv| {
        if (idv != .integer) return;
        const id: u64 = @intCast(@max(idv.integer, 0));
        self.shared.mutex.lock();
        defer self.shared.mutex.unlock();
        if (obj.get("error")) |errv| {
            if (errv == .object) {
                var msg: []const u8 = "rpc error";
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
    }
}

fn sendJson(self: *Connection, allocator: std.mem.Allocator, value: anytype) !void {
    var out = std.Io.Writer.Allocating.init(allocator);
    defer out.deinit();
    try std.json.Stringify.value(value, .{}, &out.writer);
    try writeRaw(self, out.written());
    try writeRaw(self, "\n");
}

fn writeRaw(self: *Connection, data: []const u8) !void {
    if (!self.alive or self.stdin_fd < 0) return error.NotConnected;
    var written: usize = 0;
    while (written < data.len) {
        const n = c.write(self.stdin_fd, data[written..].ptr, data.len - written);
        if (n <= 0) return error.WriteFailed;
        written += @intCast(n);
    }
}

/// Sends a request and waits for the response. Returns the result Value —
/// OWNERSHIP: the returned Value is a deep copy owned by the CALLER
/// (freed with svc.deepFreeValue). The map entry is consumed here.
pub fn request(self: *Connection, allocator: std.mem.Allocator, method: []const u8, params: anytype, timeout_ms: u32) !std.json.Value {
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
    try writeRaw(self, out.written());
    try writeRaw(self, "\n");

    const deadline = svc.nowMs() + @as(i64, @intCast(timeout_ms));
    while (true) {
        self.shared.mutex.lock();
        if (self.shared.errors) |*errs| {
            if (errs.get(id)) |err| {
                const owned = self.allocator.dupe(u8, err) catch "mcp request failed";
                _ = errs.remove(id);
                self.shared.mutex.unlock();
                defer self.allocator.free(owned);
                return error.McpRequestFailed;
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

pub fn notify(self: *Connection, allocator: std.mem.Allocator, method: []const u8, params: anytype) !void {
    var out = std.Io.Writer.Allocating.init(allocator);
    defer out.deinit();
    try std.json.Stringify.value(.{
        .jsonrpc = "2.0",
        .method = method,
        .params = params,
    }, .{}, &out.writer);
    try writeRaw(self, out.written());
    try writeRaw(self, "\n");
}

/// Calls tools/list, returns the tool definitions. The ToolDefs (including
/// their input_schema Values) are caller-owned deep copies.
pub fn listTools(self: *Connection, allocator: std.mem.Allocator) ![]ToolDef {
    const res = try request(self, allocator, "tools/list", .{}, 10_000);
    defer svc.deepFreeValue(allocator, res);
    var out = std.ArrayList(ToolDef).empty;
    defer out.deinit(allocator);
    if (res == .object) {
        if (res.object.get("tools")) |tv| {
            if (tv == .array) {
                for (tv.array.items) |item| {
                    if (item != .object) continue;
                    const obj = item.object;
                    var name: []const u8 = "";
                    if (obj.get("name")) |nv| {
                        if (nv == .string) name = nv.string;
                    }
                    if (name.len == 0) continue;
                    var desc: []const u8 = "";
                    if (obj.get("description")) |dv| {
                        if (dv == .string) desc = dv.string;
                    }
                    var schema: ?std.json.Value = null;
                    if (obj.get("inputSchema")) |sv| {
                        schema = svc.deepCopyValue(allocator, sv);
                    }
                    out.append(allocator, .{
                        .name = allocator.dupe(u8, name) catch {
                            if (schema) |s| svc.deepFreeValue(allocator, s);
                            continue;
                        },
                        .description = allocator.dupe(u8, desc) catch "",
                        .input_schema = schema,
                    }) catch {};
                }
            }
        }
    }
    return out.toOwnedSlice(allocator) catch &.{};
}

/// Calls tools/call with the given arguments, returns text content.
pub fn callTool(self: *Connection, allocator: std.mem.Allocator, name: []const u8, args: std.json.Value) !CallResult {
    const res = try request(self, allocator, "tools/call", .{
        .name = name,
        .arguments = args,
    }, 30_000);
    defer svc.deepFreeValue(allocator, res);
    var content = std.ArrayList(u8).empty;
    defer content.deinit(allocator);
    var is_error = false;
    if (res == .object) {
        const obj = res.object;
        if (obj.get("isError")) |ev| {
            if (ev == .bool) is_error = ev.bool;
        }
        if (obj.get("content")) |cv| {
            if (cv == .string) {
                content.appendSlice(allocator, cv.string) catch {};
            } else if (cv == .array) {
                for (cv.array.items) |block| {
                    if (block != .object) continue;
                    if (block.object.get("type")) |tv| {
                        if (tv == .string and std.mem.eql(u8, tv.string, "text")) {
                            if (block.object.get("text")) |tvv| {
                                if (tvv == .string) {
                                    if (content.items.len > 0) content.append(allocator, '\n') catch {};
                                    content.appendSlice(allocator, tvv.string) catch {};
                                }
                            }
                        }
                    }
                }
            }
        }
    }
    return .{
        .content = content.toOwnedSlice(allocator) catch "",
        .is_error = is_error,
    };
}

/// Splits `mcp_<server>_<tool>` into (server, tool).
pub fn splitQualified(qualified: []const u8) ?struct { server: []const u8, tool: []const u8 } {
    if (!std.mem.startsWith(u8, qualified, "mcp_")) return null;
    const rest = qualified["mcp_".len..];
    const sep = std.mem.indexOfScalar(u8, rest, '_') orelse return null;
    if (sep <= 0 or sep == rest.len - 1) return null;
    return .{ .server = rest[0..sep], .tool = rest[sep + 1 ..] };
}

pub fn isQualifiedToolName(name: []const u8) bool {
    return splitQualified(name) != null;
}

test "splitQualified handles mcp_server_tool" {
    const p = splitQualified("mcp_my-server_read_file").?;
    try std.testing.expectEqualStrings("my-server", p.server);
    try std.testing.expectEqualStrings("read_file", p.tool);
    try std.testing.expect(splitQualified("notmcp_x") == null);
    try std.testing.expect(splitQualified("mcp_") == null);
}
