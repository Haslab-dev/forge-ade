package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Haslab-dev/RakitKode/search"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/board"
	"github.com/hasdev/forge-ade/app/native/pkg/database"
	"github.com/hasdev/forge-ade/app/native/pkg/editor"
	"github.com/hasdev/forge-ade/app/native/pkg/engine"
	"github.com/hasdev/forge-ade/app/native/pkg/git"
	"github.com/hasdev/forge-ade/app/native/pkg/palette"
	"github.com/hasdev/forge-ade/app/native/pkg/settings"
	"github.com/hasdev/forge-ade/app/native/pkg/terminal"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
	"github.com/hasdev/forge-ade/app/native/pkg/workspace"
	"github.com/hasdev/forge-ade/core/remotefs"
)

// Application surface modes
const (
	ModeAgent    = 0
	ModeEditor   = 1
	ModeSettings = 2
)

// Editor activity bar tabs
const (
	ActivityExplorer = 0
	ActivityGit      = 1
	ActivityDatabase = 2
	ActivitySearch   = 3
)

// Right drawer tab constants
const (
	RightTabChanges  = 0
	RightTabTerminal = 1
	RightTabChat     = 2
	RightTabTasks    = 3
	RightTabBoard    = 4
	RightTabGit      = 5
	RightTabDatabase = 6
	RightTabFiles    = 7
	RightTabSideFile = 8
)

// Layout metrics of ForgeADE's CSS
const (
	WorkspaceTopBarHeight = float32(52)
	RightDrawerDefault    = float32(420)
	LeftSidebarDefault    = float32(260)
	TerminalHeaderHeight  = float32(36)
	StatusBarHeight       = float32(24)
	ActivityBarWidth      = float32(48)
	FileTreeDefaultWidth  = float32(230)
	FileTreeRowHeight     = float32(22)
	EditorTabBarHeight    = float32(35)
)

