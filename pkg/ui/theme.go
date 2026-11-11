package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Workspace Surface Modes (ForgeADE Dual-Surface architecture)
const (
	ModeAgent  = 0 // ✨ Autonomous Agent Mode
	ModeEditor = 1 // 💻 Code Editor Mode
)

// Right Side Drawer Tabs
const (
	RightTabGit      = 0
	RightTabDatabase = 1
	RightTabTasks    = 2
	RightTabAgent    = 3
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
	ThemePaletteDefault = "forge-default" // Forge Obsidian / Slate
	ThemePaletteBlossom = "blossom"       // Rose & Sakura pastel
	ThemePaletteGrove   = "grove"         // Forest Emerald & Sage
	ThemePaletteOcean   = "ocean"         // Deep Blue & Aqua
	ThemePaletteEmber   = "ember"         // Warm Amber & Coral
	ThemePaletteIris    = "iris"          // Purple & Violet
)

// PaletteDefinition holds colors for a theme family in dark and light modes.
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
	Appearance  string  // "dark", "light"
	PaletteID   string  // "forge-default", "blossom", "grove", "ocean", "ember", "iris"
	FontSize    float32 // 11 to 18 pt
	FontFamily  string
	ChatWidth   string  // "comfortable" (736px), "wide" (928px), "full" (100%)
	DiffScheme  string  // "classic" (Green/Red), "modern" (Blue/Orange)
	IsOpen      bool
}

// NewThemeManager constructs the styling engine with default Forge dark appearance.
func NewThemeManager() *ThemeManager {
	return &ThemeManager{
		Appearance: "dark",
		PaletteID:  ThemePaletteDefault,
		FontSize:   13,
		FontFamily: "System",
		ChatWidth:  "comfortable",
		DiffScheme: "classic",
		IsOpen:     false,
	}
}

// Toggle toggles the appearance popover / modal.
func (tm *ThemeManager) Toggle() {
	tm.IsOpen = !tm.IsOpen
}

