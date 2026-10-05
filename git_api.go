package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hasdev/forge-ade/internal/git"
	"github.com/hasdev/forge-ade/internal/llm"
)

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
	return a.gitEngine.GetCommitGraph(a.ctx, repoPath, offset, limit, branch)
}

// GetGitBranches lists local branch names for the repo.
func (a *App) GetGitBranches(repoPath string) ([]string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	return a.gitEngine.GetBranches(a.ctx, repoPath)
}

// GitCheckout switches the working tree to the given branch.
func (a *App) GitCheckout(repoPath string, branch string) error {
	repoPath = a.resolveGitRepoPath(repoPath)
	if branch == "" {
		return fmt.Errorf("branch name is required")
	}
	cmd := exec.CommandContext(a.ctx, "git", "-C", repoPath, "checkout", branch)
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
	cmd := exec.CommandContext(a.ctx, "git", "-C", repoPath, "checkout", "-b", name)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("create branch %s failed: %s", name, strings.TrimSpace(string(out)))
	}
	a.emitEvent("git:changed", repoPath)
	return nil
}

// GetGitCommitDiff returns details and patch for a single commit.
func (a *App) GetGitCommitDiff(repoPath string, hash string) (string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	return a.gitEngine.GetCommitDiff(a.ctx, repoPath, hash)
}

// GetGitCommitBody returns the full commit message (subject + body) for a commit.
func (a *App) GetGitCommitBody(repoPath string, hash string) (string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	return a.gitEngine.GetCommitBody(a.ctx, repoPath, hash)
}

// GetGitFileDiff returns the unified diff of a single working-tree file against HEAD.
func (a *App) GetGitFileDiff(repoPath string, path string) (string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	return a.gitEngine.GetFileDiff(a.ctx, repoPath, path)
}

// GetGitCommitFileDiff returns the unified diff of a single file within a commit.
func (a *App) GetGitCommitFileDiff(repoPath string, hash string, path string) (string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	return a.gitEngine.GetCommitFileDiff(a.ctx, repoPath, hash, path)
}

// GetGitFileDiffHunks returns the structured hunks of a file's diff against HEAD.
func (a *App) GetGitFileDiffHunks(repoPath string, path string) ([]git.DiffHunk, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	return a.gitEngine.GetFileDiffHunks(a.ctx, repoPath, path)
}

// RevertGitHunk reverse-applies a single diff hunk, restoring it to HEAD state.
func (a *App) RevertGitHunk(repoPath string, path string, hunkIndex int) error {
	repoPath = a.resolveGitRepoPath(repoPath)
	return a.gitEngine.RevertDiffHunk(a.ctx, repoPath, path, hunkIndex)
}

// GetGitFileContentAtCommit returns the raw file content at a given commit.
func (a *App) GetGitFileContentAtCommit(repoPath string, hash string, path string) (string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	return a.gitEngine.GetFileContentAtCommit(a.ctx, repoPath, hash, path)
}

// GetGitStatus returns lightweight git status (staged, unstaged, untracked).
func (a *App) GetGitStatus(repoPath string) (*git.GitStatusResult, error) {
	a.sweepGitDirty()
	repoPath = a.resolveGitRepoPath(repoPath)
	return a.gitEngine.GetStatus(a.ctx, repoPath)
}

// GitStage stages files.
func (a *App) GitStage(repoPath string, paths []string) error {
	repoPath = a.resolveGitRepoPath(repoPath)
	return a.gitEngine.Stage(a.ctx, repoPath, paths)
}

// GitUnstage unstages files.
func (a *App) GitUnstage(repoPath string, paths []string) error {
	repoPath = a.resolveGitRepoPath(repoPath)
	return a.gitEngine.Unstage(a.ctx, repoPath, paths)
}

// GitDiscard discards file changes.
func (a *App) GitDiscard(repoPath string, paths []string) error {
	repoPath = a.resolveGitRepoPath(repoPath)
	return a.gitEngine.Discard(a.ctx, repoPath, paths)
}

// GitCheckIgnored returns which of the given paths match gitignore rules.
func (a *App) GitCheckIgnored(repoPath string, paths []string) ([]string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	return a.gitEngine.CheckIgnored(a.ctx, repoPath, paths)
}

// GetGitConflictStageContent returns a conflicted file's content at a merge
// stage: 1 = common ancestor, 2 = ours, 3 = theirs.
func (a *App) GetGitConflictStageContent(repoPath string, path string, stage int) (string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	return a.gitEngine.GetConflictStageContent(a.ctx, repoPath, path, stage)
}

// GitResolveConflict resolves a conflicted file. action: "ours", "theirs", or
// "mark" (stage the current working-tree content).
func (a *App) GitResolveConflict(repoPath string, path string, action string) error {
	repoPath = a.resolveGitRepoPath(repoPath)
	return a.gitEngine.ResolveConflict(a.ctx, repoPath, path, action)
}

// GitCommit commits staged changes.
func (a *App) GitCommit(repoPath string, message string) error {
	repoPath = a.resolveGitRepoPath(repoPath)
	return a.gitEngine.Commit(a.ctx, repoPath, message)
}

// GitPush pushes commits to remote.
func (a *App) GitPush(repoPath string) error {
	repoPath = a.resolveGitRepoPath(repoPath)
	return a.gitEngine.Push(a.ctx, repoPath)
}

// GitFetch updates remote-tracking branches from the default remote.
func (a *App) GitFetch(repoPath string) (string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	return a.gitEngine.Fetch(a.ctx, repoPath)
}

