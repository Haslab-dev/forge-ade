// services.mcp — multi-source MCP server discovery.
// Port of src/server/discovery/mcp-config.ts: aggregates server maps from
// every major agent tool's config location (native, Claude, Codex, Cursor,
// Windsurf, Gemini, opencode), expands env vars, rebases relative paths,
// and dedupes by name (priority order, first wins).

const std = @import("std");
const svc = @import("../services.zig");

const McpServer = struct {
    name: []const u8,
    command: []const u8 = "",
    args: []const []const u8 = &.{},
    env: ?std.json.Value = null,
    enabled: bool = true,
};

const McpFile = struct {
    mcpServers: []McpServer = &.{},
};

/// The MCPServerStatus shape the frontend's ListMCPServers consumes.
const McpServerStatus = struct {
    name: []const u8,
    command: []const u8 = "",
    args: []const []const u8 = &.{},
    env: ?std.json.Value = null,
    enabled: bool = true,
    source: []const u8 = "",
    connected: bool = false,
    err: ?[]const u8 = null,
};

const OwnedServer = struct {
    name: []const u8,
    command: []const u8 = "",
    args: []const []const u8 = &.{},
    env: ?std.json.Value = null,
    url: []const u8 = "",
    enabled: bool = true,
    source: []const u8 = "",
};

fn mcpPath(env_map: *std.process.Environ.Map, allocator: std.mem.Allocator) []const u8 {
    return std.fmt.allocPrint(allocator, "{s}/mcp.json", .{svc.dataDir(env_map)}) catch "";
}

/// Reads mcp.json into an OWNED McpFile: parses with the raw buffer alive,
/// then deep-copies every string so the result survives the parse teardown.
fn readMcpFile(allocator: std.mem.Allocator, path: []const u8) McpFile {
    const raw = svc.readFileBounded(allocator, path) catch return .{ .mcpServers = &.{} };
    defer allocator.free(raw);
    const parsed = std.json.parseFromSlice(McpFile, allocator, raw, .{ .ignore_unknown_fields = true }) catch return .{ .mcpServers = &.{} };
    defer parsed.deinit();
    var list = std.ArrayList(McpServer).empty;
    for (parsed.value.mcpServers) |s| {
        list.append(allocator, .{
            .name = allocator.dupe(u8, s.name) catch continue,
            .command = allocator.dupe(u8, s.command) catch "",
            .args = s.args,
            .env = s.env,
            .enabled = s.enabled,
        }) catch {};
    }
    return .{ .mcpServers = list.toOwnedSlice(allocator) catch &.{} };
}

// ============================================================================
// Discovery helpers (ported from mcp-config.ts)
// ============================================================================

const c = struct {
    extern "c" fn access(path: [*:0]const u8, mode: c_int) c_int;
};
const F_OK: c_int = 0;

fn fileExists(allocator: std.mem.Allocator, path: []const u8) bool {
    const z = allocator.dupeZ(u8, path) catch return false;
    defer allocator.free(z);
    return c.access(z, F_OK) == 0;
}

/// Owned parse result: the Value aliases arena memory; callers must call
/// `arena.deinit()` when done.
const JsonDoc = struct {
    value: std.json.Value,
    arena: std.heap.ArenaAllocator,
};

