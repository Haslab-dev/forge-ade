package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Haslab-dev/RakitKode/agentclirc"
	"github.com/hasdev/forge-ade/app/native/pkg/terminal"
	"github.com/hasdev/forge-ade/core/agentsession"
)

// AgentCLIDefinition describes an agent CLI known to ForgeADE.
type AgentCLIDefinition struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Executable  string   `json:"executable"`
	Args        []string `json:"args"`
	Description string   `json:"description"`
	Available   bool     `json:"available"`
}

// DefaultAgentCLIs registers the official fallback agent CLIs from ForgeADE.
var DefaultAgentCLIs = []AgentCLIDefinition{
	{ID: "antigravity", Name: "Antigravity CLI", Executable: "agy", Args: []string{}, Description: "Official Google Antigravity CLI agent"},
	{ID: "claude-code", Name: "Claude Code", Executable: "claude", Args: []string{}, Description: "Anthropic Claude Code terminal agent"},
	{ID: "codex", Name: "Codex", Executable: "codex", Args: []string{}, Description: "OpenAI Codex CLI coding assistant"},
	{ID: "opencode", Name: "OpenCode", Executable: "opencode", Args: []string{}, Description: "OpenCode terminal coding assistant"},
	{ID: "commandcode", Name: "Command Code", Executable: "cmd", Args: []string{}, Description: "Command Code terminal agent with taste learning"},
	{ID: "ohmypi", Name: "OhMyPi", Executable: "omp", Args: []string{}, Description: "OhMyPi terminal agent framework"},
	{ID: "pi", Name: "Pi", Executable: "pi", Args: []string{}, Description: "Minimalist autonomous terminal agent"},
	{ID: "gemini", Name: "Gemini CLI", Executable: "gemini", Args: []string{}, Description: "Google Gemini terminal AI assistant"},
}

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
		// Restored sessions have no live process: build a deferred terminal
		// state (Restart relaunches the CLI on demand). The on-disk output log
		// is PTY output — never feed it back as input, the shell would try to
		// execute the raw escape sequences.
		term := terminal.NewTerminalStateDeferred(r.WorkingDirectory)
		if r.Executable != "" {
			term.Command = []string{r.Executable}
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
		if mgr.ActiveID == "" && status == "running" {
			mgr.ActiveID = r.ID
		}
	}
}

// RefreshConfigs reads on-disk configuration of all known agent CLIs (claude, agy, gemini, pi, codex, etc.).
func (mgr *AgentCLIManager) RefreshConfigs() {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	// Same enrichment the webview applies: per-CLI path fixes (opencode,
	// pi, omp) and the commandcode discovery fallback.
	mgr.Configs = agentsession.EnrichAllAgentRuntimeConfigs(agentclirc.List())
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
		if _, err := lookPathEnriched(c.Executable); err == nil {
			c.Installed = true
			installed = append(installed, c)
		}
	}
	return installed
}

// GetAgentCLIs returns all known Agent CLI definitions with updated availability.
func (mgr *AgentCLIManager) GetAgentCLIs() []AgentCLIDefinition {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	var result []AgentCLIDefinition
	seen := make(map[string]bool)

	// Check standard defaults — enriched PATH so GUI launches detect CLIs.
	for _, def := range DefaultAgentCLIs {
		d := def
		if resolved, err := lookPathEnriched(d.Executable); err == nil {
			d.Available = true
			d.Executable = resolved
		}
		result = append(result, d)
		seen[d.ID] = true
	}

	// Add any runtime configs discovered by agentclirc
	for _, c := range mgr.Configs {
		if seen[c.ID] {
			continue
		}
		avail := c.Installed
		if !avail {
			if _, err := lookPathEnriched(c.Executable); err == nil {
				avail = true
			}
		}
		executable := c.Executable
		if resolved, err := lookPathEnriched(c.Executable); err == nil {
			executable = resolved
			avail = true
		}
		result = append(result, AgentCLIDefinition{
			ID:          c.ID,
			Name:        c.Name,
			Executable:  executable,
			Description: fmt.Sprintf("%s runtime CLI", c.Name),
			Available:   avail,
		})
		seen[c.ID] = true
	}

	return result
}

