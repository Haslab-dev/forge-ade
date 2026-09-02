// services.lsp-manager — wire handlers for the LSP frontend contract.
// Manages per-language LSP clients and the diagnostics store.

const std = @import("std");
const svc = @import("../services.zig");
const lsp = @import("lsp.zig");

/// Portable nanosleep (Zig 0.16's std.Io.sleep needs an Io handle).
const Timespec = extern struct { tv_sec: isize, tv_nsec: isize };
extern "c" fn nanosleep(req: *const Timespec, rem: ?*Timespec) c_int;

/// Sleep-lock: spins briefly, then sleeps between polls. Handlers hold this
/// across LSP spawns/handshakes (seconds), so a pure spinlock would burn CPU.
const SpinLock = struct {
    locked: std.atomic.Value(bool) = std.atomic.Value(bool).init(false),
    fn lock(self: *SpinLock) void {
        var spins: u32 = 0;
        while (self.locked.swap(true, .acquire)) {
            spins += 1;
            if (spins > 64) {
                var ts: Timespec = .{ .tv_sec = 0, .tv_nsec = 200_000 };
                _ = nanosleep(&ts, null);
            } else {
                std.atomic.spinLoopHint();
            }
        }
    }
    fn unlock(self: *SpinLock) void {
        self.locked.store(false, .release);
    }
};

var mgr_mutex: SpinLock = .{};
var clients: ?std.StringHashMap(*lsp.Client) = null;
/// Aggregate diagnostics {filePath: {errors, warnings, diagnostics}}.
var diag_store: ?std.StringHashMap([]const u8) = null;

fn ensureStores() void {
    if (clients == null) {
        clients = std.StringHashMap(*lsp.Client).init(std.heap.c_allocator);
    }
    if (diag_store == null) {
        diag_store = std.StringHashMap([]const u8).init(std.heap.c_allocator);
    }
}

/// Returns the (possibly freshly started) client for `lang`. Callers must
/// hold mgr_mutex for as long as they use the returned client — restart/stop
/// also take the mutex, so holding it excludes deinit races.
fn getClientLocked(ctx: *svc.Call, lang: []const u8, workspace_root: []const u8) !*lsp.Client {
    ensureStores();
    if (clients.?.get(lang)) |c| {
        // Reuse if the root matches; else restart.
        if (std.mem.eql(u8, c.workspace_root, workspace_root)) return c;
        c.deinit();
        _ = clients.?.remove(lang);
    }
    const client = try lsp.start(ctx.app.allocator, lang, workspace_root, ctx.app.env_map);
    clients.?.put(lang, client) catch {
        client.deinit();
        return error.OutOfMemory;
    };
    return client;
}

fn emitDiagnostics(ctx: *svc.Call, file_path: []const u8, errors_count: usize, warnings_count: usize, diags: []const u8) void {
    const emit_fn = ctx.app.emit_fn orelse return;
    const emit_ctx = ctx.app.emit_ctx orelse return;
    var out = std.Io.Writer.Allocating.init(ctx.app.allocator);
    defer out.deinit();
    // Arena keeps the parsed diagnostics Value alive through stringify and
    // frees it with the buffer (the old code leaked a Parsed per emit).
    var arena = std.heap.ArenaAllocator.init(ctx.app.allocator);
    defer arena.deinit();
    const diags_value = std.json.parseFromSliceLeaky(std.json.Value, arena.allocator(), diags, .{}) catch std.json.Value{ .array = .empty };
    std.json.Stringify.value(.{
        .event = "lsp:diagnostics",
        .payload = .{
            .filePath = file_path,
            .errors = errors_count,
            .warnings = warnings_count,
            .diagnostics = diags_value,
        },
    }, .{}, &out.writer) catch return;
    emit_fn(emit_ctx, ctx.app.main_window_id, "services.agent", out.written()) catch {};
}

