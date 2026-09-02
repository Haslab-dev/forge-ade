// services.agent — session CRUD + turn orchestration.
// Port of src/server/agent/ (SessionStore + AgentManager facade).
// Sessions persist as JSONL per-session files under ~/.forge-ade/agent-sessions/,
// matching the append-only shape the TS store used. The turn loop itself is
// the LLM service's consumer; the bootstrap delivers the streaming event
// contract (agent:turn_start/message_delta/turn_end) with the LLM call wired
// next.

const std = @import("std");
const svc = @import("../services.zig");

const ContentBlock = struct {
    type: []const u8,
    text: ?[]const u8 = null,
    tool_call_id: ?[]const u8 = null,
    name: ?[]const u8 = null,
    arguments: ?[]const u8 = null,
    is_error: ?bool = null,
};

const AgentMessage = struct {
    id: []const u8,
    role: []const u8,
    content: []ContentBlock = &.{},
    timestamp: []const u8,
    state: ?[]const u8 = null,
    /// OpenAI-format tool_calls array serialized as JSON (assistant messages).
    tool_calls: ?[]const u8 = null,
    tool_call_id: ?[]const u8 = null,
};

const SessionMeta = struct {
    id: []const u8,
    name: []const u8,
    role: []const u8,
    projectFolder: []const u8,
    dialect: []const u8 = "",
    autoApprove: bool = false,
    createdAt: i64,
    updatedAt: i64,
    messageCount: usize,
    lastMessagePreview: []const u8 = "",
    state: []const u8 = "idle",
    contextWindow: usize = 128_000,
};

const Session = struct {
    id: []const u8,
    name: []const u8,
    role: []const u8,
    projectFolder: []const u8,
    dialect: []const u8 = "",
    autoApprove: bool = false,
    createdAt: i64,
    updatedAt: i64,
    messageCount: usize = 0,
    lastMessagePreview: []const u8 = "",
    state: []const u8 = "idle",
    contextWindow: usize = 128_000,
    messages: []AgentMessage = &.{},
    customPrompt: ?[]const u8 = null,
    customRules: ?[]const u8 = null,
    summary: ?[]const u8 = null,
};

const AgentDefinition = struct {
    id: []const u8,
    name: []const u8,
    role_filter: []const u8 = "coding",
    description: []const u8 = "",
    prompt: []const u8 = "",
    rules: []const u8 = "",
    model: ?[]const u8 = null,
};

const DEFAULT_DEFINITIONS = [_]AgentDefinition{
    .{ .id = "coder", .name = "Full-Stack Engineer", .role_filter = "coding", .description = "Builds features, fixes bugs, and runs refactors with tool access.", .prompt = "You are an expert full-stack engineer. Write clean, idiomatic code.", .rules = "1. Read files before editing.\n2. Verify changes with tests.", .model = "claude-3-7-sonnet-20250219" },
    .{ .id = "planner", .name = "Architect & Planner", .role_filter = "planning", .description = "Designs system architectures and breaks down complex phases.", .prompt = "You are a software architect. Create crisp, structured plans.", .rules = "1. List constraints.\n2. Break down into discrete phases.", .model = "claude-3-7-sonnet-20250219" },
    .{ .id = "researcher", .name = "Research Scout", .role_filter = "research", .description = "Investigates APIs, repos, and documentation.", .prompt = "You are a research scout. Gather exact facts from sources.", .rules = "1. Be evidence-first.\n2. Cite exact files and symbols.", .model = "claude-3-5-haiku-20241022" },
};

fn sessionsDir(env_map: *std.process.Environ.Map, allocator: std.mem.Allocator) []const u8 {
    return std.fmt.allocPrint(allocator, "{s}/agent-sessions", .{svc.dataDir(env_map)}) catch "";
}

fn sessionPath(env_map: *std.process.Environ.Map, allocator: std.mem.Allocator, id: []const u8) []const u8 {
    return std.fmt.allocPrint(allocator, "{s}/agent-sessions/{s}.jsonl", .{ svc.dataDir(env_map), id }) catch "";
}

fn definitionsPath(env_map: *std.process.Environ.Map, allocator: std.mem.Allocator) []const u8 {
    return std.fmt.allocPrint(allocator, "{s}/agent_definitions.json", .{svc.dataDir(env_map)}) catch "";
}

/// ~/.forge-ade/models.json (active provider config).
fn configPath(env_map: *std.process.Environ.Map, allocator: std.mem.Allocator) []const u8 {
    return std.fmt.allocPrint(allocator, "{s}/models.json", .{svc.dataDir(env_map)}) catch "";
}

fn loadDefinitions(ctx: *svc.Call) []AgentDefinition {
    const allocator = ctx.app.allocator;
    const path = definitionsPath(ctx.app.env_map, allocator);
    defer allocator.free(path);

    // Read the file ourselves so the parsed strings stay alive while we
    // deep-copy them (readJsonFile frees its temp buffer before returning).
    var list = std.ArrayList(AgentDefinition).empty;
    const raw = svc.readFileBounded(allocator, path) catch null;
    if (raw) |raw_bytes| {
        defer allocator.free(raw_bytes);
        if (std.json.parseFromSlice([]AgentDefinition, allocator, raw_bytes, .{ .ignore_unknown_fields = true })) |parsed| {
            defer parsed.deinit();
            for (parsed.value) |d| {
                list.append(allocator, .{
                    .id = allocator.dupe(u8, d.id) catch continue,
                    .name = allocator.dupe(u8, d.name) catch continue,
                    .role_filter = allocator.dupe(u8, d.role_filter) catch continue,
                    .description = allocator.dupe(u8, d.description) catch continue,
                    .prompt = allocator.dupe(u8, d.prompt) catch continue,
                    .rules = allocator.dupe(u8, d.rules) catch continue,
                    .model = if (d.model) |m| allocator.dupe(u8, m) catch null else null,
                }) catch {};
            }
        } else |_| {}
    }
    if (list.items.len > 0) return list.toOwnedSlice(allocator) catch &.{};

    // Seed defaults (copies so they survive the stack).
    for (DEFAULT_DEFINITIONS) |d| {
        list.append(allocator, .{
            .id = allocator.dupe(u8, d.id) catch continue,
            .name = allocator.dupe(u8, d.name) catch continue,
            .role_filter = allocator.dupe(u8, d.role_filter) catch continue,
            .description = allocator.dupe(u8, d.description) catch continue,
            .prompt = allocator.dupe(u8, d.prompt) catch continue,
            .rules = allocator.dupe(u8, d.rules) catch continue,
            .model = allocator.dupe(u8, d.model orelse "") catch continue,
        }) catch {};
    }
    return list.toOwnedSlice(allocator) catch &.{};
}

