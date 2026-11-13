package agent

import (
	"strings"
	"testing"
)

func TestCleanRawToolTags(t *testing.T) {
	// Complete DSML tool call block
	in := "Here is my plan.\n<|DSML|calls><｜invoke name=\"update_tasks\">{\"tasks\":[]}</｜invoke></|DSML|calls>\nDone!"
	out := cleanRawToolTags(in)
	if strings.Contains(out, "DSML") || strings.Contains(out, "update_tasks") {
		t.Fatalf("tool block not removed: %q", out)
	}
	if !strings.Contains(out, "Here is my plan.") || !strings.Contains(out, "Done!") {
		t.Fatalf("prose lost: %q", out)
	}

	// tool_call tags
	out = cleanRawToolTags("a <tool_call>{\"x\":1}</tool_call> b")
	if out != "a  b" && out != "a b" {
		t.Fatalf("tool_call not removed: %q", out)
	}

	// Unclosed block at end while streaming
	out = cleanRawToolTags("prose...\n<|DSML|calls>{\"partial")
	if strings.Contains(out, "DSML") || strings.Contains(out, "partial") {
		t.Fatalf("unclosed block not removed: %q", out)
	}
	if !strings.Contains(out, "prose...") {
		t.Fatalf("prose lost: %q", out)
	}

	// Clean text passes through
	if got := cleanRawToolTags("plain reply"); got != "plain reply" {
		t.Fatalf("clean text altered: %q", got)
	}
}

func TestCleanTaskJSONBlocks(t *testing.T) {
	in := "Saya sudah memproses.\n\n" +
		`{"completed":1,"current_focus":"","in_progress":0,"just_completed":["Inspect board"],"pending":2,"summary":"Progress: 1/3 tasks completed.\nStatus: 1 completed"}` + "\n\n" +
		"Selesai.\n\n" +
		`{"active_form":"Inspecting board","id":"1","status":"completed","title":"Inspect board"},{"active_form":"Creating task","id":"2","status":"pending","title":"Create task"},"total":3}`
	out := cleanTaskJSONBlocks(in)
	if strings.Contains(out, "active_form") || strings.Contains(out, "just_completed") {
		t.Fatalf("JSON blobs not removed: %q", out)
	}
	if !strings.Contains(out, "Saya sudah memproses.") || !strings.Contains(out, "Selesai.") {
		t.Fatalf("prose lost: %q", out)
	}
}
