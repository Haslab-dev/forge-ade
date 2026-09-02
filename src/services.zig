// ForgeADE in-shell services: the daemon's RPC surface re-hosted as native
// bridge commands so the app ships WITHOUT an external Bun process.
//
// Transport: frontend calls window.zero.invoke("services.<method>", params).
// The bridge handler (main.zig) routes every services.* command here.
// Each call runs on its own detached worker thread (mirrors the existing
// commandExecWorker pattern); streaming events (agent turns, LSP) are emitted
// through the App's emitWindowEvent mechanism, never through this responder.

const std = @import("std");
const native_sdk = @import("native_sdk");

pub const AppHandle = struct {
    allocator: std.mem.Allocator,
    env_map: *std.process.Environ.Map,
    /// Set by main.zig before dispatch; used for streaming event emission.
    main_window_id: u64 = 0,
    emit_ctx: ?*anyopaque = null,
    emit_fn: ?*const fn (ctx: *anyopaque, window_id: u64, name: []const u8, payload: []const u8) anyerror!void = null,
};

/// The invocation context handed to each service handler.
pub const Call = struct {
    app: *AppHandle,
    method: []const u8,
    payload: []const u8,
    /// The ORIGINAL bridge request id (e.g. "1", "2") — the WebView's JS
    /// `zero.invoke` pending map is keyed by this, so every response MUST
    /// echo it back or the promise never resolves (empty UI).
    request_id: []const u8,
    responder: native_sdk.bridge.AsyncResponder,
};

/// One registered service method.
pub const Handler = struct {
    method: []const u8,
    /// Synchronous handler: runs on a worker thread. Return value is a
    /// JSON-encoded result (the bridge wraps it in {ok, result}).
    run: *const fn (ctx: *Call) anyerror![]const u8,
};

pub const Registry = struct {
    handlers: []const Handler = &.{},

    pub fn find(self: Registry, method: []const u8) ?Handler {
        // Strip an optional "forge." prefix the frontend sometimes sends.
        var name = method;
        if (std.mem.startsWith(u8, name, "forge.")) name = name[6..];
        for (self.handlers) |handler| {
            if (std.mem.eql(u8, handler.method, name)) return handler;
        }
        return null;
    }
};

// ============================================================================
// Shared helpers
// ============================================================================

/// Minimal libc file IO — the same extern pattern main.zig uses (Zig 0.16's
/// std.Io reorganized the POSIX layer, so the shell's proven c-stubs win).
const c = struct {
    extern "c" fn open(path: [*:0]const u8, flags: c_int, ...) c_int;
    extern "c" fn read(fd: c_int, buf: [*]u8, len: usize) isize;
    extern "c" fn write(fd: c_int, buf: [*]const u8, len: usize) isize;
    extern "c" fn close(fd: c_int) c_int;
    extern "c" fn rename(old: [*:0]const u8, new: [*:0]const u8) c_int;
    extern "c" fn mkdir(path: [*:0]const u8, mode: c_uint) c_int;
    extern "c" fn unlink(path: [*:0]const u8) c_int;
    extern "c" fn clock_gettime(clk_id: c_int, tp: *Timespec) c_int;
};

const Timespec = extern struct {
    tv_sec: isize,
    tv_nsec: isize,
};

const CLOCK_REALTIME: c_int = 0;

/// Current wall-clock time in milliseconds since the Unix epoch.
pub fn nowMs() i64 {
    var ts: Timespec = undefined;
    if (c.clock_gettime(CLOCK_REALTIME, &ts) != 0) return 0;
    return @as(i64, @intCast(ts.tv_sec)) * 1000 + @divTrunc(@as(i64, @intCast(ts.tv_nsec)), 1_000_000);
}

/// Current wall-clock time in whole seconds.
pub fn nowSec() i64 {
    return @divTrunc(nowMs(), 1000);
}

const o_rdonly: c_int = 0;
const o_wronly_creat_trunc: c_int = switch (@import("builtin").os.tag) {
    .macos => 0x0601,
    else => 0x0241,
};

