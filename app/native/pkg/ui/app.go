package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/Haslab-dev/RakitKode/brain"
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
	"github.com/hasdev/forge-ade/app/native/pkg/studio"
	"github.com/hasdev/forge-ade/app/native/pkg/terminal"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
	"github.com/hasdev/forge-ade/app/native/pkg/usage"
	"github.com/hasdev/forge-ade/app/native/pkg/workspace"
	"github.com/hasdev/forge-ade/core/agentsession"
	"github.com/hasdev/forge-ade/core/remotefs"
	corestudio "github.com/hasdev/forge-ade/core/studio"
)

// Application surface modes
const (
	ModeAgent    = 0
	ModeEditor   = 1
	ModeSettings = 2
	ModeStudio   = 3
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
	RightTabChanges   = 0
	RightTabTerminal  = 1
	RightTabChat      = 2
	RightTabTasks     = 3
	RightTabBoard     = 4
	RightTabGit       = 5
	RightTabDatabase  = 6
	RightTabFiles     = 7
	RightTabSideFile  = 8
	RightTabBrowser   = 9
	RightTabSubagent  = 10
	RightTabDeck      = 11
	RightTabLauncher  = 12
	RightTabArtifacts = 13
)

// Layout metrics of ForgeADE's CSS
const (
	WorkspaceTopBarHeight = float32(36)
	RightDrawerDefault    = float32(360)
	LeftSidebarDefault    = float32(288)
	TerminalHeaderHeight  = float32(34)
	StatusBarHeight       = float32(24)
	ActivityBarWidth      = float32(48)
	FileTreeDefaultWidth  = float32(230)
	FileTreeRowHeight     = float32(22)
	EditorTabBarHeight    = float32(35)
)

// SideChatSession stores conversation history and state for a side chat tab.
type SideChatSession struct {
	ID        string
	Title     string
	Messages  []*agent.ChatMessage
	Input     string
	Streaming bool
}

// ModeDrawerState stores mode-isolated right drawer visibility and tabs so
// Agent Mode and Editor Mode never share or overwrite each other's drawer panes.
type ModeDrawerState struct {
	Show   bool
	Tab    int
	Tabs   []DrawerTabInstance
	Active int
}

// AppUI is the top-level native UI application state coordinating the modular domain engines.
type AppUI struct {
	Engine                         *engine.Engine
	Window                         *mygo.Window
	Workspace                      *workspace.WorkspaceState
	Editor                         *editor.EditorManager
	Terminal                       *terminal.TerminalState
	WorkspaceTerminals             map[string]*terminal.TerminalState
	EditorBottomTerminal           *terminal.TerminalState
	EditorBottomWorkspaceTerminals map[string]*terminal.TerminalState
	AgentTerminal                  *terminal.TerminalState
	AgentWorkspaceTerminals        map[string]*terminal.TerminalState
	WorkspaceEditorSnapshots       map[string]*editor.WorkspaceEditorSnapshot
	Agent                          *agent.AgentState
	Git                            *git.GitManager
	Database                       *database.DatabaseManager
	Palette                        *palette.PaletteManager
	Settings                       *settings.SettingsManager
	ThemeMgr                       *theme.ThemeManager
	UsageMgr                       *usage.UsageManager
	ACPMgr                         *agent.ACPManager
	CLIMgr                         *agent.AgentCLIManager
	Board                          *board.BoardManager
	Studio                         *studio.StudioManager
	Mode                           int    // ModeAgent, ModeEditor, ModeSettings, or ModeStudio
	PrevMode                       int    // Mode to restore when closing Settings
	AgentWorkspacePath             string // Independent active workspace for Agent Mode
	EditorWorkspacePath            string // Independent active workspace for Editor Mode
	StudioWorkspacePath            string // Independent active workspace for Studio Mode
	StudioViewMode                 string // "canvas" (default), "nodes", "code"
	StudioActiveTab                string // "assets" (default), "skills"
	StudioPromptInput              string
	StudioTimelinePlaying          bool
	StudioTimelineTime             float64
	studioTimelineAnchor           time.Time      // wall-clock anchor while the timeline plays
	studioWebviewSynced            bool           // studio WebView state push confirmed by the page
	studioWebviewReload            bool           // force WebView reload when the same artifact path is updated
	studioAssetVer                 map[string]int // project ID → last asset-file version written
	studioLastImportAt             time.Time      // debounce for the live write-tool import
	studioArtifactScanAt           time.Time      // artifact explorer scan cache stamp
	studioArtifactCache            []studioArtifactFile
	StudioSplitCompare             bool
	StudioLeftChatWidth            float32 // Resizable left chat pane width in Studio Mode
	StudioChatCollapsed            bool    // Hide the left chat pane so the viewer gets full width
	StudioShowArtifacts            bool    // Show the agent artifact explorer panel
	StudioArtifactFilterQuery      string
	StudioPreviewPath              string // Viewer-only external file path (never imported into artifacts)
	StudioSelectedSlideIdx         int
	StudioWebView                  *mygo.WebView
	StudioWebViewURL               string
	StudioNodeWebView              *mygo.WebView
	StudioNodeWebViewURL           string
	studioNodeThemeKey             string
	StudioPositivePrompt           string
	StudioNegativePrompt           string
	StudioModel                    string
	StudioSampler                  string
	StudioSteps                    int
	StudioCFGScale                 float32
	StudioSeed                     int64
	StudioAspectRatio              string
	StudioGeneratedImagePath       string
	StudioGeneratedFormat          string
	StudioGeneratedWidth           int
	StudioGeneratedHeight          int
	StudioIsGenerating             bool
	StudioGenerationProgress       float32
	StudioGenerationStatus         string
	StudioImageWorkflow            *corestudio.WorkflowPipeline
	StudioVideoWorkflow            *corestudio.WorkflowPipeline
	StudioWorkflowJSONDraft        string
	StudioWorkflowShowJSON         bool
	StudioWorkflowLogs             []string
	StudioWorkflowError            string
	StudioWorkflowRightTab         string // "preview", "gallery", "logs"
	StudioWorkflowLeftCollapsed    bool   // Collapse left presets sidebar for more preview space
	StudioWorkflowIsVideoOutput    bool
	StudioWorkflowLastInjectedJSON string
	StudioWorkflowStepDrafts       map[string]string // transient string buffers for step TextInputs
	ActiveTaskKind                 string            // "agent" or "cli"
	ShowLeftSidebar                bool              // Left sidebar visibility
	LeftSidebarView                string            // "tasks" (default) or "files" — forge-ade's leftSidebarView
	SideFileTabs                   []string          // files opened in the right sidebar from the explorer
	ActiveSideFile                 string
	ShowBottomTerminal             bool    // Collapsible bottom terminal drawer (Editor Mode)
	ShowAgentBottomTerminal        bool    // Collapsible bottom terminal drawer (Agent Mode)
	BottomTerminalHeight           float32 // Resizable terminal height
	ShowRightDrawer                bool    // Collapsible right side pane
	RightDrawerWidth               float32 // Resizable right pane width
	LeftSidebarWidth               float32 // Resizable left sidebar width
	RightDrawerTab                 int     // RightTabChanges, RightTabTerminal, RightTabChat, etc.
	// DrawerTabs holds the OPEN drawer panes as tab instances (launcher
	// tabs, terminal sessions, review, files… — duplicates allowed, ZCode
	// style). RightDrawerTab mirrors the active entry.
	DrawerTabs            []DrawerTabInstance
	drawerActiveIdx       int
	agentDrawerState      ModeDrawerState
	editorDrawerState     ModeDrawerState
	studioDrawerState     ModeDrawerState
	activeDrawerMode      int
	EditorActivity        int // ActivityExplorer, ActivityGit, ActivityDatabase
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
	SideChatSessions      map[string]*SideChatSession
	ActiveSideChatID      string

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

	// Backing pointer for the ACP permission prompt: ui.Modal dereferences
	// its open argument every frame, so it must point at stable state —
	// passing nil panics on every permission request.
	PermissionModalOpen bool

	// Backing pointer for the editor discard/overwrite/replace confirm modal.
	EditorConfirmModalOpen bool

	// Floating turn-status card (ZCode-style: Changes / Branch / Commit /
	// Progress / Agents), toggled from the composer status strip and the
	// working pill. subagentFirstSeen timestamps each subagent ID the first
	// time the card sees it, to render "Running for Xm Ys".
	ShowTurnStatusCard bool
	ShowBgShellPopover bool
	subagentFirstSeen  map[string]time.Time

	// ToolGroupExpanded tracks the open/closed state of consecutive tool
	// step groups ("Explored 3 files · Ran 2 commands"), keyed
	// sessionID-msgID-groupStartIdx — the same pattern as
	// ThinkingBlockExpanded for per-thinking-block toggles.
	ToolGroupExpanded map[string]bool

	// StatusSectionOpen tracks the collapsible sections of the floating
	// status panel (changes / progress / agents); absent = open (default).
	StatusSectionOpen map[string]bool

	// Generic confirm dialog (confirmDialog / renderConfirmDialog): one
	// pending destructive action at a time; Escape and outside click cancel.
	ShowConfirmDialog  bool
	ConfirmDialogTitle string
	ConfirmDialogDesc  string
	ConfirmDialogLabel string // confirm button label ("Delete", "Discard", ...)
	confirmOnAccept    func()
	confirmOnCancel    func()

	// Editor data-loss confirmations staged by the EditorManager contract:
	// discard-unsaved (PendingDiscard), overwrite-changed-on-disk, and
	// workspace Replace All confirmations.
	EditorPendingDiscard   []string
	EditorPendingOverwrite string
	EditorPendingReplaceN  int
	editorReplaceDecide    func(proceed bool)

	// Git client PRs / Worktrees pane states (git_pr_worktrees.go).
	PRPane                   *prPaneState
	WTPane                   *worktreePaneState
	HunkRevertPaths          map[string]bool
	gitHistoryExpanded       map[string]bool
	gitHistoryExpandedCommit string

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

	// Local Bots, ACP Handoff, Multi-Tab & Hybrid Worker Orchestration state
	AgentOpenTabs          []string
	AgentSplitSessionID    string
	AgentSplitWidth        float32
	AgentSplitInput        string
	Bots                   *agent.BotStore
	BotsSectionCollapsed   bool
	ShowHandoffModal       bool
	HandoffSourceID        string
	HandoffTargetAgentID   string
	HandoffTargetAgentType string
	HandoffTargetModel     string
	HandoffGoal            string
	HandoffNextSteps       string
	HandoffCreateWorktree  bool
	HandoffWorktreeBranch  string
	HandoffError           string
	ShowNewBotModal        bool
	NewBotName             string
	NewBotRole             string
	NewBotPrompt           string
	NewBotAgentID          string
	NewBotAgentType        string
	ShowSpawnWorkerModal   bool
	WorkerNameInput        string
	WorkerTaskInput        string
	WorkerAgentID          string
	WorkerAgentType        string
	WorkerUseWorktree      bool
	WorkerError            string
	ImageBgStyle           map[string]string // "grid" (default), "dark", "light"
	SVGViewMode            map[string]string // "preview" (default) or "code"
	imageCache             map[string]*cachedImage

	// Render-path performance caches (avoid per-frame os.Stat / os.ReadDir / allocs)
	dirExistsCache    map[string]dirExistsEntry
	studioMediaScanAt time.Time
	studioMediaCache  []corestudio.MediaAsset
	mdOptsThemeKey    string

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
	SessionHistory        []string
	SessionHistoryIdx     int
	navigatingHistory     bool
	ChangesCardCollapsed  map[int]bool
	ThinkingBlockExpanded map[string]bool
	ThinkingScrollStates  map[string]*ui.ScrollState
	InspectorCollapsed    bool

	ShowNewFileModal            bool
	NewFileName                 string
	ShowNewFolderModal          bool
	NewFolderName               string
	ShowNewTaskModal            bool
	NewTaskTitle                string
	NewTaskWorkspacePath        string // preselected folder for group-level "+ New task"
	NewTaskAgentType            string // "acp" or "cli"
	NewTaskSearchQuery          string
	NewTaskFilterTab            string // "all", "acp", "cli"
	NewTaskSelectedKind         string // "acp", "cli"
	NewTaskSelectedAgentID      string
	NewTaskSelectedName         string
	NewTaskSelectedExec         string
	NewTaskSelectedArgs         []string
	NewTaskUninstalledOpen      bool
	NewTaskStep                 string // "agent" or "workspace"
	SelectedAgentID             string
	SelectedAgentModel          string
	ShowTaskMenuModal           bool
	ShowRenameModal             bool
	RenameSessionTitle          string
	RenameSessionID             string
	RenameSessionIsCLI          bool
	ShowRenameFileModal         bool
	RenameFileOldPath           string
	RenameFilePath              string
	ShowBranchModal             bool
	BranchSearchQuery           string
	ShowModelPalette            bool
	ModelSearchQuery            string
	SelectedHarnessID           string
	SelectedProviderTab         string
	isFetchingAntigravityModels bool
	CollapsedProjectGroups      map[string]bool
	ExpandedProjectGroups       map[string]bool
	ExpandedFileTreeFolders     map[string]bool
	explorerTreeRoot            string
	explorerTreeVersion         uint64
	explorerTreeQuery           string
	explorerTreeCached          []*FileTreeNode
	ProjectFilter               string // "all" or a project name (sidebar "All projects" menu)
	TaskSortBy                  string // "updated" (default) or "created" (sidebar sort menu)
	// ProjectOrder holds the sidebar's custom project-group display order
	// (project names, earliest first, most-recently-moved last). Names not
	// listed keep the implicit alphabetical order after the ordered ones;
	// "General" is always pinned first and never stored here. Persisted via
	// ui prefs ("project_order").
	ProjectOrder             []string
	RecentWorkspaces         []string // most-recent-first history of opened folders
	OpenWorkspaces           []string // active workspaces open in this window ("This Window" group)
	ShowProjectSwitcherModal bool
	ProjectSwitcherQuery     string
	projectSwitcherSelected  int
	ShowSSHModal             bool
	ShowSSHQuickMenu         bool
	SSHLabel                 string
	SSHHost                  string
	SSHPort                  int
	SSHPortText              string
	SSHUser                  string
	SSHAuthType              string // "key_file", "password", "agent"
	SSHKeyPath               string
	SSHUploadStatus          string
	ShowRemoteUploadModal    bool
	RemoteUploadTargetDir    string
	RemoteUploadQueue        []RemoteUploadItem
	RemoteUploadIsUploading  bool
	uploadQueueMu            sync.Mutex // guards RemoteUploadQueue across dialog/upload goroutines
	sshConnectGen            int        // SSH connect generation: cancel abandons in-flight results
	sshSFTPBusyID            string     // saved host currently running the connect-then-SFTP chain

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
	ruleLoadedContent     string // disk baseline for the selected rule (dirty checks)
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
	TerminalSnippetTargetState   *terminal.TerminalState

	// Terminal active-focus badge state, visible URL cache & centered URL modal
	termFocusWasFocused  map[*terminal.TerminalState]bool
	termURLCache         map[*terminal.TerminalState]termURLCacheEntry
	ShowTerminalURLModal bool
	TerminalURLModalURLs []string

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

	closeCh        chan struct{}
	closeOnce      sync.Once
	tickerDone     chan struct{}
	lastGitPollAt  time.Time // ticker goroutine only: streaming git numstat poll
	lastPrefsAt    time.Time
	lastPrefsSnap  string
	prefsDirty     atomic.Bool // render detected a pref change; ticker persists it
	prefsSaving    atomic.Bool // a SaveUIPrefs write is in flight
	lastAutoSaveAt time.Time
	shellWarned    atomic.Bool // DefaultShell LookPath failure toasted once (ticker goroutine writes, View may read)

	localLLMOnlineFlag bool      // last probe result for Ollama / LM Studio
	localLLMCheckedAt  time.Time // when localLLMOnlineFlag was last refreshed

	// Cached window background image state (avoids per-frame os.Stat syscalls).
	bgImagePath      string
	bgImageCheckedAt time.Time
	bgImageCached    *cachedImage

	// pendingToasts queues toasts raised from background goroutines; Context.AddToast
	// silently drops calls made outside a frame, so View drains the queue in-frame.
	toastMu       sync.Mutex
	pendingToasts []ui.Toast
}

