// services.workspace — current workspace + recent projects.
// The frontend kept workspace state in localStorage; the daemon-owned
// workspace.ts is the canonical source now. This mirrors both: the current
// workspace lives in ~/.forge-ade/workspace.json, recents in the same file.

const std = @import("std");
const svc = @import("../services.zig");

const Workspace = struct {
    name: []const u8 = "",
    folders: []const []const u8 = &.{},
    isTemporary: bool = true,
    filePath: []const u8 = "",
    theme: []const u8 = "dark-plus",
};

const RecentEntry = struct {
    path: []const u8,
    name: []const u8,
    isWorkspace: bool = false,
    lastOpened: i64 = 0,
    pinned: bool = false,
    favorite: bool = false,
};

const WorkspaceState = struct {
    workspace: ?Workspace = null,
    recent: []RecentEntry = &.{},
};

fn statePath(env_map: *std.process.Environ.Map, allocator: std.mem.Allocator) []const u8 {
    return std.fmt.allocPrint(allocator, "{s}/workspace.json", .{svc.dataDir(env_map)}) catch "";
}

fn loadState(ctx: *svc.Call) WorkspaceState {
    const allocator = ctx.app.allocator;
    const path = statePath(ctx.app.env_map, allocator);
    defer allocator.free(path);
    const raw = svc.readFileBounded(allocator, path) catch return .{};
    defer allocator.free(raw);
    const parsed = std.json.parseFromSlice(WorkspaceState, allocator, raw, .{ .ignore_unknown_fields = true }) catch return .{};
    defer parsed.deinit();
    // Deep-copy the strings so the state survives the parse teardown.
    var recent_list = std.ArrayList(RecentEntry).empty;
    for (parsed.value.recent) |r| {
        recent_list.append(allocator, .{
            .path = allocator.dupe(u8, r.path) catch continue,
            .name = allocator.dupe(u8, r.name) catch continue,
            .isWorkspace = r.isWorkspace,
            .lastOpened = r.lastOpened,
            .pinned = r.pinned,
            .favorite = r.favorite,
        }) catch {};
    }
    var ws: ?Workspace = null;
    if (parsed.value.workspace) |w| {
        var folders = std.ArrayList([]const u8).empty;
        for (w.folders) |f| folders.append(allocator, allocator.dupe(u8, f) catch continue) catch {};
        ws = .{
            .name = allocator.dupe(u8, w.name) catch "",
            .folders = folders.toOwnedSlice(allocator) catch &.{},
            .isTemporary = w.isTemporary,
            .filePath = allocator.dupe(u8, w.filePath) catch "",
            .theme = allocator.dupe(u8, w.theme) catch "dark-plus",
        };
    }
    return .{ .workspace = ws, .recent = recent_list.toOwnedSlice(allocator) catch &.{} };
}

pub fn get(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const state = loadState(ctx);
    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(state.workspace orelse Workspace{ .name = "", .folders = &.{} }, .{}, &out.writer);
    return out.toOwnedSlice();
}

pub fn open(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const path = statePath(ctx.app.env_map, allocator);
    defer allocator.free(path);

    const parsed = std.json.parseFromSlice(Workspace, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid workspace payload");
        return "";
    };
    defer parsed.deinit();

    var state = loadState(ctx);
    state.workspace = parsed.value;

    // Update recents (dedupe by path, most recent first, cap 50).
    const folder = if (parsed.value.folders.len > 0) parsed.value.folders[0] else "";
    if (folder.len > 0) {
        var list = std.ArrayList(RecentEntry).empty;
        var found = false;
        for (state.recent) |entry| {
            if (std.mem.eql(u8, entry.path, folder)) {
                var updated = entry;
                updated.lastOpened = svc.nowSec();
                found = true;
                try list.append(allocator, updated);
            } else {
                try list.append(allocator, entry);
            }
        }
        if (!found) {
            try list.insert(allocator, 0, .{
                .path = folder,
                .name = std.fs.path.basename(folder),
                .lastOpened = svc.nowSec(),
            });
        }
        state.recent = list.items;
        if (state.recent.len > 50) state.recent = state.recent[0..50];
    }

    try svc.writeJsonFile(allocator, path, state);
    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(state.workspace.?, .{}, &out.writer);
    return out.toOwnedSlice();
}

pub fn save(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const path = statePath(ctx.app.env_map, allocator);
    defer allocator.free(path);
    const parsed = std.json.parseFromSlice(Workspace, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid workspace payload");
        return "";
    };
    defer parsed.deinit();
    var state = loadState(ctx);
    state.workspace = parsed.value;
    try svc.writeJsonFile(allocator, path, state);
    try svc.okCtx(ctx);
    return "";
}

pub fn close(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const path = statePath(ctx.app.env_map, allocator);
    defer allocator.free(path);
    var state = loadState(ctx);
    state.workspace = null;
    try svc.writeJsonFile(allocator, path, state);
    try svc.okCtx(ctx);
    return "";
}

pub fn recent(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const state = loadState(ctx);
    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(state.recent, .{}, &out.writer);
    return out.toOwnedSlice();
}
