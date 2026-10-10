package app

import (
	"fmt"
	"log"
	"time"

	"github.com/hasdev/forge-ade/internal/automations"
)

// ---------------------------------------------------------------------------
// Automations API (saved prompt workflows with run history)
// ---------------------------------------------------------------------------

// ListAutomations returns saved automations, newest-updated first.
func (a *App) ListAutomations() []automations.Automation {
	if a.autoMgr == nil {
		return []automations.Automation{}
	}
	return a.autoMgr.List()
}

// SaveAutomation upserts an automation (empty ID creates one).
func (a *App) SaveAutomation(input automations.AutomationSaveInput) (automations.Automation, error) {
	auto := input.ToAutomation()
	if a.autoMgr == nil {
		return automations.Automation{}, fmt.Errorf("automations manager unavailable")
	}
	saved, err := a.autoMgr.Save(auto)
	if err == nil {
		a.emitEvent("automations:changed", map[string]interface{}{})
	}
	return saved, err
}

// DeleteAutomation removes an automation by ID.
func (a *App) DeleteAutomation(id string) error {
	if a.autoMgr == nil {
		return fmt.Errorf("automations manager unavailable")
	}
	err := a.autoMgr.Delete(id)
	if err == nil {
		a.emitEvent("automations:changed", map[string]interface{}{})
	}
	return err
}

// RecordAutomationRun links a run to an automation's history. The session is
// created by the frontend (an agent session seeded with the automation's
// prompt) and referenced here so run history can reopen the conversation.
func (a *App) RecordAutomationRun(id string, sessionID string, workspace string) (automations.Automation, error) {
	if a.autoMgr == nil {
		return automations.Automation{}, fmt.Errorf("automations manager unavailable")
	}
	updated, err := a.autoMgr.RecordRun(id, automations.Run{
		SessionID: sessionID,
		Workspace: workspace,
	})
	if err == nil {
		a.emitEvent("automations:changed", map[string]interface{}{})
	}
	return updated, err
}

// SetAutomationEnabled pauses or resumes a scheduled automation.
func (a *App) SetAutomationEnabled(id string, enabled bool) (automations.Automation, error) {
	if a.autoMgr == nil {
		return automations.Automation{}, fmt.Errorf("automations manager unavailable")
	}
	updated, err := a.autoMgr.SetEnabled(id, enabled)
	if err == nil {
		a.emitEvent("automations:changed", map[string]interface{}{})
	}
	return updated, err
}

// fireAutomation is the scheduler's fire callback. It hands the fire to the
// frontend via an event: the UI runs it through the exact same internal-agent
// path as "Run now" (composer session, streaming, run history), so a
// scheduled run is visible instead of landing in a backend-only session.
// The token makes exactly one window claim the fire when several are open.
func (a *App) fireAutomation(auto automations.Automation) {
	if a.autoMgr == nil {
		return
	}
	token := fmt.Sprintf("%s-%d", auto.ID, time.Now().UnixNano())
	if _, err := a.autoMgr.BeginFire(auto.ID, token); err != nil {
		log.Printf("automations: begin fire failed for %q: %v", auto.Name, err)
		return
	}
	a.emitEvent("automations:fire", map[string]interface{}{
		"id":    auto.ID,
		"token": token,
		"name":  auto.Name,
	})
	log.Printf("automations: scheduled fire %q (token %s)", auto.Name, token)
}

// ClaimAutomationFire consumes a pending fire token; only the first caller
// (one window) may execute the automation.
func (a *App) ClaimAutomationFire(id string, token string) bool {
	if a.autoMgr == nil {
		return false
	}
	return a.autoMgr.ClaimFire(id, token)
}

// CommandResult represents stdout/stderr from executing a command.