// AppUI is the top-level native UI application state coordinating the modular domain engines.
type AppUI struct {
	Engine                *engine.Engine
	Window                *mygo.Window
	Workspace             *workspace.WorkspaceState
	Editor                *editor.EditorManager
	Terminal              *terminal.TerminalState
	Agent                 *agent.AgentState
	Git                   *git.GitManager
	Database              *database.DatabaseManager
	Palette               *palette.PaletteManager
	Settings              *settings.SettingsManager
	ThemeMgr              *theme.ThemeManager
	ACPMgr                *agent.ACPManager
	CLIMgr                *agent.AgentCLIManager
	Board                 *board.BoardManager
	Mode                  int      // ModeAgent or ModeEditor
	ActiveTaskKind        string   // "agent" or "cli"
	ShowLeftSidebar       bool     // Left sidebar visibility
	LeftSidebarView       string   // "tasks" (default) or "files" — forge-ade's leftSidebarView
	SideFileTabs          []string // files opened in the right sidebar from the explorer
	ActiveSideFile        string
	ShowBottomTerminal    bool    // Collapsible bottom terminal drawer
	BottomTerminalHeight  float32 // Resizable terminal height
	ShowRightDrawer       bool    // Collapsible right side pane
	RightDrawerWidth      float32 // Resizable right pane width
	LeftSidebarWidth      float32 // Resizable left sidebar width
	RightDrawerTab        int     // RightTabChanges, RightTabTerminal, RightTabChat, etc.
	EditorActivity        int     // ActivityExplorer, ActivityGit, ActivityDatabase
	SelectedFile          string
	FileSearchQuery       string
	FileSearchHits        []workspace.FileSearchHit
	FileSearchMode        string // "files" | "content" (ProjectSearchModal parity)
	FileSearchCase        bool
	FileSearchRegex       bool
	FileSearchWholeWord   bool
	FileSearchGlob        string
	FileSearchReplaceOpen bool
	FileSearchReplace     string
	FileSearchBusy        bool
	SearchStatusMsg       string
	ContentHits           []search.RankedResult
	ContentHitsAt         string
	SelectedDiffPath      string
	SideChatMessages      []*agent.ChatMessage
	SideChatInput         string
	SideChatModel         string
	SideChatMode          string // "full", "ask", "plan"
	SideChatStreaming     bool
	SideChatMd            markdown.Options

	// Composer autocomplete state (slash commands / @-mentions): visibility
	// derives from the composer text each frame; the dismissal latches hold
	// the text at Esc time so editing re-opens the menu.
	SlashSelected      int
	SlashDismissedAt   string
	MentionSelected    int
	MentionDismissedAt string

	// Structured agent questions (ask tool): modal flag + custom-answer
	// drafts keyed by questionDraftKey.
	ShowQuestionModal bool
	QuestionDrafts    map[string]*string

	// Git client PRs / Worktrees pane states (git_pr_worktrees.go).
	PRPane          *prPaneState
	WTPane          *worktreePaneState
	HunkRevertPaths map[string]bool

	// Inline AI edit bar (⌘I) + side-file preview modes.
	InlineAI             *editor.InlineAI
	InlineAICancel       chan struct{}
	SideFilePreviewImage string
	SideFilePreviewMd    string
	SideFileDiffMode     map[string]bool

	// Right-drawer browser viewer tab (native web view) + subagent pane.
	BrowserWebView    *mygo.WebView
	BrowserURLInput   string
	BrowserURLCurrent string
	BrowserZoom       float64

	// Thread find bar (⌘F) state — ThreadFindBar parity.
	ShowThreadFind    bool
	ThreadFindQuery   string
	ThreadFindQueryAt string
	ThreadFindMatches []int
	ThreadFindIdx     int

	ShowNewFileModal        bool
	NewFileName             string
	ShowNewFolderModal      bool
	NewFolderName           string
	ShowNewTaskModal        bool
	NewTaskTitle            string
	NewTaskWorkspacePath    string // preselected folder for group-level "+ New task"
	NewTaskAgentType        string // "acp" or "cli"
	NewTaskSearchQuery      string
	NewTaskFilterTab        string // "all", "acp", "cli"
	NewTaskSelectedKind     string // "acp", "cli"
	NewTaskSelectedAgentID  string
	NewTaskSelectedName     string
	NewTaskSelectedExec     string
	NewTaskSelectedArgs     []string
	NewTaskUninstalledOpen  bool
	NewTaskStep             string // "agent" or "workspace"
	SelectedAgentID         string
	SelectedAgentModel      string
	ShowTaskMenuModal       bool
	ShowRenameModal         bool
	RenameSessionTitle      string
	ShowRenameFileModal     bool
	RenameFileOldPath       string
	RenameFilePath          string
	ShowBranchModal         bool
	BranchSearchQuery       string
	ShowModelPalette        bool
	ModelSearchQuery        string
	SelectedHarnessID       string
	SelectedProviderTab     string
	CollapsedProjectGroups  map[string]bool
	ExpandedProjectGroups   map[string]bool
	ExpandedFileTreeFolders map[string]bool
	ProjectFilter           string   // "all" or a project name (sidebar "All projects" menu)
	TaskSortBy              string   // "updated" (default) or "created" (sidebar sort menu)
	RecentWorkspaces        []string // most-recent-first history of opened folders
	ShowSSHModal            bool
	ShowSSHQuickMenu        bool
	SSHLabel                string
	SSHHost                 string
	SSHPort                 int
	SSHPortText             string
	SSHUser                 string
	SSHAuthType             string // "key_file", "password", "agent"
	SSHKeyPath              string
	SSHUploadStatus         string
	SSHKeyPassphrase        string
	SSHPassword             string
	SSHRemotePath           string
	SSHIsConnecting         bool
	SSHErrorMsg             string
	SSHSavedConfigs         []remotefs.SSHConfig
	SSHSelectedConfigID     string
	SSHIsChangingDir        bool
	SSHNewDirPath           string
	ShowUsageModal          bool
	QuotaData               map[string]interface{}
	QuotaFetching           bool
	ShowRulesModal          bool
	SelectedRulePath        string
	RuleEditorContent       string
	BoardSearchQuery        string
	BoardViewMode           string // "" = board, "list"
	ShowBoardTaskModal      bool
	BoardTaskStatus         string
	BoardTaskTitle          string
	AgentViewMode           string // AgentViewChat / AgentViewBoard / AgentViewGit
	GitRemoteURL            string
	ShowNewBoardTaskModal   bool
	NewBoardTaskTitle       string
	NewBoardTaskDesc        string
	NewBoardTaskPriority    string
	NewBoardTaskStatus      string
}

