package main

import (
	"fmt"
	"log"

	"github.com/hasdev/forge-ade/internal/agent"
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

// fireAutomation is the scheduler's fire callback: spawn an agent session in
// the automation's workspace, seed it with the prompt, and record the run.
func (a *App) fireAutomation(auto automations.Automation) {
	if a.ctx == nil || a.agentMgr == nil {
		return
	}
	sess, err := a.agentMgr.CreateSession(auto.Name, agent.RoleCoding, auto.Workspace)
	if err != nil {
		log.Printf("automations: failed to create session for %q: %v", auto.Name, err)
		return
	}
	if err := a.agentMgr.SendMessage(a.ctx, sess.ID, auto.Prompt, nil); err != nil {
		log.Printf("automations: failed to send prompt for %q: %v", auto.Name, err)
		return
	}
	if _, err := a.autoMgr.RecordRun(auto.ID, automations.Run{
		SessionID: sess.ID,
		Workspace: auto.Workspace,
	}); err != nil {
		log.Printf("automations: failed to record run for %q: %v", auto.Name, err)
	}
	a.emitEvent("automations:changed", map[string]interface{}{})
	log.Printf("automations: fired %q → session %s", auto.Name, sess.ID)
}

// CommandResult represents stdout/stderr from executing a command.