/// Reads a file as JSON (or JSONC — strips comments + trailing commas).
fn readJsoncFile(allocator: std.mem.Allocator, path: []const u8) ?JsonDoc {
    const raw = svc.readFileBounded(allocator, path) catch return null;
    defer allocator.free(raw);
    // Strip // and /* */ comments + trailing commas (JSONC).
    var clean = std.ArrayList(u8).empty;
    defer clean.deinit(allocator);
    var in_str = false;
    var esc = false;
    var i: usize = 0;
    while (i < raw.len) : (i += 1) {
        const ch = raw[i];
        if (in_str) {
            clean.append(allocator, ch) catch {};
            if (esc) esc = false else if (ch == '\\') esc = true else if (ch == '"') in_str = false;
            continue;
        }
        switch (ch) {
            '"' => {
                in_str = true;
                clean.append(allocator, ch) catch {};
            },
            '/' => {
                if (i + 1 < raw.len and raw[i + 1] == '/') {
                    while (i < raw.len and raw[i] != '\n') i += 1;
                    clean.append(allocator, '\n') catch {};
                } else if (i + 1 < raw.len and raw[i + 1] == '*') {
                    i += 2;
                    while (i + 1 < raw.len and !(raw[i] == '*' and raw[i + 1] == '/')) i += 1;
                    i += 1;
                } else {
                    clean.append(allocator, ch) catch {};
                }
            },
            else => clean.append(allocator, ch) catch {},
        }
    }
    // Strip trailing commas before } and ].
    const cleaned = clean.items;
    var out = std.ArrayList(u8).empty;
    defer out.deinit(allocator);
    var prev_was_comma = false;
    for (cleaned, 0..) |ch, idx| {
        if ((ch == '}' or ch == ']') and prev_was_comma) {
            // Drop the comma by trimming the last appended char.
            out.items.len -= 1;
        }
        out.append(allocator, ch) catch {};
        prev_was_comma = (ch == ',');
        _ = idx;
    }
    var arena = std.heap.ArenaAllocator.init(allocator);
    const parsed = std.json.parseFromSlice(std.json.Value, arena.allocator(), out.items, .{ .ignore_unknown_fields = true }) catch {
        arena.deinit();
        return null;
    };
    return .{ .value = parsed.value, .arena = arena };
}

fn envLookup(env_map: *std.process.Environ.Map, name: []const u8) []const u8 {
    return env_map.get(name) orelse "";
}

/// Expands ${VAR} and $VAR in a string against the env.
fn expandEnv(allocator: std.mem.Allocator, env_map: *std.process.Environ.Map, value: []const u8) []const u8 {
    if (std.mem.indexOfScalar(u8, value, '$') == null) return value;
    var out = std.ArrayList(u8).empty;
    var i: usize = 0;
    while (i < value.len) {
        if (value[i] == '$') {
            var name: []const u8 = "";
            if (i + 1 < value.len and value[i + 1] == '{') {
                const end = std.mem.indexOfScalarPos(u8, value, i + 2, '}') orelse break;
                name = value[i + 2 .. end];
                i = end + 1;
            } else if (i + 1 < value.len and (std.ascii.isAlphabetic(value[i + 1]) or value[i + 1] == '_')) {
                var j = i + 1;
                while (j < value.len and (std.ascii.isAlphanumeric(value[j]) or value[j] == '_')) j += 1;
                name = value[i + 1 .. j];
                i = j;
            } else {
                out.append(allocator, value[i]) catch {};
                i += 1;
                continue;
            }
            const resolved = envLookup(env_map, name);
            out.appendSlice(allocator, resolved) catch {};
        } else {
            out.append(allocator, value[i]) catch {};
            i += 1;
        }
    }
    return out.toOwnedSlice(allocator) catch value;
}

fn getStr(obj: *const std.json.ObjectMap, key: []const u8) ?[]const u8 {
    if (obj.get(key)) |v| {
        switch (v) {
            .string => |s| return s,
            else => {},
        }
    }
    return null;
}

fn getStrArray(obj: *const std.json.ObjectMap, key: []const u8, allocator: std.mem.Allocator) []const []const u8 {
    var out = std.ArrayList([]const u8).empty;
    if (obj.get(key)) |v| {
        switch (v) {
            .array => |arr| {
                for (arr.items) |item| {
                    switch (item) {
                        .string => |s| out.append(allocator, s) catch {},
                        else => {},
                    }
                }
            },
            else => {},
        }
    }
    return out.toOwnedSlice(allocator) catch &.{};
}

