// services.misc — miscellaneous wire methods the frontend calls that don't
// fit the other service modules: git helpers, editor completion, directory
// listing, slash commands, LLM profile save, and the AI commit generator.

const std = @import("std");
const svc = @import("../services.zig");

const c = struct {
    extern "c" fn fork() c_int;
    extern "c" fn pipe(fildes: *[2]c_int) c_int;
    extern "c" fn dup2(oldfd: c_int, newfd: c_int) c_int;
    extern "c" fn close(fd: c_int) c_int;
    extern "c" fn read(fd: c_int, buf: [*]u8, len: usize) isize;
    extern "c" fn waitpid(pid: c_int, status: ?*c_int, options: c_int) c_int;
    extern "c" fn execvp(file: [*:0]const u8, argv: [*:null]const ?[*:0]const u8) c_int;
    extern "c" fn chdir(path: [*:0]const u8) c_int;
    extern "c" fn _exit(code: c_int) noreturn;
    extern "c" fn mkdir(path: [*:0]const u8, mode: c_uint) c_int;
    extern "c" fn access(path: [*:0]const u8, mode: c_int) c_int;
    extern "c" fn open(path: [*:0]const u8, flags: c_int, ...) c_int;
};

const F_OK: c_int = 0;

/// Runs a shell command and returns stdout (bounded). Mirrors the shell
/// passthrough pattern used across the services.
pub fn runShell(allocator: std.mem.Allocator, command: []const u8, cwd: []const u8) struct { stdout: []const u8, exit_code: i32 } {
    var pipe_fds: [2]c_int = undefined;
    if (c.pipe(&pipe_fds) != 0) return .{ .stdout = "", .exit_code = -1 };
    const pid = c.fork();
    if (pid < 0) {
        _ = c.close(pipe_fds[0]);
        _ = c.close(pipe_fds[1]);
        return .{ .stdout = "", .exit_code = -1 };
    }
    if (pid == 0) {
        _ = c.close(pipe_fds[0]);
        _ = c.dup2(pipe_fds[1], 1);
        _ = c.dup2(pipe_fds[1], 2);
        _ = c.close(pipe_fds[1]);
        if (cwd.len > 0) {
            const cwd_z = allocator.dupeZ(u8, cwd) catch c._exit(127);
            _ = c.chdir(cwd_z.ptr);
        }
        const sh_z = allocator.dupeZ(u8, "/bin/sh") catch c._exit(127);
        const flag_z = allocator.dupeZ(u8, "-c") catch c._exit(127);
        const cmd_z = allocator.dupeZ(u8, command) catch c._exit(127);
        const args = [_]?[*:0]const u8{ sh_z.ptr, flag_z.ptr, cmd_z.ptr, null };
        _ = c.execvp(sh_z.ptr, @ptrCast(&args));
        c._exit(127);
    }
    _ = c.close(pipe_fds[1]);
    var out = std.ArrayList(u8).empty;
    defer out.deinit(allocator);
    var buf: [8192]u8 = undefined;
    while (true) {
        const n = c.read(pipe_fds[0], &buf, buf.len);
        if (n <= 0) break;
        out.appendSlice(allocator, buf[0..@intCast(n)]) catch {};
        if (out.items.len > 2 * 1024 * 1024) break;
    }
    _ = c.close(pipe_fds[0]);
    var status: c_int = 0;
    _ = c.waitpid(pid, &status, 0);
    const status_u: u32 = @bitCast(status);
    const code: i32 = if (std.posix.W.IFEXITED(status_u)) std.posix.W.EXITSTATUS(status_u) else -1;
    return .{ .stdout = out.toOwnedSlice(allocator) catch "", .exit_code = code };
}

fn workspaceRoot(ctx: *svc.Call) []const u8 {
    const env_map = ctx.app.env_map;
    if (env_map.get("PWD")) |pwd| return pwd;
    return ".";
}

fn getStr(obj: *const std.json.ObjectMap, key: []const u8) []const u8 {
    if (obj.get(key)) |v| {
        if (v == .string) return v.string;
    }
    return "";
}

// ============================================================================
// Git helpers
// ============================================================================

