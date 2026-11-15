package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type FileStatus struct {
	Path      string `json:"path"`
	Dir       string `json:"dir"`       // parent directory path, e.g. "src/lib"
	Staging   string `json:"staging"`   // "staged", "unstaged", "untracked"
	Status    string `json:"status"`    // "M", "A", "D", "R", "?"
	Additions int    `json:"additions"` // lines added vs HEAD (0 for binary)
	Deletions int    `json:"deletions"` // lines deleted vs HEAD (0 for binary)
}

type GitStatusResult struct {
	Branch    string       `json:"branch"`
	Staged    []FileStatus `json:"staged"`
	Unstaged  []FileStatus `json:"unstaged"`
	Untracked []FileStatus `json:"untracked"`
	Conflicts []FileStatus `json:"conflicts"`
}

// porcelainEntry is one parsed `git status --porcelain=v2` record.
type porcelainEntry struct {
	Kind     string // "1" ordinary, "2" renamed/copied, "u" unmerged, "?" untracked
	XY       string
	Path     string
	OrigPath string // rename source, present only for "2" entries
}

// parsePorcelainV2Line parses one record of `git status --porcelain=v2`
// output. Paths in v2 are emitted verbatim (never quoted for spaces) and may
// contain spaces, so the path is NOT the next strings.Fields token — it is
// everything after the fixed prefix. Each format has an exact field count:
//
//	1 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <path>                       (9 fields)
//	2 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <Xscore> <path>\t<origPath>  (10 fields)
//	u <XY> <sub> <m1> <m2> <m3> <mW> <h1> <h2> <h3> <path>            (11 fields)
//	? <path>
//	! <path> (ignored entries)
//
// The parser returns ok=false for malformed lines, `#`/comment lines, and
// ignored (`!`) records, which never appear in status views.
func parsePorcelainV2Line(line string) (porcelainEntry, bool) {
	if len(line) < 2 {
		return porcelainEntry{}, false
	}
	switch line[0] {
	case '?':
		if line[1] != ' ' {
			return porcelainEntry{}, false
		}
		return porcelainEntry{Kind: "?", Path: unquoteGitPath(line[2:])}, line[2:] != ""
	case '!':
		return porcelainEntry{}, false
	case '1':
		fields := strings.SplitN(line, " ", 9)
		if len(fields) != 9 {
			return porcelainEntry{}, false
		}
		return porcelainEntry{Kind: "1", XY: fields[1], Path: unquoteGitPath(fields[8])}, true
	case '2':
		fields := strings.SplitN(line, " ", 10)
		if len(fields) != 10 {
			return porcelainEntry{}, false
		}
		e := porcelainEntry{Kind: "2", XY: fields[1]}
		// Renamed/copied entries carry "<path>\t<origPath>" in the last field.
		p, orig, hasOrig := strings.Cut(fields[9], "\t")
		e.Path = unquoteGitPath(p)
		if hasOrig {
			e.OrigPath = unquoteGitPath(orig)
		}
		return e, true
	case 'u':
		fields := strings.SplitN(line, " ", 11)
		if len(fields) != 11 {
			return porcelainEntry{}, false
		}
		return porcelainEntry{Kind: "u", XY: fields[1], Path: unquoteGitPath(fields[10])}, true
	}
	return porcelainEntry{}, false
}

// unquoteGitPath removes the C-style quoting git wraps around paths that
// contain control characters (with core.quotepath=false, unicode names stay
// verbatim; only exotic bytes get quoted).
func unquoteGitPath(p string) string {
	if len(p) >= 2 && p[0] == '"' && p[len(p)-1] == '"' {
		if unq, err := strconv.Unquote(p); err == nil {
			return unq
		}
	}
	return p
}

