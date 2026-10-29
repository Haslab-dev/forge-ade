package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hasdev/forge-ade/internal/commitmsg"
	"github.com/hasdev/forge-ade/internal/git"
)

func (a *App) gitContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	base := a.ctx
	if base == nil {
		base = context.Background()
	}
	return context.WithTimeout(base, timeout)
}

func (a *App) resolveGitRepoPath(repoPath string) string {
	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		repoPath = a.defaultFolder()
	}
	cur := repoPath
	if fi, err := os.Stat(cur); err == nil && !fi.IsDir() {
		cur = filepath.Dir(cur)
	}
	for {
		if _, err := os.Stat(filepath.Join(cur, ".git")); err == nil {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur || parent == "." || parent == "/" {
			break
		}
		cur = parent
	}
	return repoPath
}

// GetGitCommitGraph streams lightweight paginated Git commits with graph prefix.
// branch "" means all branches.
func (a *App) GetGitCommitGraph(repoPath string, offset int, limit int, branch string) (*git.CommitGraphResult, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()
	return a.gitEngine.GetCommitGraph(ctx, repoPath, offset, limit, branch)
}

// GetGitBranches lists local branch names for the repo.
func (a *App) GetGitBranches(repoPath string) ([]string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(15 * time.Second)
	defer cancel()
	return a.gitEngine.GetBranches(ctx, repoPath)
}

// GitCheckout switches the working tree to the given branch.
func (a *App) GitCheckout(repoPath string, branch string) error {
	repoPath = a.resolveGitRepoPath(repoPath)
	if branch == "" {
		return fmt.Errorf("branch name is required")
	}
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "checkout", branch)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("checkout %s failed: %s", branch, strings.TrimSpace(string(out)))
	}
	a.emitEvent("git:changed", repoPath)
	return nil
}

// GitCheckoutNewBranch creates a new branch from HEAD and switches to it
// (git checkout -b). Used by the header branch switcher's
// "Create and switch to new branch…" action.
func (a *App) GitCheckoutNewBranch(repoPath string, name string) error {
	repoPath = a.resolveGitRepoPath(repoPath)
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("branch name is required")
	}
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "checkout", "-b", name)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("create branch %s failed: %s", name, strings.TrimSpace(string(out)))
	}
	a.emitEvent("git:changed", repoPath)
	return nil
}

// GetGitCommitDiff returns details and patch for a single commit.
func (a *App) GetGitCommitDiff(repoPath string, hash string) (string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()
	return a.gitEngine.GetCommitDiff(ctx, repoPath, hash)
}

// GetGitCommitBody returns the full commit message (subject + body) for a commit.
func (a *App) GetGitCommitBody(repoPath string, hash string) (string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(15 * time.Second)
	defer cancel()
	return a.gitEngine.GetCommitBody(ctx, repoPath, hash)
}

// GetGitFileDiff returns the unified diff of a single working-tree file against HEAD.
func (a *App) GetGitFileDiff(repoPath string, path string) (string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()
	return a.gitEngine.GetFileDiff(ctx, repoPath, path)
}

// GetGitCommitFileDiff returns the unified diff of a single file within a commit.
func (a *App) GetGitCommitFileDiff(repoPath string, hash string, path string) (string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()
	return a.gitEngine.GetCommitFileDiff(ctx, repoPath, hash, path)
}

// GetGitFileDiffHunks returns the structured hunks of a file's diff against HEAD.
func (a *App) GetGitFileDiffHunks(repoPath string, path string) ([]git.DiffHunk, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()
	return a.gitEngine.GetFileDiffHunks(ctx, repoPath, path)
}

// RevertGitHunk reverse-applies a single diff hunk, restoring it to HEAD state.
func (a *App) RevertGitHunk(repoPath string, path string, hunkIndex int) error {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(15 * time.Second)
	defer cancel()
	return a.gitEngine.RevertDiffHunk(ctx, repoPath, path, hunkIndex)
}

// GetGitFileContentAtCommit returns the raw file content at a given commit.
func (a *App) GetGitFileContentAtCommit(repoPath string, hash string, path string) (string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()
	return a.gitEngine.GetFileContentAtCommit(ctx, repoPath, hash, path)
}

// GetGitStatus returns lightweight git status (staged, unstaged, untracked).
func (a *App) GetGitStatus(repoPath string) (*git.GitStatusResult, error) {
	a.sweepGitDirty()
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()
	return a.gitEngine.GetStatus(ctx, repoPath)
}

// GitStage stages files.
func (a *App) GitStage(repoPath string, paths []string) error {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()
	return a.gitEngine.Stage(ctx, repoPath, paths)
}

