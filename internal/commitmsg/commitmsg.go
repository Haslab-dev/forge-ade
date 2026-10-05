// Package commitmsg generates AI git commit messages from staged or
// working-tree diffs, with prompt construction and response scrubbing kept
// out of the binding layer.
package commitmsg

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/hasdev/forge-ade/internal/git"
	"github.com/hasdev/forge-ade/internal/llm"
)

// Generator produces commit messages from repository diffs.
type Generator struct {
	Git *git.Engine
	LLM *llm.LLMClient
}

// Generate summarizes the repo's staged diff (or, when nothing is staged,
// the working-tree diff vs HEAD) into a 1–2 line conventional commit message.
func (g *Generator) Generate(ctx context.Context, repoPath string, providerID string, model string, instruction string) (string, error) {
	diff, diffStat, err := g.collectDiff(ctx, repoPath)
	if err != nil {
		return "", err
	}

	promptContent := buildPrompt(diff, diffStat)

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

	resp, err := g.chat(ctx, providerID, model, messages)
	if err != nil {
		return "", fmt.Errorf("AI commit generation failed: %w", err)
	}

	result := strings.TrimSpace(resp.Content)
	if result == "" {
		return "", fmt.Errorf("AI commit generation returned an empty response. The provider may have failed silently — check the log")
	}
	return scrub(result), nil
}

// collectDiff prefers the staged diff, but falls back to the full working
// tree (vs HEAD) so the ✨ button works before anything is staged — the
// common case when reviewing changes straight from the agent.
func (g *Generator) collectDiff(ctx context.Context, repoPath string) (string, string, error) {
	staged, err := g.Git.GetStagedDiff(ctx, repoPath)
	if err == nil && strings.TrimSpace(staged) != "" {
		stat, _ := g.Git.GetStagedDiffStat(ctx, repoPath)
		log.Printf("[ai-commit] using staged diff_len=%d", len(staged))
		return staged, stat, nil
	}
	work, err := g.Git.GetWorkingTreeDiff(ctx, repoPath)
	if err == nil && strings.TrimSpace(work) != "" {
		stat, _ := g.Git.GetWorkingTreeDiffStat(ctx, repoPath)
		log.Printf("[ai-commit] nothing staged — using working-tree diff_len=%d", len(work))
		return work, stat, nil
	}
	return "", "", fmt.Errorf("no changes found to summarize. Stage files (+) or make working-tree changes first")
}

// buildPrompt packs the diff into the prompt, switching to a token-efficient
// diffstat + sample form for large diffs.
func buildPrompt(diff, diffStat string) string {
	if len(diff) <= 4000 {
		return fmt.Sprintf("Changes summary:\n%s\n\nDiff:\n%s", diffStat, diff)
	}
	truncatedDiff := diff
	if len(truncatedDiff) > 2000 {
		truncatedDiff = truncatedDiff[:2000] + "\n...[staged diff truncated for token efficiency]"
	}
	return fmt.Sprintf("Changes summary of changed files:\n%s\n\nPartial diff sample:\n%s", diffStat, truncatedDiff)
}

func (g *Generator) chat(ctx context.Context, providerID string, model string, messages []llm.LLMMessage) (*llm.LLMResponse, error) {
	if providerID != "" {
		// Use the streaming path even for AI commit: some providers/proxies
		// (e.g. kilo-auto/free) only answer streaming requests and return an
		// empty 200 body to non-streaming calls.
		return g.LLM.ChatWithProviderStream(ctx, providerID, model, messages, nil, nil, nil)
	}
	return g.LLM.ChatWithStream(ctx, messages, nil, nil)
}

// scrub strips Markdown code fences, headers, and table rows, keeping at
// most the first two non-empty lines of the message.
func scrub(result string) string {
	result = strings.TrimPrefix(result, "```markdown")
	result = strings.TrimPrefix(result, "```git")
	result = strings.TrimPrefix(result, "```")
	result = strings.TrimSuffix(result, "```")
	result = strings.TrimSpace(result)

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
		return strings.Join(cleanLines, "\n")
	}
	return result
}
