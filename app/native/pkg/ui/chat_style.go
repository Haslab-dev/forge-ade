package ui

// chat_style.go holds the shared ZCode-v4 chat primitives: the running-label
// shimmer, the WorkflowAgentFace tile, two-unit work durations, tool family
// classification ("Explored N files · Ran 2 commands" groups) and per-tool
// diffstat parsing. Rendering lives in agent_view.go; the subagent drawer
// reuses the face + summary helpers from here.

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
)

// ──────────────────────────────────────────────────────────────────────────
// Shimmer: ZCode's animated-gradient-text stand-in. MyGo has no gradient
// text, so running labels pulse between the muted and accent colors on a
// ~1.1s sine. Frames land at token cadence while streaming plus the 150ms
// streaming ticker, which is enough for the pulse to read as motion.
// ──────────────────────────────────────────────────────────────────────────

// shimmerPhase returns the 0..1 shimmer position for the current wall clock.
func shimmerPhase() float32 {
	const periodMs = 1100
	ms := time.Now().UnixMilli() % periodMs
	// Half-sine ease: 0→1→0 over the period.
	phase := float32(ms) / float32(periodMs)
	return 0.5 - 0.5*float32(math.Cos(2*math.Pi*float64(phase)))
}

// shimmerTextColor returns the pulsing label color for a running row.
func shimmerTextColor(tc theme.ThemeColors) ui.Color {
	p := shimmerPhase()
	// Ease between muted (dim) and accent (bright) twice per period.
	t := p
	if t > 0.5 {
		t = 1 - t
	}
	t = t * 2
	return tc.TextMuted.Mix(tc.Accent, 0.25+0.75*t)
}

// ──────────────────────────────────────────────────────────────────────────
// Durations: ZCode's two-unit format — "2m 10s", "42s", "1h 04m".
// ──────────────────────────────────────────────────────────────────────────

// formatWorkDuration renders a ZCode-style elapsed label.
func formatWorkDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int64(d.Seconds())
	if total < 60 {
		return fmt.Sprintf("%ds", total)
	}
	if total < 3600 {
		return fmt.Sprintf("%dm %02ds", total/60, total%60)
	}
	return fmt.Sprintf("%dh %02dm", total/3600, (total%3600)/60)
}

// clampRunes truncates s to max runes with an ellipsis.
func clampRunes(s string, max int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	return string(r[:max-1]) + "…"
}

// lastNonEmptyLine returns the final non-blank line of text, for the
// collapsed reasoning preview.
func lastNonEmptyLine(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if s := strings.TrimSpace(lines[i]); s != "" {
			return s
		}
	}
	return ""
}

// ──────────────────────────────────────────────────────────────────────────
// WorkflowAgentFace: the sub-agent identity tile — a rounded square filled
// with a stable per-name color, two eyes, and a status badge (green check
// when done, red ✕ when failed). Ported from ZCode's WorkflowAgentFace.
// ──────────────────────────────────────────────────────────────────────────

// agentFaceColor picks the tile color: FNV-1a hash of the name into the
// palette's graph lane colors (stable across sessions).
func agentFaceColor(tc theme.ThemeColors, name string) ui.Color {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	idx := int(h.Sum32()) % len(tc.GraphColors)
	c := tc.GraphColors[idx]
	if c.A == 0 {
		return tc.Accent
	}
	return c
}

// agentEyeColor picks an eye color with contrast against the tile fill by
// mixing toward the canvas on very bright fills.
func agentEyeColor(fill ui.Color) ui.Color {
	return ui.RGBA(0, 0, 0, 0.55)
}

