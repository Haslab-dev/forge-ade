package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SkillEntry is one discovered SKILL.md. Port of adapters/src/skills.
type SkillEntry struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	WhenToUse      string `json:"when_to_use,omitempty"`
	Path           string `json:"path"`
	Dir            string `json:"dir"`
	Scope          string `json:"scope"` // user|project|system
	Source         string `json:"source,omitempty"`
	PluginName     string `json:"plugin_name,omitempty"`
	PluginID       string `json:"plugin_id,omitempty"`
	QualifiedName  string `json:"qualified_name,omitempty"`
	Priority       int    `json:"priority"`
}

// SkillRoot is a scan root with precedence priority (lower scanned first).
type SkillRoot struct {
	Path     string
	Scope    string // "user" | "project" | "system"
	Source   string // "forge" | "agents" | "plugin"
	Priority int
	PluginID string
}

// skillWalkExclusions from skill-scan-policy.ts.
var skillWalkExclusions = map[string]bool{
	"node_modules": true, "dist": true, "build": true, "out": true, "target": true,
	"vendor": true, "coverage": true, ".cache": true, ".next": true, ".turbo": true,
	".venv": true, "__pycache__": true,
}

// DiscoverSkillRoots port of roots.ts precedence:
// 1. extra roots (project) 2. ~/.forge/skills then ~/.agents/skills (user)
// 3. walk from workdir up to git root, .forge/skills then .agents/skills
// 4. plugin roots (priority >= 1000, appended by caller).
func DiscoverSkillRoots(workingDir string, extraRoots []string, homeDir string) []SkillRoot {
	var roots []SkillRoot
	prio := 0
	next := func() int { p := prio; prio += 10; return p }

	for _, r := range extraRoots {
		roots = append(roots, SkillRoot{Path: r, Scope: "project", Source: "forge", Priority: next()})
	}
	if homeDir != "" {
		roots = append(roots, SkillRoot{Path: filepath.Join(homeDir, ".forge", "skills"), Scope: "user", Source: "forge", Priority: next()})
		roots = append(roots, SkillRoot{Path: filepath.Join(homeDir, ".agents", "skills"), Scope: "user", Source: "agents", Priority: next()})
	}
	// Walk up to git worktree root.
	if workingDir != "" {
		dir := workingDir
		for {
			for _, sub := range []string{".forge", ".agents"} {
				roots = append(roots, SkillRoot{Path: filepath.Join(dir, sub, "skills"), Scope: "project",
					Source: map[string]string{".forge": "forge", ".agents": "agents"}[sub], Priority: next()})
			}
			if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil || dir == filepath.Dir(dir) {
				break
			}
			dir = filepath.Dir(dir)
		}
	}
	return roots
}

// ScanSkills port of scan.ts: per root, check root's own SKILL.md then one
// level of subdirectories. Plugin-scope roots never follow symlinks.
func ScanSkills(roots []SkillRoot) ([]SkillEntry, []string) {
	var skills []SkillEntry
	var diagnostics []string
	seen := map[string]bool{}
	for _, root := range roots {
		entries, err := os.ReadDir(root.Path)
		if err != nil {
			if !os.IsNotExist(err) {
				diagnostics = append(diagnostics, "skill_scan_failed: "+root.Path)
			}
			continue
		}
		consider := func(skillFile, dir string) {
			data, err := os.ReadFile(skillFile)
			if err != nil {
				diagnostics = append(diagnostics, "skill_read_failed: "+skillFile)
				return
			}
			fm, _ := parseFrontmatter(string(data))
			name := fm["name"]
			if name == "" {
				name = filepath.Base(dir)
			}
			if seen[name] {
				diagnostics = append(diagnostics, "skill_duplicate_name: "+name)
				return
			}
			seen[name] = true
			desc := fm["description"]
			if len(desc) > 1024 {
				diagnostics = append(diagnostics, "skill_description_too_long: "+name)
				desc = desc[:1024]
			}
			e := SkillEntry{
				Name: name, Description: desc, WhenToUse: fm["when_to_use"],
				Path: skillFile, Dir: dir, Scope: root.Scope, Source: root.Source, Priority: root.Priority,
			}
			if root.PluginID != "" {
				e.PluginID = root.PluginID
				e.QualifiedName = root.PluginID + ":" + name
			}
			skills = append(skills, e)
		}
		// root itself may be a skill
		if fi, err := os.Stat(filepath.Join(root.Path, "SKILL.md")); err == nil && !fi.IsDir() {
			consider(filepath.Join(root.Path, "SKILL.md"), root.Path)
		}
		for _, e := range entries {
			if !e.IsDir() || skillWalkExclusions[e.Name()] {
				continue
			}
			if strings.HasPrefix(e.Name(), ".") && e.Name() != ".system" {
				continue
			}
			sub := filepath.Join(root.Path, e.Name())
			skillFile := filepath.Join(sub, "SKILL.md")
			if fi, err := os.Stat(skillFile); err == nil && !fi.IsDir() {
				consider(skillFile, sub)
			}
		}
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].Priority < skills[j].Priority })
	return skills, diagnostics
}

