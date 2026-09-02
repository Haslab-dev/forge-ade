// services.external-agent — external ACP agents (opencode, omp, pi, codex,
// claude-code, antigravity). Port of src/server/acp/registry.ts + manager.ts.
// External sessions are ordinary ForgeADE agent sessions with role
// `external:<agent-id>`; the ACP connection is spawned on demand.

const std = @import("std");
const svc = @import("../services.zig");
const acp = @import("acp-client.zig");

pub const ExternalAgentDef = struct {
    id: []const u8,
    name: []const u8,
    description: []const u8,
    command: []const u8,
    args: []const []const u8 = &.{},
};

const AGENTS = [_]ExternalAgentDef{
    .{ .id = "omp", .name = "Oh-My-Pi", .description = "Oh-My-Pi coding agent via native ACP mode (omp acp)", .command = "omp", .args = &.{"acp"} },
    .{ .id = "opencode", .name = "OpenCode", .description = "OpenCode agent via ACP (runs via npx, no install needed)", .command = "npx", .args = &.{ "-y", "opencode-ai", "acp" } },
    .{ .id = "codex", .name = "Codex", .description = "OpenAI Codex via the official codex-acp adapter (npx)", .command = "npx", .args = &.{ "-y", "@agentclientprotocol/codex-acp" } },
    .{ .id = "claude-code", .name = "Claude Code", .description = "Claude Agent SDK via the official claude-agent-acp adapter (npx)", .command = "npx", .args = &.{ "-y", "@agentclientprotocol/claude-agent-acp" } },
    .{ .id = "pi", .name = "Pi", .description = "Pi coding agent via the pi-acp adapter", .command = "pi-acp", .args = &.{} },
    .{ .id = "antigravity", .name = "Antigravity", .description = "Google Antigravity via the antigravity-acp bridge (npx; wraps agy)", .command = "npx", .args = &.{ "-y", "antigravity-acp" } },
};

pub fn findAgent(id: []const u8) ?ExternalAgentDef {
    for (AGENTS) |a| {
        if (std.mem.eql(u8, a.id, id)) return a;
    }
    return null;
}

// ============================================================================
// Connection cache (process-global, keyed by agent id)
// ============================================================================

const SpinLock = struct {
    locked: std.atomic.Value(bool) = std.atomic.Value(bool).init(false),
    fn lock(self: *SpinLock) void {
        while (self.locked.swap(true, .acquire)) std.atomic.spinLoopHint();
    }
    fn unlock(self: *SpinLock) void {
        self.locked.store(false, .release);
    }
};

var conn_mutex: SpinLock = .{};
var conns: ?std.StringHashMap(*acp.Connection) = null;

fn getConn(ctx: *svc.Call, agent: ExternalAgentDef) !*acp.Connection {
    conn_mutex.lock();
    defer conn_mutex.unlock();
    if (conns == null) {
        conns = std.StringHashMap(*acp.Connection).init(std.heap.c_allocator);
    }
    if (conns.?.get(agent.id)) |existing| return existing;
    const allocator = ctx.app.allocator;
    const conn = acp.spawn(
        allocator,
        agent.id,
        agent.command,
        agent.args,
        ctx.app.env_map,
        ctx.app.env_map.get("PWD") orelse "",
    ) catch |err| {
        std.debug.print("[acp] connect failed \"{s}\": {s}\n", .{ agent.id, @errorName(err) });
        return err;
    };
    conns.?.put(agent.id, conn) catch {
        conn.deinit();
        return error.OutOfMemory;
    };
    std.debug.print("[acp] connected \"{s}\" name={s}\n", .{ agent.id, conn.agent_name });
    return conn;
}

// ============================================================================
// Wire handlers
// ============================================================================

/// ListExternalAgents — [{id, name, description}, ...].
pub fn listAgents(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    var list = std.ArrayList(ExternalAgentDef).empty;
    defer list.deinit(allocator);
    for (AGENTS) |a| list.append(allocator, a) catch {};
    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(list.items, .{}, &out.writer);
    return out.toOwnedSlice();
}

