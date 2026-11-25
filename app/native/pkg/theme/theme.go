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
	ThemePaletteObsidian = "obsidian"      // Obsidian — neutral carbon chrome inspired by Orca
	ThemePaletteDefault  = "forge-default" // ForgeADE
	ThemePaletteBlossom  = "blossom"       // Blossom
	ThemePaletteGrove    = "grove"         // Grove
	ThemePaletteOcean    = "ocean"         // Ocean
	ThemePaletteEmber    = "ember"         // Ember
	ThemePaletteIris     = "iris"          // Iris
	ThemePaletteGlass    = "glass"         // Glass — translucent chrome over the OS material
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

// Built-in glass & background defaults and bounds.
const (
	DefaultGlassOpacity      = float32(0.42)
	MinGlassOpacity          = float32(0.00)
	MaxGlassOpacity          = float32(0.95)
	DefaultBackgroundOpacity = float32(0.85)
	MinBackgroundOpacity     = float32(0.00)
	MaxBackgroundOpacity     = float32(1.00)
	MaxBackgroundBlur        = float32(36)

	BackgroundKindNone   = ""
	BackgroundKindPreset = "preset"
	BackgroundKindImage  = "image"

	BackgroundPresetAurora   = "aurora"
	BackgroundPresetMidnight = "midnight"
	BackgroundPresetSunset   = "sunset"
	BackgroundPresetOceanic  = "oceanic"
	BackgroundPresetRose     = "rose"
	BackgroundPresetEmerald  = "emerald"
)

// BackgroundPresetMeta describes a built-in ambient gradient background preset.
type BackgroundPresetMeta struct {
	ID          string
	Name        string
	Description string
	SwatchHex   string
	Angle       float32
	DarkStops   []ui.GradientStop
	LightStops  []ui.GradientStop
	DarkGlowA   ui.Color
	DarkGlowB   ui.Color
	LightGlowA  ui.Color
	LightGlowB  ui.Color
}

// ThemeManager tracks theme preferences, appearance, font sizes, glass translucency,
// window backgrounds, and layout options.
type ThemeManager struct {
	Appearance string  // "dark", "light"
	PaletteID  string  // "obsidian", "forge-default", "blossom", "grove", "ocean", "ember", "iris", "glass"
	FontSize   float32 // base UI text size in px (text-ui-base = 0.875rem = 14px in the original at a 16px root)
	FontFamily string  // "Geist" (the original) or "" for the system font
	ChatWidth  string  // "comfortable" (736px), "wide" (928px), "full" (100%)
	DiffScheme string  // "classic" (Green/Red), "modern" (Blue/Orange)
	IsOpen     bool

	// Semi-transparent glass & custom window background settings.
	GlassEnabled         bool    // Enables semi-transparent glass chrome for any palette
	GlassOpacity         float32 // 0.00..0.95 (uses DefaultGlassOpacity when unset)
	GlassOpacitySet      bool    // Distinguishes explicit 0.00 from uninitialized struct
	BackgroundKind       string  // "", "preset", "image"
	BackgroundPreset     string  // Built-in gradient preset ID
	BackgroundImage      string  // Path to custom background image file
	BackgroundOpacity    float32 // 0.00..1.00 (uses DefaultBackgroundOpacity when unset)
	BackgroundOpacitySet bool    // Distinguishes explicit 0.00 from uninitialized struct
	BackgroundBlur       float32 // 0..36 DIPs Gaussian blur over the background layer

	// SupportsVibrancy reports that the window carries an OS translucent
	// material (mygo window Vibrancy).
	SupportsVibrancy bool

	cacheKey      string
	cachedEdTheme *editor.Theme
	cachedMdTheme *markdown.Theme
}

// DefaultFontSize is ForgeADE's default interface font size
// (--ui-font-size: 0.875rem at a 16px root).
const DefaultFontSize = 14