/// LSPDidOpen — { path, content }.
pub fn didOpen(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct { path: []const u8 = "", content: []const u8 = "" };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.okCtx(ctx);
        return "";
    };
    defer parsed.deinit();
    if (parsed.value.path.len == 0) {
        try svc.okCtx(ctx);
        return "";
    }
    const lang = lsp.languageIdFromPath(parsed.value.path);
    if (lang.len == 0) {
        try svc.okCtx(ctx);
        return "";
    }
    const ws = workspaceRoot(ctx);
    mgr_mutex.lock();
    defer mgr_mutex.unlock();
    const client = getClientLocked(ctx, lang, ws) catch {
        try svc.okCtx(ctx);
        return "";
    };
    lsp.didOpen(client, allocator, parsed.value.path, parsed.value.content, 1) catch {};
    try svc.okCtx(ctx);
    return "";
}

/// LSPDidChange — { path, content }.
pub fn didChange(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct { path: []const u8 = "", content: []const u8 = "" };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.okCtx(ctx);
        return "";
    };
    defer parsed.deinit();
    const lang = lsp.languageIdFromPath(parsed.value.path);
    if (lang.len == 0) {
        try svc.okCtx(ctx);
        return "";
    }
    mgr_mutex.lock();
    defer mgr_mutex.unlock();
    if (clients) |*cs| {
        if (cs.get(lang)) |client| {
            lsp.didChange(client, allocator, parsed.value.path, parsed.value.content, 2) catch {};
        }
    }
    try svc.okCtx(ctx);
    return "";
}

/// LSPDidSave — { path, content }.
pub fn didSave(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct { path: []const u8 = "", content: []const u8 = "" };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.okCtx(ctx);
        return "";
    };
    defer parsed.deinit();
    const lang = lsp.languageIdFromPath(parsed.value.path);
    if (lang.len > 0) {
        mgr_mutex.lock();
        defer mgr_mutex.unlock();
        if (clients) |*cs| {
            if (cs.get(lang)) |client| {
                lsp.didSave(client, allocator, parsed.value.path, parsed.value.content) catch {};
            }
        }
    }
    try svc.okCtx(ctx);
    return "";
}

/// LSPDidClose — { path }.
pub fn didClose(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct { path: []const u8 = "" };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.okCtx(ctx);
        return "";
    };
    defer parsed.deinit();
    const lang = lsp.languageIdFromPath(parsed.value.path);
    if (lang.len > 0) {
        mgr_mutex.lock();
        defer mgr_mutex.unlock();
        if (clients) |*cs| {
            if (cs.get(lang)) |client| {
                lsp.didClose(client, allocator, parsed.value.path) catch {};
            }
        }
    }
    try svc.okCtx(ctx);
    return "";
}

/// LSPGetCompletion — { path, line, character } → completion items.
pub fn getCompletion(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct { path: []const u8 = "", line: i64 = 0, character: i64 = 0 };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid payload");
        return "";
    };
    defer parsed.deinit();
    const lang = lsp.languageIdFromPath(parsed.value.path);
    if (lang.len == 0) {
        try svc.failCtx(ctx, "unsupported language");
        return "";
    }
    mgr_mutex.lock();
    defer mgr_mutex.unlock();
    const client = getClientLocked(ctx, lang, workspaceRoot(ctx)) catch {
        try svc.failCtx(ctx, "lsp server not available");
        return "";
    };
    const res = lsp.completion(client, allocator, parsed.value.path, parsed.value.line, parsed.value.character) catch {
        var out = std.Io.Writer.Allocating.init(allocator);
        try std.json.Stringify.value(@as([]struct {}, &.{}), .{}, &out.writer);
        return out.toOwnedSlice();
    };
    defer svc.deepFreeValue(allocator, res);
    // Normalize: result may be a list or {isIncomplete, items}.
    var out = std.Io.Writer.Allocating.init(allocator);
    switch (res) {
        .array => |arr| {
            try std.json.Stringify.value(arr.items, .{}, &out.writer);
        },
        .object => |obj| {
            if (obj.get("items")) |items| {
                try std.json.Stringify.value(items, .{}, &out.writer);
            } else {
                try std.json.Stringify.value(@as([]struct {}, &.{}), .{}, &out.writer);
            }
        },
        else => {
            try std.json.Stringify.value(@as([]struct {}, &.{}), .{}, &out.writer);
        },
    }
    return out.toOwnedSlice();
}

