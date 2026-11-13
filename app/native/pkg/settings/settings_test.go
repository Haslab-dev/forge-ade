package settings

import (
	"testing"
)

func TestSettingsManager(t *testing.T) {
	var savedProvider, savedModel string
	sm := NewSettingsManager(nil, func(provider, model string) {
		savedProvider = provider
		savedModel = model
	})

	if sm.Provider != "" {
		t.Fatalf("expected initial provider to be empty, got '%s'", sm.Provider)
	}

	sm.ProviderIdx = 0 // "openai"
	sm.Model = "gpt-4o"
	sm.APIKey = "sk-test"

	if err := sm.Save(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	if savedProvider != "openai" || savedModel != "gpt-4o" {
		t.Fatalf("expected saved ('openai', 'gpt-4o'), got ('%s', '%s')", savedProvider, savedModel)
	}

	// Toggle
	if sm.IsOpen {
		t.Fatalf("expected initial IsOpen false")
	}
	sm.Toggle()
	if !sm.IsOpen {
		t.Fatalf("expected IsOpen true after Toggle")
	}

	// Test multi-tab navigation
	for _, tab := range []int{SettingsTabModels, SettingsTabAppearance, SettingsTabAgents, SettingsTabShortcuts} {
		sm.ActiveTab = tab
		if sm.ActiveTab != tab {
			t.Fatalf("expected active tab %d, got %d", tab, sm.ActiveTab)
		}
	}
}