// queueToast records a toast from any goroutine and requests a frame so View
// can show it. Never call Context.AddToast directly from a goroutine.
func (a *AppUI) queueToast(t ui.Toast) {
	a.toastMu.Lock()
	a.pendingToasts = append(a.pendingToasts, t)
	a.toastMu.Unlock()
	if a.Window != nil {
		a.Window.Invalidate()
	}
}

// drainToasts flushes queued goroutine toasts into the frame context.
// Call once at the top of View.
func (a *AppUI) drainToasts(c *ui.Context) {
	a.toastMu.Lock()
	if len(a.pendingToasts) == 0 {
		a.toastMu.Unlock()
		return
	}
	pending := a.pendingToasts
	a.pendingToasts = nil
	a.toastMu.Unlock()
	for _, t := range pending {
		c.AddToast(t)
	}
}

// runOnUI runs fn on the UI/main thread and triggers a frame. Headless runs
// (tests) have no window, so fn executes inline there.
func (a *AppUI) runOnUI(fn func()) {
	if a.Window != nil {
		a.Window.Update(func() {
			defer recoverFrame("runOnUI")
			fn()
		})
	} else {
		defer recoverFrame("runOnUI-headless")
		fn()
	}
}

// recoverFrame converts a panicking frame callback into a logged error
// instead of killing the whole process. A panic mid-frame leaves the tree
// inconsistent, so the next frame rebuilds from scratch — degraded but
// alive beats dead.
func recoverFrame(where string) {
	if r := recover(); r != nil {
		log.Printf("[forge-ade] recovered panic in %s: %v\n%s", where, r, debug.Stack())
	}
}

