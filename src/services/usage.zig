// services.usage — token-usage journal at ~/.forge-ade/usage/usage.jsonl.
// Port of src/server/usage.ts (global JSONL journal + aggregates).

const std = @import("std");
const svc = @import("../services.zig");

const UsageRecord = struct {
    ts: i64,
    provider: []const u8,
    model: []const u8,
    workspace: []const u8 = "",
    sessionId: []const u8 = "",
    inputTokens: i64,
    outputTokens: i64,
    cachedTokens: i64 = 0,
    latencyMs: i64 = 0,
};

fn usageFilePath(env_map: *std.process.Environ.Map, allocator: std.mem.Allocator) []const u8 {
    return std.fmt.allocPrint(allocator, "{s}/usage/usage.jsonl", .{svc.dataDir(env_map)}) catch "";
}

/// Reads all journal lines into a list (caller frees records + backing store).
fn readRecords(ctx: *svc.Call) []UsageRecord {
    const allocator = ctx.app.allocator;
    const path = usageFilePath(ctx.app.env_map, allocator);
    defer allocator.free(path);
    const raw = svc.readFileBounded(allocator, path) catch return &.{};
    defer allocator.free(raw);

    var list = std.ArrayList(UsageRecord).empty;
    var lines = std.mem.splitScalar(u8, raw, '\n');
    while (lines.next()) |line| {
        if (line.len == 0) continue;
        const parsed = std.json.parseFromSlice(UsageRecord, allocator, line, .{ .ignore_unknown_fields = true }) catch continue;
        // Ownership: parsed.value points into `line`'s backing buffer (raw),
        // which is freed on return — so deep-copy the strings.
        const rec = parsed.value;
        const copy = UsageRecord{
            .ts = rec.ts,
            .provider = allocator.dupe(u8, rec.provider) catch continue,
            .model = allocator.dupe(u8, rec.model) catch continue,
            .workspace = allocator.dupe(u8, rec.workspace) catch continue,
            .sessionId = allocator.dupe(u8, rec.sessionId) catch continue,
            .inputTokens = rec.inputTokens,
            .outputTokens = rec.outputTokens,
            .cachedTokens = rec.cachedTokens,
            .latencyMs = rec.latencyMs,
        };
        list.append(allocator, copy) catch {};
        parsed.deinit();
    }
    return list.toOwnedSlice(allocator) catch &.{};
}

fn freeRecords(allocator: std.mem.Allocator, recs: []UsageRecord) void {
    for (recs) |rec| {
        allocator.free(rec.provider);
        allocator.free(rec.model);
        allocator.free(rec.workspace);
        allocator.free(rec.sessionId);
    }
    allocator.free(recs);
}

fn cutoffFor(filter: []const u8) i64 {
    if (std.mem.eql(u8, filter, "all") or filter.len == 0) return 0;
    if (std.mem.eql(u8, filter, "today")) {
        // Midnight UTC-ish (approximation is fine for a local journal).
        const now = @divTrunc(svc.nowMs(), 1000);
        const day_secs: i64 = 86_400;
        return now - @mod(now, day_secs);
    }
    // "24h" / "7d" / "30d"
    var days: i64 = 1;
    if (std.mem.eql(u8, filter, "7d")) days = 7;
    if (std.mem.eql(u8, filter, "30d")) days = 30;
    return @divTrunc(svc.nowMs(), 1000) - days * 86_400;
}

fn basenameOf(allocator: std.mem.Allocator, folder: []const u8) []const u8 {
    const trimmed = std.mem.trimRight(u8, folder, "/");
    if (std.fs.path.basename(trimmed).len == 0) return trimmed;
    return allocator.dupe(u8, std.fs.path.basename(trimmed)) catch trimmed;
}

/// Returns all records in the window (ts >= cutoff), newest first.
fn inWindow(ctx: *svc.Call, filter: []const u8, out: *std.ArrayList(UsageRecord)) !void {
    const allocator = ctx.app.allocator;
    const cutoff = cutoffFor(filter);
    const all = readRecords(ctx);
    defer freeRecords(ctx.app.allocator, all);
    for (all) |rec| {
        if (rec.ts >= cutoff) try out.append(allocator, rec);
    }
}

pub fn records(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const all = readRecords(ctx);
    defer freeRecords(allocator, all);
    var out = std.Io.Writer.Allocating.init(allocator);
    // Raw journal rows (all time) — the shape the frontend aggregator consumes.
    var rows = std.ArrayList(struct { ts: i64, provider: []const u8, model: []const u8, workspace: []const u8, sessionId: []const u8, inputTokens: i64, outputTokens: i64, cachedTokens: i64, latencyMs: i64 }).empty;
    defer rows.deinit(allocator);
    for (all) |rec| {
        try rows.append(allocator, .{
            .ts = rec.ts,
            .provider = rec.provider,
            .model = rec.model,
            .workspace = rec.workspace,
            .sessionId = rec.sessionId,
            .inputTokens = rec.inputTokens,
            .outputTokens = rec.outputTokens,
            .cachedTokens = rec.cachedTokens,
            .latencyMs = rec.latencyMs,
        });
    }
    try std.json.Stringify.value(rows.items, .{}, &out.writer);
    return out.toOwnedSlice();
}

pub fn overview(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const filter = svc.getString(allocator, ctx, "filter") orelse "today";
    defer if (!std.mem.eql(u8, filter, "today")) allocator.free(filter);

    var rows = std.ArrayList(UsageRecord).empty;
    defer rows.deinit(allocator);
    try inWindow(ctx, filter, &rows);

    var inputTokens: i64 = 0;
    var outputTokens: i64 = 0;
    var cachedTokens: i64 = 0;
    var latency: i64 = 0;
    for (rows.items) |r| {
        inputTokens += r.inputTokens;
        outputTokens += r.outputTokens;
        cachedTokens += r.cachedTokens;
        latency += r.latencyMs;
    }
    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(.{
        .totalTokens = inputTokens + outputTokens,
        .inputTokens = inputTokens,
        .outputTokens = outputTokens,
        .cachedTokens = cachedTokens,
        .totalCost = 0,
        .requestCount = rows.items.len,
        .avgLatencyMs = if (rows.items.len > 0) @as(i64, @intCast(@divTrunc(latency, @as(i64, @intCast(rows.items.len))))) else 0,
    }, .{}, &out.writer);
    return out.toOwnedSlice();
}
