package editor

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestInlineAIProposalLifecycle(t *testing.T) {
	ia := NewInlineAI()
	buf := "func main() {\n\tprintln(1)\n}\n"

	ia.OpenForSelection("f.go", "f.go", 0, len(buf), func() string { return buf }, nil)
	if !ia.Open || ia.HasProposal {
		t.Fatalf("open state = %v proposal=%v", ia.Open, ia.HasProposal)
	}
	if ia.SelStart != 0 || ia.SelEnd != len(buf) {
		t.Fatalf("selection = %d..%d", ia.SelStart, ia.SelEnd)
	}
	ia.Prompt = "add a comment"

	done := make(chan struct{})
	go func() {
		ia.Generate(func(ctx context.Context, system, user string, onDelta func(string)) error {
			onDelta("// hello\n")
			return nil
		}, make(chan struct{}))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Generate hung")
	}

	if !ia.HasProposal || ia.Error != "" {
		t.Fatalf("proposal=%v err=%q", ia.HasProposal, ia.Error)
	}
	if ia.Proposed != "// hello" {
		t.Fatalf("proposed = %q", ia.Proposed)
	}
	ia.Reject()
	if ia.Open || ia.HasProposal {
		t.Fatal("reject did not close")
	}
}

func TestCleanFences(t *testing.T) {
	in := "```go\nfmt.Println()\n```"
	if got := cleanFences(in); got != "fmt.Println()" {
		t.Fatalf("cleanFences = %q", got)
	}
	if got := cleanFences("plain"); got != "plain" {
		t.Fatalf("plain = %q", got)
	}
	if !strings.Contains(cleanFences("```\nabc\n```"), "abc") {
		t.Fatal("bare fence content lost")
	}
}
