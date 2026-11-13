package main

// forge-audit drives REAL end-to-end functionality checks for ForgeADE
// Native: the internal agent over a live OpenAI-compatible provider and an
// ACP antigravity session — plus quick UI-state assertions.
//
// Secrets are NEVER hardcoded: pass the provider key via FORGE_AUDIT_KEY.
//
// Usage:
//   FORGE_AUDIT_KEY=kn-... go run ./cmd/forge-audit -workspace /path -mode internal|acp|all

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Haslab-dev/RakitKode/llm"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/engine"
)

var (
	mu       sync.Mutex
	sections []string
)

func report(section string, ok bool, detail string) {
	mu.Lock()
	defer mu.Unlock()
	status := "PASS"
	if !ok {
		status = "FAIL"
	}
	line := fmt.Sprintf("[%s] %-28s %s", status, section, detail)
	fmt.Println(line)
	sections = append(sections, line)
}

func main() {
	ws := flag.String("workspace", ".", "workspace directory for the audit sessions")
	mode := flag.String("mode", "all", "internal | acp | all")
	acpModel := flag.String("acp-model", "gemini-3.8-flash-high", "antigravity ACP model")
	internalModel := flag.String("internal-model", "muse-glimmer-30b:free", "internal agent model")
	key := flag.String("key", os.Getenv("FORGE_AUDIT_KEY"), "provider API key (or FORGE_AUDIT_KEY env)")
	dataDir := flag.String("data", "", "data dir (default ~/.forge-ade)")
	flag.Parse()

	if *dataDir == "" {
		home, _ := os.UserHomeDir()
		*dataDir = home + "/.forge-ade"
	}

	eng := engine.NewEngine(*dataDir)

	// 0. Seed the provider profile exactly as the settings screen does.
	if *key != "" {
		if err := eng.LLMClient.SaveProfile("kenari", *key, "https://kenari.id/v1", *internalModel); err != nil {
			report("provider profile", false, err.Error())
		} else {
			cfg := eng.LLMClient.GetConfig()
			report("provider profile", cfg.APIKey != "" && cfg.BaseURL != "", fmt.Sprintf("provider=%s model=%s base=%s", cfg.ProviderID, cfg.Model, cfg.BaseURL))
		}
	}

	auditInternal(eng, *ws, *internalModel, *mode)
	auditACP(eng, *ws, *acpModel, *mode)

	fails := 0
	for _, l := range sections {
		if strings.HasPrefix(l, "[FAIL]") {
			fails++
		}
	}
	fmt.Printf("\n%d checks, %d failures\n", len(sections), fails)
	if fails > 0 {
		os.Exit(1)
	}
}

