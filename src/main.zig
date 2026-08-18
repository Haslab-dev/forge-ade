const std = @import("std");
const runner = @import("runner");
const native_sdk = @import("native_sdk");

pub const panic = std.debug.FullPanic(native_sdk.debug.capturePanic);

const Timespec = extern struct {
    tv_sec: isize,
    tv_nsec: isize,
};

const Stat = extern struct {
    st_dev: i32,
    st_mode: u16,
    st_nlink: u16,
    st_ino: u64,
    st_uid: u32,
    st_gid: u32,
    st_rdev: i32,
    st_atimespec: Timespec,
    st_mtimespec: Timespec,
    st_ctimespec: Timespec,
    st_birthtimespec: Timespec,
    st_size: i64,
    st_blocks: i64,
    st_blksize: i32,
    st_flags: u32,
    st_gen: u32,
    st_lspare: i32,
    st_qspare: [2]i64,
};

const c = struct {
    extern "c" fn forkpty(amaster: *c_int, name: ?[*]u8, termp: ?*const anyopaque, winp: ?*const anyopaque) c_int;
    extern "c" fn write(fd: c_int, buf: [*]const u8, len: usize) isize;
    extern "c" fn read(fd: c_int, buf: [*]u8, len: usize) isize;
    extern "c" fn open(path: [*:0]const u8, flags: c_int, ...) c_int;
    extern "c" fn close(fd: c_int) c_int;
    extern "c" fn kill(pid: c_int, sig: c_int) c_int;
    extern "c" fn waitpid(pid: c_int, status: ?*c_int, options: c_int) c_int;
    extern "c" fn ioctl(fd: c_int, request: c_ulong, ...) c_int;
    extern "c" fn execvp(file: [*:0]const u8, argv: [*:null]const ?[*:0]const u8) c_int;
    extern "c" fn chdir(path: [*:0]const u8) c_int;
    extern "c" fn getcwd(buf: [*]u8, size: usize) ?[*:0]u8;
    extern "c" fn _exit(code: c_int) noreturn;
    extern "c" fn opendir(dirname: [*:0]const u8) ?*anyopaque;
    extern "c" fn closedir(dirp: *anyopaque) c_int;
    extern "c" fn readdir(dirp: *anyopaque) ?*const Dirent;
    extern "c" fn mkdir(path: [*:0]const u8, mode: c_uint) c_int;
    extern "c" fn setenv(name: [*:0]const u8, value: [*:0]const u8, overwrite: c_int) c_int;
    extern "c" fn pipe(fildes: *[2]c_int) c_int;
    extern "c" fn fork() c_int;
    extern "c" fn dup2(oldfd: c_int, newfd: c_int) c_int;
    extern "c" fn usleep(usec: c_uint) c_int;
    extern "c" fn stat(path: [*:0]const u8, buf: *Stat) c_int;
    extern "c" fn lstat(path: [*:0]const u8, buf: *Stat) c_int;
};

const Dirent = extern struct {
    d_ino: u64,
    d_seekoff: u64,
    d_reclen: u16,
    d_namlen: u16,
    d_type: u8,
    d_name: [1024]u8,
};

const Winsize = extern struct {
    row: u16,
    col: u16,
    xpixel: u16 = 0,
    ypixel: u16 = 0,
};