// NewAppUI constructs the complete AppUI state tree.
func NewAppUI(engine *engine.Engine, rootDir string) *AppUI {
	// No workspace (empty root dir): open in editor mode with the welcome
	// state; the terminal defaults to the home directory.
	startMode := ModeAgent
	terminalDir := rootDir
	if rootDir == "" {
		startMode = ModeEditor
		home, _ := os.UserHomeDir()
		terminalDir = home
	}

	appUI := &AppUI{
		Engine:                  engine,
		Workspace:               workspace.NewWorkspaceState(rootDir),
		Editor:                  editor.NewEditorManager(),
		Terminal:                terminal.NewTerminalState(terminalDir),
		ThemeMgr:                theme.NewThemeManager(),
		Mode:                    startMode,
		ActiveTaskKind:          "agent",
		RecentWorkspaces:        loadRecentWorkspaces(),
		ShowLeftSidebar:         true,
		LeftSidebarWidth:        LeftSidebarDefault,
		RightDrawerWidth:        RightDrawerDefault,
		BottomTerminalHeight:    240,
		ShowBottomTerminal:      false,
		ShowRightDrawer:         false,
		RightDrawerTab:          RightTabChanges,
		EditorActivity:          ActivityExplorer,
		NewTaskAgentType:        "acp",
		NewTaskFilterTab:        "all",
		NewTaskStep:             "agent",
		SelectedAgentID:         "agent-agy",
		SelectedAgentModel:      "gemini-3.8-flash-high",
		SelectedHarnessID:       "agent-agy",
		SelectedProviderTab:     "all",
		CollapsedProjectGroups:  make(map[string]bool),
		ExpandedProjectGroups:   make(map[string]bool),
		ProjectFilter:           "all",
		TaskSortBy:              "updated",
		ExpandedFileTreeFolders: make(map[string]bool),
		SSHPort:                 22,
		SSHPortText:             "22",
		SSHAuthType:             "key_file",
		SSHKeyPath:              "~/.ssh/id_ed25519",
		SSHRemotePath:           "/var/www",
		SSHSavedConfigs:         make([]remotefs.SSHConfig, 0),
		SideChatModel:           "Claude 3.7 Sonnet",
		SideChatMode:            "full",
		SideChatMessages:        make([]*agent.ChatMessage, 0),
		QuestionDrafts:          make(map[string]*string),
		InlineAI:                editor.NewInlineAI(),
		SideFileDiffMode:        make(map[string]bool),
	}

	appUI.Agent = agent.NewAgentState(engine, rootDir, func() {
		if appUI.Window != nil {
			appUI.Window.Invalidate()
		}
	})
	appUI.Agent.Theme = appUI.ThemeMgr

	appUI.ACPMgr = agent.NewACPManager(engine, func() {
		if appUI.Window != nil {
			appUI.Window.Invalidate()
		}
	})
	appUI.Agent.ACPMgr = appUI.ACPMgr

	// Bus events carry the RakitKode manager UUID for internal sessions; map
	// them to the native session IDs the UI state is keyed on.
	appUI.ACPMgr.OnTokenChunk = func(sessionID, text string) {
		appUI.Agent.HandleTokenChunk(appUI.Agent.MapInternalSession(sessionID), text)
	}
	appUI.ACPMgr.OnThoughtChunk = func(sessionID, text string) {
		appUI.Agent.HandleThoughtChunk(appUI.Agent.MapInternalSession(sessionID), text)
	}
	appUI.ACPMgr.OnToolEvent = func(ev agent.ToolEvent) {
		ev.SessionID = appUI.Agent.MapInternalSession(ev.SessionID)
		appUI.Agent.HandleToolEvent(ev)
	}
	appUI.ACPMgr.OnTurnStart = func(sessionID, prompt string) {
		appUI.Agent.HandleTurnStart(appUI.Agent.MapInternalSession(sessionID), prompt)
	}
	appUI.ACPMgr.OnAsk = func(req *agent.PermissionRequest) {
		req.SessionID = appUI.Agent.MapInternalSession(req.SessionID)
		appUI.Agent.HandleAsk(req)
	}
	appUI.ACPMgr.OnQuestions = func(sessionID string, questions []agent.AgentQuestion) {
		appUI.Agent.HandleQuestions(appUI.Agent.MapInternalSession(sessionID), questions)
	}
	appUI.ACPMgr.OnUsage = func(sessionID string, usage agent.UsageInfo) {
		appUI.Agent.HandleUsage(appUI.Agent.MapInternalSession(sessionID), usage)
	}
	appUI.ACPMgr.OnCommands = func(sessionID string, commands []string) {
		appUI.Agent.HandleCommands(appUI.Agent.MapInternalSession(sessionID), commands)
	}

	appUI.ACPMgr.OnTurnDone = func(sessionID string, info agent.TurnEndInfo) {
		// Finalizes the message, maps the status, records turn changes, and
		// returns the next queued steer prompt to run.
		nativeID := appUI.Agent.MapInternalSession(sessionID)
		if next := appUI.Agent.HandleTurnDone(nativeID, info); next != "" {
			go appUI.Agent.TriggerTurn(next)
		}
	}

	// Live elapsed indicators ("Working for 12s", spinners): repaint once a
	// second while a turn is streaming.
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if appUI.Agent.IsStreaming && appUI.Window != nil {
				appUI.Window.Invalidate()
			}
		}
	}()

	appUI.CLIMgr = agent.NewAgentCLIManager(rootDir, func() {
		if appUI.Window != nil {
			appUI.Window.Invalidate()
		}
	})

	appUI.Editor.Theme = appUI.ThemeMgr

	appUI.Settings = settings.NewSettingsManager(engine, func(provider, model string) {
		if appUI.Settings.APIKey != "" {
			appUI.Agent.ActiveModel = model
		}
	})
	if appUI.Settings.APIKey != "" && appUI.Settings.Model != "" {
		appUI.Agent.ActiveModel = appUI.Settings.Model
	}
	appUI.Settings.ThemeMgr = appUI.ThemeMgr

	appUI.Database = database.NewDatabaseManager(appUI.Workspace)
	appUI.Database.OnNeedRedraw = func() {
		if appUI.Window != nil {
			appUI.Window.Invalidate()
		}
	}

	appUI.Git = git.NewGitManager(engine, rootDir, func(path string, showDiff bool) {
		fullPath := appUI.Workspace.ResolveFullPath(path)
		_, _ = appUI.Editor.OpenFile(fullPath, path)
		appUI.SelectedFile = path
		appUI.Editor.IsDiffView = showDiff
		appUI.Mode = ModeEditor
	})
	appUI.Git.OnNeedRedraw = func() {
		if appUI.Window != nil {
			appUI.Window.Invalidate()
		}
	}

	if engine != nil {
		engine.StartWatcher(rootDir, func() {
			appUI.Workspace.Refresh()
			if appUI.Git != nil {
				appUI.Git.Refresh()
			}
			if appUI.Window != nil {
				appUI.Window.Invalidate()
			}
		})
	}

	appUI.Palette = palette.NewPaletteManager(appUI.Workspace, func(path string) {
		fullPath := appUI.Workspace.ResolveFullPath(path)
		_, _ = appUI.Editor.OpenFile(fullPath, path)
		appUI.SelectedFile = path
		appUI.Mode = ModeEditor
	})
	appUI.Palette.SetCommands(appUI.paletteCommands())
	// Workspace symbol backend for the "@" go-to-symbol palette mode.
	appUI.Palette.SetSymbolSearch(func(query string, limit int) ([]palette.SymbolHit, error) {
		if engine.SearchMgr == nil {
			return nil, fmt.Errorf("search manager unavailable")
		}
		engine.SearchMgr.SetDirectories([]string{appUI.Workspace.RootDir})
		hits, err := engine.SearchMgr.SearchSymbols(query, limit)
		if err != nil {
			return nil, err
		}
		out := make([]palette.SymbolHit, 0, len(hits))
		for _, h := range hits {
			out = append(out, palette.SymbolHit{Path: h.Path, Line: h.Line, Content: h.Content})
		}
		return out, nil
	})

	if len(appUI.Workspace.Files) > 0 {
		first := appUI.Workspace.Files[0]
		appUI.SelectedFile = first.RelativePath
		_, _ = appUI.Editor.OpenFile(first.FullPath, first.RelativePath)
	}

	if appUI.Git != nil && appUI.Git.Branch != "" {
		appUI.Workspace.GitBranch = appUI.Git.Branch
		appUI.Workspace.GitDirty = len(appUI.Git.Staged)+len(appUI.Git.Unstaged)+len(appUI.Git.Untracked) > 0
	}

	if appUI.Workspace != nil && engine != nil {
		appUI.Workspace.RemoteFS = engine.RemoteFS
	}
	if appUI.Editor != nil && engine != nil {
		appUI.Editor.RemoteFS = engine.RemoteFS
	}
	appUI.LoadSavedSSHConfigs()

	return appUI
}