fn getEnv(obj: *const std.json.ObjectMap, allocator: std.mem.Allocator, env_map: *std.process.Environ.Map) ?std.json.Value {
    // env or environment (string→string map), env-expanded.
    const raw = obj.get("env") orelse obj.get("environment") orelse return null;
    switch (raw) {
        .object => |env_obj| {
            var out_obj = std.json.ObjectMap.empty;
            var it = env_obj.iterator();
            while (it.next()) |entry| {
                switch (entry.value_ptr.*) {
                    .string => |s| {
                        const expanded = expandEnv(allocator, env_map, s);
                        out_obj.put(allocator, entry.key_ptr.*, .{ .string = expanded }) catch {};
                    },
                    else => {},
                }
            }
            return .{ .object = out_obj };
        },
        else => return null,
    }
}

/// Normalizes one raw server entry. Returns an owned server (caller frees
/// the top-level struct via the list).
fn normalizeServer(
    allocator: std.mem.Allocator,
    env_map: *std.process.Environ.Map,
    name: []const u8,
    raw: *const std.json.Value,
    source: []const u8,
    config_dir: []const u8,
    srv_list: *std.ArrayList(OwnedServer),
) void {
    _ = config_dir;
    switch (raw.*) {
        .object => |obj| {
            var enabled = true;
            if (obj.get("enabled")) |v| {
                switch (v) {
                    .bool => |b| enabled = b,
                    else => {},
                }
            } else if (obj.get("disabled")) |v| {
                switch (v) {
                    .bool => |b| enabled = !b,
                    else => {},
                }
            }
            const url = getStr(&obj, "url") orelse "";
            if (url.len > 0) {
                srv_list.append(allocator, .{
                    .name = allocator.dupe(u8, name) catch return,
                    .url = allocator.dupe(u8, url) catch "",
                    .enabled = enabled,
                    .source = allocator.dupe(u8, source) catch "",
                }) catch {};
                return;
            }
            const command = getStr(&obj, "command") orelse "";
            if (command.len == 0) return; // no command or url — skip
            const args = getStrArray(&obj, "args", allocator);
            const env = getEnv(&obj, allocator, env_map);
            srv_list.append(allocator, .{
                .name = allocator.dupe(u8, name) catch return,
                .command = allocator.dupe(u8, command) catch "",
                .args = args,
                .env = env,
                .enabled = enabled,
                .source = allocator.dupe(u8, source) catch "",
            }) catch {};
        },
        else => {},
    }
}

/// Collects from a JSON file with an mcpServers (or flat) map.
fn collectFromJson(
    allocator: std.mem.Allocator,
    env_map: *std.process.Environ.Map,
    path: []const u8,
    source: []const u8,
    by_name: *std.StringHashMap(OwnedServer),
) void {
    const doc = readJsoncFile(allocator, path) orelse return;
    defer doc.arena.deinit();
    switch (doc.value) {
        .object => |root| {
            var map: ?std.json.Value = root.get("mcpServers");
            if (map == null) map = doc.value; // flat shape
            switch (map.?) {
                .object => |m| {
                    var srv_list = std.ArrayList(OwnedServer).empty;
                    defer srv_list.deinit(allocator);
                    const dir = std.fs.path.dirname(path) orelse ".";
                    var it = m.iterator();
                    while (it.next()) |entry| {
                        normalizeServer(allocator, env_map, entry.key_ptr.*, entry.value_ptr, source, dir, &srv_list);
                    }
                    // Ownership of each item's strings transfers to by_name.
                    for (srv_list.items) |s| {
                        if (!by_name.contains(s.name)) {
                            by_name.put(s.name, s) catch {};
                        }
                    }
                },
                else => {},
            }
        },
        else => {},
    }
}