/// CreateExternalAgentSession — { agentId, name, projectFolder } → creates a
/// ForgeADE session with role `external:<agentId>` and an ACP session.
/// Returns the FullAgentSession shape (meta + empty messages).
pub fn createExternalSession(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct { agentId: []const u8 = "", name: []const u8 = "", projectFolder: []const u8 = "" };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid payload");
        return "";
    };
    defer parsed.deinit();

    const agent = findAgent(parsed.value.agentId) orelse {
        try svc.failCtx(ctx, "unknown external agent");
        return "";
    };
    const conn = getConn(ctx, agent) catch {
        try svc.failCtx(ctx, "external agent failed to start");
        return "";
    };
    const acp_session_id = acp.newSession(conn, allocator, parsed.value.projectFolder) catch {
        try svc.failCtx(ctx, "external agent session/new failed");
        return "";
    };
    defer allocator.free(acp_session_id);

    // Create the ForgeADE session file (role external:<agentId>).
    const id_buf = std.fmt.allocPrint(allocator, "agent-{d}-{x}", .{ svc.nowMs(), @mod(svc.nowMs(), 0xffff) }) catch {
        try svc.failCtx(ctx, "id alloc failed");
        return "";
    };
    defer allocator.free(id_buf);
    const now = @divTrunc(svc.nowMs(), 1000);
    const role = std.fmt.allocPrint(allocator, "external:{s}", .{agent.id}) catch return "";
    defer allocator.free(role);
    const name = if (parsed.value.name.len > 0) parsed.value.name else agent.name;

    const agent_zig = @import("agent.zig");
    const session = agent_zig.makeSession(ctx, id_buf, name, role, parsed.value.projectFolder, now) catch {
        try svc.failCtx(ctx, "session create failed");
        return "";
    };
    agent_zig.setSessionExternalId(ctx, session.id, acp_session_id);
    emitExternalEvent(ctx, "session:opened", .{ .id = session.id, .name = session.name, .role = session.role, .projectFolder = session.projectFolder, .state = "idle" });
    emitExternalEvent(ctx, "agent:updated", .{ .id = session.id });

    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(session, .{}, &out.writer);
    return out.toOwnedSlice();
}

/// GetExternalAgentState — { id } → { configOptions, availableCommands }.
pub fn getExternalState(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct { id: []const u8 = "" };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid payload");
        return "";
    };
    defer parsed.deinit();
    const id = parsed.value.id;

    // The session id is `agent-...`; the role encodes the agent id.
    const agent_zig = @import("agent.zig");
    const role = agent_zig.sessionRole(ctx, id) orelse {
        try svc.failCtx(ctx, "session not found");
        return "";
    };
    if (!std.mem.startsWith(u8, role, "external:")) {
        // Not an external session — return empty state.
        var out = std.Io.Writer.Allocating.init(allocator);
        try std.json.Stringify.value(.{ .configOptions = @as([]acp.ConfigOption, &.{}), .availableCommands = @as([]acp.AvailableCommand, &.{}) }, .{}, &out.writer);
        return out.toOwnedSlice();
    }
    const agent_id = role["external:".len..];
    const agent = findAgent(agent_id) orelse {
        try svc.failCtx(ctx, "unknown external agent");
        return "";
    };
    const conn = getConn(ctx, agent) catch {
        try svc.failCtx(ctx, "external agent not connected");
        return "";
    };
    const acp_session_id = agent_zig.sessionExternalId(ctx, id) orelse {
        try svc.failCtx(ctx, "acp session id missing");
        return "";
    };
    const state = acp.getState(conn, acp_session_id);
    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(.{
        .configOptions = state.configOptions,
        .availableCommands = state.availableCommands,
    }, .{}, &out.writer);
    return out.toOwnedSlice();
}

