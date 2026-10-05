package agentsession

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// SettingsStore persists AppSettings in <dataDir>/app-settings.json. The
// default is Terminal Session mode; "agent-ui" is opt-in.
type SettingsStore struct {
	mu       sync.RWMutex
	path     string
	settings AppSettings
}

// NewSettingsStore loads (or initializes) the application settings store.
func NewSettingsStore(dataDir string) *SettingsStore {
	s := &SettingsStore{path: filepath.Join(dataDir, "app-settings.json"), settings: DefaultAppSettings()}
	s.load()
	return s
}

// legacyTerminalFont is the pre-token default font stack. A saved setting
// still holding it was never customized — adopt the current default instead.
const legacyTerminalFont = "'JetBrains Mono', 'Fira Code', Menlo, Monaco, monospace"

func (s *SettingsStore) load() {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		s.persist()
		return
	}
	var saved AppSettings
	if err := json.Unmarshal(raw, &saved); err != nil {
		log.Printf("agentsession: parse %s: %v", s.path, err)
		return
	}
	// Field-wise merge over defaults so newly added settings keep sane values.
	if saved.DefaultMode == "terminal" || saved.DefaultMode == "agent-ui" {
		s.settings.DefaultMode = saved.DefaultMode
	}
	term := saved.Terminal
	if term.Shell != "" {
		s.settings.Terminal.Shell = term.Shell
	}
	if term.FontFamily != "" && term.FontFamily != legacyTerminalFont {
		s.settings.Terminal.FontFamily = term.FontFamily
	}
	if term.FontSize > 0 {
		s.settings.Terminal.FontSize = term.FontSize
	}
	switch term.CursorStyle {
	case "block", "bar", "underline":
		s.settings.Terminal.CursorStyle = term.CursorStyle
	}
	if term.Scrollback >= 0 {
		s.settings.Terminal.Scrollback = term.Scrollback
	}
	// CursorBlink is a plain bool — present means set.
	if term.CursorBlink != DefaultTerminalSettings().CursorBlink {
		s.settings.Terminal.CursorBlink = term.CursorBlink
	}
}

func (s *SettingsStore) persist() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s.settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0644)
}

// Get returns a copy of the current settings.
func (s *SettingsStore) Get() AppSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

// Save validates and persists new settings.
func (s *SettingsStore) Save(settings AppSettings) (AppSettings, error) {
	if settings.DefaultMode != "terminal" && settings.DefaultMode != "agent-ui" {
		settings.DefaultMode = "terminal"
	}
	if settings.Terminal.FontSize <= 0 {
		settings.Terminal.FontSize = DefaultTerminalSettings().FontSize
	}
	if settings.Terminal.Scrollback < 0 {
		settings.Terminal.Scrollback = DefaultTerminalSettings().Scrollback
	}
	switch settings.Terminal.CursorStyle {
	case "block", "bar", "underline":
	default:
		settings.Terminal.CursorStyle = DefaultTerminalSettings().CursorStyle
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings = settings
	if err := s.persist(); err != nil {
		return s.settings, err
	}
	return s.settings, nil
}
