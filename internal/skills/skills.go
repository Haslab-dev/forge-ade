package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

var validSkillNameRegex = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type SkillScope string

const (
	ScopeWorkspace SkillScope = "workspace"
	ScopeGlobal    SkillScope = "global"
	ScopePlugin    SkillScope = "plugin"
	ScopeCustom    SkillScope = "custom"
)

type Skill struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Path        string     `json:"path"`
	Body        string     `json:"body"`
	Scope       SkillScope `json:"scope,omitempty"`
	Scripts     []string   `json:"scripts,omitempty"`
}

type CreateSkillRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Body        string            `json:"body"`
	Scope       string            `json:"scope,omitempty"` // "workspace" or "global"
	Scripts     map[string]string `json:"scripts,omitempty"` // filename -> content
}

type Manager struct {
	mu           sync.RWMutex
	skills       map[string]Skill
	pluginSkills map[string]Skill
	workspaceDir string
	globalDir    string
}

func NewManager() *Manager {
	home, _ := os.UserHomeDir()
	globalDir := filepath.Join(home, ".forge-ade", "skills")
	_ = os.MkdirAll(globalDir, 0755)

	m := &Manager{
		skills:       make(map[string]Skill),
		pluginSkills: make(map[string]Skill),
		globalDir:    globalDir,
	}
	m.Reload()
	return m
}

func (m *Manager) SetWorkspace(dir string) {
	m.mu.Lock()
	m.workspaceDir = dir
	m.mu.Unlock()
	m.Reload()
}

func (m *Manager) RegisterPluginSkills(pluginID string, skills []Skill) {
	m.mu.Lock()
	for _, s := range skills {
		s.Scope = ScopePlugin
		m.pluginSkills[s.Name] = s
		m.skills[s.Name] = s
	}
	m.mu.Unlock()
}

func (m *Manager) Reload() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.skills = make(map[string]Skill)

	// Retain plugin skills
	for k, v := range m.pluginSkills {
		m.skills[k] = v
	}

	// 1. Global skills (~/.forge-ade/skills)
	m.loadFromDir(m.globalDir, ScopeGlobal)

	// 2. Additional global paths (~/.agents/skills, ~/.dsh/skills)
	if home, err := os.UserHomeDir(); err == nil {
		m.loadFromDir(filepath.Join(home, ".agents", "skills"), ScopeGlobal)
		m.loadFromDir(filepath.Join(home, ".dsh", "skills"), ScopeGlobal)
	}

	// 3. Workspace skills (highest priority — overwrites global)
	ws := m.workspaceDir
	if ws == "" {
		ws, _ = os.Getwd()
	}
	if ws != "" {
		m.loadFromDir(filepath.Join(ws, ".forge", "skills"), ScopeWorkspace)
		m.loadFromDir(filepath.Join(ws, ".agents", "skills"), ScopeWorkspace)
		m.loadFromDir(filepath.Join(ws, ".dsh", "skills"), ScopeWorkspace)
	}
}

func (m *Manager) loadFromDir(dir string, scope SkillScope) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		// Bundle directory: <name>/SKILL.md
		if entry.IsDir() {
			skillFile := filepath.Join(dir, entry.Name(), "SKILL.md")
			data, err := os.ReadFile(skillFile)
			if err == nil {
				skill := parseSkill(entry.Name(), skillFile, string(data), scope)
				// Look for any scripts in <name>/scripts or <name>/
				scriptsDir := filepath.Join(dir, entry.Name(), "scripts")
				if scriptEntries, err := os.ReadDir(scriptsDir); err == nil {
					for _, se := range scriptEntries {
						if !se.IsDir() {
							skill.Scripts = append(skill.Scripts, filepath.Join(scriptsDir, se.Name()))
						}
					}
				}
				m.skills[skill.Name] = skill
			}
		} else if strings.HasSuffix(entry.Name(), ".md") && entry.Name() != "README.md" {
			// Flat skill file: <name>.md
			nameWithoutExt := strings.TrimSuffix(entry.Name(), ".md")
			filePath := filepath.Join(dir, entry.Name())
			data, err := os.ReadFile(filePath)
			if err == nil {
				skill := parseSkill(nameWithoutExt, filePath, string(data), scope)
				m.skills[skill.Name] = skill
			}
		}
	}
}

func parseSkill(dirName, path, content string, scope SkillScope) Skill {
	lines := strings.Split(content, "\n")
	name := dirName
	desc := ""
	inFrontmatter := false
	bodyLines := []string{}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			inFrontmatter = !inFrontmatter
			continue
		}
		if inFrontmatter {
			if strings.HasPrefix(trimmed, "name:") {
				name = strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
				name = strings.Trim(name, `"'`)
			} else if strings.HasPrefix(trimmed, "description:") {
				desc = strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))
				desc = strings.Trim(desc, `"'`)
			}
		} else {
			bodyLines = append(bodyLines, line)
		}
	}

	return Skill{
		Name:        name,
		Description: desc,
		Path:        path,
		Body:        strings.TrimSpace(strings.Join(bodyLines, "\n")),
		Scope:       scope,
	}
}

func (m *Manager) List() []Skill {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]Skill, 0, len(m.skills))
	for _, s := range m.skills {
		list = append(list, s)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})
	return list
}