// invalidateUI requests a redraw; safe from any goroutine and from headless
// runs (no window).
func (a *AppUI) invalidateUI() {
	if a != nil && a.Window != nil {
		a.Window.Invalidate()
	}
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
		Engine:                         engine,
		Workspace:                      workspace.NewWorkspaceStateLazy(rootDir),
		Editor:                         editor.NewEditorManager(),
		Terminal:                       terminal.NewTerminalStateDeferred(terminalDir),
		WorkspaceTerminals:             make(map[string]*terminal.TerminalState),
		EditorBottomTerminal:           terminal.NewTerminalStateDeferred(terminalDir),
		EditorBottomWorkspaceTerminals: make(map[string]*terminal.TerminalState),
		AgentTerminal:                  terminal.NewTerminalStateDeferred(terminalDir),
		AgentWorkspaceTerminals:        make(map[string]*terminal.TerminalState),
		WorkspaceEditorSnapshots:       make(map[string]*editor.WorkspaceEditorSnapshot),
		ThemeMgr:                       theme.NewThemeManager(),
		Studio:                         studio.NewStudioManager(rootDir),
		StudioWorkspacePath:            rootDir,
		StudioViewMode:                 "canvas",
		StudioActiveTab:                "assets",
		StudioLeftChatWidth:            420,
		studioAssetVer:                 make(map[string]int),
		StudioPositivePrompt:           "Cyberpunk neon city street at dusk, cinematic volumetric fog, raytraced reflections, 8k octane render",
		StudioNegativePrompt:           "blurry, low quality, artifacts, watermark",
		StudioModel:                    "flux-1-schnell",
		StudioSampler:                  "Euler a",
		StudioSteps:                    25,
		StudioCFGScale:                 7.5,
		StudioSeed:                     4928172,
		StudioAspectRatio:              "16:9",
		Mode:                           ModeAgent,
		activeDrawerMode:               ModeAgent,
		AgentWorkspacePath:             rootDir,
		EditorWorkspacePath:            rootDir,
		ActiveTaskKind:                 "agent",
		AgentViewMode:                  AgentViewChat,
		RecentWorkspaces:               loadRecentWorkspaces(),
		ShowLeftSidebar:                true,
		LeftSidebarWidth:               LeftSidebarDefault,
		RightDrawerWidth:               RightDrawerDefault,
		BottomTerminalHeight:           240,
		ShowBottomTerminal:             false,
		ShowAgentBottomTerminal:        false,
		ShowRightDrawer:                false,
		RightDrawerTab:                 RightTabLauncher,
		editorDrawerState:              ModeDrawerState{Tab: RightTabLauncher},
		ExternalOpenTarget:             "terminal",
		EditorShellPaneWidth:           380,
		EditorActivity:                 ActivityExplorer,
		NewTaskAgentType:               "acp",
		NewTaskFilterTab:               "all",
		NewTaskStep:                    "agent",
		SelectedAgentID:                "agent-agy",
		SelectedAgentModel:             "gemini-3.8-flash-high",
		SelectedHarnessID:              "agent-agy",
		SelectedProviderTab:            "all",
		CollapsedProjectGroups:         make(map[string]bool),
		ExpandedProjectGroups:          make(map[string]bool),
		ProjectFilter:                  "all",
		TaskSortBy:                     "updated",
		ExpandedFileTreeFolders:        make(map[string]bool),
		SSHPort:                        22,
		SSHPortText:                    "22",
		SSHAuthType:                    "key_file",
		SSHKeyPath:                     "~/.ssh/id_ed25519",
		SSHRemotePath:                  "/var/www",
		SSHSavedConfigs:                make([]remotefs.SSHConfig, 0),
		SideChatModel:                  "Claude 3.7 Sonnet",
		SideChatMode:                   "full",
		SideChatMessages:               make([]*agent.ChatMessage, 0),
		SideChatSessions:               make(map[string]*SideChatSession),
		QuestionDrafts:                 make(map[string]*string),
		InlineAI:                       editor.NewInlineAI(),
		FileSearchMode:                 "files",
		SideFileDiffMode:               make(map[string]bool),
		ImageZoom:                      make(map[string]float32),
		Bots:                           agent.NewBotStore(""),
		ImageFitMode:                   make(map[string]bool),
		ImageBgStyle:                   make(map[string]string),
		SVGViewMode:                    make(map[string]string),
		imageCache:                     make(map[string]*cachedImage),
		SidebarListView:                "project",
		DeckSlideCount:                 1,
		Snippets:                       terminal.NewSnippetManager(engine.DataDir),
		closeCh:                        make(chan struct{}),
		tickerDone:                     make(chan struct{}),
	}

	appUI.Editor.WorkspaceDir = rootDir
	if appUI.Studio != nil {
		appUI.Studio.SetOnChanged(func() {
			if appUI.Window != nil {
				appUI.Window.Invalidate()
			}
		})
	}
	normInitial := normalizeWorkspaceKey(rootDir)
	appUI.WorkspaceTerminals[normInitial] = appUI.Terminal
	appUI.EditorBottomWorkspaceTerminals[normInitial] = appUI.EditorBottomTerminal
	appUI.AgentWorkspaceTerminals[normInitial] = appUI.AgentTerminal
	appUI.Editor.OnDefinitionRequested = func(symbol string, matches []editor.PeekMatch) {
		appUI.ShowPeekModal = true
		appUI.PeekSymbol = symbol
		appUI.PeekResults = matches
		if appUI.Window != nil {
			appUI.Window.Invalidate()
		}
	}

	// Data-loss contracts: never discard a dirty buffer, clobber a file that
	// changed on disk, or run a workspace-wide replace without an explicit
	// confirmation. Callbacks may fire off the UI thread (auto-save ticker,
	// search goroutines), so all state hops through runOnUI.
	appUI.Editor.ConfirmBeforeDiscard = true
	appUI.Editor.OnConfirmDiscard = func(paths []string, decide func(discard bool)) {
		appUI.runOnUI(func() {
			appUI.EditorPendingDiscard = paths
			appUI.invalidateUI()
		})
	}
	appUI.Editor.OnConfirmOverwrite = func(path string, decide func(proceed bool)) {
		appUI.runOnUI(func() {
			appUI.EditorPendingOverwrite = path
			appUI.invalidateUI()
		})
	}
	appUI.Editor.OnConfirmReplace = func(matchCount int, decide func(proceed bool)) {
		appUI.runOnUI(func() {
			appUI.EditorPendingReplaceN = matchCount
			appUI.editorReplaceDecide = decide
			appUI.invalidateUI()
		})
	}
	appUI.Editor.OnSaveError = func(path string, err error) {
		appUI.queueToast(ui.Toast{
			Title:       "Save Failed",
			Description: fmt.Sprintf("%s: %v", filepath.Base(path), err),
			Type:        "error",
		})
	}
	appUI.Editor.OnTabClosed = func(path string) {
		appUI.runOnUI(func() {
			if path != "" && path == appUI.PDFCurrentPath {
				appUI.closePdfViewer()
			}
			if path != "" && path == appUI.DeckCurrentPath {
				appUI.closeDeckViewer()
			}
			appUI.invalidateUI()
		})
	}
	appUI.Editor.OnStateChanged = func() {
		appUI.invalidateUI()
	}

	appUI.loadUIPrefs()

	appUI.Agent = agent.NewAgentStateLazy(engine, rootDir, func() {
		if appUI.Window != nil {
			appUI.Window.Invalidate()
		}
	})
	appUI.Agent.Theme = appUI.ThemeMgr
	appUI.Agent.PromptContextDecorator = func(sessionID, prompt string) string {
		if appUI.Mode == ModeStudio {
			switch detectStudioIntent(prompt) {
			case IntentMotion:
				appUI.studioEnsureProjectForType(corestudio.ArtifactTypeMotion)
			case IntentSlides:
				appUI.studioEnsureProjectForType(corestudio.ArtifactTypeSlides)
			case Intent3D:
				appUI.studioEnsureProjectForType(corestudio.ArtifactTypeCodeJS)
			case IntentImage:
				appUI.studioEnsureProjectForType(corestudio.ArtifactTypeImage)
			case IntentVideo:
				appUI.studioEnsureProjectForType(corestudio.ArtifactTypeVideo)
			}
			return appUI.decorateStudioPrompt(sessionID, prompt)
		}
		return prompt
	}

	appUI.ACPMgr = agent.NewACPManagerLazy(engine, func() {
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
		// Studio Mode live import: as soon as the agent finishes writing a
		// file, offer it to the active studio project (debounced) — the
		// viewer updates mid-turn instead of waiting for turn end.
		name := strings.ToLower(ev.Name)
		if appUI.Mode == ModeStudio && ev.Status == "completed" &&
			(strings.Contains(name, "write") || strings.Contains(name, "edit") ||
				strings.Contains(name, "create") || strings.Contains(name, "apply")) {
			appUI.runOnUI(func() {
				if time.Since(appUI.studioLastImportAt) < 800*time.Millisecond {
					return
				}
				appUI.studioLastImportAt = time.Now()
				appUI.studioImportAgentArtifacts(ev.SessionID)
				if appUI.Window != nil {
					appUI.Window.Invalidate()
				}
			})
		}
	}
	appUI.ACPMgr.OnTurnStart = func(sessionID, prompt string) {
		appUI.Agent.HandleTurnStart(appUI.Agent.MapInternalSession(sessionID), prompt)
	}
	appUI.ACPMgr.OnAsk = func(req *agent.PermissionRequest) {
		req.SessionID = appUI.Agent.MapInternalSession(req.SessionID)
		appUI.Agent.HandleAsk(req)
	}
	appUI.ACPMgr.OnQuestions = func(sessionID string, questions []agent.AgentQuestion) {
		nativeID := appUI.Agent.MapInternalSession(sessionID)
		if nativeID == "" {
			// The ask may come from a skill sub-agent without a native session
			// mapping — surface the card on the active session so the user
			// still sees the questions and can answer them.
			nativeID = appUI.Agent.ActiveSessionID
		}
		if nativeID != "" {
			appUI.Agent.HandleQuestions(nativeID, questions)
		} else {
			log.Printf("[acp] ask questions dropped: no session for %q", sessionID)
		}
	}
	appUI.ACPMgr.OnUsage = func(sessionID string, u agent.UsageInfo) {
		nativeID := appUI.Agent.MapInternalSession(sessionID)
		appUI.Agent.HandleUsage(nativeID, u)
		if appUI.UsageMgr != nil {
			title := ""
			model := ""
			prov := ""
			if sess := appUI.Agent.FindSession(nativeID); sess != nil {
				title = sess.Title
				model = sess.Model
				prov = sess.Provider
			}
			appUI.UsageMgr.RecordTurn(nativeID, title, model, prov, u.InputTokens, u.OutputTokens, u.CachedTokens, u.TotalTokens, u.CostUSD)
		}
	}
	appUI.ACPMgr.OnCommands = func(sessionID string, commands []string) {
		appUI.Agent.HandleCommands(appUI.Agent.MapInternalSession(sessionID), commands)
	}

	appUI.ACPMgr.OnTurnDone = func(sessionID string, info agent.TurnEndInfo) {
		// Finalizes the message, maps the status, records turn changes, and
		// returns the next queued steer prompt to run in its originating session.
		// Agent state is lock-guarded, but every UI mutation (Mode, Editor,
		// FollowAgent) below MUST run on the main thread: this handler executes
		// on the ACPManager event-consumer goroutine (P1-2).
		nativeID := appUI.Agent.MapInternalSession(sessionID)

		if next := appUI.Agent.HandleTurnDone(nativeID, info); next != "" {
			go appUI.Agent.TriggerTurnForSession(nativeID, next)
		}
		// Studio Mode: import the files the agent wrote this turn as a new
		// project version so the viewer shows the real artifact instead of
		// the seeded placeholder.
		if appUI.Mode == ModeStudio {
			appUI.runOnUI(func() {
				appUI.studioImportAgentArtifacts(nativeID)
				if appUI.Window != nil {
					appUI.Window.Invalidate()
				}
			})
		}
		// Follow Agent: jump the editor to the file the turn edited
		// (layoutStore.isFollowAgentMode parity).
		appUI.runOnUI(func() {
			if !appUI.FollowAgent {
				return
			}
			if changed, ok := appUI.Agent.LastTurnChangedFile(nativeID); ok {
				full := appUI.Workspace.ResolveFullPath(changed)
				if _, err := appUI.Editor.OpenFile(full, changed); err == nil {
					appUI.Mode = ModeEditor
				}
				if appUI.Window != nil {
					appUI.Window.Invalidate()
				}
			}
		})
	}

	appUI.CLIMgr = agent.NewAgentCLIManagerLazy(rootDir, func() {
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

	homeDir, _ := os.UserHomeDir()
	provResolver := func(model, provID string) string {
		if appUI.Settings != nil {
			for _, prof := range appUI.Settings.GetProfiles() {
				if prof.ID == provID {
					return prof.Name
				}
			}
		}
		return usage.DefaultProviderName(model, provID)
	}
	appUI.UsageMgr = usage.NewUsageManager(filepath.Join(homeDir, ".forge-ade"), provResolver)
	go appUI.UsageMgr.BackfillHistoricalSessions()

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
		if engine.MemoryMgr != nil && rootDir != "" {
			engine.MemoryMgr.SetWorkspace(rootDir)
		}
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

	if rootDir != "" && !strings.HasPrefix(rootDir, "ssh://") {
		initWS := appUI.Workspace
		go func() {
			initWS.Refresh()
			if first, ok := initWS.FirstFile(); ok {
				appUI.runOnUI(func() {
					if appUI.SelectedFile == "" && appUI.Editor != nil && appUI.Editor.ActiveDoc() == nil {
						appUI.SelectedFile = first.RelativePath
						_, _ = appUI.Editor.OpenFile(first.FullPath, first.RelativePath)
					}
					appUI.invalidateUI()
				})
			} else if appUI.Window != nil {
				appUI.Window.Invalidate()
			}
		}()
	}

	if appUI.Git != nil {
		// Render-adjacent reads go through Snapshot, never the live fields.
		gitSnap := appUI.Git.Snapshot()
		if gitSnap.Branch != "" {
			appUI.Workspace.GitBranch = gitSnap.Branch
			appUI.Workspace.GitDirty = len(gitSnap.Staged)+len(gitSnap.Unstaged)+len(gitSnap.Untracked) > 0
		}
	}

	if appUI.Workspace != nil && engine != nil {
		appUI.Workspace.RemoteFS = engine.RemoteFS
	}
	if appUI.Editor != nil && engine != nil {
		appUI.Editor.RemoteFS = engine.RemoteFS
	}
	appUI.LoadSavedSSHConfigs()
	if appUI.Workspace != nil && appUI.Workspace.RootDir != "" {
		appUI.recordOpenWorkspace(appUI.Workspace.RootDir)
	}

	if rootDir != "" && initGit != nil {
		go initGit.Refresh()
	}

	// Live elapsed indicators ("Working for 12s", spinners): repaint once a
	// second while any session is streaming. The same tick persists pending
	// UI preferences off the render path and runs AutoSave every ~30s.
	// Spawned LAST so the ticker goroutine never observes partially
	// constructed manager fields (Settings/Terminal/Editor are all set above).
	go func() {
		defer close(appUI.tickerDone)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		// Shimmer ticker: the ZCode-style running labels ("Working for 12s",
		// tool kind labels) pulse between muted and accent on a ~1.1s sine,
		// which only reads as motion if frames land faster than the 1s tick.
		// Invalidate at 150ms while a session streams; zero cost when idle.
		shimmer := time.NewTicker(150 * time.Millisecond)
		defer shimmer.Stop()
		for {
			select {
			case <-appUI.closeCh:
				return
			case <-shimmer.C:
				if appUI.Agent.HasRunningSession() && appUI.Window != nil {
					appUI.Window.Invalidate()
				}
			case <-ticker.C:
				// A panicking tick handler must be logged and the ticker must
				// keep ticking — not take the app down (P1-6).
				func() {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("[forge-ade] ticker tick panicked: %v\n%s", r, debug.Stack())
						}
					}()
					if (appUI.Agent.HasRunningSession() || appUI.ShowBgShellPopover ||
						(appUI.Mode == ModeStudio && appUI.StudioTimelinePlaying)) && appUI.Window != nil {
						appUI.Window.Invalidate()
					}
					// Live changes pill: poll git numstat every 3s while a turn
					// edits files so the floating "+A -D" count tracks the
					// agent's edits in (near) real time.
					if appUI.Agent.HasRunningSession() && appUI.Git != nil && time.Since(appUI.lastGitPollAt) >= 3*time.Second {
						appUI.lastGitPollAt = time.Now()
						appUI.Git.RefreshAsync()
					}
					if appUI.prefsDirty.CompareAndSwap(true, false) && !appUI.prefsSaving.Swap(true) {
						go func() {
							defer appUI.prefsSaving.Store(false)
							appUI.SaveUIPrefs()
						}()
					}
					appUI.syncTerminalShell()
					if appUI.Settings != nil && appUI.Settings.AutoSave && time.Since(appUI.lastAutoSaveAt) >= 30*time.Second {
						appUI.lastAutoSaveAt = time.Now()
						appUI.Editor.SaveAllDirty()
					}
				}()
			}
		}
	}()

	appUI.syncAntigravityModelsToACP()

	return appUI
}

