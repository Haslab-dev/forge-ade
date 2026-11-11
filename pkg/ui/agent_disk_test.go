package ui

import (
	"os"
	"testing"
)

func TestDiskSessionPersistence(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "disk-sess-test-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	engine := NewEngine(tempDir)
	as1 := NewAgentState(engine, tempDir, nil)

	// Create a new real session
	sess := as1.NewSession("Real Persistent Feature Session")
	defer as1.DeleteSession(sess.ID)
	sess.Model = "gemini-3.8-flash-high"
	sess.Messages = append(sess.Messages, &ChatMessage{
		ID:   2,
		Role: "user",
		Text: "Hello real agent",
		Done: true,
	}, &ChatMessage{
		ID:   3,
		Role: "assistant",
		Text: "Hello! I am connected to the real persistence engine.",
		Done: true,
	})
	sess.Tasks.SetGoal("Implement Feature X", []TaskItem{
		{ID: "1", Title: "Read symbol definitions", Status: TaskCompleted},
		{ID: "2", Title: "Run tests", Status: TaskInProgress},
	})

	as1.SaveSessionToDisk(sess)

	// Now create a second AgentState instance pointing to the same workspace
	as2 := NewAgentState(engine, tempDir, nil)

	found := false
	for _, s := range as2.Sessions {
		if s.ID == sess.ID {
			found = true
			if s.Title != "Real Persistent Feature Session" {
				t.Fatalf("expected title 'Real Persistent Feature Session', got '%s'", s.Title)
			}
			if len(s.Messages) < 3 {
				t.Fatalf("expected at least 3 messages loaded from disk, got %d", len(s.Messages))
			}
			if s.Tasks == nil || s.Tasks.Goal != "Implement Feature X" {
				t.Fatalf("expected goal 'Implement Feature X', got %v", s.Tasks)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected session %s to be reloaded from disk", sess.ID)
	}
}
