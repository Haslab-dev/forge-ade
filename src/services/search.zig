// services.search — gitignore-aware filename/content search + replace.
// Port of src/server/search.ts. Uses a recursive dir walk (opendir/readdir
// via the same c-stubs main.zig uses) with skip rules for .git + heavy dirs.

const std = @import("std");
const svc = @import("../services.zig");

const SearchOptions = struct {
    query: []const u8 = "",
    folder: []const u8 = "",
    limit: ?usize = null,
    caseSensitive: bool = false,
    wholeWord: bool = false,
    isRegex: bool = false,
    respectGitignore: bool = true,
    /// Only used by the replace walk.
    replacement: []const u8 = "",
};

const RankedResult = struct {
    path: []const u8,
    name: []const u8,
    isDir: bool,
    score: i32,
    line: ?u32 = null,
    snippet: ?[]const u8 = null,
};

const c = struct {
    extern "c" fn opendir(dirname: [*:0]const u8) ?*anyopaque;
    extern "c" fn closedir(dirp: *anyopaque) c_int;
    extern "c" fn readdir(dirp: *anyopaque) ?*const Dirent;
    extern "c" fn open(path: [*:0]const u8, flags: c_int, ...) c_int;
    extern "c" fn read(fd: c_int, buf: [*]u8, len: usize) isize;
    extern "c" fn close(fd: c_int) c_int;
    extern "c" fn stat(path: [*:0]const u8, buf: *Stat) c_int;
};

const Stat = extern struct {
    st_dev: i32,
    st_mode: u16,
    st_nlink: u16,
    st_ino: u64,
    st_uid: u32,
    st_gid: u32,
    st_rdev: i32,
    st_atimespec: extern struct { tv_sec: isize, tv_nsec: isize },
    st_mtimespec: extern struct { tv_sec: isize, tv_nsec: isize },
    st_ctimespec: extern struct { tv_sec: isize, tv_nsec: isize },
    st_birthtimespec: extern struct { tv_sec: isize, tv_nsec: isize },
    st_size: i64,
    st_blocks: i64,
    st_blksize: i32,
    st_flags: u32,
    st_gen: u32,
    st_lspare: i32,
    st_qspare: [2]i64,
};

const Dirent = extern struct {
    d_ino: u64,
    d_seekoff: u64,
    d_reclen: u16,
    d_namlen: u16,
    d_type: u8,
    d_name: [1024]u8,
};

const SKIP_DIRS = [_][]const u8{
    ".git", "node_modules", "zig-out", ".zig-cache", ".native", "dist", "build",
    "Pods", ".gradle", "DerivedData", ".build", ".swiftpm", "Carthage", ".yarn",
    "vendor", "__pycache__", ".DS_Store", ".idea", ".vscode", ".cache", ".next",
    ".nuxt", ".turbo", "coverage", ".venv", "venv", "target", ".dart_tool",
};

fn isSkippedDir(name: []const u8) bool {
    for (SKIP_DIRS) |skip| {
        if (std.mem.eql(u8, name, skip)) return true;
    }
    return false;
}

fn isSearchableFile(allocator: std.mem.Allocator, path: []const u8) bool {
    const ext = std.fs.path.extension(path);
    const bin_exts = [_][]const u8{
        "png", "jpg", "jpeg", "gif", "webp", "svg", "ico", "avif", "icns",
        "zip", "gz", "tgz", "xz", "zst", "7z", "rar", "tar", "bz2",
        "wasm", "woff", "woff2", "ttf", "otf", "eot", "mp4", "mp3", "mov",
        "pdf", "exe", "dll", "dylib", "so", "bin",
    };
    for (bin_exts) |be| {
        if (std.mem.eql(u8, ext, be)) return false;
    }
    const path_z = allocator.dupeZ(u8, path) catch return true;
    defer allocator.free(path_z);
    var st: Stat = undefined;
    if (c.stat(path_z, &st) != 0) return false;
    return st.st_size <= 1_000_000;
}

/// Reads a file's full bytes (bounded 1MB for search).
fn readFileBytes(allocator: std.mem.Allocator, path: []const u8) ![]u8 {
    const path_z = try allocator.dupeZ(u8, path);
    defer allocator.free(path_z);
    const fd = c.open(path_z, 0);
    if (fd < 0) return error.OpenFailed;
    defer _ = c.close(fd);
    var bytes: std.ArrayList(u8) = .empty;
    errdefer bytes.deinit(allocator);
    var buf: [8192]u8 = undefined;
    while (true) {
        const n = c.read(fd, &buf, buf.len);
        if (n <= 0) break;
        try bytes.appendSlice(allocator, buf[0..@intCast(n)]);
        if (bytes.items.len > 1_000_000) break;
    }
    return bytes.toOwnedSlice(allocator);
}

