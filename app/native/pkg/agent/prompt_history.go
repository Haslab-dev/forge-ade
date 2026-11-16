package agent

// prompt_history.go provides session-based prompt history navigation for the composer.
// When the user presses ↑ or ↓ while the composer is on a single line, it browses
// through the prompts previously submitted in the active session (derived from
// user messages in session.Messages).
//
// No global prompt-history.json file is needed; each session naturally retains
// its own history persisted in its session JSON file.

import (
	"strings"
)

// LoadPromptHistory resets the browsing state. Kept for lifecycle compatibility.
func (as *AgentState) LoadPromptHistory() {
	as.mu.Lock()
	as.PromptHistoryIdx = -1
	as.PromptHistoryDraft = ""
	as.mu.Unlock()
}

// AppendPromptHistory resets the browsing state after message submission.
// The submitted prompt is naturally stored inside session.Messages.
func (as *AgentState) AppendPromptHistory(_ string) {
	as.mu.Lock()
	as.PromptHistoryIdx = -1
	as.PromptHistoryDraft = ""
	as.mu.Unlock()
}

// activeSessionUserPromptsLocked extracts user prompts from the currently active session.
// Adjacent identical prompts are deduplicated.
func (as *AgentState) activeSessionUserPromptsLocked() []string {
	if as.ActiveSessionID == "" {
		return nil
	}
	var prompts []string
	for _, s := range as.Sessions {
		if s.ID == as.ActiveSessionID {
			for _, m := range s.Messages {
				if m != nil && m.Role == "user" {
					trimmed := trimHistoryEntry(m.Text)
					if trimmed != "" {
						if len(prompts) == 0 || prompts[len(prompts)-1] != trimmed {
							prompts = append(prompts, trimmed)
						}
					}
				}
			}
			break
		}
	}
	return prompts
}

// PromptHistoryStep moves through session prompt history (delta -1 = older via ↑, +1 =
// newer via ↓) and returns the composer text to show. Entering history
// stashes the in-progress draft; leaving it restores the stash.
func (as *AgentState) PromptHistoryStep(delta int, currentText string) (string, bool) {
	as.mu.Lock()
	defer as.mu.Unlock()

	prompts := as.activeSessionUserPromptsLocked()
	if len(prompts) == 0 {
		return "", false
	}

	if as.PromptHistoryIdx == -1 {
		if delta > 0 {
			return "", false // already at the live text
		}
		as.PromptHistoryDraft = currentText
		as.PromptHistoryIdx = len(prompts) - 1
		return prompts[as.PromptHistoryIdx], true
	}

	idx := as.PromptHistoryIdx + delta
	if idx >= len(prompts) {
		// Past newest entry: restore the in-progress draft.
		as.PromptHistoryIdx = -1
		draft := as.PromptHistoryDraft
		as.PromptHistoryDraft = ""
		return draft, true
	}
	if idx < 0 {
		idx = 0
	}
	as.PromptHistoryIdx = idx
	return prompts[idx], true
}

func trimHistoryEntry(s string) string {
	return strings.TrimSpace(s)
}