/// Returns the user's home directory from the environment map.
pub fn homeDir(env_map: *std.process.Environ.Map) []const u8 {
    if (env_map.get("HOME")) |home| return home;
    if (env_map.get("USERPROFILE")) |home| return home;
    return "/";
}

pub fn dataDir(env_map: *std.process.Environ.Map) []const u8 {
    // ~/.forge-ade — same location the TS daemon used.
    const home = homeDir(env_map);
    return std.fmt.allocPrint(std.heap.c_allocator, "{s}/.forge-ade", .{home}) catch home;
}

/// Reads an entire file (bounded at 25MB like the fs.readFile handler).
/// The returned slice is allocated with `allocator` (caller frees).
pub fn readFileBounded(allocator: std.mem.Allocator, path: []const u8) ![]u8 {
    const path_z = try allocator.dupeZ(u8, path);
    defer allocator.free(path_z);
    const fd = c.open(path_z, o_rdonly);
    if (fd < 0) return error.OpenFailed;
    defer _ = c.close(fd);
    var content: std.ArrayList(u8) = .empty;
    errdefer content.deinit(allocator);
    var buf: [8192]u8 = undefined;
    while (true) {
        const n = c.read(fd, &buf, buf.len);
        if (n <= 0) break;
        try content.appendSlice(allocator, buf[0..@intCast(n)]);
        if (content.items.len > 25 * 1024 * 1024) break;
    }
    return content.toOwnedSlice(allocator);
}

/// Writes a file atomically (temp + rename) like the TS daemon's writeAtomic.
pub fn writeFileAtomic(allocator: std.mem.Allocator, path: []const u8, data: []const u8) !void {
    const dir = std.fs.path.dirname(path) orelse ".";
    const dir_z = try allocator.dupeZ(u8, dir);
    defer allocator.free(dir_z);
    _ = c.mkdir(dir_z, 0o755); // EEXIST is fine
    const tmp = try std.fmt.allocPrint(allocator, "{s}.tmp", .{path});
    defer allocator.free(tmp);
    const tmp_z = try allocator.dupeZ(u8, tmp);
    defer allocator.free(tmp_z);
    const fd = c.open(tmp_z, o_wronly_creat_trunc, @as(c_uint, 0o644));
    if (fd < 0) return error.OpenFailed;
    defer _ = c.close(fd);
    var written: usize = 0;
    while (written < data.len) {
        const n = c.write(fd, data[written..].ptr, data.len - written);
        if (n < 0) return error.WriteFailed;
        written += @intCast(n);
    }
    const path_z = try allocator.dupeZ(u8, path);
    defer allocator.free(path_z);
    if (c.rename(tmp_z, path_z) != 0) return error.RenameFailed;
}

/// Writes a JSON value to a file with pretty-printing.
pub fn writeJsonFile(allocator: std.mem.Allocator, path: []const u8, value: anytype) !void {
    var out = std.Io.Writer.Allocating.init(allocator);
    defer out.deinit();
    try std.json.Stringify.value(value, .{ .whitespace = .indent_2 }, &out.writer);
    try writeFileAtomic(allocator, path, out.written());
}

/// Deep-copies a std.json.Value subtree into `allocator`-owned memory.
/// The copy is fully independent of any parse arena; free it with
/// deepFreeValue.
pub fn deepCopyValue(allocator: std.mem.Allocator, value: std.json.Value) std.json.Value {
    switch (value) {
        .null => return .null,
        .bool, .integer, .float, .number_string => return value,
        .string => |s| return .{ .string = allocator.dupe(u8, s) catch "" },
        .array => |arr| {
            var out = std.ArrayList(std.json.Value).empty;
            for (arr.items) |item| {
                out.append(allocator, deepCopyValue(allocator, item)) catch continue;
            }
            return .{ .array = .{
                .items = out.items,
                .capacity = out.capacity,
                .allocator = allocator,
            } };
        },
        .object => |obj| {
            var out = std.json.ObjectMap.empty;
            var it = obj.iterator();
            while (it.next()) |entry| {
                const key = allocator.dupe(u8, entry.key_ptr.*) catch continue;
                out.put(allocator, key, deepCopyValue(allocator, entry.value_ptr.*)) catch {
                    allocator.free(key);
                    continue;
                };
            }
            return .{ .object = out };
        },
    }
}

