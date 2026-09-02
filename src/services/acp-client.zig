// services.acp-client — minimal ACP (Agent Client Protocol) connection.
// Speaks newline-delimited JSON-RPC 2.0 over a spawned agent process's stdio.
// Port of src/server/acp/client.ts.

const std = @import("std");
const svc = @import("../services.zig");

pub const ProtocolVersion: i64 = 1;

pub const ConfigOption = struct {
    id: []const u8 = "",
    name: []const u8 = "",
    description: []const u8 = "",
    category: []const u8 = "",
    type: []const u8 = "select",
    currentValue: ?std.json.Value = null,
    options: ?std.json.Value = null,
};

pub const SessionState = struct {
    configOptions: []ConfigOption = &.{},
    availableCommands: []AvailableCommand = &.{},
};

pub const AvailableCommand = struct {
    name: []const u8 = "",
    description: []const u8 = "",
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

/// Shared, thread-safe response state.
pub const Shared = struct {
    allocator: std.mem.Allocator,
    mutex: SpinLock = .{},
    responses: ?std.AutoHashMap(u64, std.json.Value) = null,
    errors: ?std.AutoHashMap(u64, []const u8) = null,
    alive: bool = true,
    /// Incoming client-bound requests (agent → client): method + id + params.
    /// The handler thread drains these and replies.
    requests: ?std.ArrayList(Request) = null,

    pub const Request = struct {
        id: i64,
        method: []const u8 = "",
        params: ?std.json.Value = null,
    };

    pub fn init(allocator: std.mem.Allocator) Shared {
        return .{
            .allocator = allocator,
            .responses = std.AutoHashMap(u64, std.json.Value).init(allocator),
            .errors = std.AutoHashMap(u64, []const u8).init(allocator),
            .requests = std.ArrayList(Request).empty,
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
        if (self.requests) |*r| {
            for (r.items) |req| {
                self.allocator.free(req.method);
                if (req.params) |p| svc.deepFreeValue(self.allocator, p);
            }
            r.deinit(self.allocator);
        }
    }
};

pub const Connection = struct {
    allocator: std.mem.Allocator,
    agent_id: []const u8 = "",
    pid: c_int = -1,
    stdin_fd: c_int = -1,
    stdout_fd: c_int = -1,
    alive: bool = true,
    next_id: u64 = 1,
    shared: *Shared,
    agent_name: []const u8 = "",
    /// Per-ACP-session state (configOptions + availableCommands).
    sessions: std.StringHashMap(SessionState) = undefined,
    sessions_init: bool = false,
    /// Set by the reader thread on exit; deinit waits for it (prevents
    /// use-after-free when a connection is closed under a live reader).
    reader_done: std.atomic.Value(bool) = std.atomic.Value(bool).init(false),

    pub fn deinit(self: *Connection) void {
        self.close();
        var waited: u32 = 0;
        while (!self.reader_done.load(.acquire) and waited < 3000) {
            threadSleepMs(2);
            waited += 2;
        }
        self.shared.deinit();
        self.allocator.destroy(self.shared);
        self.allocator.free(self.agent_id);
        self.allocator.free(self.agent_name);
        if (self.sessions_init) {
            var it = self.sessions.iterator();
            while (it.next()) |e| {
                self.allocator.free(e.key_ptr.*);
                for (e.value_ptr.configOptions) |o| {
                    self.allocator.free(o.id);
                    self.allocator.free(o.name);
                    self.allocator.free(o.description);
                    self.allocator.free(o.category);
                    self.allocator.free(o.type);
                }
                if (e.value_ptr.configOptions.len > 0) self.allocator.free(e.value_ptr.configOptions);
                for (e.value_ptr.availableCommands) |cmd| {
                    self.allocator.free(cmd.name);
                    self.allocator.free(cmd.description);
                }
                if (e.value_ptr.availableCommands.len > 0) self.allocator.free(e.value_ptr.availableCommands);
            }
            self.sessions.deinit();
        }
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
        std.fmt.allocPrint(allocator, "{s}/.local/bin/{s}", .{ home, command }) catch "",
        std.fmt.allocPrint(allocator, "{s}/.bun/bin/{s}", .{ home, command }) catch "",
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

/// Spawns the agent process and performs the initialize handshake.
/// The returned Connection owns its memory; free with deinit().
pub fn spawn(
    allocator: std.mem.Allocator,
    agent_id: []const u8,
    command: []const u8,
    args: []const []const u8,
    env_map: *std.process.Environ.Map,
    cwd: []const u8,
) !*Connection {
    const resolved = resolveCommand(allocator, command, env_map);
    defer if (!std.mem.eql(u8, resolved, command)) allocator.free(resolved);

    const procs = try spawnProcess(allocator, resolved, args, cwd);
    const shared = try allocator.create(Shared);
    shared.* = Shared.init(allocator);
    const conn = try allocator.create(Connection);
    conn.* = .{
        .allocator = allocator,
        .agent_id = allocator.dupe(u8, agent_id) catch "",
        .pid = procs.pid,
        .stdin_fd = procs.stdin_fd,
        .stdout_fd = procs.stdout_fd,
        .shared = shared,
    };
    conn.sessions = std.StringHashMap(SessionState).init(allocator);
    conn.sessions_init = true;
    errdefer conn.deinit();

    const thread = try std.Thread.spawn(.{}, readerLoop, .{conn});
    thread.detach();

    const res = try request(conn, allocator, "initialize", .{
        .protocolVersion = ProtocolVersion,
        .clientCapabilities = .{
            .fs = .{ .readTextFile = true, .writeTextFile = true },
            .terminal = false,
            .session = .{ .configOptions = .{ .boolean = .{} } },
        },
        .clientInfo = .{ .name = "ForgeADE", .version = "0.1.0" },
    }, 20_000);
    if (res == .object) {
        if (res.object.get("protocolVersion")) |pv| {
            if (pv == .integer and pv.integer > ProtocolVersion) return error.UnsupportedProtocol;
        }
        if (res.object.get("agentInfo")) |ai| {
            if (ai == .object) {
                if (ai.object.get("name")) |nv| {
                    if (nv == .string) conn.agent_name = allocator.dupe(u8, nv.string) catch "";
                }
            }
        }
    }
    svc.deepFreeValue(allocator, res);
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

    // Agent → client request (has method + id): queue for the handler thread.
    if (obj.get("method")) |m| {
        if (m == .string) {
            if (obj.get("id")) |idv| {
                if (idv == .integer) {
                    const params = obj.get("params");
                    self.shared.mutex.lock();
                    defer self.shared.mutex.unlock();
                    if (self.shared.requests) |*reqs| {
                        // Deep-copy both the method and params: they alias
                        // this frame's parse arena, freed on return.
                        reqs.append(allocator, .{
                            .id = idv.integer,
                            .method = allocator.dupe(u8, m.string) catch "",
                            .params = if (params) |p| svc.deepCopyValue(allocator, p) else null,
                        }) catch {};
                    }
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

fn writeRaw(self: *Connection, data: []const u8) !void {
    if (!self.alive or self.stdin_fd < 0) return error.NotConnected;
    var written: usize = 0;
    while (written < data.len) {
        const n = c.write(self.stdin_fd, data[written..].ptr, data.len - written);
        if (n <= 0) return error.WriteFailed;
        written += @intCast(n);
    }
}

fn sendFrame(self: *Connection, allocator: std.mem.Allocator, value: anytype) !void {
    var out = std.Io.Writer.Allocating.init(allocator);
    defer out.deinit();
    try std.json.Stringify.value(value, .{}, &out.writer);
    try writeRaw(self, out.written());
    try writeRaw(self, "\n");
}

pub fn request(self: *Connection, allocator: std.mem.Allocator, method: []const u8, params: anytype, timeout_ms: u32) !std.json.Value {
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
    try writeRaw(self, out.written());
    try writeRaw(self, "\n");

    const deadline = svc.nowMs() + @as(i64, @intCast(timeout_ms));
    while (true) {
        self.shared.mutex.lock();
        if (self.shared.errors) |*errs| {
            if (errs.get(id)) |err| {
                const owned = self.allocator.dupe(u8, err) catch "acp request failed";
                _ = errs.remove(id);
                self.shared.mutex.unlock();
                defer self.allocator.free(owned);
                return error.AcpRequestFailed;
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

/// Replies to a pending agent → client request.
pub fn reply(self: *Connection, allocator: std.mem.Allocator, id: i64, result: anytype) !void {
    var out = std.Io.Writer.Allocating.init(allocator);
    defer out.deinit();
    try std.json.Stringify.value(.{
        .jsonrpc = "2.0",
        .id = id,
        .result = result,
    }, .{}, &out.writer);
    try writeRaw(self, out.written());
    try writeRaw(self, "\n");
}

/// Creates a new ACP session, stores its state, returns the ACP session id.
pub fn newSession(self: *Connection, allocator: std.mem.Allocator, cwd: []const u8) ![]const u8 {
    const res = try request(self, allocator, "session/new", .{
        .cwd = cwd,
        .mcpServers = @as([]struct {}, &.{}),
    }, 30_000);
    var session_id: []const u8 = "";
    var config_options: []ConfigOption = &.{};
    if (res == .object) {
        if (res.object.get("sessionId")) |sv| {
            if (sv == .string) session_id = allocator.dupe(u8, sv.string) catch "";
        }
        if (res.object.get("configOptions")) |cv| {
            if (cv == .array) {
                var list = std.ArrayList(ConfigOption).empty;
                for (cv.array.items) |item| {
                    if (item != .object) continue;
                    const o = item.object;
                    var rec = ConfigOption{};
                    if (o.get("id")) |v| {
                        if (v == .string) rec.id = allocator.dupe(u8, v.string) catch "";
                    }
                    if (o.get("name")) |v| {
                        if (v == .string) rec.name = allocator.dupe(u8, v.string) catch "";
                    }
                    if (o.get("description")) |v| {
                        if (v == .string) rec.description = allocator.dupe(u8, v.string) catch "";
                    }
                    if (o.get("category")) |v| {
                        if (v == .string) rec.category = allocator.dupe(u8, v.string) catch "";
                    }
                    if (o.get("type")) |v| {
                        if (v == .string) rec.type = allocator.dupe(u8, v.string) catch "";
                    }
                    if (o.get("currentValue")) |v| rec.currentValue = svc.deepCopyValue(allocator, v);
                    if (o.get("options")) |v| rec.options = svc.deepCopyValue(allocator, v);
                    list.append(allocator, rec) catch {};
                }
                config_options = list.toOwnedSlice(allocator) catch &.{};
            }
        }
    }
    svc.deepFreeValue(allocator, res);
    if (session_id.len == 0) return error.NoSessionId;

    self.shared.mutex.lock();
    defer self.shared.mutex.unlock();
    if (!self.sessions_init) {
        self.sessions = std.StringHashMap(SessionState).init(allocator);
        self.sessions_init = true;
    }
    self.sessions.put(allocator.dupe(u8, session_id) catch "", .{
        .configOptions = config_options,
    }) catch {};
    return session_id;
}

pub fn getState(self: *Connection, acp_session_id: []const u8) SessionState {
    if (self.sessions.get(acp_session_id)) |s| return s;
    return .{};
}

pub fn setConfigOption(self: *Connection, allocator: std.mem.Allocator, acp_session_id: []const u8, config_id: []const u8, value: std.json.Value) !SessionState {
    const params = .{
        .sessionId = acp_session_id,
        .configId = config_id,
        .value = value,
    };
    const res = try request(self, allocator, "session/set_config_option", params, 30_000);
    defer svc.deepFreeValue(allocator, res);
    if (res == .object) {
        if (res.object.get("configOptions")) |cv| {
            if (cv == .array) {
                var list = std.ArrayList(ConfigOption).empty;
                for (cv.array.items) |item| {
                    if (item != .object) continue;
                    const o = item.object;
                    var rec = ConfigOption{};
                    if (o.get("id")) |v| {
                        if (v == .string) rec.id = allocator.dupe(u8, v.string) catch "";
                    }
                    if (o.get("name")) |v| {
                        if (v == .string) rec.name = allocator.dupe(u8, v.string) catch "";
                    }
                    if (o.get("description")) |v| {
                        if (v == .string) rec.description = allocator.dupe(u8, v.string) catch "";
                    }
                    if (o.get("category")) |v| {
                        if (v == .string) rec.category = allocator.dupe(u8, v.string) catch "";
                    }
                    if (o.get("type")) |v| {
                        if (v == .string) rec.type = allocator.dupe(u8, v.string) catch "";
                    }
                    if (o.get("currentValue")) |v| rec.currentValue = svc.deepCopyValue(allocator, v);
                    if (o.get("options")) |v| rec.options = svc.deepCopyValue(allocator, v);
                    list.append(allocator, rec) catch {};
                }
                const owned: []ConfigOption = list.toOwnedSlice(allocator) catch &.{};
                if (self.sessions.getPtr(acp_session_id)) |st| {
                    // Free the previous state's owned strings before overwrite.
                    for (st.configOptions) |old| {
                        allocator.free(old.id);
                        allocator.free(old.name);
                        allocator.free(old.description);
                        allocator.free(old.category);
                        allocator.free(old.type);
                        if (old.currentValue) |cur| svc.deepFreeValue(allocator, cur);
                        if (old.options) |opts| svc.deepFreeValue(allocator, opts);
                    }
                    if (st.configOptions.len > 0) allocator.free(st.configOptions);
                    st.configOptions = owned;
                }
                return .{ .configOptions = owned };
            }
        }
    }
    return getState(self, acp_session_id);
}

/// Runs one prompt turn. Returns when the turn ends (session/update or
/// the request resolves). This is a blocking call on the worker thread.
pub fn prompt(self: *Connection, allocator: std.mem.Allocator, acp_session_id: []const u8, text: []const u8) !void {
    const res = try request(self, allocator, "session/prompt", .{
        .sessionId = acp_session_id,
        .prompt = &.{.{ .type = "text", .text = text }},
    }, 0);
    svc.deepFreeValue(allocator, res);
}

pub fn cancel(self: *Connection, allocator: std.mem.Allocator, acp_session_id: []const u8) !void {
    try notify(self, allocator, "session/cancel", .{ .sessionId = acp_session_id });
}

/// Drains and answers pending agent → client requests (fs/read_text_file,
/// fs/write_text_file, session/request_permission). Returns the count handled.
pub fn handlePendingRequests(self: *Connection, allocator: std.mem.Allocator, cwd: []const u8) usize {
    var handled: usize = 0;
    while (true) {
        self.shared.mutex.lock();
        if (self.shared.requests) |*reqs| {
            if (reqs.items.len == 0) {
                self.shared.mutex.unlock();
                break;
            }
            const req = reqs.orderedRemove(0);
            self.shared.mutex.unlock();
            handleRequest(self, allocator, req, cwd) catch {};
            // The popped request owns its method + params — free them.
            self.allocator.free(req.method);
            if (req.params) |p| svc.deepFreeValue(self.allocator, p);
            handled += 1;
        } else {
            self.shared.mutex.unlock();
            break;
        }
    }
    return handled;
}

fn handleRequest(self: *Connection, allocator: std.mem.Allocator, req: Shared.Request, cwd: []const u8) !void {
    const method = req.method;
    if (std.mem.eql(u8, method, "fs/read_text_file")) {
        var path: []const u8 = "";
        if (req.params) |p| {
            if (p == .object) {
                if (p.object.get("path")) |pv| {
                    if (pv == .string) path = pv.string;
                }
            }
        }
        const abs = resolvePath(allocator, cwd, path);
        defer allocator.free(abs);
        const content = svc.readFileBounded(allocator, abs) catch {
            try replyErr(self, allocator, req.id, "read failed");
            return;
        };
        defer allocator.free(content);
        try reply(self, allocator, req.id, .{ .content = content });
    } else if (std.mem.eql(u8, method, "fs/write_text_file")) {
        var path: []const u8 = "";
        var content: []const u8 = "";
        if (req.params) |p| {
            if (p == .object) {
                if (p.object.get("path")) |pv| {
                    if (pv == .string) path = pv.string;
                }
                if (p.object.get("content")) |cv| {
                    if (cv == .string) content = cv.string;
                }
            }
        }
        const abs = resolvePath(allocator, cwd, path);
        defer allocator.free(abs);
        svc.writeFileAtomic(allocator, abs, content) catch {
            try replyErr(self, allocator, req.id, "write failed");
            return;
        };
        try reply(self, allocator, req.id, .{});
    } else if (std.mem.eql(u8, method, "session/request_permission")) {
        // v1 policy: allow the first non-reject option once.
        var chosen: ?[]const u8 = null;
        if (req.params) |p| {
            if (p == .object) {
                if (p.object.get("options")) |ov| {
                    if (ov == .array) {
                        for (ov.array.items) |item| {
                            if (item != .object) continue;
                            var kind: []const u8 = "";
                            var option_id: []const u8 = "";
                            if (item.object.get("kind")) |kv| {
                                if (kv == .string) kind = kv.string;
                            }
                            if (item.object.get("optionId")) |iv| {
                                if (iv == .string) option_id = iv.string;
                            }
                            if (std.mem.eql(u8, kind, "allow_once") or std.mem.eql(u8, kind, "allow_always")) {
                                chosen = option_id;
                                break;
                            }
                            if (chosen == null and !std.mem.eql(u8, kind, "reject_once") and !std.mem.eql(u8, kind, "reject_always")) {
                                chosen = option_id;
                            }
                        }
                    }
                }
            }
        }
        if (chosen) |oid| {
            try reply(self, allocator, req.id, .{ .outcome = .{ .outcome = "selected", .optionId = oid } });
        } else {
            try reply(self, allocator, req.id, .{ .outcome = .{ .outcome = "cancelled" } });
        }
    } else if (std.mem.eql(u8, method, "session/set_mode")) {
        try reply(self, allocator, req.id, .{});
    } else {
        try replyErr(self, allocator, req.id, "method not supported");
    }
}

fn resolvePath(allocator: std.mem.Allocator, cwd: []const u8, path: []const u8) []const u8 {
    if (path.len == 0) return allocator.dupe(u8, cwd) catch "";
    if (path[0] == '/') return allocator.dupe(u8, path) catch "";
    return std.fmt.allocPrint(allocator, "{s}/{s}", .{ cwd, path }) catch allocator.dupe(u8, path) catch "";
}

fn replyErr(self: *Connection, allocator: std.mem.Allocator, id: i64, message: []const u8) !void {
    var out = std.Io.Writer.Allocating.init(allocator);
    defer out.deinit();
    try std.json.Stringify.value(.{
        .jsonrpc = "2.0",
        .id = id,
    }, .{}, &out.writer);
    try out.writer.writeAll(",\"error\":{\"code\":-32603,\"message\":");
    try std.json.Stringify.value(message, .{}, &out.writer);
    try out.writer.writeAll("}}");
    try writeRaw(self, out.written());
    try writeRaw(self, "\n");
}

test "resolvePath joins cwd" {
    const allocator = std.testing.allocator;
    const p = resolvePath(allocator, "/repo", "src/main.ts");
    defer allocator.free(p);
    try std.testing.expectEqualStrings("/repo/src/main.ts", p);
    const p2 = resolvePath(allocator, "/repo", "/abs/x.ts");
    defer allocator.free(p2);
    try std.testing.expectEqualStrings("/abs/x.ts", p2);
}
