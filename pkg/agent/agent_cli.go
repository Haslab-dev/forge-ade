package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/Haslab-dev/RakitKode/agentclirc"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/terminal"
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
	Terminal  *terminal.TerminalState
}

// AgentCLIManager manages interactive Agent CLI sessions and on-disk CLI configuration discovery.
type AgentCLIManager struct {
	mu           sync.RWMutex
	Sessions     map[string]*AgentCLISession
	ActiveID     string
	Configs      []agentclirc.RuntimeConfig
	RootDir      string
	OnNeedRedraw func()
}

// NewAgentCLIManager constructs the CLI manager and scans for installed agent CLI configs.
func NewAgentCLIManager(rootDir string, onRedraw func()) *AgentCLIManager {
	mgr := &AgentCLIManager{
		Sessions:     make(map[string]*AgentCLISession),
		RootDir:      rootDir,
		OnNeedRedraw: onRedraw,
	}

	mgr.RefreshConfigs()
	mgr.LoadSessionsFromDisk()
	return mgr
}

// LoadSessionsFromDisk reads persistent terminal CLI sessions from ~/.forge-ade/agentsessions/index.json.
func (mgr *AgentCLIManager) LoadSessionsFromDisk() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	indexPath := filepath.Join(home, ".forge-ade", "agentsessions", "index.json")
	bytes, err := os.ReadFile(indexPath)
	if err != nil {
		return
	}

	var records []struct {
		ID               string `json:"id"`
		AgentID          string `json:"agentId"`
		AgentName        string `json:"agentName"`
		Title            string `json:"title"`
		Executable       string `json:"executable"`
		WorkingDirectory string `json:"workingDirectory"`
		Status           string `json:"status"`
		CreatedAt        int64  `json:"createdAt"`
		StartedAt        int64  `json:"startedAt"`
		EndedAt          int64  `json:"endedAt"`
	}
	if err := json.Unmarshal(bytes, &records); err != nil {
		return
	}

	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	for _, r := range records {
		started := time.UnixMilli(r.CreatedAt)
		if r.StartedAt > 0 {
			started = time.UnixMilli(r.StartedAt)
		}
		term := terminal.NewTerminalState(r.WorkingDirectory)
		logPath := filepath.Join(home, ".forge-ade", "agentsessions", "output", r.ID+".log")
		if logBytes, err := os.ReadFile(logPath); err == nil && len(logBytes) > 0 {
			term.SendInput(string(logBytes))
		}

		status := r.Status
		if status == "terminated" || status == "exited" {
			status = "completed"
		}

		mgr.Sessions[r.ID] = &AgentCLISession{
			ID:        r.ID,
			AgentID:   r.AgentID,
			AgentName: r.AgentName,
			Title:     r.Title,
			Command:   r.Executable,
			Cwd:       r.WorkingDirectory,
			Status:    status,
			StartedAt: started,
			Terminal:  term,
		}
		if mgr.ActiveID == "" {
			mgr.ActiveID = r.ID
		}
	}
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

	term := terminal.NewTerminalState(cwd)

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
