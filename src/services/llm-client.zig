// services.llm-client — streaming OpenAI-compatible LLM client.
// Port of src/server/agent/llm-client.ts (streamChat → streamOpenAI).
// Uses `curl -sN` for HTTPS streaming (macOS ships curl; the SDK's webview
// also has network permission), parses SSE `data:` lines incrementally, and
// invokes callbacks for content/reasoning/tool-call deltas as they arrive.

const std = @import("std");
const svc = @import("../services.zig");

pub const ProviderTarget = struct {
    providerId: []const u8 = "",
    baseURL: []const u8 = "",
    apiKey: []const u8 = "",
    model: []const u8 = "",
    contextWindow: ?u32 = null,
    maxTokens: ?u32 = null,
};

pub const LLMMessage = struct {
    role: []const u8, // system | user | assistant | tool
    content: []const u8,
    tool_call_id: ?[]const u8 = null,
    tool_calls: ?[]const u8 = null, // JSON array of {id,type,function:{name,arguments}} as a string
};

pub const ToolDefinition = struct {
    name: []const u8,
    description: []const u8 = "",
    parameters: []const u8 = "{}", // JSON schema
};

pub const ToolCall = struct {
    id: []const u8,
    name: []const u8,
    arguments: []const u8, // JSON string
};

pub const LLMResult = struct {
    content: []const u8 = "",
    reasoning: []const u8 = "",
    toolCalls: []const ToolCall = &.{},
    promptTokens: u64 = 0,
    completionTokens: u64 = 0,
    cachedTokens: u64 = 0,
    stopReason: []const u8 = "stop",
};

/// Callbacks invoked as stream chunks arrive (on the worker thread).
pub const Callbacks = struct {
    onChunk: ?*const fn (ctx: *anyopaque, delta_content: []const u8, delta_reasoning: []const u8) void = null,
    onToolCallDelta: ?*const fn (ctx: *anyopaque, index: usize, id: []const u8, name: []const u8, args: []const u8) void = null,
    ctx: *anyopaque = undefined,
};

const c = struct {
    extern "c" fn pipe(fildes: *[2]c_int) c_int;
    extern "c" fn fork() c_int;
    extern "c" fn dup2(oldfd: c_int, newfd: c_int) c_int;
    extern "c" fn close(fd: c_int) c_int;
    extern "c" fn read(fd: c_int, buf: [*]u8, len: usize) isize;
    extern "c" fn waitpid(pid: c_int, status: ?*c_int, options: c_int) c_int;
    extern "c" fn execvp(file: [*:0]const u8, argv: [*:null]const ?[*:0]const u8) c_int;
    extern "c" fn _exit(code: c_int) noreturn;
    extern "c" fn kill(pid: c_int, sig: c_int) c_int;
    extern "c" fn access(path: [*:0]const u8, mode: c_int) c_int;
};

/// Escapes a JSON string (for embedding in the request body).
fn jsonEscape(allocator: std.mem.Allocator, s: []const u8) ![]const u8 {
    var out = std.ArrayList(u8).empty;
    for (s) |ch| {
        switch (ch) {
            '"' => try out.appendSlice(allocator, "\\\""),
            '\\' => try out.appendSlice(allocator, "\\\\"),
            '\n' => try out.appendSlice(allocator, "\\n"),
            '\r' => try out.appendSlice(allocator, "\\r"),
            '\t' => try out.appendSlice(allocator, "\\t"),
            0...8, 11, 12, 14...0x1f => try out.appendSlice(allocator, "\\uFFFD"),
            else => try out.append(allocator, ch),
        }
    }
    return out.toOwnedSlice(allocator);
}

