package ui

// composer_menu.go ports the composer autocomplete menus from
// AgentTaskInputBar.tsx: the "/" slash-command popup (internal commands, ACP
// agent commands, MCP tools, skills) and the "@" file-mention menu. Menu
// visibility is derived from the composer text each frame (immediate-mode);
// keyboard navigation hooks the textarea's OnTextKey.

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/fuzzy"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
)

// composerSuggestion is one row of the slash or mention menu.
type composerSuggestion struct {
	Name        string // "/goal" or "src/app.ts"
	Description string
	Category    string // "goal","plan","security","mode","mcp","skill","custom","agent","file"
	Insert      string // text inserted on selection (defaults to Name+" ")
}

// categoryColors maps suggestion categories to the token colors of the
// webview's category badges (skill sky / goal emerald / plan purple /
// security amber / mode pink / mcp violet / custom cyan).
func categoryColor(tc theme.ThemeColors, category string) ui.Color {
	switch category {
	case "skill":
		return tc.Info
	case "goal":
		return tc.Success
	case "plan":
		return tc.Accent
	case "security":
		return tc.Warning
	case "mode":
		return tc.Update
	case "mcp":
		return tc.Accent.Alpha(0.9)
	case "custom":
		return tc.Success.Alpha(0.9)
	case "agent":
		return tc.Accent
	default:
		return tc.TextMuted
	}
}

// internalSlashCommands is the fixed catalog of forge-ade built-in commands.
var internalSlashCommands = []composerSuggestion{
	{Name: "/goal", Description: "Set or update this task's objective", Category: "goal"},
	{Name: "/plan", Description: "Switch to plan mode before acting", Category: "plan"},
	{Name: "/compact", Description: "Compact the conversation context", Category: "custom"},
	{Name: "/schedule", Description: "Schedule this prompt to run later", Category: "mode"},
	{Name: "/browser", Description: "Drive the embedded browser", Category: "mode"},
	{Name: "/boost", Description: "Boost this task's model effort", Category: "custom"},
	{Name: "/model", Description: "Choose LLM model and provider", Category: "mode"},
	{Name: "/settings", Description: "Open settings and configuration", Category: "custom"},
	{Name: "/setup", Description: "Configure API keys and providers", Category: "custom"},
	{Name: "/usage", Description: "View token usage, quota and stats", Category: "custom"},
	{Name: "/clear", Description: "Clear active conversation history", Category: "custom"},
}

// slashCatalog merges the internal commands with the active session's ACP
// advertised commands and the MCP tools, exactly as the webview builds it.
func (a *AppUI) slashCatalog(as *agent.AgentState) []composerSuggestion {
	items := append([]composerSuggestion(nil), internalSlashCommands...)

	seen := map[string]bool{}
	for _, it := range items {
		seen[it.Name] = true
	}
	add := func(name, desc, cat string) {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			return
		}
		if !strings.HasPrefix(name, "/") {
			name = "/" + name
		}
		seen[name] = true
		items = append(items, composerSuggestion{Name: name, Description: desc, Category: cat})
	}

	// Commands the connected ACP agent advertises.
	if as.ActiveSession() != nil {
		for _, cmd := range as.ActiveSession().AvailableCommands {
			add(cmd, "Agent command", "agent")
		}
	}

	// MCP tools of configured servers, as the webview menu lists them.
	if a.Engine != nil && a.Engine.MCPMgr != nil {
		for _, srv := range a.Engine.MCPMgr.ListServers() {
			if !srv.Enabled {
				continue
			}
			add("/mcp:"+srv.Name, "MCP server · "+srv.Name, "mcp")
		}
	}

	// Installed skills ($ in forge-ade parlance, reachable as /skill:name).
	if a.Engine != nil && a.Engine.SkillMgr != nil {
		for _, sk := range a.Engine.SkillMgr.List() {
			add("/skill:"+sk.Name, sk.Description, "skill")
		}
	}
	return items
}

// filterSuggestions fuzzy-filters the catalog by query prefix/substring.
func filterSuggestions(items []composerSuggestion, query string) []composerSuggestion {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return items
	}
	var out []composerSuggestion
	for _, it := range items {
		hay := strings.ToLower(it.Name + " " + it.Description)
		if strings.Contains(hay, strings.TrimPrefix(query, "/")) {
			out = append(out, it)
		}
	}
	if len(out) > 12 {
		out = out[:12]
	}
	return out
}

// activeMentionToken returns the trailing "@query" token of the composer
// text, if the caret is still inside it (no whitespace after the @).
func activeMentionToken(text string) (query string, ok bool) {
	at := strings.LastIndex(text, "@")
	if at < 0 {
		return "", false
	}
	token := text[at+1:]
	if strings.ContainsAny(token, " \n\t") {
		return "", false
	}
	return token, true
}

