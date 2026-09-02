// services.llm — provider profiles + active model.
// Port of src/server/llm.ts: reads ~/.forge-ade/models.json and returns the
// ProviderProfile[] array shape the frontend's GetProviderProfiles consumes.

const std = @import("std");
const svc = @import("../services.zig");
const config = @import("config.zig");

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
    // passthrough extras (projectId/accountEmail/refreshToken for antigravity)
    projectId: ?[]const u8 = null,
    accountEmail: ?[]const u8 = null,
    refreshToken: ?[]const u8 = null,
};

/// Parses the providers object into a ProviderAuth list (allocation-free view
/// into the parsed Value's arena — strings alias the Value's backing store).
fn providersList(allocator: std.mem.Allocator, value: *const std.json.Value) []ProviderAuth {
    var list = std.ArrayList(ProviderAuth).empty;
    switch (value.*) {
        .object => |obj| {
            var it = obj.iterator();
            while (it.next()) |entry| {
                const id = entry.key_ptr.*;
                switch (entry.value_ptr.*) {
                    .object => |p| {
                        const rec = ProviderAuth{
                            .id = id,
                            .name = strField(&p, "name") orelse id,
                            .api = strField(&p, "api") orelse "openai-completions",
                            .base_url = strField(&p, "base_url") orelse "",
                            .api_key = strField(&p, "api_key") orelse "",
                            .active_model = strField(&p, "active_model") orelse "",
                            .models = modelsList(allocator, &p) catch &.{},
                            .selected_models = strListField(allocator, &p, "selected_models"),
                            .enabled = boolField(&p, "enabled"),
                            .projectId = strField(&p, "projectId"),
                            .accountEmail = strField(&p, "accountEmail"),
                            .refreshToken = strField(&p, "refreshToken"),
                        };
                        list.append(allocator, rec) catch {};
                    },
                    else => {},
                }
            }
        },
        else => {},
    }
    return list.toOwnedSlice(allocator) catch &.{};
}

fn strField(obj: *const std.json.ObjectMap, key: []const u8) ?[]const u8 {
    if (obj.get(key)) |v| {
        switch (v) {
            .string => |s| return s,
            else => {},
        }
    }
    return null;
}

fn boolField(obj: *const std.json.ObjectMap, key: []const u8) ?bool {
    if (obj.get(key)) |v| {
        switch (v) {
            .bool => |b| return b,
            else => {},
        }
    }
    return null;
}

fn strListField(allocator: std.mem.Allocator, obj: *const std.json.ObjectMap, key: []const u8) ?[]const []const u8 {
    if (obj.get(key)) |v| {
        switch (v) {
            .array => |arr| {
                var list = std.ArrayList([]const u8).empty;
                for (arr.items) |item| {
                    switch (item) {
                        // Deep-copy each string so the result survives the
                        // parse arena teardown (parseProviders frees the raw
                        // file buffer right after).
                        .string => |s| list.append(allocator, allocator.dupe(u8, s) catch continue) catch {},
                        else => {},
                    }
                }
                return list.toOwnedSlice(allocator) catch null;
            },
            else => {},
        }
    }
    return null;
}

fn modelsList(allocator: std.mem.Allocator, obj: *const std.json.ObjectMap) ![]ModelMeta {
    var list = std.ArrayList(ModelMeta).empty;
    if (obj.get("models")) |v| {
        switch (v) {
            .array => |arr| {
                for (arr.items) |item| {
                    switch (item) {
                        .string => |s| {
                            list.append(allocator, .{ .id = allocator.dupe(u8, s) catch continue }) catch {};
                        },
                        .object => |m| {
                            const mid = strField(&m, "id") orelse continue;
                            list.append(allocator, .{
                                // Deep-copy the model metadata strings.
                                .id = allocator.dupe(u8, mid) catch continue,
                                .name = if (strField(&m, "name")) |n| allocator.dupe(u8, n) catch null else null,
                                .reasoning = boolField(&m, "reasoning"),
                                .context_window = intField(&m, "context_window"),
                                .max_tokens = intField(&m, "max_tokens"),
                            }) catch {};
                        },
                        else => {},
                    }
                }
            },
            else => {},
        }
    }
    return list.toOwnedSlice(allocator);
}

