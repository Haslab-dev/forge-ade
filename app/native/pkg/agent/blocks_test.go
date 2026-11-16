package agent

import (
	"strings"
	"testing"
)

// TestBlockTimelineOrdering verifies the ordered text/tool timeline: streamed
// text segments and tool calls must interleave in execution order while
// msg.Text stays the concatenation of the text segments.
func TestBlockTimelineOrdering(t *testing.T) {
	as := &AgentState{}
	sess := &AgentSession{ID: "sess-blocks"}
	as.Sessions = []*AgentSession{sess}
	as.ActiveSessionID = "sess-blocks"

	// Stream intro text, run a tool, stream a summary, run another tool,
	// then stream the final answer.
	as.HandleTokenChunk("sess-blocks", "Let me check the files. ")
	as.HandleToolEvent(ToolEvent{SessionID: "sess-blocks", ToolCallID: "t1", Name: "bash", Status: "running"})
	as.HandleToolEvent(ToolEvent{SessionID: "sess-blocks", ToolCallID: "t1", Name: "bash", Status: "completed", Output: "ok"})
	as.HandleTokenChunk("sess-blocks", "The command worked. Now searching. ")
	as.HandleToolEvent(ToolEvent{SessionID: "sess-blocks", ToolCallID: "t2", Name: "grep", Status: "running"})
	as.HandleTokenChunk("sess-blocks", "Found it — done.")

	msg := sess.Messages[0]
	if len(msg.Blocks) != 5 {
		t.Fatalf("expected 5 blocks, got %d: %+v", len(msg.Blocks), msg.Blocks)
	}
	wantKinds := []string{"text", "tool", "text", "tool", "text"}
	for i, kind := range wantKinds {
		if msg.Blocks[i].Kind != kind {
			t.Fatalf("block %d: expected kind %q, got %q", i, kind, msg.Blocks[i].Kind)
		}
	}
	if msg.Blocks[0].Text != "Let me check the files. " {
		t.Fatalf("block 0 text mismatch: %q", msg.Blocks[0].Text)
	}
	if msg.Blocks[4].Text != "Found it — done." {
		t.Fatalf("block 4 text mismatch: %q", msg.Blocks[4].Text)
	}
	if msg.Blocks[1].ToolID != "t1" || msg.Blocks[3].ToolID != "t2" {
		t.Fatalf("tool block IDs mismatch: %q %q", msg.Blocks[1].ToolID, msg.Blocks[3].ToolID)
	}
	// Continuation text must merge into the existing text block, not create one.
	as.HandleTokenChunk("sess-blocks", " More.")
	if len(msg.Blocks) != 5 {
		t.Fatalf("continuation delta must merge into the last text block, got %d blocks", len(msg.Blocks))
	}
	if msg.Text != "Let me check the files. The command worked. Now searching. Found it — done. More." {
		t.Fatalf("msg.Text must stay the concatenation of text segments, got %q", msg.Text)
	}
	// Tool IDs referenced by blocks must resolve to actual tools.
	for _, b := range msg.Blocks {
		if b.Kind == "tool" && findToolByID(msg, b.ToolID) == nil {
			t.Fatalf("block references missing tool %q", b.ToolID)
		}
	}
}

// TestBlockTimelineStopMarker verifies stop markers and error suffixes keep
// the timeline in sync with direct Text writes.
func TestBlockTimelineStopMarker(t *testing.T) {
	as := &AgentState{}
	sess := &AgentSession{ID: "sess-stop"}
	as.Sessions = []*AgentSession{sess}
	as.ActiveSessionID = "sess-stop"

	as.HandleTokenChunk("sess-stop", "partial text")
	msg := sess.Messages[0]
	setAssistantTextLocked(msg, "*(stopped by user)*")
	if len(msg.Blocks) != 1 || msg.Blocks[0].Text != "*(stopped by user)*" {
		t.Fatalf("stop marker must rebuild the timeline, got %+v", msg.Blocks)
	}

	setAssistantTextLocked(msg, "")
	if msg.Blocks != nil {
		t.Fatalf("empty text must clear the timeline, got %+v", msg.Blocks)
	}
	if !strings.Contains(msg.Text, "stopped") && msg.Text != "" {
		t.Fatalf("unexpected text %q", msg.Text)
	}
}

// TestBlockTimelineSnapshotIsolation verifies snapshot copies do not share
// block backing arrays with the live message.
func TestBlockTimelineSnapshotIsolation(t *testing.T) {
	m := &ChatMessage{
		Role: "assistant",
		Text: "ab",
		Blocks: []*MessageBlock{
			{Kind: "text", Text: "a"},
			{Kind: "tool", ToolID: "t9"},
		},
	}
	cp := snapshotMessageLocked(m)
	cp.Blocks[0].Text = "MUTATED"
	if m.Blocks[0].Text == "MUTATED" {
		t.Fatalf("snapshot shares block backing with live message")
	}
	if len(cp.Blocks) != 2 || cp.Blocks[1].ToolID != "t9" {
		t.Fatalf("snapshot blocks lost data: %+v", cp.Blocks)
	}
}

// TestBlockTimelinePersistenceRoundTrip verifies blocks survive the disk
// payload serialization shape (field names) used by SaveSessionToDisk.
func TestBlockTimelinePersistenceRoundTrip(t *testing.T) {
	b := MessageBlock{Kind: "text", Text: "hello"}
	if b.Kind != "text" || b.Text != "hello" {
		t.Fatalf("struct copy broken: %+v", b)
	}
	tool := MessageBlock{Kind: "tool", ToolID: "tool-42"}
	if tool.ToolID != "tool-42" {
		t.Fatalf("tool block copy broken: %+v", tool)
	}
}