// GitUnstage unstages files.
func (a *App) GitUnstage(repoPath string, paths []string) error {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()
	return a.gitEngine.Unstage(ctx, repoPath, paths)
}

// GitDiscard discards file changes.
func (a *App) GitDiscard(repoPath string, paths []string) error {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()
	return a.gitEngine.Discard(ctx, repoPath, paths)
}

// GitCheckIgnored returns which of the given paths match gitignore rules.
func (a *App) GitCheckIgnored(repoPath string, paths []string) ([]string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(15 * time.Second)
	defer cancel()
	return a.gitEngine.CheckIgnored(ctx, repoPath, paths)
}

// GetGitConflictStageContent returns a conflicted file's content at a merge
// stage: 1 = common ancestor, 2 = ours, 3 = theirs.
func (a *App) GetGitConflictStageContent(repoPath string, path string, stage int) (string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(15 * time.Second)
	defer cancel()
	return a.gitEngine.GetConflictStageContent(ctx, repoPath, path, stage)
}

// GitResolveConflict resolves a conflicted file. action: "ours", "theirs", or
// "mark" (stage the current working-tree content).
func (a *App) GitResolveConflict(repoPath string, path string, action string) error {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(15 * time.Second)
	defer cancel()
	return a.gitEngine.ResolveConflict(ctx, repoPath, path, action)
}

// GitCommit commits staged changes.
func (a *App) GitCommit(repoPath string, message string) error {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()
	return a.gitEngine.Commit(ctx, repoPath, message)
}

// GitPush pushes commits to remote.
func (a *App) GitPush(repoPath string) error {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(60 * time.Second)
	defer cancel()
	return a.gitEngine.Push(ctx, repoPath)
}

// GitFetch updates remote-tracking branches from the default remote.
func (a *App) GitFetch(repoPath string) (string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(60 * time.Second)
	defer cancel()
	return a.gitEngine.Fetch(ctx, repoPath)
}

// GitPull pulls updates from the remote tracking branch into the current branch.
func (a *App) GitPull(repoPath string) (string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(60 * time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "pull")
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git pull failed: %s", strings.TrimSpace(string(out)))
	}
	a.emitEvent("git:changed", repoPath)
	return strings.TrimSpace(string(out)), nil
}

// GitMerge merges the given source commit/branch into the current branch.
func (a *App) GitMerge(repoPath string, source string, noFF bool, squash bool) (string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()
	return a.gitEngine.Merge(ctx, repoPath, source, noFF, squash)
}

// GenerateAICommitMessage generates a commit message using AI from staged diff with targeted provider/model.
func (a *App) GenerateAICommitMessage(repoPath string, providerID string, model string, instruction string) (string, error) {
	gen := &commitmsg.Generator{Git: a.gitEngine, LLM: a.llmClient}
	ctx, cancel := a.gitContext(45 * time.Second)
	defer cancel()
	return gen.Generate(ctx, a.resolveGitRepoPath(repoPath), providerID, model, instruction)
}

// ── Pull Requests / Merge Requests Management ──────────────────────────────

// PullRequestAuthor represents an author or reviewer on GitHub / GitLab.
type PullRequestAuthor struct {
	Login string `json:"login"`
	Name  string `json:"name"`
}

// PullRequestFile represents a changed file within a pull request.
type PullRequestFile struct {
	Path       string `json:"path"`
	Additions  int    `json:"additions"`
	Deletions  int    `json:"deletions"`
	ChangeType string `json:"changeType"`
}

// PullRequestComment represents a discussion comment on a pull request.
type PullRequestComment struct {
	ID        string            `json:"id"`
	Author    PullRequestAuthor `json:"author"`
	Body      string            `json:"body"`
	CreatedAt string            `json:"createdAt"`
}

// PullRequestReview represents a formal review submitted for a pull request.
type PullRequestReview struct {
	ID          string            `json:"id"`
	Author      PullRequestAuthor `json:"author"`
	State       string            `json:"state"` // APPROVED, CHANGES_REQUESTED, COMMENTED, PENDING
	Body        string            `json:"body"`
	SubmittedAt string            `json:"submittedAt"`
}

// PullRequestItem represents a pull request summary in a list.
type PullRequestItem struct {
	Number        int                 `json:"number"`
	Title         string              `json:"title"`
	State         string              `json:"state"` // OPEN, CLOSED, MERGED
	Author        PullRequestAuthor   `json:"author"`
	HeadRefName   string              `json:"headRefName"`
	BaseRefName   string              `json:"baseRefName"`
	CreatedAt     string              `json:"createdAt"`
	UpdatedAt     string              `json:"updatedAt"`
	IsDraft       bool                `json:"isDraft"`
	URL           string              `json:"url"`
	Additions     int                 `json:"additions"`
	Deletions     int                 `json:"deletions"`
	ChangedFiles  int                 `json:"changedFiles"`
	CommentsCount int                 `json:"commentsCount"`
	ReviewsCount  int                 `json:"reviewsCount"`
	LatestReviews []PullRequestReview `json:"latestReviews,omitempty"`
}