// mentionMatches lists workspace files and folders matching the @ query (fuzzy
// basename first, exactly as AtMentionMenu.tsx flattens the file tree).
func (a *AppUI) mentionMatches(query string) []composerSuggestion {
	if a.Workspace == nil {
		return nil
	}
	ver := a.Workspace.SnapshotVersion()
	root := a.Workspace.RootDir
	if a.mentionMatchCached != nil && a.mentionQueryCached == query && a.mentionMatchVersion == ver && a.mentionMatchRoot == root {
		return a.mentionMatchCached
	}
	files := a.Workspace.FilteredFiles()
	type scored struct {
		path  string
		isDir bool
		score float64
	}
	var matches []scored
	seenDirs := make(map[string]bool)
	addCandidate := func(relPath string, isDir bool) {
		matchTarget := relPath
		if isDir && strings.HasSuffix(query, "/") {
			matchTarget = strings.TrimSuffix(relPath, "/") + "/"
		}
		if query == "" {
			matches = append(matches, scored{path: relPath, isDir: isDir, score: 0})
		} else if m := fuzzy.ScoreFilePath(query, matchTarget); m != nil {
			matches = append(matches, scored{path: relPath, isDir: isDir, score: m.Score})
		}
	}
	for _, f := range files {
		normRel := strings.TrimSuffix(strings.ReplaceAll(f.RelativePath, "\\", "/"), "/")
		if normRel == "" || normRel == "." {
			continue
		}
		if f.IsDir {
			if !seenDirs[normRel] {
				seenDirs[normRel] = true
				addCandidate(normRel, true)
			}
			continue
		}
		// Synthesize parent directories because WorkspaceState.Refresh() only
		// stores leaf files in Files.
		for dir := path.Dir(normRel); dir != "." && dir != "" && dir != "/"; dir = path.Dir(dir) {
			if !seenDirs[dir] {
				seenDirs[dir] = true
				addCandidate(dir, true)
			}
		}
		addCandidate(f.RelativePath, false)
	}
	if query != "" {
		sort.SliceStable(matches, func(i, j int) bool {
			if matches[i].score != matches[j].score {
				return matches[i].score > matches[j].score
			}
			return matches[i].path < matches[j].path
		})
	}
	const maxMentionMatches = 50
	if len(matches) > maxMentionMatches {
		matches = matches[:maxMentionMatches]
	}
	out := make([]composerSuggestion, 0, len(matches))
	for _, m := range matches {
		if m.isDir {
			dirPath := strings.TrimSuffix(m.path, "/") + "/"
			out = append(out, composerSuggestion{
				Name:        dirPath,
				Description: "Folder",
				Category:    "folder",
				Insert:      "@" + dirPath + " ",
			})
			continue
		}
		out = append(out, composerSuggestion{
			Name:     m.path,
			Category: "file",
			Insert:   "@" + m.path + " ",
		})
	}
	a.mentionQueryCached = query
	a.mentionMatchVersion = ver
	a.mentionMatchRoot = root
	a.mentionMatchCached = out
	return out
}

// renderComposerMenu draws the floating autocomplete panel above the input:
// scrollable rows with category/file/folder icon, mono name, muted description,
// selection highlight, and the pinned footer hint bar of the webview menus.
func (a *AppUI) renderComposerMenu(c *ui.Context, tc theme.ThemeColors, items []composerSuggestion, selected int, kind string) {
	panel := ui.Column(c).Radius(12).Background(tc.SurfaceRaised).
		BorderWidth(1).BorderColor(tc.Border).
		Shadow(0, 8, 24, -12, ui.RGBA(0, 0, 0, 0.45)).
		MaxHeight(260).Clip()
	panel.Children(func() {
		ui.Scroll(c.Key(kind + "-menu-scroll")).MaxHeight(220).Children(func() {
			for i, it := range items {
				idx, item := i, it
				row := ui.Row(c.Key(fmt.Sprintf("%s-item-%s-%d", kind, item.Name, idx))).
					Padding(6, 10).Gap(8).AlignItems(ui.Center)
				row.Children(func() {
					switch item.Category {
					case "folder":
						ui.Icon(c, icons.TablerFolder).Width(13).Height(13).TextColor(tc.Accent)
					case "file":
						ui.Icon(c, icons.TablerFile).Width(13).Height(13).TextColor(tc.TextMuted)
					default:
						ui.Box(c).Size(6, 6).Radius(3).Background(categoryColor(tc, item.Category))
					}
					name := ui.Text(c, item.Name).FontSize(12)
					if item.Category == "file" || item.Category == "folder" {
						name.Font("Geist Mono")
					} else {
						name.FontWeight(500)
					}
					if item.Description != "" {
						ui.Text(c, item.Description).FontSize(11).TextColor(tc.TextMuted).SingleLine().Grow(1)
					}
					ui.Box(c).Grow(1)
					if idx == selected {
						ui.Text(c, "↵").FontSize(11).TextColor(tc.TextMuted)
					}
				})
				if idx == selected {
					row.Background(tc.Text.Alpha(0.08))
				} else if row.Hovered() {
					row.Background(tc.Text.Alpha(0.05))
				}
				if row.Clicked() {
					a.applyComposerSuggestion(item)
				}
			}
		})
		footer := ui.Row(c).Padding(5, 10).BorderWidth(1, 0, 0, 0).BorderColor(tc.Border.Alpha(0.5)).Gap(8).AlignItems(ui.Center)
		footer.Children(func() {
			KbdText(c, tc, "↑↓")
			ui.Text(c, "navigate").FontSize(10).TextColor(tc.SecondaryLabel)
			KbdText(c, tc, "↵")
			ui.Text(c, "select").FontSize(10).TextColor(tc.SecondaryLabel)
			KbdText(c, tc, "esc")
			ui.Text(c, "dismiss").FontSize(10).TextColor(tc.SecondaryLabel)
			ui.Box(c).Grow(1)
			ui.Icon(c, icons.TablerSparkles).Width(10).Height(10).TextColor(tc.Accent.Alpha(0.7))
		})
	})
}