/// Re-reads a session's JSONL file into a Session (messages in order).
fn loadSession(ctx: *svc.Call, id: []const u8) ?Session {
    const allocator = ctx.app.allocator;
    const path = sessionPath(ctx.app.env_map, allocator, id);
    defer allocator.free(path);
    const raw = svc.readFileBounded(allocator, path) catch return null;
    defer allocator.free(raw);

    var messages = std.ArrayList(AgentMessage).empty;
    var meta: ?struct { name: []const u8, role: []const u8, projectFolder: []const u8, createdAt: i64 } = null;
    var lines = std.mem.splitScalar(u8, raw, '\n');
    while (lines.next()) |line| {
        if (line.len == 0) continue;
        if (std.mem.indexOf(u8, line, "\"type\":\"session\"") != null) {
            const Hdr = struct {
                type: []const u8,
                version: u32,
                id: []const u8,
                name: []const u8,
                role: []const u8,
                projectFolder: []const u8,
                createdAt: i64,
            };
            const h = std.json.parseFromSlice(Hdr, allocator, line, .{ .ignore_unknown_fields = true }) catch continue;
            meta = .{ .name = allocator.dupe(u8, h.value.name) catch "", .role = allocator.dupe(u8, h.value.role) catch "", .projectFolder = allocator.dupe(u8, h.value.projectFolder) catch "", .createdAt = h.value.createdAt };
            h.deinit();
        } else if (std.mem.indexOf(u8, line, "\"type\":\"message\"") != null) {
            const Entry = struct { type: []const u8, message: AgentMessage };
            const e = std.json.parseFromSlice(Entry, allocator, line, .{ .ignore_unknown_fields = true }) catch continue;
            const m = e.value.message;
            // Deep-copy the message (strings point into `line`).
            var blocks = std.ArrayList(ContentBlock).empty;
            for (m.content) |b| {
                blocks.append(allocator, .{
                    .type = allocator.dupe(u8, b.type) catch continue,
                    .text = if (b.text) |t| allocator.dupe(u8, t) catch null else null,
                    .tool_call_id = if (b.tool_call_id) |t| allocator.dupe(u8, t) catch null else null,
                    .name = if (b.name) |n| allocator.dupe(u8, n) catch null else null,
                    .arguments = if (b.arguments) |a| allocator.dupe(u8, a) catch null else null,
                    .is_error = b.is_error,
                }) catch {};
            }
            messages.append(allocator, .{
                .id = allocator.dupe(u8, m.id) catch continue,
                .role = allocator.dupe(u8, m.role) catch continue,
                .content = blocks.items,
                .timestamp = allocator.dupe(u8, m.timestamp) catch continue,
                .tool_calls = if (m.tool_calls) |tc| allocator.dupe(u8, tc) catch null else null,
                .tool_call_id = if (m.tool_call_id) |t| allocator.dupe(u8, t) catch null else null,
            }) catch {};
            e.deinit();
        }
    }
    const m = meta orelse return null;
    // lastMessagePreview: the last message's first text block. Guard the
    // empty-content case — content[0] on an empty slice is out-of-bounds UB
    // in ReleaseFast (assistant messages can persist with no content blocks).
    var preview: []const u8 = "";
    if (messages.items.len > 0) {
        const last = messages.items[messages.items.len - 1];
        if (last.content.len > 0) preview = last.content[0].text orelse "";
    }
    return .{
        .id = allocator.dupe(u8, id) catch return null,
        .name = m.name,
        .role = m.role,
        .projectFolder = m.projectFolder,
        .createdAt = m.createdAt,
        .updatedAt = m.createdAt,
        .messageCount = messages.items.len,
        .messages = messages.items,
        .lastMessagePreview = preview,
    };
}

/// Emits a `services.<event>` window event via the App's emit hook. The
/// payload is wrapped as {event, payload} so the frontend's zero.on handler
/// can dispatch by inner event name.
fn emitEvent(ctx: *svc.Call, event: []const u8, payload: anytype) void {
    const emit_fn = ctx.app.emit_fn orelse return;
    const emit_ctx = ctx.app.emit_ctx orelse return;
    var out = std.Io.Writer.Allocating.init(ctx.app.allocator);
    defer out.deinit();
    std.json.Stringify.value(.{ .event = event, .payload = payload }, .{}, &out.writer) catch return;
    emit_fn(emit_ctx, ctx.app.main_window_id, "services.agent", out.written()) catch {};
}

fn listSessionFiles(ctx: *svc.Call) []const []const u8 {
    const allocator = ctx.app.allocator;
    const dir = sessionsDir(ctx.app.env_map, allocator);
    defer allocator.free(dir);
    const dir_z = allocator.dupeZ(u8, dir) catch return &.{};
    defer allocator.free(dir_z);
    const Dirent = extern struct {
        d_ino: u64,
        d_seekoff: u64,
        d_reclen: u16,
        d_namlen: u16,
        d_type: u8,
        d_name: [1024]u8,
    };
    const c = struct {
        extern "c" fn opendir(dirname: [*:0]const u8) ?*anyopaque;
        extern "c" fn closedir(dirp: *anyopaque) c_int;
        extern "c" fn readdir(dirp: *anyopaque) ?*const Dirent;
    };
    const handle = c.opendir(dir_z) orelse return &.{};
    defer _ = c.closedir(handle);
    var list = std.ArrayList([]const u8).empty;
    while (c.readdir(handle)) |entry| {
        const name = std.mem.sliceTo(&entry.d_name, 0);
        if (!std.mem.endsWith(u8, name, ".jsonl")) continue;
        const id = name[0 .. name.len - 6];
        list.append(allocator, allocator.dupe(u8, id) catch continue) catch {};
    }
    return list.toOwnedSlice(allocator) catch &.{};
}

fn metaOf(ctx: *svc.Call, s: Session) SessionMeta {
    _ = ctx;
    return .{
        .id = s.id,
        .name = s.name,
        .role = s.role,
        .projectFolder = s.projectFolder,
        .dialect = s.dialect,
        .autoApprove = s.autoApprove,
        .createdAt = s.createdAt,
        .updatedAt = s.updatedAt,
        .messageCount = s.messageCount,
        .lastMessagePreview = s.lastMessagePreview,
        .state = s.state,
        .contextWindow = s.contextWindow,
    };
}

pub fn listSessions(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const ids = listSessionFiles(ctx);
    defer allocator.free(ids);

    var metas = std.ArrayList(SessionMeta).empty;
    defer metas.deinit(allocator);
    for (ids) |id| {
        defer allocator.free(id);
        if (loadSession(ctx, id)) |s| {
            const m = metaOf(ctx, s);
            metas.append(allocator, .{
                .id = m.id, .name = m.name, .role = m.role, .projectFolder = m.projectFolder,
                .dialect = m.dialect, .autoApprove = m.autoApprove, .createdAt = m.createdAt,
                .updatedAt = m.updatedAt, .messageCount = m.messageCount,
                .lastMessagePreview = m.lastMessagePreview, .state = m.state, .contextWindow = m.contextWindow,
            }) catch {};
        }
    }

    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(metas.items, .{}, &out.writer);
    return out.toOwnedSlice();
}