func (a *AppUI) paletteCommands() []palette.PaletteCommand {
	commands := []palette.PaletteCommand{
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
		{ID: "agent-mode", Label: "Switch to Agent Mode", Shortcut: "⌘1", Run: func() { a.setMode(ModeAgent) }},
		{ID: "editor-mode", Label: "Switch to Editor Mode", Shortcut: "⌘2", Run: func() { a.setMode(ModeEditor) }},
		{ID: "studio-mode", Label: "Switch to Studio Mode", Detail: "Generative media & visual design environment", Shortcut: "⌘3", Run: func() { a.setMode(ModeStudio) }},
		{ID: "studio-export", Label: "Studio: Send to Editor", Detail: "Export active studio asset to editor workspace", Shortcut: "⌘⇧E", Run: func() { a.studioExportActive() }},
		{ID: "studio-pin", Label: "Studio: Toggle Element Pinning", Detail: "Direct manipulation element selection", Shortcut: "V", Run: func() { a.studioTogglePinning() }},
		{ID: "toggle-terminal", Label: "Toggle Terminal", Detail: "Bottom terminal drawer", Shortcut: "⌘`", Run: func() {
			a.toggleBottomTerminal()
		}},
		{ID: "terminal-snippets", Label: "Terminal Snippets & History", Detail: "Saved terminal commands and history", Shortcut: "⌥S", Run: func() {
			sessID := ""
			if ts := a.snippetDefaultTerminal(); ts != nil {
				sessID = ts.ActiveSessionID
			}
			a.OpenTerminalSnippetsModal(sessID, "", "")
		}},
		{ID: "toggle-sidebar", Label: "Toggle Sidebar", Shortcut: "⌘B", Run: func() { a.ShowLeftSidebar = !a.ShowLeftSidebar }},
		{ID: "toggle-drawer", Label: "Toggle Right Drawer", Shortcut: "⌘J", Run: func() { a.ShowRightDrawer = !a.ShowRightDrawer }},
		{ID: "save", Label: "Save File", Shortcut: "⌘S", Run: func() { _ = a.Editor.SaveActiveFile() }},
		{ID: "settings", Label: "Open Settings", Shortcut: "⌘,", Run: func() { a.openSettings() }},
		{ID: "appearance", Label: "Toggle Appearance", Detail: "Dark / light", Shortcut: "⌘/", Run: a.toggleAppearance},
		{ID: "theme", Label: "Pick Theme Palette", Run: func() { a.ThemeMgr.Toggle() }},
		{ID: "open-folder", Label: "Open Workspace Folder", Run: a.openWorkspaceFolder},
		{ID: "open-finder", Label: "Open in Finder", Detail: "Reveal workspace folder in Finder", Shortcut: "⌥⌘R", Run: func() {
			path := ""
			if a.Mode == ModeEditor && a.Editor != nil && a.Editor.ActiveDoc() != nil {
				path = a.Editor.ActiveDoc().FullPath
			}
			a.openInFinder(path)
		}},
		{ID: "open-external-terminal", Label: "Open External Terminal", Detail: "Launch workspace in external terminal emulator", Shortcut: "⌥⌘T", Run: func() {
			a.openExternalTerminal("")
		}},
		{ID: "switch-project", Label: "Switch Workspace / Project (Zed style)...", Detail: "Switch active window workspace or open recent project", Shortcut: "⌥⌘O", Run: func() {
			a.OpenProjectSwitcherModal()
		}},
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

	// User-defined custom slash commands (Settings → Slash Commands) are
	// runnable from the palette: selecting one seeds the composer with its
	// prompt template.
	if a.Settings != nil {
		for _, cc := range a.Settings.CustomCommands {
			name := cc.Name
			prompt := cc.Prompt
			commands = append(commands, palette.PaletteCommand{
				ID:     "custom-command-" + name,
				Label:  name,
				Detail: cc.Description,
				Run:    func() { a.runCustomCommand(prompt) },
			})
		}
	}
	return commands
}

// openUsageModal shows the usage modal and refreshes quota data
// asynchronously — never on the render path, where a network call would
// freeze the frame.
func (a *AppUI) openUsageModal() {
	a.ShowUsageModal = true
	a.refreshQuotaAsync()
}

// refreshQuotaAsync fetches quota/usage data off the render path. Results
// publish back through runOnUI so the shared map pointer is never written
// concurrently with a render reading it.
func (a *AppUI) refreshQuotaAsync() {
	if a.QuotaFetching || a.Engine == nil {
		return
	}
	a.QuotaFetching = true
	go func() {
		data := a.Engine.GetAgentQuota("agent-agy")
		a.runOnUI(func() {
			a.QuotaFetching = false
			if data != nil {
				a.QuotaData = data
			}
			a.syncAntigravityModelsToACP()
			a.invalidateUI()
		})
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
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, "git", "-C", root, "config", "--get", "remote.origin.url").Output()
		a.runOnUI(func() {
			a.gitRemoteLoading = false
			if err == nil && a.Workspace != nil && a.Workspace.RootDir == root {
				a.GitRemoteURL = strings.TrimSpace(string(out))
			}
			a.invalidateUI()
		})
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
		if a.Engine != nil {
			wsDir := ""
			if a.Workspace != nil {
				wsDir = a.Workspace.RootDir
			}
			a.Engine.StartMCPDaemon(wsDir, func() unsafe.Pointer {
				if a.Window == nil {
					return nil
				}
				h := a.Window.NativeHandle()
				return *(*unsafe.Pointer)(unsafe.Pointer(&h))
			})
		}
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

// SetVibrancySupport records whether the window carries an OS translucent
// material (mygo window Vibrancy). Only the Glass palette needs it: without
// the material its translucent chrome falls back to the opaque base palette.
func (a *AppUI) SetVibrancySupport(v bool) {
	a.ThemeMgr.SupportsVibrancy = v
	if a.Window != nil {
		a.Window.Invalidate()
	}
}

// View builds the entire native GPU frame matching ForgeADE layout.
func (a *AppUI) View(c *ui.Context) {
	defer recoverFrame("View")
	if (a.Mode == ModeAgent || a.Mode == ModeEditor || a.Mode == ModeStudio) && a.activeDrawerMode != a.Mode {
		a.syncModeDrawerState(a.Mode)
	}
	if a.ThemeMgr != nil {
		c.SetTheme(a.ThemeMgr.ActiveTheme())
	}
	// SetTheme re-paints the root with the theme's background, which would
	// hide the OS material behind the window. When semi-transparent glass is
	// active the root stays transparent so its translucent chrome shows the
	// blurred desktop (mygo vibrancy) and/or custom window background.
	if a.ThemeMgr != nil && a.ThemeMgr.IsGlassActive() && c.Vibrancy() {
		c.Root().Background(ui.Transparent)
	}
	a.renderWindowBackground(c)
	a.drainToasts(c)

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
		// Peek definition runs the workspace symbol scan off the render path —
		// FindDefinitions can take seconds on large codebases.
		doc := a.Editor.ActiveDoc()
		if doc != nil {
			sym := a.symbolUnderCursor(doc)
			if sym != "" {
				wsDir := a.Workspace.RootDir
				go func() {
					matches := a.Editor.FindDefinitions(sym, wsDir)
					a.runOnUI(func() {
						if len(matches) == 1 {
							a.openSearchResult(matches[0].Path, matches[0].Line, "")
						} else if len(matches) > 1 {
							a.ShowPeekModal = true
							a.PeekSymbol = sym
							a.PeekResults = matches
						}
						a.invalidateUI()
					})
				}()
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
		if a.Mode == ModeStudio {
			a.studioExportActive()
		} else {
			a.Mode = ModeEditor
			a.ShowLeftSidebar = true
			a.EditorActivity = ActivityExplorer
		}
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
	if cmdOrCtrlShift(ui.KeyL) {
		var urls []string
		if ts := a.displayedTerminal(); ts != nil {
			urls = ts.VisibleURLs()
		}
		a.OpenTerminalURLModal(urls)
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
	// ⌘O stays unbound (user request): it used to open the folder picker and
	// fired accidentally while typing. Open-folder remains available through
	// the command palette ("Open Workspace Folder") and the sidebar header.
	if cmdOrCtrl(ui.Key1) {
		a.setMode(ModeAgent)
	}
	if cmdOrCtrl(ui.Key2) {
		a.setMode(ModeEditor)
	}
	if cmdOrCtrl(ui.Key3) {
		a.setMode(ModeStudio)
	}
	if a.Mode == ModeStudio && c.Shortcut(0, ui.KeyV) {
		a.studioTogglePinning()
	}
	if cmdOrCtrl(ui.KeyJ) {
		a.ShowRightDrawer = !a.ShowRightDrawer
	}
	if cmdOrCtrl(ui.KeySlash) {
		a.toggleAppearance()
	}
	if cmdOrCtrl(ui.KeyBackquote) {
		a.toggleBottomTerminal()
	}
	if c.Shortcut(ui.Cmd|ui.Alt, ui.KeyO) {
		a.OpenProjectSwitcherModal()
	}
	if c.Shortcut(ui.Alt, ui.KeyS) {
		sessID := ""
		if ts := a.snippetDefaultTerminal(); ts != nil {
			sessID = ts.ActiveSessionID
		}
		a.OpenTerminalSnippetsModal(sessID, "", "")
	}
	if c.Shortcut(ui.Cmd|ui.Alt, ui.KeyR) || c.Shortcut(ui.Ctrl|ui.Alt, ui.KeyR) {
		path := ""
		if a.Mode == ModeEditor && a.Editor != nil && a.Editor.ActiveDoc() != nil {
			path = a.Editor.ActiveDoc().FullPath
		}
		a.openInFinder(path)
	}
	if c.Shortcut(ui.Cmd|ui.Alt, ui.KeyT) || c.Shortcut(ui.Ctrl|ui.Alt, ui.KeyT) || cmdOrCtrlShift(ui.KeyC) {
		a.openExternalTerminal("")
	}
	if c.Shortcut(ui.Ctrl, ui.KeyTab) {
		a.Editor.CycleTab(1)
	}
	if c.Shortcut(ui.Ctrl|ui.Shift, ui.KeyTab) {
		a.Editor.CycleTab(-1)
	}
	if cmdOrCtrl(ui.KeyW) {
		if a.Mode == ModeEditor && a.Editor.ActivePath != "" {
			// RequestClose asks before discarding unsaved changes.
			a.Editor.RequestClose(a.Editor.ActivePath)
		}
	}

	// Root layout: in Agent Mode, anchor the Orca-style full-width bottom
	// quota/status strip cleanly below BOTH the Left Sidebar and Main workspace.
	if a.Mode == ModeAgent {
		ui.Column(c).Fill().Children(func() {
			ui.Box(c).Grow(1).FillWidth().Clip().Children(func() {
				if a.ShowLeftSidebar {
					ui.Split(c, &a.LeftSidebarWidth, func() {
						a.renderAgentSidebar(c)
					}, func() {
						a.renderMainColumn(c)
					}).SplitReset(LeftSidebarDefault).Fill()
				} else {
					a.renderMainColumn(c)
				}
			})
			a.renderAgentBottomBar(c)
		})
	} else if a.Mode == ModeStudio && a.ShowLeftSidebar {
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

	// Cheap change detection stays on the render path; the disk write happens
	// on the ticker goroutine (prefsDirty) so View never does I/O.
	if time.Since(a.lastPrefsAt) >= time.Second {
		a.lastPrefsAt = time.Now()
		if snap := a.prefsSnapshot(); snap != a.lastPrefsSnap {
			a.lastPrefsSnap = snap
			a.prefsDirty.Store(true)
		}
	}
}

type dirExistsEntry struct {
	exists    bool
	checkedAt time.Time
}

// isExistingDirCached avoids per-frame os.Stat syscalls in the render loop
// (sidebar project grouping / project selector) using a short 10s TTL cache.
func (a *AppUI) isExistingDirCached(path string) bool {
	if path == "" {
		return false
	}
	if a.dirExistsCache == nil {
		a.dirExistsCache = make(map[string]dirExistsEntry)
	}
	if ent, ok := a.dirExistsCache[path]; ok && time.Since(ent.checkedAt) < 10*time.Second {
		return ent.exists
	}
	info, err := os.Stat(path)
	ok := err == nil && info.IsDir()
	a.dirExistsCache[path] = dirExistsEntry{exists: ok, checkedAt: time.Now()}
	return ok
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
	if a.prefsDirty.Swap(false) {
		a.SaveUIPrefs()
	}
	if a.Engine != nil {
		a.Engine.Close()
	}
	if a.CLIMgr != nil {
		a.CLIMgr.StopAll()
	}
	if a.Engine != nil && a.Engine.ShellMgr != nil {
		a.Engine.ShellMgr.StopAll()
	}
	// Close EVERY per-workspace terminal, not just the active one: switching
	// workspaces parks previous TerminalStates in this map and their PTY
	// children used to outlive the app (P1-4). a.Terminal is one of the map
	// entries; TerminalState.Close is idempotent.
	if a.WorkspaceTerminals != nil {
		for key, ts := range a.WorkspaceTerminals {
			if ts != nil {
				ts.Close()
			}
			delete(a.WorkspaceTerminals, key)
		}
	}
	if a.Terminal != nil {
		a.Terminal.Close()
	}
	if a.EditorBottomWorkspaceTerminals != nil {
		for key, ts := range a.EditorBottomWorkspaceTerminals {
			if ts != nil {
				ts.Close()
			}
			delete(a.EditorBottomWorkspaceTerminals, key)
		}
	}
	if a.EditorBottomTerminal != nil {
		a.EditorBottomTerminal.Close()
	}
	if a.AgentWorkspaceTerminals != nil {
		for key, ts := range a.AgentWorkspaceTerminals {
			if ts != nil {
				ts.Close()
			}
			delete(a.AgentWorkspaceTerminals, key)
		}
	}
	if a.AgentTerminal != nil {
		a.AgentTerminal.Close()
	}
	// Kill ACP CLI children (spawned with Setpgid:true): Manager.Close
	// SIGTERM→SIGKILLs each process group so agent harnesses never outlive
	// the app (P1-4).
	if a.Engine != nil && a.Engine.ACPMgr != nil {
		a.Engine.ACPMgr.Close()
	}
	if a.ACPMgr != nil {
		a.ACPMgr.Close()
	}
}

func (a *AppUI) toggleAppearance() {
	if a.ThemeMgr.Appearance == "dark" {
		a.ThemeMgr.Appearance = "light"
	} else {
		a.ThemeMgr.Appearance = "dark"
	}
}

// resolveBackgroundImage returns the decoded background image for path using a
// short TTL stat cache so rendering never hits the filesystem every frame.
func (a *AppUI) resolveBackgroundImage(path string) *cachedImage {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	if a.bgImagePath == path && a.bgImageCached != nil &&
		(a.bgImageCached.src != nil || a.bgImageCached.err != nil) &&
		time.Since(a.bgImageCheckedAt) < 2*time.Second {
		return a.bgImageCached
	}
	ci := a.getImage(path)
	if ci != nil && !ci.loading {
		a.bgImagePath = path
		a.bgImageCheckedAt = time.Now()
		a.bgImageCached = ci
	}
	return ci
}

// pickBackgroundImageFile opens a native file dialog to pick a custom window
// background image and enables semi-transparent glass so it shows through.
func (a *AppUI) pickBackgroundImageFile() {
	if a.Window == nil || a.ThemeMgr == nil {
		return
	}
	win := a.Window
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: win,
			Title:  "Choose Background Image",
			Filters: []mygo.FileFilter{
				{
					Name:       "Images",
					Extensions: []string{"png", "jpg", "jpeg", "webp", "gif", "bmp", "tiff", "tif", "avif", "svg"},
					MIMETypes:  []string{"image/*"},
				},
			},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		a.runOnUI(func() {
			a.ThemeMgr.BackgroundKind = theme.BackgroundKindImage
			a.ThemeMgr.BackgroundImage = paths[0]
			if !a.ThemeMgr.IsGlassActive() {
				a.ThemeMgr.GlassEnabled = true
			}
			a.bgImagePath = ""
			a.bgImageCached = nil
			a.SaveUIPrefs()
			a.invalidateUI()
		})
	}()
}

// renderWindowBackground draws the configured ambient gradient preset or custom
// image behind all workspace chrome.
func (a *AppUI) glassBlurActive() bool {
	return a != nil && a.ThemeMgr != nil && a.ThemeMgr.IsGlassActive() && a.ThemeMgr.HasBackground()
}

func (a *AppUI) renderWindowBackground(c *ui.Context) {
	if a.ThemeMgr == nil || !a.ThemeMgr.HasBackground() {
		return
	}
	tm := a.ThemeMgr
	dark := tm.IsDark()
	opacity := tm.EffectiveBackgroundOpacity()
	blur := tm.EffectiveBackgroundBlur()
	pad := blur * 3

	backdrop := ui.Box(c.Key("window-bg-backdrop")).Absolute().Top(0).Left(0).Right(0).Bottom(0).
		ZIndex(-1).IgnoreInput(true).PassThrough().Clip()
	if !tm.SupportsVibrancy || !tm.IsGlassActive() {
		// Provide an opaque theme base behind partially transparent backgrounds
		// when OS vibrancy is not underneath.
		base := theme.BasePaletteColors(tm.PaletteID, dark)
		backdrop.Background(base.Canvas)
	}

	backdrop.Children(func() {
		switch tm.BackgroundKind {
		case theme.BackgroundKindPreset:
			preset, ok := theme.LookupBackgroundPreset(tm.BackgroundPreset)
			if !ok {
				return
			}
			stops := preset.DarkStops
			glowA, glowB := preset.DarkGlowA, preset.DarkGlowB
			if !dark {
				stops = preset.LightStops
				glowA, glowB = preset.LightGlowA, preset.LightGlowB
			}
			layer := ui.Box(c.Key("window-bg-preset-" + preset.ID)).Absolute().
				Top(-pad).Left(-pad).Right(-pad).Bottom(-pad).
				LinearGradient(ui.LinearGradient{
					Angle: preset.Angle,
					Stops: stops,
					Oklab: true,
				}).Opacity(opacity)
			if blur > 0 {
				layer.Blur(blur)
			}
			layer.Children(func() {
				ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).
					RadialGradient(ui.RadialGradient{
						Center: ui.Point{X: 0.2, Y: 0.18},
						Stops: []ui.GradientStop{
							{Color: glowA, At: 0},
							{Color: glowA.Alpha(0), At: 0.7},
						},
						Oklab: true,
					})
				ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).
					RadialGradient(ui.RadialGradient{
						Center: ui.Point{X: 0.82, Y: 0.8},
						Stops: []ui.GradientStop{
							{Color: glowB, At: 0},
							{Color: glowB.Alpha(0), At: 0.75},
						},
						Oklab: true,
					})
			})
		case theme.BackgroundKindImage:
			ci := a.resolveBackgroundImage(tm.BackgroundImage)
			if ci == nil || ci.src == nil {
				return
			}
			img := ui.Image(c.Key("window-bg-img"), ci.src).Absolute().
				Top(-pad).Left(-pad).Right(-pad).Bottom(-pad).
				Fit(ui.Cover).Opacity(opacity)
			if blur > 0 {
				img.Blur(blur)
			}
		}
	})
}

// confirmDialog stages the reusable confirmation dialog. Escape and outside
// click cancel it (renderConfirmDialog clears the state); onConfirm runs only
// when the user accepts. Call from the UI thread; from goroutines wrap in
// runOnUI.
func (a *AppUI) confirmDialog(title, description string, onConfirm func()) {
	a.confirmDialogLabeled(title, description, "Confirm", onConfirm, nil)
}

// confirmDialogLabeled is confirmDialog with a destructive-action button
// label ("Delete", "Discard", "Sync") and an optional onCancel side effect
// (the dialog itself always closes either way).
func (a *AppUI) confirmDialogLabeled(title, description, confirmLabel string, onConfirm, onCancel func()) {
	a.ShowConfirmDialog = true
	a.ConfirmDialogTitle = title
	a.ConfirmDialogDesc = description
	a.ConfirmDialogLabel = confirmLabel
	a.confirmOnAccept = onConfirm
	a.confirmOnCancel = onCancel
}

// acceptConfirmDialog runs the staged confirm action and closes the dialog.
func (a *AppUI) acceptConfirmDialog() {
	action := a.confirmOnAccept
	a.clearConfirmDialog()
	if action != nil {
		action()
	}
}

// cancelConfirmDialog closes the dialog without running the staged action.
func (a *AppUI) cancelConfirmDialog() {
	onCancel := a.confirmOnCancel
	a.clearConfirmDialog()
	if onCancel != nil {
		onCancel()
	}
}

func (a *AppUI) clearConfirmDialog() {
	a.ShowConfirmDialog = false
	a.ConfirmDialogTitle = ""
	a.ConfirmDialogDesc = ""
	a.ConfirmDialogLabel = ""
	a.confirmOnAccept = nil
	a.confirmOnCancel = nil
}

// syncTerminalShell propagates the configured DefaultShell to the terminal
// spawner. An invalid shell toasts once and falls back to the user's login
// shell.
func (a *AppUI) syncTerminalShell() {
	if a.Settings == nil {
		return
	}
	shell := strings.TrimSpace(a.Settings.DefaultShell)
	if shell != "" {
		if _, err := exec.LookPath(shell); err != nil {
			if a.shellWarned.CompareAndSwap(false, true) {
				a.queueToast(ui.Toast{
					Title:       "Default Shell Unavailable",
					Description: shell + " was not found on PATH — using the system default shell",
					Type:        "error",
				})
			}
			shell = ""
		}
	}
	if a.Terminal != nil {
		a.Terminal.SetDefaultShell(shell)
	}
	if a.EditorBottomTerminal != nil {
		a.EditorBottomTerminal.SetDefaultShell(shell)
	}
	if a.AgentTerminal != nil {
		a.AgentTerminal.SetDefaultShell(shell)
	}
	for _, ts := range a.WorkspaceTerminals {
		if ts != nil {
			ts.SetDefaultShell(shell)
		}
	}
	for _, ts := range a.EditorBottomWorkspaceTerminals {
		if ts != nil {
			ts.SetDefaultShell(shell)
		}
	}
	for _, ts := range a.AgentWorkspaceTerminals {
		if ts != nil {
			ts.SetDefaultShell(shell)
		}
	}
}

// runCustomCommand seeds the agent composer with a custom slash command's
// prompt template (Settings → Slash Commands, surfaced in the ⌘K palette).
func (a *AppUI) runCustomCommand(prompt string) {
	if strings.TrimSpace(prompt) == "" {
		return
	}
	a.Mode = ModeAgent
	a.AgentViewMode = AgentViewChat
	a.ActiveTaskKind = "agent"
	if a.Agent != nil {
		a.Agent.InputText = prompt
	}
	a.invalidateUI()
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

// handleNewStudioTask clears the active chat session while staying in Studio Mode
// so the user can author a brand-new slide deck, motion composition, or 3D scene.
func (a *AppUI) handleNewStudioTask() {
	if a.Agent != nil {
		a.Agent.ClearActiveSession()
		if a.Agent.ActiveAgentID != "" && a.Agent.ActiveAgentID != "agent-internal" {
			a.Agent.ActiveModel = a.Agent.ResolveModelForAgent(a.Agent.ActiveAgentID, a.Agent.ActiveModel)
			a.SelectedAgentModel = a.Agent.ActiveModel
		}
	}
	a.ActiveTaskKind = "agent"
	a.Mode = ModeStudio
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
	a.clearRemoteUploadQueue()
	a.SSHUploadStatus = ""
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
	if dir == "" || dir == "/" {
		return
	}
	norm := dir
	if !strings.HasPrefix(norm, "ssh://") {
		norm = agentsession.NormalizeWorkspacePath(dir)
	}
	if norm == "" || norm == "/" || agentsession.IsTempTestPath(norm) {
		return
	}
	rest := make([]string, 0, len(a.RecentWorkspaces))
	for _, p := range a.RecentWorkspaces {
		normP := p
		if !strings.HasPrefix(normP, "ssh://") {
			normP = agentsession.NormalizeWorkspacePath(p)
		}
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

// recordOpenWorkspace registers dir as an active workspace in the current window.
func (a *AppUI) recordOpenWorkspace(dir string) {
	if dir == "" || dir == "/" {
		return
	}
	norm := dir
	if !strings.HasPrefix(norm, "ssh://") {
		norm = agentsession.NormalizeWorkspacePath(dir)
	}
	for _, ws := range a.OpenWorkspaces {
		if sameWorkspacePath(ws, norm) {
			return
		}
	}
	a.OpenWorkspaces = append(a.OpenWorkspaces, norm)
}

// closeWorkspaceFromWindow removes ws from the current window's active group
// while preserving it in RecentWorkspaces history (Zed-style project switcher).
func (a *AppUI) closeWorkspaceFromWindow(ws string) {
	normTarget := normalizeWorkspaceKey(ws)
	var remaining []string
	for _, open := range a.OpenWorkspaces {
		if normalizeWorkspaceKey(open) != normTarget {
			remaining = append(remaining, open)
		}
	}
	a.OpenWorkspaces = remaining

	// Terminate running terminal processes for this closed workspace (all modes/docks)
	if ts, ok := a.WorkspaceTerminals[normTarget]; ok && ts != nil {
		ts.Close()
		delete(a.WorkspaceTerminals, normTarget)
	}
	if ts, ok := a.EditorBottomWorkspaceTerminals[normTarget]; ok && ts != nil {
		ts.Close()
		delete(a.EditorBottomWorkspaceTerminals, normTarget)
	}
	if ts, ok := a.AgentWorkspaceTerminals[normTarget]; ok && ts != nil {
		ts.Close()
		delete(a.AgentWorkspaceTerminals, normTarget)
	}
	delete(a.WorkspaceEditorSnapshots, normTarget)

	cur := ""
	if a.Workspace != nil {
		cur = a.Workspace.RootDir
	}
	if normalizeWorkspaceKey(cur) == normTarget {
		if len(remaining) > 0 {
			a.switchWorkspace(remaining[0])
		} else {
			a.switchWorkspace("")
		}
	}
	if a.Window != nil {
		a.Window.Invalidate()
	}
}

// OpenProjectSwitcherModal opens the Zed-style project switcher palette.
func (a *AppUI) OpenProjectSwitcherModal() {
	a.ShowProjectSwitcherModal = true
	a.ProjectSwitcherQuery = ""
	a.projectSwitcherSelected = 0
	if a.Window != nil {
		a.Window.Invalidate()
	}
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
	full := relPath
	if a.Workspace != nil {
		full = a.Workspace.ResolveFullPath(relPath)
	}
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
		if err := validatePreviewableFile(full); err != nil {
			a.queueToast(ui.Toast{Title: "Cannot open file", Description: err.Error(), Type: "error"})
			return
		}
		if doc := a.Editor.GetDoc(full); doc == nil {
			if _, err := a.Editor.OpenFile(full, relPath); err != nil {
				a.queueToast(ui.Toast{Title: "Open failed", Description: err.Error(), Type: "error"})
				return
			}
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
	// Drop the deleted project from the persisted sidebar order too — stale
	// entries otherwise re-claim a slot if the folder is ever recreated.
	prunedOrder := make([]string, 0, len(a.ProjectOrder))
	for _, n := range a.ProjectOrder {
		if n == groupName || (normFolder != "" && n == normFolder) {
			continue
		}
		prunedOrder = append(prunedOrder, n)
	}
	a.ProjectOrder = prunedOrder
	a.prefsDirty.Store(true)
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
		// switchWorkspace mutates the whole state tree — it must run on the
		// UI thread, not inside this dialog goroutine.
		a.runOnUI(func() {
			a.switchWorkspace(paths[0])
		})
	}()
}

// resolveSSHDisplayHost returns the friendly host name/IP for a connection ID.
func (a *AppUI) resolveSSHDisplayHost(connID string) string {
	for _, cfg := range a.SSHSavedConfigs {
		if cfg.ID == connID {
			if cfg.Host != "" {
				return cfg.Host
			}
			if cfg.Label != "" {
				return cfg.Label
			}
		}
	}
	return connID
}

func normalizeWorkspaceKey(dir string) string {
	if dir == "" || dir == "/" {
		return ""
	}
	if strings.HasPrefix(dir, "ssh://") {
		return dir
	}
	return agentsession.NormalizeWorkspacePath(dir)
}

func sameWorkspacePath(aDir, bDir string) bool {
	return normalizeWorkspaceKey(aDir) == normalizeWorkspaceKey(bDir)
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
	a.ensureAgentTab(s.ID)
	if a.Agent != nil {
		a.Agent.SelectSession(s.ID)
		if a.Mode == ModeStudio {
			a.studioImportAgentArtifacts(s.ID)
		}
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

// activeTerminal returns the independent TerminalState belonging to the current
// mode's dock/drawer (AgentTerminal in ModeAgent, EditorBottomTerminal in ModeEditor).
// Editor main-pane terminal tabs ("terminal://...") use a.Terminal separately.
func (a *AppUI) activeTerminal() *terminal.TerminalState {
	if a.Mode == ModeAgent && a.AgentTerminal != nil {
		return a.AgentTerminal
	}
	if a.EditorBottomTerminal != nil {
		return a.EditorBottomTerminal
	}
	return a.Terminal
}

// snippetDefaultTerminal returns the terminal that keyboard/toolbar snippet
// actions should target: the main-pane terminal when a "terminal://" tab is
// active in Editor Mode, otherwise the mode's active dock/drawer terminal.
func (a *AppUI) snippetDefaultTerminal() *terminal.TerminalState {
	if a.Mode == ModeEditor && a.Editor != nil && strings.HasPrefix(a.Editor.ActivePath, "terminal://") && a.Terminal != nil {
		return a.Terminal
	}
	return a.activeTerminal()
}

// displayedTerminal returns the terminal currently visible to the user across
// Agent CLI terminal surfaces, Editor main-pane terminal:// tabs, and the
// active mode's bottom/drawer terminal.
func (a *AppUI) displayedTerminal() *terminal.TerminalState {
	if a.Mode == ModeAgent && a.ActiveTaskKind == "cli" && a.CLIMgr != nil {
		if sess := a.CLIMgr.ActiveSession(); sess != nil && sess.Terminal != nil {
			return sess.Terminal
		}
	}
	if a.Mode == ModeEditor && a.Editor != nil && strings.HasPrefix(a.Editor.ActivePath, "terminal://") && a.Terminal != nil {
		return a.Terminal
	}
	return a.activeTerminal()
}

func (a *AppUI) isBottomTerminalVisible() bool {
	if a.Mode == ModeAgent {
		return a.ShowAgentBottomTerminal
	}
	return a.ShowBottomTerminal
}

func (a *AppUI) setBottomTerminalVisible(visible bool) {
	if a.Mode == ModeAgent {
		a.ShowAgentBottomTerminal = visible
		return
	}
	a.ShowBottomTerminal = visible
}

func (a *AppUI) toggleBottomTerminal() {
	a.setBottomTerminalVisible(!a.isBottomTerminalVisible())
}

// syncModeDrawerState swaps the right-drawer visibility and tab list when
// transitioning between ModeAgent, ModeEditor, and ModeStudio so each mode keeps
// its own independent drawer panes and terminal tab bindings.
func (a *AppUI) syncModeDrawerState(targetMode int) {
	if targetMode != ModeAgent && targetMode != ModeEditor && targetMode != ModeStudio {
		return
	}
	if a.activeDrawerMode == targetMode {
		return
	}
	cur := ModeDrawerState{
		Show:   a.ShowRightDrawer,
		Tab:    a.RightDrawerTab,
		Tabs:   a.DrawerTabs,
		Active: a.drawerActiveIdx,
	}
	if a.activeDrawerMode == ModeEditor {
		a.editorDrawerState = cur
	} else if a.activeDrawerMode == ModeStudio {
		a.studioDrawerState = cur
	} else {
		a.agentDrawerState = cur
	}

	var next ModeDrawerState
	if targetMode == ModeEditor {
		next = a.editorDrawerState
	} else if targetMode == ModeStudio {
		next = a.studioDrawerState
		if len(next.Tabs) == 0 {
			next.Tab = RightTabArtifacts
		}
	} else {
		next = a.agentDrawerState
	}
	a.ShowRightDrawer = next.Show
	a.RightDrawerTab = next.Tab
	a.DrawerTabs = next.Tabs
	a.drawerActiveIdx = next.Active
	a.activeDrawerMode = targetMode
}

func (a *AppUI) newWorkspaceTerminalState(rawDir string) *terminal.TerminalState {
	termDir := rawDir
	if termDir == "" {
		home, _ := os.UserHomeDir()
		termDir = home
	}
	ts := terminal.NewTerminalStateDeferred(termDir)
	if a.Settings != nil && strings.TrimSpace(a.Settings.DefaultShell) != "" {
		ts.SetDefaultShell(strings.TrimSpace(a.Settings.DefaultShell))
	}
	if strings.HasPrefix(rawDir, "ssh://") {
		ts.Cwd = rawDir
		if a.Engine != nil && a.Engine.RemoteFS != nil {
			if client, remotePath, ok := a.Engine.RemoteFS.ParseRemotePath(rawDir); ok && client != nil {
				prog, args, env := client.SSHCommand(remotePath)
				ts.Command = append([]string{prog}, args...)
				ts.Env = env
				_, remotePath, _, folderName := parseSSHWorkspaceURI(rawDir)
				if folderName == "" || folderName == "remote-root" || folderName == "." || folderName == "/" {
					folderName = filepath.Base(remotePath)
				}
				if folderName == "" || folderName == "." || folderName == "/" {
					folderName = "remote"
				}
				ts.Title = folderName
			}
		}
	} else {
		ts.Title = filepath.Base(termDir)
	}
	return ts
}

// setMode switches between ModeAgent, ModeEditor, and ModeSettings while
// preserving independent active workspaces and drawer/terminal state for Agent
// Mode and Editor Mode.
func (a *AppUI) setMode(mode int) {
	if a.Mode == mode {
		return
	}
	prevMode := a.Mode
	curWs := ""
	if a.Workspace != nil {
		curWs = a.Workspace.RootDir
	}
	if prevMode == ModeAgent {
		a.AgentWorkspacePath = curWs
	} else if prevMode == ModeEditor {
		a.EditorWorkspacePath = curWs
	} else if prevMode == ModeStudio {
		a.StudioWorkspacePath = curWs
	}

	// Clean up active webviews from previous mode to guarantee zero idle background processes
	if prevMode == ModeStudio && mode != ModeStudio {
		a.closeStudioWebView()
		a.closeStudioNodeWebView()
	}
	if prevMode == ModeAgent && mode != ModeAgent {
		a.closeDeckViewer()
	}
	a.syncModeDrawerState(mode)
	a.Mode = mode
	if mode == ModeAgent {
		if normalizeWorkspaceKey(a.AgentWorkspacePath) != normalizeWorkspaceKey(curWs) {
			a.switchWorkspaceForMode(a.AgentWorkspacePath, true, prevMode)
		}
	} else if mode == ModeEditor {
		if normalizeWorkspaceKey(a.EditorWorkspacePath) != normalizeWorkspaceKey(curWs) {
			a.switchWorkspaceForMode(a.EditorWorkspacePath, true, prevMode)
		}
	} else if mode == ModeStudio {
		if normalizeWorkspaceKey(a.StudioWorkspacePath) != normalizeWorkspaceKey(curWs) {
			a.switchWorkspaceForMode(a.StudioWorkspacePath, true, prevMode)
		}
		if a.Studio != nil && a.Workspace != nil {
			a.Studio.SetWorkspace(a.Workspace.RootDir)
		}
		a.ShowLeftSidebar = false
	}
	if a.Window != nil {
		a.Window.Invalidate()
	}
}

func (a *AppUI) switchWorkspace(dir string) {
	a.switchWorkspaceInternal(dir, false)
}

func (a *AppUI) switchWorkspaceInternal(dir string, keepActiveSession bool) {
	a.switchWorkspaceForMode(dir, keepActiveSession, a.Mode)
}

func (a *AppUI) switchWorkspaceForMode(dir string, keepActiveSession bool, fromMode int) {
	rawDir := dir
	if !strings.HasPrefix(rawDir, "ssh://") {
		rawDir = agentsession.NormalizeWorkspacePath(dir)
	}

	prevWs := ""
	if a.Workspace != nil {
		prevWs = a.Workspace.RootDir
	}
	prevKey := normalizeWorkspaceKey(prevWs)
	newKey := normalizeWorkspaceKey(rawDir)

	if prevWs != "" && prevKey == newKey {
		if a.Window != nil {
			a.Window.Invalidate()
		}
		return
	}

	// 1. Stash outgoing workspace state (mode-isolated terminals and editor tabs)
	if a.WorkspaceTerminals == nil {
		a.WorkspaceTerminals = make(map[string]*terminal.TerminalState)
	}
	if a.EditorBottomWorkspaceTerminals == nil {
		a.EditorBottomWorkspaceTerminals = make(map[string]*terminal.TerminalState)
	}
	if a.AgentWorkspaceTerminals == nil {
		a.AgentWorkspaceTerminals = make(map[string]*terminal.TerminalState)
	}
	if a.WorkspaceEditorSnapshots == nil {
		a.WorkspaceEditorSnapshots = make(map[string]*editor.WorkspaceEditorSnapshot)
	}
	if prevWs != "" {
		if fromMode == ModeEditor {
			if a.Terminal != nil {
				a.WorkspaceTerminals[prevKey] = a.Terminal
			}
			if a.EditorBottomTerminal != nil {
				a.EditorBottomWorkspaceTerminals[prevKey] = a.EditorBottomTerminal
			}
			if a.Editor != nil {
				a.WorkspaceEditorSnapshots[prevKey] = a.Editor.TakeSnapshot()
			}
		} else {
			if a.AgentTerminal != nil {
				a.AgentWorkspaceTerminals[prevKey] = a.AgentTerminal
			}
		}
	}

	// 2. Set up incoming workspace
	a.Workspace = workspace.NewWorkspaceState(rawDir)
	a.recordRecentWorkspace(rawDir)
	if rawDir != "" && rawDir != "/" {
		a.recordOpenWorkspace(rawDir)
	}
	if a.Engine != nil {
		a.Workspace.RemoteFS = a.Engine.RemoteFS
		if strings.HasPrefix(rawDir, "ssh://") {
			connID, _, _, _ := parseSSHWorkspaceURI(rawDir)
			if !a.Engine.IsSSHConnected(connID) {
				for _, cfg := range a.SSHSavedConfigs {
					if cfg.ID == connID {
						cfgCopy := cfg
						go func() {
							_, _ = a.Engine.RemoteFS.Connect(cfgCopy)
							if a.Workspace != nil && a.Workspace.RootDir == rawDir {
								a.Workspace.Refresh()
							}
							if a.Window != nil {
								a.Window.Invalidate()
							}
						}()
						break
					}
				}
			} else {
				go func() {
					if a.Workspace != nil && a.Workspace.RootDir == rawDir {
						a.Workspace.Refresh()
					}
					if a.Window != nil {
						a.Window.Invalidate()
					}
				}()
			}
		}
	}

	// 3. Restore or initialize editor state for new workspace (Editor Mode only)
	if a.Mode == ModeEditor && a.Editor != nil {
		a.Editor.WorkspaceDir = rawDir
		if a.Engine != nil {
			a.Editor.RemoteFS = a.Engine.RemoteFS
		}
		if snap, ok := a.WorkspaceEditorSnapshots[newKey]; ok && snap != nil {
			a.Editor.RestoreSnapshot(snap)
		} else {
			a.Editor.CloseAllForWorkspace(rawDir)
		}
	}

	// 4. Update mode-specific active workspace & agent workspace
	if a.Mode == ModeEditor {
		a.EditorWorkspacePath = rawDir
	} else if a.Mode == ModeStudio {
		a.StudioWorkspacePath = rawDir
		if a.Studio != nil {
			a.Studio.SetWorkspace(rawDir)
		}
	} else {
		a.AgentWorkspacePath = rawDir
		if a.Agent != nil {
			if !keepActiveSession {
				if sess := a.Agent.ActiveSession(); sess != nil && agentsession.NormalizeWorkspacePath(sess.WorkspacePath) != rawDir {
					a.Agent.ClearActiveSession()
				}
			}
			a.Agent.SetWorkspacePath(rawDir)
		}
	}

	// 5. Restore or initialize mode-specific terminal state for new workspace
	if a.Mode == ModeEditor {
		if existingTerm, ok := a.WorkspaceTerminals[newKey]; ok && existingTerm != nil {
			a.Terminal = existingTerm
		} else {
			ts := a.newWorkspaceTerminalState(rawDir)
			a.WorkspaceTerminals[newKey] = ts
			a.Terminal = ts
		}
		if existingBottom, ok := a.EditorBottomWorkspaceTerminals[newKey]; ok && existingBottom != nil {
			a.EditorBottomTerminal = existingBottom
		} else {
			ts := a.newWorkspaceTerminalState(rawDir)
			a.EditorBottomWorkspaceTerminals[newKey] = ts
			a.EditorBottomTerminal = ts
		}
	} else {
		if existingTerm, ok := a.AgentWorkspaceTerminals[newKey]; ok && existingTerm != nil {
			a.AgentTerminal = existingTerm
		} else {
			ts := a.newWorkspaceTerminalState(rawDir)
			a.AgentWorkspaceTerminals[newKey] = ts
			a.AgentTerminal = ts
		}
	}
	// Drop state bound to the old workspace so stale relative paths can never
	// render the wrong files against the new root.
	a.SelectedFile = ""
	a.SelectedDiffPath = ""
	a.SideFileTabs = nil
	a.ActiveSideFile = ""
	a.SideFilePreviewImage = ""
	a.SideFilePreviewMd = ""
	a.SideFileDiffMode = make(map[string]bool)
	a.FileSearchQuery = ""
	a.FileSearchHits = nil
	a.FileSearchBusy = false
	a.FileSearchReplaceOpen = false
	a.ContentHits = nil
	a.ContentHitsAt = ""
	a.SearchStatusMsg = ""
	a.ShowProjectSearch = false
	a.DeckFiles = nil
	a.DeckFilesScanned = false
	a.GitRemoteURL = ""
	a.gitRemoteRoot = ""
	a.gitRemoteLoading = false
	a.RuleFilesRoot = ""
	a.RuleFilesCache = nil
	a.PDFFileSize = 0
	a.PDFFileSizePath = ""
	a.closePdfViewer()
	a.closeDeckViewer()
	a.ShowThreadFind = false
	a.ThreadFindQuery = ""
	a.ThreadFindMatches = nil

	// Re-point the living-memory brain at the new workspace (per-project
	// facts/digests/notes). SSH and empty workspaces have no local brain.
	if a.Engine != nil && a.Engine.AgentMgr != nil {
		if rawDir != "" && !strings.HasPrefix(rawDir, "ssh://") {
			if br, err := brain.Open(rawDir); err == nil {
				a.Engine.AgentMgr.SetBrain(br)
			} else {
				log.Printf("[app] brain open %s: %v", rawDir, err)
			}
		} else {
			a.Engine.AgentMgr.SetBrain(nil)
		}
	}

	if strings.HasPrefix(rawDir, "ssh://") {
		// Stop any active local filesystem watcher to prevent background CPU/RAM churn and descriptor leaks
		if a.Engine != nil {
			a.Engine.StopWatcher()
		}
		if a.Git != nil {
			a.Git = git.NewGitManagerLazy(nil, "", a.Git.OnFileSelect)
		}
		if a.Database != nil {
			a.Database.Workspace = a.Workspace
		}
		if a.Editor != nil && len(a.Editor.OpenTabPaths) == 0 {
			if first, ok := a.Workspace.FirstFile(); ok {
				a.SelectedFile = first.RelativePath
				_, _ = a.Editor.OpenFile(first.FullPath, first.RelativePath)
			}
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
		if a.Engine.MemoryMgr != nil {
			a.Engine.MemoryMgr.SetWorkspace(rawDir)
		}
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
	if a.Editor != nil && len(a.Editor.OpenTabPaths) == 0 {
		if first, ok := a.Workspace.FirstFile(); ok {
			a.SelectedFile = first.RelativePath
			_, _ = a.Editor.OpenFile(first.FullPath, first.RelativePath)
		}
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
	a.setMode(target)
}

func (a *AppUI) renderMainColumn(c *ui.Context) {
	if a.Mode != ModeStudio {
		// Enforce zero idle WebKit background processes when outside Studio Mode
		if a.StudioWebView != nil || a.StudioNodeWebView != nil {
			a.closeStudioWebView()
			a.closeStudioNodeWebView()
		}
	}

	if a.Mode == ModeSettings {
		a.renderSettingsScreen(c)
		return
	}
	if a.Mode == ModeStudio {
		ui.Row(c).Grow(1).FillHeight().AlignItems(ui.Stretch).Children(func() {
			ui.Column(c).Grow(1).FillHeight().Clip().Children(func() {
				a.renderWorkspaceHeader(c)

				ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
					a.renderStudioSurface(c)
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

	// Agent Mode multi-pane workspace: primary center pane (+ optional side-by-side
	// split session pane, Orca style) + collapsible right drawer.
	ui.Row(c).Grow(1).FillHeight().AlignItems(ui.Stretch).Children(func() {
		splitSess := (*agent.AgentSession)(nil)
		if a.AgentSplitSessionID != "" && !a.IsGitClientOpen && a.AgentViewMode == AgentViewChat {
			splitSess = a.findAgentSessionByID(a.AgentSplitSessionID)
		}

		renderPrimaryPane := func() {
			ui.Column(c).Grow(1).FillHeight().Clip().Children(func() {
				a.renderWorkspaceHeader(c)

				ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
					a.renderAgentSurface(c)
				})
			})
		}

		if splitSess != nil {
			half := float32(480)
			if winW, _ := c.Size(); winW > 0 {
				avail := winW
				if a.ShowLeftSidebar {
					avail -= a.LeftSidebarWidth
				}
				if a.ShowRightDrawer {
					avail -= a.RightDrawerWidth
				}
				if avail > 640 {
					half = avail / 2
				}
			}
			if a.AgentSplitWidth <= 0 {
				a.AgentSplitWidth = half
			}
			ui.Split(c, &a.AgentSplitWidth, func() {
				renderPrimaryPane()
			}, func() {
				a.renderSplitSessionPane(c, a.ThemeMgr.Colors(), splitSess)
			}).SplitReset(half).Grow(1).FillHeight()
		} else {
			renderPrimaryPane()
		}

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