/// Codex TOML subset: [mcp_servers.<name>] with command/args/env.
fn collectFromCodexToml(
    allocator: std.mem.Allocator,
    env_map: *std.process.Environ.Map,
    path: []const u8,
    source: []const u8,
    by_name: *std.StringHashMap(OwnedServer),
) void {
    _ = env_map;
    const raw = svc.readFileBounded(allocator, path) catch return;
    defer allocator.free(raw);
    var current: ?[]const u8 = null;
    var srv_list = std.ArrayList(OwnedServer).empty;
    defer srv_list.deinit(allocator);
    var lines = std.mem.splitScalar(u8, raw, '\n');
    while (lines.next()) |line_raw| {
        const line = std.mem.trim(u8, line_raw, " \r\t");
        if (line.len == 0 or line[0] == '#') continue;
        if (std.mem.startsWith(u8, line, "[mcp_servers.")) {
            const rest = line["[mcp_servers.".len..];
            const end = std.mem.indexOfScalar(u8, rest, ']') orelse continue;
            const name = std.mem.trim(u8, rest[0..end], " \t\"");
            current = name;
            continue;
        }
        if (std.mem.startsWith(u8, line, "[")) {
            current = null;
            continue;
        }
        const name = current orelse continue;
        const eq = std.mem.indexOfScalar(u8, line, '=') orelse continue;
        const key = std.mem.trim(u8, line[0..eq], " \t");
        const value = std.mem.trim(u8, line[eq + 1 ..], " \t");
        // Find or create the server record.
        var found = false;
        for (srv_list.items) |*s| {
            if (std.mem.eql(u8, s.name, name)) {
                if (std.mem.eql(u8, key, "command")) {
                    s.command = allocator.dupe(u8, value) catch "";
                } else if (std.mem.eql(u8, key, "args")) {
                    // [ "a", "b" ] — split on commas, strip quotes.
                    var args = std.ArrayList([]const u8).empty;
                    var inner = value;
                    if (inner.len >= 2 and inner[0] == '[') inner = inner[1 .. inner.len - 1];
                    var parts = std.mem.splitScalar(u8, inner, ',');
                    while (parts.next()) |p| {
                        const t = std.mem.trim(u8, p, " \t\"'");
                        if (t.len > 0) args.append(allocator, allocator.dupe(u8, t) catch continue) catch {};
                    }
                    s.args = args.toOwnedSlice(allocator) catch &.{};
                }
                found = true;
                break;
            }
        }
        if (!found) {
            var args: []const []const u8 = &.{};
            var cmd: []const u8 = "";
            if (std.mem.eql(u8, key, "command")) {
                cmd = allocator.dupe(u8, value) catch "";
            } else if (std.mem.eql(u8, key, "args")) {
                var inner = value;
                if (inner.len >= 2 and inner[0] == '[') inner = inner[1 .. inner.len - 1];
                var parts = std.mem.splitScalar(u8, inner, ',');
                var al = std.ArrayList([]const u8).empty;
                while (parts.next()) |p| {
                    const t = std.mem.trim(u8, p, " \t\"'");
                    if (t.len > 0) al.append(allocator, allocator.dupe(u8, t) catch continue) catch {};
                }
                args = al.toOwnedSlice(allocator) catch &.{};
            }
            srv_list.append(allocator, .{
                .name = allocator.dupe(u8, name) catch continue,
                .command = cmd,
                .args = args,
                .source = allocator.dupe(u8, source) catch "",
                .enabled = true,
            }) catch {};
        }
    }
    for (srv_list.items) |s| {
        if (!by_name.contains(s.name)) by_name.put(s.name, s) catch {};
    }
}

