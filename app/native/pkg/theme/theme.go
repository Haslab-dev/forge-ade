package theme

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown"
)

// Workspace Surface Modes (ForgeADE Dual-Surface architecture)
const (
	ModeAgent  = 0 // Autonomous Agent Mode
	ModeEditor = 1 // Code Editor Mode
)

// ForgeADE Type Scale Tokens (text-ui-* scale in index.css)
const (
	Text5xs    = float32(7)  // 0.4375rem
	Text4xs    = float32(8)  // 0.5rem
	Text3xs    = float32(10) // 0.625rem
	Text2xs    = float32(11) // 0.6875rem
	TextUIXs   = float32(12) // 0.75rem
	TextUISm   = float32(13) // 0.8125rem
	TextUIBase = float32(14) // 0.875rem
	TextUILg   = float32(17) // 1.0625rem
	TextUIXl   = float32(22) // 1.375rem
	TextUI2xl  = float32(26) // 1.625rem
)

// ForgeADE Spacing & Radius Tokens
const (
	RadiusSm  = float32(6)  // calc(var(--radius) - 4px)
	RadiusMd  = float32(8)  // calc(var(--radius) - 2px)
	RadiusLg  = float32(10) // var(--radius)
	RadiusXl  = float32(14) // calc(var(--radius) + 4px)
	Radius2xl = float32(18) // calc(var(--radius) + 8px)
)

// Right Side Drawer Tabs (matching ForgeADE AgentRightSidebar)
const (
	RightTabChanges  = 0
	RightTabTerminal = 1
	RightTabChat     = 2
	RightTabTasks    = 3
	RightTabGit      = 4
	RightTabDatabase = 5
	RightTabBoard    = 6
	RightTabAgent    = 2 // alias for RightTabChat (backward compatibility)
)

// Editor Activity Bar Tabs
const (
	ActivityExplorer = 0
	ActivityGit      = 1
	ActivityDatabase = 2
)

// UI Tab Constants (kept for backward compatibility)
const (
	TabEditor   = 0
	TabDiff     = 1
	TabTerminal = 2
	TabAgent    = 3
	TabGit      = 4
	TabDatabase = 5
	TabSplit    = 6
)

// TabNames defines user-facing labels for the navigation bar.
var TabNames = []string{
	"Editor",
	"Diff",
	"Terminal",
	"Agent AI",
	"Source Control",
	"Database",
	"Split View",
}

// FormatLineCol returns a formatted string like "Ln 12, Col 4"
func FormatLineCol(line, col int) string {
	return fmt.Sprintf("Ln %d, Col %d", line+1, col+1)
}

// ShortenPath abbreviates a long absolute path for display in status bars.
func ShortenPath(fullPath, rootDir string) string {
	if strings.HasPrefix(fullPath, rootDir) {
		rel := strings.TrimPrefix(fullPath, rootDir)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			return "."
		}
		return rel
	}
	return fullPath
}

// ---------------------------------------------------------------------------
// ForgeADE Theme Palettes & Styling Engine
// ---------------------------------------------------------------------------

// Built-in theme palette identifiers matching ForgeADE themePalettes.ts
const (
	ThemePaletteDefault = "forge-default" // ForgeADE
	ThemePaletteBlossom = "blossom"       // Blossom
	ThemePaletteGrove   = "grove"         // Grove
	ThemePaletteOcean   = "ocean"         // Ocean
	ThemePaletteEmber   = "ember"         // Ember
	ThemePaletteIris    = "iris"          // Iris
	ThemePaletteGlass   = "glass"         // Glass — translucent chrome over the OS material
)

// PaletteDefinition describes one theme family for the pickers.
type PaletteDefinition struct {
	ID          string
	Name        string
	Description string
	SwatchHex   string // Primary accent color for UI swatch preview
	DarkTheme   func() *ui.Theme
	LightTheme  func() *ui.Theme
}

// ThemeManager tracks theme preferences, appearance, font sizes, and layout options.
type ThemeManager struct {
	Appearance string  // "dark", "light"
	PaletteID  string  // "forge-default", "blossom", "grove", "ocean", "ember", "iris", "glass"
	FontSize   float32 // base UI text size in px (text-ui-base = 0.875rem = 14px in the original at a 16px root)
	FontFamily string  // "Geist" (the original) or "" for the system font
	ChatWidth  string  // "comfortable" (736px), "wide" (928px), "full" (100%)
	DiffScheme string  // "classic" (Green/Red), "modern" (Blue/Orange)
	IsOpen     bool

	// SupportsVibrancy reports that the window carries an OS translucent
	// material (mygo window Vibrancy). Only the Glass palette needs it.
	SupportsVibrancy bool

	cacheKey      string
	cachedEdTheme *editor.Theme
	cachedMdTheme *markdown.Theme
}

// DefaultFontSize is ForgeADE's default interface font size
// (--ui-font-size: 0.875rem at a 16px root).
const DefaultFontSize = 14

// NewThemeManager constructs the styling engine with default Forge dark appearance.
func NewThemeManager() *ThemeManager {
	return &ThemeManager{
		Appearance: "dark",
		PaletteID:  ThemePaletteDefault,
		FontSize:   DefaultFontSize,
		FontFamily: FontSans,
		ChatWidth:  "full",
		DiffScheme: "classic",
		IsOpen:     false,
	}
}

