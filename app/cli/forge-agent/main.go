// forge-agent runs the ForgeADE agent engine headlessly.
//
// It wires the same RakitKode subsystems as the desktop bootstrap (LLM,
// tools, skills, plugins, memory, MCP, shell) but without any GUI: give it a
// workspace and a prompt and it streams the agent's turn to stdout, so CI,
// scripts, or other apps can spawn ForgeADE sub-agents.
//
// Examples:
//
//	forge-agent --workspace . --prompt "Summarize the architecture of this repo"
//	forge-agent --workspace ./api --prompt "Fix the failing tests" --approve-all
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Haslab-dev/RakitKode/agent"
	"github.com/Haslab-dev/RakitKode/events"
	"github.com/Haslab-dev/RakitKode/llm"
	"github.com/Haslab-dev/RakitKode/mcp"
	"github.com/Haslab-dev/RakitKode/memory"
	"github.com/Haslab-dev/RakitKode/plugins"
	"github.com/Haslab-dev/RakitKode/search"
	"github.com/Haslab-dev/RakitKode/skills"
	"github.com/Haslab-dev/RakitKode/store"
	"github.com/Haslab-dev/RakitKode/tools"
	"github.com/hasdev/forge-ade/core/terminal"
)

// shellManager adapts core/terminal's PTY manager to the engine's
// agent.ShellManager interface, mirroring the desktop app's adapter.
type shellManager struct {
	*terminal.Manager
}

func (s shellManager) CreateShell(name, folder string) (agent.ShellSession, error) {
	return s.Manager.CreateShell(name, folder)
}

type options struct {
	workspace  string
	prompt     string
	name       string
	dataDir    string
	approveAll bool
	quiet      bool
	timeout    time.Duration
}

func main() {
	opts := options{}
	flag.StringVar(&opts.workspace, "workspace", ".", "workspace folder the agent operates on")
	flag.StringVar(&opts.prompt, "prompt", "", "task prompt to run (required)")
	flag.StringVar(&opts.name, "name", "forge-agent", "session name")
	flag.StringVar(&opts.dataDir, "data-dir", "", "ForgeADE data dir (default ~/.forge-ade)")
	flag.BoolVar(&opts.approveAll, "approve-all", false, "auto-approve every tool permission request")
	flag.BoolVar(&opts.quiet, "quiet", false, "suppress thinking output")
	flag.DurationVar(&opts.timeout, "timeout", 15*time.Minute, "turn timeout")
	flag.Parse()

	if opts.prompt == "" {
		fmt.Fprintln(os.Stderr, "forge-agent: --prompt is required")
		flag.Usage()
		os.Exit(2)
	}
	workspace, err := filepath.Abs(opts.workspace)
	if err != nil {
		fatal("resolve workspace: %v", err)
	}
	if info, err := os.Stat(workspace); err != nil || !info.IsDir() {
		fatal("workspace is not a directory: %s", workspace)
	}

	turnDone := run(opts, workspace)
	<-turnDone
}

// run wires the engine and drives one turn; it returns a channel closed when
// the turn (or the timeout / interrupt) settles.
func run(opts options, workspace string) <-chan struct{} {
	done := make(chan struct{})

	dataDir := opts.dataDir
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fatal("resolve home: %v", err)
		}
		dataDir = filepath.Join(home, ".forge-ade")
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		fatal("create data dir: %v", err)
	}

	bus := events.NewBus()
	store.SetRoot(dataDir)

	llmClient := llm.NewLLMClient(dataDir)
	toolReg := tools.NewRegistry(search.NewSearchManager())
	skillMgr := skills.NewManager(filepath.Join(dataDir, "skills"))
	pluginMgr := plugins.NewManager(dataDir, bus)
	memoryMgr := memory.New(dataDir)
	memoryMgr.SetGlobalDir(dataDir)
	mcpMgr := mcp.NewManager(dataDir)

	sm := terminal.NewManager(bus)
	agentMgr := agent.NewManagerWithOptions(agent.ManagerOptions{
		LLMClient:    llmClient,
		ToolRegistry: toolReg,
		Skills:       skillMgr,
		Plugins:      pluginMgr,
		MCP:          mcpMgr,
		Shell:        shellManager{Manager: sm},
		Bus:          bus,
		DataDir:      dataDir,
	})
	agentMgr.SetMemoryManager(memoryMgr)

	ctx, cancel := context.WithTimeout(context.Background(), opts.timeout)
	defer cancel()

	// Interrupt support: Ctrl+C stops the turn gracefully.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	sess, err := agentMgr.CreateSession(opts.name, agent.RoleFilter(""), workspace)
	if err != nil {
		fatal("create session: %v", err)
	}

	// Stream the turn.
	subscribe := func(ev events.EventType, handler func(events.Event)) {
		bus.Subscribe(ev, func(e events.Event) {
			if id, _ := e.Data["session_id"].(string); id == sess.ID {
				handler(e)
			}
		})
	}
	subscribe(events.AgentMessageDelta, func(e events.Event) {
		kind, _ := e.Data["kind"].(string)
		delta, _ := e.Data["delta"].(string)
		if kind == "thinking" && opts.quiet {
			return
		}
		fmt.Print(delta)
	})
	subscribe(events.AgentToolStart, func(e events.Event) {
		name, _ := e.Data["name"].(string)
		fmt.Fprintf(os.Stderr, "\n[tool] %s\n", name)
	})
	subscribe(events.AgentToolEnd, func(e events.Event) {
		if failed, _ := e.Data["failed"].(bool); failed {
			name, _ := e.Data["name"].(string)
			fmt.Fprintf(os.Stderr, "[tool] %s failed\n", name)
		}
	})
	subscribe(events.AgentPermission, func(e events.Event) {
		name, _ := e.Data["name"].(string)
		fmt.Fprintf(os.Stderr, "[permission] %s (%s)\n", name, map[bool]string{true: "approved", false: "denied"}[opts.approveAll])
		if opts.approveAll {
			_ = agentMgr.RespondApproval(ctx, sess.ID, true, true)
		}
	})

	go func() {
		defer close(done)
		defer fmt.Println()
		turnErr := make(chan error, 1)
		subscribe(events.AgentTurnEnd, func(e events.Event) {
			errMsg, _ := e.Data["error"].(string)
			turnErr <- nil
			if errMsg != "" {
				fmt.Fprintf(os.Stderr, "\n[turn error] %s\n", errMsg)
			}
		})
		if err := agentMgr.SendMessage(ctx, sess.ID, opts.prompt, nil); err != nil {
			fatal("send message: %v", err)
		}
		select {
		case <-turnErr:
		case <-ctx.Done():
			fmt.Fprintf(os.Stderr, "\n[timeout] %s\n", opts.timeout)
			agentMgr.StopTurn(sess.ID)
		case <-sigCh:
			fmt.Fprintln(os.Stderr, "\n[interrupt] stopping turn…")
			agentMgr.StopTurn(sess.ID)
		}
	}()
	return done
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "forge-agent: "+format+"\n", args...)
	os.Exit(1)
}