/// opencode format: "mcp": { name: { type, command: [...], environment: {} } }
fn collectFromOpencode(
    allocator: std.mem.Allocator,
    env_map: *std.process.Environ.Map,
    path: []const u8,
    source: []const u8,
    by_name: *std.StringHashMap(OwnedServer),
) void {
    const doc = readJsoncFile(allocator, path) orelse return;
    defer doc.arena.deinit();
    switch (doc.value) {
        .object => |root| {
            const mcp = root.get("mcp") orelse return;
            switch (mcp) {
                .object => |m| {
                    var srv_list = std.ArrayList(OwnedServer).empty;
                    defer srv_list.deinit(allocator);
                    var it = m.iterator();
                    while (it.next()) |entry| {
                        switch (entry.value_ptr.*) {
                            .object => |rec| {
                                var enabled = true;
                                if (rec.get("enabled")) |v| {
                                    switch (v) {
                                        .bool => |b| enabled = b,
                                        else => {},
                                    }
                                }
                                const url = getStr(&rec, "url") orelse "";
                                if (url.len > 0) {
                                    srv_list.append(allocator, .{
                                        .name = allocator.dupe(u8, entry.key_ptr.*) catch continue,
                                        .url = allocator.dupe(u8, url) catch "",
                                        .enabled = enabled,
                                        .source = allocator.dupe(u8, source) catch "",
                                    }) catch {};
                                    continue;
                                }
                                // command is an ARRAY: [cmd, arg1, arg2]
                                if (rec.get("command")) |cmdv| {
                                    switch (cmdv) {
                                        .array => |arr| {
                                            if (arr.items.len == 0) continue;
                                            var parts = std.ArrayList([]const u8).empty;
                                            for (arr.items) |item| {
                                                switch (item) {
                                                    .string => |s| parts.append(allocator, allocator.dupe(u8, s) catch continue) catch {},
                                                    else => {},
                                                }
                                            }
                                            if (parts.items.len == 0) continue;
                                            const env = getEnv(&rec, allocator, env_map);
                                            srv_list.append(allocator, .{
                                                .name = allocator.dupe(u8, entry.key_ptr.*) catch continue,
                                                .command = allocator.dupe(u8, parts.items[0]) catch "",
                                                .args = parts.items,
                                                .env = env,
                                                .enabled = enabled,
                                                .source = allocator.dupe(u8, source) catch "",
                                            }) catch {};
                                        },
                                        else => {},
                                    }
                                }
                            },
                            else => {},
                        }
                    }
                    for (srv_list.items) |s| {
                        if (!by_name.contains(s.name)) by_name.put(s.name, s) catch {};
                    }
                },
                else => {},
            }
        },
        else => {},
    }
}