// GetStatus returns lightweight git status using porcelain v2 format.
// Results are cached per repo for statusTTL; concurrent callers within that
// window share one cached result instead of each spawning `git status`
// (which contended on .git/index.lock and stalled the UI on large repos).
// Mutations invalidate the cache so status is always fresh after a change.
//
// The in-flight wait is a loop: after waiting on a fetch, the cache is
// re-read because an invalidation (Invalidate/InvalidateAll fires on every
// file event, e.g. during a branch checkout) may have dropped or replaced
// the entry mid-wait. The previous implementation fell through to the TTL
// branch with the mutex already unlocked and crashed on double Unlock.
func (e *Engine) GetStatus(ctx context.Context, repoPath string) (*GitStatusResult, error) {
	if root, ok := FindRepoRoot(repoPath); ok {
		repoPath = root
	}
	for {
		e.statusMu.Lock()
		ent, ok := e.statusCache[repoPath]
		if ok && ent.done != nil {
			// A fetch is in flight: wait outside the lock, then re-check.
			done := ent.done
			e.statusMu.Unlock()
			select {
			case <-done:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			continue
		}
		if ok && time.Since(ent.cachedAt) < statusTTL {
			res, err := ent.res, ent.err
			e.statusMu.Unlock()
			return res, err
		}
		// No valid entry — become the fetcher.
		ent = &statusEntry{done: make(chan struct{})}
		e.statusCache[repoPath] = ent
		e.statusMu.Unlock()

		res, err := e.runGitStatus(ctx, repoPath)

		e.statusMu.Lock()
		// Publish only if this entry is still the registered one: a
		// concurrent Invalidate/InvalidateAll may have dropped it mid-fetch
		// (the next caller will re-run status). done is closed exactly once,
		// whether or not the result was published.
		if cur, ok := e.statusCache[repoPath]; ok && cur == ent {
			ent.res = res
			ent.err = err
			ent.cachedAt = time.Now()
		}
		close(ent.done)
		ent.done = nil
		e.statusMu.Unlock()
		return res, err
	}
}

// resolveBranch retrieves the current branch name safely via rev-parse,
// symbolic-ref, direct .git/HEAD inspection, or status cache, avoiding "unknown".
func (e *Engine) resolveBranch(ctx context.Context, repoPath string) string {
	if root, ok := FindRepoRoot(repoPath); ok {
		repoPath = root
	}

	// 1. Try rev-parse --abbrev-ref HEAD
	cmd := gitCmd(ctx, repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if out, err := cmd.Output(); err == nil {
		b := strings.TrimSpace(string(out))
		if b != "" && b != "HEAD" {
			return b
		}
	}

	// 2. Try symbolic-ref --short HEAD
	cmdRef := gitCmd(ctx, repoPath, "symbolic-ref", "--short", "HEAD")
	if out, err := cmdRef.Output(); err == nil {
		b := strings.TrimSpace(string(out))
		if b != "" {
			return b
		}
	}

	// 3. Fallback: inspect .git/HEAD directly without spawning git processes
	headFile := filepath.Join(repoPath, ".git", "HEAD")
	if data, err := os.ReadFile(headFile); err == nil {
		content := strings.TrimSpace(string(data))
		if strings.HasPrefix(content, "ref: refs/heads/") {
			return strings.TrimPrefix(content, "ref: refs/heads/")
		}
		// Worktrees / git submodules gitdir pointer
		if strings.HasPrefix(content, "gitdir:") {
			gitDir := strings.TrimSpace(strings.TrimPrefix(content, "gitdir:"))
			if !filepath.IsAbs(gitDir) {
				gitDir = filepath.Join(repoPath, gitDir)
			}
			if data2, err2 := os.ReadFile(filepath.Join(gitDir, "HEAD")); err2 == nil {
				c2 := strings.TrimSpace(string(data2))
				if strings.HasPrefix(c2, "ref: refs/heads/") {
					return strings.TrimPrefix(c2, "ref: refs/heads/")
				}
			}
		}
		if len(content) >= 7 {
			return fmt.Sprintf("(detached at %s)", content[:7])
		}
	}

	// 4. Fallback to existing non-empty branch in status cache
	e.statusMu.Lock()
	if ent, ok := e.statusCache[repoPath]; ok && ent.res != nil && ent.res.Branch != "" && ent.res.Branch != "unknown" && ent.res.Branch != "(unknown)" {
		prev := ent.res.Branch
		e.statusMu.Unlock()
		return prev
	}
	e.statusMu.Unlock()

	return "main"
}

func (e *Engine) runGitStatus(ctx context.Context, repoPath string) (*GitStatusResult, error) {
	if root, ok := FindRepoRoot(repoPath); ok {
		repoPath = root
	}
	// -uall lists every untracked FILE individually (VS Code behavior) instead
	// of collapsing an untracked directory into a single "? dir/" entry whose
	// filename is empty and can't be staged/opened. core.quotepath=false keeps
	// unicode filenames verbatim (otherwise git C-quotes them); the parser
	// still unquotes for the control-character case.
	cmd := gitCmd(ctx, repoPath, "-c", "core.quotepath=false", "status", "--porcelain=v2", "-b", "-uall")

	outBytes, err := cmd.CombinedOutput()
	// If index.lock contention happened (e.g. during branch checkout), retry once
	if err != nil && strings.Contains(string(outBytes), "index.lock") {
		select {
		case <-time.After(60 * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		cmdRetry := gitCmd(ctx, repoPath, "-c", "core.quotepath=false", "status", "--porcelain=v2", "-b", "-uall")
		outBytes, err = cmdRetry.CombinedOutput()
	}

	if err != nil {
		// Surface the real failure instead of fabricating a clean status:
		// callers must be able to tell "no changes" apart from "git is broken"
		// (missing repo, corrupt index, git not installed, ...).
		return nil, fmt.Errorf("git status %s: %s", repoPath, strings.TrimSpace(string(outBytes)))
	}

	res := &GitStatusResult{
		Branch:    "main",
		Staged:    make([]FileStatus, 0),
		Unstaged:  make([]FileStatus, 0),
		Untracked: make([]FileStatus, 0),
		Conflicts: make([]FileStatus, 0),
	}

	scanner := bufio.NewScanner(bytes.NewReader(outBytes))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "# branch.head ") {
			res.Branch = strings.TrimPrefix(line, "# branch.head ")
			continue
		}

		entry, ok := parsePorcelainV2Line(line)
		if !ok {
			continue
		}

		switch entry.Kind {
		case "1", "2":
			// Ordinary (1) or renamed/copied (2) changed entry. XY is two
			// chars: X = index (staged) state, Y = worktree (unstaged) state.
			if len(entry.XY) != 2 {
				continue
			}
			stagedChar := string(entry.XY[0])
			unstagedChar := string(entry.XY[1])

			if stagedChar != "." {
				res.Staged = append(res.Staged, FileStatus{
					Path:    entry.Path,
					Dir:     dirOf(entry.Path),
					Staging: "staged",
					Status:  stagedChar,
				})
			}
			if unstagedChar != "." {
				res.Unstaged = append(res.Unstaged, FileStatus{
					Path:    entry.Path,
					Dir:     dirOf(entry.Path),
					Staging: "unstaged",
					Status:  unstagedChar,
				})
			}

		case "u":
			// Unmerged (conflict) entry: XY is e.g. UU, AU, UA, DU, AA, DD.
			res.Conflicts = append(res.Conflicts, FileStatus{
				Path:    entry.Path,
				Dir:     dirOf(entry.Path),
				Staging: "conflict",
				Status:  entry.XY,
			})

		case "?":
			// Untracked file: ? <path>
			res.Untracked = append(res.Untracked, FileStatus{
				Path:    entry.Path,
				Dir:     dirOf(entry.Path),
				Staging: "untracked",
				Status:  "?",
			})
		}
	}

	if res.Branch == "" || res.Branch == "unknown" || res.Branch == "(unknown)" {
		res.Branch = e.resolveBranch(ctx, repoPath)
	}

	annotateNumstat(ctx, repoPath, res)
	return res, nil
}

const (
	maxUntrackedLineCountFiles = 64
	maxUntrackedLineCountBytes = 256 << 10 // 256 KB
)

// annotateNumstat fills Additions/Deletions from `git diff --numstat`
// (worktree and index); untracked files get their line count as additions
// since numstat does not cover them. Display-only metadata: any failure
// leaves the counts at zero.
func annotateNumstat(ctx context.Context, repoPath string, res *GitStatusResult) {
	if len(res.Staged) > 0 {
		index := numstatMap(ctx, repoPath, true)
		for i := range res.Staged {
			if counts, ok := index[res.Staged[i].Path]; ok {
				res.Staged[i].Additions, res.Staged[i].Deletions = counts[0], counts[1]
			}
		}
	}
	if len(res.Unstaged) > 0 {
		worktree := numstatMap(ctx, repoPath, false)
		for i := range res.Unstaged {
			if counts, ok := worktree[res.Unstaged[i].Path]; ok {
				res.Unstaged[i].Additions, res.Unstaged[i].Deletions = counts[0], counts[1]
			}
		}
	}
	limit := len(res.Untracked)
	if limit > maxUntrackedLineCountFiles {
		limit = maxUntrackedLineCountFiles
	}
	for i := 0; i < limit; i++ {
		res.Untracked[i].Additions = countUntrackedLines(repoPath, res.Untracked[i].Path)
	}
}

// numstatMap parses `git diff --numstat` into path → {additions, deletions}.
// Binary files report "-" and map to zero. Rename entries use the
// `dir/{old => new}` notation and are folded to the destination path.
func numstatMap(ctx context.Context, repoPath string, cached bool) map[string][2]int {
	args := []string{"diff", "--numstat"}
	if cached {
		args = append(args, "--cached")
	}
	cmd := gitCmd(ctx, repoPath, args...)
	out, err := cmd.Output()
	counts := make(map[string][2]int)
	if err != nil {
		return counts
	}
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		add, _ := strconv.Atoi(parts[0])
		del, _ := strconv.Atoi(parts[1])
		counts[numstatRenamePath(parts[2])] = [2]int{add, del}
	}
	return counts
}

// numstatRenamePath resolves a numstat rename path (`dir/{old => new}`,
// `{a => b}.go`, or plain `old => new`) to the destination path.
func numstatRenamePath(p string) string {
	if !strings.Contains(p, "=>") {
		return p
	}
	prefix, body, suffix := "", p, ""
	if open := strings.Index(p, "{"); open != -1 {
		if close := strings.LastIndex(p, "}"); close > open {
			prefix, body, suffix = p[:open], p[open+1:close], p[close+1:]
		}
	}
	parts := strings.SplitN(body, "=>", 2)
	if len(parts) != 2 {
		return p
	}
	return prefix + strings.TrimSpace(parts[1]) + suffix
}

// countUntrackedLines counts a new file's lines as additions. Binary
// (NUL byte in the first 8KB) and oversized files report 0.
// Uses a fixed 4KB stack buffer instead of reading the entire file into heap memory.
func countUntrackedLines(repoPath, path string) int {
	full := filepath.Join(repoPath, path)
	info, err := os.Stat(full)
	if err != nil || info.IsDir() || info.Size() == 0 || info.Size() > maxUntrackedLineCountBytes {
		return 0
	}
	f, err := os.Open(full)
	if err != nil {
		return 0
	}
	defer f.Close()

	var buf [4096]byte
	lines := 0
	totalRead := 0
	var lastByte byte
	for {
		n, readErr := f.Read(buf[:])
		if n > 0 {
			chunk := buf[:n]
			if totalRead < 8000 {
				checkLen := 8000 - totalRead
				if checkLen > n {
					checkLen = n
				}
				if bytes.IndexByte(chunk[:checkLen], 0) >= 0 {
					return 0
				}
			}
			lines += bytes.Count(chunk, []byte{'\n'})
			lastByte = chunk[n-1]
			totalRead += n
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return 0
		}
	}
	if totalRead == 0 {
		return 0
	}
	if lastByte != '\n' {
		lines++
	}
	return lines
}

// dirOf returns the parent directory of a path, or "" for a top-level file.
func dirOf(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx <= 0 {
		return ""
	}
	return path[:idx]
}

// CheckIgnored returns the list of paths that match gitignore rules.
func (e *Engine) CheckIgnored(ctx context.Context, repoPath string, paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	args := []string{"check-ignore", "--"}
	args = append(args, paths...)
	cmd := gitCmd(ctx, repoPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
	}
	var ignored []string
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			ignored = append(ignored, line)
		}
	}
	return ignored, nil
}