// parseFrontmatter extracts a leading `---` YAML block as flat key: value
// pairs (folded/literal block scalars supported for description values).
func parseFrontmatter(content string) (map[string]string, string) {
	fm := map[string]string{}
	body := content
	content = strings.TrimPrefix(content, "\ufeff")
	if !strings.HasPrefix(content, "---") {
		return fm, body
	}
	rest := content[3:]
	nl := strings.Index(rest, "\n")
	if nl < 0 {
		return fm, body
	}
	end := strings.Index(rest[nl+1:], "\n---")
	if end < 0 {
		return fm, body
	}
	block := rest[nl+1 : nl+1+end]
	body = strings.TrimSpace(rest[nl+1+end+4:])
	lines := strings.Split(strings.ReplaceAll(block, "\r\n", "\n"), "\n")
	var lastKey string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			// block scalar continuation
			if lastKey != "" {
				fm[lastKey] += " " + trimmed
			}
			continue
		}
		idx := strings.Index(trimmed, ":")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(trimmed[:idx])
		val := strings.TrimSpace(trimmed[idx+1:])
		val = strings.Trim(val, `"'`)
		if val == ">" || val == "|" || val == ">-" || val == "|-" || val == ">+" || val == "|+" {
			val = ""
		}
		fm[key] = val
		lastKey = key
	}
	return fm, body
}

// SkillsContextSection renders the discovery list injected near the Skill
// tool (spec §3.4), 20k char budget with names-only degradation.
func SkillsContextSection(skills []SkillEntry) string {
	if len(skills) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("The following skills are available for use with the Skill tool:\n\n")
	const budget = 20000
	size := 0
	degraded := false
	for _, s := range skills {
		name := s.Name
		if s.QualifiedName != "" {
			name = s.QualifiedName
		}
		line := fmt.Sprintf("- %s: %s", name, s.Description)
		if s.WhenToUse != "" {
			line += " - " + s.WhenToUse
		}
		if len(line) > 250 {
			line = line[:250]
		}
		if s.QualifiedName != "" {
			line += fmt.Sprintf(" (also loadable as %s)", s.Name)
		}
		line += fmt.Sprintf(" (file: %s)", s.Path)
		if size+len(line) > budget {
			degraded = true
			break
		}
		size += len(line)
		b.WriteString(line + "\n")
	}
	if degraded {
		b.Reset()
		b.WriteString("The following skills are available for use with the Skill tool:\n\n")
		for _, s := range skills {
			b.WriteString(fmt.Sprintf("- %s (file: %s)\n", s.Name, s.Path))
		}
	}
	return b.String()
}

// RenderSkillContent port of the Skill tool output envelope.
func RenderSkillContent(e SkillEntry, maxBytes int) (string, error) {
	data, err := os.ReadFile(e.Path)
	if err != nil {
		return "", err
	}
	_, body := parseFrontmatter(string(data))
	if maxBytes <= 0 {
		maxBytes = 100000
	}
	truncated := ""
	if len(body) > maxBytes {
		body = body[:maxBytes]
		truncated = "\n[Skill content truncated]"
	}
	body = strings.ReplaceAll(body, "${CLAUDE_SKILL_DIR}", e.Dir)
	body = strings.ReplaceAll(body, "${FORGE_SKILL_DIR}", e.Dir)
	content := fmt.Sprintf("<skill_content name=%q>\n# Skill: %s\n%s\nBase directory for this skill: %s%s\n</skill_content>",
		e.Name, e.Name, body, e.Dir, truncated)
	return content, nil
}