/// Every known MCP config location, priority order (first wins per name).
fn discoverInto(ctx: *svc.Call, allocator: std.mem.Allocator, by_name: *std.StringHashMap(OwnedServer)) void {
    const env_map = ctx.app.env_map;
    const home = svc.homeDir(env_map);
    const cwd = env_map.get("PWD") orelse env_map.get("CWD") orelse ".";

    // (kind, source, path)
    const json_paths = [_]struct { src: []const u8, path: []const u8 }{
        .{ .src = "native:project", .path = std.fmt.allocPrint(allocator, "{s}/mcp.json", .{cwd}) catch return },
        .{ .src = "native:project", .path = std.fmt.allocPrint(allocator, "{s}/.mcp.json", .{cwd}) catch return },
        .{ .src = "native:user", .path = mcpPath(env_map, allocator) },
        .{ .src = "native:user-omp", .path = std.fmt.allocPrint(allocator, "{s}/.omp/agent/mcp.json", .{home}) catch return },
        .{ .src = "claude:user", .path = std.fmt.allocPrint(allocator, "{s}/.claude.json", .{home}) catch return },
        .{ .src = "claude:user-dir", .path = std.fmt.allocPrint(allocator, "{s}/.claude/mcp.json", .{home}) catch return },
        .{ .src = "claude:project", .path = std.fmt.allocPrint(allocator, "{s}/.claude/mcp.json", .{cwd}) catch return },
        .{ .src = "claude:project-alt", .path = std.fmt.allocPrint(allocator, "{s}/.claude/.mcp.json", .{cwd}) catch return },
        .{ .src = "cursor:user", .path = std.fmt.allocPrint(allocator, "{s}/.cursor/mcp.json", .{home}) catch return },
        .{ .src = "cursor:project", .path = std.fmt.allocPrint(allocator, "{s}/.cursor/mcp.json", .{cwd}) catch return },
        .{ .src = "windsurf:user", .path = std.fmt.allocPrint(allocator, "{s}/.codeium/windsurf/mcp_config.json", .{home}) catch return },
        .{ .src = "gemini:user", .path = std.fmt.allocPrint(allocator, "{s}/.gemini/settings.json", .{home}) catch return },
        .{ .src = "gemini:project", .path = std.fmt.allocPrint(allocator, "{s}/.gemini/settings.json", .{cwd}) catch return },
        .{ .src = "opencode:user", .path = std.fmt.allocPrint(allocator, "{s}/.config/opencode/opencode.jsonc", .{home}) catch return },
        .{ .src = "opencode:user-json", .path = std.fmt.allocPrint(allocator, "{s}/.config/opencode/opencode.json", .{home}) catch return },
        .{ .src = "opencode:project", .path = std.fmt.allocPrint(allocator, "{s}/.opencode/opencode.json", .{cwd}) catch return },
    };
    for (json_paths) |jp| {
        if (fileExists(allocator, jp.path)) {
            const src_kind: enum { json, opencode } = if (std.mem.indexOf(u8, jp.src, "opencode:") != null) .opencode else .json;
            switch (src_kind) {
                .json => collectFromJson(allocator, env_map, jp.path, jp.src, by_name),
                .opencode => collectFromOpencode(allocator, env_map, jp.path, jp.src, by_name),
            }
        }
        allocator.free(jp.path);
    }
    // Codex TOML
    const codex_paths = [_][]const u8{
        std.fmt.allocPrint(allocator, "{s}/.codex/config.toml", .{home}) catch return,
        std.fmt.allocPrint(allocator, "{s}/.codex/config.toml", .{cwd}) catch return,
    };
    for (codex_paths) |p| {
        if (fileExists(allocator, p)) collectFromCodexToml(allocator, env_map, p, "codex", by_name);
        allocator.free(p);
    }
}

pub fn servers(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    // Discovery allocates its owned strings from a dedicated arena so they
    // never overlap the response writer's buffer (avoids memcpy-alias panics).
    var arena = std.heap.ArenaAllocator.init(allocator);
    defer arena.deinit();
    const dalloc = arena.allocator();
    var by_name = std.StringHashMap(OwnedServer).init(dalloc);
    discoverInto(ctx, dalloc, &by_name);

    var list = std.ArrayList(McpServerStatus).empty;
    defer list.deinit(allocator);
    var it = by_name.iterator();
    while (it.next()) |entry| {
        const s = entry.value_ptr.*;
        list.append(allocator, .{
            .name = s.name,
            .command = s.command,
            .args = s.args,
            .env = s.env,
            .enabled = s.enabled,
            .source = s.source,
        }) catch {};
    }

    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(list.items, .{}, &out.writer);
    return out.toOwnedSlice();
}

pub fn saveServer(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const path = mcpPath(ctx.app.env_map, allocator);
    defer allocator.free(path);
    const parsed = std.json.parseFromSlice(McpServer, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid mcp server payload");
        return "";
    };
    defer parsed.deinit();

    var file: McpFile = readMcpFile(allocator, path);
    var list = std.ArrayList(McpServer).empty;
    defer list.deinit(allocator);
    var replaced = false;
    for (file.mcpServers) |s| {
        if (std.mem.eql(u8, s.name, parsed.value.name)) {
            list.append(allocator, parsed.value) catch {};
            replaced = true;
        } else {
            list.append(allocator, s) catch {};
        }
    }
    if (!replaced) list.append(allocator, parsed.value) catch {};
    file.mcpServers = list.items;
    try svc.writeJsonFile(allocator, path, file);
    try svc.okCtx(ctx);
    return "";
}