// Stage adds files to staging index.
func (e *Engine) Stage(ctx context.Context, repoPath string, paths []string) error {
	defer e.invalidate(repoPath)
	args := []string{"add"}
	if len(paths) == 0 {
		args = append(args, ".")
	} else {
		args = append(args, "--")
		args = append(args, paths...)
	}
	cmd := gitCmd(ctx, repoPath, args...)
	return cmd.Run()
}

// Unstage removes files from staging index.
func (e *Engine) Unstage(ctx context.Context, repoPath string, paths []string) error {
	defer e.invalidate(repoPath)
	args := []string{"restore", "--staged"}
	if len(paths) == 0 {
		args = append(args, ".")
	} else {
		args = append(args, "--")
		args = append(args, paths...)
	}
	cmd := gitCmd(ctx, repoPath, args...)
	return cmd.Run()
}

// Discard reverts file changes back to HEAD, mirroring VS Code's "Discard
// Changes": tracked files are fully reset (staged + worktree) with
// `git restore --staged --worktree`, and untracked files are removed with
// `git clean`. For each path only the command that applies is run — running
// `git clean` on a tracked file (or `restore` on an untracked one) errors, so
// each path falls back to the other command before being counted as failed.
func (e *Engine) Discard(ctx context.Context, repoPath string, paths []string) error {
	defer e.invalidate(repoPath)
	if len(paths) == 0 {
		cmd1 := gitCmd(ctx, repoPath, "restore", "--staged", "--worktree", ".")
		_ = cmd1.Run()

		cmd2 := gitCmd(ctx, repoPath, "clean", "-fd")
		return cmd2.Run()
	}

	var failures, processed int
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			continue
		}
		processed++
		restoreCmd := gitCmd(ctx, repoPath, "restore", "--staged", "--worktree", "--", p)
		if err := restoreCmd.Run(); err != nil {
			// Not a tracked file (untracked, staged-new, or a new dir) — remove it.
			cleanCmd := gitCmd(ctx, repoPath, "clean", "-fd", "--", p)
			if cerr := cleanCmd.Run(); cerr != nil {
				failures++
			}
		}
	}

	// Only report an error when every path failed both restore AND clean.
	if processed > 0 && failures == processed {
		return fmt.Errorf("discard failed: no tracked or untracked changes to discard for the given paths")
	}
	return nil
}

