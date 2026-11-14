package ui

// prefs.go persists the UI preferences that the wails version kept in
// localStorage: theme, layout sizes/visibility, follow-agent, view modes,
// filter state. Stored as ~/.forge-ade/ui-prefs.json.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hasdev/forge-ade/app/native/pkg/theme"
)

// uiPrefs is the persisted shape.
type uiPrefs struct {
	Appearance           string  `json:"appearance"`
	PaletteID            string  `json:"palette_id"`
	ChatWidth            string  `json:"chat_width"`
	DiffScheme           string  `json:"diff_scheme"`
	ShowLeftSidebar      bool    `json:"show_left_sidebar"`
	ShowRightDrawer      bool    `json:"show_right_drawer"`
	ShowBottomTerminal   bool    `json:"show_bottom_terminal"`
	LeftSidebarWidth     float32 `json:"left_sidebar_width"`
	RightDrawerWidth     float32 `json:"right_drawer_width"`
	BottomTerminalHeight float32 `json:"bottom_terminal_height"`
	FollowAgent          bool    `json:"follow_agent"`
	AgentViewMode        string  `json:"agent_view_mode"`
	LeftSidebarView      string  `json:"left_sidebar_view"`
	ProjectFilter        string  `json:"project_filter"`
	TaskSortBy           string  `json:"task_sort_by"`
}

func prefsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".forge-ade", "ui-prefs.json")
}

// loadUIPrefs applies persisted preferences onto the app state + theme.
func (a *AppUI) loadUIPrefs() {
	path := prefsPath()
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var p uiPrefs
	if err := json.Unmarshal(data, &p); err != nil {
		return
	}

	if a.ThemeMgr != nil {
		switch p.Appearance {
		case "dark", "light":
			a.ThemeMgr.Appearance = p.Appearance
		}
		if theme.IsKnownPalette(p.PaletteID) {
			a.ThemeMgr.PaletteID = p.PaletteID
		}
		switch p.ChatWidth {
		case "comfortable", "wide", "full":
			a.ThemeMgr.ChatWidth = p.ChatWidth
		}
		switch p.DiffScheme {
		case "classic", "modern":
			a.ThemeMgr.DiffScheme = p.DiffScheme
		}
	}

	// Layout: restore sizes only within sane clamps so a corrupt file never
	// breaks the layout.
	if p.LeftSidebarWidth >= 200 && p.LeftSidebarWidth <= 600 {
		a.LeftSidebarWidth = p.LeftSidebarWidth
	}
	if p.RightDrawerWidth >= 280 && p.RightDrawerWidth <= 900 {
		a.RightDrawerWidth = p.RightDrawerWidth
	}
	if p.BottomTerminalHeight >= 120 && p.BottomTerminalHeight <= 600 {
		a.BottomTerminalHeight = p.BottomTerminalHeight
	}
	if p.ShowLeftSidebar != a.ShowLeftSidebar {
		a.ShowLeftSidebar = p.ShowLeftSidebar
	}
	a.ShowRightDrawer = p.ShowRightDrawer
	a.ShowBottomTerminal = p.ShowBottomTerminal
	a.FollowAgent = p.FollowAgent
	switch p.AgentViewMode {
	case AgentViewChat, AgentViewBoard, AgentViewGit:
		a.AgentViewMode = p.AgentViewMode
	}
	switch p.LeftSidebarView {
	case "tasks", "files":
		a.LeftSidebarView = p.LeftSidebarView
	}
	if p.ProjectFilter != "" {
		a.ProjectFilter = p.ProjectFilter
	}
	switch p.TaskSortBy {
	case "updated", "created":
		a.TaskSortBy = p.TaskSortBy
	}
}

// SaveUIPrefs snapshots the current preferences to disk (best effort).
func (a *AppUI) SaveUIPrefs() {
	path := prefsPath()
	if path == "" {
		return
	}
	p := uiPrefs{
		Appearance:           a.ThemeMgr.Appearance,
		PaletteID:            a.ThemeMgr.PaletteID,
		ChatWidth:            a.ThemeMgr.ChatWidth,
		DiffScheme:           a.ThemeMgr.DiffScheme,
		ShowLeftSidebar:      a.ShowLeftSidebar,
		ShowRightDrawer:      a.ShowRightDrawer,
		ShowBottomTerminal:   a.ShowBottomTerminal,
		LeftSidebarWidth:     a.LeftSidebarWidth,
		RightDrawerWidth:     a.RightDrawerWidth,
		BottomTerminalHeight: a.BottomTerminalHeight,
		FollowAgent:          a.FollowAgent,
		AgentViewMode:        a.AgentViewMode,
		LeftSidebarView:      a.LeftSidebarView,
		ProjectFilter:        a.ProjectFilter,
		TaskSortBy:           a.TaskSortBy,
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, data, 0o644)
}

var _ = theme.ThemePaletteDefault

// prefsSnapshot renders a compact signature of the persisted fields so the
// periodic saver can detect changes without writing every second.
func (a *AppUI) prefsSnapshot() string {
	f := func(b bool) string {
		if b {
			return "1"
		}
		return "0"
	}
	f32 := func(v float32) string { return fmt.Sprintf("%.0f", v) }
	return prefsSnap{
		a.ThemeMgr.Appearance, a.ThemeMgr.PaletteID, a.ThemeMgr.ChatWidth, a.ThemeMgr.DiffScheme,
		f(a.ShowLeftSidebar), f(a.ShowRightDrawer), f(a.ShowBottomTerminal),
		f32(a.LeftSidebarWidth), f32(a.RightDrawerWidth), f32(a.BottomTerminalHeight),
		f(a.FollowAgent), a.AgentViewMode, a.LeftSidebarView, a.ProjectFilter, a.TaskSortBy,
	}.String()
}

type prefsSnap [15]string

func (p prefsSnap) String() string {
	return strings.Join(p[:], "|")
}
