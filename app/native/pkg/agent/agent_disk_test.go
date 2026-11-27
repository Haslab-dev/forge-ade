package agent

import (
	"os"
	"testing"
	"time"

	"github.com/hasdev/forge-ade/app/native/pkg/engine"
)

func TestDiskSessionPersistence(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "disk-sess-test-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	eng := engine.NewEngine(tempDir)
	as1 := NewAgentState(eng, tempDir, nil)

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
	as2 := NewAgentState(eng, tempDir, nil)

	found := false
	for _, s := range as2.Sessions {
		if s.ID == sess.ID {
			found = true
			if s.Title != "Real Persistent Feature Session" {
				t.Fatalf("expected title 'Real Persistent Feature Session', got '%s'", s.Title)
			}
			if len(s.Messages) < 2 {
				t.Fatalf("expected at least 2 messages loaded from disk, got %d", len(s.Messages))
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

func TestSteerQueuePersistenceAndTurnDrain(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "steer-queue-test-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	eng := engine.NewEngine(tempDir)
	as1 := NewAgentState(eng, tempDir, nil)
	sess := as1.NewSession("Steer Test")
	defer as1.DeleteSession(sess.ID)
	as1.SaveSessionToDisk(sess)

	// Mutate under Lock/Unlock and persist outside the lock without deadlocking.
	done := make(chan struct{})
	go func() {
		defer close(done)
		as1.Lock()
		as1.SteerQueue = []string{"first steer", "second steer", "third steer"}
		// Remove middle item as tray delete does.
		as1.SteerQueue = append(as1.SteerQueue[:1], as1.SteerQueue[2:]...)
		as1.Unlock()
		as1.SaveSteerQueues()
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("steer queue mutation + SaveSteerQueues deadlocked")
	}

	// Verify reload onto a fresh AgentState.
	as2 := NewAgentState(eng, tempDir, nil)
	if len(as2.SteerQueue) != 2 || as2.SteerQueue[0] != "first steer" || as2.SteerQueue[1] != "third steer" {
		t.Fatalf("expected restored SteerQueue [first steer, third steer], got %v", as2.SteerQueue)
	}

	// Verify HandleTurnDone drains one steer item cleanly without deadlocking.
	as2.Lock()
	as2.ActiveSessionID = sess.ID
	as2.Unlock()
	drainCh := make(chan string, 1)
	go func() {
		drainCh <- as2.HandleTurnDone(sess.ID, TurnEndInfo{Status: "completed"})
	}()
	select {
	case next := <-drainCh:
		if next != "first steer" {
			t.Fatalf("expected next steer 'first steer', got %q", next)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("HandleTurnDone deadlocked draining steer queue")
	}

	if len(as2.SteerQueue) != 1 || as2.SteerQueue[0] != "third steer" {
		t.Fatalf("expected remaining SteerQueue [third steer], got %v", as2.SteerQueue)
	}
}

func TestHandoffAndSubsessionMetadataDiskPersistence(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "handoff-disk-test-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	eng := engine.NewEngine(tempDir)
	as1 := NewAgentState(eng, tempDir, nil)
	sess := as1.NewSession("Branched Handoff")
	defer as1.DeleteSession(sess.ID)

	as1.Lock()
	sess.HandoffFromID = "src-session-1"
	sess.HandoffFromAgent = "Claude"
	sess.HandoffToIDs = []string{"child-2"}
	sess.ParentSessionID = "parent-0"
	sess.WorkerName = "worker-a"
	sess.BotID = "bot-1"
	as1.Unlock()

	as1.SaveSessionToDisk(sess)

	as2 := NewAgentState(eng, tempDir, nil)
	as2.LoadSessionsFromDisk(tempDir)

	var reloaded *AgentSession
	for _, s := range as2.AllSessions() {
		if s.ID == sess.ID {
			reloaded = s
			break
		}
	}
	if reloaded == nil {
		t.Fatalf("expected reloaded session %s", sess.ID)
	}
	if reloaded.HandoffFromID != "src-session-1" || reloaded.HandoffFromAgent != "Claude" {
		t.Fatalf("expected handoff fields preserved, got from=%q agent=%q", reloaded.HandoffFromID, reloaded.HandoffFromAgent)
	}
	if len(reloaded.HandoffToIDs) != 1 || reloaded.HandoffToIDs[0] != "child-2" {
		t.Fatalf("expected HandoffToIDs=[child-2], got %v", reloaded.HandoffToIDs)
	}
	if reloaded.ParentSessionID != "parent-0" || reloaded.WorkerName != "worker-a" || reloaded.BotID != "bot-1" {
		t.Fatalf("expected subsession fields preserved, got parent=%q worker=%q bot=%q",
			reloaded.ParentSessionID, reloaded.WorkerName, reloaded.BotID)
	}
}

func TestCompactActiveSessionDoesNotDeadlock(t *testing.T) {
	tempDir := t.TempDir()
	eng := engine.NewEngine(tempDir)
	as := NewAgentState(eng, tempDir, nil)
	sess := as.NewSession("Compact Test")
	defer as.DeleteSession(sess.ID)

	as.Lock()
	for i := 0; i < 10; i++ {
		sess.Messages = append(sess.Messages, &ChatMessage{ID: i + 1, Role: "user", Text: "msg", Done: true})
	}
	sess.LastUsage = &UsageInfo{TotalTokens: 1500000}
	as.Messages = sess.Messages
	as.Unlock()

	done := make(chan struct{})
	var removed int
	var err error
	go func() {
		defer close(done)
		removed, err = as.CompactActiveSession()
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("CompactActiveSession deadlocked")
	}
	if err != nil {
		t.Fatalf("CompactActiveSession error: %v", err)
	}
	if removed != 6 {
		t.Fatalf("expected 6 removed messages, got %d", removed)
	}
	if len(sess.Messages) != 4 {
		t.Fatalf("expected 4 remaining messages, got %d", len(sess.Messages))
	}
	if sess.LastUsage != nil {
		t.Fatalf("expected LastUsage cleared after compact")
	}
}
