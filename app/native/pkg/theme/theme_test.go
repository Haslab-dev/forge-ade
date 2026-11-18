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
	if len(palettes) != 7 {
		t.Fatalf("expected 7 built-in ForgeADE palettes, got %d", len(palettes))
	}

	expectedIDs := []string{
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
}