/// GitCommit — { repoPath, message, amend } → "ok".
pub fn gitCommit(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct { repoPath: []const u8 = "", message: []const u8 = "", amend: ?bool = null };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid payload");
        return "";
    };
    defer parsed.deinit();
    const repo = if (parsed.value.repoPath.len > 0) parsed.value.repoPath else workspaceRoot(ctx);
    const amend_flag = if (parsed.value.amend orelse false) " --amend" else "";
    const msg = std.fmt.allocPrint(allocator, "git commit{s} -m {s}", .{ amend_flag, parsed.value.message }) catch {
        try svc.failCtx(ctx, "alloc failed");
        return "";
    };
    defer allocator.free(msg);
    const res = runShell(allocator, msg, repo);
    defer allocator.free(res.stdout);
    if (res.exit_code != 0) {
        try svc.failCtx(ctx, "git commit failed");
        return "";
    }
    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value("ok", .{}, &out.writer);
    return out.toOwnedSlice();
}

/// GitPush — { repoPath, force } → output.
pub fn gitPush(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct { repoPath: []const u8 = "", force: ?bool = null };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid payload");
        return "";
    };
    defer parsed.deinit();
    const repo = if (parsed.value.repoPath.len > 0) parsed.value.repoPath else workspaceRoot(ctx);
    const force_flag = if (parsed.value.force orelse false) " --force" else "";
    const cmd = std.fmt.allocPrint(allocator, "git push{s}", .{force_flag}) catch {
        try svc.failCtx(ctx, "alloc failed");
        return "";
    };
    defer allocator.free(cmd);
    const res = runShell(allocator, cmd, repo);
    defer allocator.free(res.stdout);
    if (res.exit_code != 0) {
        try svc.failCtx(ctx, "git push failed");
        return "";
    }
    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(res.stdout, .{}, &out.writer);
    return out.toOwnedSlice();
}

/// GitFetch — { repoPath } → output.
pub fn gitFetch(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct { repoPath: []const u8 = "" };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid payload");
        return "";
    };
    defer parsed.deinit();
    const repo = if (parsed.value.repoPath.len > 0) parsed.value.repoPath else workspaceRoot(ctx);
    const res = runShell(allocator, "git fetch", repo);
    defer allocator.free(res.stdout);
    if (res.exit_code != 0) {
        try svc.failCtx(ctx, "git fetch failed");
        return "";
    }
    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(res.stdout, .{}, &out.writer);
    return out.toOwnedSlice();
}

/// GitMerge — { repoPath, source, noFF, squash } → output.
pub fn gitMerge(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct { repoPath: []const u8 = "", source: []const u8 = "", noFF: ?bool = null, squash: ?bool = null };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid payload");
        return "";
    };
    defer parsed.deinit();
    const repo = if (parsed.value.repoPath.len > 0) parsed.value.repoPath else workspaceRoot(ctx);
    const noff = if (parsed.value.noFF orelse false) " --no-ff" else "";
    const squash = if (parsed.value.squash orelse false) " --squash" else "";
    const cmd = std.fmt.allocPrint(allocator, "git merge{s}{s} {s}", .{ noff, squash, parsed.value.source }) catch {
        try svc.failCtx(ctx, "alloc failed");
        return "";
    };
    defer allocator.free(cmd);
    const res = runShell(allocator, cmd, repo);
    defer allocator.free(res.stdout);
    if (res.exit_code != 0) {
        try svc.failCtx(ctx, "git merge failed");
        return "";
    }
    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(res.stdout, .{}, &out.writer);
    return out.toOwnedSlice();
}

// ============================================================================
// Editor helpers
// ============================================================================

/// GetCompletion — { prefix, path } → keyword/snippet completions.
pub fn getCompletion(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct { prefix: []const u8 = "", path: []const u8 = "" };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid payload");
        return "";
    };
    defer parsed.deinit();
    const prefix = parsed.value.prefix;
    const Item = struct { Name: []const u8, Kind: []const u8, Detail: []const u8 };
    const items = [_]Item{
        .{ .Name = "console.log", .Kind = "snippet", .Detail = "console.log(...)" },
        .{ .Name = "function", .Kind = "keyword", .Detail = "function declaration" },
        .{ .Name = "import", .Kind = "keyword", .Detail = "import statement" },
        .{ .Name = "export", .Kind = "keyword", .Detail = "export statement" },
        .{ .Name = "interface", .Kind = "keyword", .Detail = "interface declaration" },
        .{ .Name = "const", .Kind = "keyword", .Detail = "const declaration" },
        .{ .Name = "let", .Kind = "keyword", .Detail = "let declaration" },
        .{ .Name = "return", .Kind = "keyword", .Detail = "return statement" },
    };
    var list = std.ArrayList(Item).empty;
    defer list.deinit(allocator);
    for (items) |it| {
        if (std.mem.startsWith(u8, it.Name, prefix)) list.append(allocator, it) catch {};
    }
    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(list.items, .{}, &out.writer);
    return out.toOwnedSlice();
}