/// LSPGetHover — { path, line, character } → hover result or null.
pub fn getHover(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct { path: []const u8 = "", line: i64 = 0, character: i64 = 0 };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid payload");
        return "";
    };
    defer parsed.deinit();
    const lang = lsp.languageIdFromPath(parsed.value.path);
    if (lang.len == 0) {
        var out = std.Io.Writer.Allocating.init(allocator);
        try std.json.Stringify.value(null, .{}, &out.writer);
        return out.toOwnedSlice();
    }
    mgr_mutex.lock();
    defer mgr_mutex.unlock();
    const client = getClientLocked(ctx, lang, workspaceRoot(ctx)) catch {
        var out = std.Io.Writer.Allocating.init(allocator);
        try std.json.Stringify.value(null, .{}, &out.writer);
        return out.toOwnedSlice();
    };
    const res = lsp.hover(client, allocator, parsed.value.path, parsed.value.line, parsed.value.character) catch {
        var out = std.Io.Writer.Allocating.init(allocator);
        try std.json.Stringify.value(null, .{}, &out.writer);
        return out.toOwnedSlice();
    };
    defer svc.deepFreeValue(allocator, res);
    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(res, .{}, &out.writer);
    return out.toOwnedSlice();
}

/// LSPGetDefinition/Declaration/TypeDefinition/Implementation — location list.
fn locationResult(ctx: *svc.Call, method: enum { definition, declaration, type_definition, implementation }) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct { path: []const u8 = "", line: i64 = 0, character: i64 = 0 };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid payload");
        return "";
    };
    defer parsed.deinit();
    const lang = lsp.languageIdFromPath(parsed.value.path);
    if (lang.len == 0) {
        var out = std.Io.Writer.Allocating.init(allocator);
        try std.json.Stringify.value(@as([]struct {}, &.{}), .{}, &out.writer);
        return out.toOwnedSlice();
    }
    mgr_mutex.lock();
    defer mgr_mutex.unlock();
    const client = getClientLocked(ctx, lang, workspaceRoot(ctx)) catch {
        var out = std.Io.Writer.Allocating.init(allocator);
        try std.json.Stringify.value(@as([]struct {}, &.{}), .{}, &out.writer);
        return out.toOwnedSlice();
    };
    const res = switch (method) {
        .definition => lsp.definition(client, allocator, parsed.value.path, parsed.value.line, parsed.value.character),
        .declaration => lsp.declaration(client, allocator, parsed.value.path, parsed.value.line, parsed.value.character),
        .type_definition => lsp.typeDefinition(client, allocator, parsed.value.path, parsed.value.line, parsed.value.character),
        .implementation => lsp.implementation(client, allocator, parsed.value.path, parsed.value.line, parsed.value.character),
    } catch {
        var out = std.Io.Writer.Allocating.init(allocator);
        try std.json.Stringify.value(@as([]struct {}, &.{}), .{}, &out.writer);
        return out.toOwnedSlice();
    };
    defer svc.deepFreeValue(allocator, res);
    var out = std.Io.Writer.Allocating.init(allocator);
    switch (res) {
        .array => |arr| try std.json.Stringify.value(arr.items, .{}, &out.writer),
        .object => try std.json.Stringify.value(res, .{}, &out.writer),
        else => try std.json.Stringify.value(@as([]struct {}, &.{}), .{}, &out.writer),
    }
    return out.toOwnedSlice();
}

pub fn getDefinition(ctx: *svc.Call) anyerror![]const u8 {
    return locationResult(ctx, .definition);
}
pub fn getDeclaration(ctx: *svc.Call) anyerror![]const u8 {
    return locationResult(ctx, .declaration);
}
pub fn getTypeDefinition(ctx: *svc.Call) anyerror![]const u8 {
    return locationResult(ctx, .type_definition);
}
pub fn getImplementation(ctx: *svc.Call) anyerror![]const u8 {
    return locationResult(ctx, .implementation);
}

