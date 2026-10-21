package app

import (
	"testing"

	"github.com/Haslab-dev/RakitKode/agentclirc"
)

func TestEnrichAllAgentRuntimeConfigs(t *testing.T) {
	configs := []agentclirc.RuntimeConfig{
		{ID: "opencode", Name: "OpenCode"},
		{ID: "pi", Name: "Pi"},
		{ID: "ohmypi", Name: "OhMyPi"},
	}

	enriched := enrichAllAgentRuntimeConfigs(configs)

	var oc, pi, omp, cmd *agentclirc.RuntimeConfig
	for i := range enriched {
		switch enriched[i].ID {
		case "opencode":
			oc = &enriched[i]
		case "pi":
			pi = &enriched[i]
		case "ohmypi":
			omp = &enriched[i]
		case "commandcode":
			cmd = &enriched[i]
		}
	}

	if oc != nil && len(oc.Providers) > 0 {
		models := oc.Providers[0].Models
		if len(models) == 0 {
			t.Error("expected models for opencode")
		}
	}

	if pi != nil && len(pi.Providers) > 0 {
		models := pi.Providers[0].Models
		if len(models) == 0 {
			t.Error("expected models for pi")
		}
	}

	if omp != nil && len(omp.Providers) > 0 {
		models := omp.Providers[0].Models
		if len(models) == 0 {
			t.Error("expected models for omp")
		}
	}

	if cmd == nil {
		t.Error("expected commandcode to be added to configs")
	}
}
