package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Entry represents a single memory item persisted across agent turns.
type Entry struct {
	ID        string    `json:"id"`
	Key       string    `json:"key"`
	Content   string    `json:"content"`
	Category  string    `json:"category,omitempty"` // "project", "architecture", "preference", "rule"
	Scope     string    `json:"scope,omitempty"`    // "workspace", "global"
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Manager manages workspace and global persistent memory for agents.
type Manager struct {
	mu          sync.RWMutex
	workspaceDir string
	globalDir    string
	entries     map[string]Entry
}

// New creates a new memory Manager.
func New(workspaceDir string) *Manager {
	home, _ := os.UserHomeDir()
	globalDir := filepath.Join(home, ".forge-ade")

	m := &Manager{
		workspaceDir: workspaceDir,
		globalDir:    globalDir,
		entries:      make(map[string]Entry),
	}
	m.Reload()
	return m
}

// SetWorkspace updates the active workspace directory and reloads memories.
func (m *Manager) SetWorkspace(dir string) {
	m.mu.Lock()
	m.workspaceDir = dir
	m.mu.Unlock()
	m.Reload()
}

// Reload loads all memories from workspace (.forge/memory.json) and global (~/.forge-ade/memory.json).
func (m *Manager) Reload() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.entries = make(map[string]Entry)

	// 1. Global memories
	if m.globalDir != "" {
		m.loadFromDir(m.globalDir, "global")
	}

	// 2. Workspace memories (takes precedence)
	if m.workspaceDir != "" {
		wsForge := filepath.Join(m.workspaceDir, ".forge")
		m.loadFromDir(wsForge, "workspace")
	}
}

func (m *Manager) loadFromDir(dir string, scope string) {
	jsonFile := filepath.Join(dir, "memory.json")
	if data, err := os.ReadFile(jsonFile); err == nil {
		var list []Entry
		if err := json.Unmarshal(data, &list); err == nil {
			for _, e := range list {
				if e.ID == "" {
					e.ID = e.Key
				}
				if e.Scope == "" {
					e.Scope = scope
				}
				m.entries[e.ID] = e
			}
		}
	} else {
		// Fallback: check MEMORY.md if memory.json doesn't exist
		mdFile := filepath.Join(dir, "MEMORY.md")
		if mdData, err := os.ReadFile(mdFile); err == nil && len(mdData) > 0 {
			content := strings.TrimSpace(string(mdData))
			if content != "" {
				id := fmt.Sprintf("%s-general", scope)
				m.entries[id] = Entry{
					ID:        id,
					Key:       "project_guidelines",
					Content:   content,
					Category:  "project",
					Scope:     scope,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}
			}
		}
	}
}

// List returns all registered memory entries.
func (m *Manager) List() []Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]Entry, 0, len(m.entries))
	for _, e := range m.entries {
		list = append(list, e)
	}
	return list
}

// Search returns entries matching the given query string.
func (m *Manager) Search(query string) []Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return m.List()
	}

	var results []Entry
	for _, e := range m.entries {
		if strings.Contains(strings.ToLower(e.Key), q) ||
			strings.Contains(strings.ToLower(e.Content), q) ||
			strings.Contains(strings.ToLower(e.Category), q) {
			results = append(results, e)
		}
	}
	return results
}

// Save stores or updates a memory entry and persists it to disk.
func (m *Manager) Save(e Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if e.Key == "" {
		return fmt.Errorf("memory key cannot be empty")
	}
	if e.ID == "" {
		e.ID = strings.ToLower(strings.ReplaceAll(e.Key, " ", "-"))
	}
	if e.Category == "" {
		e.Category = "project"
	}
	if e.Scope != "global" {
		e.Scope = "workspace"
	}
	now := time.Now()
	if e.CreatedAt.IsZero() {
		e.CreatedAt = now
	}
	e.UpdatedAt = now

	m.entries[e.ID] = e

	return m.persistLocked(e.Scope)
}

// Delete removes a memory entry by ID or Key.
func (m *Manager) Delete(idOrKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var targetScope string
	for id, e := range m.entries {
		if id == idOrKey || e.Key == idOrKey {
			targetScope = e.Scope
			delete(m.entries, id)
			break
		}
	}

	if targetScope != "" {
		return m.persistLocked(targetScope)
	}
	return nil
}

func (m *Manager) persistLocked(scope string) error {
	var targetDir string
	if scope == "global" {
		targetDir = m.globalDir
	} else {
		if m.workspaceDir == "" {
			return nil
		}
		targetDir = filepath.Join(m.workspaceDir, ".forge")
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	var scopeList []Entry
	for _, e := range m.entries {
		if e.Scope == scope {
			scopeList = append(scopeList, e)
		}
	}

	data, err := json.MarshalIndent(scopeList, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(targetDir, "memory.json"), data, 0644); err != nil {
		return err
	}

	// Also generate human-readable MEMORY.md
	var md strings.Builder
	md.WriteString("# Persistent Memory\n\n")
	md.WriteString("This file is automatically maintained by Forge ADE agent memory harness.\n\n")
	for _, e := range scopeList {
		md.WriteString(fmt.Sprintf("## %s (%s)\n", e.Key, e.Category))
		md.WriteString(e.Content)
		md.WriteString("\n\n")
	}
	_ = os.WriteFile(filepath.Join(targetDir, "MEMORY.md"), []byte(md.String()), 0644)

	return nil
}

// FormatPrompt constructs the <project_memory> block for the agent system prompt.
func (m *Manager) FormatPrompt() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.entries) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n<project_memory>\n")
	sb.WriteString("The following long-term memories and project conventions have been persisted across sessions:\n")
	for _, e := range m.entries {
		sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", e.Category, e.Key, e.Content))
	}
	sb.WriteString("</project_memory>\n")
	return sb.String()
}