/// SetExternalAgentConfig — { id, configId, value } → refreshed state.
pub fn setExternalConfig(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct { id: []const u8 = "", configId: []const u8 = "", value: std.json.Value = .{ .string = "" } };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid payload");
        return "";
    };
    defer parsed.deinit();
    const id = parsed.value.id;

    const agent_zig = @import("agent.zig");
    const role = agent_zig.sessionRole(ctx, id) orelse {
        try svc.failCtx(ctx, "session not found");
        return "";
    };
    if (!std.mem.startsWith(u8, role, "external:")) {
        var out = std.Io.Writer.Allocating.init(allocator);
        try std.json.Stringify.value(.{ .configOptions = @as([]acp.ConfigOption, &.{}), .availableCommands = @as([]acp.AvailableCommand, &.{}) }, .{}, &out.writer);
        return out.toOwnedSlice();
    }
    const agent_id = role["external:".len..];
    const agent = findAgent(agent_id) orelse {
        try svc.failCtx(ctx, "unknown external agent");
        return "";
    };
    const conn = getConn(ctx, agent) catch {
        try svc.failCtx(ctx, "external agent not connected");
        return "";
    };
    const acp_session_id = agent_zig.sessionExternalId(ctx, id) orelse {
        try svc.failCtx(ctx, "acp session id missing");
        return "";
    };
    const state = acp.setConfigOption(conn, allocator, acp_session_id, parsed.value.configId, parsed.value.value) catch {
        try svc.failCtx(ctx, "set config option failed");
        return "";
    };
    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(.{
        .configOptions = state.configOptions,
        .availableCommands = state.availableCommands,
    }, .{}, &out.writer);
    return out.toOwnedSlice();
}

/// SendAgentMessage for external sessions — routed through ACP prompt.
/// The session role `external:<id>` selects the connection; the session file
/// stores the ACP session id in a `acpSessionId` field on the header.
pub fn sendExternalMessage(ctx: *svc.Call, id: []const u8, text: []const u8) anyerror!void {
    const allocator = ctx.app.allocator;
    const agent_zig = @import("agent.zig");
    const role = agent_zig.sessionRole(ctx, id) orelse return error.SessionNotFound;
    if (!std.mem.startsWith(u8, role, "external:")) return error.NotExternalSession;
    const agent_id = role["external:".len..];
    const agent = findAgent(agent_id) orelse return error.UnknownAgent;
    const conn = getConn(ctx, agent) catch return error.AgentStartFailed;
    const acp_session_id = agent_zig.sessionExternalId(ctx, id) orelse return error.NoAcpSession;
    // Drain any pending client requests (permission, fs) from prior turns.
    _ = acp.handlePendingRequests(conn, allocator, ctx.app.env_map.get("PWD") orelse "");
    acp.prompt(conn, allocator, acp_session_id, text) catch |err| {
        emitExternalEvent(ctx, "agent:error", .{ .id = id, .message = "acp prompt failed" });
        emitExternalEvent(ctx, "agent:turn_end", .{ .id = id, .ok = false });
        return err;
    };
    _ = acp.handlePendingRequests(conn, allocator, ctx.app.env_map.get("PWD") orelse "");
    emitExternalEvent(ctx, "agent:turn_end", .{ .id = id, .ok = true });
}

fn emitExternalEvent(ctx: *svc.Call, event: []const u8, payload: anytype) void {
    const emit_fn = ctx.app.emit_fn orelse return;
    const emit_ctx = ctx.app.emit_ctx orelse return;
    var out = std.Io.Writer.Allocating.init(ctx.app.allocator);
    defer out.deinit();
    std.json.Stringify.value(.{ .event = event, .payload = payload }, .{}, &out.writer) catch return;
    emit_fn(emit_ctx, ctx.app.main_window_id, "services.agent", out.written()) catch {};
}

test "findAgent resolves registry ids" {
    try std.testing.expect(findAgent("opencode") != null);
    try std.testing.expect(findAgent("pi") != null);
    try std.testing.expect(findAgent("nope") == null);
}