pub fn deleteServer(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const path = mcpPath(ctx.app.env_map, allocator);
    defer allocator.free(path);
    const Payload = struct { name: []const u8 = "" };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid payload");
        return "";
    };
    defer parsed.deinit();

    var file: McpFile = readMcpFile(allocator, path);
    var list = std.ArrayList(McpServer).empty;
    defer list.deinit(allocator);
    for (file.mcpServers) |s| {
        if (!std.mem.eql(u8, s.name, parsed.value.name)) list.append(allocator, s) catch {};
    }
    file.mcpServers = list.items;
    try svc.writeJsonFile(allocator, path, file);
    try svc.okCtx(ctx);
    return "";
}

pub fn tools(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;

    // Discover servers, connect the enabled ones, and return their tools.
    var arena = std.heap.ArenaAllocator.init(allocator);
    defer arena.deinit();
    const dalloc = arena.allocator();
    var by_name = std.StringHashMap(OwnedServer).init(dalloc);
    discoverInto(ctx, dalloc, &by_name);

    var tools_list = std.ArrayList(McpToolStatus).empty;
    defer tools_list.deinit(allocator);
    // Keep every defs slice alive until after stringify (tools_list borrows
    // the ToolDefs' description/schema pointers), then free them all.
    var defs_slices = std.ArrayList([]mcp_client.ToolDef).empty;
    defer defs_slices.deinit(allocator);
    var it = by_name.iterator();
    while (it.next()) |entry| {
        const s = entry.value_ptr.*;
        if (!s.enabled) continue;
        // Ensure a live connection for this server.
        const conn = ensureConnected(ctx, s) catch {
            // Connection failed — skip its tools (settings shows the error).
            continue;
        };
        const defs = mcp_client.listTools(conn, allocator) catch continue;
        defs_slices.append(allocator, defs) catch {
            freeToolDefs(allocator, defs);
            continue;
        };
        for (defs) |d| {
            tools_list.append(allocator, .{
                .name = std.fmt.allocPrint(allocator, "mcp_{s}_{s}", .{ s.name, d.name }) catch continue,
                .description = d.description,
                .server = s.name,
                .parameters = d.input_schema,
            }) catch {};
        }
    }

    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(tools_list.items, .{}, &out.writer);
    for (defs_slices.items) |defs| freeToolDefs(allocator, defs);
    return out.toOwnedSlice();
}

fn freeToolDefs(allocator: std.mem.Allocator, defs: []mcp_client.ToolDef) void {
    for (defs) |d| {
        allocator.free(d.name);
        if (d.description.len > 0) allocator.free(d.description);
        if (d.input_schema) |s| svc.deepFreeValue(allocator, s);
    }
    if (defs.len > 0) allocator.free(defs);
}

/// Reconnects all servers: closes live connections, re-discovers, and
/// reconnects enabled servers. Returns {connected: [...], failed: [...]}.
pub fn reconnect(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    var arena = std.heap.ArenaAllocator.init(allocator);
    defer arena.deinit();
    const dalloc = arena.allocator();

    closeAllConnections();

    var by_name = std.StringHashMap(OwnedServer).init(dalloc);
    discoverInto(ctx, dalloc, &by_name);

    var connected = std.ArrayList([]const u8).empty;
    defer connected.deinit(allocator);
    var failed = std.ArrayList([]const u8).empty;
    defer failed.deinit(allocator);
    var it = by_name.iterator();
    while (it.next()) |entry| {
        const s = entry.value_ptr.*;
        if (!s.enabled) continue;
        if (ensureConnected(ctx, s)) |_| {
            connected.append(allocator, s.name) catch {};
        } else |_| {
            failed.append(allocator, s.name) catch {};
        }
    }

    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(.{
        .connected = connected.items,
        .failed = failed.items,
    }, .{}, &out.writer);
    return out.toOwnedSlice();
}