// PullRequestDetail represents detailed information for a single pull request.
type PullRequestDetail struct {
	Number       int                  `json:"number"`
	Title        string               `json:"title"`
	State        string               `json:"state"`
	Body         string               `json:"body"`
	Author       PullRequestAuthor    `json:"author"`
	HeadRefName  string               `json:"headRefName"`
	BaseRefName  string               `json:"baseRefName"`
	CreatedAt    string               `json:"createdAt"`
	UpdatedAt    string               `json:"updatedAt"`
	IsDraft      bool                 `json:"isDraft"`
	URL          string               `json:"url"`
	Additions    int                  `json:"additions"`
	Deletions    int                  `json:"deletions"`
	ChangedFiles int                  `json:"changedFiles"`
	Mergeable    string               `json:"mergeable"`
	Files        []PullRequestFile    `json:"files"`
	Comments     []PullRequestComment `json:"comments"`
	Reviews      []PullRequestReview  `json:"reviews"`
}

func runGhCommand(ctx context.Context, repoPath string, args ...string) ([]byte, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, fmt.Errorf("GitHub CLI (gh) is not installed or not found in PATH. Install gh or authenticate with 'gh auth login' to manage pull requests")
	}

	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GH_NO_UPDATE_NOTIFIER=1", "CLICOLOR=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errStr := strings.TrimSpace(stderr.String())
		if errStr == "" {
			errStr = strings.TrimSpace(stdout.String())
		}
		if errStr == "" {
			errStr = err.Error()
		}
		return nil, fmt.Errorf("%s", errStr)
	}
	return stdout.Bytes(), nil
}

// GetPullRequests lists pull requests for the repository with optional state filter ("open", "closed", "merged", "all").
func (a *App) GetPullRequests(repoPath string, state string) ([]PullRequestItem, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(45 * time.Second)
	defer cancel()

	state = strings.ToLower(strings.TrimSpace(state))
	if state == "" {
		state = "all"
	}
	fields := "number,title,state,author,headRefName,baseRefName,createdAt,updatedAt,isDraft,url,comments,reviews,additions,deletions,changedFiles"
	out, err := runGhCommand(ctx, repoPath, "pr", "list", "--state", state, "--limit", "50", "--json", fields)
	if err != nil {
		return nil, err
	}

	var rawItems []struct {
		Number       int                  `json:"number"`
		Title        string               `json:"title"`
		State        string               `json:"state"`
		Author       PullRequestAuthor    `json:"author"`
		HeadRefName  string               `json:"headRefName"`
		BaseRefName  string               `json:"baseRefName"`
		CreatedAt    string               `json:"createdAt"`
		UpdatedAt    string               `json:"updatedAt"`
		IsDraft      bool                 `json:"isDraft"`
		URL          string               `json:"url"`
		Additions    int                  `json:"additions"`
		Deletions    int                  `json:"deletions"`
		ChangedFiles int                  `json:"changedFiles"`
		Comments     []PullRequestComment `json:"comments"`
		Reviews      []PullRequestReview  `json:"reviews"`
	}

	if err := json.Unmarshal(out, &rawItems); err != nil {
		return nil, fmt.Errorf("failed to parse pull requests: %w", err)
	}

	items := make([]PullRequestItem, len(rawItems))
	for i, r := range rawItems {
		items[i] = PullRequestItem{
			Number:        r.Number,
			Title:         r.Title,
			State:         r.State,
			Author:        r.Author,
			HeadRefName:   r.HeadRefName,
			BaseRefName:   r.BaseRefName,
			CreatedAt:     r.CreatedAt,
			UpdatedAt:     r.UpdatedAt,
			IsDraft:       r.IsDraft,
			URL:           r.URL,
			Additions:     r.Additions,
			Deletions:     r.Deletions,
			ChangedFiles:  r.ChangedFiles,
			CommentsCount: len(r.Comments),
			ReviewsCount:  len(r.Reviews),
			LatestReviews: r.Reviews,
		}
	}
	return items, nil
}

// GetPullRequestDetail returns full metadata, body, comments, reviews, and changed files for a PR.
func (a *App) GetPullRequestDetail(repoPath string, prNumber int) (*PullRequestDetail, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(45 * time.Second)
	defer cancel()

	fields := "number,title,state,body,author,headRefName,baseRefName,createdAt,updatedAt,isDraft,url,comments,reviews,files,additions,deletions,changedFiles,mergeable"
	out, err := runGhCommand(ctx, repoPath, "pr", "view", fmt.Sprintf("%d", prNumber), "--json", fields)
	if err != nil {
		return nil, err
	}

	var detail PullRequestDetail
	if err := json.Unmarshal(out, &detail); err != nil {
		return nil, fmt.Errorf("failed to parse PR detail: %w", err)
	}
	return &detail, nil
}

