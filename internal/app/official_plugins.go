package app

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unsafe"

	"github.com/hasdev/forge-ade/internal/browseruse"
	"github.com/hasdev/forge-ade/internal/events"
	"github.com/hasdev/forge-ade/internal/mcp"
	"github.com/hasdev/forge-ade/internal/skills"
	"github.com/hasdev/forge-ade/internal/tools"
)

// ---------------------------------------------------------------------------
// Official plugins: Computer Use (ZCode's official MCP server) and Browser Use
// (native CDP implementation + control skill). Both extend the internal agent
// the same way ZCode's harness does — tools plus a guidance skill.
// ---------------------------------------------------------------------------

// seedComputerUse wires ZCode's official computer-use MCP server into the
// app's MCP manager so its tools appear as computer-use/<tool> (e.g.
// computer-use/get_app_state, computer-use/screenshot). The server ships in
// the ZCode plugin cache; we run it with plain node. Skipped silently when
// the cache or node is missing, or when the user already configured (or
// deliberately deleted) a "computer-use" server.
func seedComputerUse(mcpMgr *mcp.Manager, skillMgr *skills.Manager) {
	serverJS, pluginDir := locateComputerUseServer()
	if serverJS == "" {
		return
	}
	node, err := exec.LookPath("node")
	if err != nil {
		return
	}

	configured := false
	for _, s := range mcpMgr.ListServers() {
		if s.Name == "computer-use" {
			configured = true
		}
	}
	if !configured {
		_, _ = mcpMgr.SaveServer(mcp.ServerConfig{
			Name:    "computer-use",
			Command: node,
			Args:    []string{serverJS},
			Env: map[string]string{
				"ZCODE_PLUGIN_ROOT": pluginDir,
				"NODE_ENV":          "production",
			},
			Enabled: true,
		})
	}

	// Register the computer-use guidance skill (observe → act → verify loop).
	if skillMgr != nil {
		if skill, ok := loadSkillMD(filepath.Join(pluginDir, "skills", "computer-use", "SKILL.md")); ok {
			skillMgr.RegisterPluginSkills("computer-use", []skills.Skill{skill})
		}
	}
}

// locateComputerUseServer finds the newest computer-use plugin version in the
// ZCode plugin cache. Returns ("", "") when absent.
func locateComputerUseServer() (serverJS string, pluginDir string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", ""
	}
	root := filepath.Join(home, ".zcode", "cli", "plugins", "cache", "zcode-plugins-official", "computer-use")
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", ""
	}
	versions := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			versions = append(versions, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(versions))) // works for plain semver strings here
	for _, v := range versions {
		candidate := filepath.Join(root, v, "dist", "mcp", "server.js")
		if fi, err := os.Stat(candidate); err == nil && fi.Size() > 0 {
			return candidate, filepath.Join(root, v)
		}
	}
	return "", ""
}

// loadSkillMD parses a SKILL.md's frontmatter into a skill.
func loadSkillMD(path string) (skills.Skill, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return skills.Skill{}, false
	}
	text := string(data)
	skill := skills.Skill{Path: path, Body: text}
	if strings.HasPrefix(text, "---") {
		if end := strings.Index(text[3:], "---"); end >= 0 {
			front := text[3 : 3+end]
			skill.Body = strings.TrimSpace(text[3+end+3:])
			var meta map[string]string
			if err := json.Unmarshal([]byte(toJSONish(front)), &meta); err == nil {
				skill.Name = meta["name"]
				skill.Description = meta["description"]
			}
		}
	}
	if skill.Name == "" {
		skill.Name = strings.TrimSuffix(filepath.Base(path), ".md")
	}
	return skill, true
}

// toJSONish converts `key: value` YAML frontmatter lines to a JSON object
// (enough for the two fields we need, without a YAML dependency).
func toJSONish(front string) string {
	parts := make([]string, 0)
	for _, line := range strings.Split(front, "\n") {
		kv := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.Trim(strings.TrimSpace(kv[1]), `"'`)
		parts = append(parts, fmt.Sprintf("%q: %q", key, val))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// browserUseSkill is the control-browser playbook adapted to forge-ade's
// native browser tools (the counterpart of ZCode's browser-use plugin skills).
const browserUseSkillBody = `# Browser Use

You control a real Chromium browser with the browser_* tools. The user watches
every action live in the Browser viewer panel in the right sidebar.

## Core loop: snapshot → act → verify

1. Open pages with browser_navigate. It returns the page outline with element
   refs — that outline is your eyes; you do not have vision by default.
2. Before any interaction, make sure your snapshot is fresh (browser_navigate
   and every action return one; call browser_snapshot otherwise).
3. Act on refs, never guesses: browser_click ref=e12, browser_type ref=e5
   (submit: true for search boxes), browser_key for Enter/Escape/arrows,
   browser_scroll to reveal more content.
4. Verify the result from the returned outline. If a ref is rejected as stale,
   the page changed — take a new browser_snapshot and re-target.
5. Use browser_screenshot only to check visual layout (images, colors,
   rendering); it returns a file path and updates the viewer. Use
   browser_tabs to open/select/close tabs when a task spans several pages.
6. Long forms: fill fields one ref at a time, then click the submit ref.
7. When done with the browser, call browser_close to free resources.

Rules:
- Never fabricate refs. If an element is missing from the snapshot, scroll or
  navigate; if it is truly absent, say so.
- One action per call, then read the returned outline before continuing.
- Pages can change under you — after any click that navigates, re-snapshot.
- Do NOT call browser_navigate again when the viewer is already showing the
  target URL (check the snapshot header / last navigate result). Navigating
  reloads the page and resets in-page state — counters, forms, scroll
  position. To act on an already-open page, take browser_snapshot and act on
  its refs.
- A click result includes clicked_label (the element's text right after your
  click) and loads (how many times the page has loaded). If loads jumps
  between actions, the page reloaded and reset its state — find out why
  before re-clicking; usually you (re-)navigated, or the app under test
  hot-reloaded.
- If loads climbs between two actions WITHOUT you navigating, the app under
  test is reload-storming (its dev server is restarting, or its hot-reload
  websocket keeps dying). The click flow waits for it to go quiet and
  re-lands your click once (retried_after_reload); when reload_storm is
  present the page was still churning — re-snapshot and verify the effect
  instead of firing more blind actions, and prefer consolidating the app on
  a single dev server before retrying.
`

func seedBrowserUse(skillMgr *skills.Manager) {
	if skillMgr == nil {
		return
	}
	skillMgr.RegisterPluginSkills("browser-use", []skills.Skill{{
		Name:        "browser-use",
		Description: "Control the managed browser: navigate, snapshot-first interaction with element refs, screenshots, and tabs. Load before any web task.",
		Body:        browserUseSkillBody,
	}})
}

// wireBrowserUse creates the browser-use manager (in-app WKWebView engine on
// macOS, headless CDP fallback), registers its tools into the agent registry,
// and streams its frames/status onto the event bus.
func wireBrowserUse(dataDir string, bus *events.Bus, toolReg *tools.Registry, windowFn func() unsafe.Pointer) *browseruse.Manager {
	bm := browseruse.NewManager(dataDir)
	bm.SetWindowProvider(windowFn)
	bm.SetEmitter(func(event string, data map[string]any) {
		bus.Publish(events.Event{Type: events.EventType(event), Data: data})
	})
	tools.RegisterBrowserTools(toolReg, bm)
	return bm
}