pub fn getSession(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct { id: []const u8 = "" };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid payload");
        return "";
    };
    defer parsed.deinit();
    const s = loadSession(ctx, parsed.value.id) orelse {
        try svc.failCtx(ctx, "session not found");
        return "";
    };
    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(s, .{}, &out.writer);
    return out.toOwnedSlice();
}

/// Creates the session JSONL file (header line) and returns the Session.
/// Used by createSession and external-agent sessions.
pub fn makeSession(ctx: *svc.Call, id: []const u8, name: []const u8, role: []const u8, project_folder: []const u8, now: i64) !Session {
    const allocator = ctx.app.allocator;
    const path = sessionPath(ctx.app.env_map, allocator, id);
    defer allocator.free(path);

    // Ensure dir exists.
    const dir = sessionsDir(ctx.app.env_map, allocator);
    defer allocator.free(dir);
    const dir_z = allocator.dupeZ(u8, dir) catch return error.OutOfMemory;
    defer allocator.free(dir_z);
    const c_sys = struct {
        extern "c" fn mkdir(path: [*:0]const u8, mode: c_uint) c_int;
    };
    _ = c_sys.mkdir(dir_z, 0o755); // EEXIST fine

    const session = Session{
        .id = allocator.dupe(u8, id) catch return error.OutOfMemory,
        .name = allocator.dupe(u8, name) catch return error.OutOfMemory,
        .role = allocator.dupe(u8, role) catch return error.OutOfMemory,
        .projectFolder = allocator.dupe(u8, project_folder) catch return error.OutOfMemory,
        .createdAt = now,
        .updatedAt = now,
    };
    var out = std.Io.Writer.Allocating.init(allocator);
    defer out.deinit();
    try std.json.Stringify.value(.{
        .type = "session",
        .version = 1,
        .id = session.id,
        .name = session.name,
        .role = session.role,
        .projectFolder = session.projectFolder,
        .createdAt = session.createdAt,
    }, .{}, &out.writer);
    const line = try std.fmt.allocPrint(allocator, "{s}\n", .{out.written()});
    defer allocator.free(line);
    try svc.writeFileAtomic(allocator, path, line);
    return session;
}

/// Reads the session header's role field.
pub fn sessionRole(ctx: *svc.Call, id: []const u8) ?[]const u8 {
    const allocator = ctx.app.allocator;
    const path = sessionPath(ctx.app.env_map, allocator, id);
    defer allocator.free(path);
    const raw = svc.readFileBounded(allocator, path) catch return null;
    defer allocator.free(raw);
    var lines = std.mem.splitScalar(u8, raw, '\n');
    while (lines.next()) |line| {
        if (line.len == 0) continue;
        if (std.mem.indexOf(u8, line, "\"type\":\"session\"") == null) continue;
        const Hdr = struct { role: []const u8 = "" };
        const parsed = std.json.parseFromSlice(Hdr, allocator, line, .{ .ignore_unknown_fields = true }) catch return null;
        defer parsed.deinit();
        return allocator.dupe(u8, parsed.value.role) catch "";
    }
    return null;
}

/// Reads the session header's acpSessionId (external sessions only).
pub fn sessionExternalId(ctx: *svc.Call, id: []const u8) ?[]const u8 {
    const allocator = ctx.app.allocator;
    const path = sessionPath(ctx.app.env_map, allocator, id);
    defer allocator.free(path);
    const raw = svc.readFileBounded(allocator, path) catch return null;
    defer allocator.free(raw);
    var lines = std.mem.splitScalar(u8, raw, '\n');
    while (lines.next()) |line| {
        if (line.len == 0) continue;
        if (std.mem.indexOf(u8, line, "\"type\":\"session\"") == null) continue;
        const Hdr = struct { acpSessionId: ?[]const u8 = null };
        const parsed = std.json.parseFromSlice(Hdr, allocator, line, .{ .ignore_unknown_fields = true }) catch return null;
        defer parsed.deinit();
        if (parsed.value.acpSessionId) |sid| return allocator.dupe(u8, sid) catch "";
    }
    return null;
}

/// Sets (or clears) the acpSessionId on the session header line.
pub fn setSessionExternalId(ctx: *svc.Call, id: []const u8, acp_id: ?[]const u8) void {
    const allocator = ctx.app.allocator;
    const path = sessionPath(ctx.app.env_map, allocator, id);
    defer allocator.free(path);
    const raw = svc.readFileBounded(allocator, path) catch return;
    defer allocator.free(raw);
    var out = std.ArrayList(u8).empty;
    defer out.deinit(allocator);
    var first = true;
    var lines = std.mem.splitScalar(u8, raw, '\n');
    while (lines.next()) |line| {
        if (line.len == 0) continue;
        if (first and std.mem.indexOf(u8, line, "\"type\":\"session\"") != null) {
            const Hdr = struct {
                type: []const u8,
                version: u32,
                id: []const u8,
                name: []const u8,
                role: []const u8,
                projectFolder: []const u8,
                createdAt: i64,
            };
            const parsed = std.json.parseFromSlice(Hdr, allocator, line, .{ .ignore_unknown_fields = true }) catch {
                out.appendSlice(allocator, line) catch {};
                out.append(allocator, '\n') catch {};
                first = false;
                continue;
            };
            defer parsed.deinit();
            var hdr_out = std.Io.Writer.Allocating.init(allocator);
            defer hdr_out.deinit();
            std.json.Stringify.value(.{
                .type = "session",
                .version = 1,
                .id = parsed.value.id,
                .name = parsed.value.name,
                .role = parsed.value.role,
                .projectFolder = parsed.value.projectFolder,
                .createdAt = parsed.value.createdAt,
                .acpSessionId = acp_id,
            }, .{}, &hdr_out.writer) catch {};
            out.appendSlice(allocator, hdr_out.written()) catch {};
            out.append(allocator, '\n') catch {};
        } else {
            out.appendSlice(allocator, line) catch {};
            out.append(allocator, '\n') catch {};
        }
        first = false;
    }
    svc.writeFileAtomic(allocator, path, out.items) catch {};
}

pub fn createSession(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct {
        name: []const u8 = "Agent",
        role: []const u8 = "coding",
        projectFolder: []const u8 = "",
    };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid payload");
        return "";
    };
    defer parsed.deinit();

    const now = @divTrunc(svc.nowMs(), 1000);
    const id_buf = std.fmt.allocPrint(allocator, "agent-{d}-{x}", .{ svc.nowMs(), @mod(svc.nowMs(), 0xffff) }) catch {
        try svc.failCtx(ctx, "id alloc failed");
        return "";
    };
    const session = try makeSession(ctx, id_buf, parsed.value.name, parsed.value.role, parsed.value.projectFolder, now);

    emitEvent(ctx, "session:opened", metaOf(ctx, session));
    emitEvent(ctx, "agent:updated", .{ .id = session.id });

    var res = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(session, .{}, &res.writer);
    return res.toOwnedSlice();
}