/// A .gitignore pattern (simple glob). Ports the essential forms: `**/`,
/// trailing `/` (dir-only), `/*` suffix, and bare basenames.
const GitignoreRule = struct {
    pattern: []const u8,
    dir_only: bool = false,
};

fn parseGitignore(allocator: std.mem.Allocator, text: []const u8) []GitignoreRule {
    var rules = std.ArrayList(GitignoreRule).empty;
    var lines = std.mem.splitScalar(u8, text, '\n');
    while (lines.next()) |line_raw| {
        const line = std.mem.trim(u8, line_raw, " \r\t");
        if (line.len == 0 or line[0] == '#') continue;
        if (std.mem.startsWith(u8, line, "!")) continue; // negation unsupported
        var pat = line;
        var dir_only = false;
        if (std.mem.endsWith(u8, pat, "/")) {
            dir_only = true;
            pat = pat[0 .. pat.len - 1];
        }
        // Strip leading **/ (matches anywhere).
        if (std.mem.startsWith(u8, pat, "**/")) pat = pat[3..];
        rules.append(allocator, .{ .pattern = allocator.dupe(u8, pat) catch continue, .dir_only = dir_only }) catch {};
    }
    return rules.toOwnedSlice(allocator) catch &.{};
}

fn matchesRule(rule: GitignoreRule, rel: []const u8, is_dir: bool) bool {
    if (rule.dir_only and !is_dir) return false;
    const pat = rule.pattern;
    // Trailing /* on a pattern like .cache/* → match contents of .cache/.
    if (std.mem.endsWith(u8, pat, "/*")) {
        const prefix = pat[0 .. pat.len - 2];
        return std.mem.startsWith(u8, rel, prefix) and rel.len > prefix.len;
    }
    // Basename-only pattern matches at any depth.
    if (std.mem.indexOfScalar(u8, pat, '/') == null) {
        const base = std.fs.path.basename(rel);
        return std.mem.eql(u8, base, pat) or std.mem.eql(u8, rel, pat);
    }
    // Full relative path match (with glob * on segments).
    return globMatch(pat, rel);
}

fn globMatch(pattern: []const u8, text: []const u8) bool {
    // Minimal * wildcard matcher (no ? / []).
    var pi: usize = 0;
    var ti: usize = 0;
    var star: ?usize = null;
    var star_ti: usize = 0;
    while (ti < text.len) {
        if (pi < pattern.len and (pattern[pi] == text[ti] or pattern[pi] == '*')) {
            if (pattern[pi] == '*') {
                star = pi;
                star_ti = ti;
                pi += 1;
            } else {
                pi += 1;
                ti += 1;
            }
        } else if (star) |s| {
            pi = s + 1;
            star_ti += 1;
            ti = star_ti;
        } else return false;
    }
    while (pi < pattern.len and pattern[pi] == '*') pi += 1;
    return pi == pattern.len;
}

const WalkResult = struct {
    results: std.ArrayList(RankedResult),
    limit: usize,
    done: bool = false,
};

/// The concrete context types passed through walkDir.
pub const WalkContext = struct {
    allocator: std.mem.Allocator,
    opts: SearchOptions = .{},
    results: *std.ArrayList(RankedResult) = undefined,
    limit: usize = 50,
    files_changed: usize = 0,
    total_replacements: usize = 0,
    files: *std.ArrayList([]const u8) = undefined,
};

/// Depth-first walk; `on_file` returns false to stop the whole walk.
fn walkDir(
    allocator: std.mem.Allocator,
    dir: []const u8,
    rules: ?[]GitignoreRule,
    ctx: *WalkContext,
    on_file: *const fn (ctx: *WalkContext, full_path: []const u8, is_dir: bool) bool,
) void {
    const dir_z = allocator.dupeZ(u8, dir) catch return;
    defer allocator.free(dir_z);
    const handle = c.opendir(dir_z) orelse return;
    defer _ = c.closedir(handle);

    while (c.readdir(handle)) |entry_ptr| {
        const name = std.mem.sliceTo(&entry_ptr.d_name, 0);
        if (std.mem.eql(u8, name, ".") or std.mem.eql(u8, name, "..")) continue;
        if (isSkippedDir(name)) continue;

        const is_dir = entry_ptr.d_type == 4;
        const full_path = std.fs.path.join(allocator, &.{ dir, name }) catch continue;
        defer allocator.free(full_path);

        if (rules) |rs| {
            const rel = if (full_path.len > dir.len and std.mem.startsWith(u8, full_path, dir))
                full_path[dir.len + 1 ..]
            else
                full_path;
            var ignored = false;
            for (rs) |rule| {
                if (matchesRule(rule, rel, is_dir)) {
                    ignored = true;
                    break;
                }
            }
            if (ignored) continue;
        }

        if (!on_file(ctx, full_path, is_dir)) return;
        if (is_dir) walkDir(allocator, full_path, rules, ctx, on_file);
    }
}

