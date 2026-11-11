package ui

import (
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/Haslab-dev/RakitKode/agentclirc"
	"github.com/egoist/mygo/ui"
)

// AgentCLISession represents an interactive agent terminal task session.
type AgentCLISession struct {
	ID        string
	AgentID   string
	AgentName string
	Title     string
	Command   string
	Args      []string
	Cwd       string
	Status    string // "running", "completed", "error"
	ExitCode  int
	StartedAt time.Time
	Terminal  *TerminalState
}

// AgentCLIManager manages interactive Agent CLI sessions and on-disk CLI configuration discovery.
type AgentCLIManager struct {
	mu             sync.RWMutex
	Sessions       map[string]*AgentCLISession
	ActiveID       string
	Configs        []agentclirc.RuntimeConfig
	RootDir        string
	OnNeedRedraw   func()
}

// NewAgentCLIManager constructs the CLI manager and scans for installed agent CLI configs.
func NewAgentCLIManager(rootDir string, onRedraw func()) *AgentCLIManager {
	mgr := &AgentCLIManager{
		Sessions:     make(map[string]*AgentCLISession),
		RootDir:      rootDir,
		OnNeedRedraw: onRedraw,
	}

	mgr.RefreshConfigs()
	return mgr
}

// RefreshConfigs reads on-disk configuration of all known agent CLIs (claude, agy, gemini, pi, codex, etc.).
func (mgr *AgentCLIManager) RefreshConfigs() {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	mgr.Configs = agentclirc.List()
}

// InstalledConfigs returns the subset of agent CLIs that are currently installed on the host.
func (mgr *AgentCLIManager) InstalledConfigs() []agentclirc.RuntimeConfig {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	var installed []agentclirc.RuntimeConfig
	for _, c := range mgr.Configs {
		if c.Installed {
			installed = append(installed, c)
			continue
		}
		// Double-check PATH
		if _, err := exec.LookPath(c.Executable); err == nil {
			c.Installed = true
			installed = append(installed, c)
		}
	}
	return installed
}

// CreateSession launches an interactive agent CLI task session in the terminal.
func (mgr *AgentCLIManager) CreateSession(agentID, agentName, command string, args []string, cwd string) *AgentCLISession {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if cwd == "" {
		cwd = mgr.RootDir
	}

	sessionID := fmt.Sprintf("cli-%s-%d", agentID, time.Now().UnixNano()%100000)
	title := fmt.Sprintf("%s Task", agentName)

	term := NewTerminalState(cwd)

	sess := &AgentCLISession{
		ID:        sessionID,
		AgentID:   agentID,
		AgentName: agentName,
		Title:     title,
		Command:   command,
		Args:      args,
		Cwd:       cwd,
		Status:    "running",
		StartedAt: time.Now(),
		Terminal:  term,
	}

	mgr.Sessions[sessionID] = sess
	mgr.ActiveID = sessionID

	// Send initial launch command to embedded terminal
	go func() {
		time.Sleep(200 * time.Millisecond)
		cmdLine := command
		for _, a := range args {
			cmdLine += " " + a
		}
		term.SendInput(cmdLine + "\n")
	}()

	return sess
}

// ActiveSession returns the currently selected CLI session.
func (mgr *AgentCLIManager) ActiveSession() *AgentCLISession {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	return mgr.Sessions[mgr.ActiveID]
}

// StopSession sends interrupt signal to the active CLI session.
func (mgr *AgentCLIManager) StopSession(sessionID string) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if sess, ok := mgr.Sessions[sessionID]; ok {
		if sess.Terminal != nil {
			sess.Terminal.SendInput("\x03") // Ctrl+C
		}
		sess.Status = "completed"
	}
}

// RenderTerminalSessionSurface renders the interactive Agent CLI surface matching ForgeADE's TerminalSessionSurface.
func (mgr *AgentCLIManager) RenderTerminalSessionSurface(c *ui.Context, sessionID string) {
	t := c.Theme()

	mgr.mu.RLock()
	sess := mgr.Sessions[sessionID]
	mgr.mu.RUnlock()

	if sess == nil {
		ui.Column(c).Grow(1).Center().Children(func() {
			ui.Text(c, "No active CLI session").FontSize(16).Bold().TextColor(t.TextMuted)
		})
		return
	}

	ui.Column(c).Fill().Children(func() {
		// 1. Session Top Bar
		ui.Row(c).Gap(10).Padding(8, 14).AlignItems(ui.Center).Background(t.Surface).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Text(c, "🤖").FontSize(14)
			ui.Text(c, sess.AgentName).Bold().FontSize(13)
			ui.Text(c, sess.Title).FontSize(12).TextColor(t.TextMuted)

			ui.Box(c).Grow(1)

			// Status badge
			if sess.Status == "running" {
				ui.Text(c, "🟢 Running").FontSize(11).TextColor(t.Accent)
			} else {
				ui.Text(c, fmt.Sprintf("⚪ Stopped (code %d)", sess.ExitCode)).FontSize(11).TextColor(t.TextMuted)
			}

			// Elapsed time
			elapsed := time.Since(sess.StartedAt).Truncate(time.Second)
			ui.Text(c, fmt.Sprintf("⏱ %s", elapsed)).FontSize(11).TextColor(t.TextMuted)

			// Actions: Restart / Stop
			if sess.Status == "running" {
				if ui.Button(c, "⏹ Stop").FontSize(11).Clicked() {
					mgr.StopSession(sess.ID)
				}
			} else {
				if ui.PrimaryButton(c, "↻ Restart").FontSize(11).Clicked() {
					sess.Status = "running"
					sess.StartedAt = time.Now()
					if sess.Terminal != nil {
						sess.Terminal.SendInput(sess.Command + "\n")
					}
				}
			}
		})

		// 2. Main Terminal Content
		ui.Column(c).Grow(1).FillHeight().Children(func() {
			if sess.Terminal != nil {
				sess.Terminal.RenderTerminal(c)
			}
		})
	})
}