pub fn deleteSession(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct { id: []const u8 = "" };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid payload");
        return "";
    };
    defer parsed.deinit();
    const path = sessionPath(ctx.app.env_map, allocator, parsed.value.id);
    defer allocator.free(path);
    const path_z = allocator.dupeZ(u8, path) catch return "";
    defer allocator.free(path_z);
    _ = std.c.unlink(path_z);
    emitEvent(ctx, "session:closed", .{ .id = parsed.value.id });
    try svc.okCtx(ctx);
    return "";
}

pub fn updateSession(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct {
        id: []const u8 = "",
        name: []const u8 = "",
        role: []const u8 = "",
        customPrompt: []const u8 = "",
        customRules: []const u8 = "",
    };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid payload");
        return "";
    };
    defer parsed.deinit();
    var s = loadSession(ctx, parsed.value.id) orelse {
        try svc.failCtx(ctx, "session not found");
        return "";
    };
    if (parsed.value.name.len > 0) s.name = parsed.value.name;
    if (parsed.value.role.len > 0) s.role = parsed.value.role;
    s.updatedAt = @divTrunc(svc.nowMs(), 1000);
    // Rewrite the header entry in place (best-effort): append a new header
    // line so the frontend's meta refresh picks up the rename.
    const path = sessionPath(ctx.app.env_map, allocator, parsed.value.id);
    defer allocator.free(path);
    const raw = svc.readFileBounded(allocator, path) catch "";
    defer allocator.free(raw);
    var out = std.ArrayList(u8).empty;
    defer out.deinit(allocator);
    var first = true;
    var lines = std.mem.splitScalar(u8, raw, '\n');
    while (lines.next()) |line| {
        if (line.len == 0) continue;
        if (first and std.mem.indexOf(u8, line, "\"type\":\"session\"") != null) {
            var hdr_out = std.Io.Writer.Allocating.init(allocator);
            defer hdr_out.deinit();
            std.json.Stringify.value(.{
                .type = "session",
                .version = 1,
                .id = s.id,
                .name = s.name,
                .role = s.role,
                .projectFolder = s.projectFolder,
                .createdAt = s.createdAt,
            }, .{}, &hdr_out.writer) catch {};
            try out.appendSlice(allocator, hdr_out.written());
            try out.append(allocator, '\n');
        } else {
            try out.appendSlice(allocator, line);
            try out.append(allocator, '\n');
        }
        first = false;
    }
    try svc.writeFileAtomic(allocator, path, out.items);
    emitEvent(ctx, "agent:updated", .{ .id = parsed.value.id });
    try svc.okCtx(ctx);
    return "";
}

/// Loads the active provider target from ~/.forge-ade/models.json.
pub fn loadActiveTargetPublic(ctx: *svc.Call, allocator: std.mem.Allocator) ?llm_client.ProviderTarget {
    return loadActiveTarget(ctx, allocator);
}

/// Loads the active provider target from ~/.forge-ade/models.json.
fn loadActiveTarget(ctx: *svc.Call, allocator: std.mem.Allocator) ?llm_client.ProviderTarget {
    const path = configPath(ctx.app.env_map, allocator);
    defer allocator.free(path);
    const raw = svc.readFileBounded(allocator, path) catch return null;
    defer allocator.free(raw);
    const parsed = std.json.parseFromSlice(std.json.Value, allocator, raw, .{ .ignore_unknown_fields = true }) catch return null;
    defer parsed.deinit();
    switch (parsed.value) {
        .object => |root| {
            var default_provider: []const u8 = "";
            if (root.get("default_provider")) |v| {
                if (v == .string) default_provider = v.string;
            }
            const providers = root.get("providers") orelse return null;
            switch (providers) {
                .object => |pobj| {
                    var chosen: ?*const std.json.Value = null;
                    if (default_provider.len > 0) {
                        if (pobj.getPtr(default_provider)) |p| chosen = p;
                    }
                    if (chosen == null) {
                        var it = pobj.iterator();
                        if (it.next()) |first| chosen = first.value_ptr;
                    }
                    const p = chosen orelse return null;
                    switch (p.*) {
                        .object => |po| {
                            const getStr = struct {
                                fn f(o: *const std.json.ObjectMap, k: []const u8) []const u8 {
                                    if (o.get(k)) |v| {
                                        if (v == .string) return v.string;
                                    }
                                    return "";
                                }
                            }.f;
                            const base_url = getStr(&po, "base_url");
                            const api_key = getStr(&po, "api_key");
                            const active_model = getStr(&po, "active_model");
                            if (base_url.len == 0 or api_key.len == 0 or active_model.len == 0) return null;
                            // Deep-copy: the parsed Value (and all its strings)
                            // is freed by parsed.deinit() below — the target
                            // must own its own copies.
                            return .{
                                .providerId = allocator.dupe(u8, getStr(&po, "id")) catch return null,
                                .baseURL = allocator.dupe(u8, base_url) catch return null,
                                .apiKey = allocator.dupe(u8, api_key) catch return null,
                                .model = allocator.dupe(u8, active_model) catch return null,
                            };
                        },
                        else => return null,
                    }
                },
                else => return null,
            }
        },
        else => return null,
    }
}

/// Reads a session's messages into LLMMessage[] for the API call.
/// Returns fully-owned copies (caller frees each role/content + the slice).
fn sessionToLlmMessages(allocator: std.mem.Allocator, path: []const u8, tail: usize) []llm_client.LLMMessage {
    var messages = std.ArrayList(llm_client.LLMMessage).empty;
    const raw = svc.readFileBounded(allocator, path) catch return &.{};
    defer allocator.free(raw);
    var lines = std.mem.splitScalar(u8, raw, '\n');
    while (lines.next()) |line| {
        if (line.len == 0) continue;
        if (std.mem.indexOf(u8, line, "\"type\":\"message\"") == null) continue;
        const Entry = struct {
            type: []const u8,
            message: struct {
                id: []const u8,
                role: []const u8,
                timestamp: []const u8,
                content: []const struct { type: []const u8, text: ?[]const u8 = null, tool_call_id: ?[]const u8 = null, name: ?[]const u8 = null, arguments: ?[]const u8 = null, is_error: ?bool = null },
                tool_calls: ?[]const u8 = null,
                tool_call_id: ?[]const u8 = null,
            },
        };
        const e = std.json.parseFromSlice(Entry, allocator, line, .{ .ignore_unknown_fields = true }) catch continue;
        defer e.deinit();
        const m = e.value.message;

        if (std.mem.eql(u8, m.role, "tool")) {
            // tool role: content is the tool result; attach tool_call_id.
            var text: []const u8 = "";
            if (m.content.len > 0) text = m.content[0].text orelse "";
            const role = allocator.dupe(u8, m.role) catch continue;
            const content = allocator.dupe(u8, text) catch {
                allocator.free(role);
                continue;
            };
            const tcid = allocator.dupe(u8, if (m.tool_call_id) |t| t else if (m.content.len > 0) m.content[0].tool_call_id orelse "" else "") catch continue;
            messages.append(allocator, .{ .role = role, .content = content, .tool_call_id = tcid }) catch {};
            continue;
        }

        var text: []const u8 = "";
        if (m.content.len > 0) text = m.content[0].text orelse "";
        // Deep-copy role + content so they survive the parse arena and the
        // caller's frees.
        const role = allocator.dupe(u8, m.role) catch continue;
        const content = allocator.dupe(u8, text) catch {
            allocator.free(role);
            continue;
        };
        // Assistant messages carrying tool_calls: pass the raw JSON array so
        // the next LLM iteration sees them (multi-step tool loops).
        const tool_calls = if (m.tool_calls) |tc|
            allocator.dupe(u8, tc) catch null
        else if (std.mem.eql(u8, m.role, "assistant") and m.content.len > 0)
            buildToolCallsJson(allocator, m.content) catch null
        else
            null;
        messages.append(allocator, .{
            .role = role,
            .content = content,
            .tool_calls = tool_calls,
        }) catch {};
    }
    var out = messages.toOwnedSlice(allocator) catch return &.{};
    if (tail > 0 and out.len > tail) {
        // The prefix items (before the tail window) are owned — free them so
        // the returned tail slice has no leaked prefix.
        for (out[0 .. out.len - tail]) |m| {
            allocator.free(m.role);
            allocator.free(m.content);
            if (m.tool_calls) |tc| allocator.free(tc);
            if (m.tool_call_id) |t| allocator.free(t);
        }
        const tail_start = out.len - tail;
        const tail_slice = allocator.dupe(llm_client.LLMMessage, out[tail_start..]) catch return out;
        allocator.free(out);
        return tail_slice;
    }
    return out;
}