/// Builds the OpenAI chat/completions request body JSON.
fn buildBody(allocator: std.mem.Allocator, target: *const ProviderTarget, messages: []const LLMMessage, tools: []const ToolDefinition) ![]const u8 {
    var out = std.ArrayList(u8).empty;
    try out.appendSlice(allocator, "{\"model\":");
    const model_j = try jsonEscape(allocator, target.model);
    defer allocator.free(model_j);
    try out.appendSlice(allocator, "\"");
    try out.appendSlice(allocator, model_j);
    try out.appendSlice(allocator, "\",\"stream\":true,\"stream_options\":{\"include_usage\":true},\"messages\":[");

    for (messages, 0..) |m, i| {
        if (i > 0) try out.append(allocator, ',');
        try out.appendSlice(allocator, "{\"role\":\"");
        try out.appendSlice(allocator, m.role);
        try out.appendSlice(allocator, "\",\"content\":");
        if (std.mem.eql(u8, m.role, "tool")) {
            const content_j = try jsonEscape(allocator, m.content);
            defer allocator.free(content_j);
            try out.appendSlice(allocator, "\"");
            try out.appendSlice(allocator, content_j);
            try out.appendSlice(allocator, "\"");
            if (m.tool_call_id) |tid| {
                const tid_j = try jsonEscape(allocator, tid);
                defer allocator.free(tid_j);
                try out.appendSlice(allocator, ",\"tool_call_id\":\"");
                try out.appendSlice(allocator, tid_j);
                try out.appendSlice(allocator, "\"");
            }
        } else {
            const content_j = try jsonEscape(allocator, m.content);
            defer allocator.free(content_j);
            try out.appendSlice(allocator, "\"");
            try out.appendSlice(allocator, content_j);
            try out.appendSlice(allocator, "\"");
        }
        if (m.tool_calls) |tcs| {
            try out.appendSlice(allocator, ",\"tool_calls\":");
            try out.appendSlice(allocator, tcs);
        }
        try out.append(allocator, '}');
    }
    try out.appendSlice(allocator, "]");

    if (tools.len > 0) {
        try out.appendSlice(allocator, ",\"tools\":[");
        for (tools, 0..) |t, i| {
            if (i > 0) try out.append(allocator, ',');
            try out.appendSlice(allocator, "{\"type\":\"function\",\"function\":{\"name\":");
            const name_j = try jsonEscape(allocator, t.name);
            defer allocator.free(name_j);
            try out.appendSlice(allocator, "\"");
            try out.appendSlice(allocator, name_j);
            try out.appendSlice(allocator, "\"");
            if (t.description.len > 0) {
                const desc_j = try jsonEscape(allocator, t.description);
                defer allocator.free(desc_j);
                try out.appendSlice(allocator, ",\"description\":\"");
                try out.appendSlice(allocator, desc_j);
                try out.appendSlice(allocator, "\"");
            }
            try out.appendSlice(allocator, ",\"parameters\":");
            if (t.parameters.len == 0) {
                try out.appendSlice(allocator, "{\"type\":\"object\",\"properties\":{}}");
            } else {
                try out.appendSlice(allocator, t.parameters);
            }
            try out.appendSlice(allocator, "}}");
        }
        try out.appendSlice(allocator, "]");
    }
    try out.appendSlice(allocator, "}");
    return out.toOwnedSlice(allocator);
}

/// Reads a pipe fd until EOF, collecting bytes (bounded).
fn readAllFd(allocator: std.mem.Allocator, fd: c_int) ![]u8 {
    var out = std.ArrayList(u8).empty;
    var buf: [8192]u8 = undefined;
    while (true) {
        const n = c.read(fd, &buf, buf.len);
        if (n <= 0) break;
        try out.appendSlice(allocator, buf[0..@intCast(n)]);
        if (out.items.len > 2 * 1024 * 1024) break;
    }
    return out.toOwnedSlice(allocator);
}

const ToolAccum = struct {
    id: []const u8 = "",
    name: []const u8 = "",
    args: []const u8 = "",
    fn append(self: *ToolAccum, allocator: std.mem.Allocator, id: []const u8, name: []const u8, args: []const u8) void {
        if (id.len > 0 and self.id.len == 0) self.id = allocator.dupe(u8, id) catch self.id;
        if (name.len > 0) {
            var n = std.ArrayList(u8).empty;
            n.appendSlice(allocator, self.name) catch {};
            n.appendSlice(allocator, name) catch {};
            self.name = n.toOwnedSlice(allocator) catch self.name;
        }
        if (args.len > 0) {
            var a = std.ArrayList(u8).empty;
            a.appendSlice(allocator, self.args) catch {};
            a.appendSlice(allocator, args) catch {};
            self.args = a.toOwnedSlice(allocator) catch self.args;
        }
    }
};