/// GetMembers — { instance, path } → common JS members.
pub fn getMembers(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const items = [_]struct { Name: []const u8, Kind: []const u8, Detail: []const u8 }{
        .{ .Name = "length", .Kind = "property", .Detail = "number" },
        .{ .Name = "toString", .Kind = "method", .Detail = "(): string" },
        .{ .Name = "map", .Kind = "method", .Detail = "(fn) => []" },
        .{ .Name = "filter", .Kind = "method", .Detail = "(fn) => []" },
        .{ .Name = "forEach", .Kind = "method", .Detail = "(fn) => void" },
        .{ .Name = "find", .Kind = "method", .Detail = "(fn) => T | undefined" },
    };
    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(items, .{}, &out.writer);
    return out.toOwnedSlice();
}

/// ListDirectory — { path } → decorated tree JSON string (the explorer
/// consumes the decorated nodes with gitIgnored/hidden flags).
pub fn listDirectory(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct { path: []const u8 = "" };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid payload");
        return "";
    };
    defer parsed.deinit();
    const dir = parsed.value.path;

    const Node = struct {
        path: []const u8,
        name: []const u8,
        isDir: bool,
        size: u64 = 0,
        modTime: i64 = 0,
        gitIgnored: bool = false,
        hidden: bool = false,
    };

    var nodes = std.ArrayList(Node).empty;
    defer nodes.deinit(allocator);

    const dir_z = allocator.dupeZ(u8, dir) catch {
        var out = std.Io.Writer.Allocating.init(allocator);
        try std.json.Stringify.value(@as([]Node, &.{}), .{}, &out.writer);
        return out.toOwnedSlice();
    };
    defer allocator.free(dir_z);

    const Dirent = extern struct {
        d_ino: u64,
        d_seekoff: u64,
        d_reclen: u16,
        d_namlen: u16,
        d_type: u8,
        d_name: [1024]u8,
    };
    const sys = struct {
        extern "c" fn opendir(dirname: [*:0]const u8) ?*anyopaque;
        extern "c" fn closedir(dirp: *anyopaque) c_int;
        extern "c" fn readdir(dirp: *anyopaque) ?*const Dirent;
    };
    const handle = sys.opendir(dir_z) orelse {
        var out = std.Io.Writer.Allocating.init(allocator);
        try std.json.Stringify.value(@as([]Node, &.{}), .{}, &out.writer);
        return out.toOwnedSlice();
    };
    defer _ = sys.closedir(handle);

    while (sys.readdir(handle)) |entry| {
        const name = std.mem.sliceTo(&entry.d_name, 0);
        if (name.len == 0) continue;
        if (std.mem.eql(u8, name, ".") or std.mem.eql(u8, name, "..")) continue;
        const is_dir = entry.d_type == 4; // DT_DIR
        const hidden = name.len > 0 and name[0] == '.';
        const full = std.fmt.allocPrint(allocator, "{s}/{s}", .{ dir, name }) catch continue;
        nodes.append(allocator, .{
            .path = full,
            .name = allocator.dupe(u8, name) catch "",
            .isDir = is_dir,
            .hidden = hidden,
        }) catch {};
    }

    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(nodes.items, .{}, &out.writer);
    return out.toOwnedSlice();
}

/// ReadFileBase64 — { path } → base64 string.
pub fn readFileBase64(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct { path: []const u8 = "" };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid payload");
        return "";
    };
    defer parsed.deinit();
    const raw = svc.readFileBounded(allocator, parsed.value.path) catch {
        try svc.failCtx(ctx, "read failed");
        return "";
    };
    defer allocator.free(raw);
    const encoder = std.base64.standard.Encoder;
    const encoded = allocator.alloc(u8, encoder.calcSize(raw.len)) catch {
        try svc.failCtx(ctx, "alloc failed");
        return "";
    };
    defer allocator.free(encoded);
    _ = encoder.encode(encoded, raw);
    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(encoded, .{}, &out.writer);
    return out.toOwnedSlice();
}

// ============================================================================
// Filesystem helpers
// ============================================================================