fn loadGitignores(allocator: std.mem.Allocator, root: []const u8) ?[]GitignoreRule {
    const gi_path = std.fs.path.join(allocator, &.{ root, ".gitignore" }) catch return null;
    defer allocator.free(gi_path);
    const raw = svc.readFileBounded(allocator, gi_path) catch return null;
    defer allocator.free(raw);
    return parseGitignore(allocator, raw);
}

fn parseOpts(ctx: *svc.Call) SearchOptions {
    const parsed = std.json.parseFromSlice(SearchOptions, ctx.app.allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch return .{};
    defer parsed.deinit();
    return parsed.value;
}

fn buildMatcher(opts: SearchOptions, allocator: std.mem.Allocator) ?*const fn ([]const u8, []const u8, SearchOptions) bool {
    _ = allocator;
    _ = opts;
    return null;
}

fn matchesName(name: []const u8, opts: SearchOptions) bool {
    return substringMatch(name, opts.query, opts.caseSensitive);
}

fn substringMatch(haystack: []const u8, needle: []const u8, case_sensitive: bool) bool {
    if (needle.len == 0) return false;
    if (case_sensitive) return std.mem.indexOf(u8, haystack, needle) != null;
    return std.ascii.indexOfIgnoreCase(haystack, needle) != null;
}

fn filenameContext(ctx: *WalkContext, full_path: []const u8, is_dir: bool) bool {
    const allocator = ctx.allocator;
    const opts = ctx.opts;
    const base = std.fs.path.basename(full_path);
    if (!matchesName(base, opts)) return true;
    const exact = !is_dir and std.ascii.eqlIgnoreCase(base, opts.query);
    ctx.results.append(allocator, .{
        .path = allocator.dupe(u8, full_path) catch return true,
        .name = allocator.dupe(u8, base) catch return true,
        .isDir = is_dir,
        .score = if (exact) 100 else if (is_dir) 60 else 50,
    }) catch {};
    return ctx.results.items.len < ctx.limit;
}

pub fn filename(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const opts = parseOpts(ctx);
    var folder = opts.folder;
    defer if (folder.len > 0) allocator.free(folder);
    if (folder.len == 0) folder = svc.homeDir(ctx.app.env_map);

    const rules = if (opts.respectGitignore) loadGitignores(allocator, folder) else null;
    defer if (rules) |rs| allocator.free(rs);

    var results = std.ArrayList(RankedResult).empty;
    defer {
        for (results.items) |r| {
            allocator.free(r.path);
            allocator.free(r.name);
        }
        results.deinit(allocator);
    }

    var fctx = WalkContext{ .allocator = allocator, .opts = opts, .results = &results, .limit = opts.limit orelse 50 };
    walkDir(allocator, folder, rules, &fctx, filenameContext);

    // Sort: score desc, then path asc.
    std.mem.sort(RankedResult, results.items, {}, struct {
        fn lessThan(_: void, a: RankedResult, b: RankedResult) bool {
            if (a.score != b.score) return a.score > b.score;
            return std.mem.lessThan(u8, a.path, b.path);
        }
    }.lessThan);

    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(results.items, .{}, &out.writer);
    return out.toOwnedSlice();
}

const ContentCtx = WalkContext;

fn contentContext(ctx: *WalkContext, full_path: []const u8, is_dir: bool) bool {
    const allocator = ctx.allocator;
    _ = is_dir;
    if (!isSearchableFile(allocator, full_path)) return true;
    const raw = readFileBytes(allocator, full_path) catch return true;
    defer allocator.free(raw);
    var line_start: usize = 0;
    var line_no: u32 = 0;
    while (line_start <= raw.len) {
        const line_end = std.mem.indexOfScalarPos(u8, raw, line_start, '\n') orelse raw.len;
        if (line_end == line_start and line_start == raw.len) break;
        line_no += 1;
        const line = raw[line_start..line_end];
        if (substringMatch(line, ctx.opts.query, ctx.opts.caseSensitive)) {
            ctx.results.append(allocator, .{
                .path = allocator.dupe(u8, full_path) catch return true,
                .name = allocator.dupe(u8, std.fs.path.basename(full_path)) catch return true,
                .isDir = false,
                .score = 1,
                .line = line_no,
                .snippet = allocator.dupe(u8, std.mem.trim(u8, line, " \r\t")) catch return true,
            }) catch {};
            if (ctx.results.items.len >= ctx.limit) return false;
        }
        line_start = line_end + 1;
    }
    return true;
}

pub fn content(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const opts = parseOpts(ctx);
    var folder = opts.folder;
    defer if (folder.len > 0) allocator.free(folder);
    if (folder.len == 0) folder = svc.homeDir(ctx.app.env_map);

    const rules = if (opts.respectGitignore) loadGitignores(allocator, folder) else null;
    defer if (rules) |rs| allocator.free(rs);

    var results = std.ArrayList(RankedResult).empty;
    defer {
        for (results.items) |r| {
            if (r.path.len > 0) allocator.free(r.path);
            if (r.name.len > 0) allocator.free(r.name);
            if (r.snippet) |s| allocator.free(s);
        }
        results.deinit(allocator);
    }

    var fctx = ContentCtx{ .allocator = allocator, .opts = opts, .results = &results, .limit = opts.limit orelse 100 };
    walkDir(allocator, folder, rules, &fctx, contentContext);

    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(results.items, .{}, &out.writer);
    return out.toOwnedSlice();
}

const ReplaceOptions = struct {
    query: []const u8 = "",
    replacement: []const u8 = "",
    folder: []const u8 = "",
    caseSensitive: bool = false,
    respectGitignore: bool = true,
};

const ReplaceCtx = WalkContext;

fn replaceContext(ctx: *WalkContext, full_path: []const u8, is_dir: bool) bool {
    const allocator = ctx.allocator;
    _ = is_dir;
    if (!isSearchableFile(allocator, full_path)) return true;
    const raw = readFileBytes(allocator, full_path) catch return true;
    defer allocator.free(raw);

    // Simple literal replace (case-insensitive when requested) — the regex
    // engine is intentionally out of scope for the bootstrap.
    const needle = ctx.opts.query;
    const replacement = ctx.opts.replacement;
    if (needle.len == 0) return true;

    var out = std.ArrayList(u8).empty;
    defer out.deinit(allocator);
    var count: usize = 0;
    var pos: usize = 0;
    while (pos < raw.len) {
        const idx = if (ctx.opts.caseSensitive)
            std.mem.indexOfPos(u8, raw, pos, needle)
        else
            std.ascii.indexOfIgnoreCasePos(raw, pos, needle);
        if (idx) |i| {
            out.appendSlice(allocator, raw[pos..i]) catch return true;
            out.appendSlice(allocator, replacement) catch return true;
            pos = i + needle.len;
            count += 1;
        } else {
            out.appendSlice(allocator, raw[pos..]) catch return true;
            break;
        }
    }
    if (count == 0) return true;
    if (!std.mem.eql(u8, out.items, raw)) {
        svc.writeFileAtomic(allocator, full_path, out.items) catch return true;
        ctx.files_changed += 1;
        ctx.total_replacements += count;
        ctx.files.append(allocator, allocator.dupe(u8, full_path) catch return true) catch {};
    }
    return true;
}

pub fn replace(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    const opts = std.json.parseFromSlice(ReplaceOptions, allocator, ctx.payload, .{ .ignore_unknown_fields = true }) catch {
        try svc.failCtx(ctx, "invalid replace payload");
        return "";
    };
    defer opts.deinit();
    var folder = opts.value.folder;
    defer if (folder.len > 0) allocator.free(folder);
    if (folder.len == 0) folder = svc.homeDir(ctx.app.env_map);

    const rules = if (opts.value.respectGitignore) loadGitignores(allocator, folder) else null;
    defer if (rules) |rs| allocator.free(rs);

    var files = std.ArrayList([]const u8).empty;
    defer {
        for (files.items) |f| allocator.free(f);
        files.deinit(allocator);
    }
    var fctx = WalkContext{ .allocator = allocator, .files = &files };
    fctx.opts = .{ .query = opts.value.query, .replacement = opts.value.replacement, .caseSensitive = opts.value.caseSensitive, .respectGitignore = opts.value.respectGitignore };
    fctx.limit = std.math.maxInt(usize);
    walkDir(allocator, folder, rules, &fctx, replaceContext);

    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(.{
        .filesChanged = fctx.files_changed,
        .totalReplacements = fctx.total_replacements,
        .files = fctx.files.items,
    }, .{}, &out.writer);
    return out.toOwnedSlice();
}