const tiocswinsz: c_ulong = switch (@import("builtin").os.tag) {
    .linux => 0x5414,
    .macos => 0x80087467,
    else => 0,
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

const PtySession = struct {
    id: usize,
    master_fd: c_int,
    child_pid: c_int,
    window_id: u64,
};

const LspSession = struct {
    id: usize,
    stdin_fd: c_int,
    stdout_fd: c_int,
    child_pid: c_int,
    window_id: u64,
};

const FileInfo = struct {
    mtime_sec: isize,
    mtime_nsec: isize,
    size: i64,
};

const dev_origins = [_][]const u8{ "*", "zero://app", "zero://inline", "http://127.0.0.1:5173", "http://localhost:5173" };
const full_permissions = [_][]const u8{ "window", "filesystem", "command", "dialog", "clipboard", "network", "notifications", "credentials" };

const command_policies = [_]native_sdk.bridge.CommandPolicy{
    .{ .name = "terminal.spawn", .origins = &dev_origins },
    .{ .name = "terminal.write", .origins = &dev_origins },
    .{ .name = "terminal.resize", .origins = &dev_origins },
    .{ .name = "terminal.kill", .origins = &dev_origins },
    .{ .name = "terminal.list", .origins = &dev_origins },
    .{ .name = "fs.readDir", .origins = &dev_origins },
    .{ .name = "fs.readFile", .origins = &dev_origins },
    .{ .name = "fs.writeFile", .origins = &dev_origins },
    .{ .name = "fs.createFile", .origins = &dev_origins },
    .{ .name = "fs.createDir", .origins = &dev_origins },
    .{ .name = "fs.getCwd", .origins = &dev_origins },
    .{ .name = "fs.watch", .origins = &dev_origins },
    .{ .name = "fs.unwatch", .origins = &dev_origins },
    .{ .name = "fs.search", .origins = &dev_origins },
    .{ .name = "git.status", .origins = &dev_origins },
    .{ .name = "git.commit", .origins = &dev_origins },
    .{ .name = "git.push", .origins = &dev_origins },
    .{ .name = "git.stage", .origins = &dev_origins },
    .{ .name = "git.unstage", .origins = &dev_origins },
    .{ .name = "git.log", .origins = &dev_origins },
    .{ .name = "git.graph", .origins = &dev_origins },
    .{ .name = "git.show", .origins = &dev_origins },
    .{ .name = "git.diff", .origins = &dev_origins },
    .{ .name = "lsp.start", .origins = &dev_origins },
    .{ .name = "lsp.write", .origins = &dev_origins },
    .{ .name = "lsp.stop", .origins = &dev_origins },
    .{ .name = "command.exec", .origins = &dev_origins },
};

const App = struct {
    env_map: *std.process.Environ.Map,
    runtime: ?*native_sdk.Runtime = null,
    allocator: std.mem.Allocator,
    sessions: std.ArrayList(*PtySession),
    lsp_sessions: std.ArrayList(*LspSession),
    watched_paths: std.ArrayList([]const u8),
    known_files: std.StringHashMap(FileInfo),
    mutex: SpinLock = .{},
    next_session_id: usize = 1,
    next_lsp_id: usize = 1,
    main_window_id: u64 = 0,
    exiting: bool = false,
    watcher_started: bool = false,
    initial_scan_done: bool = false,
    handlers: [27]native_sdk.bridge.AsyncHandler = undefined,
    fn init(allocator: std.mem.Allocator, env_map: *std.process.Environ.Map) App {
        return .{
            .allocator = allocator,
            .env_map = env_map,
            .sessions = .empty,
            .lsp_sessions = .empty,
            .watched_paths = .empty,
            .known_files = std.StringHashMap(FileInfo).init(allocator),
        };
    }

    fn deinit(self: *App) void {
        self.mutex.lock();
        self.exiting = true;
        self.mutex.unlock();

        self.mutex.lock();
        defer self.mutex.unlock();

        // Teardown PTY sessions
        for (self.sessions.items) |session| {
            if (session.child_pid > 0) {
                _ = c.kill(session.child_pid, 9);
                _ = c.waitpid(session.child_pid, null, 1);
            }
            if (session.master_fd >= 0) {
                _ = c.close(session.master_fd);
                session.master_fd = -1;
            }
            self.allocator.destroy(session);
        }
        self.sessions.deinit(self.allocator);

        // Teardown LSP sessions
        for (self.lsp_sessions.items) |lsp| {
            if (lsp.child_pid > 0) {
                _ = c.kill(lsp.child_pid, 9);
                _ = c.waitpid(lsp.child_pid, null, 1);
            }
            if (lsp.stdin_fd >= 0) _ = c.close(lsp.stdin_fd);
            if (lsp.stdout_fd >= 0) _ = c.close(lsp.stdout_fd);
            self.allocator.destroy(lsp);
        }
        self.lsp_sessions.deinit(self.allocator);

        for (self.watched_paths.items) |p| {
            self.allocator.free(p);
        }
        self.watched_paths.deinit(self.allocator);

        var it = self.known_files.keyIterator();
        while (it.next()) |key| {
            self.allocator.free(key.*);
        }
        self.known_files.deinit();
    }

    fn app(self: *App) native_sdk.App {
        return .{
            .context = self,
            .name = "forge-ade-native",
            .source = native_sdk.frontend.productionSource(.{ .dist = "frontend/dist" }),
            .source_fn = source,
            .start_fn = start,
        };
    }

    fn source(context: *anyopaque) anyerror!native_sdk.WebViewSource {
        const self: *App = @ptrCast(@alignCast(context));
        return native_sdk.frontend.sourceFromEnv(self.env_map, .{
            .dist = "frontend/dist",
            .entry = "index.html",
        });
    }

    fn start(context: *anyopaque, runtime: *native_sdk.Runtime) anyerror!void {
        const self: *App = @ptrCast(@alignCast(context));
        self.runtime = runtime;

        if (!self.watcher_started) {
            self.watcher_started = true;
            const watcher_thread = try std.Thread.spawn(.{}, fsWatcherThread, .{self});
            watcher_thread.detach();
        }
    }

    fn bridge(self: *App) native_sdk.BridgeDispatcher {
        self.handlers = .{
            .{ .name = "terminal.spawn", .context = self, .invoke_fn = handleTerminalSpawn },
            .{ .name = "terminal.write", .context = self, .invoke_fn = handleTerminalWrite },
            .{ .name = "terminal.resize", .context = self, .invoke_fn = handleTerminalResize },
            .{ .name = "terminal.kill", .context = self, .invoke_fn = handleTerminalKill },
            .{ .name = "terminal.list", .context = self, .invoke_fn = handleTerminalList },
            .{ .name = "fs.readDir", .context = self, .invoke_fn = handleFsReadDir },
            .{ .name = "fs.readFile", .context = self, .invoke_fn = handleFsReadFile },
            .{ .name = "fs.writeFile", .context = self, .invoke_fn = handleFsWriteFile },
            .{ .name = "fs.createFile", .context = self, .invoke_fn = handleFsCreateFile },
            .{ .name = "fs.createDir", .context = self, .invoke_fn = handleFsCreateDir },
            .{ .name = "fs.getCwd", .context = self, .invoke_fn = handleFsGetCwd },
            .{ .name = "fs.watch", .context = self, .invoke_fn = handleFsWatch },
            .{ .name = "fs.unwatch", .context = self, .invoke_fn = handleFsUnwatch },
            .{ .name = "fs.search", .context = self, .invoke_fn = handleFsSearch },
            .{ .name = "git.status", .context = self, .invoke_fn = handleGitStatus },
            .{ .name = "git.commit", .context = self, .invoke_fn = handleGitCommit },
            .{ .name = "git.push", .context = self, .invoke_fn = handleGitPush },
            .{ .name = "git.stage", .context = self, .invoke_fn = handleGitStage },
            .{ .name = "git.unstage", .context = self, .invoke_fn = handleGitUnstage },
            .{ .name = "git.log", .context = self, .invoke_fn = handleGitLog },
            .{ .name = "git.graph", .context = self, .invoke_fn = handleGitGraph },
            .{ .name = "git.show", .context = self, .invoke_fn = handleGitShow },
            .{ .name = "git.diff", .context = self, .invoke_fn = handleGitDiff },
            .{ .name = "lsp.start", .context = self, .invoke_fn = handleLspStart },
            .{ .name = "lsp.write", .context = self, .invoke_fn = handleLspWrite },
            .{ .name = "lsp.stop", .context = self, .invoke_fn = handleLspStop },
            .{ .name = "command.exec", .context = self, .invoke_fn = handleCommandExec },
        };
        return .{
            .policy = .{
                .enabled = true,
                .permissions = &full_permissions,
                .commands = &command_policies,
            },
            .async_registry = .{ .handlers = &self.handlers },
        };
    }

    fn emitTerminalData(self: *App, window_id: u64, session_id: usize, bytes: []const u8) !void {
        const runtime = self.runtime orelse return;
        const allocator = self.allocator;

        const encoder = std.base64.standard.Encoder;
        const encoded_len = encoder.calcSize(bytes.len);
        const encoded = try allocator.alloc(u8, encoded_len);
        defer allocator.free(encoded);
        _ = encoder.encode(encoded, bytes);

        var out = std.Io.Writer.Allocating.init(allocator);
        defer out.deinit();
        try std.json.Stringify.value(.{
            .sessionId = session_id,
            .data = encoded,
        }, .{}, &out.writer);

        try runtime.options.platform.services.emitWindowEvent(window_id, "terminal.data", out.written());
    }

    fn emitLspData(self: *App, window_id: u64, lsp_id: usize, text: []const u8) !void {
        const runtime = self.runtime orelse return;
        const allocator = self.allocator;

        var out = std.Io.Writer.Allocating.init(allocator);
        defer out.deinit();
        try std.json.Stringify.value(.{
            .lspId = lsp_id,
            .data = text,
        }, .{}, &out.writer);

        try runtime.options.platform.services.emitWindowEvent(window_id, "lsp.data", out.written());
    }

    fn emitFsChange(self: *App, path: []const u8, kind: []const u8) !void {
        const runtime = self.runtime orelse return;
        const allocator = self.allocator;

        var out = std.Io.Writer.Allocating.init(allocator);
        defer out.deinit();
        try std.json.Stringify.value(.{
            .path = path,
            .kind = kind,
        }, .{}, &out.writer);

        const win_id = if (self.main_window_id != 0) self.main_window_id else 0;
        try runtime.options.platform.services.emitWindowEvent(win_id, "fs.change", out.written());
    }

    fn handleSessionExit(self: *App, session_id: usize) void {
        self.mutex.lock();
        defer self.mutex.unlock();

        if (self.exiting) return;

        for (self.sessions.items, 0..) |session, index| {
            if (session.id == session_id) {
                if (session.child_pid > 0) {
                    _ = c.waitpid(session.child_pid, null, 1);
                }
                if (session.master_fd >= 0) {
                    _ = c.close(session.master_fd);
                    session.master_fd = -1;
                }

                var out = std.Io.Writer.Allocating.init(self.allocator);
                defer out.deinit();
                std.json.Stringify.value(.{
                    .sessionId = session_id,
                }, .{}, &out.writer) catch {};

                if (self.runtime) |runtime| {
                    runtime.options.platform.services.emitWindowEvent(session.window_id, "terminal.exit", out.written()) catch {};
                }

                _ = self.sessions.swapRemove(index);
                self.allocator.destroy(session);
                break;
            }
        }
    }
};

// ============================================================================
// PTY Reader Thread
// ============================================================================

fn ptyReaderThread(self: *App, session: *PtySession) void {
    var buf: [65536]u8 = undefined;
    while (true) {
        const fd = session.master_fd;
        if (fd < 0) break;
        const n = c.read(fd, &buf, buf.len);
        if (n <= 0) break;

        self.mutex.lock();
        const is_exiting = self.exiting;
        self.mutex.unlock();
        if (is_exiting) return;

        self.emitTerminalData(session.window_id, session.id, buf[0..@intCast(n)]) catch {};
    }

    self.mutex.lock();
    const is_exiting = self.exiting;
    self.mutex.unlock();
    if (is_exiting) return;

    self.handleSessionExit(session.id);
}

// ============================================================================
// Filesystem Watcher Thread (§6)
// ============================================================================

fn scanDirRecursive(self: *App, arena_allocator: std.mem.Allocator, current_files: *std.StringHashMap(FileInfo), dir_path: []const u8, depth: usize) void {
    if (depth > 5) return;

    const dir_z = arena_allocator.dupeZ(u8, dir_path) catch return;
    const handle = c.opendir(dir_z);
    if (handle == null) return;
    defer _ = c.closedir(handle.?);

    while (c.readdir(handle.?)) |entry_ptr| {
        const name = std.mem.sliceTo(&entry_ptr.d_name, 0);
        if (std.mem.eql(u8, name, ".") or std.mem.eql(u8, name, "..") or
            std.mem.eql(u8, name, ".git") or std.mem.eql(u8, name, "node_modules") or
            std.mem.eql(u8, name, "zig-out") or std.mem.eql(u8, name, "dist") or
            std.mem.eql(u8, name, ".native") or std.mem.eql(u8, name, ".DS_Store"))
        {
            continue;
        }

        const full_path = std.fs.path.join(arena_allocator, &.{ dir_path, name }) catch continue;
        if (entry_ptr.d_type == 4) {
            scanDirRecursive(self, arena_allocator, current_files, full_path, depth + 1);
        } else {
            const full_path_z = arena_allocator.dupeZ(u8, full_path) catch continue;
            var stat_buf: Stat = undefined;
            if (c.stat(full_path_z, &stat_buf) == 0) {
                const info = FileInfo{
                    .mtime_sec = stat_buf.st_mtimespec.tv_sec,
                    .mtime_nsec = stat_buf.st_mtimespec.tv_nsec,
                    .size = stat_buf.st_size,
                };
                current_files.put(full_path, info) catch {};

                if (self.initial_scan_done) {
                    if (self.known_files.get(full_path)) |old_info| {
                        if (old_info.mtime_sec != info.mtime_sec or
                            old_info.mtime_nsec != info.mtime_nsec or
                            old_info.size != info.size)
                        {
                            self.emitFsChange(full_path, "modify") catch {};
                        }
                    } else {
                        self.emitFsChange(full_path, "create") catch {};
                    }
                }
            }
        }
    }
}

fn fsWatcherThread(self: *App) void {
    while (true) {
        _ = c.usleep(400_000); // 400ms polling for responsive file discovery

        self.mutex.lock();
        const is_exiting = self.exiting;
        self.mutex.unlock();
        if (is_exiting) break;

        self.mutex.lock();
        const watched_len = self.watched_paths.items.len;
        self.mutex.unlock();

        if (watched_len == 0) continue;

        var scan_arena = std.heap.ArenaAllocator.init(self.allocator);
        const scan_allocator = scan_arena.allocator();

        var current_files = std.StringHashMap(FileInfo).init(scan_allocator);

        self.mutex.lock();
        for (self.watched_paths.items) |root_dir| {
            scanDirRecursive(self, scan_allocator, &current_files, root_dir, 0);
        }

        // Check for deletions if initial scan is already complete
        if (self.initial_scan_done) {
            var it = self.known_files.keyIterator();
            while (it.next()) |known_path_ptr| {
                const known_path = known_path_ptr.*;
                if (!current_files.contains(known_path)) {
                    self.emitFsChange(known_path, "delete") catch {};
                }
            }
        }

        // Replace known_files with new scan snapshot
        var old_it = self.known_files.keyIterator();
        while (old_it.next()) |key| {
            self.allocator.free(key.*);
        }
        self.known_files.clearRetainingCapacity();

        var curr_it = current_files.iterator();
        while (curr_it.next()) |entry| {
            const path_dupe = self.allocator.dupe(u8, entry.key_ptr.*) catch continue;
            self.known_files.put(path_dupe, entry.value_ptr.*) catch {};
        }

        self.initial_scan_done = true;
        self.mutex.unlock();

        scan_arena.deinit();
    }
}

// ============================================================================
// LSP Reader Thread (§19–§23)
// ============================================================================

fn lspReaderThread(self: *App, lsp: *LspSession) void {
    var buffer: [16384]u8 = undefined;
    while (true) {
        const fd = lsp.stdout_fd;
        if (fd < 0) break;
        const n = c.read(fd, &buffer, buffer.len);
        if (n <= 0) break;

        self.mutex.lock();
        const is_exiting = self.exiting;
        self.mutex.unlock();
        if (is_exiting) return;

        self.emitLspData(lsp.window_id, lsp.id, buffer[0..@intCast(n)]) catch {};
    }
}

// ============================================================================
// Terminal Bridge Handlers
// ============================================================================

fn handleTerminalSpawn(context: *anyopaque, invocation: native_sdk.bridge.Invocation, responder: native_sdk.bridge.AsyncResponder) anyerror!void {
    const self: *App = @ptrCast(@alignCast(context));
    self.main_window_id = invocation.source.window_id;

    const Payload = struct {
        cwd: []const u8 = "",
        cols: ?u16 = null,
        rows: ?u16 = null,
    };

    const parsed = std.json.parseFromSlice(Payload, self.allocator, invocation.request.payload, .{ .ignore_unknown_fields = true }) catch |err| {
        try responder.fail(invocation.request.id, .invalid_request, @errorName(err));
        return;
    };
    defer parsed.deinit();

    const cols = parsed.value.cols orelse 80;
    const rows = parsed.value.rows orelse 24;

    var arena = std.heap.ArenaAllocator.init(self.allocator);
    defer arena.deinit();
    const arena_allocator = arena.allocator();

    const shell = self.env_map.get("SHELL") orelse "/bin/zsh";
    const shell_z = try arena_allocator.dupeZ(u8, shell);
    const arg0_z = try arena_allocator.dupeZ(u8, std.fs.path.basename(shell));
    const arg1_z = try arena_allocator.dupeZ(u8, "-l");
    const child_argv = [_:null]?[*:0]const u8{ arg0_z.ptr, arg1_z.ptr };

    var ws = Winsize{ .row = rows, .col = cols };
    var master_fd: c_int = -1;

    const pid = c.forkpty(&master_fd, null, null, &ws);
    if (pid < 0) {
        try responder.fail(invocation.request.id, .handler_failed, "forkpty failed");
        return;
    }

    if (pid == 0) {
        // Essential environment setup for correct terminal backspace and ANSI colors
        _ = c.setenv("TERM", "xterm-256color", 1);
        _ = c.setenv("COLORTERM", "truecolor", 1);
        _ = c.setenv("LANG", "en_US.UTF-8", 1);
        _ = c.setenv("LC_ALL", "en_US.UTF-8", 1);

        if (parsed.value.cwd.len > 0) {
            const cwd_z = arena_allocator.dupeZ(u8, parsed.value.cwd) catch c._exit(127);
            _ = c.chdir(cwd_z.ptr);
        }

        _ = c.execvp(shell_z.ptr, &child_argv);
        c._exit(127);
    }

    self.mutex.lock();
    defer self.mutex.unlock();

    const session = try self.allocator.create(PtySession);
    session.* = .{
        .id = self.next_session_id,
        .master_fd = master_fd,
        .child_pid = pid,
        .window_id = invocation.source.window_id,
    };
    self.next_session_id += 1;
    try self.sessions.append(self.allocator, session);

    const thread = try std.Thread.spawn(.{}, ptyReaderThread, .{ self, session });
    thread.detach();

    var response_buf: [256]u8 = undefined;
    const response = try std.fmt.bufPrint(&response_buf, "{{\"sessionId\":{d},\"pid\":{d}}}", .{ session.id, pid });
    try responder.success(invocation.request.id, response);
}

fn handleTerminalList(context: *anyopaque, invocation: native_sdk.bridge.Invocation, responder: native_sdk.bridge.AsyncResponder) anyerror!void {
    const self: *App = @ptrCast(@alignCast(context));
    self.main_window_id = invocation.source.window_id;

    const SessionInfo = struct {
        sessionId: usize,
        pid: c_int,
    };

    var arena = std.heap.ArenaAllocator.init(self.allocator);
    defer arena.deinit();
    const arena_allocator = arena.allocator();

    var list: std.ArrayList(SessionInfo) = .empty;
    defer list.deinit(arena_allocator);

    self.mutex.lock();
    for (self.sessions.items) |s| {
        try list.append(arena_allocator, .{
            .sessionId = s.id,
            .pid = s.child_pid,
        });
    }
    self.mutex.unlock();

    var out = std.Io.Writer.Allocating.init(self.allocator);
    defer out.deinit();

    try std.json.Stringify.value(.{
        .sessions = list.items,
    }, .{}, &out.writer);

    try responder.success(invocation.request.id, out.written());
}

fn handleTerminalWrite(context: *anyopaque, invocation: native_sdk.bridge.Invocation, responder: native_sdk.bridge.AsyncResponder) anyerror!void {
    const self: *App = @ptrCast(@alignCast(context));
    self.main_window_id = invocation.source.window_id;

    const Payload = struct {
        sessionId: usize,
        data: []const u8,
    };

    const parsed = std.json.parseFromSlice(Payload, self.allocator, invocation.request.payload, .{ .ignore_unknown_fields = true }) catch |err| {
        try responder.fail(invocation.request.id, .invalid_request, @errorName(err));
        return;
    };
    defer parsed.deinit();

    self.mutex.lock();
    const session_opt = for (self.sessions.items) |s| {
        if (s.id == parsed.value.sessionId) break s;
    } else null;
    self.mutex.unlock();

    const session = session_opt orelse {
        try responder.fail(invocation.request.id, .invalid_request, "Session not found");
        return;
    };

    if (session.master_fd < 0) {
        try responder.fail(invocation.request.id, .invalid_request, "Session closed");
        return;
    }

    const decoder = std.base64.standard.Decoder;
    const decoded_len = try decoder.calcSizeForSlice(parsed.value.data);
    const decoded = try self.allocator.alloc(u8, decoded_len);
    defer self.allocator.free(decoded);

    try decoder.decode(decoded, parsed.value.data);

    var total_written: usize = 0;
    while (total_written < decoded.len) {
        if (session.master_fd < 0) break;
        const n = c.write(session.master_fd, decoded[total_written..].ptr, decoded.len - total_written);
        if (n < 0) {
            try responder.fail(invocation.request.id, .handler_failed, "write failed");
            return;
        }
        total_written += @intCast(n);
    }

    try responder.success(invocation.request.id, "{\"ok\":true}");
}

fn handleTerminalResize(context: *anyopaque, invocation: native_sdk.bridge.Invocation, responder: native_sdk.bridge.AsyncResponder) anyerror!void {
    const self: *App = @ptrCast(@alignCast(context));
    self.main_window_id = invocation.source.window_id;

    const Payload = struct {
        sessionId: usize,
        cols: u16,
        rows: u16,
    };

    const parsed = std.json.parseFromSlice(Payload, self.allocator, invocation.request.payload, .{ .ignore_unknown_fields = true }) catch |err| {
        try responder.fail(invocation.request.id, .invalid_request, @errorName(err));
        return;
    };
    defer parsed.deinit();

    self.mutex.lock();
    const session_opt = for (self.sessions.items) |s| {
        if (s.id == parsed.value.sessionId) break s;
    } else null;
    self.mutex.unlock();

    const session = session_opt orelse {
        try responder.fail(invocation.request.id, .invalid_request, "Session not found");
        return;
    };

    if (session.master_fd < 0) {
        try responder.success(invocation.request.id, "{\"ok\":true}");
        return;
    }

    var ws = Winsize{
        .row = if (parsed.value.rows == 0) 1 else parsed.value.rows,
        .col = if (parsed.value.cols == 0) 1 else parsed.value.cols,
    };
    _ = c.ioctl(session.master_fd, tiocswinsz, &ws);

    try responder.success(invocation.request.id, "{\"ok\":true}");
}

fn handleTerminalKill(context: *anyopaque, invocation: native_sdk.bridge.Invocation, responder: native_sdk.bridge.AsyncResponder) anyerror!void {
    const self: *App = @ptrCast(@alignCast(context));
    self.main_window_id = invocation.source.window_id;

    const Payload = struct {
        sessionId: usize,
    };

    const parsed = std.json.parseFromSlice(Payload, self.allocator, invocation.request.payload, .{ .ignore_unknown_fields = true }) catch |err| {
        try responder.fail(invocation.request.id, .invalid_request, @errorName(err));
        return;
    };
    defer parsed.deinit();

    self.mutex.lock();
    defer self.mutex.unlock();

    for (self.sessions.items, 0..) |session, index| {
        if (session.id == parsed.value.sessionId) {
            if (session.child_pid > 0) {
                _ = c.kill(session.child_pid, 9);
                _ = c.waitpid(session.child_pid, null, 1);
            }
            if (session.master_fd >= 0) {
                _ = c.close(session.master_fd);
                session.master_fd = -1;
            }
            _ = self.sessions.swapRemove(index);
            self.allocator.destroy(session);
            break;
        }
    }

    try responder.success(invocation.request.id, "{\"ok\":true}");
}

// ============================================================================
// Filesystem Watcher Handlers
// ============================================================================

fn handleFsWatch(context: *anyopaque, invocation: native_sdk.bridge.Invocation, responder: native_sdk.bridge.AsyncResponder) anyerror!void {
    const self: *App = @ptrCast(@alignCast(context));
    self.main_window_id = invocation.source.window_id;

    const Payload = struct {
        path: []const u8,
    };

    const parsed = std.json.parseFromSlice(Payload, self.allocator, invocation.request.payload, .{ .ignore_unknown_fields = true }) catch |err| {
        try responder.fail(invocation.request.id, .invalid_request, @errorName(err));
        return;
    };
    defer parsed.deinit();

    self.mutex.lock();
    defer self.mutex.unlock();

    const already_watched = for (self.watched_paths.items) |p| {
        if (std.mem.eql(u8, p, parsed.value.path)) break true;
    } else false;

    if (!already_watched) {
        const path_copy = try self.allocator.dupe(u8, parsed.value.path);
        try self.watched_paths.append(self.allocator, path_copy);
    }

    try responder.success(invocation.request.id, "{\"ok\":true}");
}

fn handleFsUnwatch(context: *anyopaque, invocation: native_sdk.bridge.Invocation, responder: native_sdk.bridge.AsyncResponder) anyerror!void {
    const self: *App = @ptrCast(@alignCast(context));
    self.main_window_id = invocation.source.window_id;

    const Payload = struct {
        path: []const u8,
    };

    const parsed = std.json.parseFromSlice(Payload, self.allocator, invocation.request.payload, .{ .ignore_unknown_fields = true }) catch |err| {
        try responder.fail(invocation.request.id, .invalid_request, @errorName(err));
        return;
    };
    defer parsed.deinit();

    self.mutex.lock();
    defer self.mutex.unlock();

    for (self.watched_paths.items, 0..) |p, index| {
        if (std.mem.eql(u8, p, parsed.value.path)) {
            self.allocator.free(p);
            _ = self.watched_paths.swapRemove(index);
            break;
        }
    }

    try responder.success(invocation.request.id, "{\"ok\":true}");
}

// ============================================================================
// Filesystem Search (§30)
// ============================================================================

const SearchMatch = struct {
    path: []const u8,
    line: usize,
    column: usize,
    preview: []const u8,
};

fn searchInFile(arena_allocator: std.mem.Allocator, matches: *std.ArrayList(SearchMatch), file_path: []const u8, query: []const u8, case_sensitive: bool, max_matches: usize) void {
    if (matches.items.len >= max_matches) return;

    const path_z = arena_allocator.dupeZ(u8, file_path) catch return;

    var stat_buf: Stat = undefined;
    if (c.lstat(path_z, &stat_buf) != 0) return;

    // Only regular files
    if ((stat_buf.st_mode & 0o170000) != 0o100000) return;

    // Skip files > 1MB
    if (stat_buf.st_size > 1024 * 1024) return;

    // Open non-blocking
    const fd = c.open(path_z, 0x0004);
    if (fd < 0) return;
    defer _ = c.close(fd);

    var content: std.ArrayList(u8) = .empty;
    var buf: [8192]u8 = undefined;
    while (true) {
        const n = c.read(fd, &buf, buf.len);
        if (n <= 0) break;
        content.appendSlice(arena_allocator, buf[0..@intCast(n)]) catch break;
        if (content.items.len > 1024 * 1024) break;
    }

    var line_iter = std.mem.splitScalar(u8, content.items, '\n');
    var line_num: usize = 1;

    while (line_iter.next()) |line_text| {
        if (matches.items.len >= max_matches) break;

        var col_idx: ?usize = null;
        if (case_sensitive) {
            col_idx = std.mem.indexOf(u8, line_text, query);
        } else {
            var lower_line = arena_allocator.alloc(u8, line_text.len) catch break;
            for (line_text, 0..) |ch, i| {
                lower_line[i] = std.ascii.toLower(ch);
            }
            var lower_query = arena_allocator.alloc(u8, query.len) catch break;
            for (query, 0..) |ch, i| {
                lower_query[i] = std.ascii.toLower(ch);
            }
            col_idx = std.mem.indexOf(u8, lower_line, lower_query);
        }

        if (col_idx) |c_idx| {
            const preview_text = if (line_text.len > 120) line_text[0..120] else line_text;
            const trimmed = std.mem.trim(u8, preview_text, "\r\t");
            matches.append(arena_allocator, .{
                .path = arena_allocator.dupe(u8, file_path) catch continue,
                .line = line_num,
                .column = c_idx + 1,
                .preview = arena_allocator.dupe(u8, trimmed) catch continue,
            }) catch break;
        }

        line_num += 1;
    }
}

fn searchDirRecursive(self: *App, arena_allocator: std.mem.Allocator, matches: *std.ArrayList(SearchMatch), dir_path: []const u8, query: []const u8, case_sensitive: bool, max_matches: usize, depth: usize) void {
    if (depth > 5 or matches.items.len >= max_matches) return;

    const dir_z = arena_allocator.dupeZ(u8, dir_path) catch return;
    const handle = c.opendir(dir_z);
    if (handle == null) return;
    defer _ = c.closedir(handle.?);

    while (c.readdir(handle.?)) |entry_ptr| {
        if (matches.items.len >= max_matches) break;
        const name = std.mem.sliceTo(&entry_ptr.d_name, 0);
        if (std.mem.eql(u8, name, ".") or std.mem.eql(u8, name, "..") or
            std.mem.eql(u8, name, ".git") or std.mem.eql(u8, name, "node_modules") or
            std.mem.eql(u8, name, "zig-out") or std.mem.eql(u8, name, "dist") or
            std.mem.eql(u8, name, ".native") or std.mem.eql(u8, name, ".DS_Store"))
        {
            continue;
        }

        const full_path = std.fs.path.join(arena_allocator, &.{ dir_path, name }) catch continue;
        if (entry_ptr.d_type == 4) {
            searchDirRecursive(self, arena_allocator, matches, full_path, query, case_sensitive, max_matches, depth + 1);
        } else {
            searchInFile(arena_allocator, matches, full_path, query, case_sensitive, max_matches);
        }
    }
}

fn handleFsSearch(context: *anyopaque, invocation: native_sdk.bridge.Invocation, responder: native_sdk.bridge.AsyncResponder) anyerror!void {
    const self: *App = @ptrCast(@alignCast(context));
    self.main_window_id = invocation.source.window_id;

    const Payload = struct {
        query: []const u8,
        roots: ?[][]const u8 = null,
        caseSensitive: bool = false,
        maxResults: ?usize = null,
    };

    const parsed = std.json.parseFromSlice(Payload, self.allocator, invocation.request.payload, .{ .ignore_unknown_fields = true }) catch |err| {
        try responder.fail(invocation.request.id, .invalid_request, @errorName(err));
        return;
    };
    defer parsed.deinit();

    if (parsed.value.query.len == 0) {
        try responder.success(invocation.request.id, "{\"matches\":[],\"totalMatches\":0}");
        return;
    }

    var arena = std.heap.ArenaAllocator.init(self.allocator);
    defer arena.deinit();
    const arena_allocator = arena.allocator();

    var matches: std.ArrayList(SearchMatch) = .empty;
    defer matches.deinit(arena_allocator);

    const max_matches = parsed.value.maxResults orelse 200;

    var roots_copy: std.ArrayList([]const u8) = .empty;
    defer roots_copy.deinit(arena_allocator);

    self.mutex.lock();
    if (parsed.value.roots) |r| {
        for (r) |path| {
            roots_copy.append(arena_allocator, arena_allocator.dupe(u8, path) catch continue) catch {};
        }
    } else {
        for (self.watched_paths.items) |path| {
            roots_copy.append(arena_allocator, arena_allocator.dupe(u8, path) catch continue) catch {};
        }
    }
    self.mutex.unlock();

    for (roots_copy.items) |root_dir| {
        if (matches.items.len >= max_matches) break;
        searchDirRecursive(self, arena_allocator, &matches, root_dir, parsed.value.query, parsed.value.caseSensitive, max_matches, 0);
    }

    var out = std.Io.Writer.Allocating.init(self.allocator);
    defer out.deinit();

    try std.json.Stringify.value(.{
        .matches = matches.items,
        .totalMatches = matches.items.len,
    }, .{}, &out.writer);

    try responder.success(invocation.request.id, out.written());
}

// ============================================================================
// Git Integration (§33)
// ============================================================================

fn runGitCommand(allocator: std.mem.Allocator, cwd: []const u8, args: []const ?[*:0]const u8) ![]u8 {
    var out_pipe: [2]c_int = undefined;
    if (c.pipe(&out_pipe) != 0) return error.PipeFailed;

    const pid = c.fork();
    if (pid < 0) {
        _ = c.close(out_pipe[0]);
        _ = c.close(out_pipe[1]);
        return error.ForkFailed;
    }

    if (pid == 0) {
        _ = c.close(out_pipe[0]);
        _ = c.dup2(out_pipe[1], 1);
        _ = c.dup2(out_pipe[1], 2);
        _ = c.close(out_pipe[1]);

        if (cwd.len > 0) {
            var arena = std.heap.ArenaAllocator.init(allocator);
            const arena_allocator = arena.allocator();
            const cwd_z = arena_allocator.dupeZ(u8, cwd) catch c._exit(127);
            _ = c.chdir(cwd_z.ptr);
        }

        const bin = args[0] orelse c._exit(127);
        _ = c.execvp(bin, @ptrCast(args.ptr));
        c._exit(127);
    }
    _ = c.close(out_pipe[1]);

    var output: std.ArrayList(u8) = .empty;
    var buf: [4096]u8 = undefined;
    while (true) {
        const n = c.read(out_pipe[0], &buf, buf.len);
        if (n <= 0) break;
        try output.appendSlice(allocator, buf[0..@intCast(n)]);
    }
    _ = c.close(out_pipe[0]);
    _ = c.waitpid(pid, null, 0);

    return output.toOwnedSlice(allocator);
}

fn handleGitStatus(context: *anyopaque, invocation: native_sdk.bridge.Invocation, responder: native_sdk.bridge.AsyncResponder) anyerror!void {
    const self: *App = @ptrCast(@alignCast(context));
    self.main_window_id = invocation.source.window_id;

    const Payload = struct {
        cwd: []const u8 = "",
    };

    const parsed = std.json.parseFromSlice(Payload, self.allocator, invocation.request.payload, .{ .ignore_unknown_fields = true }) catch |err| {
        try responder.fail(invocation.request.id, .invalid_request, @errorName(err));
        return;
    };
    defer parsed.deinit();

    var arena = std.heap.ArenaAllocator.init(self.allocator);
    defer arena.deinit();
    const arena_allocator = arena.allocator();

    const git_z = try arena_allocator.dupeZ(u8, "git");
    const status_z = try arena_allocator.dupeZ(u8, "status");
    const porcelain_z = try arena_allocator.dupeZ(u8, "--porcelain=v1");
    const branch_z = try arena_allocator.dupeZ(u8, "-b");
    const output = runGitCommand(arena_allocator, parsed.value.cwd, &.{ git_z.ptr, status_z.ptr, porcelain_z.ptr, branch_z.ptr, null }) catch "";
    var branch: []const u8 = "main";
    var modified: std.ArrayList([]const u8) = .empty;
    var untracked: std.ArrayList([]const u8) = .empty;
    var added: std.ArrayList([]const u8) = .empty;
    var deleted: std.ArrayList([]const u8) = .empty;

    var lines = std.mem.splitScalar(u8, output, '\n');
    var is_first = true;
    while (lines.next()) |line| {
        const trimmed = std.mem.trim(u8, line, " \r\t");
        if (trimmed.len == 0) continue;

        if (is_first and std.mem.startsWith(u8, trimmed, "## ")) {
            is_first = false;
            const branch_part = trimmed[3..];
            if (std.mem.indexOf(u8, branch_part, "...")) |dots_idx| {
                branch = branch_part[0..dots_idx];
            } else if (std.mem.indexOf(u8, branch_part, " ")) |space_idx| {
                branch = branch_part[0..space_idx];
            } else {
                branch = branch_part;
            }
            continue;
        }
        is_first = false;

        if (trimmed.len >= 3) {
            const index_status = trimmed[0];
            const work_status = trimmed[1];
            const file_path = std.mem.trim(u8, trimmed[2..], " \r\t");

            if (index_status == '?' and work_status == '?') {
                try untracked.append(arena_allocator, file_path);
            } else {
                // Handle rename/copy score prefix and " -> " separator: take destination path for display
                var path_to_store = file_path;
                if (index_status == 'R' or index_status == 'C') {
                    if (std.mem.indexOf(u8, file_path, " -> ")) |arrow| {
                        path_to_store = std.mem.trim(u8, file_path[arrow + 4 ..], " \\r\\t");
                    } else {
                        var i: usize = 0;
                        while (i < path_to_store.len and (path_to_store[i] == ' ' or (path_to_store[i] >= '0' and path_to_store[i] <= '9'))) : (i += 1) {}
                        path_to_store = std.mem.trim(u8, path_to_store[i..], " \\r\\t");
                        if (path_to_store.len == 0) path_to_store = file_path;
                    }
                }
                // Index (staged) states -> bucket `added` (panel's "Staged")
                if (index_status == 'M' or index_status == 'A' or index_status == 'D' or index_status == 'R' or index_status == 'C') {
                    try added.append(arena_allocator, path_to_store);
                }
                // Worktree (unstaged) states -> buckets `modified` / `deleted`
                if (work_status == 'M') {
                    try modified.append(arena_allocator, path_to_store);
                } else if (work_status == 'D') {
                    try deleted.append(arena_allocator, path_to_store);
                }
            }
        }
    }

    var out = std.Io.Writer.Allocating.init(self.allocator);
    defer out.deinit();

    try std.json.Stringify.value(.{
        .branch = branch,
        .isClean = (modified.items.len == 0 and untracked.items.len == 0 and added.items.len == 0 and deleted.items.len == 0),
        .modified = modified.items,
        .untracked = untracked.items,
        .added = added.items,
        .deleted = deleted.items,
    }, .{}, &out.writer);

    try responder.success(invocation.request.id, out.written());
}

fn handleGitCommit(context: *anyopaque, invocation: native_sdk.bridge.Invocation, responder: native_sdk.bridge.AsyncResponder) anyerror!void {
    const self: *App = @ptrCast(@alignCast(context));
    self.main_window_id = invocation.source.window_id;

    const Payload = struct {
        cwd: []const u8 = "",
        message: []const u8,
    };

    const parsed = std.json.parseFromSlice(Payload, self.allocator, invocation.request.payload, .{ .ignore_unknown_fields = true }) catch |err| {
        try responder.fail(invocation.request.id, .invalid_request, @errorName(err));
        return;
    };
    defer parsed.deinit();

    var arena = std.heap.ArenaAllocator.init(self.allocator);
    defer arena.deinit();
    const arena_allocator = arena.allocator();

    const git_z = try arena_allocator.dupeZ(u8, "git");
    const commit_z = try arena_allocator.dupeZ(u8, "commit");
    const m_z = try arena_allocator.dupeZ(u8, "-m");
    const msg_z = try arena_allocator.dupeZ(u8, parsed.value.message);

    _ = runGitCommand(arena_allocator, parsed.value.cwd, &.{ git_z.ptr, commit_z.ptr, m_z.ptr, msg_z.ptr, null }) catch |err| {
        try responder.fail(invocation.request.id, .handler_failed, @errorName(err));
        return;
    };

    try responder.success(invocation.request.id, "{\"ok\":true}");
}

fn handleGitPush(context: *anyopaque, invocation: native_sdk.bridge.Invocation, responder: native_sdk.bridge.AsyncResponder) anyerror!void {
    const self: *App = @ptrCast(@alignCast(context));
    self.main_window_id = invocation.source.window_id;

    const Payload = struct {
        cwd: []const u8 = "",
    };

    const parsed = std.json.parseFromSlice(Payload, self.allocator, invocation.request.payload, .{ .ignore_unknown_fields = true }) catch |err| {
        try responder.fail(invocation.request.id, .invalid_request, @errorName(err));
        return;
    };
    defer parsed.deinit();

    var arena = std.heap.ArenaAllocator.init(self.allocator);
    defer arena.deinit();
    const arena_allocator = arena.allocator();

    const git_z = try arena_allocator.dupeZ(u8, "git");
    const push_z = try arena_allocator.dupeZ(u8, "push");

    _ = runGitCommand(arena_allocator, parsed.value.cwd, &.{ git_z.ptr, push_z.ptr, null }) catch |err| {
        try responder.fail(invocation.request.id, .handler_failed, @errorName(err));
        return;
    };

    try responder.success(invocation.request.id, "{\"ok\":true}");
}

fn handleGitStage(context: *anyopaque, invocation: native_sdk.bridge.Invocation, responder: native_sdk.bridge.AsyncResponder) anyerror!void {
    const self: *App = @ptrCast(@alignCast(context));
    self.main_window_id = invocation.source.window_id;

    const Payload = struct {
        cwd: []const u8 = "",
        path: []const u8,
    };

    const parsed = std.json.parseFromSlice(Payload, self.allocator, invocation.request.payload, .{ .ignore_unknown_fields = true }) catch |err| {
        try responder.fail(invocation.request.id, .invalid_request, @errorName(err));
        return;
    };
    defer parsed.deinit();

    var arena = std.heap.ArenaAllocator.init(self.allocator);
    defer arena.deinit();
    const arena_allocator = arena.allocator();

    const git_z = try arena_allocator.dupeZ(u8, "git");
    const add_z = try arena_allocator.dupeZ(u8, "add");
    const path_z = try arena_allocator.dupeZ(u8, parsed.value.path);

    _ = runGitCommand(arena_allocator, parsed.value.cwd, &.{ git_z.ptr, add_z.ptr, path_z.ptr, null }) catch |err| {
        try responder.fail(invocation.request.id, .handler_failed, @errorName(err));
        return;
    };

    try responder.success(invocation.request.id, "{\"ok\":true}");
}

fn handleGitUnstage(context: *anyopaque, invocation: native_sdk.bridge.Invocation, responder: native_sdk.bridge.AsyncResponder) anyerror!void {
    const self: *App = @ptrCast(@alignCast(context));
    self.main_window_id = invocation.source.window_id;

    const Payload = struct {
        cwd: []const u8 = "",
        path: []const u8,
    };

    const parsed = std.json.parseFromSlice(Payload, self.allocator, invocation.request.payload, .{ .ignore_unknown_fields = true }) catch |err| {
        try responder.fail(invocation.request.id, .invalid_request, @errorName(err));
        return;
    };
    defer parsed.deinit();

    var arena = std.heap.ArenaAllocator.init(self.allocator);
    defer arena.deinit();
    const arena_allocator = arena.allocator();

    const git_z = try arena_allocator.dupeZ(u8, "git");
    const reset_z = try arena_allocator.dupeZ(u8, "restore");
    const staged_z = try arena_allocator.dupeZ(u8, "--staged");
    const path_z = try arena_allocator.dupeZ(u8, parsed.value.path);

    _ = runGitCommand(arena_allocator, parsed.value.cwd, &.{ git_z.ptr, reset_z.ptr, staged_z.ptr, path_z.ptr, null }) catch |err| {
        try responder.fail(invocation.request.id, .handler_failed, @errorName(err));
        return;
    };

    try responder.success(invocation.request.id, "{\"ok\":true}");
}

fn handleGitLog(context: *anyopaque, invocation: native_sdk.bridge.Invocation, responder: native_sdk.bridge.AsyncResponder) anyerror!void {
    const self: *App = @ptrCast(@alignCast(context));
    self.main_window_id = invocation.source.window_id;

    const Payload = struct {
        cwd: []const u8 = "",
        limit: ?usize = null,
        skip: ?usize = null,
    };

    const parsed = std.json.parseFromSlice(Payload, self.allocator, invocation.request.payload, .{ .ignore_unknown_fields = true }) catch |err| {
        try responder.fail(invocation.request.id, .invalid_request, @errorName(err));
        return;
    };
    defer parsed.deinit();

    var arena = std.heap.ArenaAllocator.init(self.allocator);
    defer arena.deinit();
    const arena_allocator = arena.allocator();

    const limit = parsed.value.limit orelse 20;
    const limit_str = try std.fmt.allocPrint(arena_allocator, "-n{d}", .{limit});
    const skip = parsed.value.skip orelse 0;
    const skip_str = try std.fmt.allocPrint(arena_allocator, "--skip={d}", .{skip});

    const git_z = try arena_allocator.dupeZ(u8, "git");
    const log_z = try arena_allocator.dupeZ(u8, "log");
    const date_z = try arena_allocator.dupeZ(u8, "--date=relative");
    const format_z = try arena_allocator.dupeZ(u8, "--pretty=format:%H\x1f%h\x1f%an\x1f%ad\x1f%s");
    const limit_z = try arena_allocator.dupeZ(u8, limit_str);
    const skip_z = try arena_allocator.dupeZ(u8, skip_str);

    const output = runGitCommand(arena_allocator, parsed.value.cwd, &.{ git_z.ptr, log_z.ptr, date_z.ptr, format_z.ptr, skip_z.ptr, limit_z.ptr, null }) catch "";
    const CommitItem = struct {
        hash: []const u8,
        shortHash: []const u8,
        author: []const u8,
        date: []const u8,
        message: []const u8,
    };

    var commits: std.ArrayList(CommitItem) = .empty;
    defer commits.deinit(arena_allocator);

    var lines = std.mem.splitScalar(u8, output, '\n');
    while (lines.next()) |line| {
        const trimmed = std.mem.trim(u8, line, " \r\t");
        if (trimmed.len < 8) continue;
        var parts = std.mem.splitScalar(u8, trimmed, '\x1f');
        const hash = parts.next() orelse continue;
        const short_hash = parts.next() orelse (if (hash.len > 7) hash[0..7] else hash);
        const author = parts.next() orelse "";
        const date = parts.next() orelse "";
        const msg = parts.next() orelse "";
        try commits.append(arena_allocator, .{
            .hash = hash,
            .shortHash = short_hash,
            .author = author,
            .date = date,
            .message = msg,
        });
    }

    var out = std.Io.Writer.Allocating.init(self.allocator);
    defer out.deinit();

    try std.json.Stringify.value(.{
        .commits = commits.items,
    }, .{}, &out.writer);

    try responder.success(invocation.request.id, out.written());
}
fn handleGitGraph(context: *anyopaque, invocation: native_sdk.bridge.Invocation, responder: native_sdk.bridge.AsyncResponder) anyerror!void {
    const self: *App = @ptrCast(@alignCast(context));
    self.main_window_id = invocation.source.window_id;

    const Payload = struct {
        cwd: []const u8 = "",
        limit: ?usize = null,
        skip: ?usize = null,
    };

    const parsed = std.json.parseFromSlice(Payload, self.allocator, invocation.request.payload, .{ .ignore_unknown_fields = true }) catch |err| {
        try responder.fail(invocation.request.id, .invalid_request, @errorName(err));
        return;
    };
    defer parsed.deinit();

    var arena = std.heap.ArenaAllocator.init(self.allocator);
    defer arena.deinit();
    const arena_allocator = arena.allocator();

    const limit = parsed.value.limit orelse 20;
    const limit_str = try std.fmt.allocPrint(arena_allocator, "-n{d}", .{limit});
    const skip = parsed.value.skip orelse 0;
    const skip_str = try std.fmt.allocPrint(arena_allocator, "--skip={d}", .{skip});

    const git_z = try arena_allocator.dupeZ(u8, "git");
    const log_z = try arena_allocator.dupeZ(u8, "log");
    const all_z = try arena_allocator.dupeZ(u8, "--all");
    const date_z = try arena_allocator.dupeZ(u8, "--date=format:%d %b %Y %H:%M");
    const format_z = try arena_allocator.dupeZ(u8, "--pretty=format:%H\x1f%h\x1f%d\x1f%s\x1f%an\x1f%ad\x1f%p");
    const limit_z = try arena_allocator.dupeZ(u8, limit_str);
    const skip_z = try arena_allocator.dupeZ(u8, skip_str);

    const output = runGitCommand(arena_allocator, parsed.value.cwd, &.{ git_z.ptr, log_z.ptr, all_z.ptr, date_z.ptr, format_z.ptr, skip_z.ptr, limit_z.ptr, null }) catch "";
    const GraphCommit = struct {
        hash: []const u8,
        shortHash: []const u8,
        refs: [][]const u8,
        message: []const u8,
        author: []const u8,
        date: []const u8,
        parents: [][]const u8,
    };

    var commits: std.ArrayList(GraphCommit) = .empty;
    defer commits.deinit(arena_allocator);

    var lines = std.mem.splitScalar(u8, output, '\n');
    while (lines.next()) |line| {
        const trimmed = std.mem.trim(u8, line, " \r\t");
        if (trimmed.len < 10) continue;

        var parts = std.mem.splitScalar(u8, trimmed, '\x1f');
        const hash = parts.next() orelse continue;
        const short_hash = parts.next() orelse continue;
        const raw_refs = parts.next() orelse "";
        const message = parts.next() orelse "";
        const author = parts.next() orelse "";
        const date = parts.next() orelse "";
        const raw_parents = parts.next() orelse "";

        var refs_list: std.ArrayList([]const u8) = .empty;
        const clean_refs = std.mem.trim(u8, raw_refs, " ()\r\t");
        if (clean_refs.len > 0) {
            var ref_parts = std.mem.splitSequence(u8, clean_refs, ", ");
            while (ref_parts.next()) |rp| {
                if (rp.len > 0) {
                    try refs_list.append(arena_allocator, rp);
                }
            }
        }

        var parents_list: std.ArrayList([]const u8) = .empty;
        if (raw_parents.len > 0) {
            var parent_parts = std.mem.splitScalar(u8, raw_parents, ' ');
            while (parent_parts.next()) |pp| {
                if (pp.len > 0) {
                    try parents_list.append(arena_allocator, pp);
                }
            }
        }

        try commits.append(arena_allocator, .{
            .hash = hash,
            .shortHash = short_hash,
            .refs = refs_list.items,
            .message = message,
            .author = author,
            .date = date,
            .parents = parents_list.items,
        });
    }

    var out = std.Io.Writer.Allocating.init(self.allocator);
    defer out.deinit();

    try std.json.Stringify.value(.{
        .commits = commits.items,
    }, .{}, &out.writer);

    try responder.success(invocation.request.id, out.written());
}
fn handleGitShow(context: *anyopaque, invocation: native_sdk.bridge.Invocation, responder: native_sdk.bridge.AsyncResponder) anyerror!void {
    const self: *App = @ptrCast(@alignCast(context));
    self.main_window_id = invocation.source.window_id;

    const Payload = struct {
        cwd: []const u8 = "",
        hash: []const u8,
    };

    const parsed = std.json.parseFromSlice(Payload, self.allocator, invocation.request.payload, .{ .ignore_unknown_fields = true }) catch |err| {
        try responder.fail(invocation.request.id, .invalid_request, @errorName(err));
        return;
    };
    defer parsed.deinit();

    var arena = std.heap.ArenaAllocator.init(self.allocator);
    defer arena.deinit();
    const arena_allocator = arena.allocator();

    const raw_hash = std.mem.trim(u8, parsed.value.hash, " \"'\r\n\t");
    const clean_hash = if (raw_hash.len == 0) "HEAD" else raw_hash;

    var full_hash: []const u8 = clean_hash;
    var short_hash: []const u8 = if (clean_hash.len > 7) clean_hash[0..7] else clean_hash;
    var author: []const u8 = "";
    var email: []const u8 = "";
    var date: []const u8 = "";
    var message: []const u8 = "";

    const ChangedFile = struct {
        status: []const u8,
        path: []const u8,
        additions: usize = 0,
        deletions: usize = 0,
    };
    var files: std.ArrayList(ChangedFile) = .empty;
    defer files.deinit(arena_allocator);

    const git_z = try arena_allocator.dupeZ(u8, "git");
    const hash_z = try arena_allocator.dupeZ(u8, clean_hash);

    // 1. Fetch metadata via git log -1 with formatted date
    const log_z = try arena_allocator.dupeZ(u8, "log");
    const n1_z = try arena_allocator.dupeZ(u8, "-1");
    const date_fmt_z = try arena_allocator.dupeZ(u8, "--date=format:%b %d, %Y");
    const format_z = try arena_allocator.dupeZ(u8, "--pretty=format:__FORGE_META__|%H|%h|%an|%ae|%ad|%B__FORGE_END_META__");

    const meta_output = runGitCommand(arena_allocator, parsed.value.cwd, &.{ git_z.ptr, log_z.ptr, n1_z.ptr, date_fmt_z.ptr, format_z.ptr, hash_z.ptr, null }) catch "";
    if (std.mem.indexOf(u8, meta_output, "__FORGE_META__|")) |start_idx| {
        const meta_start = start_idx + "__FORGE_META__|".len;
        if (std.mem.indexOf(u8, meta_output[meta_start..], "__FORGE_END_META__")) |end_idx| {
            const meta_str = meta_output[meta_start .. meta_start + end_idx];
            var meta_parts = std.mem.splitScalar(u8, meta_str, '|');
            if (meta_parts.next()) |h| full_hash = h;
            if (meta_parts.next()) |sh| short_hash = sh;
            if (meta_parts.next()) |a| author = a;
            if (meta_parts.next()) |e| email = e;
            if (meta_parts.next()) |d| date = d;
            if (meta_parts.next()) |m| message = std.mem.trim(u8, m, " \r\n\t");
        }
    }

    // 2. Fetch numstat additions/deletions via git show --numstat --format="" <hash>
    var total_additions: usize = 0;
    var total_deletions: usize = 0;

    const show_z = try arena_allocator.dupeZ(u8, "show");
    const numstat_z = try arena_allocator.dupeZ(u8, "--numstat");
    const format_empty_z = try arena_allocator.dupeZ(u8, "--format=");

    const numstat_output = runGitCommand(arena_allocator, parsed.value.cwd, &.{ git_z.ptr, show_z.ptr, numstat_z.ptr, format_empty_z.ptr, hash_z.ptr, null }) catch "";
    var numstat_lines = std.mem.splitScalar(u8, numstat_output, '\n');
    while (numstat_lines.next()) |line| {
        const trimmed = std.mem.trim(u8, line, " \r\t");
        if (trimmed.len < 3) continue;

        var parts = std.mem.splitScalar(u8, trimmed, '\t');
        const adds_str = parts.next() orelse continue;
        const dels_str = parts.next() orelse continue;
        const raw_path = parts.next() orelse continue;

        const adds = std.fmt.parseInt(usize, adds_str, 10) catch 0;
        const dels = std.fmt.parseInt(usize, dels_str, 10) catch 0;
        total_additions += adds;
        total_deletions += dels;

        const clean_fpath = std.mem.trim(u8, raw_path, " \r\t");
        if (clean_fpath.len == 0) continue;

        try files.append(arena_allocator, .{
            .status = if (dels > 0 and adds == 0) "D" else if (adds > 0 and dels == 0) "A" else "M",
            .path = clean_fpath,
            .additions = adds,
            .deletions = dels,
        });
    }

    // 3. Fallback to name-status if numstat produced no files
    if (files.items.len == 0) {
        const name_status_z = try arena_allocator.dupeZ(u8, "--name-status");
        const name_status_output = runGitCommand(arena_allocator, parsed.value.cwd, &.{ git_z.ptr, show_z.ptr, name_status_z.ptr, format_empty_z.ptr, hash_z.ptr, null }) catch "";
        var name_lines = std.mem.splitScalar(u8, name_status_output, '\n');
        while (name_lines.next()) |line| {
            const trimmed = std.mem.trim(u8, line, " \r\t");
            if (trimmed.len < 2) continue;

            var status_part: []const u8 = "";
            var path_part: []const u8 = "";
            if (std.mem.indexOfScalar(u8, trimmed, '\t')) |t_idx| {
                status_part = std.mem.trim(u8, trimmed[0..t_idx], " \r\t");
                const remainder = std.mem.trim(u8, trimmed[t_idx + 1 ..], " \r\t");
                if (std.mem.indexOfScalar(u8, remainder, '\t')) |second_t| {
                    path_part = std.mem.trim(u8, remainder[second_t + 1 ..], " \r\t");
                } else {
                    path_part = remainder;
                }
            } else if (std.mem.indexOfScalar(u8, trimmed, ' ')) |s_idx| {
                status_part = std.mem.trim(u8, trimmed[0..s_idx], " \r\t");
                const remainder = std.mem.trim(u8, trimmed[s_idx + 1 ..], " \r\t");
                if (std.mem.indexOfScalar(u8, remainder, ' ')) |second_s| {
                    path_part = std.mem.trim(u8, remainder[second_s + 1 ..], " \r\t");
                } else {
                    path_part = remainder;
                }
            }

            const clean_status = if (status_part.len > 0 and (status_part[0] == 'R' or status_part[0] == 'r'))
                "R"
            else if (status_part.len > 0 and (status_part[0] == 'C' or status_part[0] == 'c'))
                "C"
            else
                status_part;

            if (clean_status.len > 0 and path_part.len > 0) {
                try files.append(arena_allocator, .{
                    .status = clean_status,
                    .path = path_part,
                    .additions = 0,
                    .deletions = 0,
                });
            }
        }
    }

    var out = std.Io.Writer.Allocating.init(self.allocator);
    defer out.deinit();

    try std.json.Stringify.value(.{
        .hash = full_hash,
        .shortHash = short_hash,
        .author = author,
        .email = email,
        .date = date,
        .message = message,
        .totalAdditions = total_additions,
        .totalDeletions = total_deletions,
        .files = files.items,
    }, .{}, &out.writer);

    try responder.success(invocation.request.id, out.written());
}

fn handleGitDiff(context: *anyopaque, invocation: native_sdk.bridge.Invocation, responder: native_sdk.bridge.AsyncResponder) anyerror!void {
    const self: *App = @ptrCast(@alignCast(context));
    self.main_window_id = invocation.source.window_id;

    const Payload = struct {
        cwd: []const u8 = "",
        hash: []const u8 = "HEAD",
        path: []const u8 = "",
    };

    const parsed = std.json.parseFromSlice(Payload, self.allocator, invocation.request.payload, .{ .ignore_unknown_fields = true }) catch |err| {
        try responder.fail(invocation.request.id, .invalid_request, @errorName(err));
        return;
    };
    defer parsed.deinit();

    var arena = std.heap.ArenaAllocator.init(self.allocator);
    defer arena.deinit();
    const arena_allocator = arena.allocator();

    const raw_hash = std.mem.trim(u8, parsed.value.hash, " \"'\r\n\t");
    const clean_hash = if (raw_hash.len == 0) "HEAD" else raw_hash;
    const clean_path = std.mem.trim(u8, parsed.value.path, " \"'\r\n\t");

    const is_working_diff = std.mem.eql(u8, clean_hash, "WORKING") or std.mem.eql(u8, clean_hash, "UNCOMMITTED");

    var output: []u8 = "";
    var full_hash: []const u8 = clean_hash;
    var short_hash: []const u8 = if (clean_hash.len > 7) clean_hash[0..7] else clean_hash;
    var author: []const u8 = "";
    var email: []const u8 = "";
    var date: []const u8 = "";
    var message: []const u8 = "";

    if (is_working_diff) {
        full_hash = "WORKING";
        short_hash = "WORKING";
        author = "You";
        email = "Working Tree";
        date = "Uncommitted Changes";
        message = if (clean_path.len > 0)
            try std.fmt.allocPrint(arena_allocator, "Changes in {s}", .{clean_path})
        else
            "Working tree uncommitted changes";

        const git_z = try arena_allocator.dupeZ(u8, "git");
        const diff_z = try arena_allocator.dupeZ(u8, "diff");
        const head_z = try arena_allocator.dupeZ(u8, "HEAD");

        if (clean_path.len > 0) {
            const sep_z = try arena_allocator.dupeZ(u8, "--");
            const path_z = try arena_allocator.dupeZ(u8, clean_path);

            output = runGitCommand(arena_allocator, parsed.value.cwd, &.{ git_z.ptr, diff_z.ptr, head_z.ptr, sep_z.ptr, path_z.ptr, null }) catch "";
            if (output.len == 0) {
                output = runGitCommand(arena_allocator, parsed.value.cwd, &.{ git_z.ptr, diff_z.ptr, sep_z.ptr, path_z.ptr, null }) catch "";
            }
            if (output.len == 0) {
                const cached_z = try arena_allocator.dupeZ(u8, "--cached");
                output = runGitCommand(arena_allocator, parsed.value.cwd, &.{ git_z.ptr, diff_z.ptr, cached_z.ptr, sep_z.ptr, path_z.ptr, null }) catch "";
            }
        } else {
            output = runGitCommand(arena_allocator, parsed.value.cwd, &.{ git_z.ptr, diff_z.ptr, head_z.ptr, null }) catch "";
        }
    } else {
        const git_z = try arena_allocator.dupeZ(u8, "git");
        const show_z = try arena_allocator.dupeZ(u8, "show");
        const format_z = try arena_allocator.dupeZ(u8, "--pretty=format:__DIFF_META__|%H|%h|%an|%ae|%ad|%B__DIFF_END_META__");
        const patch_z = try arena_allocator.dupeZ(u8, "-p");
        const hash_z = try arena_allocator.dupeZ(u8, clean_hash);

        if (clean_path.len > 0) {
            const sep_z = try arena_allocator.dupeZ(u8, "--");
            const path_z = try arena_allocator.dupeZ(u8, clean_path);
            output = runGitCommand(arena_allocator, parsed.value.cwd, &.{ git_z.ptr, show_z.ptr, format_z.ptr, patch_z.ptr, hash_z.ptr, sep_z.ptr, path_z.ptr, null }) catch "";
        } else {
            output = runGitCommand(arena_allocator, parsed.value.cwd, &.{ git_z.ptr, show_z.ptr, format_z.ptr, patch_z.ptr, hash_z.ptr, null }) catch "";
        }
    }
    const DiffLine = struct {
        type: []const u8,
        oldLine: ?usize = null,
        newLine: ?usize = null,
        text: []const u8,
    };

    const DiffFile = struct {
        path: []const u8,
        additions: usize,
        deletions: usize,
        lines: []const DiffLine,
    };

    var files: std.ArrayList(DiffFile) = .empty;
    defer files.deinit(arena_allocator);

    var total_additions: usize = 0;
    var total_deletions: usize = 0;

    if (is_working_diff) {
        if (output.len == 0 and parsed.value.path.len > 0) {
            // Untracked file fallback: read file directly from disk
            const full_file_path = if (parsed.value.cwd.len > 0)
                try std.fs.path.join(arena_allocator, &.{ parsed.value.cwd, parsed.value.path })
            else
                parsed.value.path;
            const full_file_z = try arena_allocator.dupeZ(u8, full_file_path);
            const fd = c.open(full_file_z, 0x0004);
            if (fd >= 0) {
                defer _ = c.close(fd);
                var content_buf: std.ArrayList(u8) = .empty;
                var b: [8192]u8 = undefined;
                while (true) {
                    const n = c.read(fd, &b, b.len);
                    if (n <= 0) break;
                    content_buf.appendSlice(arena_allocator, b[0..@intCast(n)]) catch break;
                    if (content_buf.items.len > 1024 * 1024) break;
                }

                var untracked_lines: std.ArrayList(DiffLine) = .empty;
                var line_it = std.mem.splitScalar(u8, content_buf.items, '\n');
                var line_c: usize = 1;
                while (line_it.next()) |lt| {
                    try untracked_lines.append(arena_allocator, .{
                        .type = "insert",
                        .newLine = line_c,
                        .text = try arena_allocator.dupe(u8, std.mem.trim(u8, lt, "\r")),
                    });
                    line_c += 1;
                }

                try files.append(arena_allocator, .{
                    .path = parsed.value.path,
                    .additions = untracked_lines.items.len,
                    .deletions = 0,
                    .lines = untracked_lines.items,
                });
                total_additions = untracked_lines.items.len;
            }
        } else if (output.len > 0) {
            var current_file_path: ?[]const u8 = null;
            var current_file_lines: std.ArrayList(DiffLine) = .empty;
            var current_adds: usize = 0;
            var current_dels: usize = 0;
            var old_line_cur: usize = 1;
            var new_line_cur: usize = 1;

            var lines_iter = std.mem.splitScalar(u8, output, '\n');
            while (lines_iter.next()) |line| {
                if (std.mem.startsWith(u8, line, "diff --git ")) {
                    if (current_file_path) |cp| {
                        if (current_file_lines.items.len > 0 or current_adds > 0 or current_dels > 0) {
                            try files.append(arena_allocator, .{
                                .path = cp,
                                .additions = current_adds,
                                .deletions = current_dels,
                                .lines = try arena_allocator.dupe(DiffLine, current_file_lines.items),
                            });
                            current_file_lines.clearRetainingCapacity();
                            current_adds = 0;
                            current_dels = 0;
                        }
                    }
                    if (std.mem.indexOf(u8, line, " b/")) |b_idx| {
                        const raw_path = line[b_idx + 3 ..];
                        if (std.mem.eql(u8, raw_path, "/dev/null") or std.mem.eql(u8, raw_path, "dev/null")) {
                            if (std.mem.indexOf(u8, line, " a/")) |a_idx| {
                                const end_a = std.mem.indexOf(u8, line[a_idx + 3 ..], " b/") orelse (line.len - (a_idx + 3));
                                current_file_path = arena_allocator.dupe(u8, line[a_idx + 3 .. a_idx + 3 + end_a]) catch null;
                            } else {
                                current_file_path = arena_allocator.dupe(u8, raw_path) catch null;
                            }
                        } else {
                            current_file_path = arena_allocator.dupe(u8, raw_path) catch null;
                        }
                    } else if (parsed.value.path.len > 0) {
                        current_file_path = parsed.value.path;
                    }
                    continue;
                }

                if (std.mem.startsWith(u8, line, "@@ ")) {
                    var parts = std.mem.splitScalar(u8, line, ' ');
                    _ = parts.next();
                    const old_part = parts.next() orelse "-1";
                    const new_part = parts.next() orelse "+1";

                    const old_num_str = if (std.mem.indexOfScalar(u8, old_part, ',')) |c_idx| old_part[1..c_idx] else if (old_part.len > 1) old_part[1..] else "1";
                    const new_num_str = if (std.mem.indexOfScalar(u8, new_part, ',')) |c_idx| new_part[1..c_idx] else if (new_part.len > 1) new_part[1..] else "1";

                    old_line_cur = std.fmt.parseInt(usize, old_num_str, 10) catch 1;
                    new_line_cur = std.fmt.parseInt(usize, new_num_str, 10) catch 1;
                    continue;
                }

                if (current_file_path == null) continue;
                if (std.mem.startsWith(u8, line, "index ") or std.mem.startsWith(u8, line, "--- ") or std.mem.startsWith(u8, line, "+++ ")) {
                    continue;
                }

                if (line.len > 0 and line[0] == '+') {
                    current_adds += 1;
                    total_additions += 1;
                    try current_file_lines.append(arena_allocator, .{
                        .type = "insert",
                        .newLine = new_line_cur,
                        .text = try arena_allocator.dupe(u8, line[1..]),
                    });
                    new_line_cur += 1;
                } else if (line.len > 0 and line[0] == '-') {
                    current_dels += 1;
                    total_deletions += 1;
                    try current_file_lines.append(arena_allocator, .{
                        .type = "delete",
                        .oldLine = old_line_cur,
                        .text = try arena_allocator.dupe(u8, line[1..]),
                    });
                    old_line_cur += 1;
                } else if (line.len > 0 and line[0] == ' ') {
                    try current_file_lines.append(arena_allocator, .{
                        .type = "context",
                        .oldLine = old_line_cur,
                        .newLine = new_line_cur,
                        .text = try arena_allocator.dupe(u8, line[1..]),
                    });
                    old_line_cur += 1;
                    new_line_cur += 1;
                }
            }

            if (current_file_path) |cp| {
                if (current_file_lines.items.len > 0 or current_adds > 0 or current_dels > 0 or files.items.len == 0) {
                    try files.append(arena_allocator, .{
                        .path = cp,
                        .additions = current_adds,
                        .deletions = current_dels,
                        .lines = try arena_allocator.dupe(DiffLine, current_file_lines.items),
                    });
                }
            }
        }
    } else {
        if (std.mem.indexOf(u8, output, "__DIFF_META__|")) |start_idx| {
            const meta_start = start_idx + "__DIFF_META__|".len;
            if (std.mem.indexOf(u8, output[meta_start..], "__DIFF_END_META__")) |end_idx| {
                const meta_str = output[meta_start .. meta_start + end_idx];
                const rest_str = output[meta_start + end_idx + "__DIFF_END_META__".len ..];

                var meta_parts = std.mem.splitScalar(u8, meta_str, '|');
                if (meta_parts.next()) |h| full_hash = h;
                if (meta_parts.next()) |sh| short_hash = sh;
                if (meta_parts.next()) |a| author = a;
                if (meta_parts.next()) |e| email = e;
                if (meta_parts.next()) |d| date = d;
                if (meta_parts.next()) |m| message = std.mem.trim(u8, m, " \r\n\t");

                var current_file_path: ?[]const u8 = null;
                var current_file_lines: std.ArrayList(DiffLine) = .empty;
                var current_adds: usize = 0;
                var current_dels: usize = 0;
                var old_line_cur: usize = 1;
                var new_line_cur: usize = 1;

                var lines_iter = std.mem.splitScalar(u8, rest_str, '\n');
                while (lines_iter.next()) |line| {
                    if (std.mem.startsWith(u8, line, "diff --git ")) {
                        if (current_file_path) |cp| {
                            if (current_file_lines.items.len > 0 or current_adds > 0 or current_dels > 0) {
                                try files.append(arena_allocator, .{
                                    .path = cp,
                                    .additions = current_adds,
                                    .deletions = current_dels,
                                    .lines = try arena_allocator.dupe(DiffLine, current_file_lines.items),
                                });
                                current_file_lines.clearRetainingCapacity();
                                current_adds = 0;
                                current_dels = 0;
                            }
                        }
                        if (std.mem.indexOf(u8, line, " b/")) |b_idx| {
                            const raw_path = line[b_idx + 3 ..];
                            if (std.mem.eql(u8, raw_path, "/dev/null") or std.mem.eql(u8, raw_path, "dev/null")) {
                                if (std.mem.indexOf(u8, line, " a/")) |a_idx| {
                                    const end_a = std.mem.indexOf(u8, line[a_idx + 3 ..], " b/") orelse (line.len - (a_idx + 3));
                                    current_file_path = arena_allocator.dupe(u8, line[a_idx + 3 .. a_idx + 3 + end_a]) catch null;
                                } else {
                                    current_file_path = arena_allocator.dupe(u8, raw_path) catch null;
                                }
                            } else {
                                current_file_path = arena_allocator.dupe(u8, raw_path) catch null;
                            }
                        } else if (parsed.value.path.len > 0) {
                            current_file_path = parsed.value.path;
                        }
                        continue;
                    }

                    if (std.mem.startsWith(u8, line, "@@ ")) {
                        var parts = std.mem.splitScalar(u8, line, ' ');
                        _ = parts.next();
                        const old_part = parts.next() orelse "-1";
                        const new_part = parts.next() orelse "+1";

                        const old_num_str = if (std.mem.indexOfScalar(u8, old_part, ',')) |c_idx| old_part[1..c_idx] else if (old_part.len > 1) old_part[1..] else "1";
                        const new_num_str = if (std.mem.indexOfScalar(u8, new_part, ',')) |c_idx| new_part[1..c_idx] else if (new_part.len > 1) new_part[1..] else "1";

                        old_line_cur = std.fmt.parseInt(usize, old_num_str, 10) catch 1;
                        new_line_cur = std.fmt.parseInt(usize, new_num_str, 10) catch 1;
                        continue;
                    }

                    if (current_file_path == null) continue;
                    if (std.mem.startsWith(u8, line, "index ") or std.mem.startsWith(u8, line, "--- ") or std.mem.startsWith(u8, line, "+++ ")) {
                        continue;
                    }

                    if (line.len > 0 and line[0] == '+') {
                        current_adds += 1;
                        total_additions += 1;
                        try current_file_lines.append(arena_allocator, .{
                            .type = "insert",
                            .newLine = new_line_cur,
                            .text = try arena_allocator.dupe(u8, line[1..]),
                        });
                        new_line_cur += 1;
                    } else if (line.len > 0 and line[0] == '-') {
                        current_dels += 1;
                        total_deletions += 1;
                        try current_file_lines.append(arena_allocator, .{
                            .type = "delete",
                            .oldLine = old_line_cur,
                            .text = try arena_allocator.dupe(u8, line[1..]),
                        });
                        old_line_cur += 1;
                    } else if (line.len > 0 and line[0] == ' ') {
                        try current_file_lines.append(arena_allocator, .{
                            .type = "context",
                            .oldLine = old_line_cur,
                            .newLine = new_line_cur,
                            .text = try arena_allocator.dupe(u8, line[1..]),
                        });
                        old_line_cur += 1;
                        new_line_cur += 1;
                    }
                }

                if (current_file_path) |cp| {
                    if (current_file_lines.items.len > 0 or current_adds > 0 or current_dels > 0 or files.items.len == 0) {
                        try files.append(arena_allocator, .{
                            .path = cp,
                            .additions = current_adds,
                            .deletions = current_dels,
                            .lines = try arena_allocator.dupe(DiffLine, current_file_lines.items),
                        });
                    }
                }
            }
        }
    }
    var out = std.Io.Writer.Allocating.init(self.allocator);
    defer out.deinit();

    try std.json.Stringify.value(.{
        .hash = full_hash,
        .shortHash = short_hash,
        .author = author,
        .email = email,
        .date = date,
        .message = message,
        .totalAdditions = total_additions,
        .totalDeletions = total_deletions,
        .files = files.items,
    }, .{}, &out.writer);

    try responder.success(invocation.request.id, out.written());
}

// ============================================================================
// Filesystem Directory & File Handlers
// ============================================================================

const FileEntry = struct {
    name: []const u8,
    path: []const u8,
    isDir: bool,
};

fn entryLessThan(_: void, a: FileEntry, b: FileEntry) bool {
    if (a.isDir != b.isDir) {
        return a.isDir;
    }
    return std.mem.lessThan(u8, a.name, b.name);
}

fn handleFsReadDir(context: *anyopaque, invocation: native_sdk.bridge.Invocation, responder: native_sdk.bridge.AsyncResponder) anyerror!void {
    const self: *App = @ptrCast(@alignCast(context));
    self.main_window_id = invocation.source.window_id;

    const Payload = struct {
        path: []const u8 = "",
    };

    const parsed = std.json.parseFromSlice(Payload, self.allocator, invocation.request.payload, .{ .ignore_unknown_fields = true }) catch |err| {
        try responder.fail(invocation.request.id, .invalid_request, @errorName(err));
        return;
    };
    defer parsed.deinit();

    var arena = std.heap.ArenaAllocator.init(self.allocator);
    defer arena.deinit();
    const arena_allocator = arena.allocator();

    var cwd_buf: [1024]u8 = undefined;
    const cwd_ptr = c.getcwd(&cwd_buf, cwd_buf.len);
    const cwd_str = if (cwd_ptr) |p| std.mem.sliceTo(p, 0) else ".";

    const target_dir = if (parsed.value.path.len > 0) parsed.value.path else cwd_str;
    const target_dir_z = try arena_allocator.dupeZ(u8, target_dir);

    const dir_handle = c.opendir(target_dir_z);
    if (dir_handle == null) {
        try responder.fail(invocation.request.id, .handler_failed, "Cannot open directory");
        return;
    }
    defer _ = c.closedir(dir_handle.?);

    var entries: std.ArrayList(FileEntry) = .empty;
    defer entries.deinit(arena_allocator);

    while (c.readdir(dir_handle.?)) |entry_ptr| {
        const name = std.mem.sliceTo(&entry_ptr.d_name, 0);
        if (std.mem.eql(u8, name, ".") or std.mem.eql(u8, name, "..") or std.mem.eql(u8, name, ".git") or std.mem.eql(u8, name, "node_modules")) {
            continue;
        }

        const is_dir = (entry_ptr.d_type == 4);
        const full_path = if (std.mem.eql(u8, target_dir, "."))
            try arena_allocator.dupe(u8, name)
        else
            try std.fs.path.join(arena_allocator, &.{ target_dir, name });

        try entries.append(arena_allocator, .{
            .name = try arena_allocator.dupe(u8, name),
            .path = full_path,
            .isDir = is_dir,
        });
    }

    std.sort.block(FileEntry, entries.items, {}, entryLessThan);

    var out = std.Io.Writer.Allocating.init(self.allocator);
    defer out.deinit();

    try std.json.Stringify.value(.{
        .path = target_dir,
        .entries = entries.items,
    }, .{}, &out.writer);

    try responder.success(invocation.request.id, out.written());
}

fn handleFsReadFile(context: *anyopaque, invocation: native_sdk.bridge.Invocation, responder: native_sdk.bridge.AsyncResponder) anyerror!void {
    const self: *App = @ptrCast(@alignCast(context));
    self.main_window_id = invocation.source.window_id;

    const Payload = struct {
        path: []const u8,
    };

    const parsed = std.json.parseFromSlice(Payload, self.allocator, invocation.request.payload, .{ .ignore_unknown_fields = true }) catch |err| {
        try responder.fail(invocation.request.id, .invalid_request, @errorName(err));
        return;
    };
    defer parsed.deinit();

    var arena = std.heap.ArenaAllocator.init(self.allocator);
    defer arena.deinit();
    const arena_allocator = arena.allocator();

    const path_z = try arena_allocator.dupeZ(u8, parsed.value.path);
    const fd = c.open(path_z, 0);
    if (fd < 0) {
        try responder.fail(invocation.request.id, .handler_failed, "File not found or cannot open");
        return;
    }
    defer _ = c.close(fd);

    var content: std.ArrayList(u8) = .empty;
    defer content.deinit(arena_allocator);

    var buf: [8192]u8 = undefined;
    while (true) {
        const n = c.read(fd, &buf, buf.len);
        if (n <= 0) break;
        try content.appendSlice(arena_allocator, buf[0..@intCast(n)]);
        if (content.items.len > 25 * 1024 * 1024) break;
    }

    const ext = std.fs.path.extension(parsed.value.path);
    const is_image = std.mem.eql(u8, ext, ".png") or std.mem.eql(u8, ext, ".jpg") or std.mem.eql(u8, ext, ".jpeg") or std.mem.eql(u8, ext, ".gif") or std.mem.eql(u8, ext, ".webp") or std.mem.eql(u8, ext, ".ico") or std.mem.eql(u8, ext, ".bmp");
    const is_utf8 = std.unicode.utf8ValidateSlice(content.items);

    var out = std.Io.Writer.Allocating.init(self.allocator);
    defer out.deinit();

    if (is_image or !is_utf8) {
        const encoder = std.base64.standard.Encoder;
        const encoded_len = encoder.calcSize(content.items.len);
        const encoded = try arena_allocator.alloc(u8, encoded_len);
        _ = encoder.encode(encoded, content.items);

        try std.json.Stringify.value(.{
            .path = parsed.value.path,
            .content = if (is_utf8) content.items else "",
            .base64 = encoded,
            .isBinary = true,
            .size = content.items.len,
        }, .{}, &out.writer);
    } else {
        try std.json.Stringify.value(.{
            .path = parsed.value.path,
            .content = content.items,
            .isBinary = false,
            .size = content.items.len,
        }, .{}, &out.writer);
    }

    try responder.success(invocation.request.id, out.written());
}

fn handleFsWriteFile(context: *anyopaque, invocation: native_sdk.bridge.Invocation, responder: native_sdk.bridge.AsyncResponder) anyerror!void {
    const self: *App = @ptrCast(@alignCast(context));
    self.main_window_id = invocation.source.window_id;

    const Payload = struct {
        path: []const u8,
        content: ?[]const u8 = null,
        base64: ?[]const u8 = null,
        isBinary: ?bool = null,
    };

    const parsed = std.json.parseFromSlice(Payload, self.allocator, invocation.request.payload, .{ .ignore_unknown_fields = true }) catch |err| {
        try responder.fail(invocation.request.id, .invalid_request, @errorName(err));
        return;
    };
    defer parsed.deinit();

    var arena = std.heap.ArenaAllocator.init(self.allocator);
    defer arena.deinit();
    const arena_allocator = arena.allocator();

    const path_z = try arena_allocator.dupeZ(u8, parsed.value.path);
    const flags: c_int = switch (@import("builtin").os.tag) {
        .macos => 0x0601,
        else => 0x0241,
    };
    const fd = c.open(path_z, flags, @as(c_uint, 0o644));
    if (fd < 0) {
        try responder.fail(invocation.request.id, .handler_failed, "Cannot open file for writing");
        return;
    }
    defer _ = c.close(fd);

    if (parsed.value.base64) |b64| {
        const decoder = std.base64.standard.Decoder;
        const decoded_len = decoder.calcSizeForSlice(b64) catch |err| {
            try responder.fail(invocation.request.id, .invalid_request, @errorName(err));
            return;
        };
        const decoded = try arena_allocator.alloc(u8, decoded_len);
        decoder.decode(decoded, b64) catch |err| {
            try responder.fail(invocation.request.id, .invalid_request, @errorName(err));
            return;
        };

        var total_written: usize = 0;
        while (total_written < decoded.len) {
            const n = c.write(fd, decoded[total_written..].ptr, decoded.len - total_written);
            if (n < 0) {
                try responder.fail(invocation.request.id, .handler_failed, "Write error");
                return;
            }
            total_written += @intCast(n);
        }
    } else if (parsed.value.content) |txt| {
        var total_written: usize = 0;
        while (total_written < txt.len) {
            const n = c.write(fd, txt[total_written..].ptr, txt.len - total_written);
            if (n < 0) {
                try responder.fail(invocation.request.id, .handler_failed, "Write error");
                return;
            }
            total_written += @intCast(n);
        }
    }

    var out = std.Io.Writer.Allocating.init(self.allocator);
    defer out.deinit();

    try std.json.Stringify.value(.{
        .ok = true,
        .path = parsed.value.path,
    }, .{}, &out.writer);

    try responder.success(invocation.request.id, out.written());
}

fn handleFsCreateFile(context: *anyopaque, invocation: native_sdk.bridge.Invocation, responder: native_sdk.bridge.AsyncResponder) anyerror!void {
    const self: *App = @ptrCast(@alignCast(context));
    self.main_window_id = invocation.source.window_id;

    const Payload = struct {
        path: []const u8,
    };

    const parsed = std.json.parseFromSlice(Payload, self.allocator, invocation.request.payload, .{ .ignore_unknown_fields = true }) catch |err| {
        try responder.fail(invocation.request.id, .invalid_request, @errorName(err));
        return;
    };
    defer parsed.deinit();

    var arena = std.heap.ArenaAllocator.init(self.allocator);
    defer arena.deinit();
    const arena_allocator = arena.allocator();

    const path_z = try arena_allocator.dupeZ(u8, parsed.value.path);
    const flags: c_int = switch (@import("builtin").os.tag) {
        .macos => 0x0601,
        else => 0x0241,
    };
    const fd = c.open(path_z, flags, @as(c_uint, 0o644));
    if (fd < 0) {
        try responder.fail(invocation.request.id, .handler_failed, "Cannot create file");
        return;
    }
    _ = c.close(fd);

    var out = std.Io.Writer.Allocating.init(self.allocator);
    defer out.deinit();

    try std.json.Stringify.value(.{
        .ok = true,
        .path = parsed.value.path,
    }, .{}, &out.writer);

    try responder.success(invocation.request.id, out.written());
}

fn handleFsCreateDir(context: *anyopaque, invocation: native_sdk.bridge.Invocation, responder: native_sdk.bridge.AsyncResponder) anyerror!void {
    const self: *App = @ptrCast(@alignCast(context));
    self.main_window_id = invocation.source.window_id;

    const Payload = struct {
        path: []const u8,
    };

    const parsed = std.json.parseFromSlice(Payload, self.allocator, invocation.request.payload, .{ .ignore_unknown_fields = true }) catch |err| {
        try responder.fail(invocation.request.id, .invalid_request, @errorName(err));
        return;
    };
    defer parsed.deinit();

    var arena = std.heap.ArenaAllocator.init(self.allocator);
    defer arena.deinit();
    const arena_allocator = arena.allocator();

    const path_z = try arena_allocator.dupeZ(u8, parsed.value.path);
    const res = c.mkdir(path_z, 0o755);
    if (res != 0) {
        try responder.fail(invocation.request.id, .handler_failed, "Cannot create directory");
        return;
    }

    var out = std.Io.Writer.Allocating.init(self.allocator);
    defer out.deinit();

    try std.json.Stringify.value(.{
        .ok = true,
        .path = parsed.value.path,
    }, .{}, &out.writer);

    try responder.success(invocation.request.id, out.written());
}

fn handleFsGetCwd(context: *anyopaque, invocation: native_sdk.bridge.Invocation, responder: native_sdk.bridge.AsyncResponder) anyerror!void {
    const self: *App = @ptrCast(@alignCast(context));
    self.main_window_id = invocation.source.window_id;

    var cwd_buf: [1024]u8 = undefined;
    const cwd_ptr = c.getcwd(&cwd_buf, cwd_buf.len);
    const cwd_str = if (cwd_ptr) |p| std.mem.sliceTo(p, 0) else ".";
    const home_str = self.env_map.get("HOME") orelse "";

    var out = std.Io.Writer.Allocating.init(self.allocator);
    defer out.deinit();

    try std.json.Stringify.value(.{
        .cwd = cwd_str,
        .homedir = home_str,
    }, .{}, &out.writer);

    try responder.success(invocation.request.id, out.written());
}

// ============================================================================
// LSP Bridge Handlers (§19–§23)
// ============================================================================

fn handleLspStart(context: *anyopaque, invocation: native_sdk.bridge.Invocation, responder: native_sdk.bridge.AsyncResponder) anyerror!void {
    const self: *App = @ptrCast(@alignCast(context));
    self.main_window_id = invocation.source.window_id;

    const Payload = struct {
        command: []const u8,
        cwd: []const u8 = "",
    };

    const parsed = std.json.parseFromSlice(Payload, self.allocator, invocation.request.payload, .{ .ignore_unknown_fields = true }) catch |err| {
        try responder.fail(invocation.request.id, .invalid_request, @errorName(err));
        return;
    };
    defer parsed.deinit();

    var arena = std.heap.ArenaAllocator.init(self.allocator);
    defer arena.deinit();
    const arena_allocator = arena.allocator();

    var in_pipe: [2]c_int = undefined;
    var out_pipe: [2]c_int = undefined;

    if (c.pipe(&in_pipe) != 0 or c.pipe(&out_pipe) != 0) {
        try responder.fail(invocation.request.id, .handler_failed, "pipe failed");
        return;
    }

    const pid = c.fork();
    if (pid < 0) {
        _ = c.close(in_pipe[0]);
        _ = c.close(in_pipe[1]);
        _ = c.close(out_pipe[0]);
        _ = c.close(out_pipe[1]);
        try responder.fail(invocation.request.id, .handler_failed, "fork failed");
        return;
    }

    if (pid == 0) {
        // Child process
        _ = c.close(in_pipe[1]); // Close write end of stdin
        _ = c.close(out_pipe[0]); // Close read end of stdout

        _ = c.dup2(in_pipe[0], 0);
        _ = c.dup2(out_pipe[1], 1);
        _ = c.dup2(out_pipe[1], 2);

        if (parsed.value.cwd.len > 0) {
            const cwd_z = arena_allocator.dupeZ(u8, parsed.value.cwd) catch c._exit(127);
            _ = c.chdir(cwd_z.ptr);
        }

        const cmd_z = try arena_allocator.dupeZ(u8, parsed.value.command);
        const sh_z = try arena_allocator.dupeZ(u8, "/bin/sh");
        const c_flag_z = try arena_allocator.dupeZ(u8, "-c");
        const child_argv = [_:null]?[*:0]const u8{ sh_z.ptr, c_flag_z.ptr, cmd_z.ptr };

        _ = c.execvp(sh_z.ptr, &child_argv);
        c._exit(127);
    }

    _ = c.close(in_pipe[0]);
    _ = c.close(out_pipe[1]);

    self.mutex.lock();
    defer self.mutex.unlock();

    const lsp = try self.allocator.create(LspSession);
    lsp.* = .{
        .id = self.next_lsp_id,
        .stdin_fd = in_pipe[1],
        .stdout_fd = out_pipe[0],
        .child_pid = pid,
        .window_id = invocation.source.window_id,
    };
    self.next_lsp_id += 1;
    try self.lsp_sessions.append(self.allocator, lsp);

    const thread = try std.Thread.spawn(.{}, lspReaderThread, .{ self, lsp });
    thread.detach();

    var response_buf: [256]u8 = undefined;
    const response = try std.fmt.bufPrint(&response_buf, "{{\"lspId\":{d}}}", .{lsp.id});
    try responder.success(invocation.request.id, response);
}

fn handleLspWrite(context: *anyopaque, invocation: native_sdk.bridge.Invocation, responder: native_sdk.bridge.AsyncResponder) anyerror!void {
    const self: *App = @ptrCast(@alignCast(context));
    self.main_window_id = invocation.source.window_id;

    const Payload = struct {
        lspId: usize,
        data: []const u8,
    };

    const parsed = std.json.parseFromSlice(Payload, self.allocator, invocation.request.payload, .{ .ignore_unknown_fields = true }) catch |err| {
        try responder.fail(invocation.request.id, .invalid_request, @errorName(err));
        return;
    };
    defer parsed.deinit();

    self.mutex.lock();
    const lsp_opt = for (self.lsp_sessions.items) |s| {
        if (s.id == parsed.value.lspId) break s;
    } else null;
    self.mutex.unlock();

    const lsp = lsp_opt orelse {
        try responder.fail(invocation.request.id, .invalid_request, "LSP session not found");
        return;
    };

    if (lsp.stdin_fd < 0) {
        try responder.fail(invocation.request.id, .invalid_request, "LSP stdin closed");
        return;
    }

    var header_buf: [128]u8 = undefined;
    const header = try std.fmt.bufPrint(&header_buf, "Content-Length: {d}\r\n\r\n", .{parsed.value.data.len});

    _ = c.write(lsp.stdin_fd, header.ptr, header.len);
    _ = c.write(lsp.stdin_fd, parsed.value.data.ptr, parsed.value.data.len);

    try responder.success(invocation.request.id, "{\"ok\":true}");
}

fn handleLspStop(context: *anyopaque, invocation: native_sdk.bridge.Invocation, responder: native_sdk.bridge.AsyncResponder) anyerror!void {
    const self: *App = @ptrCast(@alignCast(context));
    self.main_window_id = invocation.source.window_id;

    const Payload = struct {
        lspId: usize,
    };

    const parsed = std.json.parseFromSlice(Payload, self.allocator, invocation.request.payload, .{ .ignore_unknown_fields = true }) catch |err| {
        try responder.fail(invocation.request.id, .invalid_request, @errorName(err));
        return;
    };
    defer parsed.deinit();

    self.mutex.lock();
    defer self.mutex.unlock();

    for (self.lsp_sessions.items, 0..) |lsp, index| {
        if (lsp.id == parsed.value.lspId) {
            if (lsp.child_pid > 0) {
                _ = c.kill(lsp.child_pid, 9);
                _ = c.waitpid(lsp.child_pid, null, 1);
            }
            if (lsp.stdin_fd >= 0) _ = c.close(lsp.stdin_fd);
            if (lsp.stdout_fd >= 0) _ = c.close(lsp.stdout_fd);

            _ = self.lsp_sessions.swapRemove(index);
            self.allocator.destroy(lsp);
            break;
        }
    }

    try responder.success(invocation.request.id, "{\"ok\":true}");
}

fn runShellCommand(allocator: std.mem.Allocator, cwd: []const u8, command: []const u8) !struct { output: []u8, exit_code: i32 } {
    var out_pipe: [2]c_int = undefined;
    if (c.pipe(&out_pipe) != 0) return error.PipeFailed;

    const pid = c.fork();
    if (pid < 0) {
        _ = c.close(out_pipe[0]);
        _ = c.close(out_pipe[1]);
        return error.ForkFailed;
    }

    if (pid == 0) {
        _ = c.close(out_pipe[0]);
        _ = c.dup2(out_pipe[1], 1);
        _ = c.dup2(out_pipe[1], 2);
        _ = c.close(out_pipe[1]);

        if (cwd.len > 0) {
            var arena = std.heap.ArenaAllocator.init(allocator);
            const arena_allocator = arena.allocator();
            const cwd_z = arena_allocator.dupeZ(u8, cwd) catch c._exit(127);
            _ = c.chdir(cwd_z.ptr);
        }

        var arena = std.heap.ArenaAllocator.init(allocator);
        const arena_allocator = arena.allocator();
        const sh_z = arena_allocator.dupeZ(u8, "/bin/sh") catch c._exit(127);
        const flag_z = arena_allocator.dupeZ(u8, "-c") catch c._exit(127);
        const cmd_z = arena_allocator.dupeZ(u8, command) catch c._exit(127);
        const args = [_]?[*:0]const u8{ sh_z.ptr, flag_z.ptr, cmd_z.ptr, null };
        _ = c.execvp(sh_z.ptr, @ptrCast(&args));
        c._exit(127);
    }
    _ = c.close(out_pipe[1]);

    var output: std.ArrayList(u8) = .empty;
    var buf: [4096]u8 = undefined;
    while (true) {
        const n = c.read(out_pipe[0], &buf, buf.len);
        if (n <= 0) break;
        try output.appendSlice(allocator, buf[0..@intCast(n)]);
    }
    _ = c.close(out_pipe[0]);
    var status: c_int = 0;
    _ = c.waitpid(pid, &status, 0);

    const exit_code: i32 = if (status >= 0) @intCast((status >> 8) & 0xff) else -1;
    return .{
        .output = try output.toOwnedSlice(allocator),
        .exit_code = exit_code,
    };
}

fn handleCommandExec(context: *anyopaque, invocation: native_sdk.bridge.Invocation, responder: native_sdk.bridge.AsyncResponder) anyerror!void {
    const self: *App = @ptrCast(@alignCast(context));
    self.main_window_id = invocation.source.window_id;

    const Payload = struct {
        command: []const u8,
        cwd: []const u8 = "",
    };

    const parsed = std.json.parseFromSlice(Payload, self.allocator, invocation.request.payload, .{ .ignore_unknown_fields = true }) catch |err| {
        try responder.fail(invocation.request.id, .invalid_request, @errorName(err));
        return;
    };
    defer parsed.deinit();

    var arena = std.heap.ArenaAllocator.init(self.allocator);
    defer arena.deinit();
    const arena_allocator = arena.allocator();

    const res = runShellCommand(arena_allocator, parsed.value.cwd, parsed.value.command) catch |err| {
        try responder.fail(invocation.request.id, .internal_error, @errorName(err));
        return;
    };

    var out = std.Io.Writer.Allocating.init(self.allocator);
    defer out.deinit();

    try std.json.Stringify.value(.{
        .output = res.output,
        .exitCode = res.exit_code,
        .success = (res.exit_code == 0),
    }, .{}, &out.writer);

    try responder.success(invocation.request.id, out.written());
}

const builtin_commands = [_]native_sdk.bridge.CommandPolicy{
    .{ .name = "native-sdk.dialog.openFile", .origins = &dev_origins },
    .{ .name = "native-sdk.dialog.saveFile", .origins = &dev_origins },
    .{ .name = "native-sdk.dialog.showMessage", .origins = &dev_origins },
};

pub fn main(init: std.process.Init) !void {
    var app = App.init(std.heap.c_allocator, init.environ_map);
    defer app.deinit();

    try runner.runWithOptions(app.app(), .{
        .app_name = "Forge Ade Native",
        .window_title = "Forge Ade Native",
        .bundle_id = "dev.native_sdk.forge-ade-native",
        .icon_path = "assets/icon.png",
        .bridge = app.bridge(),
        .builtin_bridge = .{
            .enabled = true,
            .permissions = &full_permissions,
            .commands = &builtin_commands,
        },
        .security = .{
            .permissions = &full_permissions,
            .navigation = .{ .allowed_origins = &dev_origins },
        },
    }, init);
}
