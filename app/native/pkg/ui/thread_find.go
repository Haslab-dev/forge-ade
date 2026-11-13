package ui

// thread_find.go ports ThreadFindBar (⌘F find-in-thread, useAgentThreadSearch)
// and the left turn scrubber (micro-minimap of turns with hover preview and
// click-to-scroll) from AgentActiveSessionView.tsx.

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
)

// threadFindMatches recomputes the match set: case-insensitive substring
// over each message's text, thinking, tool labels, and tool output.
func threadFindMatches(as *agent.AgentState, query string) []int {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	q := strings.ToLower(query)
	var matches []int
	for i, m := range as.Messages {
		if threadMessageHaystack(m) != "" && strings.Contains(threadMessageHaystack(m), q) {
			matches = append(matches, i)
		}
	}
	return matches
}

func threadMessageHaystack(m *agent.ChatMessage) string {
	var b strings.Builder
	b.WriteString(m.Text)
	b.WriteString("\n")
	b.WriteString(m.Thinking)
	for _, t := range m.Tools {
		b.WriteString(t.ToolName)
		b.WriteString("\n")
		b.WriteString(t.Command)
		b.WriteString("\n")
		b.WriteString(t.Output)
		b.WriteString("\n")
	}
	return strings.ToLower(b.String())
}

// renderThreadFindBar draws the floating find bar docked under the session
// header: query input, "n of m", prev/next, close — ThreadFindBar.tsx parity.
func (a *AppUI) renderThreadFindBar(c *ui.Context, tc theme.ThemeColors, as *agent.AgentState) {
	bar := ui.Row(c.Key("thread-find")).Height(36).Gap(8).Padding(0, leftInsetOf(a, c), 6, leftInsetOf(a, c)).
		AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.6))
	bar.Children(func() {
		searchBox := ui.Row(c).Height(28).Radius(8).Padding(0, 8).Gap(6).AlignItems(ui.Center).
			Background(tc.Surface).BorderWidth(1).BorderColor(tc.Focus.Alpha(0.5)).Grow(1).MaxWidth(360)
		searchBox.Children(func() {
			ui.Icon(c, icons.TablerSearch).Width(13).Height(13).TextColor(tc.TextMuted)
			ui.TextInput(c, &a.ThreadFindQuery).Placeholder("Find in thread…").Grow(1)
		})
		// Recompute matches whenever the query changed (immediate mode).
		if a.ThreadFindQueryAt != a.ThreadFindQuery {
			a.ThreadFindQueryAt = a.ThreadFindQuery
			a.ThreadFindMatches = threadFindMatches(as, a.ThreadFindQuery)
			a.ThreadFindIdx = 0
			if len(a.ThreadFindMatches) > 0 {
				as.ListState.ScrollIntoView(a.ThreadFindMatches[0])
				as.ListState.FollowEnd = false
			}
		}
		total := len(a.ThreadFindMatches)
		if total > 0 {
			ui.Text(c, fmt.Sprintf("%d of %d", a.ThreadFindIdx+1, total)).FontSize(11).TextColor(tc.TextMuted)
		} else if strings.TrimSpace(a.ThreadFindQuery) != "" {
			ui.Text(c, "No results").FontSize(11).TextColor(tc.TextMuted)
		}
		prev := IconButton(c, tc, icons.TablerChevronUp, 13)
		if prev.Clicked() && total > 0 {
			a.ThreadFindIdx = (a.ThreadFindIdx - 1 + total) % total
			as.ListState.ScrollIntoView(a.ThreadFindMatches[a.ThreadFindIdx])
			as.ListState.FollowEnd = false
		}
		next := IconButton(c, tc, icons.TablerChevronDown, 13)
		if next.Clicked() && total > 0 {
			a.ThreadFindIdx = (a.ThreadFindIdx + 1) % total
			as.ListState.ScrollIntoView(a.ThreadFindMatches[a.ThreadFindIdx])
			as.ListState.FollowEnd = false
		}
		closeBtn := IconButton(c, tc, icons.TablerX, 13)
		if closeBtn.Clicked() {
			a.ShowThreadFind = false
			a.ThreadFindQuery = ""
			a.ThreadFindMatches = nil
		}
	})
}

// leftInsetOf resolves the chat lane inset for bars aligned with the list.
func leftInsetOf(a *AppUI, c *ui.Context) float32 {
	inset, _ := a.chatLaneInset(c)
	return inset
}

// renderTurnScrubber draws the left micro-minimap: one tick per user turn,
// active tick highlighted, hover preview, click scrolls to the turn — shown
// only when the thread has ≥2 turns, as forge-ade's scrubber does.
func (a *AppUI) renderTurnScrubber(c *ui.Context, tc theme.ThemeColors, as *agent.AgentState) {
	var turns []int // message indices of user turns
	for i, m := range as.Messages {
		if m.Role == "user" {
			turns = append(turns, i)
		}
	}
	if len(turns) < 2 {
		return
	}

	// Visible window (roughly mirrors the scrubber's viewport highlight).
	first, last := as.ListState.Visible()

	rail := ui.Row(c.Key("turn-scrubber")).Width(14).FillHeight().Padding(6, 4).Justify(ui.Center)
	rail.Children(func() {
		col := ui.Column(c).Gap(6).AlignItems(ui.Center)
		col.Children(func() {
			for ti, msgIdx := range turns {
				ti, msgIdx := ti, msgIdx
				inView := msgIdx >= first && msgIdx <= last
				tick := ui.Box(c.Key(fmt.Sprintf("tick-%d", ti))).Size(8, 4).Radius(2)
				if inView {
					tick.Background(tc.Accent)
				} else {
					tick.Background(tc.Border)
				}
				preview := ""
				if m := as.Messages[msgIdx]; m != nil {
					preview = m.Text
					if len(preview) > 60 {
						preview = preview[:60] + "…"
					}
					preview = strings.ReplaceAll(preview, "\n", " ")
				}
				tick.Tooltip(fmt.Sprintf("Turn %d — %s", ti+1, preview))
				if tick.Clicked() {
					as.ListState.ScrollIntoView(msgIdx)
					as.ListState.FollowEnd = false
				}
			}
		})
	})
}
