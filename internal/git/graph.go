package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type CommitNode struct {
	Hash        string    `json:"hash"`
	ShortHash   string    `json:"short_hash"`
	Parents     []string  `json:"parents"`
	AuthorName  string    `json:"author_name"`
	AuthorEmail string    `json:"author_email"`
	Timestamp   time.Time `json:"timestamp"`
	Message     string    `json:"message"`     // subject line
	GraphPrefix string    `json:"graph_prefix"`
	Decorations string    `json:"decorations"` // e.g. " (HEAD -> main, origin/main)"
}

type CommitGraphResult struct {
	Commits    []CommitNode `json:"commits"`
	TotalCount int          `json:"total_count"`
	HasMore    bool         `json:"has_more"`
	Offset     int          `json:"offset"`
	Limit      int          `json:"limit"`
}

type Engine struct{}

func NewEngine() *Engine {
	return &Engine{}
}

// GetCommitGraph fetches lightweight streaming git graph commits with pagination.
// Avoids memory leaks by relying on native C git stdout streaming instead of loading object graphs into RAM.
func (e *Engine) GetCommitGraph(ctx context.Context, repoPath string, offset int, limit int) (*CommitGraphResult, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	// 1. Get total commit count (fast)
	countCmd := exec.CommandContext(ctx, "git", "rev-list", "--count", "HEAD")
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

	cmd := exec.CommandContext(ctx, "git", "log", "--graph", "--oneline", skipArg, maxArg, formatArg, "--all")
	cmd.Dir = repoPath

	outBytes, err := cmd.CombinedOutput()
	if err != nil {
		return &CommitGraphResult{Commits: []CommitNode{}, TotalCount: 0, HasMore: false}, nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(outBytes))
	var commits []CommitNode

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