fn intField(obj: *const std.json.ObjectMap, key: []const u8) ?u32 {
    if (obj.get(key)) |v| {
        switch (v) {
            .integer => |i| return @intCast(@max(i, 0)),
            .float => |f| return @intFromFloat(@max(f, 0)),
            else => {},
        }
    }
    return null;
}

/// The ProviderProfile[] the frontend expects from GetProviderProfiles.
const ProviderProfile = struct {
    id: []const u8,
    name: []const u8,
    provider: []const u8,
    apiKey: []const u8,
    baseURL: []const u8,
    activeModel: []const u8,
    models: []ModelMeta,
    selected_models: []const []const u8,
    enabled: bool,
    projectId: ?[]const u8 = null,
    accountEmail: ?[]const u8 = null,
    refreshToken: ?[]const u8 = null,
};

/// Reads + parses the providers from models.json into a list. The returned
/// slices alias the parsed Value's arena; callers must keep the file's raw
/// bytes alive for the lifetime of the result (we re-read here to keep the
/// parse self-contained).
fn parseProviders(ctx: *svc.Call) []ProviderAuth {
    const allocator = ctx.app.allocator;
    const path = config.modelsPath(ctx.app.env_map, allocator);
    defer allocator.free(path);
    const raw = svc.readFileBounded(allocator, path) catch return &.{};
    defer allocator.free(raw);
    const parsed = std.json.parseFromSlice(std.json.Value, allocator, raw, .{ .ignore_unknown_fields = true }) catch return &.{};
    defer parsed.deinit();
    switch (parsed.value) {
        .object => |root| {
            if (root.get("providers")) |*providers_value| {
                const list = providersList(allocator, providers_value);
                defer allocator.free(list);
                // Deep-copy strings out of the parsed Value's arena so they
                // survive past parsed.deinit().
                var copies = std.ArrayList(ProviderAuth).empty;
                for (list) |p| {
                    copies.append(allocator, .{
                        .id = allocator.dupe(u8, p.id) catch continue,
                        .name = allocator.dupe(u8, p.name) catch continue,
                        .api = allocator.dupe(u8, p.api) catch continue,
                        .base_url = allocator.dupe(u8, p.base_url) catch continue,
                        .api_key = allocator.dupe(u8, p.api_key) catch continue,
                        .active_model = allocator.dupe(u8, p.active_model) catch continue,
                        .models = p.models,
                        .selected_models = p.selected_models,
                        .enabled = p.enabled,
                        .projectId = if (p.projectId) |v| allocator.dupe(u8, v) catch null else null,
                        .accountEmail = if (p.accountEmail) |v| allocator.dupe(u8, v) catch null else null,
                        .refreshToken = if (p.refreshToken) |v| allocator.dupe(u8, v) catch null else null,
                    }) catch {};
                }
                return copies.toOwnedSlice(allocator) catch &.{};
            }
        },
        else => {},
    }
    return &.{};
}