func (a *AppUI) paletteCommands() []palette.PaletteCommand {
	return []palette.PaletteCommand{
		{ID: "new-task", Label: "New Task", Detail: "Start a new agent task (open New Agent UI)", Shortcut: "⌘N", Run: func() {
			a.NewAgentTask("")
		}},
		{ID: "new-task-agent", Label: "New Task with Agent / CLI...", Detail: "Select ACP Agent or CLI harness", Shortcut: "⇧⌘N", Run: func() {
			a.OpenNewTaskModal()
		}},
		{ID: "select-model", Label: "Select Model / Harness", Detail: "Choose AI model or ACP harness", Shortcut: "⌘M", Run: func() {
			a.ShowModelPalette = true
			a.ModelSearchQuery = ""
		}},
		{ID: "agent-mode", Label: "Switch to Agent Mode", Shortcut: "⌘1", Run: func() { a.Mode = ModeAgent }},
		{ID: "editor-mode", Label: "Switch to Editor Mode", Shortcut: "⌘2", Run: func() { a.Mode = ModeEditor }},
		{ID: "toggle-terminal", Label: "Toggle Terminal", Detail: "Bottom terminal drawer", Shortcut: "⌘`", Run: func() {
			a.ShowBottomTerminal = !a.ShowBottomTerminal
		}},
		{ID: "toggle-sidebar", Label: "Toggle Sidebar", Shortcut: "⌘B", Run: func() { a.ShowLeftSidebar = !a.ShowLeftSidebar }},
		{ID: "toggle-drawer", Label: "Toggle Right Drawer", Shortcut: "⌘J", Run: func() { a.ShowRightDrawer = !a.ShowRightDrawer }},
		{ID: "save", Label: "Save File", Shortcut: "⌘S", Run: func() { _ = a.Editor.SaveActiveFile() }},
		{ID: "settings", Label: "Open Settings", Shortcut: "⌘,", Run: func() { a.Settings.Toggle() }},
		{ID: "appearance", Label: "Toggle Appearance", Detail: "Dark / light", Shortcut: "⌘/", Run: a.toggleAppearance},
		{ID: "theme", Label: "Pick Theme Palette", Run: func() { a.ThemeMgr.Toggle() }},
		{ID: "open-folder", Label: "Open Workspace Folder", Shortcut: "⌘O", Run: a.openWorkspaceFolder},
		{ID: "board", Label: "Show Workflow Board", Run: func() {
			a.ShowRightDrawer = true
			a.RightDrawerTab = RightTabBoard
			a.RefreshBoard()
		}},
		{ID: "new-board-task", Label: "Board: Create New Task", Detail: "Add task to workflow kanban board", Run: func() {
			a.NewBoardTaskTitle = ""
			a.NewBoardTaskDesc = ""
			a.NewBoardTaskPriority = "medium"
			a.NewBoardTaskStatus = "todo"
			a.ShowNewBoardTaskModal = true
		}},
		{ID: "usage-quota", Label: "Agent: Usage & Quota Limits", Detail: "Inspect context window tokens, weekly & 5-hour quotas", Run: func() {
			a.openUsageModal()
		}},
		{ID: "unified-rules", Label: "Rules: Unified Project Rules Manager", Detail: "Manage and sync AGENTS.md, .cursorrules, CLAUDE.md", Run: func() {
			a.OpenRulesModal()
		}},
		{ID: "changes", Label: "Show Git Changes", Run: func() {
			a.ShowRightDrawer = true
			a.RightDrawerTab = RightTabChanges
		}},
		{ID: "refresh-files", Label: "Refresh Workspace Files", Run: func() {
			a.Workspace.Refresh()
			if a.Git != nil {
				go a.Git.Refresh()
			}
		}},
		{ID: "ssh-connect", Label: "SSH: Connect to Remote Host...", Detail: "Open SSH & SFTP connection manager", Shortcut: "⇧⌘R", Run: func() {
			a.OpenSSHModal()
		}},
		{ID: "ssh-quick", Label: "SSH: Remote Actions Menu", Detail: "Quick menu for active SSH connection", Run: func() {
			if strings.HasPrefix(a.Workspace.RootDir, "ssh://") {
				a.ShowSSHQuickMenu = true
			} else {
				a.OpenSSHModal()
			}
		}},
		{ID: "ssh-disconnect", Label: "SSH: Disconnect from Remote Host", Detail: "Close remote workspace and return to local files", Run: func() {
			a.DisconnectSSH()
		}},
	}
}