// Commit creates a git commit. If the first attempt fails (e.g. an
// index.lock race or a hook raced with staging), the retry stages ONLY the
// paths that were already staged before the first attempt — never
// `git add -A`, which would silently sweep unrelated working-tree changes
// (secrets, WIP, generated files) into the commit.
func (e *Engine) Commit(ctx context.Context, repoPath string, message string) error {
	defer e.invalidate(repoPath)
	if strings.TrimSpace(message) == "" {
		return fmt.Errorf("commit message cannot be empty")
	}

	// Capture the already-staged paths up front (quotepath=false keeps
	// unicode filenames verbatim so the retry re-stages the exact paths).
	stagedOut, _ := gitCmd(ctx, repoPath, "-c", "core.quotepath=false", "diff", "--cached", "--name-only").Output()
	var stagedPaths []string
	for _, l := range strings.Split(string(stagedOut), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			stagedPaths = append(stagedPaths, l)
		}
	}

	cmd := gitCmd(ctx, repoPath, "commit", "-m", message)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	if len(stagedPaths) == 0 {
		// Nothing was staged to re-stage — retrying could only widen the
		// commit, so surface the original failure.
		return fmt.Errorf("git commit: %s", strings.TrimSpace(string(out)))
	}

	addArgs := append([]string{"add", "--"}, stagedPaths...)
	_ = gitCmd(ctx, repoPath, addArgs...).Run()

	cmd2 := gitCmd(ctx, repoPath, "commit", "-m", message)
	out2, err2 := cmd2.CombinedOutput()
	if err2 != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out2)))
	}
	return nil
}