// NewThemeManager constructs the styling engine with default Obsidian dark appearance.
func NewThemeManager() *ThemeManager {
	return &ThemeManager{
		Appearance:           "dark",
		PaletteID:            ThemePaletteObsidian,
		FontSize:             DefaultFontSize,
		FontFamily:           FontSans,
		ChatWidth:            "full",
		DiffScheme:           "classic",
		GlassOpacity:         DefaultGlassOpacity,
		GlassOpacitySet:      true,
		BackgroundOpacity:    DefaultBackgroundOpacity,
		BackgroundOpacitySet: true,
		IsOpen:               false,
	}
}

// Toggle toggles the appearance popover / modal.
func (tm *ThemeManager) Toggle() {
	tm.IsOpen = !tm.IsOpen
}

// IsDark reports the active appearance.
func (tm *ThemeManager) IsDark() bool { return tm.Appearance != "light" }

// IsGlassActive reports whether semi-transparent glass chrome is enabled
// (either via the Glass palette or the GlassEnabled toggle on any palette).
func (tm *ThemeManager) IsGlassActive() bool {
	return tm.GlassEnabled || tm.PaletteID == ThemePaletteGlass
}

// EffectiveGlassOpacity returns the clamped glass opacity in [MinGlassOpacity, MaxGlassOpacity].
func (tm *ThemeManager) EffectiveGlassOpacity() float32 {
	v := tm.GlassOpacity
	if !tm.GlassOpacitySet && v <= 0 {
		return DefaultGlassOpacity
	}
	if v < MinGlassOpacity {
		return MinGlassOpacity
	}
	if v > MaxGlassOpacity {
		return MaxGlassOpacity
	}
	return v
}

// HasBackground reports whether a window background preset or image is active.
func (tm *ThemeManager) HasBackground() bool {
	switch tm.BackgroundKind {
	case BackgroundKindPreset:
		_, ok := LookupBackgroundPreset(tm.BackgroundPreset)
		return ok
	case BackgroundKindImage:
		return strings.TrimSpace(tm.BackgroundImage) != ""
	default:
		return false
	}
}

// EffectiveBackgroundOpacity returns the clamped background opacity in [MinBackgroundOpacity, MaxBackgroundOpacity].
func (tm *ThemeManager) EffectiveBackgroundOpacity() float32 {
	v := tm.BackgroundOpacity
	if !tm.BackgroundOpacitySet && v <= 0 {
		return DefaultBackgroundOpacity
	}
	if v < MinBackgroundOpacity {
		return MinBackgroundOpacity
	}
	if v > MaxBackgroundOpacity {
		return MaxBackgroundOpacity
	}
	return v
}

// EffectiveBackgroundBlur returns the clamped background blur sigma in [0, MaxBackgroundBlur].
func (tm *ThemeManager) EffectiveBackgroundBlur() float32 {
	if tm.BackgroundBlur <= 0 {
		return 0
	}
	if tm.BackgroundBlur > MaxBackgroundBlur {
		return MaxBackgroundBlur
	}
	return tm.BackgroundBlur
}

// Colors returns the full ForgeADE role set for the active appearance.
func (tm *ThemeManager) Colors() ThemeColors {
	base := BasePaletteColors(tm.PaletteID, tm.IsDark())
	if tm.IsGlassActive() {
		if !tm.SupportsVibrancy && !tm.HasBackground() {
			// No OS material behind the window (e.g. Linux) and no custom
			// background layer: translucent chrome would expose the raw desktop
			// unblurred — fall back to opaque base colors instead.
			return base
		}
		return GlassChrome(base, tm.IsDark(), tm.EffectiveGlassOpacity())
	}
	return base
}