/// Returns the ProviderProfile[] array (the GetProviderProfiles wire shape).
pub fn profiles(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const providers = parseProviders(ctx);
    defer {
        for (providers) |p| {
            allocator.free(p.id);
            allocator.free(p.name);
            allocator.free(p.api);
            allocator.free(p.base_url);
            allocator.free(p.api_key);
            allocator.free(p.active_model);
            if (p.projectId) |v| allocator.free(v);
            if (p.accountEmail) |v| allocator.free(v);
            if (p.refreshToken) |v| allocator.free(v);
            for (p.models) |mm| {
                allocator.free(mm.id);
                if (mm.name) |n| allocator.free(n);
            }
            if (p.models.len > 0) allocator.free(p.models);
            if (p.selected_models) |sel| {
                for (sel) |s| allocator.free(s);
                allocator.free(sel);
            }
        }
        allocator.free(providers);
    }
    var out = std.Io.Writer.Allocating.init(allocator);

    var list = std.ArrayList(ProviderProfile).empty;
    defer {
        for (list.items) |p| allocator.free(p.selected_models);
        list.deinit(allocator);
    }
    for (providers) |p| {
        // selected_models: explicit selection filtered to catalog, else all ids.
        var all_ids = std.ArrayList([]const u8).empty;
        for (p.models) |mm| all_ids.append(allocator, mm.id) catch {};
        var selection = std.ArrayList([]const u8).empty;
        if (p.selected_models) |sel| {
            for (sel) |id| {
                var found = false;
                for (all_ids.items) |a| {
                    if (std.mem.eql(u8, id, a)) { found = true; break; }
                }
                if (found) selection.append(allocator, id) catch {};
            }
            if (selection.items.len == 0) {
                for (all_ids.items) |a| selection.append(allocator, a) catch {};
            }
        } else {
            for (all_ids.items) |a| selection.append(allocator, a) catch {};
        }
        const sel = selection.toOwnedSlice(allocator) catch &.{};
        list.append(allocator, .{
            .id = p.id,
            .name = p.name,
            .provider = p.api,
            .apiKey = p.api_key,
            .baseURL = p.base_url,
            .activeModel = p.active_model,
            .models = p.models,
            .selected_models = sel,
            .enabled = p.enabled orelse true,
            .projectId = p.projectId,
            .accountEmail = p.accountEmail,
            .refreshToken = p.refreshToken,
        }) catch {};
    }
    try std.json.Stringify.value(list.items, .{}, &out.writer);
    return out.toOwnedSlice();
}

/// GetLLMConfig shape: { provider_id, api_key, base_url, model, activeProfile, profiles }.
pub fn active(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const providers = parseProviders(ctx);
    defer {
        for (providers) |p| {
            allocator.free(p.id);
            allocator.free(p.name);
            allocator.free(p.api);
            allocator.free(p.base_url);
            allocator.free(p.api_key);
            allocator.free(p.active_model);
            if (p.projectId) |v| allocator.free(v);
            if (p.accountEmail) |v| allocator.free(v);
            if (p.refreshToken) |v| allocator.free(v);
            for (p.models) |mm| {
                allocator.free(mm.id);
                if (mm.name) |n| allocator.free(n);
            }
            if (p.models.len > 0) allocator.free(p.models);
            if (p.selected_models) |sel| {
                for (sel) |s| allocator.free(s);
                allocator.free(sel);
            }
        }
        allocator.free(providers);
    }
    // Read default_provider.
    const path = config.modelsPath(ctx.app.env_map, allocator);
    defer allocator.free(path);
    var default_provider: []const u8 = "";
    if (svc.readFileBounded(allocator, path)) |raw| {
        defer allocator.free(raw);
        if (std.json.parseFromSlice(std.json.Value, allocator, raw, .{ .ignore_unknown_fields = true })) |parsed| {
            defer parsed.deinit();
            switch (parsed.value) {
                .object => |root| {
                    if (root.get("default_provider")) |v| {
                        switch (v) {
                            .string => |s| default_provider = s,
                            else => {},
                        }
                    }
                },
                else => {},
            }
        } else |_| {}
    } else |_| {}

    var out = std.Io.Writer.Allocating.init(allocator);

    const empty = struct {
        fn write(o: anytype) !void {
            try std.json.Stringify.value(.{
                .provider_id = "",
                .api_key = "",
                .base_url = "",
                .model = "",
                .activeProfile = null,
                .profiles = @as([]ProviderProfile, &.{}),
            }, .{}, o);
        }
    };

    var active_profile: ?ProviderAuth = null;
    if (default_provider.len > 0) {
        for (providers) |p| {
            if (std.mem.eql(u8, p.id, default_provider)) { active_profile = p; break; }
        }
    }
    if (active_profile == null and providers.len > 0) active_profile = providers[0];
    const p = active_profile orelse {
        try empty.write(&out.writer);
        return out.toOwnedSlice();
    };
    try std.json.Stringify.value(.{
        .provider_id = p.id,
        .api_key = p.api_key,
        .base_url = p.base_url,
        .model = p.active_model,
        .activeProfile = .{
            .id = p.id,
            .name = p.name,
            .provider = p.api,
            .apiKey = p.api_key,
            .baseURL = p.base_url,
            .activeModel = p.active_model,
            .models = p.models,
            .selected_models = p.selected_models orelse &.{},
            .enabled = p.enabled orelse true,
        },
        .profiles = @as([]ProviderProfile, &.{}),
    }, .{}, &out.writer);
    return out.toOwnedSlice();
}

