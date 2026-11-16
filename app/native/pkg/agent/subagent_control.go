package agent

// subagent_control.go exposes UI-facing control over individual subagent
// turns of the active internal session (stop one running subagent from the
// floating turn-status card).

// StopSubagent cancels the running turn of one subagent belonging to the
// given parent session. Safe to call from any goroutine.
func (as *AgentState) StopSubagent(parentSessionID, subagentID string) {
	if as == nil || as.engine == nil || as.engine.AgentMgr == nil {
		return
	}
	mgrID := as.internalManagerID(parentSessionID)
	if mgrID == "" {
		return
	}
	as.engine.AgentMgr.StopTurn(subagentID)
}
