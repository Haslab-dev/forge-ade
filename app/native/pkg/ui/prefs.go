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
	Appearance           string   `json:"appearance"`
	PaletteID            string   `json:"palette_id"`
	GlassEnabled         bool     `json:"glass_enabled,omitempty"`
	GlassOpacity         *float32 `json:"glass_opacity,omitempty"`
	BackgroundKind       string   `json:"background_kind,omitempty"`
	BackgroundPreset     string   `json:"background_preset,omitempty"`
	BackgroundImage      string   `json:"background_image,omitempty"`
	BackgroundOpacity    *float32 `json:"background_opacity,omitempty"`
	BackgroundBlur       float32  `json:"background_blur,omitempty"`
	ChatWidth            string   `json:"chat_width"`
	DiffScheme           string   `json:"diff_scheme"`
	ShowLeftSidebar      bool     `json:"show_left_sidebar"`
	ShowRightDrawer      bool     `json:"show_right_drawer"`
	ShowBottomTerminal   bool     `json:"show_bottom_terminal"`
	LeftSidebarWidth     float32  `json:"left_sidebar_width"`
	RightDrawerWidth     float32  `json:"right_drawer_width"`
	BottomTerminalHeight float32  `json:"bottom_terminal_height"`
	FollowAgent          bool     `json:"follow_agent"`
	LeftSidebarView      string   `json:"left_sidebar_view"`
	SidebarListView      string   `json:"sidebar_list_view"`
	ProjectFilter        string   `json:"project_filter"`
	TaskSortBy           string   `json:"task_sort_by"`
	ProjectOrder         []string `json:"project_order"` // custom sidebar project-group order
}

func (a *AppUI) prefsPath() string {
	if a != nil && a.Engine != nil && a.Engine.DataDir != "" {
		return filepath.Join(a.Engine.DataDir, "ui-prefs.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".forge-ade", "ui-prefs.json")
}

// loadUIPrefs applies persisted preferences onto the app state + theme.
func (a *AppUI) loadUIPrefs() {
	path := a.prefsPath()
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
		a.ThemeMgr.GlassEnabled = p.GlassEnabled
		if p.GlassOpacity != nil {
			a.ThemeMgr.GlassOpacity = *p.GlassOpacity
			a.ThemeMgr.GlassOpacitySet = true
		}
		switch p.BackgroundKind {
		case theme.BackgroundKindNone, theme.BackgroundKindPreset, theme.BackgroundKindImage:
			a.ThemeMgr.BackgroundKind = p.BackgroundKind
		}
		if _, ok := theme.LookupBackgroundPreset(p.BackgroundPreset); ok {
			a.ThemeMgr.BackgroundPreset = p.BackgroundPreset
		}
		if p.BackgroundImage != "" {
			a.ThemeMgr.BackgroundImage = p.BackgroundImage
		}
		if p.BackgroundOpacity != nil {
			a.ThemeMgr.BackgroundOpacity = *p.BackgroundOpacity
			a.ThemeMgr.BackgroundOpacitySet = true
		}
		if p.BackgroundBlur >= 0 {
			a.ThemeMgr.BackgroundBlur = p.BackgroundBlur
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
	// Terminal dock starts closed on app launch (deliberately not auto-opened on startup).
	a.ShowBottomTerminal = false
	a.FollowAgent = p.FollowAgent
	// Intentional reset: the app always opens in chat view; agent_view_mode
	// is deliberately not persisted.
	a.AgentViewMode = AgentViewChat
	switch p.LeftSidebarView {
	case "tasks", "files":
		a.LeftSidebarView = p.LeftSidebarView
	}
	switch p.SidebarListView {
	case "project", "timeline":
		a.SidebarListView = p.SidebarListView
	}
	if p.ProjectFilter != "" {
		a.ProjectFilter = p.ProjectFilter
	}
	switch p.TaskSortBy {
	case "updated", "created":
		a.TaskSortBy = p.TaskSortBy
	}
	// Custom project-group order survives restarts; "General" is pinned by
	// the renderer regardless of what the stored list contains.
	if len(p.ProjectOrder) > 0 {
		a.ProjectOrder = p.ProjectOrder
	}
}

// SaveUIPrefs snapshots the current preferences to disk (best effort).
func (a *AppUI) SaveUIPrefs() {
	path := a.prefsPath()
	if path == "" {
		return
	}
	glassOp := a.ThemeMgr.EffectiveGlassOpacity()
	bgOp := a.ThemeMgr.EffectiveBackgroundOpacity()
	p := uiPrefs{
		Appearance:           a.ThemeMgr.Appearance,
		PaletteID:            a.ThemeMgr.PaletteID,
		GlassEnabled:         a.ThemeMgr.GlassEnabled,
		GlassOpacity:         &glassOp,
		BackgroundKind:       a.ThemeMgr.BackgroundKind,
		BackgroundPreset:     a.ThemeMgr.BackgroundPreset,
		BackgroundImage:      a.ThemeMgr.BackgroundImage,
		BackgroundOpacity:    &bgOp,
		BackgroundBlur:       a.ThemeMgr.EffectiveBackgroundBlur(),
		ChatWidth:            a.ThemeMgr.ChatWidth,
		DiffScheme:           a.ThemeMgr.DiffScheme,
		ShowLeftSidebar:      a.ShowLeftSidebar,
		ShowRightDrawer:      a.ShowRightDrawer,
		ShowBottomTerminal:   a.ShowBottomTerminal,
		LeftSidebarWidth:     a.LeftSidebarWidth,
		RightDrawerWidth:     a.RightDrawerWidth,
		BottomTerminalHeight: a.BottomTerminalHeight,
		FollowAgent:          a.FollowAgent,
		LeftSidebarView:      a.LeftSidebarView,
		SidebarListView:      a.SidebarListView,
		ProjectFilter:        a.ProjectFilter,
		TaskSortBy:           a.TaskSortBy,
		ProjectOrder:         a.ProjectOrder,
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	// Atomic: temp+rename so a crash or two racing writers can never leave a
	// truncated ui-prefs.json (loadUIPrefs silently resets on a bad file,
	// which would wipe every saved pref).
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
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
		f(a.ThemeMgr.GlassEnabled), fmt.Sprintf("%.2f", a.ThemeMgr.EffectiveGlassOpacity()),
		a.ThemeMgr.BackgroundKind, a.ThemeMgr.BackgroundPreset, a.ThemeMgr.BackgroundImage,
		fmt.Sprintf("%.2f", a.ThemeMgr.EffectiveBackgroundOpacity()), fmt.Sprintf("%.1f", a.ThemeMgr.EffectiveBackgroundBlur()),
		f(a.ShowLeftSidebar), f(a.ShowRightDrawer), f(a.ShowBottomTerminal),
		f32(a.LeftSidebarWidth), f32(a.RightDrawerWidth), f32(a.BottomTerminalHeight),
		f(a.FollowAgent), a.LeftSidebarView, a.SidebarListView, a.ProjectFilter, a.TaskSortBy,
		strings.Join(a.ProjectOrder, ","),
	}.String()
}

type prefsSnap [23]string

func (p prefsSnap) String() string {
	return strings.Join(p[:], "|")
}
