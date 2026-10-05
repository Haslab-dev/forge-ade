package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// RecentEntry represents a recently opened workspace or folder.
type RecentEntry struct {
	Path        string    `json:"path"`
	Name        string    `json:"name"`
	IsWorkspace bool      `json:"isWorkspace"`
	LastOpened  time.Time `json:"lastOpened"`
	Pinned      bool      `json:"pinned"`
	Favorite    bool      `json:"favorite"`
}

// Manager manages workspace lifecycle and recent projects.
type Manager struct {
	mu         sync.RWMutex
	current    *Workspace
	recent     []RecentEntry
	dataDir    string
	recentPath string
}

// NewManager creates a new workspace manager.
func NewManager(dataDir string) (*Manager, error) {
	recentPath := filepath.Join(dataDir, "recent.json")

	m := &Manager{
		dataDir:    dataDir,
		recentPath: recentPath,
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	m.loadRecent()
	return m, nil
}

// OpenFolder opens a folder as a temporary workspace.
func (m *Manager) OpenFolder(folderPath string) (*Workspace, error) {
	absPath, err := filepath.Abs(folderPath)
	if err != nil {
		return nil, fmt.Errorf("resolve folder path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("access folder: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", absPath)
	}

	ws := NewTemporary(absPath)

	m.mu.Lock()
	m.current = ws
	m.mu.Unlock()

	m.addRecent(absPath, false)
	return ws, nil
}

// OpenWorkspace loads a .workspace file.
func (m *Manager) OpenWorkspace(filePath string) (*Workspace, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace path: %w", err)
	}

	ws, err := Load(absPath)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.current = ws
	m.mu.Unlock()

	m.addRecent(absPath, true)
	return ws, nil
}

// Current returns the current workspace.
func (m *Manager) Current() *Workspace {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

// IsOpen returns true if a workspace is currently open.
func (m *Manager) IsOpen() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current != nil
}

// SaveCurrent saves the current workspace.
func (m *Manager) SaveCurrent() error {
	m.mu.RLock()
	ws := m.current
	m.mu.RUnlock()

	if ws == nil {
		return fmt.Errorf("no workspace open")
	}
	return ws.Save()
}

// SaveCurrentAs saves the current workspace to a new file.
func (m *Manager) SaveCurrentAs(filePath string) error {
	m.mu.RLock()
	ws := m.current
	m.mu.RUnlock()

	if ws == nil {
		return fmt.Errorf("no workspace open")
	}
	return ws.SaveAs(filePath)
}

// Close closes the current workspace.
func (m *Manager) Close() {
	m.mu.Lock()
	m.current = nil
	m.mu.Unlock()
}

// GetRecent returns the lt of recent projects.
func (m *Manager) GetRecent() []RecentEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries := make([]RecentEntry, len(m.recent))
	copy(entries, m.recent)
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Pinned != entries[j].Pinned {
			return entries[i].Pinned
		}
		return entries[i].LastOpened.After(entries[j].LastOpened)
	})
	return entries
}

// PinRecent pins or unpins a recent entry.
func (m *Manager) PinRecent(path string, pinned bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.recent {
		if m.recent[i].Path == path {
			m.recent[i].Pinned = pinned
			m.saveRecentLocked()
			return
		}
	}
}

// FavoriteRecent marks a recent entry as favorite.
func (m *Manager) FavoriteRecent(path string, favorite bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.recent {
		if m.recent[i].Path == path {
			m.recent[i].Favorite = favorite
			m.saveRecentLocked()
			return
		}
	}
}

// RemoveRecent removes a recent entry.
func (m *Manager) RemoveRecent(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, entry := range m.recent {
		if entry.Path == path {
			m.recent = append(m.recent[:i], m.recent[i+1:]...)
			m.saveRecentLocked()
			return
		}
	}
}

func (m *Manager) addRecent(path string, isWorkspace bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Update or add
	for i, entry := range m.recent {
		if entry.Path == path {
			m.recent[i].LastOpened = time.Now()
			m.recent[i].IsWorkspace = isWorkspace
			m.saveRecentLocked()
			return
		}
	}

	name := filepath.Base(path)
	if ext := filepath.Ext(name); ext == ".workspace" {
		name = name[:len(name)-len(ext)]
	}

	// Prepend
	m.recent = append([]RecentEntry{{
		Path:        path,
		Name:        name,
		IsWorkspace: isWorkspace,
		LastOpened:  time.Now(),
	}}, m.recent...)

	// Keep only last 50
	if len(m.recent) > 50 {
		m.recent = m.recent[:50]
	}

	m.saveRecentLocked()
}

func (m *Manager) loadRecent() {
	data, err := os.ReadFile(m.recentPath)
	if err != nil {
		m.recent = []RecentEntry{}
		return
	}

	var entries []RecentEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		m.recent = []RecentEntry{}
		return
	}
	m.recent = entries
}

func (m *Manager) saveRecentLocked() {
	data, err := json.MarshalIndent(m.recent, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(m.recentPath, data, 0644)
}