// GitMerge merges the given source commit/branch into the current branch.
func (a *App) GitMerge(repoPath string, source string, noFF bool, squash bool) (string, error) {
	repoPath = a.resolveGitRepoPath(repoPath)
	return a.gitEngine.Merge(a.ctx, repoPath, source, noFF, squash)
}

// GenerateAICommitMessage generates a commit message using AI from staged diff with targeted provider/model.
func (a *App) GenerateAICommitMessage(repoPath string, providerID string, model string, instruction string) (string, error) {
	log.Printf("[ai-commit] start: repo=%q provider=%q model=%q instruction=%q", repoPath, providerID, model, instruction)
	repoPath = a.resolveGitRepoPath(repoPath)

	// Prefer the staged diff, but fall back to the full working tree (vs HEAD)
	// so the ✨ button works before anything is staged — the common case when
	// reviewing changes straight from the agent.
	diff, diffStat, diffErr := func() (string, string, error) {
		staged, err := a.gitEngine.GetStagedDiff(a.ctx, repoPath)
		if err == nil && strings.TrimSpace(staged) != "" {
			stat, _ := a.gitEngine.GetStagedDiffStat(a.ctx, repoPath)
			log.Printf("[ai-commit] using staged diff_len=%d", len(staged))
			return staged, stat, nil
		}
		work, err := a.gitEngine.GetWorkingTreeDiff(a.ctx, repoPath)
		if err == nil && strings.TrimSpace(work) != "" {
			stat, _ := a.gitEngine.GetWorkingTreeDiffStat(a.ctx, repoPath)
			log.Printf("[ai-commit] nothing staged — using working-tree diff_len=%d", len(work))
			return work, stat, nil
		}
		return "", "", fmt.Errorf("no changes found to summarize. Stage files (+) or make working-tree changes first")
	}()
	if diffErr != nil {
		log.Printf("[ai-commit] no diff available: %v", diffErr)
		return "", diffErr
	}
	log.Printf("[ai-commit] diff_len=%d (truncated mode=%v)", len(diff), len(diff) > 4000)

	var promptContent string
	if len(diff) > 4000 {
		// Token efficient mode for large diffs / many files: send diffstat + first 2000 chars of diff
		truncatedDiff := diff
		if len(truncatedDiff) > 2000 {
			truncatedDiff = truncatedDiff[:2000] + "\n...[staged diff truncated for token efficiency]"
		}
		promptContent = fmt.Sprintf("Changes summary of changed files:\n%s\n\nPartial diff sample:\n%s", diffStat, truncatedDiff)
	} else {
		// Small changes: send full details
		promptContent = fmt.Sprintf("Changes summary:\n%s\n\nDiff:\n%s", diffStat, diff)
	}

	messages := []llm.LLMMessage{
		{
			Role:    llm.RoleSystem,
			Content: "CRITICAL: You are a Git commit message generator. Your output MUST be ONLY a concise 1 to 2 line Git commit message following conventional commits format (e.g., 'docs(readme): rewrite architecture guide and update tech stack'). DO NOT include any analysis, section headings, Markdown tables, or explanations. ONLY output the raw commit message text.",
		},
		{
			Role:    llm.RoleUser,
			Content: promptContent,
		},
	}

	// Optional user instruction appended as a follow-up so it overrides the default style.
	if strings.TrimSpace(instruction) != "" {
		messages = append(messages, llm.LLMMessage{
			Role:    llm.RoleUser,
			Content: "Additional instruction for the commit message: " + strings.TrimSpace(instruction),
		})
	}

	var resp *llm.LLMResponse
	var err error
	if providerID != "" {
		log.Printf("[ai-commit] calling ChatWithProviderStream provider=%q model=%q", providerID, model)
		// Use the streaming path even for AI commit: some providers/proxies
		// (e.g. kilo-auto/free) only answer streaming requests and return an
		// empty 200 body to non-streaming calls.
		resp, err = a.llmClient.ChatWithProviderStream(a.ctx, providerID, model, messages, nil, nil, nil)
	} else {
		cfg := a.llmClient.GetConfig()
		log.Printf("[ai-commit] calling ChatWithStream (active provider=%q model=%q)", cfg.ProviderID, cfg.Model)
		resp, err = a.llmClient.ChatWithStream(a.ctx, messages, nil, nil)
	}
	if err != nil {
		log.Printf("[ai-commit] LLM call error: %v", err)
		return "", fmt.Errorf("AI commit generation failed: %w", err)
	}
	log.Printf("[ai-commit] LLM response: content_len=%d reasoning_len=%d tokens=%+v", len(resp.Content), len(resp.Reasoning), resp.TokenUsage)

	result := strings.TrimSpace(resp.Content)
	if result == "" {
		log.Printf("[ai-commit] WARNING: empty content after trim (reasoning only?)")
		return "", fmt.Errorf("AI commit generation returned an empty response. The provider may have failed silently — check the log")
	}

	// Clean up any extra Markdown codeblock fences if present
	result = strings.TrimPrefix(result, "```markdown")
	result = strings.TrimPrefix(result, "```git")
	result = strings.TrimPrefix(result, "```")
	result = strings.TrimSuffix(result, "```")
	result = strings.TrimSpace(result)

	// Filter out any leftover Markdown headers or analysis tables
	lines := strings.Split(result, "\n")
	var cleanLines []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "###") || strings.HasPrefix(trimmed, "|") || strings.HasPrefix(trimmed, "---") || strings.HasPrefix(trimmed, "**") {
			continue
		}
		if trimmed != "" {
			cleanLines = append(cleanLines, trimmed)
		}
		if len(cleanLines) >= 2 {
			break
		}
	}

	if len(cleanLines) > 0 {
		result = strings.Join(cleanLines, "\n")
	}

	return result, nil
}
