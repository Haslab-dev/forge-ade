package theme

import (
	"testing"
)

func TestThemePalettes(t *testing.T) {
	tm := NewThemeManager()

	if tm.Appearance != "dark" {
		t.Fatalf("expected default appearance 'dark', got '%s'", tm.Appearance)
	}

	palettes := tm.AvailablePalettes()
	if len(palettes) != 8 {
		t.Fatalf("expected 8 built-in ForgeADE palettes, got %d", len(palettes))
	}

	expectedIDs := []string{
		ThemePaletteObsidian,
		ThemePaletteDefault,
		ThemePaletteBlossom,
		ThemePaletteGrove,
		ThemePaletteOcean,
		ThemePaletteEmber,
		ThemePaletteIris,
		ThemePaletteGlass,
	}

	for _, id := range expectedIDs {
		found := false
		for _, p := range palettes {
			if p.ID == id {
				found = true
				dark := p.DarkTheme()
				light := p.LightTheme()
				if dark == nil || light == nil {
					t.Fatalf("palette %s returned nil theme", id)
				}
				if !dark.Dark || light.Dark {
					t.Fatalf("palette %s has incorrect Dark flag", id)
				}
				// The pickers render theme.SwatchColor(SwatchHex); ui.Hex
				// panics on unparseable input, so every palette must carry a
				// valid hex — an empty one broke the Appearance screen.
				if p.SwatchHex == "" {
					t.Fatalf("palette %s has an empty swatch", id)
				}
				SwatchColor(p.SwatchHex)
				break
			}
		}
		if !found {
			t.Fatalf("missing expected palette %s", id)
		}
	}

	// Test ActiveTheme generation
	tm.PaletteID = ThemePaletteOcean
	tm.Appearance = "light"
	tm.FontSize = 14
	th := tm.ActiveTheme()
	if th == nil {
		t.Fatalf("expected active theme to be non-nil")
	}
	if th.Dark {
		t.Fatalf("expected active theme to be light")
	}
	if th.FontSize != 14 {
		t.Fatalf("expected active theme font size 14, got %f", th.FontSize)
	}

	for _, id := range expectedIDs {
		if !IsKnownPalette(id) {
			t.Fatalf("expected IsKnownPalette(%q) to be true", id)
		}
	}
	if IsKnownPalette("unknown-palette") {
		t.Fatalf("expected IsKnownPalette('unknown-palette') to be false")
	}
}

func TestGlassAndBackgroundTheme(t *testing.T) {
	tm := NewThemeManager()
	tm.PaletteID = ThemePaletteOcean
	tm.Appearance = "dark"

	// Without vibrancy or background, GlassEnabled still falls back to opaque.
	tm.GlassEnabled = true
	opaque := tm.Colors()
	if opaque.Canvas.A != 255 {
		t.Fatalf("expected opaque Canvas when neither vibrancy nor background is active, got alpha %d", opaque.Canvas.A)
	}

	// With a background preset (even without OS vibrancy), glass becomes translucent.
	tm.BackgroundKind = BackgroundKindPreset
	tm.BackgroundPreset = BackgroundPresetAurora
	if !tm.HasBackground() {
		t.Fatalf("expected HasBackground() to be true for aurora preset")
	}
	tm.GlassOpacity = 0.00
	tm.GlassOpacitySet = true
	clear := tm.Colors()
	if clear.Canvas.A != 0 {
		t.Fatalf("expected Canvas alpha 0 at 0%% glass opacity, got %d", clear.Canvas.A)
	}

	tm.GlassOpacity = 0.40
	translucent := tm.Colors()
	if translucent.Canvas.A <= clear.Canvas.A || translucent.Canvas.A >= 255 || translucent.Sidebar.A >= 255 {
		t.Fatalf("expected translucent Canvas and Sidebar in glass mode, got canvas=%d sidebar=%d", translucent.Canvas.A, translucent.Sidebar.A)
	}

	// Higher glass opacity yields higher alpha.
	tm.GlassOpacity = 0.85
	denser := tm.Colors()
	if denser.Canvas.A <= translucent.Canvas.A {
		t.Fatalf("expected denser glass opacity to increase Canvas alpha (%d <= %d)", denser.Canvas.A, translucent.Canvas.A)
	}

	// Background presets validate cleanly.
	presets := AvailableBackgroundPresets()
	if len(presets) < 4 {
		t.Fatalf("expected at least 4 background presets, got %d", len(presets))
	}
	for _, p := range presets {
		if len(p.DarkStops) < 2 || len(p.LightStops) < 2 {
			t.Fatalf("preset %s missing gradient stops", p.ID)
		}
		SwatchColor(p.SwatchHex)
	}
}
