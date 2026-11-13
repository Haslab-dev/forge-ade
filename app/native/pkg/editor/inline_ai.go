package editor

// inline_ai.go ports the webview's inline AI edit feature
// (features/editor/inlineAi): ⌘I opens a floating bar over the editor, the
// instruction runs through the LLM, the proposal diffs against the target
// range, and Accept/Reject/Retry resolve it.

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// InlineAI is the state of one inline AI edit session.
type InlineAI struct {
	mu sync.Mutex

	Open       bool
	Prompt     string
	Generating bool
	Error      string
	StartedAt  time.Time

	// Target document identity + affected range.
	FullPath    string
	RelPath     string
	OrigText    string // buffer text before the proposal
	SelStart    int    // byte offset of the selection (or 0 = whole file)
	SelEnd      int
	Proposed    string // replacement text proposed by the model
	HasProposal bool

	// Editor handles (set by the UI layer): get/apply buffer text.
	GetText    func() string
	ApplyRange func(start, end int, newText string)
}

// OpenForSelection arms the bar over the given document + selection.
func (ia *InlineAI) OpenForSelection(fullPath, relPath string, selStart, selEnd int, getText func() string, apply func(start, end int, newText string)) {
	ia.mu.Lock()
	defer ia.mu.Unlock()
	ia.Open = true
	ia.Prompt = ""
	ia.Error = ""
	ia.Proposed = ""
	ia.HasProposal = false
	ia.FullPath = fullPath
	ia.RelPath = relPath
	ia.SelStart = selStart
	ia.SelEnd = selEnd
	ia.GetText = getText
	ia.ApplyRange = apply
	if ia.GetText != nil {
		ia.OrigText = ia.GetText()
	}
}

func (ia *InlineAI) Close() {
	ia.mu.Lock()
	defer ia.mu.Unlock()
	ia.Open = false
	ia.Generating = false
}

// Generate runs the instruction against the target range. llmCall streams
// the model output; it receives the system and user prompts and a delta
// callback, so this package stays free of any LLM dependency.
func (ia *InlineAI) Generate(llmCall func(ctx context.Context, system, user string, onDelta func(string)) error, cancelCh <-chan struct{}) {
	ia.mu.Lock()
	if ia.GetText == nil || ia.Prompt == "" {
		ia.mu.Unlock()
		return
	}
	target := ia.OrigText
	if ia.SelEnd > ia.SelStart && ia.SelEnd <= len(ia.OrigText) {
		target = ia.OrigText[ia.SelStart:ia.SelEnd]
	}
	wholeFile := ia.SelEnd <= ia.SelStart
	instr := ia.Prompt
	ia.Generating = true
	ia.Error = ""
	ia.HasProposal = false
	ia.Proposed = ""
	ia.StartedAt = time.Now()
	ia.mu.Unlock()

	sys := "You are a code editing assistant inside an IDE. Apply the user's instruction to the given code. " +
		"Respond with ONLY the resulting code, no explanations, no markdown fences. " +
		"Preserve the original indentation style."
	user := fmt.Sprintf("Instruction: %s\n\nCode%s:\n%s",
		instr,
		map[bool]string{true: " (whole file)", false: " (selection)"}[wholeFile],
		target)

	var out strings.Builder
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-cancelCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	err := llmCall(ctx, sys, user, func(delta string) {
		out.WriteString(delta)
		ia.mu.Lock()
		ia.Proposed = out.String()
		ia.mu.Unlock()
	})

	ia.mu.Lock()
	ia.Generating = false
	if err != nil {
		ia.Error = err.Error()
	} else {
		text := cleanFences(ia.Proposed)
		ia.Proposed = text
		ia.HasProposal = text != ""
		if !ia.HasProposal {
			ia.Error = "empty response"
		}
	}
	ia.mu.Unlock()
}

// Accept applies the proposal to the buffer range.
func (ia *InlineAI) Accept() {
	ia.mu.Lock()
	defer ia.mu.Unlock()
	if !ia.HasProposal || ia.ApplyRange == nil {
		return
	}
	ia.ApplyRange(ia.SelStart, ia.SelEnd, ia.Proposed)
	ia.Open = false
	ia.HasProposal = false
}

// Reject dismisses the proposal.
func (ia *InlineAI) Reject() {
	ia.mu.Lock()
	defer ia.mu.Unlock()
	ia.Open = false
	ia.HasProposal = false
	ia.Proposed = ""
}

// cleanFences strips markdown code fences models sometimes add.
func cleanFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if nl := strings.Index(s, "\n"); nl >= 0 {
			s = s[nl+1:]
		}
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
	}
	return strings.Trim(s, "\n")
}

// NewInlineAI creates the shared inline-AI edit session state.
func NewInlineAI() *InlineAI {
	return &InlineAI{}
}