// AvailablePalettes returns all built-in ForgeADE color palettes.
func (tm *ThemeManager) AvailablePalettes() []PaletteDefinition {
	return []PaletteDefinition{
		{
			ID:          ThemePaletteDefault,
			Name:        "Forge Default",
			Description: "Obsidian neutral chrome with indigo iris accents",
			SwatchHex:   "#6366f1",
			DarkTheme: func() *ui.Theme {
				t := ui.DarkTheme()
				t.Background = ui.Hex("#121214")
				t.Surface = ui.Hex("#18181b")
				t.SurfaceHover = ui.Hex("#27272a")
				t.Border = ui.Hex("#27272a")
				t.Text = ui.Hex("#f4f4f5")
				t.TextMuted = ui.Hex("#a1a1aa")
				t.Accent = ui.Hex("#6366f1")
				t.AccentHover = ui.Hex("#4f46e5")
				t.AccentText = ui.Hex("#ffffff")
				t.Selection = ui.Hex("#312e81")
				t.Focus = ui.Hex("#818cf8")
				return t
			},
			LightTheme: func() *ui.Theme {
				t := ui.LightTheme()
				t.Background = ui.Hex("#ffffff")
				t.Surface = ui.Hex("#f4f4f5")
				t.SurfaceHover = ui.Hex("#e4e4e7")
				t.Border = ui.Hex("#e4e4e7")
				t.Text = ui.Hex("#18181b")
				t.TextMuted = ui.Hex("#71717a")
				t.Accent = ui.Hex("#4f46e5")
				t.AccentHover = ui.Hex("#4338ca")
				t.AccentText = ui.Hex("#ffffff")
				t.Selection = ui.Hex("#c7d2fe")
				t.Focus = ui.Hex("#6366f1")
				return t
			},
		},
		{
			ID:          ThemePaletteBlossom,
			Name:        "Blossom",
			Description: "Rose & Sakura pastel with soft warm contrasts",
			SwatchHex:   "#f43f5e",
			DarkTheme: func() *ui.Theme {
				t := ui.DarkTheme()
				t.Background = ui.Hex("#150d12")
				t.Surface = ui.Hex("#1f131a")
				t.SurfaceHover = ui.Hex("#2d1a25")
				t.Border = ui.Hex("#361d2b")
				t.Text = ui.Hex("#fdf2f8")
				t.TextMuted = ui.Hex("#f472b6")
				t.Accent = ui.Hex("#f43f5e")
				t.AccentHover = ui.Hex("#e11d48")
				t.AccentText = ui.Hex("#ffffff")
				t.Selection = ui.Hex("#831843")
				t.Focus = ui.Hex("#fb7185")
				return t
			},
			LightTheme: func() *ui.Theme {
				t := ui.LightTheme()
				t.Background = ui.Hex("#fff1f2")
				t.Surface = ui.Hex("#ffe4e6")
				t.SurfaceHover = ui.Hex("#fecdd3")
				t.Border = ui.Hex("#fecdd3")
				t.Text = ui.Hex("#881337")
				t.TextMuted = ui.Hex("#9f1239")
				t.Accent = ui.Hex("#e11d48")
				t.AccentHover = ui.Hex("#be123c")
				t.AccentText = ui.Hex("#ffffff")
				t.Selection = ui.Hex("#fecdd3")
				t.Focus = ui.Hex("#f43f5e")
				return t
			},
		},
		{
			ID:          ThemePaletteGrove,
			Name:        "Grove",
			Description: "Forest emerald and sage nature aesthetic",
			SwatchHex:   "#10b981",
			DarkTheme: func() *ui.Theme {
				t := ui.DarkTheme()
				t.Background = ui.Hex("#0a140f")
				t.Surface = ui.Hex("#122019")
				t.SurfaceHover = ui.Hex("#1a2e24")
				t.Border = ui.Hex("#1b3327")
				t.Text = ui.Hex("#ecfdf5")
				t.TextMuted = ui.Hex("#6ee7b7")
				t.Accent = ui.Hex("#10b981")
				t.AccentHover = ui.Hex("#059669")
				t.AccentText = ui.Hex("#ffffff")
				t.Selection = ui.Hex("#064e3b")
				t.Focus = ui.Hex("#34d399")
				return t
			},
			LightTheme: func() *ui.Theme {
				t := ui.LightTheme()
				t.Background = ui.Hex("#f0fdf4")
				t.Surface = ui.Hex("#dcfce7")
				t.SurfaceHover = ui.Hex("#bbf7d0")
				t.Border = ui.Hex("#bbf7d0")
				t.Text = ui.Hex("#064e3b")
				t.TextMuted = ui.Hex("#047857")
				t.Accent = ui.Hex("#059669")
				t.AccentHover = ui.Hex("#047857")
				t.AccentText = ui.Hex("#ffffff")
				t.Selection = ui.Hex("#bbf7d0")
				t.Focus = ui.Hex("#10b981")
				return t
			},
		},
		{
			ID:          ThemePaletteOcean,
			Name:        "Ocean",
			Description: "Deep sea navy and electric cyan highlights",
			SwatchHex:   "#0ea5e9",
			DarkTheme: func() *ui.Theme {
				t := ui.DarkTheme()
				t.Background = ui.Hex("#0b1320")
				t.Surface = ui.Hex("#111d30")
				t.SurfaceHover = ui.Hex("#182942")
				t.Border = ui.Hex("#1e3250")
				t.Text = ui.Hex("#f0f9ff")
				t.TextMuted = ui.Hex("#7dd3fc")
				t.Accent = ui.Hex("#0ea5e9")
				t.AccentHover = ui.Hex("#0284c7")
				t.AccentText = ui.Hex("#ffffff")
				t.Selection = ui.Hex("#0c4a6e")
				t.Focus = ui.Hex("#38bdf8")
				return t
			},
			LightTheme: func() *ui.Theme {
				t := ui.LightTheme()
				t.Background = ui.Hex("#f0f9ff")
				t.Surface = ui.Hex("#e0f2fe")
				t.SurfaceHover = ui.Hex("#bae6fd")
				t.Border = ui.Hex("#bae6fd")
				t.Text = ui.Hex("#082f49")
				t.TextMuted = ui.Hex("#0369a1")
				t.Accent = ui.Hex("#0284c7")
				t.AccentHover = ui.Hex("#0369a1")
				t.AccentText = ui.Hex("#ffffff")
				t.Selection = ui.Hex("#bae6fd")
				t.Focus = ui.Hex("#0ea5e9")
				return t
			},
		},
		{
			ID:          ThemePaletteEmber,
			Name:        "Ember",
			Description: "Warm sunset amber and terracotta tones",
			SwatchHex:   "#f59e0b",
			DarkTheme: func() *ui.Theme {
				t := ui.DarkTheme()
				t.Background = ui.Hex("#16110a")
				t.Surface = ui.Hex("#221a10")
				t.SurfaceHover = ui.Hex("#302416")
				t.Border = ui.Hex("#382917")
				t.Text = ui.Hex("#fffbeb")
				t.TextMuted = ui.Hex("#fcd34d")
				t.Accent = ui.Hex("#f59e0b")
				t.AccentHover = ui.Hex("#d97706")
				t.AccentText = ui.Hex("#000000")
				t.Selection = ui.Hex("#78350f")
				t.Focus = ui.Hex("#fbbf24")
				return t
			},
			LightTheme: func() *ui.Theme {
				t := ui.LightTheme()
				t.Background = ui.Hex("#fffbeb")
				t.Surface = ui.Hex("#fef3c7")
				t.SurfaceHover = ui.Hex("#fde68a")
				t.Border = ui.Hex("#fde68a")
				t.Text = ui.Hex("#78350f")
				t.TextMuted = ui.Hex("#b45309")
				t.Accent = ui.Hex("#d97706")
				t.AccentHover = ui.Hex("#b45309")
				t.AccentText = ui.Hex("#ffffff")
				t.Selection = ui.Hex("#fde68a")
				t.Focus = ui.Hex("#f59e0b")
				return t
			},
		},
		{
			ID:          ThemePaletteIris,
			Name:        "Iris",
			Description: "Deep purple and vibrant lavender violet",
			SwatchHex:   "#8b5cf6",
			DarkTheme: func() *ui.Theme {
				t := ui.DarkTheme()
				t.Background = ui.Hex("#130e1f")
				t.Surface = ui.Hex("#1c152e")
				t.SurfaceHover = ui.Hex("#281d42")
				t.Border = ui.Hex("#32254f")
				t.Text = ui.Hex("#faf5ff")
				t.TextMuted = ui.Hex("#c084fc")
				t.Accent = ui.Hex("#8b5cf6")
				t.AccentHover = ui.Hex("#7c3aed")
				t.AccentText = ui.Hex("#ffffff")
				t.Selection = ui.Hex("#4c1d95")
				t.Focus = ui.Hex("#a855f7")
				return t
			},
			LightTheme: func() *ui.Theme {
				t := ui.LightTheme()
				t.Background = ui.Hex("#faf5ff")
				t.Surface = ui.Hex("#f3e8ff")
				t.SurfaceHover = ui.Hex("#e9d5ff")
				t.Border = ui.Hex("#e9d5ff")
				t.Text = ui.Hex("#3b0764")
				t.TextMuted = ui.Hex("#6b21a8")
				t.Accent = ui.Hex("#7c3aed")
				t.AccentHover = ui.Hex("#6d28d9")
				t.AccentText = ui.Hex("#ffffff")
				t.Selection = ui.Hex("#e9d5ff")
				t.Focus = ui.Hex("#8b5cf6")
				return t
			},
		},
	}
}