// openUsageModal shows the usage modal and refreshes quota data
// asynchronously — never on the render path, where a network call would
// freeze the frame.
func (a *AppUI) openUsageModal() {
	a.ShowUsageModal = true
	a.refreshQuotaAsync()
}

// refreshQuotaAsync fetches quota/usage data off the render path.
func (a *AppUI) refreshQuotaAsync() {
	if a.QuotaFetching || a.Engine == nil {
		return
	}
	a.QuotaFetching = true
	go func() {
		defer func() {
			a.QuotaFetching = false
			if a.Window != nil {
				a.Window.Invalidate()
			}
		}()
		data := a.Engine.GetAgentQuota("agent-agy")
		if data != nil {
			a.QuotaData = data
		}
	}()
}

// loadGitRemoteURL reads the origin remote URL for the Git Client badge.
func (a *AppUI) loadGitRemoteURL() {
	if a.Engine == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "-C", a.Workspace.RootDir, "config", "--get", "remote.origin.url").Output()
	if err == nil {
		a.GitRemoteURL = strings.TrimSpace(string(out))
	}
}

// RefreshBoard reloads the workflow board from .forge/board_tasks.json.
func (a *AppUI) RefreshBoard() {
	if a.Board == nil {
		a.Board = board.NewBoardManager(a.Workspace.RootDir)
	} else {
		a.Board.RootDir = a.Workspace.RootDir
	}
	a.Board.Refresh()
}

// SetWindow attaches the native window reference for redraw invalidations.
func (a *AppUI) SetWindow(w *mygo.Window) {
	a.Window = w

	// Files dropped anywhere the composer does not take them attach to the
	// draft, as forge-ade accepts drops across the window.
	if w != nil {
		w.OnFileDrop(func(e *mygo.FileDropEvent) {
			if len(e.Paths) == 0 {
				return
			}
			a.Agent.AttachFiles(e.Paths)
			if a.Window != nil {
				a.Window.Invalidate()
			}
		})
	}
}