/// Builds the OpenAI `tool_calls` JSON array from persisted tool_call blocks.
/// Accepts any block slice exposing type/tool_call_id/name/arguments fields.
fn buildToolCallsJson(allocator: std.mem.Allocator, content: anytype) ![]const u8 {
    var out = std.Io.Writer.Allocating.init(allocator);
    errdefer out.deinit();
    try out.writer.writeAll("[");
    var first = true;
    for (content) |b| {
        const btype: []const u8 = b.type;
        if (!std.mem.eql(u8, btype, "tool_call")) continue;
        if (!first) try out.writer.writeAll(",");
        first = false;
        try out.writer.writeAll("{\"id\":");
        try std.json.Stringify.value(b.tool_call_id orelse "", .{}, &out.writer);
        try out.writer.writeAll(",\"type\":\"function\",\"function\":{\"name\":");
        try std.json.Stringify.value(b.name orelse "", .{}, &out.writer);
        try out.writer.writeAll(",\"arguments\":");
        const args = b.arguments orelse "{}";
        // arguments is already a JSON string — embed raw if valid, else stringify.
        if (std.json.parseFromSlice(std.json.Value, allocator, args, .{})) |_| {
            try out.writer.writeAll(args);
        } else |_| {
            try std.json.Stringify.value(args, .{}, &out.writer);
        }
        try out.writer.writeAll("}}");
    }
    try out.writer.writeAll("]");
    return out.toOwnedSlice();
}

const ToolCallEvent = struct {
    id: []const u8,
    name: []const u8,
    arguments: []const u8,
};

/// Executes one tool call (read/write/glob/grep/bash/mcp_*) via the native
/// shell (or the MCP client for mcp_<server>_<tool> names). Returns the
/// tool result text (owned — caller frees).
fn executeTool(ctx: *svc.Call, allocator: std.mem.Allocator, name: []const u8, args_json: []const u8, cwd: []const u8) []const u8 {
    // MCP tools route through the stdio client.
    const mcp_zig_mod = @import("mcp.zig");
    const mcp_client_mod = @import("mcp-client.zig");
    if (mcp_client_mod.isQualifiedToolName(name)) {
        return mcp_zig_mod.callQualifiedTool(ctx, name, args_json) catch |err| {
            return std.fmt.allocPrint(allocator, "mcp call failed: {s}", .{@errorName(err)}) catch "mcp call failed";
        };
    }
    const parsed = std.json.parseFromSlice(std.json.Value, allocator, args_json, .{}) catch return allocator.dupe(u8, "error: invalid tool arguments") catch "";
    defer parsed.deinit();
    var out = std.ArrayList(u8).empty;
    const shell = struct {
        extern "c" fn fork() c_int;
        extern "c" fn pipe(fildes: *[2]c_int) c_int;
        extern "c" fn dup2(oldfd: c_int, newfd: c_int) c_int;
        extern "c" fn close(fd: c_int) c_int;
        extern "c" fn read(fd: c_int, buf: [*]u8, len: usize) isize;
        extern "c" fn waitpid(pid: c_int, status: ?*c_int, options: c_int) c_int;
        extern "c" fn execvp(file: [*:0]const u8, argv: [*:null]const ?[*:0]const u8) c_int;
        extern "c" fn chdir(path: [*:0]const u8) c_int;
        extern "c" fn _exit(code: c_int) noreturn;
    };

    const getStr = struct {
        fn f(o: *const std.json.ObjectMap, k: []const u8) []const u8 {
            if (o.get(k)) |v| {
                if (v == .string) return v.string;
            }
            return "";
        }
    }.f;

    var command_buf = std.ArrayList(u8).empty;
    defer command_buf.deinit(allocator);

    if (std.mem.eql(u8, name, "bash") or std.mem.eql(u8, name, "shell") or std.mem.eql(u8, name, "run")) {
        const cmd = getStr(&parsed.value.object, "command");
        if (cmd.len == 0) {
            const desc = getStr(&parsed.value.object, "description");
            command_buf.appendSlice(allocator, desc) catch {};
        } else {
            command_buf.appendSlice(allocator, cmd) catch {};
        }
    } else if (std.mem.eql(u8, name, "read_file") or std.mem.eql(u8, name, "read")) {
        const path = getStr(&parsed.value.object, "path");
        command_buf.appendSlice(allocator, "cat ") catch {};
        command_buf.appendSlice(allocator, path) catch {};
    } else if (std.mem.eql(u8, name, "write_file") or std.mem.eql(u8, name, "write")) {
        const path = getStr(&parsed.value.object, "path");
        const content = getStr(&parsed.value.object, "content");
        command_buf.appendSlice(allocator, "mkdir -p \"$(dirname ") catch {};
        command_buf.appendSlice(allocator, path) catch {};
        command_buf.appendSlice(allocator, ")\" && cat > ") catch {};
        command_buf.appendSlice(allocator, path) catch {};
        command_buf.appendSlice(allocator, " << 'FORGE_EOF'\n") catch {};
        command_buf.appendSlice(allocator, content) catch {};
        command_buf.appendSlice(allocator, "\nFORGE_EOF") catch {};
    } else if (std.mem.eql(u8, name, "glob") or std.mem.eql(u8, name, "find")) {
        const pattern = getStr(&parsed.value.object, "pattern");
        command_buf.appendSlice(allocator, "find . -path '*/") catch {};
        command_buf.appendSlice(allocator, pattern) catch {};
        command_buf.appendSlice(allocator, "' 2>/dev/null | head -100") catch {};
    } else if (std.mem.eql(u8, name, "grep") or std.mem.eql(u8, name, "search")) {
        const q = getStr(&parsed.value.object, "query");
        command_buf.appendSlice(allocator, "grep -rn --include='*' -m 5 ") catch {};
        command_buf.appendSlice(allocator, q) catch {};
        command_buf.appendSlice(allocator, " . 2>/dev/null | head -50") catch {};
    } else if (std.mem.eql(u8, name, "list_dir")) {
        const path = getStr(&parsed.value.object, "path");
        command_buf.appendSlice(allocator, "ls -la ") catch {};
        command_buf.appendSlice(allocator, if (path.len > 0) path else ".") catch {};
    } else {
        return allocator.dupe(u8, "error: unknown tool") catch "";
    }

    var pipe_fds: [2]c_int = undefined;
    if (shell.pipe(&pipe_fds) != 0) return allocator.dupe(u8, "error: pipe failed") catch "";
    const pid = shell.fork();
    if (pid < 0) return allocator.dupe(u8, "error: fork failed") catch "";
    if (pid == 0) {
        _ = shell.close(pipe_fds[0]);
        _ = shell.dup2(pipe_fds[1], 1);
        _ = shell.dup2(pipe_fds[1], 2);
        _ = shell.close(pipe_fds[1]);
        if (cwd.len > 0) {
            const cwd_z = allocator.dupeZ(u8, cwd) catch shell._exit(127);
            _ = shell.chdir(cwd_z.ptr);
        }
        const sh_z = allocator.dupeZ(u8, "/bin/sh") catch shell._exit(127);
        const flag_z = allocator.dupeZ(u8, "-c") catch shell._exit(127);
        const cmd_z = allocator.dupeZ(u8, command_buf.items) catch shell._exit(127);
        const args = [_]?[*:0]const u8{ sh_z.ptr, flag_z.ptr, cmd_z.ptr, null };
        _ = shell.execvp(sh_z.ptr, @ptrCast(&args));
        shell._exit(127);
    }
    _ = shell.close(pipe_fds[1]);
    var buf: [8192]u8 = undefined;
    while (true) {
        const n = shell.read(pipe_fds[0], &buf, buf.len);
        if (n <= 0) break;
        out.appendSlice(allocator, buf[0..@intCast(n)]) catch {};
        if (out.items.len > 200_000) break;
    }
    _ = shell.close(pipe_fds[0]);
    _ = shell.waitpid(pid, null, 0);
    return out.toOwnedSlice(allocator) catch allocator.dupe(u8, "error: oom") catch "";
}