pub fn saveProfiles(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const path = config.modelsPath(ctx.app.env_map, allocator);
    defer allocator.free(path);

    // Single arena for BOTH the file and the incoming payload: every
    // ObjectMap.put below runs against arena-owned maps (see setActiveModel).
    var arena = std.heap.ArenaAllocator.init(allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const raw = svc.readFileBounded(allocator, path) catch return "";
    defer allocator.free(raw);
    var root = std.json.parseFromSliceLeaky(std.json.Value, a, raw, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "models.json unparsable");
        return "";
    };

    const Payload = struct { profiles: ?[]const std.json.Value = null };
    const payload = std.json.parseFromSliceLeaky(Payload, a, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid profiles payload");
        return "";
    };
    const incoming = payload.profiles orelse &.{};

    switch (root) {
        .object => |*root_obj| {
            var providers_obj: std.json.ObjectMap = .empty;
            for (incoming) |item| {
                switch (item) {
                    .object => |p| {
                        const id = strField(&p, "id") orelse strField(&p, "provider_id") orelse continue;
                        // Preserve existing catalog/active_model when absent.
                        var merged = p;
                        if (root_obj.getPtr("providers")) |pv| {
                            switch (pv.*) {
                                .object => |existing| {
                                    if (existing.getPtr(id)) |ep| {
                                        switch (ep.*) {
                                            .object => |*eo| {
                                                if (merged.getPtr("models") == null) {
                                                    if (eo.getPtr("models")) |em| try merged.put(a, "models", em.*);
                                                }
                                                if ((merged.getPtr("active_model") orelse merged.getPtr("activeModel")) == null) {
                                                    if (eo.getPtr("active_model")) |am| try merged.put(a, "active_model", am.*);
                                                }
                                            },
                                            else => {},
                                        }
                                    }
                                },
                                else => {},
                            }
                        }
                        try providers_obj.put(a, id, .{ .object = merged });
                    },
                    else => {},
                }
            }
            try root_obj.put(a, "providers", .{ .object = providers_obj });
        },
        else => {},
    }
    try svc.writeJsonFile(allocator, path, root);
    try svc.okCtx(ctx);
    return "";
}

/// SetActiveModel — { providerId, model } → set active_model + default_provider.
///
/// Mutation safety: models.json is parsed with parseFromSliceLeaky into a
/// local arena and every ObjectMap.put uses THAT arena's allocator. The
/// parsed map's internal storage is arena-owned, so growing it with the
/// c_allocator would free arena memory ("pointer being freed was not
/// allocated" → crash).
pub fn setActiveModel(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const path = config.modelsPath(ctx.app.env_map, allocator);
    defer allocator.free(path);

    var arena = std.heap.ArenaAllocator.init(allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const Payload = struct { providerId: []const u8 = "", model: []const u8 = "" };
    const payload = std.json.parseFromSliceLeaky(Payload, a, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid payload");
        return "";
    };

    const raw = svc.readFileBounded(allocator, path) catch return "";
    defer allocator.free(raw);
    var root = std.json.parseFromSliceLeaky(std.json.Value, a, raw, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "models.json unparsable");
        return "";
    };

    switch (root) {
        .object => |*root_obj| {
            try root_obj.put(a, "default_provider", .{ .string = payload.providerId });
            if (root_obj.getPtr("providers")) |pv| {
                switch (pv.*) {
                    .object => |*providers_obj| {
                        if (providers_obj.getPtr(payload.providerId)) |p| {
                            switch (p.*) {
                                .object => |*po| try po.put(a, "active_model", .{ .string = payload.model }),
                                else => {},
                            }
                        }
                    },
                    else => {},
                }
            }
        },
        else => {},
    }
    try svc.writeJsonFile(allocator, path, root);
    try svc.okCtx(ctx);
    return "";
}