// View builds the entire native GPU frame matching ForgeADE layout.
func (a *AppUI) View(c *ui.Context) {
	if a.ThemeMgr != nil {
		c.SetTheme(a.ThemeMgr.ActiveTheme())
	}

	// Global Keyboard Shortcuts
	if c.Shortcut(ui.Cmd, ui.KeyS) {
		if err := a.Editor.SaveActiveFile(); err == nil {
			doc := a.Editor.ActiveDoc()
			name := "File"
			if doc != nil {
				name = doc.RelativePath
			}
			c.AddToast(ui.Toast{
				Title:       "Saved",
				Description: fmt.Sprintf("%s saved to disk", name),
				Type:        "success",
			})
		}
	}
	if c.Shortcut(ui.Cmd, ui.KeyK) {
		a.Palette.Toggle()
	}
	if c.Shortcut(ui.Cmd, ui.KeyP) {
		a.Palette.Toggle()
	}
	cmdOrCtrl := func(key ui.Key) bool {
		return c.Shortcut(ui.Cmd, key) || c.Shortcut(ui.Ctrl, key)
	}
	cmdOrCtrlShift := func(key ui.Key) bool {
		return c.Shortcut(ui.Cmd|ui.Shift, key) || c.Shortcut(ui.Ctrl|ui.Shift, key)
	}

	if cmdOrCtrlShift(ui.KeyP) {
		a.Palette.OpenCommands()
	}
	if cmdOrCtrlShift(ui.KeyF) {
		// Search in files, as forge-ade's Search activity (Zed style).
		a.Mode = ModeEditor
		a.AgentViewMode = AgentViewChat
		a.ShowLeftSidebar = true
		a.EditorActivity = ActivitySearch
	}
	if cmdOrCtrl(ui.KeyY) && a.Agent != nil && a.Agent.ActiveSession() != nil {
		// Approve the pending permission (⌘Y, AgentApprovalBar parity).
		if as := a.Agent.ActiveSession(); as != nil && len(as.Messages) > 0 {
			if last := as.Messages[len(as.Messages)-1]; last.PendingApproval != nil && !last.PendingApproval.Resolved {
				a.respondApproval(last.PendingApproval.SessionID, last.PendingApproval.ToolCallID, true, false)
			}
		}
	}
	if cmdOrCtrl(ui.KeyI) && a.Mode == ModeEditor {
		// Edit with AI over the selection (CodeEditorPane's Mod-i).
		a.openInlineAI()
	}
	if c.Shortcut(ui.Shift|ui.Alt, ui.KeyF) && a.Mode == ModeEditor {
		// Format Document (Shift-Alt-F).
		a.formatActiveDocument(c)
	}
	if cmdOrCtrl(ui.KeyF) && a.Mode == ModeAgent && a.Agent != nil && a.Agent.ActiveSession() != nil {
		// Find-in-thread, as forge-ade's ⌘F ThreadFindBar in the chat surface.
		a.ShowThreadFind = !a.ShowThreadFind
		if !a.ShowThreadFind {
			a.ThreadFindQuery = ""
			a.ThreadFindMatches = nil
		}
	}
	if cmdOrCtrlShift(ui.KeyE) {
		a.Mode = ModeEditor
		a.ShowLeftSidebar = true
		a.EditorActivity = ActivityExplorer
	}
	if cmdOrCtrlShift(ui.KeyO) {
		// Go to symbol (⌘⇧O) — the "@" palette mode.
		a.Mode = ModeEditor
		a.Palette.OpenSymbols()
		a.Palette.Query = "@"
	}
	if cmdOrCtrlShift(ui.KeyG) {
		a.Mode = ModeEditor
		a.ShowLeftSidebar = true
		a.EditorActivity = ActivityGit
	}
	if cmdOrCtrlShift(ui.KeyJ) {
		a.ShowRightDrawer = !a.ShowRightDrawer
	}
	if cmdOrCtrl(ui.KeyComma) {
		a.Mode = ModeSettings
		if a.Settings != nil {
			a.Settings.IsOpen = true
		}
	}
	if cmdOrCtrl(ui.KeyB) {
		a.ShowLeftSidebar = !a.ShowLeftSidebar
	}
	if cmdOrCtrl(ui.KeyN) {
		// New task opens the ACP/CLI agent picker, as the user expects.
		a.OpenNewTaskModal()
	}
	if cmdOrCtrlShift(ui.KeyN) {
		a.OpenNewTaskModal()
	}
	if cmdOrCtrl(ui.KeyM) {
		a.ShowModelPalette = !a.ShowModelPalette
		a.ModelSearchQuery = ""
	}
	if cmdOrCtrl(ui.KeyO) {
		a.openWorkspaceFolder()
	}
	if cmdOrCtrl(ui.Key1) {
		a.Mode = ModeAgent
	}
	if cmdOrCtrl(ui.Key2) {
		a.Mode = ModeEditor
	}
	if cmdOrCtrl(ui.KeyJ) {
		a.ShowRightDrawer = !a.ShowRightDrawer
	}
	if cmdOrCtrl(ui.KeySlash) {
		a.toggleAppearance()
	}
	if cmdOrCtrl(ui.KeyBackquote) {
		a.ShowBottomTerminal = !a.ShowBottomTerminal
	}
	if c.Shortcut(ui.Ctrl, ui.KeyTab) {
		a.Editor.CycleTab(1)
	}
	if c.Shortcut(ui.Ctrl|ui.Shift, ui.KeyTab) {
		a.Editor.CycleTab(-1)
	}
	if cmdOrCtrl(ui.KeyW) {
		if a.Mode == ModeEditor && a.Editor.ActivePath != "" {
			a.Editor.CloseTab(a.Editor.ActivePath)
		}
	}

	// Root layout
	if a.Mode == ModeAgent && a.ShowLeftSidebar {
		ui.Split(c, &a.LeftSidebarWidth, func() {
			a.renderAgentSidebar(c)
		}, func() {
			a.renderMainColumn(c)
		}).SplitReset(LeftSidebarDefault).Fill()
	} else {
		a.renderMainColumn(c)
	}

	// Overlays & Modals
	a.renderModals(c)
}