/// Appends an assistant message (with content + tool calls) to the session JSONL.
fn appendAssistantMessage(allocator: std.mem.Allocator, path: []const u8, msg_id: []const u8, ts: []const u8, content: []const u8, toolCalls: []const ToolCallEvent) void {
    var out = std.Io.Writer.Allocating.init(allocator);
    defer out.deinit();
    // Build the persisted message's content blocks: text first, then a
    // tool_call block per tool call (the shape the frontend's session
    // renderer and loadSession both understand).
    var blocks = std.ArrayList(struct { type: []const u8, text: ?[]const u8 = null, tool_call_id: ?[]const u8 = null, name: ?[]const u8 = null, arguments: ?[]const u8 = null }).empty;
    defer blocks.deinit(allocator);
    if (content.len > 0) blocks.append(allocator, .{ .type = "text", .text = content }) catch {};
    for (toolCalls) |tc| {
        blocks.append(allocator, .{
            .type = "tool_call",
            .tool_call_id = tc.id,
            .name = tc.name,
            .arguments = tc.arguments,
        }) catch {};
    }
    // OpenAI-format tool_calls array (for the LLM reconstruction path).
    const tool_calls_json = buildToolCallsJson(allocator, blocks.items) catch null;
    defer if (tool_calls_json) |j| allocator.free(j);

    std.json.Stringify.value(.{
        .type = "message",
        .id = msg_id,
        .parentId = null,
        .timestamp = ts,
        .message = .{
            .id = msg_id,
            .role = "assistant",
            .timestamp = ts,
            .content = blocks.items,
            .tool_calls = tool_calls_json,
            .state = "done",
        },
    }, .{}, &out.writer) catch return;
    const existing = svc.readFileBounded(allocator, path) catch "";
    defer allocator.free(existing);
    var full = std.ArrayList(u8).empty;
    defer full.deinit(allocator);
    full.appendSlice(allocator, existing) catch {};
    full.appendSlice(allocator, out.written()) catch {};
    full.append(allocator, '\n') catch {};
    svc.writeFileAtomic(allocator, path, full.items) catch {};
}

/// Appends a tool role message (tool result) to the session JSONL, pairing
/// with the assistant's tool_call via tool_call_id.
fn appendToolResultMessage(allocator: std.mem.Allocator, path: []const u8, toolCallId: []const u8, name: []const u8, text: []const u8, isError: bool) void {
    var out = std.Io.Writer.Allocating.init(allocator);
    defer out.deinit();
    const msg_id = std.fmt.allocPrint(allocator, "msg-{d}-r", .{svc.nowMs()}) catch return;
    defer allocator.free(msg_id);
    const ts = std.fmt.allocPrint(allocator, "{d}", .{svc.nowMs()}) catch return;
    defer allocator.free(ts);
    std.json.Stringify.value(.{
        .type = "message",
        .id = msg_id,
        .parentId = null,
        .timestamp = ts,
        .message = .{
            .id = msg_id,
            .role = "tool",
            .timestamp = ts,
            .content = &.{.{ .type = "tool_result", .tool_call_id = toolCallId, .name = name, .text = text, .is_error = isError }},
            .tool_call_id = toolCallId,
            .state = "done",
        },
    }, .{}, &out.writer) catch return;
    const existing = svc.readFileBounded(allocator, path) catch "";
    defer allocator.free(existing);
    var full = std.ArrayList(u8).empty;
    defer full.deinit(allocator);
    full.appendSlice(allocator, existing) catch {};
    full.appendSlice(allocator, out.written()) catch {};
    full.append(allocator, '\n') catch {};
    svc.writeFileAtomic(allocator, path, full.items) catch {};
}

const llm_client = @import("llm-client.zig");