/// Streams one chat completion from an OpenAI-compatible endpoint via curl.
/// `emit_event` is called on the worker thread for each delta; the final
/// LLMResult is returned (owned slices from `allocator`).
pub fn streamChat(
    allocator: std.mem.Allocator,
    target: *const ProviderTarget,
    messages: []const LLMMessage,
    tools: []const ToolDefinition,
    cb: Callbacks,
    abort_flag: *std.atomic.Value(bool),
) !LLMResult {
    const body = try buildBody(allocator, target, messages, tools);
    defer allocator.free(body);

    // Build curl args: POST {base}/chat/completions with streaming.
    var base = target.baseURL;
    while (base.len > 0 and base[base.len - 1] == '/') base = base[0 .. base.len - 1];
    const url = try std.fmt.allocPrint(allocator, "{s}/chat/completions", .{base});
    defer allocator.free(url);

    // Write the body to a temp file so curl can --data-binary @file (avoids
    // argv length limits on huge prompts). Unlinked after the request.
    const tmp = try std.fmt.allocPrint(allocator, "/tmp/forge-llm-body-{d}-{d}.json", .{ svc.nowMs(), @mod(svc.nowMs() * 7919, 1_000_000) });
    defer allocator.free(tmp);
    const tmp_z = try allocator.dupeZ(u8, tmp);
    defer allocator.free(tmp_z);
    try svc.writeFileAtomic(allocator, tmp, body);

    var pipe_fds: [2]c_int = undefined;
    if (c.pipe(&pipe_fds) != 0) return error.PipeFailed;
    const pid = c.fork();
    if (pid < 0) {
        _ = c.close(pipe_fds[0]);
        _ = c.close(pipe_fds[1]);
        return error.ForkFailed;
    }
    if (pid == 0) {
        _ = c.close(pipe_fds[0]);
        _ = c.dup2(pipe_fds[1], 1);
        _ = c.dup2(pipe_fds[1], 2);
        _ = c.close(pipe_fds[1]);
        // curl -sN -X POST -H "Content-Type: application/json" -H "Authorization: Bearer KEY" --data-binary @FILE URL
        const curl_z = allocator.dupeZ(u8, "curl") catch c._exit(127);
        const flag_s = allocator.dupeZ(u8, "-sN") catch c._exit(127);
        const flag_x = allocator.dupeZ(u8, "-X") catch c._exit(127);
        const post = allocator.dupeZ(u8, "POST") catch c._exit(127);
        const flag_h1 = allocator.dupeZ(u8, "-H") catch c._exit(127);
        const h1 = allocator.dupeZ(u8, "Content-Type: application/json") catch c._exit(127);
        const flag_h2 = allocator.dupeZ(u8, "-H") catch c._exit(127);
        const auth_tmp = std.fmt.allocPrint(allocator, "Authorization: Bearer {s}", .{target.apiKey}) catch c._exit(127);
        const auth = allocator.dupeZ(u8, auth_tmp) catch c._exit(127);
        const flag_db = allocator.dupeZ(u8, "--data-binary") catch c._exit(127);
        const at_tmp0 = std.fmt.allocPrint(allocator, "@{s}", .{tmp}) catch c._exit(127);
        const at_tmp = allocator.dupeZ(u8, at_tmp0) catch c._exit(127);
        const url_z = allocator.dupeZ(u8, url) catch c._exit(127);
        const args = [_]?[*:0]const u8{ curl_z.ptr, flag_s.ptr, flag_x.ptr, post.ptr, flag_h1.ptr, h1.ptr, flag_h2.ptr, auth.ptr, flag_db.ptr, at_tmp.ptr, url_z.ptr, null };
        _ = c.execvp(curl_z.ptr, @ptrCast(&args));
        c._exit(127);
    }
    _ = c.close(pipe_fds[1]);

    // Read + parse SSE from the pipe.
    var out = std.ArrayList(u8).empty;
    defer out.deinit(allocator);
    var buf: [16384]u8 = undefined;
    var acc_content = std.ArrayList(u8).empty;
    var acc_reasoning = std.ArrayList(u8).empty;
    defer acc_content.deinit(allocator);
    defer acc_reasoning.deinit(allocator);
    var tool_map = std.AutoHashMap(usize, ToolAccum).init(allocator);
    defer tool_map.deinit();
    var tool_order = std.ArrayList(usize).empty;
    defer tool_order.deinit(allocator);
    var prompt_tokens: u64 = 0;
    var completion_tokens: u64 = 0;
    var cached_tokens: u64 = 0;
    var stop_reason: []const u8 = "stop";

    while (!abort_flag.load(.acquire)) {
        const n = c.read(pipe_fds[0], &buf, buf.len);
        if (n <= 0) break;
        std.debug.print("[sse] read {d} bytes: {s}\n", .{ n, buf[0..@intCast(n)] });
        try out.appendSlice(allocator, buf[0..@intCast(n)]);
        // Process complete lines in the buffer.
        var consumed: usize = 0;
        var idx: usize = 0;
        while (std.mem.indexOfScalarPos(u8, out.items, idx, '\n')) |nl| {
            const line = std.mem.trim(u8, out.items[idx..nl], " \r\t");
            idx = nl + 1;
            consumed = idx;
            if (!std.mem.startsWith(u8, line, "data:")) continue;
            const data = std.mem.trim(u8, line["data:".len..], " \t");
            if (data.len == 0 or std.mem.eql(u8, data, "[DONE]")) continue;
            parseSseChunk(allocator, data, &acc_content, &acc_reasoning, &tool_map, &tool_order, &prompt_tokens, &completion_tokens, &cached_tokens, &stop_reason, &cb) catch continue;
        }
        if (consumed > 0) {
            var remaining = std.ArrayList(u8).empty;
            remaining.appendSlice(allocator, out.items[consumed..]) catch {};
            out.deinit(allocator);
            out = remaining;
        }
    }
    _ = c.close(pipe_fds[0]);
    _ = c.waitpid(pid, null, 0);
    // The request body temp file is no longer needed — remove it (the old
    // code left one behind on disk for every call).
    _ = std.c.unlink(tmp_z.ptr);

    // Finalize tool calls.
    var toolCalls = std.ArrayList(ToolCall).empty;
    defer toolCalls.deinit(allocator);
    for (tool_order.items) |i| {
        if (tool_map.get(i)) |t| {
            if (t.name.len == 0) continue;
            toolCalls.append(allocator, .{
                .id = allocator.dupe(u8, t.id) catch "",
                .name = allocator.dupe(u8, t.name) catch "",
                .arguments = allocator.dupe(u8, t.args) catch "",
            }) catch {};
        }
    }

    return .{
        .content = acc_content.toOwnedSlice(allocator) catch "",
        .reasoning = acc_reasoning.toOwnedSlice(allocator) catch "",
        .toolCalls = toolCalls.toOwnedSlice(allocator) catch &.{},
        .promptTokens = prompt_tokens,
        .completionTokens = completion_tokens,
        .cachedTokens = cached_tokens,
        .stopReason = stop_reason,
    };
}

