package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// WorkspaceRuleFile describes a single detected rule or instruction file.
type WorkspaceRuleFile struct {
	Path      string `json:"path"`
	Format    string `json:"format"`
	Exists    bool   `json:"exists"`
	Content   string `json:"content"`
	UpdatedAt int64  `json:"updatedAt"`
}

// WorkspaceRulesResult contains all detected rule files and a unified prompt string.
type WorkspaceRulesResult struct {
	ActiveRules     []WorkspaceRuleFile `json:"activeRules"`
	EffectivePrompt string              `json:"effectivePrompt"`
}

var knownRuleFiles = []struct {
	path   string
	format string
}{
	{"AGENTS.md", "agents_md"},
	{".cursorrules", "cursorrules"},
	{"CLAUDE.md", "claude_md"},
	{".forge/rules.md", "forge_rules"},
	{".github/copilot-instructions.md", "copilot_instructions"},
}

// GetWorkspaceRules inspects the workspace root for agent rule files and builds a unified instruction set.
func GetWorkspaceRules(workspacePath string) (*WorkspaceRulesResult, error) {
	if workspacePath == "" {
		return &WorkspaceRulesResult{}, nil
	}

	result := &WorkspaceRulesResult{}
	var effectiveParts []string

	for _, k := range knownRuleFiles {
		fullPath := filepath.Join(workspacePath, k.path)
		info, err := os.Stat(fullPath)
		if err == nil && !info.IsDir() {
			data, readErr := os.ReadFile(fullPath)
			if readErr == nil {
				content := string(data)
				rule := WorkspaceRuleFile{
					Path:      k.path,
					Format:    k.format,
					Exists:    true,
					Content:   content,
					UpdatedAt: info.ModTime().UnixMilli(),
				}
				result.ActiveRules = append(result.ActiveRules, rule)
				trimmed := strings.TrimSpace(content)
				if trimmed != "" {
					effectiveParts = append(effectiveParts, fmt.Sprintf("### Project Rule: %s\n%s", k.path, trimmed))
				}
			}
		} else {
			result.ActiveRules = append(result.ActiveRules, WorkspaceRuleFile{
				Path:   k.path,
				Format: k.format,
				Exists: false,
			})
		}
	}

	result.EffectivePrompt = strings.Join(effectiveParts, "\n\n")
	return result, nil
}

// SaveWorkspaceRule updates or creates a specific rule file in the workspace.
func SaveWorkspaceRule(workspacePath string, rulePath string, content string) error {
	if workspacePath == "" || rulePath == "" {
		return fmt.Errorf("workspacePath and rulePath cannot be empty")
	}
	// Sanitize rulePath to avoid directory traversal
	cleanRel := filepath.Clean(rulePath)
	if strings.HasPrefix(cleanRel, "..") {
		return fmt.Errorf("invalid rule path: %s", rulePath)
	}
	target := filepath.Join(workspacePath, cleanRel)
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	return os.WriteFile(target, []byte(content), 0644)
}

// SyncWorkspaceRule copies/converts instructions from one format/file to another.
func SyncWorkspaceRule(workspacePath string, sourcePath string, targetPath string) error {
	rules, err := GetWorkspaceRules(workspacePath)
	if err != nil {
		return err
	}
	var srcContent string
	for _, r := range rules.ActiveRules {
		if r.Path == sourcePath && r.Exists {
			srcContent = r.Content
			break
		}
	}
	if srcContent == "" {
		return fmt.Errorf("source rule file %q does not exist or is empty", sourcePath)
	}

	header := fmt.Sprintf("# Synced from %s on %s\n\n", sourcePath, time.Now().Format(time.RFC3339))
	return SaveWorkspaceRule(workspacePath, targetPath, header+srcContent)
}