// agentFace renders the 20×20 identity tile. status: "running", "done",
// "failed" or "idle" (idle = closed eyes, used for ended agents).
func agentFace(c *ui.Context, tc theme.ThemeColors, name, status string) {
	fill := agentFaceColor(tc, name)
	tile := ui.Box(c).Size(20, 20).Radius(7).Background(fill.Alpha(0.9)).
		AlignItems(ui.Center).Justify(ui.Center)
	tile.Children(func() {
		eye := agentEyeColor(fill)
		if status == "idle" {
			// Closed eyes: two short dashes.
			ui.Row(c).Gap(3).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(4, 1.5).Radius(1).Background(eye)
				ui.Box(c).Size(4, 1.5).Radius(1).Background(eye)
			})
			return
		}
		ui.Row(c).Gap(3).AlignItems(ui.Center).Padding(0, 0, 3, 0).Children(func() {
			ui.Box(c).Size(3, 3.5).Radius(1.5).Background(eye)
			ui.Box(c).Size(3, 3.5).Radius(1.5).Background(eye)
		})
	})
	// Status badge overlays the tile's bottom-left corner, ZCode-style.
	switch status {
	case "done":
		ui.Box(c).Absolute().Bottom(0).Left(0).Size(11, 11).Radius(6).
			Background(tc.Success).AlignItems(ui.Center).Justify(ui.Center).
			BorderWidth(1.5).BorderColor(tc.Canvas).Children(func() {
			ui.Icon(c, icons.TablerCheck).Width(7).Height(7).TextColor(tc.Canvas)
		})
	case "failed":
		ui.Box(c).Absolute().Bottom(0).Left(0).Size(11, 11).Radius(6).
			Background(tc.Error).AlignItems(ui.Center).Justify(ui.Center).
			BorderWidth(1.5).BorderColor(tc.Canvas).Children(func() {
			ui.Icon(c, icons.TablerX).Width(7).Height(7).TextColor(tc.Canvas)
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────
// Tool classification: one shared detection pass for the summary rows, the
// consecutive-step groups and the drawer's read-only rows.
// ──────────────────────────────────────────────────────────────────────────

// toolFamily buckets: consecutive explore/run/change calls collapse into
// ZCode statistics groups; the others render as standalone rows.
const (
	tfExplore = "explore"
	tfRun     = "run"
	tfChange  = "change"
	tfAgent   = "agent"
	tfMCP     = "mcp"
	tfSkill   = "skill"
	tfDesign  = "design"
	tfOther   = "other"
)

// toolVisual is the classifyTool result: everything a summary row needs.
type toolVisual struct {
	Family     string
	Icon       *ui.SVG
	KindLabel  string // "Ran", "Edited", "Read", "Searched"…
	Badge      string // legacy chip text ("" when the kind label covers it)
	Label      string // summary text (path / command / query / role)
	FilePath   string // set when Label is a workspace path (clickable chip)
	IsRead     bool
	IsEdit     bool
	IsBash     bool
	IsSubagent bool
	IsMCPLoad  bool
	SubPrompt  string // subagent prompt argument
	SubRole    string // subagent role/type argument
}

// classifyTool ports the renderToolTraceRow detection heuristics so rows and
// groups agree on iconography and labels.
func classifyTool(t *agent.ToolExecution) toolVisual {
	v := toolVisual{Family: tfOther, Icon: icons.TablerTerminal, Label: t.ToolName}
	if t == nil {
		return v
	}
	args := parseToolArgsCached(t)
	nl := strings.ToLower(t.ToolName)
	isSkill := strings.Contains(nl, "skill")
	isMCPLoad := strings.Contains(nl, "mcp_load") || strings.Contains(nl, "load_mcp") || strings.Contains(nl, "mcp_connect") || strings.Contains(nl, "connect_mcp") || strings.Contains(nl, "mcp_server") || strings.Contains(nl, "list_mcp")
	isMCPTool := strings.HasPrefix(nl, "mcp__") || strings.HasPrefix(nl, "mcp_") || strings.Contains(nl, "mcp_tool") || strings.Contains(nl, "mcptool")
	isDesignTool := strings.Contains(nl, "deck") || strings.Contains(nl, "motion") || strings.Contains(nl, "opendesign") || strings.Contains(nl, "open-design") || strings.Contains(nl, "forge-design") || strings.Contains(nl, "hyperframe") || isDeckOrMotionFile(args.Path)
	isBash := strings.Contains(nl, "bash") || strings.Contains(nl, "shell") || strings.Contains(nl, "exec") || strings.Contains(nl, "terminal") || strings.Contains(nl, "run_command")
	isRead := strings.Contains(nl, "read") || strings.Contains(nl, "view") || strings.Contains(nl, "cat")
	isEdit := strings.Contains(nl, "edit") || strings.Contains(nl, "write") || strings.Contains(nl, "patch") || strings.Contains(nl, "create") || strings.Contains(nl, "delete") || strings.Contains(nl, "remove")
	isSearch := strings.Contains(nl, "search") || strings.Contains(nl, "grep") || strings.Contains(nl, "find") || strings.Contains(nl, "glob") || strings.Contains(nl, "list") || strings.Contains(nl, "fetch") || strings.Contains(nl, "web")
	isSubagent := strings.Contains(nl, "invoke_subagent") || strings.Contains(nl, "subagent") || nl == "task" || strings.Contains(nl, "spawn_subagent")
	// Task-list tools (manage_task, update_task, todo_write…) drive the
	// checklist rather than touching the world.
	isTaskTool := !isSubagent && (strings.Contains(nl, "task") || strings.Contains(nl, "todo"))

	switch {
	case isSkill:
		v.Family = tfSkill
		v.Icon = icons.TablerSparkles
		v.KindLabel = "Skill"
		v.Badge = "Skill"
		v.Label = extractSkillLabel(t.Command, t.ToolName)
	case isMCPLoad:
		v.Family = tfMCP
		v.Icon = icons.TablerPlug
		v.KindLabel = "Loaded MCP"
		v.IsMCPLoad = true
		v.Badge = "MCP Server"
		v.Label = extractMCPServerLabel(t.Command, t.ToolName)
	case isMCPTool:
		v.Family = tfMCP
		v.Icon = icons.TablerCpu
		server, tool := extractMCPToolParts(t.ToolName)
		if server != "" {
			v.Badge = "MCP: " + server
		} else {
			v.Badge = "MCP"
		}
		v.KindLabel = "MCP"
		v.Label = tool
	case isTaskTool:
		v.Family = tfOther
		v.Icon = icons.TablerChecklist
		v.KindLabel = "Updated tasks"
		// Prefer the affected task title from the args.
		var rawArgs map[string]interface{}
		_ = json.Unmarshal([]byte(t.Command), &rawArgs)
		for _, key := range []string{"title", "Title", "task", "Task", "name", "Name"} {
			if s, ok := rawArgs[key].(string); ok && strings.TrimSpace(s) != "" {
				v.Label = clampRunes(s, 64)
				break
			}
		}
	case isDesignTool:
		v.Family = tfDesign
		v.Icon = icons.TablerPalette
		v.KindLabel = "Design"
		v.Badge = "Forge Design"
		if args.Path != "" {
			v.Label = args.Path
			v.FilePath = args.Path
		} else {
			v.Label = "Presentation / Motion"
		}
	case isSubagent:
		v.Family = tfAgent
		v.Icon = icons.TablerBot
		v.KindLabel = "Delegated"
		v.IsSubagent = true
		var rawArgs map[string]interface{}
		_ = json.Unmarshal([]byte(t.Command), &rawArgs)
		for _, key := range []string{"Role", "role", "TypeName", "typename", "subagent_type", "SubagentType"} {
			if r, ok := rawArgs[key].(string); ok && r != "" {
				v.SubRole = r
				break
			}
		}
		for _, key := range []string{"Prompt", "prompt", "Description", "description"} {
			if p, ok := rawArgs[key].(string); ok && p != "" {
				v.SubPrompt = p
				break
			}
		}
		if v.SubRole == "" {
			v.SubRole = "Subagent"
		}
		if v.SubPrompt != "" {
			v.Label = clampRunes(v.SubPrompt, 80)
		} else {
			v.Label = v.SubRole
		}
	case isEdit:
		v.Family = tfChange
		v.Icon = icons.TablerFilePen
		v.IsEdit = true
		if strings.Contains(nl, "delete") || strings.Contains(nl, "remove") {
			v.KindLabel = "Delete"
			v.Badge = "Delete"
		} else if strings.Contains(nl, "write") || strings.Contains(nl, "create") {
			v.KindLabel = "Write"
			v.Badge = "Write"
		} else {
			v.KindLabel = "Edit"
			v.Badge = "Edit"
		}
		v.Label = args.Path
		v.FilePath = args.Path
	case isRead:
		v.Family = tfExplore
		v.Icon = icons.TablerFileText
		v.KindLabel = "Read"
		v.IsRead = true
		v.Label = args.Path
		v.FilePath = args.Path
	case isSearch:
		v.Family = tfExplore
		v.Icon = icons.TablerSearch
		v.KindLabel = "Search"
		if args.Query != "" {
			v.Label = args.Query
			if args.Path != "" {
				v.FilePath = args.Path
			}
		} else if args.Path != "" {
			v.Label = args.Path
			v.FilePath = args.Path
		} else {
			v.Label = t.ToolName
		}
	case isBash:
		v.Family = tfRun
		v.Icon = icons.TablerTerminal
		v.KindLabel = "Terminal"
		v.IsBash = true
		if cmd := extractShellCommand(t.Command, args.Cmd); cmd != "" {
			v.Label = cmd
		} else {
			v.Label = t.ToolName
		}
	default:
		// Unknown tool: the tool name acts as the kind label, ZCode's
		// fallback renderer behavior.
		v.KindLabel = t.ToolName
		if args.Path != "" {
			v.Label = args.Path
			v.FilePath = args.Path
		} else {
			v.Label = ""
		}
	}
	if strings.TrimSpace(v.KindLabel) == "" {
		v.KindLabel = t.ToolName
	}
	return v
}

// groupNounCounts tallies a run of grouped tools into ZCode's activity
// nouns: searches, files (reads), commands (bash) and edits.
type groupNouns struct {
	search, file, command, edit int
}

// line renders "1 search, 2 files" — pluralized, zero groups omitted.
func (n groupNouns) line() string {
	parts := make([]string, 0, 4)
	add := func(count int, singular, plural string) {
		if count <= 0 {
			return
		}
		word := plural
		if count == 1 {
			word = singular
		}
		parts = append(parts, fmt.Sprintf("%d %s", count, word))
	}
	add(n.search, "search", "searches")
	add(n.file, "file", "files")
	add(n.command, "command", "commands")
	add(n.edit, "edit", "edits")
	return strings.Join(parts, ", ")
}

// kind picks the group's leading word by dominant activity — ZCode's
// "Explore" / "Ran" style.
func (n groupNouns) kind() string {
	best := n.search
	word := "Explore"
	if n.file > best {
		best, word = n.file, "Explore"
	}
	if n.command > best {
		best, word = n.command, "Ran"
	}
	if n.edit > best {
		best, word = n.edit, "Edit"
	}
	if best <= 0 {
		return "Worked"
	}
	return word
}

// groupRepresentativeIcon picks the group icon by the dominant kind word,
// mirroring ZCode's group iconography.
func groupRepresentativeIcon(kind string) *ui.SVG {
	switch kind {
	case "Edit":
		return icons.TablerFilePen
	case "Ran":
		return icons.TablerTerminal
	case "Worked":
		return icons.TablerChecklist
	default:
		return icons.TablerSearch
	}
}

// ──────────────────────────────────────────────────────────────────────────
// Per-tool diffstats: "+N −N" on edit rows. Parsed from the unified patch in
// the tool output (or estimated from old/new args when the harness returns
// no patch), cached per tool ID so a big diff is parsed once per frame pass.
// ──────────────────────────────────────────────────────────────────────────

var (
	diffStatMu    sync.Mutex
	diffStatCache = map[string]diffStat{}
)

type diffStat struct {
	add, del int
	ok       bool
	outLen   int // output length at parse time (invalidate on append)
}

// toolDiffStatCached returns the +/- line counts for a completed edit tool.
func toolDiffStatCached(t *agent.ToolExecution) (add, del int, ok bool) {
	if t == nil || t.Status == "running" {
		return 0, 0, false
	}
	diffStatMu.Lock()
	defer diffStatMu.Unlock()
	if st, hit := diffStatCache[t.ID]; hit && st.outLen == len(t.Output) {
		return st.add, st.del, st.ok
	}
	st := computeDiffStat(t)
	if len(diffStatCache) > 512 {
		diffStatCache = map[string]diffStat{} // bounded; oldest entries dropped wholesale
	}
	diffStatCache[t.ID] = st
	return st.add, st.del, st.ok
}

func computeDiffStat(t *agent.ToolExecution) diffStat {
	args := parseToolArgsCached(t)
	patch := extractDiffFromToolOutput(t.Output, args.Path)
	if patch != "" {
		add, del := countPatchLines(patch)
		if add > 0 || del > 0 {
			return diffStat{add: add, del: del, ok: true, outLen: len(t.Output)}
		}
	}
	// No patch in the output: estimate from old/new buffers, the same shape
	// ZCode shows for buffer diffs.
	if args.OldString != "" || args.NewString != "" {
		add, del := countBufferDiffLines(args.OldString, args.NewString)
		return diffStat{add: add, del: del, ok: add > 0 || del > 0, outLen: len(t.Output)}
	}
	if len(args.Edits) > 0 {
		add, del := 0, 0
		for _, ep := range args.Edits {
			a2, d2 := countBufferDiffLines(ep.OldString, ep.NewString)
			add += a2
			del += d2
		}
		return diffStat{add: add, del: del, ok: add > 0 || del > 0, outLen: len(t.Output)}
	}
	if args.Content != "" && args.Path != "" {
		lines := strings.Count(args.Content, "\n") + 1
		return diffStat{add: lines, del: 0, ok: true, outLen: len(t.Output)}
	}
	return diffStat{outLen: len(t.Output)}
}

// countPatchLines counts +/− lines in a unified diff, skipping headers.
func countPatchLines(patch string) (add, del int) {
	for _, line := range strings.Split(patch, "\n") {
		switch {
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"), strings.HasPrefix(line, "diff "), strings.HasPrefix(line, "index "):
			continue
		case strings.HasPrefix(line, "+"):
			add++
		case strings.HasPrefix(line, "-"):
			del++
		}
	}
	return add, del
}

// countBufferDiffLines estimates +/- counts between two buffers by treating
// old lines not present in new as deletions and vice versa.
func countBufferDiffLines(oldStr, newStr string) (add, del int) {
	oldLines := splitNonEmptyLines(oldStr)
	newLines := splitNonEmptyLines(newStr)
	oldSet := make(map[string]int, len(oldLines))
	for _, l := range oldLines {
		oldSet[l]++
	}
	newSet := make(map[string]int, len(newLines))
	for _, l := range newLines {
		newSet[l]++
	}
	for _, l := range newLines {
		if oldSet[l] > 0 {
			oldSet[l]--
		} else {
			add++
		}
	}
	for _, l := range oldLines {
		if newSet[l] > 0 {
			newSet[l]--
		} else {
			del++
		}
	}
	return add, del
}

func splitNonEmptyLines(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	out := parts[:0]
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return out
}