// CreateSessionWithTitle launches an agent CLI task session with a custom title.
func (mgr *AgentCLIManager) CreateSessionWithTitle(agentID, agentName, command string, args []string, cwd string, customTitle string) *AgentCLISession {
	sess := mgr.CreateSession(agentID, agentName, command, args, cwd)
	if sess != nil && strings.TrimSpace(customTitle) != "" {
		sess.Title = strings.TrimSpace(customTitle)
	}
	return sess
}

// CreateSession launches an interactive agent CLI task session. As forge-ade's
// backend does, the CLI executable itself becomes the PTY program — no wrapping
// shell, so its escape sequences never leak into a shell prompt.
func (mgr *AgentCLIManager) CreateSession(agentID, agentName, command string, args []string, cwd string) *AgentCLISession {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if cwd == "" {
		cwd = mgr.RootDir
	}

	sessionID := fmt.Sprintf("cli-%s-%d", agentID, time.Now().UnixNano()%100000)
	title := fmt.Sprintf("%s Task", agentName)

	term := terminal.NewTerminalStateDeferred(cwd)
	term.Command = append([]string{command}, args...)
	status := "running"
	if err := term.Restart(); err != nil {
		status = "error"
	}

	sess := &AgentCLISession{
		ID:        sessionID,
		AgentID:   agentID,
		AgentName: agentName,
		Title:     title,
		Command:   command,
		Args:      args,
		Cwd:       cwd,
		Status:    status,
		StartedAt: time.Now(),
		Terminal:  term,
	}

	mgr.Sessions[sessionID] = sess
	mgr.ActiveID = sessionID

	return sess
}

// ActiveSession returns the currently selected CLI session.
func (mgr *AgentCLIManager) ActiveSession() *AgentCLISession {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	return mgr.Sessions[mgr.ActiveID]
}

// StopSession stops a CLI session: Ctrl+C first (graceful for TUIs), then the
// process is killed if it survived the grace window.
func (mgr *AgentCLIManager) StopSession(sessionID string) {
	mgr.mu.RLock()
	sess := mgr.Sessions[sessionID]
	var term *terminal.TerminalState
	if sess != nil {
		term = sess.Terminal
	}
	mgr.mu.RUnlock()
	if term == nil {
		return
	}

	term.SendInput("\x03")
	go func() {
		time.Sleep(800 * time.Millisecond)
		if !term.IsExited() {
			term.Close()
		}
	}()

	mgr.mu.Lock()
	if sess, ok := mgr.Sessions[sessionID]; ok {
		sess.Status = "completed"
	}
	mgr.mu.Unlock()
}

// RestartSession relaunches the session's CLI within the same record, as
// forge-ade's Restart does — the executable becomes the new PTY program again.
func (mgr *AgentCLIManager) RestartSession(sessionID string) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	sess := mgr.Sessions[sessionID]
	if sess == nil || sess.Terminal == nil || strings.TrimSpace(sess.Command) == "" {
		return
	}
	sess.Terminal.Close()
	sess.Terminal.Command = append([]string{sess.Command}, sess.Args...)
	if err := sess.Terminal.Restart(); err != nil {
		sess.Status = "error"
		return
	}
	sess.Status = "running"
	sess.ExitCode = 0
	sess.StartedAt = time.Now()
}

// DeleteSession removes a CLI session and terminates its process.
func (mgr *AgentCLIManager) DeleteSession(sessionID string) {
	mgr.mu.Lock()
	sess := mgr.Sessions[sessionID]
	var term *terminal.TerminalState
	if sess != nil {
		term = sess.Terminal
		delete(mgr.Sessions, sessionID)
	}
	if mgr.ActiveID == sessionID {
		mgr.ActiveID = ""
		for id := range mgr.Sessions {
			mgr.ActiveID = id
			break
		}
	}
	mgr.mu.Unlock()

	if term != nil {
		term.Close()
	}
}

// SyncExitStatus flips a running session to completed once its PTY process
// has exited, so the UI reflects the real lifecycle.
func (mgr *AgentCLIManager) SyncExitStatus() {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	for _, sess := range mgr.Sessions {
		if sess.Status != "running" || sess.Terminal == nil {
			continue
		}
		if sess.Terminal.IsExited() {
			sess.Status = "completed"
			sess.ExitCode = sess.Terminal.ExitStatus()
		}
	}
}