// AvailablePalettes returns all built-in ForgeADE color palettes.
func (tm *ThemeManager) AvailablePalettes() []PaletteDefinition {
	swatches := map[string]string{
		ThemePaletteObsidian: "#27272a",
		ThemePaletteDefault:  "#6366f1",
		ThemePaletteBlossom:  "#f43f5e",
		ThemePaletteGrove:    "#10b981",
		ThemePaletteOcean:    "#0ea5e9",
		ThemePaletteEmber:    "#f59e0b",
		ThemePaletteIris:     "#8b5cf6",
		ThemePaletteGlass:    "#94a3b8",
	}
	metas := []struct {
		id, name, desc string
	}{
		{ThemePaletteObsidian, "Obsidian", "Neutral carbon chrome with hairline borders and high-signal accents"},
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
	glassActive := tm.IsGlassActive() && (tm.SupportsVibrancy || tm.HasBackground())
	return fmt.Sprintf("%s|%s|%g|%s|%s|%v|%.2f",
		tm.Appearance, tm.PaletteID, tm.FontSize, tm.FontFamily, tm.DiffScheme,
		glassActive, tm.EffectiveGlassOpacity())
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
	case ThemePaletteObsidian, ThemePaletteDefault, ThemePaletteBlossom,
		ThemePaletteGrove, ThemePaletteOcean, ThemePaletteEmber,
		ThemePaletteIris, ThemePaletteGlass:
		return true
	}
	return false
}