func (a *AppUI) toggleAppearance() {
	if a.ThemeMgr.Appearance == "dark" {
		a.ThemeMgr.Appearance = "light"
	} else {
		a.ThemeMgr.Appearance = "dark"
	}
}

// NewAgentTask switches to Agent Mode and opens the New Agent UI ("What should we do in <project>?")
// in the specified workspace (or active workspace if workspacePath is empty).
func (a *AppUI) NewAgentTask(workspacePath string) {
	a.Mode = ModeAgent
	a.ActiveTaskKind = "agent"
	a.ShowNewTaskModal = false

	if workspacePath != "" && workspacePath != a.Workspace.RootDir {
		a.switchWorkspace(workspacePath)
	}

	if a.Agent != nil {
		a.Agent.ClearActiveSession()
	}

	if a.Window != nil {
		a.Window.Invalidate()
	}
}

func (a *AppUI) OpenNewTaskModal() {
	a.ShowNewTaskModal = true
	a.NewTaskTitle = ""
	a.NewTaskSearchQuery = ""
	a.NewTaskFilterTab = "all"
	a.NewTaskStep = "agent"
	a.NewTaskSelectedKind = ""
	a.NewTaskSelectedAgentID = ""
	a.NewTaskSelectedName = ""
	a.NewTaskSelectedExec = ""
	a.NewTaskSelectedArgs = nil
	a.NewTaskUninstalledOpen = false
	if a.ACPMgr != nil {
		a.ACPMgr.RefreshAvailability()
	}
	if a.CLIMgr != nil {
		a.CLIMgr.RefreshConfigs()
	}
}

// OpenNewTaskModalFor opens the ACP/CLI agent picker pre-bound to a project
// folder, as forge-ade's group-level "+ New task" does.
func (a *AppUI) OpenNewTaskModalFor(folderPath string) {
	a.OpenNewTaskModal()
	a.NewTaskWorkspacePath = folderPath
}

// ── Recent workspaces ────────────────────────────────────────────────────
// Persisted most-recent-first list of opened folders, shown on the welcome
// pane and in the workspace switcher menus.

const recentWorkspacesCap = 12

func recentWorkspacesPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".forge-ade", "recent-workspaces.json")
}

// loadRecentWorkspaces reads the persisted opened-folder history.
func loadRecentWorkspaces() []string {
	path := recentWorkspacesPath()
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var list []string
	if err := json.Unmarshal(data, &list); err != nil {
		return nil
	}
	return list
}

func saveRecentWorkspaces(list []string) {
	path := recentWorkspacesPath()
	if path == "" {
		return
	}
	if data, err := json.MarshalIndent(list, "", "  "); err == nil {
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = os.WriteFile(path, data, 0o644)
	}
}

// recordRecentWorkspace moves dir to the front of the history (deduped) and
// persists it. Empty paths (General) are not recorded.
func (a *AppUI) recordRecentWorkspace(dir string) {
	if dir == "" || dir == "/" || strings.HasPrefix(dir, "ssh://") {
		return
	}
	rest := make([]string, 0, len(a.RecentWorkspaces))
	for _, p := range a.RecentWorkspaces {
		if p != dir {
			rest = append(rest, p)
		}
	}
	a.RecentWorkspaces = append([]string{dir}, rest...)
	if len(a.RecentWorkspaces) > recentWorkspacesCap {
		a.RecentWorkspaces = a.RecentWorkspaces[:recentWorkspacesCap]
	}
	saveRecentWorkspaces(a.RecentWorkspaces)
}

// openSideFile opens a workspace file in the right sidebar's file tabs, as
// forge-ade's explorer "Open in Side Preview" does.
func (a *AppUI) openSideFile(relPath string) {
	full := a.Workspace.ResolveFullPath(relPath)
	if doc := a.Editor.GetDoc(full); doc == nil {
		_, _ = a.Editor.OpenFile(full, relPath)
	}
	for i, p := range a.SideFileTabs {
		if p == relPath {
			a.SideFileTabs = append(a.SideFileTabs[:i], a.SideFileTabs[i+1:]...)
			break
		}
	}
	a.SideFileTabs = append([]string{relPath}, a.SideFileTabs...)
	a.ActiveSideFile = relPath
	a.ShowRightDrawer = true
	a.RightDrawerTab = RightTabSideFile
	a.SelectedFile = relPath
}