// ActiveTheme generates the configured Theme according to current appearance and palette.
func (tm *ThemeManager) ActiveTheme() *ui.Theme {
	palettes := tm.AvailablePalettes()
	var def PaletteDefinition
	for _, p := range palettes {
		if p.ID == tm.PaletteID {
			def = p
			break
		}
	}
	if def.ID == "" {
		def = palettes[0]
	}

	var theme *ui.Theme
	if tm.Appearance == "light" {
		theme = def.LightTheme()
	} else {
		theme = def.DarkTheme()
	}

	if tm.FontSize > 0 {
		theme.FontSize = tm.FontSize
	}

	return theme
}

// RenderThemePicker renders ForgeADE's ThemePicker modal.
func (tm *ThemeManager) RenderThemePicker(c *ui.Context) {
	t := c.Theme()

	ui.Modal(c, &tm.IsOpen, func() {
		ui.Column(c).Width(480).Radius(12).Background(t.Surface).BorderWidth(1).BorderColor(t.Border).Padding(16, 20).Gap(14).Children(func() {
			// Header
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "🎨 Theme & Appearance").Bold().FontSize(15)
				ui.Box(c).Grow(1)
				if ui.Button(c, "✕").FontSize(12).Clicked() {
					tm.IsOpen = false
				}
			})

			// 1. Appearance Switcher (Dark / Light)
			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, "APPEARANCE").FontSize(10).Bold().TextColor(t.TextMuted)
				ui.Row(c).Gap(8).Children(func() {
					darkBtn := ui.Button(c, "🌙 Dark")
					if tm.Appearance == "dark" {
						darkBtn = ui.PrimaryButton(c, "🌙 Dark")
					}
					if darkBtn.Clicked() {
						tm.Appearance = "dark"
					}

					lightBtn := ui.Button(c, "☀️ Light")
					if tm.Appearance == "light" {
						lightBtn = ui.PrimaryButton(c, "☀️ Light")
					}
					if lightBtn.Clicked() {
						tm.Appearance = "light"
					}
				})
			})

			// 2. Palette Gallery (6 ForgeADE color themes)
			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, "COLOR PALETTE").FontSize(10).Bold().TextColor(t.TextMuted)
				ui.Row(c).Gap(6).Children(func() {
					for _, p := range tm.AvailablePalettes() {
						thisID := p.ID
						btnLabel := fmt.Sprintf("%s", p.Name)
						var btn ui.Element
						if tm.PaletteID == thisID {
							btn = ui.PrimaryButton(c, btnLabel).FontSize(11)
						} else {
							btn = ui.Button(c, btnLabel).FontSize(11)
						}
						if btn.Clicked() {
							tm.PaletteID = thisID
						}
					}
				})
			})

			// 3. Chat Lane Width (Comfortable / Wide / Full)
			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, "CHAT WIDTH").FontSize(10).Bold().TextColor(t.TextMuted)
				ui.Row(c).Gap(8).Children(func() {
					widths := []struct {
						id    string
						label string
					}{
						{"comfortable", "Comfortable (736px)"},
						{"wide", "Wide (928px)"},
						{"full", "Full Width (100%)"},
					}
					for _, w := range widths {
						thisW := w.id
						var btn ui.Element
						if tm.ChatWidth == thisW {
							btn = ui.PrimaryButton(c, w.label).FontSize(11)
						} else {
							btn = ui.Button(c, w.label).FontSize(11)
						}
						if btn.Clicked() {
							tm.ChatWidth = thisW
						}
					}
				})
			})

			// 4. Diff Colors Scheme
			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, "DIFF COLOR SCHEME").FontSize(10).Bold().TextColor(t.TextMuted)
				ui.Row(c).Gap(8).Children(func() {
					schemes := []struct {
						id    string
						label string
					}{
						{"classic", "Classic (Green / Red)"},
						{"modern", "Modern (Blue / Orange)"},
					}
					for _, s := range schemes {
						thisS := s.id
						var btn ui.Element
						if tm.DiffScheme == thisS {
							btn = ui.PrimaryButton(c, s.label).FontSize(11)
						} else {
							btn = ui.Button(c, s.label).FontSize(11)
						}
						if btn.Clicked() {
							tm.DiffScheme = thisS
						}
					}
				})
			})

			// Close action button
			ui.Row(c).Padding(4, 0).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Grow(1)
				if ui.PrimaryButton(c, "Apply & Done").Clicked() {
					tm.IsOpen = false
				}
			})
		})
	})
}