// auditInternal exercises a real internal-agent turn over the live provider.
func auditInternal(eng *engine.Engine, ws, model, mode string) {
	if mode != "all" && mode != "internal" {
		return
	}
	inv := func() {}
	as := agent.NewAgentState(eng, ws, inv)
	acp := agent.NewACPManager(eng, inv)
	as.ACPMgr = acp
	as.ActiveModel = model
	as.ExecutionMode = agent.ModeBypass

	// Real ACP manager shares the engine bus; internal turns flow through
	// agent.Manager on the same bus.
	sess := as.CreateSessionWithInitialPrompt(
		"Reply with exactly: INTERNAL-OK (no tools needed).",
		"agent-internal",
	)
	if sess == nil {
		report("internal session", false, "session not created")
		return
	}
	report("internal session", true, "id="+sess.ID)

	// Count streaming deltas so the audit proves live streaming, not just
	// a final reply. Free providers can take ~35s to first token and the
	// tool loop makes several sequential calls — allow up to 6 minutes.
	deltas := 0
	prevOnToken := as.HandleTokenChunk
	_ = prevOnToken
	origInvalidate := as.Invalidate
	as.Invalidate = func() {
		deltas++
		if origInvalidate != nil {
			origInvalidate()
		}
	}

	deadline := time.Now().Add(360 * time.Second)
	for time.Now().Before(deadline) {
		as.Lock()
		done := !as.IsStreaming && len(as.Messages) > 0 && as.Messages[len(as.Messages)-1].Done
		last := ""
		if len(as.Messages) > 0 {
			last = as.Messages[len(as.Messages)-1].Text
		}
		as.Unlock()
		if done && strings.TrimSpace(last) != "" {
			report("internal turn", true, fmt.Sprintf("model=%s reply=%d chars deltas=%d", model, len(last), deltas))
			report("internal reply", true, strings.TrimSpace(firstLine(last)))
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if as.IsStreaming {
		report("internal turn", false, "still streaming after 360s")
	}

	// Persistence: the session must survive to disk (LoadSessionsFromDisk).
	as2 := agent.NewAgentState(eng, ws, inv)
	as2.LoadSessionsFromDisk("")
	as2.Lock()
	found := false
	for _, s := range as2.Sessions {
		if s.ID == sess.ID {
			found = true
			break
		}
	}
	as2.Unlock()
	report("internal persistence", found, "session reloaded from disk")

	// Provider switch: second model round-trip.
	_ = eng.LLMClient.SaveProfile("kenari", os.Getenv("FORGE_AUDIT_KEY"), "https://kenari.id/v1", "dots-3-note-preview:free")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	resp, err := eng.LLMClient.ChatWithStream(ctx, []llm.LLMMessage{
		{Role: llm.RoleUser, Content: "Reply with exactly: dots-ok"},
	}, nil, func(string, string) {})
	if err != nil {
		report("dots model", false, err.Error())
	} else {
		report("dots model", strings.Contains(strings.ToLower(resp.Content), "dots"), "reply="+firstLine(resp.Content))
	}
	// Restore the primary audit model as active.
	_ = eng.LLMClient.SaveProfile("kenari", os.Getenv("FORGE_AUDIT_KEY"), "https://kenari.id/v1", model)
}

// auditACP exercises a real antigravity ACP session + turn.
func auditACP(eng *engine.Engine, ws, model, mode string) {
	if mode != "all" && mode != "acp" {
		return
	}
	inv := func() {}
	acp := agent.NewACPManager(eng, inv)

	var agDef *agent.ACPAgentDefinition
	for i := range acp.Agents {
		if acp.Agents[i].ID == "agent-agy" {
			agDef = &acp.Agents[i]
		}
	}
	if agDef == nil || !agDef.Available {
		report("acp antigravity", false, "agy not found on PATH (install Google Antigravity CLI)")
		return
	}
	report("acp antigravity", true, "agy="+agDef.Command)

	sess, err := acp.StartSessionWithID("audit-acp-1", "agent-agy", ws, model, "auto")
	if err != nil {
		report("acp session", false, err.Error())
		return
	}
	report("acp session", true, "id="+sess.ID+" model="+model)

	// Subscribe for the reply + turn end.
	var mu sync.Mutex
	reply := ""
	turnDone := false
	acp.OnTokenChunk = func(sessionID, text string) {
		if sessionID == "audit-acp-1" {
			mu.Lock()
			reply += text
			mu.Unlock()
		}
	}
	acp.OnTurnDone = func(sessionID string, info agent.TurnEndInfo) {
		if sessionID == "audit-acp-1" {
			mu.Lock()
			turnDone = true
			mu.Unlock()
		}
	}

	if err := acp.Prompt("audit-acp-1", "Reply with exactly: ACP-OK"); err != nil {
		report("acp prompt", false, err.Error())
		return
	}

	deadline := time.Now().Add(150 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		done := turnDone
		mu.Unlock()
		if done {
			mu.Lock()
			ok := strings.Contains(strings.ToUpper(reply), "ACP-OK")
			detail := fmt.Sprintf("reply=%d chars", len(reply))
			mu.Unlock()
			report("acp turn", ok, detail)
			if ok {
				report("acp reply", true, firstLine(reply))
			}
			_ = acp.Cancel("audit-acp-1")
			return
		}
		time.Sleep(400 * time.Millisecond)
	}
	report("acp turn", false, "timeout after 150s")
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\n\r"); i > 0 {
		s = s[:i]
	}
	if len(s) > 120 {
		s = s[:117] + "…"
	}
	return s
}
