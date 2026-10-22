package app

import (
	"github.com/Haslab-dev/RakitKode/agentclirc"
	"github.com/hasdev/forge-ade/internal/agentsession"
)

func enrichAllAgentRuntimeConfigs(configs []agentclirc.RuntimeConfig) []agentclirc.RuntimeConfig {
	return agentsession.EnrichAllAgentRuntimeConfigs(configs)
}