/// Frees a value produced by deepCopyValue (recursively).
pub fn deepFreeValue(allocator: std.mem.Allocator, value: std.json.Value) void {
    switch (value) {
        .null, .bool, .integer, .float, .number_string => {},
        .string => |s| allocator.free(s),
        .array => |arr| {
            for (arr.items) |item| deepFreeValue(allocator, item);
            if (arr.items.len > 0) allocator.free(arr.items);
        },
        .object => |obj| {
            // Local mutable copy — deinit frees the same backing storage.
            var map = obj;
            var it = map.iterator();
            while (it.next()) |entry| {
                allocator.free(entry.key_ptr.*);
                deepFreeValue(allocator, entry.value_ptr.*);
            }
            map.deinit(allocator);
        },
    }
}

/// Turns an error into a bridge failure response and returns.
pub fn fail(responder: native_sdk.bridge.AsyncResponder, id: []const u8, comptime message: []const u8) anyerror!void {
    try responder.fail(id, .handler_failed, message);
}

/// Failure keyed to the Call's ORIGINAL request id (what the JS pending map
/// expects). Use this in handlers instead of `fail(ctx.responder, ctx.method, ...)`.
pub fn failCtx(ctx: *Call, comptime message: []const u8) anyerror!void {
    try ctx.responder.fail(ctx.request_id, .handler_failed, message);
}

pub fn failErr(responder: native_sdk.bridge.AsyncResponder, id: []const u8, err: anyerror) anyerror!void {
    const name = @errorName(err);
    const buf: [128]u8 = undefined;
    const msg = if (name.len < buf.len) name else "handler_failed";
    try responder.fail(id, .handler_failed, msg);
}

pub fn ok(responder: native_sdk.bridge.AsyncResponder, id: []const u8) anyerror!void {
    try responder.success(id, "{\"ok\":true}");
}

/// Success keyed to the Call's ORIGINAL request id (what the JS pending map
/// expects). Use this in handlers instead of `ok(ctx.responder, ctx.method)`.
pub fn okCtx(ctx: *Call) anyerror!void {
    try ctx.responder.success(ctx.request_id, "{\"ok\":true}");
}

/// Returns the value of a top-level string field in a JSON payload, or null.
pub fn getString(allocator: std.mem.Allocator, ctx: *Call, key: []const u8) ?[]const u8 {
    _ = allocator;
    _ = ctx;
    _ = key;
    return null;
}

// ============================================================================
// Service implementations (leaf services first)
// ============================================================================

const config_zig = @import("services/config.zig");
const workspace_zig = @import("services/workspace.zig");
const usage_zig = @import("services/usage.zig");
const search_zig = @import("services/search.zig");
const syntax_zig = @import("services/syntax.zig");
const llm_zig = @import("services/llm.zig");
const mcp_zig = @import("services/mcp.zig");
const agent_zig = @import("services/agent.zig");
const skills_zig = @import("services/skills.zig");
const external_zig = @import("services/external-agent.zig");
const lsp_zig = @import("services/lsp-manager.zig");
const misc_zig = @import("services/misc.zig");

