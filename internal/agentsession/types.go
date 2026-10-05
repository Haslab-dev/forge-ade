// Package agentsession implements Terminal Session mode: ForgeADE launches
// external agent CLIs (Pi, OpenCode, Codex, Claude Code, …) in dedicated PTYs
// and lets the CLI own the entire agent interaction experience. ForgeADE only
// manages sessions — process/PTY lifecycle, terminal rendering, metadata, and
// history persistence. It never interprets what the agent is doing.
package agentsession

// Status is the lifecycle state of a terminal session's process.
type Status string

const (
	StatusStarting   Status = "starting"
	StatusRunning    Status = "running"
	StatusExited     Status = "exited"     // process ended on its own
	StatusFailed     Status = "failed"     // process could not be started
	StatusTerminated Status = "terminated" // killed by the user or app shutdown
)

// Session is one agent CLI process in its own PTY. Sessions are independent:
// each owns its process, PTY, terminal buffer, workspace, environment, and
// lifecycle. Sessions never share terminal state.
type Session struct {
	ID               string  `json:"id"`
	AgentID          string  `json:"agentId"`
	AgentName        string  `json:"agentName"`
	Title            string  `json:"title"`
	// TitleAuto is true when Title was derived automatically (workspace folder
	// name at creation, or the user's first terminal input) rather than set
	// explicitly — only then may later inputs rename the session.
	TitleAuto        bool    `json:"titleAuto"`
	WorkspacePath    string  `json:"workspacePath,omitempty"`
	WorkingDirectory string  `json:"workingDirectory"`
	Executable       string  `json:"executable"`
	Args             []string `json:"args,omitempty"`
	PID              int     `json:"pid,omitempty"`
	CreatedAt        int64   `json:"createdAt"`
	StartedAt        int64   `json:"startedAt,omitempty"`
	EndedAt          int64   `json:"endedAt,omitempty"`
	Status           Status  `json:"status"`
	ExitCode         *int    `json:"exitCode,omitempty"`
	Error            string  `json:"error,omitempty"`
	// ResumeHint holds resume args captured from the CLI's own output (e.g.
	// Antigravity prints "agy --conversation=<id>" on exit). When present it
	// wins over the config's ResumeArgs on restart, so each session continues
	// ITS conversation rather than whatever the CLI considers most recent.
	ResumeHint []string `json:"resumeHint,omitempty"`
}

// Running reports whether the session's process is alive (or starting).
func (s *Session) Running() bool {
	return s.Status == StatusStarting || s.Status == StatusRunning
}

// AgentCLIConfig is the user-configurable launch definition for one agent CLI.
// Executable/args/environment are configurable because users install CLIs via
// Homebrew, npm, Bun, standalone binaries, or local development builds.
type AgentCLIConfig struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Executable       string            `json:"executable"`
	Args             []string          `json:"args,omitempty"`
	// ResumeArgs are appended to Args when RESTARTING a session, so the CLI
	// continues its previous conversation instead of starting a fresh thread
	// (e.g. ["--continue"]). Empty list disables continuation; a config saved
	// without the field adopts the built-in default.
	ResumeArgs       []string          `json:"resumeArgs,omitempty"`
	WorkingDirectory string            `json:"workingDirectory,omitempty"`
	Environment      map[string]string `json:"environment,omitempty"`
	Enabled          bool              `json:"enabled"`
}

// TerminalSettings controls how session terminals are rendered.
type TerminalSettings struct {
	Shell       string `json:"shell"`       // login shell for plain shells; empty = auto-detect
	FontFamily  string `json:"fontFamily"`
	FontSize    int    `json:"fontSize"`
	CursorStyle string `json:"cursorStyle"` // block | bar | underline
	CursorBlink bool   `json:"cursorBlink"`
	Scrollback  int    `json:"scrollback"`
}

// AppSettings is the persisted application configuration. DefaultMode picks
// the experience the app opens with: "terminal" (Terminal Session, the
// default) or "agent-ui" (the dormant native Agent UI mode).
type AppSettings struct {
	DefaultMode string           `json:"defaultMode"`
	Terminal    TerminalSettings `json:"terminal"`
}

// DefaultTerminalSettings returns the built-in terminal rendering defaults.
// FontFamily mirrors the app's --font-mono token so terminal and UI mono text
// share one stack.
func DefaultTerminalSettings() TerminalSettings {
	return TerminalSettings{
		Shell:       "",
		FontFamily:  "'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, monospace",
		FontSize:    13,
		CursorStyle: "block",
		CursorBlink: true,
		Scrollback:  5000,
	}
}

// DefaultAppSettings returns the built-in application defaults: Terminal
// Session mode.
func DefaultAppSettings() AppSettings {
	return AppSettings{
		DefaultMode: "terminal",
		Terminal:    DefaultTerminalSettings(),
	}
}

// DefaultAgentCLIConfigs returns the built-in agent CLI definitions. All are
// enabled by default with their conventional executable names; users override
// any field in Settings → Agent CLIs. ResumeArgs continue the CLI's previous
// conversation on restart; a resume hint captured from the CLI's own output
// (see Session.ResumeHint) takes precedence.
func DefaultAgentCLIConfigs() []AgentCLIConfig {
	return []AgentCLIConfig{
		{ID: "pi", Name: "Pi", Executable: "pi", ResumeArgs: []string{"--continue"}, Enabled: true},
		{ID: "ohmypi", Name: "OhMyPi", Executable: "omp", ResumeArgs: []string{"--continue"}, Enabled: true},
		{ID: "opencode", Name: "OpenCode", Executable: "opencode", ResumeArgs: []string{"--continue"}, Enabled: true},
		{ID: "antigravity", Name: "Antigravity CLI", Executable: "agy", ResumeArgs: []string{"--continue"}, Enabled: true},
		{ID: "codex", Name: "Codex", Executable: "codex", ResumeArgs: []string{"resume", "--last"}, Enabled: true},
		{ID: "claude-code", Name: "Claude Code", Executable: "claude", ResumeArgs: []string{"--continue"}, Enabled: true},
	}
}

// renamedResumeDefaults tracks built-in ResumeArgs that changed in an update.
// A saved config still holding the OLD value is treated as un-edited and
// adopts the new default.
var renamedResumeDefaults = map[string]string{
	"ohmypi": "--resume", // omp resumes directly via --continue; bare --resume opens the interactive picker
}

// renamedDefaults tracks built-in executables that changed in an update. A
// saved config still holding the OLD default is treated as un-edited and
// adopts the new default instead of preserving the stale binary name.
var renamedDefaults = map[string]string{
	"antigravity": "antigravity", // "antigravity" → "agy"
}
