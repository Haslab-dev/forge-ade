package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// Version is the current workspace file format version.
const Version = 1

// Workspace represents an entire development environment.
type Workspace struct {
	mu sync.RWMutex `json:"-"`

	// FilePath is the path to the .workspace file (empty for temporary workspaces).
	FilePath string `yaml:"-" json:"filePath,omitempty"`

	// Version of the workspace file format.
	Version int `yaml:"version" json:"version"`

	// Name is the display name of the workspace.
	Name string `yaml:"name" json:"name"`

	// Folders is the list of directory paths in the workspace.
	Folders []string `yaml:"folders" json:"folders"`

	// Settings contains workspace-level settings.
	Settings Settings `yaml:"settings" json:"settings"`

	// Git configuration.
	Git GitConfig `yaml:"git" json:"git"`

	// Agents configuration.
	Agents map[string]AgentConfig `yaml:"agents" json:"agents"`

	// Terminals configuration.
	Terminals map[string]TerminalConfig `yaml:"terminals" json:"terminals"`

	// IsTemporary is true if this workspace was created from "Open Folder".
	IsTemporary bool `yaml:"isTemporary" json:"isTemporary"`
}

// Settings contains workspace-level settings.
type Settings struct {
	Theme string `yaml:"theme" json:"theme"`
}

// GitConfig contains Git-related workspace settings.
type GitConfig struct {
	AutoFetch bool `yaml:"autoFetch" json:"autoFetch"`
}

// AgentConfig configures an AI agent for the workspace.
type AgentConfig struct {
	Provider string `yaml:"provider" json:"provider"`
	Model    string `yaml:"model,omitempty" json:"model,omitempty"`
}

// TerminalConfig configures a terminal session.
type TerminalConfig struct {
	Shell string `yaml:"shell" json:"shell"`
	CWD   string `yaml:"cwd,omitempty" json:"cwd,omitempty"`
}

// NewTemporary creates a temporary workspace from a single folder.
func NewTemporary(folderPath string) *Workspace {
	absPath, _ := filepath.Abs(folderPath)
	name := filepath.Base(absPath)

	return &Workspace{
		Version:     Version,
		Name:        name,
		Folders:     []string{absPath},
		IsTemporary: true,
		Settings: Settings{
			Theme: "dark",
		},
		Git: GitConfig{
			AutoFetch: true,
		},
		Agents:    make(map[string]AgentConfig),
		Terminals: make(map[string]TerminalConfig),
	}
}

// New creates a new workspace with the given name and folders.
func New(name string, folders []string) *Workspace {
	return &Workspace{
		Version:     Version,
		Name:        name,
		Folders:     folders,
		IsTemporary: true,
		Settings: Settings{
			Theme: "dark",
		},
		Git: GitConfig{
			AutoFetch: true,
		},
		Agents:    make(map[string]AgentConfig),
		Terminals: make(map[string]TerminalConfig),
	}
}

// Load reads a workspace from a .workspace YAML file.
func Load(filePath string) (*Workspace, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read workspace file: %w", err)
	}

	var ws Workspace
	if err := yaml.Unmarshal(data, &ws); err != nil {
		return nil, fmt.Errorf("parse workspace file: %w", err)
	}

	ws.FilePath = filePath
	ws.IsTemporary = false

	// Resolve relative folder paths to absolute
	for i, folder := range ws.Folders {
		if !filepath.IsAbs(folder) {
			abs := filepath.Join(filepath.Dir(filePath), folder)
			ws.Folders[i] = filepath.Clean(abs)
		}
	}

	if ws.Agents == nil {
		ws.Agents = make(map[string]AgentConfig)
	}
	if ws.Terminals == nil {
		ws.Terminals = make(map[string]TerminalConfig)
	}

	return &ws, nil
}

// Save writes the workspace to its file path.
func (w *Workspace) Save() error {
	w.mu.RLock()
	filePath := w.FilePath
	w.mu.RUnlock()

	if filePath == "" {
		return fmt.Errorf("workspace: no file path set")
	}

	w.mu.RLock()
	// Convert absolute folder paths to relative for saving
	baseDir := filepath.Dir(filePath)
	relFolders := make([]string, len(w.Folders))
	for i, folder := range w.Folders {
		rel, err := filepath.Rel(baseDir, folder)
		if err != nil {
			relFolders[i] = folder
		} else {
			relFolders[i] = "./" + rel
		}
	}
	w.mu.RUnlock()

	w.mu.Lock()
	origFolders := w.Folders
	w.Folders = relFolders
	data, err := yaml.Marshal(w)
	w.Folders = origFolders
	w.mu.Unlock()

	if err != nil {
		return fmt.Errorf("marshal workspace: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("write workspace file: %w", err)
	}

	return nil
}

// SaveAs writes the workspace to a new file path and sets it as the current path.
func (w *Workspace) SaveAs(filePath string) error {
	w.mu.Lock()
	w.FilePath = filePath
	w.IsTemporary = false
	w.mu.Unlock()
	return w.Save()
}

// AddFolder adds a folder to the workspace.
func (w *Workspace) AddFolder(folderPath string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	absPath, _ := filepath.Abs(folderPath)
	for _, f := range w.Folders {
		if f == absPath {
			return
		}
	}
	w.Folders = append(w.Folders, absPath)
}

// RemoveFolder removes a folder from the workspace.
func (w *Workspace) RemoveFolder(folderPath string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	absPath, _ := filepath.Abs(folderPath)
	for i, f := range w.Folders {
		if f == absPath {
			w.Folders = append(w.Folders[:i], w.Folders[i+1:]...)
			return true
		}
	}
	return false
}

// GetFolders returns a copy of the workspace folder list.
func (w *Workspace) GetFolders() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	folders := make([]string, len(w.Folders))
	copy(folders, w.Folders)
	return folders
}

// SetName sets the workspace name.
func (w *Workspace) SetName(name string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Name = name
}

// GetName returns the workspace name.
func (w *Workspace) GetName() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.Name
}