// Push pushes committed commits to remote.
func (e *Engine) Push(ctx context.Context, repoPath string) error {
	defer e.invalidate(repoPath)
	cmd := gitCmd(ctx, repoPath, "push")
	out, err := cmd.CombinedOutput()
	if err != nil {
		cmdUpstream := gitCmd(ctx, repoPath, "push", "-u", "origin", "HEAD")
		_, errUpstream := cmdUpstream.CombinedOutput()
		if errUpstream != nil {
			return fmt.Errorf("git push: %s", string(out))
		}
	}
	return nil
}

// GetStagedDiff returns the staged diff for AI commit generation.
func (e *Engine) GetStagedDiff(ctx context.Context, repoPath string) (string, error) {
	cmd := gitCmd(ctx, repoPath, "diff", "--staged")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// GetStagedDiffStat returns the diff stat summary for the staged changes.
func (e *Engine) GetStagedDiffStat(ctx context.Context, repoPath string) (string, error) {
	cmd := gitCmd(ctx, repoPath, "diff", "--staged", "--stat")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// GetWorkingTreeDiff returns the full working-tree diff vs HEAD (staged +
// unstaged tracked changes) — AI commit fallback when nothing is staged.
func (e *Engine) GetWorkingTreeDiff(ctx context.Context, repoPath string) (string, error) {
	cmd := gitCmd(ctx, repoPath, "diff", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// GetWorkingTreeDiffStat returns the diff stat summary of the working tree vs HEAD.
func (e *Engine) GetWorkingTreeDiffStat(ctx context.Context, repoPath string) (string, error) {
	cmd := gitCmd(ctx, repoPath, "diff", "HEAD", "--stat")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// GetFileDiff returns the unified diff for a single file against HEAD
// (combines staged + unstaged working-tree changes).
func (e *Engine) GetFileDiff(ctx context.Context, repoPath string, path string) (string, error) {
	if root, ok := FindRepoRoot(repoPath); ok {
		repoPath = root
	}
	if strings.TrimSpace(path) == "" || path == "all" || path == "Working Tree Changes" {
		cmd := gitCmd(ctx, repoPath, "diff", "HEAD")
		out, err := cmd.CombinedOutput()
		if err != nil {
			cmd2 := gitCmd(ctx, repoPath, "diff")
			out2, _ := cmd2.CombinedOutput()
			return string(out2), nil
		}
		return string(out), nil
	}

	// Resolve path relative to repoPath if absolute
	if filepath.IsAbs(path) {
		if rel, err := filepath.Rel(repoPath, path); err == nil {
			path = rel
		}
	}

	cmd := gitCmd(ctx, repoPath, "diff", "HEAD", "--", path)
	out, err := cmd.CombinedOutput()
	if err == nil && len(strings.TrimSpace(string(out))) > 0 {
		return string(out), nil
	}

	cmd2 := gitCmd(ctx, repoPath, "diff", "--", path)
	out2, _ := cmd2.CombinedOutput()
	if len(strings.TrimSpace(string(out2))) > 0 {
		return string(out2), nil
	}

	// Try top-level repo pathspec or wildcard if not matched yet
	if !strings.HasPrefix(path, ":") {
		topPath := ":/" + strings.TrimPrefix(path, "/")
		cmdTop := gitCmd(ctx, repoPath, "diff", "HEAD", "--", topPath)
		if outTop, errTop := cmdTop.CombinedOutput(); errTop == nil && len(strings.TrimSpace(string(outTop))) > 0 {
			return string(outTop), nil
		}
		cmdTop2 := gitCmd(ctx, repoPath, "diff", "--", topPath)
		if outTop2, errTop2 := cmdTop2.CombinedOutput(); errTop2 == nil && len(strings.TrimSpace(string(outTop2))) > 0 {
			return string(outTop2), nil
		}
		if !strings.Contains(path, "/") {
			cmdWild := gitCmd(ctx, repoPath, "diff", "HEAD", "--", ":/**/"+path)
			if outWild, errWild := cmdWild.CombinedOutput(); errWild == nil && len(strings.TrimSpace(string(outWild))) > 0 {
				return string(outWild), nil
			}
			cmdWild2 := gitCmd(ctx, repoPath, "diff", "--", ":/**/"+path)
			if outWild2, errWild2 := cmdWild2.CombinedOutput(); errWild2 == nil && len(strings.TrimSpace(string(outWild2))) > 0 {
				return string(outWild2), nil
			}
		}
	}

	// If untracked file, show whole file content as additions (capped at 1MB, non-binary)
	fullPath := path
	if !filepath.IsAbs(fullPath) {
		fullPath = filepath.Join(repoPath, path)
	}
	if info, statErr := os.Stat(fullPath); statErr == nil && !info.IsDir() && info.Size() <= 1<<20 {
		if fileData, readErr := os.ReadFile(fullPath); readErr == nil {
			head := fileData
			if len(head) > 8000 {
				head = head[:8000]
			}
			if bytes.IndexByte(head, 0) < 0 {
				lines := strings.Split(string(fileData), "\n")
				var b strings.Builder
				b.Grow(len(fileData) + len(lines) + 64)
				b.WriteString(fmt.Sprintf("--- /dev/null\n+++ b/%s\n@@ -0,0 +1,%d @@\n", path, len(lines)))
				for _, l := range lines {
					b.WriteString("+" + l + "\n")
				}
				return b.String(), nil
			}
		}
	}

	return string(out), nil
}

// Fetch updates remote-tracking branches from the default remote.
func (e *Engine) Fetch(ctx context.Context, repoPath string) (string, error) {
	defer e.invalidate(repoPath)
	cmd := gitCmd(ctx, repoPath, "fetch", "--prune")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Merge merges the given source commit/branch into the current branch.
// When noFF is true a merge commit is always created; when squash is true
// changes are applied without creating a merge commit.
func (e *Engine) Merge(ctx context.Context, repoPath string, source string, noFF bool, squash bool) (string, error) {
	defer e.invalidate(repoPath)
	if strings.TrimSpace(source) == "" {
		return "", fmt.Errorf("merge source cannot be empty")
	}
	args := []string{"merge"}
	if noFF {
		args = append(args, "--no-ff")
	}
	if squash {
		args = append(args, "--squash")
	}
	args = append(args, "--")
	args = append(args, source)
	cmd := gitCmd(ctx, repoPath, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
