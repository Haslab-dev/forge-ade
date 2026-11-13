package app

import (
	"testing"
)

func TestCommandCodeDiscovery(t *testing.T) {
	cfg := commandCodeRuntimeConfig()
	if cfg == nil {
		t.Skip("Command Code not installed or no ~/.commandcode found on this machine")
	}

	if cfg.ID != "commandcode" {
		t.Errorf("expected ID 'commandcode', got '%s'", cfg.ID)
	}
	if cfg.Executable != "cmd" {
		t.Errorf("expected Executable 'cmd', got '%s'", cfg.Executable)
	}

	if len(cfg.Providers) > 0 {
		models := cfg.Providers[0].Models
		if len(models) == 0 {
			t.Error("expected non-empty model list in providers")
		}
	}
}