test "parseProviders strings survive the parse arena and stringify cleanly" {
    // Regression test for the `@memcpy arguments alias` panic: parseProviders
    // must deep-copy models/selected_models (not just the top-level fields),
    // so stringifying after the raw buffer is freed never aliases the writer's
    // own buffer during growth.
    const allocator = std.testing.allocator;

    const sample =
        \\{"version":1,"default_provider":"anthropic","providers":{
        \\"anthropic":{"id":"anthropic","name":"Anthropic","api":"anthropic","base_url":"https://api.anthropic.com","api_key":"sk-test","active_model":"claude-3-7-sonnet","models":[{"id":"claude-3-7-sonnet-20250219","name":"Claude 3.7 Sonnet","context_window":200000},{"id":"claude-3-5-haiku-20241022"}],"selected_models":["claude-3-7-sonnet-20250219"]}
        \\}}
    ;

    // Mimic parseProviders: parse, deep-copy, free the raw buffer.
    const parsed = try std.json.parseFromSlice(std.json.Value, allocator, sample, .{ .ignore_unknown_fields = true });
    defer parsed.deinit();
    const providers = parsed.value.object.get("providers").?;

    // Build a ProviderAuth with deep copies the way providersList does.
    const auth = ProviderAuth{
        .id = allocator.dupe(u8, "anthropic") catch unreachable,
        .name = allocator.dupe(u8, "Anthropic") catch unreachable,
        .api = allocator.dupe(u8, "anthropic") catch unreachable,
        .base_url = allocator.dupe(u8, "https://api.anthropic.com") catch unreachable,
        .api_key = allocator.dupe(u8, "sk-test") catch unreachable,
        .active_model = allocator.dupe(u8, "claude-3-7-sonnet") catch unreachable,
        .models = modelsList(allocator, &providers.object) catch &.{},
        .selected_models = strListField(allocator, &providers.object, "selected_models"),
    };
    defer {
        allocator.free(auth.id);
        allocator.free(auth.name);
        allocator.free(auth.api);
        allocator.free(auth.base_url);
        allocator.free(auth.api_key);
        allocator.free(auth.active_model);
        for (auth.models) |m| {
            allocator.free(m.id);
            if (m.name) |n| allocator.free(n);
        }
        allocator.free(auth.models);
        if (auth.selected_models) |sel| {
            for (sel) |s| allocator.free(s);
            allocator.free(sel);
        }
    }

    // Free the parse arena + raw buffer BEFORE stringifying (this is where
    // stale aliases would be reused by the allocating writer and panic).
    parsed.deinit();

    // Now stringify into an allocating writer — must not @memcpy-alias.
    var out = std.Io.Writer.Allocating.init(allocator);
    defer out.deinit();
    try std.json.Stringify.value(.{
        .id = auth.id,
        .name = auth.name,
        .base_url = auth.base_url,
        .api_key = auth.api_key,
        .active_model = auth.active_model,
        .models = auth.models,
        .selected_models = auth.selected_models orelse &.{},
    }, .{}, &out.writer);
    const json = out.written();
    try std.testing.expect(std.mem.indexOf(u8, json, "claude-3-7-sonnet-20250219") != null);
}
