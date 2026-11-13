package agent

// prompt_history.go ports the webview's promptHistoryStore: the last 100
// submitted prompts, browsable with ↑/↓ at the composer (only at the caret
// boundaries, which for the native single-line flow means while the text has
// no line breaks), persisted to <dataDir>/prompt-history.json.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/hasdev/forge-ade/app/native/pkg/engine"
)

const promptHistoryMax = 100

// LoadPromptHistory reads the persisted prompt history.
func (as *AgentState) LoadPromptHistory() {
	path, err := promptHistoryPath(as.engine)
	if err != nil {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var history []string
	if err := json.Unmarshal(data, &history); err != nil || len(history) == 0 {
		return
	}
	as.mu.Lock()
	as.PromptHistory = history
	as.PromptHistoryIdx = -1
	as.mu.Unlock()
}

// AppendPromptHistory records a submitted prompt (deduped against the most
// recent entry, like the webview store) and persists the list.
func (as *AgentState) AppendPromptHistory(prompt string) {
	prompt = trimHistoryEntry(prompt)
	if prompt == "" {
		return
	}
	as.mu.Lock()
	if len(as.PromptHistory) > 0 && as.PromptHistory[len(as.PromptHistory)-1] == prompt {
		as.mu.Unlock()
		return
	}
	as.PromptHistory = append(as.PromptHistory, prompt)
	if len(as.PromptHistory) > promptHistoryMax {
		as.PromptHistory = as.PromptHistory[len(as.PromptHistory)-promptHistoryMax:]
	}
	as.PromptHistoryIdx = -1
	as.PromptHistoryDraft = ""
	history := append([]string(nil), as.PromptHistory...)
	as.mu.Unlock()

	if path, err := promptHistoryPath(as.engine); err == nil {
		_ = os.MkdirAll(filepath.Dir(path), 0755)
		if data, err := json.Marshal(history); err == nil {
			_ = os.WriteFile(path, data, 0644)
		}
	}
}

// PromptHistoryStep moves through history (delta -1 = older via ↑, +1 =
// newer via ↓) and returns the composer text to show. Entering history
// stashes the in-progress text; leaving it restores the stash.
func (as *AgentState) PromptHistoryStep(delta int, currentText string) (string, bool) {
	as.mu.Lock()
	defer as.mu.Unlock()
	if len(as.PromptHistory) == 0 {
		return "", false
	}
	if as.PromptHistoryIdx == -1 {
		if delta > 0 {
			return "", false // already at the live text
		}
		as.PromptHistoryDraft = currentText
		as.PromptHistoryIdx = len(as.PromptHistory) - 1
		return as.PromptHistory[as.PromptHistoryIdx], true
	}
	idx := as.PromptHistoryIdx + delta
	if idx >= len(as.PromptHistory) {
		// Past the newest entry: restore the in-progress draft.
		as.PromptHistoryIdx = -1
		return as.PromptHistoryDraft, true
	}
	if idx < 0 {
		idx = 0
	}
	as.PromptHistoryIdx = idx
	return as.PromptHistory[idx], true
}

func trimHistoryEntry(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == '\n' || s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == '\n' || s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

func promptHistoryPath(e *engine.Engine) (string, error) {
	if e == nil {
		return "", errors.New("engine unavailable")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := e.DataDir
	if dir == "" {
		dir = filepath.Join(home, ".forge-ade")
	}
	return filepath.Join(dir, "prompt-history.json"), nil
}