func (m *Manager) Get(name string) (Skill, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.skills[name]
	return s, ok
}

func (m *Manager) GetByPath(path string) (Skill, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, s := range m.skills {
		if s.Path == path {
			return s, true
		}
	}
	return Skill{}, false
}

func (s *Skill) BaseDir() string {
	return filepath.Dir(s.Path)
}

// CreateSkill creates a new skill directory with SKILL.md and any scripts.
func (m *Manager) CreateSkill(req CreateSkillRequest, activeWorkspace string) (*Skill, error) {
	req.Name = strings.TrimSpace(strings.ToLower(req.Name))
	if req.Name == "" {
		return nil, fmt.Errorf("skill name cannot be empty")
	}
	if !validSkillNameRegex.MatchString(req.Name) {
		return nil, fmt.Errorf("skill name must be kebab-case (e.g. 'code-review', 'unit-test-guide')")
	}

	ws := activeWorkspace
	if ws == "" {
		m.mu.RLock()
		ws = m.workspaceDir
		m.mu.RUnlock()
	}

	scope := req.Scope
	if scope == "" {
		scope = "workspace"
	}

	var targetDir string
	var skillScope SkillScope

	if scope == "workspace" && ws != "" {
		targetDir = filepath.Join(ws, ".forge", "skills", req.Name)
		skillScope = ScopeWorkspace
	} else {
		targetDir = filepath.Join(m.globalDir, req.Name)
		skillScope = ScopeGlobal
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create skill directory %s: %w", targetDir, err)
	}

	// Write scripts
	var scriptsPaths []string
	if len(req.Scripts) > 0 {
		scriptsDir := filepath.Join(targetDir, "scripts")
		_ = os.MkdirAll(scriptsDir, 0755)
		for filename, content := range req.Scripts {
			scriptFile := filepath.Join(scriptsDir, filename)
			if err := os.WriteFile(scriptFile, []byte(content), 0755); err != nil {
				return nil, fmt.Errorf("failed to write script %s: %w", filename, err)
			}
			scriptsPaths = append(scriptsPaths, scriptFile)
		}
	}

	// Build SKILL.md content
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("name: %s\n", req.Name))
	if req.Description != "" {
		sb.WriteString(fmt.Sprintf("description: %s\n", req.Description))
	}
	sb.WriteString("---\n\n")
	sb.WriteString(strings.TrimSpace(req.Body))
	sb.WriteString("\n")

	skillPath := filepath.Join(targetDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte(sb.String()), 0644); err != nil {
		return nil, fmt.Errorf("failed to write SKILL.md to %s: %w", skillPath, err)
	}

	skill := Skill{
		Name:        req.Name,
		Description: req.Description,
		Path:        skillPath,
		Body:        strings.TrimSpace(req.Body),
		Scope:       skillScope,
		Scripts:     scriptsPaths,
	}

	m.mu.Lock()
	m.skills[req.Name] = skill
	m.mu.Unlock()

	return &skill, nil
}

// DeleteSkill deletes a skill from disk and unregisters it.
func (m *Manager) DeleteSkill(name string) error {
	m.mu.Lock()
	skill, ok := m.skills[name]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("skill %s not found", name)
	}
	delete(m.skills, name)
	m.mu.Unlock()

	if skill.Path != "" {
		baseDir := filepath.Dir(skill.Path)
		// If it's a directory bundle <name>/SKILL.md, remove the folder
		if filepath.Base(baseDir) == name {
			_ = os.RemoveAll(baseDir)
		} else {
			_ = os.Remove(skill.Path)
		}
	}

	return nil
}

// CatalogMarkdown returns the model-facing compact skill catalog (like dsh-fork).
// Only emits name and description, saving tokens while letting model invoke the loader.
func (m *Manager) CatalogMarkdown() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.skills) == 0 {
		return ""
	}

	skills := make([]Skill, 0, len(m.skills))
	for _, s := range m.skills {
		skills = append(skills, s)
	}
	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})

	var sb strings.Builder
	sb.WriteString("\n## Available skills (expert task playbooks)\n")
	sb.WriteString("When a user task names or matches one of these domains, call the `skill` or `load_skill` tool with the exact name to load its complete instructions before acting.\n")
	sb.WriteString("<available_skills>\n")
	for _, s := range skills {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", s.Name, s.Description))
	}
	sb.WriteString("</available_skills>\n")
	return sb.String()
}

// InvocationMessage renders the skill body as a user message to inject into the
// conversation (user-invocation form).
func (s *Skill) InvocationMessage(args string) string {
	var sb strings.Builder
	sb.WriteString("The user invoked the skill `")
	sb.WriteString(s.Name)
	sb.WriteString("`.")
	sb.WriteString("\nSkill directory: ")
	sb.WriteString(s.BaseDir())
	if strings.TrimSpace(args) != "" {
		sb.WriteString("\nUser arguments: ")
		sb.WriteString(strings.TrimSpace(args))
	}
	sb.WriteString("\n\nSkill instructions (SKILL.md):\n")
	sb.WriteString("```\n")
	sb.WriteString(s.Body)
	sb.WriteString("\n```\n")
	sb.WriteString("\nFollow these instructions to complete the user's request. You may read or run any")
	sb.WriteString(" scripts in the skill directory above to help.")
	return sb.String()
}
