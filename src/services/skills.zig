// services.skills — multi-source skill discovery.
// Port of src/server/discovery/skills.ts: SKILL.md files (YAML frontmatter)
// discovered from every agent tool's conventional location, deduped by name
// with source priority, and by realpath (symlinked copies load once).

const std = @import("std");
const svc = @import("../services.zig");

const SkillInfo = struct {
    name: []const u8,
    description: []const u8 = "",
    path: []const u8,
    source: []const u8 = "",
    enabled: bool = true,
};

const c = struct {
    extern "c" fn opendir(dirname: [*:0]const u8) ?*anyopaque;
    extern "c" fn closedir(dirp: *anyopaque) c_int;
    extern "c" fn readdir(dirp: *anyopaque) ?*const Dirent;
    extern "c" fn open(path: [*:0]const u8, flags: c_int, ...) c_int;
    extern "c" fn read(fd: c_int, buf: [*]u8, len: usize) isize;
    extern "c" fn close(fd: c_int) c_int;
    extern "c" fn realpath(path: [*:0]const u8, resolved: [*]u8) ?[*:0]u8;
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

fn fileExists(allocator: std.mem.Allocator, path: []const u8) bool {
    const z = allocator.dupeZ(u8, path) catch return false;
    defer allocator.free(z);
    var st: Stat = undefined;
    return c.stat(z, &st) == 0;
}

fn isDir(allocator: std.mem.Allocator, path: []const u8) bool {
    const z = allocator.dupeZ(u8, path) catch return false;
    defer allocator.free(z);
    var st: Stat = undefined;
    if (c.stat(z, &st) != 0) return false;
    return (st.st_mode & 0o170000) == 0o040000;
}

fn readSmallFile(allocator: std.mem.Allocator, path: []const u8) ![]u8 {
    const path_z = try allocator.dupeZ(u8, path);
    defer allocator.free(path_z);
    const fd = c.open(path_z, 0);
    if (fd < 0) return error.OpenFailed;
    defer _ = c.close(fd);
    var content: std.ArrayList(u8) = .empty;
    errdefer content.deinit(allocator);
    var buf: [4096]u8 = undefined;
    while (true) {
        const n = c.read(fd, &buf, buf.len);
        if (n <= 0) break;
        try content.appendSlice(allocator, buf[0..@intCast(n)]);
        if (content.items.len > 256 * 1024) break;
    }
    return content.toOwnedSlice(allocator);
}

/// Minimal YAML frontmatter: `---` block, top-level `key: value` pairs.
fn parseFrontmatter(allocator: std.mem.Allocator, raw: []const u8, key: []const u8) []const u8 {
    if (!std.mem.startsWith(u8, raw, "---")) return "";
    const end = std.mem.indexOfPos(u8, raw, 3, "\n---") orelse return "";
    const fm = raw[3..end];
    var lines = std.mem.splitScalar(u8, fm, '\n');
    while (lines.next()) |line| {
        const idx = std.mem.indexOfScalar(u8, line, ':') orelse continue;
        if (idx <= 0) continue;
        const k = std.mem.trim(u8, line[0..idx], " \t");
        if (!std.mem.eql(u8, k, key)) continue;
        var v = std.mem.trim(u8, line[idx + 1 ..], " \t\r");
        // Strip surrounding quotes.
        if (v.len >= 2 and (v[0] == '"' or v[0] == '\'') and v[v.len - 1] == v[0]) v = v[1 .. v.len - 1];
        return allocator.dupe(u8, v) catch "";
    }
    return "";
}

/// Walks up from cwd collecting ancestor dirs (closest first), stopping at a
/// dir containing .git (repo boundary) or the filesystem root.
fn ancestorsFrom(allocator: std.mem.Allocator, cwd: []const u8, out: *std.ArrayList([]const u8)) void {
    var current = allocator.dupe(u8, cwd) catch return;
    defer allocator.free(current);
    while (true) {
        out.append(allocator, allocator.dupe(u8, current) catch return) catch {};
        if (fileExists(allocator, std.fmt.allocPrint(allocator, "{s}/.git", .{current}) catch "")) break;
        const parent = std.fs.path.dirname(current) orelse break;
        if (std.mem.eql(u8, parent, current)) break;
        current = allocator.dupe(u8, parent) catch break;
    }
}

/// Scans one source dir for skills. Layout A: <dir>/<name>/SKILL.md.
/// Layout B: <dir>/<name>.md. Dedupes by realpath.
fn scanDir(
    allocator: std.mem.Allocator,
    source_dir: []const u8,
    source_tag: []const u8,
    by_name: *std.StringHashMap(SkillInfo),
    seen_real: *std.StringHashMap(void),
) void {
    const dir_z = allocator.dupeZ(u8, source_dir) catch return;
    defer allocator.free(dir_z);
    const handle = c.opendir(dir_z) orelse return;
    defer _ = c.closedir(handle);

    while (c.readdir(handle)) |entry| {
        const name = std.mem.sliceTo(&entry.d_name, 0);
        if (std.mem.eql(u8, name, ".") or std.mem.eql(u8, name, "..")) continue;

        var skill_md: ?[]const u8 = null;
        var skill_dir_name: ?[]const u8 = null;
        if (entry.d_type == 4) {
            // Layout A: subdir/SKILL.md
            const dir_path = std.fs.path.join(allocator, &.{ source_dir, name }) catch continue;
            const md = std.fs.path.join(allocator, &.{ dir_path, "SKILL.md" }) catch continue;
            if (fileExists(allocator, md)) {
                skill_md = allocator.dupe(u8, md) catch continue;
                skill_dir_name = name;
            }
        } else if (entry.d_type == 8) {
            // Layout B: <name>.md
            if (!std.mem.endsWith(u8, name, ".md")) continue;
            const md = std.fs.path.join(allocator, &.{ source_dir, name }) catch continue;
            if (fileExists(allocator, md)) {
                skill_md = allocator.dupe(u8, md) catch continue;
                skill_dir_name = name[0 .. name.len - 3];
            }
        }
        const md = skill_md orelse continue;
        const dir_name = skill_dir_name orelse continue;

        const raw = readSmallFile(allocator, md) catch continue;
        const fm_name = parseFrontmatter(allocator, raw, "name");
        const description = parseFrontmatter(allocator, raw, "description");
        const skill_name = if (fm_name.len > 0) fm_name else dir_name;
        if (skill_name.len == 0) continue;

        // realpath dedup (symlinked copies load once).
        const z = allocator.dupeZ(u8, md) catch continue;
        var resolved_buf: [4096]u8 = undefined;
        if (c.realpath(z, &resolved_buf)) |rp| {
            const real = std.mem.sliceTo(rp, 0);
            if (seen_real.contains(real)) continue;
            seen_real.put(allocator.dupe(u8, real) catch continue, {}) catch {};
        } else {
            if (seen_real.contains(md)) continue;
            seen_real.put(allocator.dupe(u8, md) catch continue, {}) catch {};
        }

        if (!by_name.contains(skill_name)) {
            by_name.put(allocator.dupe(u8, skill_name) catch continue, .{
                .name = allocator.dupe(u8, skill_name) catch continue,
                .description = allocator.dupe(u8, description) catch "",
                .path = allocator.dupe(u8, md) catch continue,
                .source = allocator.dupe(u8, source_tag) catch "",
                .enabled = true,
            }) catch {};
        }
    }
}

/// Discovers skills from project sources (walking ancestors up to repo
/// boundary) + user sources, in priority order. All returned SkillInfo field
/// slices are allocated from `arena` — kept separate from the response
/// writer's allocator so the JSON stringify can never alias its own buffer.
fn discover(ctx: *svc.Call, arena: std.mem.Allocator) []SkillInfo {
    const allocator = arena;
    const env_map = ctx.app.env_map;
    const home = svc.homeDir(env_map);
    const cwd = env_map.get("PWD") orelse env_map.get("CWD") orelse ".";

    var by_name = std.StringHashMap(SkillInfo).init(allocator);
    defer by_name.deinit();
    var seen_real = std.StringHashMap(void).init(allocator);
    defer seen_real.deinit();

    // Project sources: walk ancestors, check each agent dir.
    var ancestors = std.ArrayList([]const u8).empty;
    defer ancestors.deinit(allocator);
    ancestorsFrom(allocator, cwd, &ancestors);
    for (ancestors.items) |dir| {
        const dirs = [_][]const u8{
            std.fmt.allocPrint(allocator, "{s}/.agents/skills", .{dir}) catch continue,
            std.fmt.allocPrint(allocator, "{s}/.agent/skills", .{dir}) catch continue,
            std.fmt.allocPrint(allocator, "{s}/.claude/skills", .{dir}) catch continue,
            std.fmt.allocPrint(allocator, "{s}/.codex/skills", .{dir}) catch continue,
            std.fmt.allocPrint(allocator, "{s}/.gemini/skills", .{dir}) catch continue,
            std.fmt.allocPrint(allocator, "{s}/.opencode/skills", .{dir}) catch continue,
            std.fmt.allocPrint(allocator, "{s}/.github/skills", .{dir}) catch continue,
            std.fmt.allocPrint(allocator, "{s}/.omp/skills", .{dir}) catch continue,
        };
        for (dirs) |d| {
            if (fileExists(allocator, d)) {
                const tag = std.fmt.allocPrint(allocator, "{s}:project", .{std.fs.path.basename(d)}) catch continue;
                scanDir(allocator, d, tag, &by_name, &seen_real);
            }
        }
    }

    // User sources.
    const user_dirs = [_][]const u8{
        std.fmt.allocPrint(allocator, "{s}/.forge-ade/skills", .{home}) catch return &.{},
        std.fmt.allocPrint(allocator, "{s}/.agents/skills", .{home}) catch return &.{},
        std.fmt.allocPrint(allocator, "{s}/.config/skills", .{home}) catch return &.{},
        std.fmt.allocPrint(allocator, "{s}/.skills", .{home}) catch return &.{},
        std.fmt.allocPrint(allocator, "{s}/.claude/skills", .{home}) catch return &.{},
        std.fmt.allocPrint(allocator, "{s}/.codex/skills", .{home}) catch return &.{},
        std.fmt.allocPrint(allocator, "{s}/.gemini/skills", .{home}) catch return &.{},
        std.fmt.allocPrint(allocator, "{s}/.config/opencode/skills", .{home}) catch return &.{},
    };
    for (user_dirs) |d| {
        if (fileExists(allocator, d)) {
            const tag = std.fmt.allocPrint(allocator, "{s}:user", .{std.fs.path.basename(d)}) catch continue;
            scanDir(allocator, d, tag, &by_name, &seen_real);
        }
    }

    // Sort by name.
    var list = std.ArrayList(SkillInfo).empty;
    var it = by_name.iterator();
    while (it.next()) |entry| list.append(allocator, entry.value_ptr.*) catch {};
    std.mem.sort(SkillInfo, list.items, {}, struct {
        fn lessThan(_: void, a: SkillInfo, b: SkillInfo) bool {
            return std.mem.lessThan(u8, a.name, b.name);
        }
    }.lessThan);
    return list.toOwnedSlice(allocator) catch &.{};
}

fn freeSkills(allocator: std.mem.Allocator, skills: []SkillInfo) void {
    allocator.free(skills);
}

pub fn listAll(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    // Arena for the skill field dupes — never overlaps the response writer.
    var arena = std.heap.ArenaAllocator.init(allocator);
    defer arena.deinit();
    const skills = discover(ctx, arena.allocator());
    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(skills, .{}, &out.writer);
    return out.toOwnedSlice();
}

pub fn listEnabled(ctx: *svc.Call) anyerror![]const u8 {
    const allocator = ctx.app.allocator;
    var arena = std.heap.ArenaAllocator.init(allocator);
    defer arena.deinit();
    const skills = discover(ctx, arena.allocator());
    var out = std.Io.Writer.Allocating.init(allocator);
    try std.json.Stringify.value(skills, .{}, &out.writer);
    return out.toOwnedSlice();
}