/// CreateFile — { path } → ok.
pub fn createFile(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct { path: []const u8 = "" };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid payload");
        return "";
    };
    defer parsed.deinit();
    const path = parsed.value.path;
    if (path.len == 0) {
        try svc.failCtx(ctx, "path required");
        return "";
    }
    const z = allocator.dupeZ(u8, path) catch {
        try svc.failCtx(ctx, "alloc failed");
        return "";
    };
    defer allocator.free(z);
    const o_wronly_creat: c_int = switch (@import("builtin").os.tag) {
        .macos => 0x0601,
        else => 0x0241,
    };
    const fd = c.open(z, o_wronly_creat, @as(c_uint, 0o644));
    if (fd < 0) {
        try svc.failCtx(ctx, "create failed");
        return "";
    }
    _ = c.close(fd);
    try svc.okCtx(ctx);
    return "";
}

/// CreateFolder — { path } → ok.
pub fn createFolder(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct { path: []const u8 = "" };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid payload");
        return "";
    };
    defer parsed.deinit();
    const path = parsed.value.path;
    if (path.len == 0) {
        try svc.failCtx(ctx, "path required");
        return "";
    }
    const z = allocator.dupeZ(u8, path) catch {
        try svc.failCtx(ctx, "alloc failed");
        return "";
    };
    defer allocator.free(z);
    _ = c.mkdir(z, 0o755); // EEXIST fine
    try svc.okCtx(ctx);
    return "";
}

// ============================================================================
// Slash commands
// ============================================================================

/// ListSlashCommands — { query } → known slash commands.
pub fn listSlashCommands(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const commands = [_]struct { name: []const u8, description: []const u8, kind: []const u8 }{
        .{ .name = "yolo", .description = "Auto-approve all tool calls for this turn", .kind = "toggle" },
        .{ .name = "clear", .description = "Clear the current session transcript", .kind = "action" },
        .{ .name = "compact", .description = "Summarize older messages to save context", .kind = "action" },
    };
    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(commands, .{}, &out.writer);
    return out.toOwnedSlice();
}

/// ExecuteSlashCommand — { sessionId, text } → { handled, message }.
pub fn executeSlashCommand(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct { sessionId: ?[]const u8 = null, text: []const u8 = "" };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid payload");
        return "";
    };
    defer parsed.deinit();
    const text = parsed.value.text;
    var out = std.Io.Writer.Allocating.init(allocator);
    if (std.mem.startsWith(u8, text, "/clear")) {
        try std.json.Stringify.value(.{ .handled = true, .message = "cleared" }, .{}, &out.writer);
    } else if (std.mem.startsWith(u8, text, "/yolo")) {
        try std.json.Stringify.value(.{ .handled = true, .message = "yolo mode enabled" }, .{}, &out.writer);
    } else {
        // Forward to the agent as a normal message.
        try std.json.Stringify.value(.{ .handled = false }, .{}, &out.writer);
    }
    return out.toOwnedSlice();
}

// ============================================================================
// Misc
// ============================================================================

/// OpenExternalURL — { url } → ok (native openUrl handled on the frontend;
/// this is the loopback fallback).
pub fn openExternalUrl(ctx: *svc.Call) anyerror![]const u8 {
    try svc.okCtx(ctx);
    return "";
}

/// SaveLLMProfile — { providerId, apiKey, baseURL, model } → ok.
///
/// Mutation safety: models.json is parsed leaky into a local arena and every
/// ObjectMap.put uses that arena's allocator — mutating arena-owned maps with
/// the c_allocator frees arena memory (malloc crash).
pub fn saveLlmProfile(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct { providerId: []const u8 = "", apiKey: []const u8 = "", baseURL: []const u8 = "", model: []const u8 = "" };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid payload");
        return "";
    };
    defer parsed.deinit();

    // Persist into models.json (same shape as saveProfiles: providers map).
    const path = @import("config.zig").modelsPath(ctx.app.env_map, allocator);
    defer allocator.free(path);
    const raw = svc.readFileBounded(allocator, path) catch "";
    defer allocator.free(raw);

    var arena = std.heap.ArenaAllocator.init(allocator);
    defer arena.deinit();
    const a = arena.allocator();
    var root = std.json.parseFromSliceLeaky(std.json.Value, a, raw, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "models.json unparsable");
        return "";
    };
    switch (root) {
        .object => |*root_obj| {
            var providers_obj: std.json.ObjectMap = .empty;
            if (root_obj.getPtr("providers")) |pv| {
                if (pv.* == .object) providers_obj = pv.object;
            }
            var merged = std.json.ObjectMap.empty;
            if (providers_obj.getPtr(parsed.value.providerId)) |ep| {
                if (ep.* == .object) merged = ep.object;
            }
            if (parsed.value.apiKey.len > 0) merged.put(a, "api_key", .{ .string = parsed.value.apiKey }) catch {};
            if (parsed.value.baseURL.len > 0) merged.put(a, "base_url", .{ .string = parsed.value.baseURL }) catch {};
            if (parsed.value.model.len > 0) merged.put(a, "active_model", .{ .string = parsed.value.model }) catch {};
            if (parsed.value.model.len > 0) merged.put(a, "id", .{ .string = parsed.value.providerId }) catch {};
            merged.put(a, "name", .{ .string = parsed.value.providerId }) catch {};
            merged.put(a, "api", .{ .string = "openai-completions" }) catch {};
            providers_obj.put(a, parsed.value.providerId, .{ .object = merged }) catch {};
            root_obj.put(a, "providers", .{ .object = providers_obj }) catch {};
            root_obj.put(a, "default_provider", .{ .string = parsed.value.providerId }) catch {};
        },
        else => {},
    }
    try svc.writeJsonFile(allocator, path, root);
    try svc.okCtx(ctx);
    return "";
}