pub fn sendMessage(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct {
        id: []const u8 = "",
        message: []const u8 = "",
        content: []const u8 = "",
        files: []const []const u8 = &.{},
    };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid payload");
        return "";
    };
    defer parsed.deinit();
    const text = if (parsed.value.message.len > 0) parsed.value.message else parsed.value.content;
    if (parsed.value.id.len == 0 or text.len == 0) {
        try svc.failCtx(ctx, "id and message required");
        return "";
    }
    const id = parsed.value.id;

    // Persist the user message.
    const path = sessionPath(ctx.app.env_map, allocator, id);
    defer allocator.free(path);
    const msg_id = std.fmt.allocPrint(allocator, "msg-{d}", .{svc.nowMs()}) catch return "";
    defer allocator.free(msg_id);
    const ts = std.fmt.allocPrint(allocator, "{d}", .{svc.nowMs()}) catch return "";
    defer allocator.free(ts);
    var msg_out = std.Io.Writer.Allocating.init(allocator);
    defer msg_out.deinit();
    try std.json.Stringify.value(.{
        .type = "message",
        .id = msg_id,
        .parentId = null,
        .timestamp = ts,
        .message = .{
            .id = msg_id,
            .role = "user",
            .content = &.{.{ .type = "text", .text = text }},
            .timestamp = ts,
        },
    }, .{}, &msg_out.writer);

    const existing = svc.readFileBounded(allocator, path) catch "";
    defer allocator.free(existing);
    var full = std.ArrayList(u8).empty;
    defer full.deinit(allocator);
    try full.appendSlice(allocator, existing);
    try full.appendSlice(allocator, msg_out.written());
    try full.append(allocator, '\n');
    try svc.writeFileAtomic(allocator, path, full.items);

    emitEvent(ctx, "agent:updated", .{ .id = id });
    emitEvent(ctx, "agent:turn_start", .{ .id = id });

    // External ACP sessions route to the agent's own process (opencode/pi/...).
    if (sessionRole(ctx, id)) |role| {
        if (std.mem.startsWith(u8, role, "external:")) {
            const external_zig = @import("external-agent.zig");
            external_zig.sendExternalMessage(ctx, id, text) catch {
                emitEvent(ctx, "agent:error", .{ .id = id, .message = "external agent failed" });
                emitEvent(ctx, "agent:turn_end", .{ .id = id, .ok = false });
            };
            try svc.okCtx(ctx);
            return "";
        }
    }

    // Resolve the active provider.
    const target = loadActiveTarget(ctx, allocator) orelse {
        emitEvent(ctx, "agent:error", .{ .id = id, .message = "no active LLM provider configured — add a provider profile in Settings" });
        emitEvent(ctx, "agent:turn_end", .{ .id = id, .ok = false });
        try svc.okCtx(ctx);
        return "";
    };
    defer {
        allocator.free(target.providerId);
        allocator.free(target.baseURL);
        allocator.free(target.apiKey);
        allocator.free(target.model);
    }

    const session_messages = sessionToLlmMessages(allocator, path, 30);
    defer {
        for (session_messages) |m| {
            allocator.free(m.role);
            allocator.free(m.content);
        }
        allocator.free(session_messages);
    }

    // Build the system prompt + user messages.
    var msgs = std.ArrayList(llm_client.LLMMessage).empty;
    defer msgs.deinit(allocator);
    msgs.append(allocator, .{
        .role = "system",
        .content = "You are ForgeADE, an expert coding agent inside the user's IDE. Work in the current project. Read files before editing. Verify changes. Be concise but complete. You have tools: read_file(path), write_file(path, content), bash(command), glob(pattern), grep(query), list_dir(path).",
    }) catch {};
    for (session_messages) |m| msgs.append(allocator, m) catch {};

    var abort = std.atomic.Value(bool).init(false);

    // Streaming callbacks → events.
    const CbCtx = struct {
        ctx: *svc.Call,
        session_id: []const u8,
        a_msg_id: []const u8,
        fn onChunk(c: *anyopaque, delta_content: []const u8, delta_reasoning: []const u8) void {
            const self: *@This() = @ptrCast(@alignCast(c));
            std.debug.print("[agent] chunk c={d} r={d}\n", .{ delta_content.len, delta_reasoning.len });
            if (delta_content.len > 0) emitEvent(self.ctx, "agent:message_delta", .{ .id = self.session_id, .kind = "text", .delta = delta_content });
            if (delta_reasoning.len > 0) emitEvent(self.ctx, "agent:message_delta", .{ .id = self.session_id, .kind = "thinking", .delta = delta_reasoning });
        }
        fn onTool(c: *anyopaque, index: usize, t_id: []const u8, t_name: []const u8, t_args: []const u8) void {
            const self: *@This() = @ptrCast(@alignCast(c));
            emitEvent(self.ctx, "agent:tool_start", .{ .id = self.session_id, .index = index, .name = t_name, .toolCallId = t_id });
            if (t_args.len > 0) emitEvent(self.ctx, "agent:tool_delta", .{ .id = self.session_id, .index = index, .args = t_args });
        }
    };
    var cb_ctx = CbCtx{ .ctx = ctx, .session_id = id, .a_msg_id = msg_id };

    const a_msg_id = std.fmt.allocPrint(allocator, "msg-{d}-a", .{svc.nowMs()}) catch return "";
    defer allocator.free(a_msg_id);
    cb_ctx.a_msg_id = a_msg_id;
    emitEvent(ctx, "agent:message_start", .{ .id = id, .messageId = a_msg_id });

    // Core tool schemas (OpenAI function-calling format).
    const tool_defs = [_]llm_client.ToolDefinition{
        .{ .name = "bash", .description = "Run a shell command in the project directory and return its stdout.", .parameters = "{\"type\":\"object\",\"properties\":{\"command\":{\"type\":\"string\",\"description\":\"The shell command to run\"}},\"required\":[\"command\"]}" },
        .{ .name = "read_file", .description = "Read a file's contents from the project.", .parameters = "{\"type\":\"object\",\"properties\":{\"path\":{\"type\":\"string\"}},\"required\":[\"path\"]}" },
        .{ .name = "write_file", .description = "Write content to a file (creates parent dirs).", .parameters = "{\"type\":\"object\",\"properties\":{\"path\":{\"type\":\"string\"},\"content\":{\"type\":\"string\"}},\"required\":[\"path\",\"content\"]}" },
        .{ .name = "list_dir", .description = "List the files and directories at a path.", .parameters = "{\"type\":\"object\",\"properties\":{\"path\":{\"type\":\"string\"}},\"required\":[]}" },
        .{ .name = "glob", .description = "Find files matching a glob pattern under the project.", .parameters = "{\"type\":\"object\",\"properties\":{\"pattern\":{\"type\":\"string\"}},\"required\":[\"pattern\"]}" },
        .{ .name = "grep", .description = "Search file contents for a query string.", .parameters = "{\"type\":\"object\",\"properties\":{\"query\":{\"type\":\"string\"}},\"required\":[\"query\"]}" },
    };

    const result = llm_client.streamChat(allocator, &target, msgs.items, &tool_defs, .{
        .onChunk = CbCtx.onChunk,
        .onToolCallDelta = CbCtx.onTool,
        .ctx = &cb_ctx,
    }, &abort) catch |err| {
        const err_msg = std.fmt.allocPrint(allocator, "LLM call failed: {s}", .{@errorName(err)}) catch "LLM call failed";
        defer allocator.free(err_msg);
        emitEvent(ctx, "agent:error", .{ .id = id, .message = err_msg });
        emitEvent(ctx, "agent:message_end", .{ .id = id, .messageId = a_msg_id, .message = .{ .id = a_msg_id, .role = "assistant", .content = &.{}, .timestamp = ts, .state = "done" } });
        emitEvent(ctx, "agent:turn_end", .{ .id = id, .ok = false });
        try svc.okCtx(ctx);
        return "";
    };

    // Persist the assistant message + tool calls.
    var tool_calls = std.ArrayList(ToolCallEvent).empty;
    defer tool_calls.deinit(allocator);
    for (result.toolCalls) |tc| {
        tool_calls.append(allocator, .{
            .id = tc.id,
            .name = tc.name,
            .arguments = tc.arguments,
        }) catch {};
    }
    appendAssistantMessage(allocator, path, a_msg_id, ts, result.content, tool_calls.items);
    emitEvent(ctx, "agent:message_end", .{
        .id = id,
        .messageId = a_msg_id,
        .message = .{
            .id = a_msg_id,
            .role = "assistant",
            .content = &.{.{ .type = "text", .text = result.content }},
            .timestamp = ts,
            .state = "done",
        },
    });

    // Execute tool calls (simple sequential; no approval gate in bootstrap).
    const cwd = ctx.app.env_map.get("PWD") orelse "";
    // Also pick up text-embedded <tool_call>{json}</tool_call> blocks (some
    // models emit tool calls in text even when tools are declared).
    for (extractTextToolCalls(allocator, result.content)) |t| {
        tool_calls.append(allocator, t) catch {};
    }
    for (tool_calls.items, 0..) |tc, tool_index| {
        std.debug.print("[agent] executing tool {s} args={s}\n", .{ tc.name, tc.arguments });
        emitEvent(ctx, "agent:tool_start", .{ .id = id, .index = tool_index, .name = tc.name, .toolCallId = tc.id });
        const output = executeTool(ctx, allocator, tc.name, tc.arguments, cwd);
        std.debug.print("[agent] tool {s} -> {d} bytes\n", .{ tc.name, output.len });
        // Persist the tool result so the turn_end refetch keeps it visible.
        appendToolResultMessage(allocator, path, tc.id, tc.name, output, false);
        emitEvent(ctx, "agent:tool_end", .{
            .id = id,
            .index = tool_index,
            .toolCallId = tc.id,
            .name = tc.name,
            .result = output,
            .isError = false,
        });
        if (output.len > 0) allocator.free(output);
    }

    emitEvent(ctx, "agent:turn_end", .{
        .id = id,
        .ok = true,
        .usage = .{ .at = svc.nowMs(), .promptTokens = result.promptTokens, .completionTokens = result.completionTokens, .cachedTokens = result.cachedTokens, .durationMs = 0 },
        .contextWindow = target.contextWindow orelse 128_000,
    });
    // Free the LLM result — every reference to its strings was consumed above
    // (assistant message persist, tool loop, events).
    allocator.free(result.content);
    if (result.reasoning.len > 0) allocator.free(result.reasoning);
    for (result.toolCalls) |tc| {
        allocator.free(tc.id);
        allocator.free(tc.name);
        allocator.free(tc.arguments);
    }
    if (result.toolCalls.len > 0) allocator.free(result.toolCalls);
    try svc.okCtx(ctx);
    return "";
}

