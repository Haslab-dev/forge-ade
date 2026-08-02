package skills

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
	Body        string `json:"body"`
}

type Manager struct {
	mu     sync.RWMutex
	skills map[string]Skill
}

func NewManager() *Manager {
	m := &Manager{
		skills: make(map[string]Skill),
	}
	m.Reload()
	return m
}

func (m *Manager) Reload() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.skills = make(map[string]Skill)

	// 1. Global skills (~/.forge-ade/skills)
	if home, err := os.UserHomeDir(); err == nil {
		globalDir := filepath.Join(home, ".forge-ade", "skills")
		m.loadFromDir(globalDir)
	}

	// 2. Current workspace skills (.agents/skills)
	if cwd, err := os.Getwd(); err == nil {
		wsDir := filepath.Join(cwd, ".agents", "skills")
		m.loadFromDir(wsDir)
	}
}

func (m *Manager) loadFromDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			skillFile := filepath.Join(dir, entry.Name(), "SKILL.md")
			data, err := os.ReadFile(skillFile)
			if err == nil {
				skill := parseSkill(entry.Name(), skillFile, string(data))
				m.skills[skill.Name] = skill
			}
		}
	}
}

func parseSkill(dirName, path, content string) Skill {
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
			} else if strings.HasPrefix(trimmed, "description:") {
				desc = strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))
			}
		} else {
			bodyLines = append(bodyLines, line)
		}
	}

	return Skill{
		Name:        name,
		Description: desc,
		Path:        path,
		Body:        strings.Join(bodyLines, "\n"),
	}
}

func (m *Manager) List() []Skill {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]Skill, 0, len(m.skills))
	for _, s := range m.skills {
		list = append(list, s)
	}
	return list
}

func (m *Manager) Get(name string) (Skill, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.skills[name]
	return s, ok
}

// GetByPath looks up a skill by its SKILL.md file path (used for invocation
// routing when the caller only has a path).
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

// BaseDir returns the directory containing the skill's SKILL.md (where its
// scripts/ and templates/ live), so the model can resolve relative paths.
func (s *Skill) BaseDir() string {
	return filepath.Dir(s.Path)
}

// InvocationMessage renders the skill body as a user message to inject into the
// conversation (user-invocation form)
// form). The model sees the skill's full instructions + its base dir and can
// resolve scripts/templates relative to it.
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
