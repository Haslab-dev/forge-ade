package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type CommitNode struct {
	Hash        string    `json:"hash"`
	ShortHash   string    `json:"short_hash"`
	Parents     []string  `json:"parents"`
	AuthorName  string    `json:"author_name"`
	AuthorEmail string    `json:"author_email"`
	Timestamp   time.Time `json:"timestamp"`
	Message     string    `json:"message"`
	GraphPrefix string    `json:"graph_prefix"`
	Decorations string    `json:"decorations"`
	Status      string    `json:"status"`
}

type CommitGraphResult struct {
	Commits    []CommitNode `json:"commits"`
	TotalCount int          `json:"total_count"`
	HasMore    bool         `json:"has_more"`
	Offset     int          `json:"offset"`
	Limit      int          `json:"limit"`
}

type Engine struct {
	statusMu   sync.Mutex
	statusCache map[string]*statusEntry
}

// statusTTL caps how often git status is re-run per repo. Repeated reads
// (branch badge, explorer badges, git panel) hit the cached result instead of
// spawning a fresh `git status` every call, which pegged the CPU on large
// repos. Mutations (stage/commit/discard/...) invalidate the entry so the
// next call is always fresh.
const statusTTL = 5 * time.Second

// statusEntry tracks one git status run per repo. While in-flight it acts as
// a singleflight gate: concurrent callers share one `git status` spawn
// instead of N processes (which contended on .git/index.lock and made the
// app hang on large repos). After completion the result is cached until the
// TTL expires or a mutation invalidates it.
type statusEntry struct {
	res       *GitStatusResult
	err       error
	done      chan struct{} // non-nil while in-flight
	cachedAt  time.Time
}

// invalidate drops the cached status for a repo so the next GetStatus
// re-runs git status. Call after any mutation (stage, commit, discard...) or
// after file-system changes that may affect the working tree.
func (e *Engine) invalidate(repoPath string) {
	e.statusMu.Lock()
	delete(e.statusCache, repoPath)
	e.statusMu.Unlock()
}

// Invalidate drops the cached status for a repo, forcing the next GetStatus
// to re-run git status. Call when the working tree may have changed outside
// the Engine's own mutation methods (e.g. external file edits).
func (e *Engine) Invalidate(repoPath string) {
	e.invalidate(repoPath)
}

// InvalidateAll drops every cached status. Used by the lazy dirty sweep: file
// events only set a dirty flag, and the next status consumer clears the whole
// cache once so each repo re-runs at most once per TTL window.
func (e *Engine) InvalidateAll() {
	e.statusMu.Lock()
	e.statusCache = make(map[string]*statusEntry)
	e.statusMu.Unlock()
}

func NewEngine() *Engine {
	return &Engine{statusCache: make(map[string]*statusEntry)}
}

// GetCommitGraph fetches lightweight streaming git graph commits with pagination.
// Avoids memory leaks by relying on native C git stdout streaming instead of loading object graphs into RAM.
// GetBranches returns local branch names for the repo.
func (e *Engine) GetBranches(ctx context.Context, repoPath string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "for-each-ref", "--format=%(refname:short)", "refs/heads")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