/// Parses `<tool_call>{ "name": ..., "arguments": {...} }</tool_call>` blocks
/// embedded in the assistant text into ToolCallEvent (owned copies).
fn extractTextToolCalls(allocator: std.mem.Allocator, content: []const u8) []ToolCallEvent {
    var out = std.ArrayList(ToolCallEvent).empty;
    var pos: usize = 0;
    while (std.mem.indexOfPos(u8, content, pos, "<tool_call>")) |start| {
        const body_start = start + "<tool_call>".len;
        const end = std.mem.indexOfPos(u8, content, body_start, "</tool_call>") orelse break;
        const body = content[body_start..end];
        pos = end + "</tool_call>".len;
        const parsed = std.json.parseFromSlice(std.json.Value, allocator, body, .{}) catch continue;
        defer parsed.deinit();
        switch (parsed.value) {
            .object => |obj| {
                var name: []const u8 = "";
                if (obj.get("name")) |v| {
                    if (v == .string) name = v.string;
                }
                if (name.len == 0) continue;
                var args_json: []const u8 = "";
                if (obj.get("arguments")) |v| {
                    switch (v) {
                        .object => |ao| {
                            _ = ao;
                            // Serialize the arguments Value via its own writer.
                            var w = std.Io.Writer.Allocating.init(allocator);
                            defer w.deinit();
                            std.json.Stringify.value(v, .{}, &w.writer) catch {};
                            if (w.written().len > 0) args_json = w.toOwnedSlice() catch "";
                        },
                        .string => |s| args_json = s,
                        else => {},
                    }
                }
                const id = std.fmt.allocPrint(allocator, "call-{d}-{d}", .{ svc.nowMs(), out.items.len }) catch continue;
                out.append(allocator, .{
                    .id = id,
                    .name = allocator.dupe(u8, name) catch continue,
                    .arguments = allocator.dupe(u8, if (args_json.len > 0) args_json else "{}") catch continue,
                }) catch {};
            },
            else => {},
        }
    }
    return out.toOwnedSlice(allocator) catch &.{};
}

pub fn stopTurn(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct { id: []const u8 = "" };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid payload");
        return "";
    };
    defer parsed.deinit();
    emitEvent(ctx, "agent:turn_end", .{ .id = parsed.value.id, .ok = false, .stopped = true });
    try svc.okCtx(ctx);
    return "";
}

pub fn respondApproval(ctx: *svc.Call) anyerror![]const u8 {
    try svc.okCtx(ctx);
    return "";
}

pub fn definitions(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const defs = loadDefinitions(ctx);
    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(defs, .{}, &out.writer);
    return out.toOwnedSlice();
}

pub fn saveDefinition(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const path = definitionsPath(ctx.app.env_map, allocator);
    defer allocator.free(path);
    const parsed = std.json.parseFromSlice(AgentDefinition, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid definition payload");
        return "";
    };
    defer parsed.deinit();

    const existing = loadDefinitions(ctx);
    var list = std.ArrayList(AgentDefinition).empty;
    var replaced = false;
    for (existing) |d| {
        if (std.mem.eql(u8, d.id, parsed.value.id)) {
            list.append(allocator, parsed.value) catch {};
            replaced = true;
        } else {
            list.append(allocator, d) catch {};
        }
    }
    if (!replaced) list.append(allocator, parsed.value) catch {};
    try svc.writeJsonFile(allocator, path, list.items);
    emitEvent(ctx, "agent:config:changed", .{});
    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(parsed.value, .{}, &out.writer);
    return out.toOwnedSlice();
}
