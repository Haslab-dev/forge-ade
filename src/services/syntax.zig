// services.syntax — lightweight syntax check + formatting.
// Port of src/server/syntax.ts + formatter.ts, scoped to JSON (and a basic
// bracket check for other text). The full esbuild/TS syntax engine stays out
// of scope for the bootstrap.

const std = @import("std");
const svc = @import("../services.zig");

const Diagnostic = struct {
    line: u32,
    column: u32,
    message: []const u8,
    severity: []const u8 = "error",
};

fn checkJson(allocator: std.mem.Allocator, content: []const u8, out: *std.ArrayList(Diagnostic)) void {
    var line: u32 = 1;
    var col: u32 = 1;
    var depth: i32 = 0;
    var in_str = false;
    var esc = false;
    for (content) |ch| {
        if (in_str) {
            if (esc) esc = false else if (ch == '\\') esc = true else if (ch == '"') in_str = false;
        } else switch (ch) {
            '"' => in_str = true,
            '{', '[' => depth += 1,
            '}', ']' => depth -= 1,
            else => {},
        }
        if (ch == '\n') {
            line += 1;
            col = 1;
        } else col += 1;
        if (depth < 0) break;
    }
    if (depth != 0) {
        out.append(allocator, .{
            .line = line,
            .column = col,
            .message = "Unbalanced braces/brackets",
            .severity = "error",
        }) catch {};
    }
}

fn formatJson(allocator: std.mem.Allocator, content: []const u8) ![]u8 {
    const parsed = try std.json.parseFromSlice(std.json.Value, allocator, content, .{});
    defer parsed.deinit();
    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(parsed.value, .{ .whitespace = .indent_2 }, &out.writer);
    return out.toOwnedSlice();
}

pub fn check(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct {
        path: []const u8 = "",
        content: []const u8 = "",
    };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid payload");
        return "";
    };
    defer parsed.deinit();

    var diags = std.ArrayList(Diagnostic).empty;
    defer {
        for (diags.items) |d| allocator.free(d.message);
        diags.deinit(allocator);
    }

    const path = parsed.value.path;
    if (std.mem.endsWith(u8, path, ".json")) {
        checkJson(allocator, parsed.value.content, &diags);
    }
    // For everything else, run the same bracket balance check.
    checkJson(allocator, parsed.value.content, &diags);

    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(diags.items, .{}, &out.writer);
    return out.toOwnedSlice();
}

pub fn format(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const Payload = struct {
        path: []const u8 = "",
        content: []const u8 = "",
        tabWidth: ?u8 = null,
        useTabs: ?bool = null,
    };
    const parsed = std.json.parseFromSlice(Payload, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid payload");
        return "";
    };
    defer parsed.deinit();

    const path = parsed.value.path;
    const content = parsed.value.content;
    var out = std.Io.Writer.Allocating.init(allocator);

    if (std.mem.endsWith(u8, path, ".json")) {
        const formatted = formatJson(allocator, content) catch {
            try std.json.Stringify.value(content, .{}, &out.writer);
            return out.toOwnedSlice();
        };
        defer allocator.free(formatted);
        try std.json.Stringify.value(formatted, .{}, &out.writer);
    } else {
        // Non-JSON: pass through unchanged (no esbuild in the bootstrap).
        try std.json.Stringify.value(content, .{}, &out.writer);
    }
    return out.toOwnedSlice();
}