func (e *Engine) GetCommitGraph(ctx context.Context, repoPath string, offset int, limit int, branch string) (*CommitGraphResult, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	revRange := "--all"
	if branch != "" {
		revRange = branch
	}

	// 1. Get total commit count (fast)
	countCmd := exec.CommandContext(ctx, "git", "rev-list", "--count", revRange)
	countCmd.Dir = repoPath
	countOut, err := countCmd.Output()
	totalCount := 0
	if err == nil {
		totalCount, _ = strconv.Atoi(strings.TrimSpace(string(countOut)))
	}

	// 2. Fetch paginated graph commits using format string
	// Format: %H|%P|%an|%ae|%at|%s|%d
	skipArg := fmt.Sprintf("--skip=%d", offset)
	maxArg := fmt.Sprintf("-n%d", limit)
	formatArg := "--format=format:GITCOMMIT|%H|%P|%an|%ae|%at|%s|%d"

	cmd := exec.CommandContext(ctx, "git", "log", "--graph", "--oneline", skipArg, maxArg, formatArg, revRange)
	cmd.Dir = repoPath

	outBytes, err := cmd.CombinedOutput()
	if err != nil {
		return &CommitGraphResult{Commits: []CommitNode{}, TotalCount: 0, HasMore: false}, nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(outBytes))
	var commits []CommitNode

	pushedSet, stashSet := commitStatusSets(ctx, repoPath)

	for scanner.Scan() {
		line := scanner.Text()
		idx := strings.Index(line, "GITCOMMIT|")
		if idx == -1 {
			continue
		}

		graphPrefix := line[:idx]
		metaStr := line[idx+len("GITCOMMIT|"):]
		parts := strings.Split(metaStr, "|")
		if len(parts) < 6 {
			continue
		}

		hash := parts[0]
		parentStr := parts[1]
		authorName := parts[2]
		authorEmail := parts[3]
		tsSec, _ := strconv.ParseInt(parts[4], 10, 64)
		msg := parts[5]

		var decorations string
		if len(parts) > 6 {
			decorations = parts[6]
		}

		var parents []string
		if strings.TrimSpace(parentStr) != "" {
			parents = strings.Split(parentStr, " ")
		}

		shortHash := hash
		if len(hash) >= 7 {
			shortHash = hash[:7]
		}

		status := "local"
		if stashSet[hash] {
			status = "stash"
		} else if pushedSet[hash] {
			status = "pushed"
		}

		commits = append(commits, CommitNode{
			Hash:        hash,
			ShortHash:   shortHash,
			Parents:     parents,
			AuthorName:  authorName,
			AuthorEmail: authorEmail,
			Timestamp:   time.Unix(tsSec, 0),
			Message:     msg,
			GraphPrefix: graphPrefix,
			Decorations: decorations,
			Status:      status,
		})
	}

	hasMore := offset+len(commits) < totalCount

	return &CommitGraphResult{
		Commits:    commits,
		TotalCount: totalCount,
		HasMore:    hasMore,
		Offset:     offset,
		Limit:      limit,
	}, nil
}

// commitStatusSets builds sets of commit hashes that are pushed (reachable from
// any remote-tracking branch) or the current stash tip.
func commitStatusSets(ctx context.Context, repoPath string) (pushed, stash map[string]bool) {
	pushed = map[string]bool{}
	stash = map[string]bool{}

	// Remote-tracking refs (refs/remotes/*) → pushed commits.
	refCmd := exec.CommandContext(ctx, "git", "for-each-ref", "--format=%(refname)", "refs/remotes/")
	refCmd.Dir = repoPath
	out, err := refCmd.Output()
	if err == nil {
		var refs []string
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line != "" {
				refs = append(refs, line)
			}
		}
		if len(refs) > 0 {
			args := append([]string{"rev-list"}, refs...)
			revCmd := exec.CommandContext(ctx, "git", args...)
			revCmd.Dir = repoPath
			out, err := revCmd.Output()
			if err == nil {
				for _, h := range strings.Fields(string(out)) {
					pushed[h] = true
				}
			}
		}
	}

	// Stash tip (refs/stash) → stash commit.
	parseCmd := exec.CommandContext(ctx, "git", "rev-parse", "--verify", "--quiet", "refs/stash")
	parseCmd.Dir = repoPath
	if tip, err := parseCmd.Output(); err == nil {
		if h := strings.TrimSpace(string(tip)); h != "" {
			stash[h] = true
		}
	}

	return pushed, stash
}

// GetCommitDiff retrieves diff details on demand for a single commit without keeping full diffs in RAM.
func (e *Engine) GetCommitDiff(ctx context.Context, repoPath string, hash string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "show", "--patch", "--stat", hash)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git show error: %w", err)
	}
	return string(out), nil
}

// GetCommitBody returns the full commit message (subject + body description).
func (e *Engine) GetCommitBody(ctx context.Context, repoPath string, hash string) (string, error) {
	if strings.TrimSpace(hash) == "" {
		return "", fmt.Errorf("commit hash cannot be empty")
	}
	cmd := exec.CommandContext(ctx, "git", "log", "-1", "--format=%B", hash)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git log error: %w", err)
	}
	return string(out), nil
}

// GetCommitFileDiff returns the unified diff of a single file within a commit.
func (e *Engine) GetCommitFileDiff(ctx context.Context, repoPath string, hash string, path string) (string, error) {
	if strings.TrimSpace(hash) == "" || strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("commit hash and file path are required")
	}
	cmd := exec.CommandContext(ctx, "git", "show", "--patch", "--stat", hash, "--", path)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git show error: %w", err)
	}
	return string(out), nil
}

// GetFileContentAtCommit returns the raw file content at a given commit.
func (e *Engine) GetFileContentAtCommit(ctx context.Context, repoPath string, hash string, path string) (string, error) {
	if strings.TrimSpace(hash) == "" || strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("commit hash and file path are required")
	}
	ref := hash + ":" + path
	cmd := exec.CommandContext(ctx, "git", "show", ref)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git show %s: %w", ref, err)
	}
	return string(out), nil
}