// AvailableBackgroundPresets returns the built-in ambient gradient background presets.
func AvailableBackgroundPresets() []BackgroundPresetMeta {
	return []BackgroundPresetMeta{
		{
			ID:          BackgroundPresetAurora,
			Name:        "Aurora",
			Description: "Deep indigo sky with emerald & violet northern glow",
			SwatchHex:   "#38bdf8",
			Angle:       135,
			DarkStops: []ui.GradientStop{
				{Color: ui.Hex("#090d16"), At: 0},
				{Color: ui.Hex("#0f172a"), At: 0.45},
				{Color: ui.Hex("#132a3e"), At: 0.78},
				{Color: ui.Hex("#1e1b4b"), At: 1},
			},
			LightStops: []ui.GradientStop{
				{Color: ui.Hex("#eef2ff"), At: 0},
				{Color: ui.Hex("#e0f2fe"), At: 0.5},
				{Color: ui.Hex("#ecfdf5"), At: 1},
			},
			DarkGlowA:  ui.RGBA(56, 189, 248, 0.28),
			DarkGlowB:  ui.RGBA(168, 85, 247, 0.24),
			LightGlowA: ui.RGBA(56, 189, 248, 0.25),
			LightGlowB: ui.RGBA(167, 139, 250, 0.22),
		},
		{
			ID:          BackgroundPresetMidnight,
			Name:        "Midnight",
			Description: "Subtle slate-carbon gradient with cobalt depth",
			SwatchHex:   "#4f46e5",
			Angle:       150,
			DarkStops: []ui.GradientStop{
				{Color: ui.Hex("#07080d"), At: 0},
				{Color: ui.Hex("#0d111c"), At: 0.55},
				{Color: ui.Hex("#141b30"), At: 1},
			},
			LightStops: []ui.GradientStop{
				{Color: ui.Hex("#f8fafc"), At: 0},
				{Color: ui.Hex("#eef2ff"), At: 0.55},
				{Color: ui.Hex("#e2e8f0"), At: 1},
			},
			DarkGlowA:  ui.RGBA(99, 102, 241, 0.24),
			DarkGlowB:  ui.RGBA(30, 64, 175, 0.20),
			LightGlowA: ui.RGBA(99, 102, 241, 0.16),
			LightGlowB: ui.RGBA(148, 163, 184, 0.20),
		},
		{
			ID:          BackgroundPresetOceanic,
			Name:        "Oceanic",
			Description: "Deep abyssal teal and electric cyan horizon",
			SwatchHex:   "#0ea5e9",
			Angle:       140,
			DarkStops: []ui.GradientStop{
				{Color: ui.Hex("#04121d"), At: 0},
				{Color: ui.Hex("#082436"), At: 0.5},
				{Color: ui.Hex("#0c3b4f"), At: 1},
			},
			LightStops: []ui.GradientStop{
				{Color: ui.Hex("#ecfeff"), At: 0},
				{Color: ui.Hex("#e0f2fe"), At: 0.55},
				{Color: ui.Hex("#dbeafe"), At: 1},
			},
			DarkGlowA:  ui.RGBA(14, 165, 233, 0.28),
			DarkGlowB:  ui.RGBA(20, 184, 166, 0.22),
			LightGlowA: ui.RGBA(14, 165, 233, 0.22),
			LightGlowB: ui.RGBA(45, 212, 191, 0.18),
		},
		{
			ID:          BackgroundPresetSunset,
			Name:        "Sunset",
			Description: "Warm dusk amber fading into deep plum",
			SwatchHex:   "#f97316",
			Angle:       135,
			DarkStops: []ui.GradientStop{
				{Color: ui.Hex("#180c14"), At: 0},
				{Color: ui.Hex("#2b121c"), At: 0.5},
				{Color: ui.Hex("#381b14"), At: 1},
			},
			LightStops: []ui.GradientStop{
				{Color: ui.Hex("#fff7ed"), At: 0},
				{Color: ui.Hex("#ffedd5"), At: 0.5},
				{Color: ui.Hex("#fce7f3"), At: 1},
			},
			DarkGlowA:  ui.RGBA(249, 115, 22, 0.25),
			DarkGlowB:  ui.RGBA(236, 72, 153, 0.22),
			LightGlowA: ui.RGBA(251, 146, 60, 0.24),
			LightGlowB: ui.RGBA(244, 114, 182, 0.18),
		},
		{
			ID:          BackgroundPresetRose,
			Name:        "Sakura",
			Description: "Soft rosewood and lavender mist",
			SwatchHex:   "#f43f5e",
			Angle:       145,
			DarkStops: []ui.GradientStop{
				{Color: ui.Hex("#150b16"), At: 0},
				{Color: ui.Hex("#241127"), At: 0.55},
				{Color: ui.Hex("#2f1423"), At: 1},
			},
			LightStops: []ui.GradientStop{
				{Color: ui.Hex("#fff1f2"), At: 0},
				{Color: ui.Hex("#fce7f3"), At: 0.55},
				{Color: ui.Hex("#f3e8ff"), At: 1},
			},
			DarkGlowA:  ui.RGBA(244, 63, 94, 0.24),
			DarkGlowB:  ui.RGBA(192, 132, 252, 0.20),
			LightGlowA: ui.RGBA(244, 63, 94, 0.18),
			LightGlowB: ui.RGBA(192, 132, 252, 0.20),
		},
		{
			ID:          BackgroundPresetEmerald,
			Name:        "Forest",
			Description: "Mossy obsidian pine with soft sage luminescence",
			SwatchHex:   "#10b981",
			Angle:       140,
			DarkStops: []ui.GradientStop{
				{Color: ui.Hex("#07130f"), At: 0},
				{Color: ui.Hex("#0c231b"), At: 0.55},
				{Color: ui.Hex("#102f25"), At: 1},
			},
			LightStops: []ui.GradientStop{
				{Color: ui.Hex("#ecfdf5"), At: 0},
				{Color: ui.Hex("#d1fae5"), At: 0.55},
				{Color: ui.Hex("#e0f2fe"), At: 1},
			},
			DarkGlowA:  ui.RGBA(16, 185, 129, 0.24),
			DarkGlowB:  ui.RGBA(52, 211, 153, 0.16),
			LightGlowA: ui.RGBA(16, 185, 129, 0.20),
			LightGlowB: ui.RGBA(56, 189, 248, 0.16),
		},
	}
}

// LookupBackgroundPreset returns metadata for a built-in background preset.
func LookupBackgroundPreset(id string) (BackgroundPresetMeta, bool) {
	for _, p := range AvailableBackgroundPresets() {
		if p.ID == id {
			return p, true
		}
	}
	return BackgroundPresetMeta{}, false
}