// closeSideFile removes a file tab from the right sidebar.
func (a *AppUI) closeSideFile(relPath string) {
	for i, p := range a.SideFileTabs {
		if p == relPath {
			a.SideFileTabs = append(a.SideFileTabs[:i], a.SideFileTabs[i+1:]...)
			break
		}
	}
	if a.ActiveSideFile == relPath {
		if len(a.SideFileTabs) > 0 {
			a.ActiveSideFile = a.SideFileTabs[0]
		} else {
			a.ActiveSideFile = ""
			a.RightDrawerTab = RightTabChanges
		}
	}
}

// deleteWorkspaceGroup removes every task session bound to a project folder
// and releases the workspace (back to General) when it is the active one —
// forge-ade's "Delete workspace" group action. Files on disk are untouched.
func (a *AppUI) deleteWorkspaceGroup(c *ui.Context, groupName, folderPath string) {
	if groupName == "General" || folderPath == "" {
		return
	}
	if a.Agent != nil {
		for _, sess := range a.Agent.FilteredSessions() {
			if resolveProjectName(sess.WorkspacePath) == groupName {
				a.Agent.DeleteSession(sess.ID)
			}
		}
	}
	if a.CLIMgr != nil {
		for id, cs := range a.CLIMgr.Sessions {
			if resolveProjectName(cs.Cwd) == groupName {
				a.CLIMgr.DeleteSession(id)
			}
		}
	}
	if a.Workspace != nil && a.Workspace.RootDir == folderPath {
		a.switchWorkspace("")
	}
	c.AddToast(ui.Toast{Title: "Workspace Removed", Description: groupName, Type: "info"})
}

func (a *AppUI) openWorkspaceFolder() {
	if a.Window == nil {
		return
	}
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:    a.Window,
			Title:     "Open Workspace Folder",
			Directory: true,
		})
		if err != nil || len(paths) == 0 || paths[0] == "" {
			return
		}
		a.switchWorkspace(paths[0])
	}()
}

func (a *AppUI) switchWorkspace(dir string) {
	a.Workspace = workspace.NewWorkspaceState(dir)
	a.recordRecentWorkspace(dir)
	if a.Engine != nil {
		a.Workspace.RemoteFS = a.Engine.RemoteFS
		a.Workspace.Refresh()
	}
	if a.Editor != nil && a.Engine != nil {
		a.Editor.RemoteFS = a.Engine.RemoteFS
	}
	a.Agent.SetWorkspacePath(dir)
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = home
	}
	a.Terminal.Cwd = dir
	a.Editor.CloseAll()
	a.SelectedFile = ""
	a.SelectedDiffPath = ""

	if strings.HasPrefix(dir, "ssh://") {
		// Remote SSH workspace
		if a.Database != nil {
			a.Database.Workspace = a.Workspace
		}
		if len(a.Workspace.Files) > 0 {
			first := a.Workspace.Files[0]
			a.SelectedFile = first.RelativePath
			_, _ = a.Editor.OpenFile(first.FullPath, first.RelativePath)
		}
		if a.Window != nil {
			a.Window.Invalidate()
		}
		return
	}

	if a.Database != nil {
		a.Database.Workspace = a.Workspace
		go a.Database.ScanDatabases()
	}
	if a.Git != nil {
		a.Git = git.NewGitManager(a.Engine, dir, a.Git.OnFileSelect)
		a.Git.OnNeedRedraw = func() {
			if a.Window != nil {
				a.Window.Invalidate()
			}
		}
		go a.Git.Refresh()
	}
	if a.Engine != nil {
		a.Engine.StartWatcher(dir, func() {
			a.Workspace.Refresh()
			if a.Git != nil {
				a.Git.Refresh()
			}
			if a.Window != nil {
				a.Window.Invalidate()
			}
		})
	}
	if len(a.Workspace.Files) > 0 {
		first := a.Workspace.Files[0]
		a.SelectedFile = first.RelativePath
		_, _ = a.Editor.OpenFile(first.FullPath, first.RelativePath)
	}
	if a.Window != nil {
		a.Window.Invalidate()
	}
}

func (a *AppUI) renderMainColumn(c *ui.Context) {
	ui.Column(c).Grow(1).FillHeight().Children(func() {
		a.renderWorkspaceHeader(c)

		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			switch a.Mode {
			case ModeSettings:
				a.renderSettingsScreen(c)
			case ModeEditor:
				a.renderEditorSurface(c)
			default:
				a.renderAgentSurface(c)
			}
		})

		if a.Mode == ModeEditor {
			a.renderStatusBar(c)
		}
	})
}