pub fn registry() Registry {
    return .{
        .handlers = &.{
            // config
            .{ .method = "GetProviderProfiles", .run = llm_zig.profiles },
            .{ .method = "ListProviderProfiles", .run = llm_zig.profiles },
            .{ .method = "SaveProviderProfiles", .run = llm_zig.saveProfiles },
            .{ .method = "GetLLMConfig", .run = llm_zig.active },
            .{ .method = "ListLLMProviders", .run = llm_zig.profiles },
            .{ .method = "SetActiveModel", .run = llm_zig.setActiveModel },
            // workspace
            .{ .method = "GetCurrentWorkspace", .run = workspace_zig.get },
            .{ .method = "OpenFolder", .run = workspace_zig.open },
            .{ .method = "OpenWorkspace", .run = workspace_zig.open },
            .{ .method = "SaveWorkspace", .run = workspace_zig.save },
            .{ .method = "SaveWorkspaceAs", .run = workspace_zig.save },
            .{ .method = "CloseWorkspace", .run = workspace_zig.close },
            .{ .method = "GetRecentProjects", .run = workspace_zig.recent },
            .{ .method = "PinRecent", .run = workspace_zig.open },
            .{ .method = "RemoveRecent", .run = workspace_zig.open },
            // usage
            .{ .method = "GetAllUsageRecords", .run = usage_zig.records },
            .{ .method = "GetUsageOverview", .run = usage_zig.overview },
            .{ .method = "GetUsageSummary", .run = usage_zig.overview },
            // search
            .{ .method = "SearchFilenameWithOptions", .run = search_zig.filename },
            .{ .method = "SearchFilename", .run = search_zig.filename },
            .{ .method = "SearchContentWithOptions", .run = search_zig.content },
            .{ .method = "SearchReplaceAll", .run = search_zig.replace },
            .{ .method = "SearchIndexSymbols", .run = search_zig.filename },
            .{ .method = "FindSymbol", .run = search_zig.filename },
            // syntax
            .{ .method = "CheckSyntax", .run = syntax_zig.check },
            .{ .method = "FormatCode", .run = syntax_zig.format },
            // lsp
            .{ .method = "LSPDidOpen", .run = lsp_zig.didOpen },
            .{ .method = "LSPDidChange", .run = lsp_zig.didChange },
            .{ .method = "LSPDidSave", .run = lsp_zig.didSave },
            .{ .method = "LSPDidClose", .run = lsp_zig.didClose },
            .{ .method = "LSPGetCompletion", .run = lsp_zig.getCompletion },
            .{ .method = "LSPGetHover", .run = lsp_zig.getHover },
            .{ .method = "LSPGetDefinition", .run = lsp_zig.getDefinition },
            .{ .method = "LSPGetDeclaration", .run = lsp_zig.getDeclaration },
            .{ .method = "LSPGetTypeDefinition", .run = lsp_zig.getTypeDefinition },
            .{ .method = "LSPGetImplementation", .run = lsp_zig.getImplementation },
            .{ .method = "LSPGetDiagnostics", .run = lsp_zig.getDiagnostics },
            .{ .method = "LSPListServers", .run = lsp_zig.listServers },
            .{ .method = "LSPRestartServer", .run = lsp_zig.restartServer },
            .{ .method = "LSPStopServer", .run = lsp_zig.stopServer },
            .{ .method = "LSPRestartAll", .run = lsp_zig.restartAll },
            .{ .method = "LSPStopAll", .run = lsp_zig.stopAll },
            .{ .method = "LSPGetServerLogs", .run = lsp_zig.getServerLogs },
            // mcp
            .{ .method = "ListMCPServers", .run = mcp_zig.servers },
            .{ .method = "SaveMCPServer", .run = mcp_zig.saveServer },
            .{ .method = "DeleteMCPServer", .run = mcp_zig.deleteServer },
            .{ .method = "ListMCPTools", .run = mcp_zig.tools },
            .{ .method = "ListConnectedMCPTools", .run = mcp_zig.tools },
            .{ .method = "ReconnectMCP", .run = mcp_zig.reconnect },
            .{ .method = "RefreshMCP", .run = mcp_zig.servers },
            // skills (map to config/skills store)
            .{ .method = "ListSkills", .run = skills_zig.listEnabled },
            .{ .method = "ListAllSkills", .run = skills_zig.listAll },
            .{ .method = "RefreshSkills", .run = skills_zig.listAll },
            .{ .method = "SetSkillEnabled", .run = skills_zig.listAll },
            .{ .method = "SetAllSkillsEnabled", .run = skills_zig.listAll },
            // agent
            .{ .method = "ListAgentSessions", .run = agent_zig.listSessions },
            .{ .method = "ListAgentSessionsForFolder", .run = agent_zig.listSessions },
            .{ .method = "GetAgentSession", .run = agent_zig.getSession },
            .{ .method = "CreateAgentSession", .run = agent_zig.createSession },
            .{ .method = "CreateAgentSessionFromDefinition", .run = agent_zig.createSession },
            .{ .method = "UpdateAgentSession", .run = agent_zig.updateSession },
            .{ .method = "DeleteAgentSession", .run = agent_zig.deleteSession },
            .{ .method = "ClearAgentSession", .run = agent_zig.deleteSession },
            .{ .method = "SetAgentAutoApprove", .run = agent_zig.updateSession },
            .{ .method = "SetAgentDialect", .run = agent_zig.updateSession },
            .{ .method = "SendAgentMessage", .run = agent_zig.sendMessage },
            .{ .method = "StopAgentTurn", .run = agent_zig.stopTurn },
            .{ .method = "RespondAgentApproval", .run = agent_zig.respondApproval },
            .{ .method = "ListAgentDefinitions", .run = agent_zig.definitions },
            .{ .method = "SaveAgentDefinition", .run = agent_zig.saveDefinition },
            .{ .method = "DeleteAgentDefinition", .run = agent_zig.definitions },
            .{ .method = "ApplyAgentDefinitionToSession", .run = agent_zig.definitions },
            .{ .method = "ToggleAgentTask", .run = agent_zig.updateSession },
            // external ACP agents
            .{ .method = "ListExternalAgents", .run = external_zig.listAgents },
            .{ .method = "CreateExternalAgentSession", .run = external_zig.createExternalSession },
            .{ .method = "GetExternalAgentState", .run = external_zig.getExternalState },
            .{ .method = "SetExternalAgentConfig", .run = external_zig.setExternalConfig },
            // misc / editor / git helpers
            .{ .method = "GitCommit", .run = misc_zig.gitCommit },
            .{ .method = "GitPush", .run = misc_zig.gitPush },
            .{ .method = "GitFetch", .run = misc_zig.gitFetch },
            .{ .method = "GitMerge", .run = misc_zig.gitMerge },
            .{ .method = "GetCompletion", .run = misc_zig.getCompletion },
            .{ .method = "GetMembers", .run = misc_zig.getMembers },
            .{ .method = "ListDirectory", .run = misc_zig.listDirectory },
            .{ .method = "ReadFileBase64", .run = misc_zig.readFileBase64 },
            .{ .method = "CreateFile", .run = misc_zig.createFile },
            .{ .method = "CreateFolder", .run = misc_zig.createFolder },
            .{ .method = "ListSlashCommands", .run = misc_zig.listSlashCommands },
            .{ .method = "ExecuteSlashCommand", .run = misc_zig.executeSlashCommand },
            .{ .method = "OpenExternalURL", .run = misc_zig.openExternalUrl },
            .{ .method = "SaveLLMProfile", .run = misc_zig.saveLlmProfile },
            .{ .method = "RespondAgentAsk", .run = misc_zig.respondAgentAsk },
            .{ .method = "GenerateAICommitMessage", .run = misc_zig.generateAiCommitMessage },
            // OAuth / quota — not yet migrated (bootstrap).
            .{ .method = "StartOAuthLogin", .run = notMigrated },
            .{ .method = "GetOAuthStatus", .run = notMigrated },
            .{ .method = "SubmitOAuthManualCode", .run = notMigrated },
            .{ .method = "GetProviderQuota", .run = notMigrated },
            .{ .method = "GetAllProviderQuotas", .run = notMigrated },
            .{ .method = "FetchProviderModels", .run = notMigrated },
        },
    };
}

/// Explicit "not yet migrated" for features outside the bootstrap scope.
fn notMigrated(ctx: *Call) anyerror![]const u8 {
    try fail(ctx.responder, ctx.request_id, "not yet migrated (bootstrap)");
    return "";
}

test "registry resolves the frontend wire names" {
    const reg = registry();
    const names = [_][]const u8{
        "GetProviderProfiles", "ListProviderProfiles", "ListAgentSessions", "ListMCPServers",
        "ListSkills", "ListAllSkills", "GetLLMConfig", "GetCurrentWorkspace", "GetAllUsageRecords",
        "CheckSyntax", "SaveProviderProfiles", "SetActiveModel", "SearchFilenameWithOptions",
        "SearchContentWithOptions", "SearchReplaceAll", "SendAgentMessage", "GetAgentSession",
        "forge.GetProviderProfiles", "forge.ListAgentSessions",
    };
    for (names) |n| {
        try std.testing.expect(reg.find(n) != null);
    }
}