/// RespondAgentAsk — { id, answers } → ok (bootstrap: no pending ask).
pub fn respondAgentAsk(ctx: *svc.Call) anyerror![]const u8 {
    try svc.okCtx(ctx);
    return "";
}

/// GenerateAICommitMessage — { repoPath, providerId, model, instruction } →
/// a suggested commit message via the active LLM provider.
pub fn generateAiCommitMessage(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct { repoPath: []const u8 = "", providerId: ?[]const u8 = null, model: ?[]const u8 = null, instruction: ?[]const u8 = null };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid payload");
        return "";
    };
    defer parsed.deinit();
    const repo = if (parsed.value.repoPath.len > 0) parsed.value.repoPath else workspaceRoot(ctx);

    // Collect staged diff summary.
    const diff_res = runShell(allocator, "git diff --cached --stat", repo);
    defer allocator.free(diff_res.stdout);
    const status_res = runShell(allocator, "git status --porcelain=v1", repo);
    defer allocator.free(status_res.stdout);

    // Build a compact context for the LLM.
    const ctx_str = std.fmt.allocPrint(
        allocator,
        "Staged changes:\n{s}\n\nWorking tree:\n{s}",
        .{ diff_res.stdout, status_res.stdout },
    ) catch {
        try svc.failCtx(ctx, "alloc failed");
        return "";
    };
    defer allocator.free(ctx_str);

    const agent_zig = @import("agent.zig");
    const llm_client = @import("llm-client.zig");
    const target = agent_zig.loadActiveTargetPublic(ctx, allocator) orelse {
        try svc.failCtx(ctx, "no active LLM provider configured");
        return "";
    };
    defer {
        allocator.free(target.providerId);
        allocator.free(target.baseURL);
        allocator.free(target.apiKey);
        allocator.free(target.model);
    }
    const model = parsed.value.model orelse target.model;

    var msgs = std.ArrayList(llm_client.LLMMessage).empty;
    defer msgs.deinit(allocator);
    msgs.append(allocator, .{
        .role = "system",
        .content = "You write concise conventional git commit messages. Output ONLY the commit message, no explanation, no quotes.",
    }) catch {};
    const user_prompt = std.fmt.allocPrint(
        allocator,
        "Write a commit message for:\n{s}\n\nInstruction: {s}",
        .{ ctx_str, parsed.value.instruction orelse "Follow conventional commits (feat/fix/refactor/etc)." },
    ) catch {
        try svc.failCtx(ctx, "alloc failed");
        return "";
    };
    defer allocator.free(user_prompt);
    msgs.append(allocator, .{ .role = "user", .content = user_prompt }) catch {};

    var target2 = target;
    target2.model = model;
    var abort = std.atomic.Value(bool).init(false);
    const result = llm_client.streamChat(allocator, &target2, msgs.items, &.{}, .{}, &abort) catch |err| {
        const err_msg = std.fmt.allocPrint(allocator, "LLM call failed: {s}", .{@errorName(err)}) catch "LLM call failed";
        defer allocator.free(err_msg);
        svc.failErr(ctx.responder, ctx.request_id, err) catch {};
        return "";
    };
    defer {
        allocator.free(result.content);
        if (result.reasoning.len > 0) allocator.free(result.reasoning);
        for (result.toolCalls) |tc| {
            allocator.free(tc.id);
            allocator.free(tc.name);
            allocator.free(tc.arguments);
        }
        if (result.toolCalls.len > 0) allocator.free(result.toolCalls);
    }
    const trimmed = std.mem.trim(u8, result.content, " \r\n\t\"'`");
    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(trimmed, .{}, &out.writer);
    return out.toOwnedSlice();
}