// composerSlashState derives the slash-menu visibility and filtered items
// from the composer text: active while the text starts with "/", until the
// user dismisses it with Esc (latched on the dismissed text).
func (a *AppUI) composerSlashState(as *agent.AgentState) ([]composerSuggestion, bool) {
	text := as.InputText
	if !strings.HasPrefix(text, "/") {
		a.SlashSelected = 0
		return nil, false
	}
	if text == a.SlashDismissedAt {
		return nil, false
	}
	items := filterSuggestions(a.slashCatalog(as), text)
	return items, len(items) > 0
}

// composerMentionState derives the @-mention menu from the trailing
// "@token" of the composer text.
func (a *AppUI) composerMentionState(as *agent.AgentState) ([]composerSuggestion, string, bool) {
	query, ok := activeMentionToken(as.InputText)
	if !ok {
		a.MentionSelected = 0
		return nil, "", false
	}
	latch := "@" + query
	if latch == a.MentionDismissedAt {
		return nil, "", false
	}
	items := a.mentionMatches(query)
	return items, query, len(items) > 0
}

// applyComposerSuggestion inserts the suggestion into the composer or executes built-in slash actions.
func (a *AppUI) applyComposerSuggestion(item composerSuggestion) {
	switch item.Name {
	case "/settings", "/setup":
		a.openSettings()
		a.Agent.InputText = ""
		a.SlashDismissedAt = ""
		a.MentionDismissedAt = ""
		if a.Window != nil {
			a.Window.Invalidate()
		}
		return
	case "/model":
		a.ShowModelPalette = true
		a.ModelSearchQuery = ""
		a.Agent.InputText = ""
		a.SlashDismissedAt = ""
		a.MentionDismissedAt = ""
		if a.Window != nil {
			a.Window.Invalidate()
		}
		return
	case "/usage":
		a.openUsageModal()
		a.Agent.InputText = ""
		a.SlashDismissedAt = ""
		a.MentionDismissedAt = ""
		if a.Window != nil {
			a.Window.Invalidate()
		}
		return
	case "/clear":
		// Shared locked implementation: nils both the session's messages and
		// the active as.Messages alias, resets streaming state, and persists
		// the emptied session to disk (the previous inline version did none
		// of that: it dropped messages without the lock, left as.Messages
		// pointing at the old slice, and never saved).
		a.Agent.ClearConversation()
		a.Agent.InputText = ""
		a.SlashDismissedAt = ""
		a.MentionDismissedAt = ""
		if a.Window != nil {
			a.Window.Invalidate()
		}
		return
	case "/compact":
		a.Agent.InputText = "Please compact our conversation context: synthesize all progress, decisions, architectural constraints, and pending tasks into a dense, high-signal summary, and discard superseded back-and-forth turns."
		a.SlashDismissedAt = ""
		a.MentionDismissedAt = ""
		if a.Window != nil {
			a.Window.Invalidate()
		}
		return
	}

	insert := item.Insert
	if insert == "" {
		insert = item.Name + " "
	}
	if item.Category == "file" || strings.HasPrefix(insert, "@") {
		if at := strings.LastIndex(a.Agent.InputText, "@"); at >= 0 {
			a.Agent.InputText = a.Agent.InputText[:at] + insert
		} else {
			a.Agent.InputText += insert
		}
	} else {
		a.Agent.InputText = insert
	}
	a.SlashDismissedAt = ""
	a.MentionDismissedAt = ""
	if a.Window != nil {
		a.Window.Invalidate()
	}
}
