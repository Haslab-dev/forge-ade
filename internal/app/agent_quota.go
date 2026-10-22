package app

import (
	"github.com/hasdev/forge-ade/internal/quota"
)

// GetAgentQuota returns account, tier, and real-time quota usage for agents (e.g. Command Code, Antigravity, etc.)
func (a *App) GetAgentQuota(agentID string) (map[string]interface{}, error) {
	return quota.GetAgentQuota(agentID, func() map[string]interface{} {
		if a == nil {
			return nil
		}
		return a.GetUsageSummary()
	})
}

// formatAntigravityResetDuration preserves backwards compatibility for internal tests.
func formatAntigravityResetDuration(remainingFraction float64, resetTimeStr string) string {
	return quota.FormatAntigravityResetDuration(remainingFraction, resetTimeStr)
}
