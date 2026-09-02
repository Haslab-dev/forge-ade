// services.config — provider profiles + model catalog stored at ~/.forge-ade.
// Port of src/server/config.ts's canonical models.json shape.

const std = @import("std");
const svc = @import("../services.zig");

const ModelMeta = struct {
    id: []const u8,
    name: ?[]const u8 = null,
    reasoning: ?bool = null,
    input: ?[]const []const u8 = null,
    context_window: ?u32 = null,
    max_tokens: ?u32 = null,
};

const ProviderAuth = struct {
    id: []const u8,
    name: []const u8,
    api: []const u8,
    base_url: []const u8,
    api_key: []const u8,
    auth: []const u8 = "apiKey",
    active_model: []const u8,
    models: []ModelMeta = &.{},
    selected_models: ?[]const []const u8 = null,
    enabled: ?bool = null,
};

const ModelsFile = struct {
    version: u32 = 1,
    default_provider: []const u8 = "",
    providers: []ProviderAuth = &.{},
};

/// ~/.forge-ade/models.json
pub fn modelsPath(env_map: *std.process.Environ.Map, allocator: std.mem.Allocator) []const u8 {
    return std.fmt.allocPrint(allocator, "{s}/models.json", .{svc.dataDir(env_map)}) catch "";
}

pub fn get(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const path = modelsPath(ctx.app.env_map, allocator);
    defer allocator.free(path);

    var out = std.Io.Writer.Allocating.init(allocator);
    // Keep the raw buffer alive while we stringify the parsed value (the
    // parsed strings alias it — freeing first would dangle them).
    const raw = svc.readFileBounded(allocator, path) catch {
        try std.json.Stringify.value(.{
            .version = 1,
            .default_provider = "",
            .providers = &.{},
        }, .{}, &out.writer);
        return out.toOwnedSlice();
    };
    defer allocator.free(raw);
    if (std.json.parseFromSlice(ModelsFile, allocator, raw, .{ .ignore_unknown_fields = true })) |models| {
        defer models.deinit();
        try std.json.Stringify.value(.{
            .version = models.value.version,
            .default_provider = models.value.default_provider,
            .providers = models.value.providers,
        }, .{}, &out.writer);
    } else |_| {
        try std.json.Stringify.value(.{
            .version = 1,
            .default_provider = "",
            .providers = &.{},
        }, .{}, &out.writer);
    }
    return out.toOwnedSlice();
}

pub fn save(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const path = modelsPath(ctx.app.env_map, allocator);
    defer allocator.free(path);

    const parsed = std.json.parseFromSlice(ModelsFile, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid models payload");
        return "";
    };
    defer parsed.deinit();

    try svc.writeJsonFile(allocator, path, parsed.value);
    try svc.okCtx(ctx);
    return "";
}
