package ui

import (
	"testing"
)

func TestSettingsManager(t *testing.T) {
	var savedProvider, savedModel string
	sm := NewSettingsManager(nil, func(provider, model string) {
		savedProvider = provider
		savedModel = model
	})

	if sm.Provider != "anthropic" {
		t.Fatalf("expected initial provider 'anthropic', got '%s'", sm.Provider)
	}

	sm.providerIdx = 0 // "openai"
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
}
