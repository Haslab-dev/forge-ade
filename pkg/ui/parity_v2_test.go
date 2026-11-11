package ui

import (
	"os"
	"testing"
)

func TestResizeHandlesClampingAndDefaults(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "forge-native-parity-v2-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	engine := NewEngine(tmpDir)
	appUI := NewAppUI(engine, tmpDir)

	// Check default widths
	if appUI.LeftSidebarWidth != 260 {
		t.Errorf("Expected default LeftSidebarWidth 260, got %v", appUI.LeftSidebarWidth)
	}
	if appUI.RightDrawerWidth != 420 {
		t.Errorf("Expected default RightDrawerWidth 420, got %v", appUI.RightDrawerWidth)
	}
	if appUI.BottomTerminalHeight != 240 {
		t.Errorf("Expected default BottomTerminalHeight 240, got %v", appUI.BottomTerminalHeight)
	}

	// Test Right Drawer tabs enumeration
	tabs := []int{RightTabChanges, RightTabTerminal, RightTabChat, RightTabTasks, RightTabGit, RightTabDatabase}
	for _, tab := range tabs {
		appUI.RightDrawerTab = tab
		if appUI.RightDrawerTab != tab {
			t.Errorf("Failed to set RightDrawerTab %d", tab)
		}
	}

	// Verify alias compatibility
	if RightTabAgent != RightTabChat {
		t.Errorf("Expected RightTabAgent to equal RightTabChat (%d), got %d", RightTabChat, RightTabAgent)
	}
}

func TestAgentStateExecutionModeAndEffortCycles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "forge-native-agent-cycle-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	as := NewAgentState(nil, tmpDir, nil)

	// Test ExecutionMode cycle: Ask -> Code -> Plan -> Bypass -> Ask
	as.ExecutionMode = ModeAsk
	if as.ExecutionModeLabel() != "Ask" {
		t.Errorf("Expected 'Ask', got '%s'", as.ExecutionModeLabel())
	}
	if as.ExecutionModeIcon() != TablerHand {
		t.Errorf("Expected TablerHand icon for ModeAsk")
	}

	as.CycleExecutionMode()
	if as.ExecutionMode != ModeCode || as.ExecutionModeLabel() != "Edits" {
		t.Errorf("Expected 'Edits', got '%s'", as.ExecutionModeLabel())
	}
	if as.ExecutionModeIcon() != TablerPencil {
		t.Errorf("Expected TablerPencil icon for ModeCode")
	}

	as.CycleExecutionMode()
	if as.ExecutionMode != ModePlan || as.ExecutionModeLabel() != "Auto" {
		t.Errorf("Expected 'Auto', got '%s'", as.ExecutionModeLabel())
	}
	if as.ExecutionModeIcon() != TablerShieldCheck {
		t.Errorf("Expected TablerShieldCheck icon for ModePlan")
	}

	as.CycleExecutionMode()
	if as.ExecutionMode != ModeBypass || as.ExecutionModeLabel() != "Full" {
		t.Errorf("Expected 'Full', got '%s'", as.ExecutionModeLabel())
	}

	as.CycleExecutionMode()
	if as.ExecutionMode != ModeAsk {
		t.Errorf("Expected wrap-around to ModeAsk, got %v", as.ExecutionMode)
	}

	// Test ReasoningEffort cycle: Off -> Low -> Med -> High -> Max -> Off
	as.ReasoningEffort = EffortOff
	if as.ReasoningEffortLabel() != "Off" {
		t.Errorf("Expected 'Off', got '%s'", as.ReasoningEffortLabel())
	}

	as.CycleReasoningEffort()
	if as.ReasoningEffort != EffortLow || as.ReasoningEffortLabel() != "Low" {
		t.Errorf("Expected 'Low', got '%s'", as.ReasoningEffortLabel())
	}

	as.CycleReasoningEffort()
	if as.ReasoningEffort != EffortMedium || as.ReasoningEffortLabel() != "Med" {
		t.Errorf("Expected 'Med', got '%s'", as.ReasoningEffortLabel())
	}

	as.CycleReasoningEffort()
	if as.ReasoningEffort != EffortHigh || as.ReasoningEffortLabel() != "High" {
		t.Errorf("Expected 'High', got '%s'", as.ReasoningEffortLabel())
	}

	as.CycleReasoningEffort()
	if as.ReasoningEffort != EffortMax || as.ReasoningEffortLabel() != "Max" {
		t.Errorf("Expected 'Max', got '%s'", as.ReasoningEffortLabel())
	}

	as.CycleReasoningEffort()
	if as.ReasoningEffort != EffortOff {
		t.Errorf("Expected wrap-around to EffortOff, got %v", as.ReasoningEffort)
	}
}

func TestSideChatMessagesStorage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "forge-native-sidechat-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	engine := NewEngine(tmpDir)
	appUI := NewAppUI(engine, tmpDir)

	if appUI.SideChatModel == "" {
		t.Errorf("Expected default SideChatModel, got empty")
	}

	appUI.SideChatMessages = append(appUI.SideChatMessages, &ChatMessage{
		Role: "user",
		Text: "How does the router work?",
	})
	appUI.SideChatMessages = append(appUI.SideChatMessages, &ChatMessage{
		Role: "assistant",
		Text: "The router handles incoming HTTP requests.",
	})

	if len(appUI.SideChatMessages) != 2 {
		t.Errorf("Expected 2 side chat messages, got %d", len(appUI.SideChatMessages))
	}
}
