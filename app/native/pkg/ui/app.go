package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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
	"github.com/hasdev/forge-ade/core/agentsession"
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
	RightTabBrowser  = 9
	RightTabSubagent = 10
	RightTabDeck     = 11
	RightTabLauncher = 12
)

// Layout metrics of ForgeADE's CSS
const (
	WorkspaceTopBarHeight = float32(42)
	RightDrawerDefault    = float32(360)
	LeftSidebarDefault    = float32(288)
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
	Mode                  int      // ModeAgent, ModeEditor, or ModeSettings
	PrevMode              int      // Mode to restore when closing Settings
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
	SlashSelected       int
	SlashDismissedAt    string
	MentionSelected     int
	MentionDismissedAt  string
	LastFollowSessionID string
	mentionQueryCached  string
	mentionMatchVersion uint64
	mentionMatchRoot    string
	mentionMatchCached  []composerSuggestion

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
	sidePreviewImgPath   string
	sidePreviewImgMod    int64
	sidePreviewImgSrc    ui.ImageSource
	sidePreviewMdPath    string
	sidePreviewMdMod     int64
	sidePreviewMdText    string

	// Image & SVG Viewer state
	ImageZoom    map[string]float32 // zoom multiplier (0 = fit to window)
	ImageFitMode map[string]bool    // fit to window flag
	ImageBgStyle map[string]string  // "grid" (default), "dark", "light"
	SVGViewMode  map[string]string  // "preview" (default) or "code"
	imageCache   map[string]*cachedImage

	// Right-drawer browser viewer tab (native web view) + subagent pane.
	BrowserWebView    *mygo.WebView
	BrowserURLInput   string
	BrowserURLCurrent string
	BrowserZoom       float64

	// Right-drawer / Editor PDF viewer
	PDFWebView      *mygo.WebView
	PDFCurrentPath  string
	PDFFileSize     int64
	PDFFileSizePath string

	// Thread find bar (⌘F) state — ThreadFindBar parity.
	ShowThreadFind      bool
	ThreadFindQuery     string
	ThreadFindQueryAt   string
	ThreadFindSessionID string
	ThreadFindMsgCount  int
	ThreadFindMatches   []int
	ThreadFindIdx       int

	// Navigation history & ZCode inspector/card UI state.
	SessionHistory       []string
	SessionHistoryIdx    int
	navigatingHistory    bool
	ChangesCardCollapsed map[int]bool
	InspectorCollapsed   bool

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
	RenameSessionID         string
	RenameSessionIsCLI      bool
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
	explorerTreeRoot        string
	explorerTreeVersion     uint64
	explorerTreeQuery       string
	explorerTreeCached      []*FileTreeNode
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
	ShowRemoteUploadModal   bool
	RemoteUploadTargetDir   string
	RemoteUploadQueue       []RemoteUploadItem
	RemoteUploadIsUploading bool

	// External Open Target in header split button ("terminal" | "finder" | "zed")
	ExternalOpenTarget string

	// Editor In-Pane Shell
	EditorShellInPane    bool
	EditorShellPaneWidth float32

	// SSH Host Search in SSH modal
	SSHHostSearch string

	// Follow Agent: auto-jump the editor to files the agent edits.
	FollowAgent           bool
	GitHistoryWidth       float32
	SidebarSearchOpen     bool
	ShowProjectSearch     bool
	SSHKeyPassphrase      string
	SSHPassword           string
	SSHRemotePath         string
	SSHIsConnecting       bool
	SSHErrorMsg           string
	SSHSavedConfigs       []remotefs.SSHConfig
	SSHSelectedConfigID   string
	SSHIsChangingDir      bool
	SSHNewDirPath         string
	ShowUsageModal        bool
	QuotaData             map[string]interface{}
	QuotaFetching         bool
	ShowRulesModal        bool
	SelectedRulePath      string
	RuleEditorContent     string
	RuleFilesRoot         string
	RuleFilesCache        map[string]StandardRuleFile
	BoardSearchQuery      string
	BoardViewMode         string // "" = board, "list"
	ShowBoardTaskModal    bool
	BoardTaskStatus       string
	BoardTaskTitle        string
	AgentViewMode         string // AgentViewChat / AgentViewBoard / AgentViewGit
	IsGitClientOpen       bool   // Full Git Client surface in Editor Mode
	GitRemoteURL          string
	gitRemoteRoot         string
	gitRemoteLoading      bool
	ShowNewBoardTaskModal bool
	NewBoardTaskTitle     string
	NewBoardTaskDesc      string
	NewBoardTaskPriority  string
	NewBoardTaskStatus    string

	// Terminal Snippets & Command History
	Snippets                     *terminal.SnippetManager
	ShowTerminalSnippetsModal    bool
	TerminalSnippetTab           string // "snippets" | "history"
	TerminalSnippetSearch        string
	TerminalSnippetCategory      string
	TerminalSnippetDraftTitle    string
	TerminalSnippetDraftCmd      string
	TerminalSnippetDraftCat      string
	TerminalSnippetDraftDesc     string
	TerminalSnippetEditID        string
	TerminalSnippetFormOpen      bool
	TerminalSnippetTargetSession string

	// Timeline vs Project view in sidebar
	SidebarListView string // "project" (default) or "timeline"

	// Subagent chat pane & selection in right drawer
	SelectedSubagentID string
	SubagentChatInput  string

	// Peek Definition modal
	ShowPeekModal bool
	PeekSymbol    string
	PeekResults   []editor.PeekMatch

	// Rename Symbol modal
	ShowRenameSymbolModal bool
	RenameSymbolTarget    string
	RenameSymbolNewName   string

	// JSON Payload Inspector modal
	ShowJSONModal    bool
	JSONModalTitle   string
	JSONModalContent string

	// Plans & Progress Modal
	ShowPlanModal bool

	// Deck & Presentation tab
	DeckActiveSlide  int
	DeckSlideCount   int
	DeckIsPlaying    bool
	DeckWebView      *mygo.WebView
	DeckCurrentPath  string
	DeckMode         string
	DeckFiles        []string
	DeckFilesScanned bool

	closeCh       chan struct{}
	closeOnce     sync.Once
	tickerDone    chan struct{}
	lastPrefsAt   time.Time
	lastPrefsSnap string
}