// Toggle toggles the appearance popover / modal.
func (tm *ThemeManager) Toggle() {
	tm.IsOpen = !tm.IsOpen
}

// IsDark reports the active appearance.
func (tm *ThemeManager) IsDark() bool { return tm.Appearance != "light" }

// Colors returns the full ForgeADE role set for the active appearance.
func (tm *ThemeManager) Colors() ThemeColors {
	if tm.PaletteID == ThemePaletteGlass && !tm.SupportsVibrancy {
		// No OS material behind the window (e.g. Linux): translucent chrome
		// would expose the raw desktop unblurred — fall back to the opaque
		// base palette instead.
		return PaletteColors(ThemePaletteDefault, tm.IsDark())
	}
	return PaletteColors(tm.PaletteID, tm.IsDark())
}

// AvailablePalettes returns all built-in ForgeADE color palettes.
func (tm *ThemeManager) AvailablePalettes() []PaletteDefinition {
	swatches := map[string]string{
		ThemePaletteDefault: "#6366f1",
		ThemePaletteBlossom: "#f43f5e",
		ThemePaletteGrove:   "#10b981",
		ThemePaletteOcean:   "#0ea5e9",
		ThemePaletteEmber:   "#f59e0b",
		ThemePaletteIris:    "#8b5cf6",
		ThemePaletteGlass:   "#94a3b8",
	}
	metas := []struct {
		id, name, desc string
	}{
		{ThemePaletteDefault, "ForgeADE", "Cool gray-blue chrome with a cobalt action color"},
		{ThemePaletteBlossom, "Blossom", "Rose & sakura pastel with soft warm contrasts"},
		{ThemePaletteGrove, "Grove", "Forest emerald and sage nature aesthetic"},
		{ThemePaletteOcean, "Ocean", "Deep sea navy and electric cyan highlights"},
		{ThemePaletteEmber, "Ember", "Warm sunset amber and terracotta tones"},
		{ThemePaletteIris, "Iris", "Deep purple and vibrant lavender violet"},
		{ThemePaletteGlass, "Glass", "Translucent frosted chrome over the desktop material"},
	}
	out := make([]PaletteDefinition, 0, len(metas))
	for _, m := range metas {
		id := m.id
		out = append(out, PaletteDefinition{
			ID:          id,
			Name:        m.name,
			Description: m.desc,
			SwatchHex:   swatches[id],
			DarkTheme:   func() *ui.Theme { return MapTheme(PaletteColors(id, true), true) },
			LightTheme:  func() *ui.Theme { return MapTheme(PaletteColors(id, false), false) },
		})
	}
	return out
}

// ActiveTheme generates the configured Theme according to current appearance and palette.
func (tm *ThemeManager) ActiveTheme() *ui.Theme {
	t := MapTheme(tm.Colors(), tm.IsDark())
	if tm.FontSize > 0 {
		t.FontSize = tm.FontSize
	}
	if tm.FontFamily != "" {
		t.Font = tm.FontFamily
	}
	return t
}

func (tm *ThemeManager) themeCacheKey() string {
	return fmt.Sprintf("%s|%s|%g|%s|%s", tm.Appearance, tm.PaletteID, tm.FontSize, tm.FontFamily, tm.DiffScheme)
}

func (tm *ThemeManager) ensureCachedThemes() {
	key := tm.themeCacheKey()
	if tm.cacheKey == key && tm.cachedEdTheme != nil && tm.cachedMdTheme != nil {
		return
	}
	tc := tm.Colors()
	dark := tm.IsDark()
	edTheme := ForgeEditorTheme(tc, dark, tm.DiffScheme)
	family := tm.FontFamily
	if family == "" {
		family = FontSans
	}
	mdTheme := ForgeMarkdownTheme(tc, dark, edTheme, EditorCodeFontSize, family, monoFor(family))
	tm.cachedEdTheme = edTheme
	tm.cachedMdTheme = mdTheme
	tm.cacheKey = key
}

// EditorTheme returns the cached code editor theme, rebuilding only when theme parameters change.
func (tm *ThemeManager) EditorTheme() *editor.Theme {
	tm.ensureCachedThemes()
	return tm.cachedEdTheme
}

// MarkdownTheme returns the cached chat Markdown theme, rebuilding only when theme parameters change.
func (tm *ThemeManager) MarkdownTheme() *markdown.Theme {
	tm.ensureCachedThemes()
	return tm.cachedMdTheme
}

// SwatchColor resolves a palette swatch hex string into a semantic UI color token.
func SwatchColor(hex string) ui.Color {
	return ui.Hex(hex)
}

// MonoFor pairs the interface family with the mono family, as --font-mono does in the original.
func MonoFor(family string) string {
	if family == FontSans || family == "" {
		return FontMono
	}
	return family
}

func monoFor(family string) string {
	return MonoFor(family)
}

// ChatLaneWidth returns the chat content lane width in DIPs
// (--chat-content-max-width), or 0 for full width.
func (tm *ThemeManager) ChatLaneWidth() float32 {
	switch tm.ChatWidth {
	case "wide":
		return 928 // 58rem
	case "full":
		return 0
	default:
		return 736 // 46rem
	}
}

// IsKnownPalette reports whether a palette id is one of the built-in packs.
func IsKnownPalette(id string) bool {
	switch id {
	case "forge-default", "blossom", "grove", "ocean", "ember", "iris":
		return true
	}
	return false
}