/// Calls an MCP tool by qualified name (mcp_<server>_<tool>). Used by the
/// agent's tool executor. Returns the text content.
pub fn callQualifiedTool(ctx: *svc.Call, qualified: []const u8, args_json: []const u8) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const parsed = mcp_client.splitQualified(qualified) orelse return error.InvalidMcpToolName;
    const server_name = parsed.server;
    const tool_name = parsed.tool;

    var arena = std.heap.ArenaAllocator.init(allocator);
    defer arena.deinit();
    const dalloc = arena.allocator();
    var by_name = std.StringHashMap(OwnedServer).init(dalloc);
    discoverInto(ctx, dalloc, &by_name);

    const server = by_name.get(server_name) orelse return error.McpServerNotFound;
    const conn = ensureConnected(ctx, server) catch return error.McpNotConnected;

    // Parse args (object) for the call.
    var args: std.json.Value = .{ .object = .{} };
    if (args_json.len > 0) {
        const parsed_args = std.json.parseFromSlice(std.json.Value, allocator, args_json, .{}) catch return error.InvalidArgs;
        defer parsed_args.deinit();
        if (parsed_args.value == .object) args = parsed_args.value;
    }
    const res = mcp_client.callTool(conn, allocator, tool_name, args) catch return error.McpCallFailed;
    defer allocator.free(res.content);
    if (res.is_error) return error.McpToolError;
    return allocator.dupe(u8, res.content) catch error.OutOfMemory;
}

// ============================================================================
// Connection cache
// ============================================================================

const mcp_client = @import("mcp-client.zig");

const McpToolStatus = struct {
    name: []const u8,
    description: []const u8 = "",
    server: []const u8,
    parameters: ?std.json.Value = null,
};

/// Process-wide connection cache, keyed by server name. Guards against
/// concurrent connects. Connections live for the app's lifetime (closed on
/// reconnect/delete).
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
var connections: ?std.StringHashMap(*mcp_client.Connection) = null;

fn ensureConnected(
    ctx: *svc.Call,
    server: OwnedServer,
) !*mcp_client.Connection {
    conn_mutex.lock();
    defer conn_mutex.unlock();
    if (connections == null) {
        connections = std.StringHashMap(*mcp_client.Connection).init(std.heap.c_allocator);
    }
    if (connections.?.get(server.name)) |existing| return existing;
    if (server.command.len == 0) return error.NoCommand;

    const allocator = ctx.app.allocator;
    const conn = mcp_client.spawn(
        allocator,
        server.command,
        server.args,
        ctx.app.env_map,
        ctx.app.env_map.get("PWD") orelse "",
        server.name,
    ) catch |err| {
        std.debug.print("[mcp] connect failed \"{s}\": {s}\n", .{ server.name, @errorName(err) });
        return err;
    };
    // The map outlives the caller's discovery arena, which owns server.name —
    // store an independent copy as the key (freed in closeAllConnections).
    const key = std.heap.c_allocator.dupe(u8, server.name) catch {
        conn.deinit();
        return error.OutOfMemory;
    };
    connections.?.put(key, conn) catch {
        std.heap.c_allocator.free(key);
        conn.deinit();
        return error.OutOfMemory;
    };
    std.debug.print("[mcp] connected \"{s}\"\n", .{server.name});
    return conn;
}

fn closeAllConnections() void {
    conn_mutex.lock();
    defer conn_mutex.unlock();
    if (connections) |*conns| {
        var it = conns.iterator();
        while (it.next()) |e| {
            e.value_ptr.*.deinit();
            std.heap.c_allocator.free(e.key_ptr.*);
        }
        conns.clearRetainingCapacity();
    }
}