// NewAppUI constructs the complete AppUI state tree.
func NewAppUI(engine *engine.Engine, rootDir string) *AppUI {
	// Always launch in Agent Mode on the New Task view; if opened with no
	// workspace folder, the terminal defaults to the user's home directory.
	terminalDir := rootDir
	if rootDir == "" {
		home, _ := os.UserHomeDir()
		terminalDir = home
	}

	appUI := &AppUI{
		Engine:                  engine,
		Workspace:               workspace.NewWorkspaceState(rootDir),
		Editor:                  editor.NewEditorManager(),
		Terminal:                terminal.NewTerminalState(terminalDir),
		ThemeMgr:                theme.NewThemeManager(),
		Mode:                    ModeAgent,
		ActiveTaskKind:          "agent",
		AgentViewMode:           AgentViewChat,
		RecentWorkspaces:        loadRecentWorkspaces(),
		ShowLeftSidebar:         true,
		LeftSidebarWidth:        LeftSidebarDefault,
		RightDrawerWidth:        RightDrawerDefault,
		BottomTerminalHeight:    240,
		ShowBottomTerminal:      false,
		ShowRightDrawer:         false,
		RightDrawerTab:          RightTabLauncher,
		ExternalOpenTarget:      "terminal",
		EditorShellPaneWidth:    380,
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
		FileSearchMode:          "files",
		SideFileDiffMode:        make(map[string]bool),
		ImageZoom:               make(map[string]float32),
		ImageFitMode:            make(map[string]bool),
		ImageBgStyle:            make(map[string]string),
		SVGViewMode:             make(map[string]string),
		imageCache:              make(map[string]*cachedImage),
		SidebarListView:         "project",
		DeckSlideCount:          1,
		Snippets:                terminal.NewSnippetManager(engine.DataDir),
		closeCh:                 make(chan struct{}),
		tickerDone:              make(chan struct{}),
	}

	appUI.Editor.WorkspaceDir = rootDir
	appUI.Editor.OnDefinitionRequested = func(symbol string, matches []editor.PeekMatch) {
		appUI.ShowPeekModal = true
		appUI.PeekSymbol = symbol
		appUI.PeekResults = matches
		if appUI.Window != nil {
			appUI.Window.Invalidate()
		}
	}

	appUI.loadUIPrefs()

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
		// returns the next queued steer prompt to run in its originating session.
		nativeID := appUI.Agent.MapInternalSession(sessionID)
		if next := appUI.Agent.HandleTurnDone(nativeID, info); next != "" {
			go appUI.Agent.TriggerTurnForSession(nativeID, next)
		}
		// Follow Agent: jump the editor to the file the turn edited
		// (layoutStore.isFollowAgentMode parity).
		if appUI.FollowAgent {
			if changed, ok := appUI.Agent.LastTurnChangedFile(nativeID); ok {
				full := appUI.Workspace.ResolveFullPath(changed)
				if _, err := appUI.Editor.OpenFile(full, changed); err == nil {
					appUI.Mode = ModeEditor
				}
				if appUI.Window != nil {
					appUI.Window.Invalidate()
				}
			}
		}
	}

	// Live elapsed indicators ("Working for 12s", spinners): repaint once a
	// second while any session is streaming.
	go func() {
		defer close(appUI.tickerDone)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-appUI.closeCh:
				return
			case <-ticker.C:
				if appUI.Agent.HasRunningSession() && appUI.Window != nil {
					appUI.Window.Invalidate()
				}
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

	gitEng := engine
	if rootDir == "" {
		gitEng = nil
	}
	appUI.Git = git.NewGitManagerLazy(gitEng, rootDir, func(path string, showDiff bool) {
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
	initGit := appUI.Git

	if engine != nil {
		initWS := appUI.Workspace
		engine.StartWatcher(rootDir, func() {
			if initWS != nil {
				initWS.Refresh()
			}
			if initGit != nil {
				initGit.Refresh()
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

	if first, ok := appUI.Workspace.FirstFile(); ok {
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

	if rootDir != "" && initGit != nil {
		go initGit.Refresh()
	}

	return appUI
}

func (a *AppUI) paletteCommands() []palette.PaletteCommand {
	return []palette.PaletteCommand{
		{ID: "new-task", Label: "New Task", Detail: "Start a new agent task", Shortcut: "⌘N", Run: func() {
			a.handleNewTask()
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
		{ID: "terminal-snippets", Label: "Terminal Snippets & History", Detail: "Saved terminal commands and history", Shortcut: "⌥S", Run: func() {
			sessID := ""
			if a.Terminal != nil {
				sessID = a.Terminal.ActiveSessionID
			}
			a.OpenTerminalSnippetsModal(sessID, "", "")
		}},
		{ID: "toggle-sidebar", Label: "Toggle Sidebar", Shortcut: "⌘B", Run: func() { a.ShowLeftSidebar = !a.ShowLeftSidebar }},
		{ID: "toggle-drawer", Label: "Toggle Right Drawer", Shortcut: "⌘J", Run: func() { a.ShowRightDrawer = !a.ShowRightDrawer }},
		{ID: "save", Label: "Save File", Shortcut: "⌘S", Run: func() { _ = a.Editor.SaveActiveFile() }},
		{ID: "settings", Label: "Open Settings", Shortcut: "⌘,", Run: func() { a.openSettings() }},
		{ID: "appearance", Label: "Toggle Appearance", Detail: "Dark / light", Shortcut: "⌘/", Run: a.toggleAppearance},
		{ID: "theme", Label: "Pick Theme Palette", Run: func() { a.ThemeMgr.Toggle() }},
		{ID: "open-folder", Label: "Open Workspace Folder", Shortcut: "⌘O", Run: a.openWorkspaceFolder},
		{ID: "board", Label: "Show Workflow Board", Run: func() {
			a.Mode = ModeAgent
			a.AgentViewMode = AgentViewBoard
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
		{ID: "ssh-upload", Label: "SSH: Upload Files to Remote Host (SFTP)", Detail: "Upload local files or folders to active remote workspace", Run: func() {
			a.OpenRemoteFileUploader("")
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

// loadGitRemoteURL reads the origin remote URL for the Git Client badge asynchronously.
func (a *AppUI) loadGitRemoteURL() {
	if a.Engine == nil || a.Workspace == nil {
		return
	}
	dir := a.Workspace.RootDir
	if dir == "" || strings.HasPrefix(dir, "ssh://") {
		return
	}
	if a.gitRemoteLoading || a.gitRemoteRoot == dir {
		return
	}
	a.gitRemoteLoading = true
	a.gitRemoteRoot = dir
	go func(root string) {
		defer func() {
			a.gitRemoteLoading = false
			if a.Window != nil {
				a.Window.Invalidate()
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, "git", "-C", root, "config", "--get", "remote.origin.url").Output()
		if err == nil && a.Workspace != nil && a.Workspace.RootDir == root {
			a.GitRemoteURL = strings.TrimSpace(string(out))
		}
	}(dir)
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
		// Project search modal (Zed style), prefilled with the selection.
		a.openProjectSearch()
	}
	if cmdOrCtrl(ui.KeyY) && a.Agent != nil && a.Agent.ActiveSession() != nil {
		// Approve the pending permission (⌘Y, AgentApprovalBar parity).
		if as := a.Agent.ActiveSession(); as != nil && len(as.Messages) > 0 {
			if last := as.Messages[len(as.Messages)-1]; last.PendingApproval != nil && !last.PendingApproval.Resolved {
				a.resolvePendingApproval(last, true, false)
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
	if c.Shortcut(0, ui.KeyF12) && a.Mode == ModeEditor {
		doc := a.Editor.ActiveDoc()
		if doc != nil {
			sym := a.symbolUnderCursor(doc)
			if sym != "" {
				matches := a.Editor.FindDefinitions(sym, a.Workspace.RootDir)
				if len(matches) == 1 {
					a.openSearchResult(matches[0].Path, matches[0].Line, "")
				} else if len(matches) > 1 {
					a.ShowPeekModal = true
					a.PeekSymbol = sym
					a.PeekResults = matches
				}
			}
		}
	}
	if c.Shortcut(0, ui.KeyF2) && a.Mode == ModeEditor {
		doc := a.Editor.ActiveDoc()
		if doc != nil {
			sym := a.symbolUnderCursor(doc)
			if sym != "" {
				a.ShowRenameSymbolModal = true
				a.RenameSymbolTarget = sym
				a.RenameSymbolNewName = sym
			}
		}
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
		a.Palette.OpenSymbols()
		a.Palette.Query = "@"
	}
	if cmdOrCtrlShift(ui.KeyG) {
		if a.Mode == ModeEditor {
			a.IsGitClientOpen = !a.IsGitClientOpen
			if a.IsGitClientOpen {
				if a.Git != nil {
					go a.Git.Refresh()
				}
				a.loadGitRemoteURL()
			}
		} else {
			a.openGitClientSurface()
		}
	}
	if cmdOrCtrlShift(ui.KeyJ) {
		a.ShowRightDrawer = !a.ShowRightDrawer
	}
	if cmdOrCtrl(ui.KeyComma) {
		if a.Mode == ModeSettings {
			a.closeSettings()
		} else {
			a.openSettings()
		}
	}
	if cmdOrCtrl(ui.KeyB) {
		a.ShowLeftSidebar = !a.ShowLeftSidebar
	}
	if cmdOrCtrl(ui.KeyN) {
		a.handleNewTask()
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
	if c.Shortcut(ui.Alt, ui.KeyS) {
		sessID := ""
		if a.Terminal != nil {
			sessID = a.Terminal.ActiveSessionID
		}
		a.OpenTerminalSnippetsModal(sessID, "", "")
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

	if time.Since(a.lastPrefsAt) >= time.Second {
		a.lastPrefsAt = time.Now()
		if snap := a.prefsSnapshot(); snap != a.lastPrefsSnap {
			a.lastPrefsSnap = snap
			a.SaveUIPrefs()
		}
	}
}

// Close releases background tickers and terminal processes owned by AppUI.
func (a *AppUI) Close() {
	if a == nil {
		return
	}
	a.closeOnce.Do(func() {
		if a.closeCh != nil {
			close(a.closeCh)
		}
	})
	if a.tickerDone != nil {
		<-a.tickerDone
	}
	if a.Engine != nil {
		a.Engine.StopWatcher()
	}
	if a.Terminal != nil {
		a.Terminal.Close()
	}
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
	a.AgentViewMode = AgentViewChat
	a.ActiveTaskKind = "agent"
	a.ShowNewTaskModal = false

	if workspacePath != "" && workspacePath != a.Workspace.RootDir {
		a.switchWorkspace(workspacePath)
	}

	if a.Agent != nil {
		a.Agent.ClearActiveSession()
		if a.Agent.ActiveAgentID != "" && a.Agent.ActiveAgentID != "agent-internal" {
			a.Agent.ActiveModel = a.Agent.ResolveModelForAgent(a.Agent.ActiveAgentID, a.Agent.ActiveModel)
			a.SelectedAgentModel = a.Agent.ActiveModel
		}
	}

	if a.Window != nil {
		a.Window.Invalidate()
	}
}

// NewAgentTaskInProject switches to Agent Mode and opens the New Agent UI
// pre-targeted to the given sidebar project group ("General" resets to empty RootDir).
func (a *AppUI) NewAgentTaskInProject(projectName, folderPath string) {
	targetWs := folderPath
	if !strings.HasPrefix(targetWs, "ssh://") {
		targetWs = agentsession.NormalizeWorkspacePath(targetWs)
	}
	if strings.EqualFold(projectName, "General") {
		targetWs = ""
	}
	curWs := ""
	if a.Workspace != nil {
		curWs = a.Workspace.RootDir
		if !strings.HasPrefix(curWs, "ssh://") {
			curWs = agentsession.NormalizeWorkspacePath(curWs)
		}
	}

	a.Mode = ModeAgent
	a.AgentViewMode = AgentViewChat
	a.ActiveTaskKind = "agent"
	a.ShowNewTaskModal = false
	if projectName != "" && a.CollapsedProjectGroups != nil {
		delete(a.CollapsedProjectGroups, projectName)
	}

	if a.Workspace == nil || targetWs != curWs {
		a.switchWorkspace(targetWs)
	}

	if a.Agent != nil {
		a.Agent.ClearActiveSession()
		if a.Agent.ActiveAgentID != "" && a.Agent.ActiveAgentID != "agent-internal" {
			a.Agent.ActiveModel = a.Agent.ResolveModelForAgent(a.Agent.ActiveAgentID, a.Agent.ActiveModel)
			a.SelectedAgentModel = a.Agent.ActiveModel
		}
	}

	if a.Window != nil {
		a.Window.Invalidate()
	}
}

func (a *AppUI) openRenameSessionModal(sessID, currentTitle string, isCLI bool) {
	a.RenameSessionID = sessID
	a.RenameSessionIsCLI = isCLI
	a.RenameSessionTitle = currentTitle
	a.ShowRenameModal = true
	if a.Window != nil {
		a.Window.Invalidate()
	}
}

// selectACPAgentForNewTask switches the workspace if needed, resets the composer to
// a fresh New Task state without stopping running background sessions, and auto-selects
// the default model and reasoning effort defined by agentID.
func (a *AppUI) selectACPAgentForNewTask(agentID, wsPath string) {
	curWs := ""
	if a.Workspace != nil {
		curWs = a.Workspace.RootDir
	}
	if wsPath != "" && !sameWorkspacePath(wsPath, curWs) {
		a.switchWorkspace(wsPath)
	}
	if a.Agent != nil {
		a.Agent.ClearActiveSession()
		model := a.Agent.SelectACPAgent(agentID)
		a.SelectedAgentID = a.Agent.ActiveAgentID
		a.SelectedHarnessID = a.Agent.ActiveAgentID
		if model != "" {
			a.SelectedAgentModel = model
		}
	} else {
		a.SelectedAgentID = agentID
		a.SelectedHarnessID = agentID
	}
	a.ActiveTaskKind = "agent"
	a.Mode = ModeAgent
	a.AgentViewMode = AgentViewChat
	a.ShowNewTaskModal = false
	if a.Window != nil {
		a.Window.Invalidate()
	}
}

// handleNewTask prepares a fresh task session in the current or general workspace
// without opening any modal dialog, exactly matching AgentSidebar.tsx handleNewTask.
func (a *AppUI) handleNewTask() {
	if a.Agent != nil {
		a.Agent.ClearActiveSession()
		if a.Agent.ActiveAgentID != "" && a.Agent.ActiveAgentID != "agent-internal" {
			a.Agent.ActiveModel = a.Agent.ResolveModelForAgent(a.Agent.ActiveAgentID, a.Agent.ActiveModel)
			a.SelectedAgentModel = a.Agent.ActiveModel
		}
	}
	a.ActiveTaskKind = "agent"
	a.Mode = ModeAgent
	a.AgentViewMode = AgentViewChat
	a.ShowNewTaskModal = false
	if a.Window != nil {
		a.Window.Invalidate()
	}
}

func (a *AppUI) OpenNewTaskModal() {
	a.ShowNewTaskModal = true
	a.NewTaskWorkspacePath = ""
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
	if !strings.HasPrefix(folderPath, "ssh://") {
		folderPath = agentsession.NormalizeWorkspacePath(folderPath)
	}
	a.NewTaskWorkspacePath = folderPath
}

// RemoteUploadItem tracks a file or folder in the RemoteFileUploaderModal queue.
type RemoteUploadItem struct {
	ID        string
	LocalPath string
	Name      string
	Size      int64
	DestPath  string
	Status    string // "pending" | "uploading" | "completed" | "error"
	ErrorMsg  string
}

// OpenRemoteFileUploader opens the RemoteFileUploaderModal for the current remote workspace.
func (a *AppUI) OpenRemoteFileUploader(targetDir string) {
	if targetDir == "" && a.Workspace != nil {
		targetDir = a.Workspace.RootDir
	}
	if strings.HasPrefix(targetDir, "ssh://") {
		_, remPath, _, _ := parseSSHWorkspaceURI(targetDir)
		targetDir = remPath
	}
	if targetDir == "" {
		targetDir = "/"
	}
	a.RemoteUploadTargetDir = targetDir
	a.RemoteUploadQueue = nil
	a.RemoteUploadIsUploading = false
	a.ShowRemoteUploadModal = true
	if a.Window != nil {
		a.Window.Invalidate()
	}
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
	var raw []string
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	seen := make(map[string]bool, len(raw))
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		norm := p
		if !strings.HasPrefix(norm, "ssh://") {
			norm = agentsession.NormalizeWorkspacePath(norm)
		}
		if norm == "" || norm == "/" || agentsession.IsTempTestPath(norm) || seen[norm] {
			continue
		}
		seen[norm] = true
		out = append(out, norm)
	}
	return out
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
// persists it. Empty paths (General) and temporary test paths are not recorded.
func (a *AppUI) recordRecentWorkspace(dir string) {
	if dir == "" || dir == "/" || strings.HasPrefix(dir, "ssh://") {
		return
	}
	norm := agentsession.NormalizeWorkspacePath(dir)
	if norm == "" || norm == "/" || agentsession.IsTempTestPath(norm) {
		return
	}
	rest := make([]string, 0, len(a.RecentWorkspaces))
	for _, p := range a.RecentWorkspaces {
		normP := agentsession.NormalizeWorkspacePath(p)
		if normP != "" && normP != norm && !agentsession.IsTempTestPath(normP) {
			rest = append(rest, normP)
		}
	}
	a.RecentWorkspaces = append([]string{norm}, rest...)
	if len(a.RecentWorkspaces) > recentWorkspacesCap {
		a.RecentWorkspaces = a.RecentWorkspaces[:recentWorkspacesCap]
	}
	saveRecentWorkspaces(a.RecentWorkspaces)
}

// openSearchResult opens a search result (file or symbol) according to current mode:
// in Agent mode, it opens in the right sidebar without leaving agent chat;
// in Editor mode, it opens in the main editor pane.
func (a *AppUI) openSearchResult(relPath string, line int, query string) {
	if relPath == "" {
		return
	}
	full := a.Workspace.ResolveFullPath(relPath)
	doc, _ := a.Editor.OpenFile(full, relPath)
	if doc != nil && line > 0 {
		a.jumpDocumentToLine(doc, line, 0)
	}
	a.SelectedFile = relPath

	if a.Mode == ModeAgent {
		// Agent mode: keep agent chat in main pane, open file in right sidebar!
		a.openSideFile(relPath)
		if doc != nil && line > 0 && doc.Editor != nil {
			a.jumpDocumentToLine(doc, line, 0)
		}
	} else {
		// Editor mode: open in main pane
		a.Mode = ModeEditor
		a.IsGitClientOpen = false
	}
	if a.Window != nil {
		a.Window.Invalidate()
	}
}

// symbolUnderCursor returns the identifier under the active cursor or current selection.
func (a *AppUI) symbolUnderCursor(doc *editor.OpenDocument) string {
	if doc == nil || doc.Editor == nil || doc.Editor.Buffer == nil {
		return ""
	}
	selText := strings.TrimSpace(doc.Editor.SelectedText())
	if selText != "" && !strings.ContainsAny(selText, " \t\r\n(){}[],;:") {
		return selText
	}
	cursor := doc.Editor.Cursor()
	word, _, _ := editor.WordAtOffset(doc.Editor.Buffer, cursor)
	return word
}

// openSideFile opens a workspace file in the right sidebar's file tabs, as
// forge-ade's explorer "Open in Side Preview" does.
func (a *AppUI) openSideFile(relPath string) {
	full := a.Workspace.ResolveFullPath(relPath)
	lower := strings.ToLower(relPath)
	if strings.HasSuffix(lower, ".pdf") {
		a.openPdfViewer(full)
	} else if isDeckOrMotionFile(lower) {
		a.openDeckViewer(full)
	} else if isImageOrSVG(lower) {
		a.SideFilePreviewImage = relPath
		a.SideFilePreviewMd = ""
		if doc := a.Editor.GetDoc(full); doc == nil {
			_, _ = a.Editor.OpenFile(full, relPath)
		}
	} else {
		if doc := a.Editor.GetDoc(full); doc == nil {
			_, _ = a.Editor.OpenFile(full, relPath)
		}
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
	lower := strings.ToLower(relPath)
	if strings.HasSuffix(lower, ".pdf") {
		a.closePdfViewer()
	} else if isDeckOrMotionFile(lower) {
		a.closeDeckViewer()
	}
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
// across all tabs and removes it from recent workspaces (switching back to General
// when it is the active workspace). Files inside the project directory are untouched.
func (a *AppUI) deleteWorkspaceGroup(c *ui.Context, groupName, folderPath string) {
	normFolder := folderPath
	if !strings.HasPrefix(normFolder, "ssh://") {
		normFolder = agentsession.NormalizeWorkspacePath(folderPath)
	}
	if strings.EqualFold(groupName, "General") || (groupName == "" && normFolder == "") {
		return
	}
	if a.Agent != nil {
		a.Agent.DeleteSessionsByProject(groupName, normFolder)
	}
	if a.CLIMgr != nil {
		for _, cs := range a.CLIMgr.SortedSessions() {
			if cs == nil {
				continue
			}
			csWs := agentsession.NormalizeWorkspacePath(cs.Cwd)
			if (normFolder != "" && csWs == normFolder) || (normFolder == "" && resolveProjectName(csWs) == groupName) {
				a.CLIMgr.DeleteSession(cs.ID)
			}
		}
	}
	if len(a.RecentWorkspaces) > 0 {
		kept := make([]string, 0, len(a.RecentWorkspaces))
		for _, p := range a.RecentWorkspaces {
			normP := p
			if !strings.HasPrefix(normP, "ssh://") {
				normP = agentsession.NormalizeWorkspacePath(p)
			}
			if normP == "" {
				continue
			}
			if (normFolder != "" && normP == normFolder) || (groupName != "" && resolveProjectName(normP) == groupName) {
				continue
			}
			kept = append(kept, normP)
		}
		a.RecentWorkspaces = kept
		saveRecentWorkspaces(a.RecentWorkspaces)
	}
	if a.ProjectFilter == groupName {
		a.ProjectFilter = "all"
	}
	if a.Workspace != nil {
		curWs := a.Workspace.RootDir
		if !strings.HasPrefix(curWs, "ssh://") {
			curWs = agentsession.NormalizeWorkspacePath(curWs)
		}
		if (normFolder != "" && curWs == normFolder) || (normFolder == "" && resolveProjectName(curWs) == groupName) {
			a.switchWorkspace("")
		}
	}
	if c != nil {
		c.AddToast(ui.Toast{Title: "Workspace Removed", Description: groupName, Type: "info"})
	}
	if a.Window != nil {
		a.Window.Invalidate()
	}
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

func sameWorkspacePath(aDir, bDir string) bool {
	normA := aDir
	if !strings.HasPrefix(normA, "ssh://") {
		normA = agentsession.NormalizeWorkspacePath(aDir)
	}
	normB := bDir
	if !strings.HasPrefix(normB, "ssh://") {
		normB = agentsession.NormalizeWorkspacePath(bDir)
	}
	return normA == normB
}

// selectAgentSession activates the agent chat session first and switches the
// active workspace to match (if different) without clearing the newly selected session.
func (a *AppUI) selectAgentSession(s *agent.AgentSession) {
	if s == nil {
		return
	}
	if !a.navigatingHistory {
		a.recordSessionHistory(s.ID)
	}
	a.ActiveTaskKind = "agent"
	a.AgentViewMode = AgentViewChat
	if a.Agent != nil {
		a.Agent.SelectSession(s.ID)
	}
	targetWs := agentsession.NormalizeWorkspacePath(s.WorkspacePath)
	curWs := ""
	if a.Workspace != nil {
		curWs = a.Workspace.RootDir
	}
	if a.Workspace == nil || !sameWorkspacePath(curWs, targetWs) {
		a.switchWorkspaceInternal(targetWs, true)
	} else if a.Window != nil {
		a.Window.Invalidate()
	}
}

func (a *AppUI) recordSessionHistory(id string) {
	if id == "" {
		return
	}
	if a.SessionHistoryIdx >= 0 && a.SessionHistoryIdx < len(a.SessionHistory) {
		if a.SessionHistory[a.SessionHistoryIdx] == id {
			return
		}
		a.SessionHistory = append(a.SessionHistory[:a.SessionHistoryIdx+1], id)
	} else {
		a.SessionHistory = append(a.SessionHistory, id)
	}
	if len(a.SessionHistory) > 40 {
		a.SessionHistory = a.SessionHistory[len(a.SessionHistory)-40:]
	}
	a.SessionHistoryIdx = len(a.SessionHistory) - 1
}

// cycleAgentSession steps backward (delta < 0) or forward (delta > 0) through
// visited sessions or, when history has <2 entries, through active sessions.
func (a *AppUI) cycleAgentSession(delta int) {
	if a.Agent == nil {
		return
	}
	if len(a.SessionHistory) >= 2 {
		nextIdx := a.SessionHistoryIdx + delta
		if nextIdx >= 0 && nextIdx < len(a.SessionHistory) {
			targetID := a.SessionHistory[nextIdx]
			if s := a.findSessionByID(targetID); s != nil {
				a.SessionHistoryIdx = nextIdx
				a.navigatingHistory = true
				a.selectAgentSession(s)
				a.navigatingHistory = false
				return
			}
		}
	}
	a.Agent.Lock()
	sessions := append([]*agent.AgentSession(nil), a.Agent.Sessions...)
	activeID := a.Agent.ActiveSessionID
	a.Agent.Unlock()
	if len(sessions) == 0 {
		return
	}
	cur := 0
	for i, s := range sessions {
		if s.ID == activeID {
			cur = i
			break
		}
	}
	next := (cur + delta + len(sessions)) % len(sessions)
	a.selectAgentSession(sessions[next])
}

func (a *AppUI) findSessionByID(id string) *agent.AgentSession {
	if a.Agent == nil || id == "" {
		return nil
	}
	a.Agent.Lock()
	defer a.Agent.Unlock()
	for _, s := range a.Agent.Sessions {
		if s != nil && s.ID == id {
			return s
		}
	}
	return nil
}

// selectCLISession activates the CLI session first and switches the active
// workspace to match its working directory (if different).
func (a *AppUI) selectCLISession(cs *agent.AgentCLISession) {
	if cs == nil {
		return
	}
	a.ActiveTaskKind = "cli"
	a.AgentViewMode = AgentViewChat
	if a.CLIMgr != nil {
		a.CLIMgr.ActiveID = cs.ID
	}
	targetWs := agentsession.NormalizeWorkspacePath(cs.Cwd)
	curWs := ""
	if a.Workspace != nil {
		curWs = a.Workspace.RootDir
	}
	if a.Workspace == nil || !sameWorkspacePath(curWs, targetWs) {
		a.switchWorkspaceInternal(targetWs, true)
	} else if a.Window != nil {
		a.Window.Invalidate()
	}
}

func (a *AppUI) switchWorkspace(dir string) {
	a.switchWorkspaceInternal(dir, false)
}

func (a *AppUI) switchWorkspaceInternal(dir string, keepActiveSession bool) {
	rawDir := dir
	if !strings.HasPrefix(rawDir, "ssh://") {
		rawDir = agentsession.NormalizeWorkspacePath(dir)
	}
	a.Workspace = workspace.NewWorkspaceState(rawDir)
	a.recordRecentWorkspace(rawDir)
	if a.Engine != nil {
		a.Workspace.RemoteFS = a.Engine.RemoteFS
		if strings.HasPrefix(rawDir, "ssh://") {
			a.Workspace.Refresh()
		}
	}
	if a.Editor != nil {
		a.Editor.WorkspaceDir = rawDir
		if a.Engine != nil {
			a.Editor.RemoteFS = a.Engine.RemoteFS
		}
	}
	if a.Agent != nil {
		if !keepActiveSession {
			if sess := a.Agent.ActiveSession(); sess != nil && agentsession.NormalizeWorkspacePath(sess.WorkspacePath) != rawDir {
				a.Agent.ClearActiveSession()
			}
		}
		a.Agent.SetWorkspacePath(rawDir)
	}
	termDir := rawDir
	if termDir == "" {
		home, _ := os.UserHomeDir()
		termDir = home
	}
	if a.Terminal != nil {
		if strings.HasPrefix(rawDir, "ssh://") {
			a.Terminal.Cwd = rawDir
		} else if a.Terminal.Cwd != termDir {
			_ = a.Terminal.ResetShell(termDir)
		}
	}
	if a.Editor != nil {
		a.Editor.CloseAll()
	}
	a.SelectedFile = ""
	a.SelectedDiffPath = ""
	a.DeckFiles = nil
	a.DeckFilesScanned = false
	a.GitRemoteURL = ""
	a.gitRemoteRoot = ""
	a.RuleFilesRoot = ""
	a.RuleFilesCache = nil
	a.PDFFileSize = 0
	a.PDFFileSizePath = ""

	if strings.HasPrefix(rawDir, "ssh://") {
		// Remote SSH workspace
		if a.Database != nil {
			a.Database.Workspace = a.Workspace
		}
		if first, ok := a.Workspace.FirstFile(); a.Editor != nil && ok {
			a.SelectedFile = first.RelativePath
			_, _ = a.Editor.OpenFile(first.FullPath, first.RelativePath)
		}
		if a.Window != nil {
			a.Window.Invalidate()
		}
		return
	}

	if a.Database != nil {
		a.Database.SetWorkspaceAndScan(a.Workspace)
	}
	if a.Git != nil {
		gitEng := a.Engine
		if rawDir == "" {
			gitEng = nil
		}
		a.Git = git.NewGitManagerLazy(gitEng, rawDir, a.Git.OnFileSelect)
		a.Git.OnNeedRedraw = func() {
			if a.Window != nil {
				a.Window.Invalidate()
			}
		}
		if rawDir != "" {
			go a.Git.Refresh()
		}
	}
	if rawDir != "" && a.Engine != nil {
		ws := a.Workspace
		gm := a.Git
		a.Engine.StartWatcher(rawDir, func() {
			if ws != nil {
				ws.Refresh()
			}
			if gm != nil {
				gm.Refresh()
			}
			if a.Window != nil {
				a.Window.Invalidate()
			}
		})
	}
	if first, ok := a.Workspace.FirstFile(); a.Editor != nil && ok {
		a.SelectedFile = first.RelativePath
		_, _ = a.Editor.OpenFile(first.FullPath, first.RelativePath)
	}
	if a.Window != nil {
		a.Window.Invalidate()
	}
}

func (a *AppUI) openSettings(tab ...int) {
	if a.Mode != ModeSettings {
		a.PrevMode = a.Mode
	}
	a.Mode = ModeSettings
	if a.Settings != nil {
		a.Settings.IsOpen = true
		a.Settings.StatusMsg = ""
		if len(tab) > 0 {
			a.Settings.ActiveTab = tab[0]
		}
	}
}

func (a *AppUI) closeSettings() {
	if a.Settings != nil {
		a.Settings.IsOpen = false
	}
	target := a.PrevMode
	if target == ModeSettings {
		target = ModeAgent
	}
	a.Mode = target
}

func (a *AppUI) renderMainColumn(c *ui.Context) {
	if a.Mode == ModeSettings {
		a.renderSettingsScreen(c)
		return
	}
	if a.Mode == ModeEditor {
		ui.Column(c).Grow(1).FillHeight().Children(func() {
			a.renderWorkspaceHeader(c)

			ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
				a.renderEditorSurface(c)
			})

			a.renderStatusBar(c)
		})
		return
	}

	// ZCode 3-column layout in Agent Mode: Center workspace (header + chat/terminal)
	// alongside a full-height collapsible Right Sidebar (spanning Y=0 to bottom).
	ui.Row(c).Grow(1).FillHeight().AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Grow(1).FillHeight().Clip().Children(func() {
			a.renderWorkspaceHeader(c)

			ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
				a.renderAgentSurface(c)
			})
		})

		if a.ShowRightDrawer {
			maxRight := float32(750)
			if winW, _ := c.Size(); winW > 0 && winW*0.6 < maxRight {
				maxRight = winW * 0.6
			}
			LeftResizeHandle(c, &a.RightDrawerWidth, 280, maxRight, RightDrawerDefault)
			a.renderRightDrawer(c)
		}
	})
}