fn parseSseChunk(
    allocator: std.mem.Allocator,
    data: []const u8,
    acc_content: *std.ArrayList(u8),
    acc_reasoning: *std.ArrayList(u8),
    tool_map: *std.AutoHashMap(usize, ToolAccum),
    tool_order: *std.ArrayList(usize),
    prompt_tokens: *u64,
    completion_tokens: *u64,
    cached_tokens: *u64,
    stop_reason: *[]const u8,
    cb: *const Callbacks,
) !void {
    const parsed = std.json.parseFromSlice(std.json.Value, allocator, data, .{}) catch {
        std.debug.print("[sse] parse fail: {s}\n", .{if (data.len > 120) data[0..120] else data});
        return;
    };
    defer parsed.deinit();
    const root = parsed.value;
    std.debug.print("[sse] chunk: {s}\n", .{if (data.len > 100) data[0..100] else data});
    // in-stream error payload
    if (root == .object) {
        if (root.object.get("error")) |errv| {
            if (errv == .object) {
                if (errv.object.get("message")) |m| {
                    if (m == .string) return error.ProviderError;
                }
            }
        }
    }
    const choices = if (root == .object) root.object.get("choices") else null;
    if (choices) |ch| {
        switch (ch) {
            .array => |arr| {
                if (arr.items.len == 0) return;
                const choice = arr.items[0];
                if (choice != .object) return;
                if (choice.object.get("finish_reason")) |fr| {
                    if (fr == .string) {
                        const frs = fr.string;
                        // Deep-copy: frs aliases this chunk's parse arena,
                        // which dies at the end of parseSseChunk.
                        stop_reason.* = allocator.dupe(u8, if (std.mem.eql(u8, frs, "tool_calls")) "tool_use" else frs) catch "stop";
                    }
                }
                const delta = choice.object.get("delta");
                if (delta) |d| {
                    if (d != .object) return;
                    if (d.object.get("content")) |cv| {
                        if (cv == .string and cv.string.len > 0) {
                            try acc_content.appendSlice(allocator, cv.string);
                            if (cb.onChunk) |fnc| fnc(cb.ctx, cv.string, "");
                        }
                    }
                    var reasoning: []const u8 = "";
                    if (d.object.get("reasoning_content")) |rv| {
                        if (rv == .string) reasoning = rv.string;
                    } else if (d.object.get("reasoning")) |rv| {
                        if (rv == .string) reasoning = rv.string;
                    }
                    if (reasoning.len > 0) {
                        try acc_reasoning.appendSlice(allocator, reasoning);
                        if (cb.onChunk) |fnc| fnc(cb.ctx, "", reasoning);
                    }
                    if (d.object.get("tool_calls")) |tcv| {
                        switch (tcv) {
                            .array => |tcs| {
                                for (tcs.items) |tc| {
                                    if (tc != .object) continue;
                                    const index: usize = if (tc.object.get("index")) |iv|
                                        (if (iv == .integer) @intCast(@max(iv.integer, 0)) else 0)
                                    else
                                        0;
                                    var id: []const u8 = "";
                                    var name: []const u8 = "";
                                    var args: []const u8 = "";
                                    if (tc.object.get("id")) |iv| {
                                        if (iv == .string) id = iv.string;
                                    }
                                    if (tc.object.get("function")) |fv| {
                                        if (fv == .object) {
                                            if (fv.object.get("name")) |nv| {
                                                if (nv == .string) name = nv.string;
                                            }
                                            if (fv.object.get("arguments")) |av| {
                                                if (av == .string) args = av.string;
                                            }
                                        }
                                    }
                                    if (!tool_map.contains(index)) {
                                        tool_map.put(index, .{}) catch {};
                                        tool_order.append(allocator, index) catch {};
                                    }
                                    var entry = tool_map.getPtr(index).?;
                                    entry.append(allocator, id, name, args);
                                    if (cb.onToolCallDelta) |fnc| fnc(cb.ctx, index, id, name, args);
                                }
                            },
                            else => {},
                        }
                    }
                }
            },
            else => {},
        }
    }
    if (root == .object) {
        if (root.object.get("usage")) |uv| {
            if (uv == .object) {
                if (uv.object.get("prompt_tokens")) |v| {
                    if (v == .integer) prompt_tokens.* = @intCast(@max(v.integer, 0));
                }
                if (uv.object.get("completion_tokens")) |v| {
                    if (v == .integer) completion_tokens.* = @intCast(@max(v.integer, 0));
                }
                if (uv.object.get("prompt_tokens_details")) |pd| {
                    if (pd == .object) {
                        if (pd.object.get("cached_tokens")) |v| {
                            if (v == .integer) cached_tokens.* = @intCast(@max(v.integer, 0));
                        }
                    }
                }
            }
        }
    }
}