// GetPullRequestDiff returns the raw unified diff for a pull request.
func (a *App) GetPullRequestDiff(repoPath string, prNumber int) (string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(45 * time.Second)
	defer cancel()

	out, err := runGhCommand(ctx, repoPath, "pr", "diff", fmt.Sprintf("%d", prNumber))
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// CreatePullRequestComment posts a general comment on a pull request.
func (a *App) CreatePullRequestComment(repoPath string, prNumber int, body string) error {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()

	body = strings.TrimSpace(body)
	if body == "" {
		return fmt.Errorf("comment body cannot be empty")
	}
	_, err := runGhCommand(ctx, repoPath, "pr", "comment", fmt.Sprintf("%d", prNumber), "--body", body)
	return err
}

// ReviewPullRequest submits an approval, request changes, or review comment for a pull request.
func (a *App) ReviewPullRequest(repoPath string, prNumber int, action string, body string) error {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(45 * time.Second)
	defer cancel()

	args := []string{"pr", "review", fmt.Sprintf("%d", prNumber)}
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "approve":
		args = append(args, "--approve")
		if strings.TrimSpace(body) != "" {
			args = append(args, "-b", body)
		}
	case "request-changes", "request_changes":
		args = append(args, "--request-changes")
		if strings.TrimSpace(body) != "" {
			args = append(args, "-b", body)
		} else {
			args = append(args, "-b", "Changes requested.")
		}
	default:
		args = append(args, "--comment")
		if strings.TrimSpace(body) != "" {
			args = append(args, "-b", body)
		} else {
			args = append(args, "-b", "Review comment.")
		}
	}
	_, err := runGhCommand(ctx, repoPath, args...)
	return err
}

// MergePullRequest merges the pull request with the chosen strategy ("merge", "squash", "rebase").
func (a *App) MergePullRequest(repoPath string, prNumber int, method string) (string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(60 * time.Second)
	defer cancel()

	args := []string{"pr", "merge", fmt.Sprintf("%d", prNumber)}
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "squash":
		args = append(args, "--squash")
	case "rebase":
		args = append(args, "--rebase")
	default:
		args = append(args, "--merge")
	}
	out, err := runGhCommand(ctx, repoPath, args...)
	a.emitEvent("git:changed", repoPath)
	return string(out), err
}

// ClosePullRequest closes an open pull request.
func (a *App) ClosePullRequest(repoPath string, prNumber int) error {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()

	_, err := runGhCommand(ctx, repoPath, "pr", "close", fmt.Sprintf("%d", prNumber))
	return err
}

// ReopenPullRequest reopens a closed pull request.
func (a *App) ReopenPullRequest(repoPath string, prNumber int) error {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(30 * time.Second)
	defer cancel()

	_, err := runGhCommand(ctx, repoPath, "pr", "reopen", fmt.Sprintf("%d", prNumber))
	return err
}

// CheckoutPullRequest checks out the pull request's branch locally.
func (a *App) CheckoutPullRequest(repoPath string, prNumber int) (string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(60 * time.Second)
	defer cancel()

	out, err := runGhCommand(ctx, repoPath, "pr", "checkout", fmt.Sprintf("%d", prNumber))
	a.emitEvent("git:changed", repoPath)
	return string(out), err
}

// CreatePullRequest opens a new pull request on the repository remote.
func (a *App) CreatePullRequest(repoPath string, title string, body string, base string, head string, draft bool) (*PullRequestItem, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	ctx, cancel := a.gitContext(60 * time.Second)
	defer cancel()

	title = strings.TrimSpace(title)
	if title == "" {
		return nil, fmt.Errorf("pull request title is required")
	}

	args := []string{"pr", "create", "--title", title, "--body", body}
	if strings.TrimSpace(base) != "" {
		args = append(args, "--base", strings.TrimSpace(base))
	}
	if strings.TrimSpace(head) != "" {
		args = append(args, "--head", strings.TrimSpace(head))
	}
	if draft {
		args = append(args, "--draft")
	}

	out, err := runGhCommand(ctx, repoPath, args...)
	if err != nil {
		return nil, err
	}

	urlStr := strings.TrimSpace(string(out))
	a.emitEvent("git:changed", repoPath)
	return &PullRequestItem{
		Title: title,
		URL:   urlStr,
		State: "OPEN",
	}, nil
}