/// LSPGetDiagnostics — { path } → {filePath: {errors, warnings, diagnostics}}.
pub fn getDiagnostics(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    mgr_mutex.lock();
    defer mgr_mutex.unlock();
    // One arena for every per-entry parse (the old code leaked a Parsed per
    // store entry per call).
    var arena = std.heap.ArenaAllocator.init(allocator);
    defer arena.deinit();
    var out_map = std.json.ObjectMap.empty;
    if (diag_store) |*store| {
        var it = store.iterator();
        while (it.next()) |e| {
            const parsed = std.json.parseFromSliceLeaky(std.json.Value, arena.allocator(), e.value_ptr.*, .{}) catch continue;
            out_map.put(allocator, e.key_ptr.*, parsed) catch {};
        }
    }
    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(std.json.Value{ .object = out_map }, .{}, &out.writer);
    return out.toOwnedSlice();
}

/// LSPListServers — server infos.
pub fn listServers(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    ensureStores();
    mgr_mutex.lock();
    defer mgr_mutex.unlock();
    var list = std.ArrayList(lsp.ServerInfo).empty;
    defer list.deinit(allocator);
    if (clients) |*cs| {
        var it = cs.iterator();
        while (it.next()) |e| {
            list.append(allocator, lsp.serverInfo(e.value_ptr.*)) catch {};
        }
    }
    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(list.items, .{}, &out.writer);
    return out.toOwnedSlice();
}

/// LSPRestartServer — { languageId } → true.
pub fn restartServer(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct { languageId: []const u8 = "", id: []const u8 = "" };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.okCtx(ctx);
        return "";
    };
    defer parsed.deinit();
    const lang = if (parsed.value.languageId.len > 0) parsed.value.languageId else parsed.value.id;
    ensureStores();
    mgr_mutex.lock();
    defer mgr_mutex.unlock();
    if (lang.len > 0) {
        if (clients) |*cs| {
            if (cs.get(lang)) |client| {
                client.deinit();
                _ = cs.remove(lang);
            }
        }
    }
    try svc.okCtx(ctx);
    return "";
}

/// LSPStopServer — { languageId } → true.
pub fn stopServer(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct { languageId: []const u8 = "", id: []const u8 = "" };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.okCtx(ctx);
        return "";
    };
    defer parsed.deinit();
    const lang = if (parsed.value.languageId.len > 0) parsed.value.languageId else parsed.value.id;
    ensureStores();
    mgr_mutex.lock();
    defer mgr_mutex.unlock();
    if (lang.len > 0) {
        if (clients) |*cs| {
            if (cs.get(lang)) |client| {
                client.deinit();
                _ = cs.remove(lang);
            }
        }
    }
    try svc.okCtx(ctx);
    return "";
}

/// LSPRestartAll → {lang: true}.
pub fn restartAll(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    ensureStores();
    mgr_mutex.lock();
    defer mgr_mutex.unlock();
    var map = std.json.ObjectMap.empty;
    if (clients) |*cs| {
        var it = cs.iterator();
        while (it.next()) |e| {
            e.value_ptr.*.deinit();
            map.put(allocator, e.key_ptr.*, .{ .bool = true }) catch {};
        }
        cs.clearRetainingCapacity();
    }
    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(std.json.Value{ .object = map }, .{}, &out.writer);
    return out.toOwnedSlice();
}

/// LSPStopAll → true.
pub fn stopAll(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    ensureStores();
    mgr_mutex.lock();
    defer mgr_mutex.unlock();
    if (clients) |*cs| {
        var it = cs.iterator();
        while (it.next()) |e| {
            e.value_ptr.*.deinit();
        }
        cs.clearRetainingCapacity();
    }
    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(true, .{}, &out.writer);
    return out.toOwnedSlice();
}

/// LSPGetServerLogs — { languageId } → [].
pub fn getServerLogs(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(@as([]struct {}, &.{}), .{}, &out.writer);
    return out.toOwnedSlice();
}

fn workspaceRoot(ctx: *svc.Call) []const u8 {
    const env_map = ctx.app.env_map;
    if (env_map.get("PWD")) |pwd| return pwd;
    return ".";
}
