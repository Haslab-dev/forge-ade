package git

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hasdev/forge-ade/app/native/pkg/engine"
	internalgit "github.com/hasdev/forge-ade/core/git"
)

// Git Sub-Tab Constants
const (
	GitTabChanges   = 0 // Staged and Unstaged Working Tree Changes
	GitTabHistory   = 1 // Commit Log & Graph
	GitTabBranches  = 2 // Branch Management
	GitTabPRs       = 3 // Pull Requests / Merge Requests
	GitTabWorktrees = 4 // Linked Worktrees
)

// GitManager manages repository status, staging, unstaging, AI commits, and branches.
type GitManager struct {
	mu                    sync.RWMutex
	engine                *engine.Engine
	RootDir               string
	DiscardPendingPath    string // file awaiting discard confirmation
	Branch                string
	ActiveTab             int
	Staged                []internalgit.FileStatus
	Unstaged              []internalgit.FileStatus
	Untracked             []internalgit.FileStatus
	Conflicts             []internalgit.FileStatus
	Commits               []internalgit.CommitNode
	Branches              []string
	CommitMsg             string
	CommitSummary         string
	CommitDesc            string
	NewBranchName         string
	StatusMsg             string
	IsGeneratingAI        bool
	IsFetching            bool
	IsPulling             bool
	IsPushing             bool
	SelectedCommitHash    string
	SelectedCommitDiff    string
	SelectedCommitMessage string
	SelectedCommitAuthor  string
	SelectedCommitFiles   []CommitFileDiff
	LoadingCommitDiff     bool
	SearchQuery           string
	DiffStarted           bool

	OnFileSelect func(path string, showDiff bool)
	OnNeedRedraw func()

	// OnAICommitResult delivers the AI-generated commit message (already
	// split into summary and description) once generation finishes. Called
	// from GenerateAICommit's goroutine; the implementation must hop to the
	// UI thread (CommitSummary/CommitDesc are bound to text-input widgets
	// there). When nil (headless/tests, no UI attached), GenerateAICommit
	// falls back to writing the fields under lock.
	OnAICommitResult func(summary, desc string)

	fileDiffCache       map[string]string
	fileDiffStale       map[string]bool
	fileDiffPending     map[string]bool
	lastHistoryAt       time.Time
	cachedRemoteSource  *internalgit.GitRemoteSource
	remoteSourceLoaded  bool
	remoteSourceLoading bool
	cachedRootDir       string
}

// CommitFileDiff is one file's patch within the selected commit.
type CommitFileDiff struct {
	Path      string
	Additions int
	Deletions int
	Patch     string
	Diff      interface{} `json:"-"` // *mygoeditor.DiffState, built lazily by the UI
}

// NewGitManager creates an initialized GitManager for the repository.
func NewGitManager(engine *engine.Engine, rootDir string, onFileSelect func(path string, showDiff bool)) *GitManager {
	gm := NewGitManagerLazy(engine, rootDir, onFileSelect)
	gm.Refresh()
	return gm
}

// NewGitManagerLazy creates a GitManager without running an initial synchronous Refresh.
func NewGitManagerLazy(engine *engine.Engine, rootDir string, onFileSelect func(path string, showDiff bool)) *GitManager {
	return &GitManager{
		engine:          engine,
		RootDir:         rootDir,
		ActiveTab:       GitTabChanges,
		OnFileSelect:    onFileSelect,
		fileDiffCache:   make(map[string]string),
		fileDiffStale:   make(map[string]bool),
		fileDiffPending: make(map[string]bool),
		cachedRootDir:   rootDir,
	}
}

func (gm *GitManager) redraw() {
	if gm.OnNeedRedraw != nil {
		gm.OnNeedRedraw()
	}
}

// GitSnapshot is a consistent point-in-time copy of GitManager state for
// render paths. Background goroutines (Refresh, loadHistoryAndBranches,
// Fetch/Pull/Push) write the exported fields under mu, so reading fields
// directly from the render thread races; always render from a Snapshot.
type GitSnapshot struct {
	RootDir               string
	Branch                string
	Staged                []internalgit.FileStatus
	Unstaged              []internalgit.FileStatus
	Untracked             []internalgit.FileStatus
	Conflicts             []internalgit.FileStatus
	Commits               []internalgit.CommitNode
	Branches              []string
	StatusMsg             string
	CommitMsg             string
	CommitSummary         string
	CommitDesc            string
	IsGeneratingAI        bool
	IsFetching            bool
	IsPulling             bool
	IsPushing             bool
	SelectedCommitHash    string
	SelectedCommitDiff    string
	SelectedCommitMessage string
	SelectedCommitAuthor  string
	SelectedCommitFiles   []CommitFileDiff
	LoadingCommitDiff     bool
	DiscardPendingPath    string
}

// Snapshot returns the render-relevant state under RLock. Slice fields are
// returned without copying: writers only ever replace them wholesale (never
// mutate elements in place), so a returned slice is a stable snapshot.
func (gm *GitManager) Snapshot() GitSnapshot {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	return GitSnapshot{
		RootDir:               gm.RootDir,
		Branch:                gm.Branch,
		Staged:                gm.Staged,
		Unstaged:              gm.Unstaged,
		Untracked:             gm.Untracked,
		Conflicts:             gm.Conflicts,
		Commits:               gm.Commits,
		Branches:              gm.Branches,
		StatusMsg:             gm.StatusMsg,
		CommitMsg:             gm.CommitMsg,
		CommitSummary:         gm.CommitSummary,
		CommitDesc:            gm.CommitDesc,
		IsGeneratingAI:        gm.IsGeneratingAI,
		IsFetching:            gm.IsFetching,
		IsPulling:             gm.IsPulling,
		IsPushing:             gm.IsPushing,
		SelectedCommitHash:    gm.SelectedCommitHash,
		SelectedCommitDiff:    gm.SelectedCommitDiff,
		SelectedCommitMessage: gm.SelectedCommitMessage,
		SelectedCommitAuthor:  gm.SelectedCommitAuthor,
		SelectedCommitFiles:   gm.SelectedCommitFiles,
		LoadingCommitDiff:     gm.LoadingCommitDiff,
		DiscardPendingPath:    gm.DiscardPendingPath,
	}
}

// SetCommitMessage sets the commit message fields under lock. Safe from any
// goroutine; render paths read these through Snapshot.
func (gm *GitManager) SetCommitMessage(msg, summary, desc string) {
	gm.mu.Lock()
	gm.CommitMsg = msg
	gm.CommitSummary = summary
	gm.CommitDesc = desc
	gm.mu.Unlock()
}

// ClearCommitDraft empties the commit message fields under lock. The UI
// calls this from its commit-completion path on the UI thread (the fields
// are bound to text-input widgets there); Commit itself no longer clears
// them from its goroutine.
func (gm *GitManager) ClearCommitDraft() {
	gm.mu.Lock()
	gm.CommitMsg = ""
	gm.CommitSummary = ""
	gm.CommitDesc = ""
	gm.mu.Unlock()
}

// SetDiscardPending records or clears the file awaiting discard confirmation
// under lock. The render thread (discard button handlers) and Discard's
// goroutine both write this field, so both sides must take the lock.
func (gm *GitManager) SetDiscardPending(path string) {
	gm.mu.Lock()
	gm.DiscardPendingPath = path
	gm.mu.Unlock()
}

// SetWorkspace updates the repository root, resets cached state, and refreshes.
func (gm *GitManager) SetWorkspace(rootDir string) {
	gm.mu.Lock()
	gm.RootDir = rootDir
	gm.cachedRootDir = rootDir
	gm.fileDiffCache = make(map[string]string)
	gm.fileDiffStale = make(map[string]bool)
	gm.fileDiffPending = make(map[string]bool)
	gm.lastHistoryAt = time.Time{}
	gm.cachedRemoteSource = nil
	gm.remoteSourceLoaded = false
	gm.remoteSourceLoading = false
	gm.mu.Unlock()
	gm.Refresh()
}

// GetCachedFileDiff returns a cached working-tree unified diff for relPath if
// ready, or queues a background fetch and redraw without blocking UI renders.
// Stale entries are returned immediately (stale-while-revalidate) while a fresh
// diff is fetched asynchronously, preventing "Loading diff" flicker on refresh.
func (gm *GitManager) GetCachedFileDiff(relPath string) (string, bool) {
	gm.mu.Lock()
	if gm.fileDiffCache == nil {
		gm.fileDiffCache = make(map[string]string)
	}
	if gm.fileDiffStale == nil {
		gm.fileDiffStale = make(map[string]bool)
	}
	if gm.fileDiffPending == nil {
		gm.fileDiffPending = make(map[string]bool)
	}
	cachedPatch, hasCached := gm.fileDiffCache[relPath]
	isStale := gm.fileDiffStale[relPath]
	if hasCached && !isStale {
		gm.mu.Unlock()
		return cachedPatch, true
	}
	if !gm.fileDiffPending[relPath] && gm.engine != nil && gm.RootDir != "" && relPath != "" {
		gm.fileDiffPending[relPath] = true
		eng := gm.engine
		root := gm.RootDir
		gm.mu.Unlock()
		go func() {
			patch := eng.GetGitDiff(root, relPath)
			gm.mu.Lock()
			if gm.RootDir == root {
				if gm.fileDiffCache == nil {
					gm.fileDiffCache = make(map[string]string)
				}
				gm.fileDiffCache[relPath] = patch
				delete(gm.fileDiffStale, relPath)
			}
			delete(gm.fileDiffPending, relPath)
			gm.mu.Unlock()
			gm.redraw()
		}()
		if hasCached {
			return cachedPatch, true
		}
		return "", false
	}
	gm.mu.Unlock()
	if hasCached {
		return cachedPatch, true
	}
	return "", false
}

// Refresh updates branch information, porcelain file status, history, and branches.
// The git status exec runs without the lock held so concurrent callers and
// render-path Snapshot reads never block on it.
func (gm *GitManager) Refresh() {
	gm.mu.Lock()
	// Mark cached diffs stale rather than wiping them so active DiffViews keep
	// showing the previous patch while refreshing in the background.
	if gm.fileDiffStale == nil {
		gm.fileDiffStale = make(map[string]bool)
	}
	for k := range gm.fileDiffCache {
		gm.fileDiffStale[k] = true
	}
	if gm.RootDir != gm.cachedRootDir {
		gm.fileDiffCache = make(map[string]string)
		gm.fileDiffStale = make(map[string]bool)
		gm.fileDiffPending = make(map[string]bool)
		gm.lastHistoryAt = time.Time{}
		gm.cachedRemoteSource = nil
		gm.remoteSourceLoaded = false
		gm.remoteSourceLoading = false
		gm.cachedRootDir = gm.RootDir
	}
	eng := gm.engine
	root := gm.RootDir
	shouldLoadHistory := gm.lastHistoryAt.IsZero() || time.Since(gm.lastHistoryAt) >= 5*time.Second
	if shouldLoadHistory {
		gm.lastHistoryAt = time.Now()
	}
	gm.mu.Unlock()

	if eng == nil {
		return
	}

	st, err := eng.GetGitStatus(root)
	if err != nil {
		gm.mu.Lock()
		if gm.RootDir == root {
			gm.StatusMsg = fmt.Sprintf("Git error: %v", err)
		}
		gm.mu.Unlock()
		gm.redraw()
		return
	}

	gm.mu.Lock()
	if gm.RootDir == root {
		gm.Branch = st.Branch
		gm.Staged = st.Staged
		gm.Unstaged = st.Unstaged
		gm.Untracked = st.Untracked
		gm.Conflicts = st.Conflicts
		gm.StatusMsg = fmt.Sprintf("Refreshed (Branch: %s)", st.Branch)
	}
	gm.mu.Unlock()

	// Fetch history, branches & remote source in background (throttled to avoid
	// re-running git log --graph on every stage/unstage or 3s poll).
	if shouldLoadHistory {
		go gm.loadHistoryAndBranches()
	}
	gm.redraw()
}

// RefreshAsync runs Refresh on a goroutine. Use from render paths and event
// handlers so multi-second git execs never block the frame.
func (gm *GitManager) RefreshAsync() {
	go gm.Refresh()
}

func (gm *GitManager) loadHistoryAndBranches() {
	gm.mu.Lock()
	eng := gm.engine
	root := gm.RootDir
	fetchRemote := eng != nil && eng.GitEngine != nil && root != "" && !gm.remoteSourceLoaded && !gm.remoteSourceLoading
	if fetchRemote {
		gm.remoteSourceLoading = true
	}
	gm.mu.Unlock()

	if eng == nil {
		return
	}

	graph, err := eng.GetGitCommitGraph(root, 30)
	branches, bErr := eng.GetGitBranches(root)
	var remoteSrc *internalgit.GitRemoteSource
	if fetchRemote {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		src, rErr := eng.GitEngine.GetRemoteSource(ctx, root)
		cancel()
		if rErr == nil {
			remoteSrc = src
		}
	}

	gm.mu.Lock()
	if gm.RootDir == root {
		if err == nil && graph != nil {
			gm.Commits = graph.Commits
		}
		if bErr == nil {
			gm.Branches = branches
		}
		if fetchRemote {
			gm.cachedRemoteSource = remoteSrc
			gm.remoteSourceLoaded = true
			gm.remoteSourceLoading = false
		}
	} else if fetchRemote {
		gm.remoteSourceLoading = false
	}
	gm.mu.Unlock()

	gm.redraw()
}

// Stage stages a single file.
func (gm *GitManager) Stage(path string) {
	gm.mu.RLock()
	eng, root := gm.engine, gm.RootDir
	gm.mu.RUnlock()
	if eng == nil {
		return
	}
	_ = eng.GitStage(root, []string{path})
	gm.Refresh()
	if gm.OnNeedRedraw != nil {
		gm.OnNeedRedraw()
	}
}

// StageAll stages all modified and untracked files.
func (gm *GitManager) StageAll() {
	gm.mu.RLock()
	eng, root := gm.engine, gm.RootDir
	gm.mu.RUnlock()
	if eng == nil {
		return
	}
	_ = eng.GitStage(root, []string{"."})
	gm.Refresh()
	if gm.OnNeedRedraw != nil {
		gm.OnNeedRedraw()
	}
}

// Unstage unstages a staged file back to the working tree.
func (gm *GitManager) Unstage(path string) {
	gm.mu.RLock()
	eng, root := gm.engine, gm.RootDir
	gm.mu.RUnlock()
	if eng == nil {
		return
	}
	_ = eng.GitUnstage(root, []string{path})
	gm.Refresh()
	if gm.OnNeedRedraw != nil {
		gm.OnNeedRedraw()
	}
}

// UnstageAll unstages all staged files back to working tree.
// Discard reverts the working-tree changes of one file, as forge-ade's
// discard action does.
func (gm *GitManager) Discard(path string) {
	gm.mu.Lock()
	eng := gm.engine
	root := gm.RootDir
	gm.DiscardPendingPath = ""
	gm.mu.Unlock()
	if eng == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := eng.GitEngine.Discard(ctx, root, []string{path}); err == nil {
		gm.Refresh()
	}
}

func (gm *GitManager) UnstageAll() {
	if gm.engine == nil {
		return
	}
	gm.mu.RLock()
	stagedPaths := make([]string, 0, len(gm.Staged))
	for _, f := range gm.Staged {
		stagedPaths = append(stagedPaths, f.Path)
	}
	gm.mu.RUnlock()
	if len(stagedPaths) > 0 {
		gm.mu.RLock()
		eng, root := gm.engine, gm.RootDir
		gm.mu.RUnlock()
		_ = eng.GitUnstage(root, stagedPaths)
	}
	gm.Refresh()
	if gm.OnNeedRedraw != nil {
		gm.OnNeedRedraw()
	}
}

// Fetch fetches from git remotes.
func (gm *GitManager) Fetch() error {
	if gm.engine == nil {
		return fmt.Errorf("no engine")
	}
	gm.mu.Lock()
	gm.IsFetching = true
	gm.mu.Unlock()
	if gm.OnNeedRedraw != nil {
		gm.OnNeedRedraw()
	}
	defer func() {
		gm.mu.Lock()
		gm.IsFetching = false
		gm.mu.Unlock()
		if gm.OnNeedRedraw != nil {
			gm.OnNeedRedraw()
		}
	}()
	gm.mu.RLock()
	eng, root := gm.engine, gm.RootDir
	gm.mu.RUnlock()
	err := eng.GitFetch(root)
	gm.Refresh()
	return err
}

// Pull pulls from tracking branch.
func (gm *GitManager) Pull() error {
	if gm.engine == nil {
		return fmt.Errorf("no engine")
	}
	gm.mu.Lock()
	gm.IsPulling = true
	gm.mu.Unlock()
	if gm.OnNeedRedraw != nil {
		gm.OnNeedRedraw()
	}
	defer func() {
		gm.mu.Lock()
		gm.IsPulling = false
		gm.mu.Unlock()
		if gm.OnNeedRedraw != nil {
			gm.OnNeedRedraw()
		}
	}()
	gm.mu.RLock()
	eng, root := gm.engine, gm.RootDir
	gm.mu.RUnlock()
	err := eng.GitPull(root)
	gm.Refresh()
	return err
}

// Push pushes committed commits to upstream.
func (gm *GitManager) Push() error {
	if gm.engine == nil {
		return fmt.Errorf("no engine")
	}
	gm.mu.Lock()
	gm.IsPushing = true
	gm.mu.Unlock()
	if gm.OnNeedRedraw != nil {
		gm.OnNeedRedraw()
	}
	defer func() {
		gm.mu.Lock()
		gm.IsPushing = false
		gm.mu.Unlock()
		if gm.OnNeedRedraw != nil {
			gm.OnNeedRedraw()
		}
	}()
	gm.mu.RLock()
	eng, root := gm.engine, gm.RootDir
	gm.mu.RUnlock()
	err := eng.GitPush(root)
	gm.Refresh()
	return err
}

// SelectCommit loads diff for the designated commit hash.
// SelectCommit loads a commit's diff and splits it into per-file patches
// with +/- counts, for the Git Client's detail pane.
func (gm *GitManager) SelectCommit(hash string) {
	if gm.engine == nil {
		return
	}
	gm.mu.Lock()
	gm.SelectedCommitHash = hash
	gm.SelectedCommitDiff = ""
	gm.SelectedCommitFiles = nil
	gm.LoadingCommitDiff = true
	for _, cm := range gm.Commits {
		if cm.Hash == hash {
			gm.SelectedCommitMessage = cm.Message
			gm.SelectedCommitAuthor = cm.AuthorName
			break
		}
	}
	gm.mu.Unlock()
	gm.redraw()
	eng := gm.engine
	gm.mu.RLock()
	root := gm.RootDir
	gm.mu.RUnlock()
	go func() {
		diff := eng.GetCommitDiff(root, hash)
		files := splitCommitDiff(diff)
		gm.mu.Lock()
		if gm.SelectedCommitHash == hash {
			gm.SelectedCommitDiff = diff
			gm.SelectedCommitFiles = files
			gm.LoadingCommitDiff = false
		}
		gm.mu.Unlock()
		gm.redraw()
	}()
}

// splitCommitDiff breaks a full commit patch into per-file patches with
// +/- counts counted from the hunks.
func splitCommitDiff(diff string) []CommitFileDiff {
	if diff == "" {
		return nil
	}
	var out []CommitFileDiff
	var cur *CommitFileDiff
	var curBuf strings.Builder
	flush := func() {
		if cur != nil {
			cur.Patch = curBuf.String()
			for _, line := range strings.Split(cur.Patch, "\n") {
				if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
					cur.Additions++
				} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
					cur.Deletions++
				}
			}
			out = append(out, *cur)
		}
		cur = nil
		curBuf.Reset()
	}
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			flush()
			cur = &CommitFileDiff{}
			// "diff --git a/pkg/x.go b/pkg/x.go"
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				cur.Path = strings.TrimPrefix(parts[len(parts)-1], "b/")
			}
			curBuf.WriteString(line + "\n")
			continue
		}
		if cur == nil {
			continue
		}
		curBuf.WriteString(line + "\n")
	}
	flush()
	return out
}

// Commit creates a new commit with the provided commit message.
func (gm *GitManager) Commit(msg string) error {
	if gm.engine == nil {
		return fmt.Errorf("no engine available")
	}
	if strings.TrimSpace(msg) == "" {
		return fmt.Errorf("commit message cannot be empty")
	}

	gm.mu.RLock()
	root := gm.RootDir
	gm.mu.RUnlock()
	err := gm.engine.GitCommit(root, msg)
	gm.mu.Lock()
	if err != nil {
		gm.StatusMsg = fmt.Sprintf("Commit error: %v", err)
	} else {
		// The commit draft fields are NOT cleared here: they are bound to
		// text-input widgets on the UI thread, so the UI clears them via
		// ClearCommitDraft from its commit-completion path on the UI thread.
		gm.StatusMsg = "Committed successfully"
	}
	gm.mu.Unlock()

	gm.Refresh()
	if gm.OnNeedRedraw != nil {
		gm.OnNeedRedraw()
	}
	return err
}

// SwitchBranch switches HEAD to the designated local branch.
func (gm *GitManager) SwitchBranch(branch string) error {
	if gm.engine == nil {
		return fmt.Errorf("no engine available")
	}
	gm.mu.RLock()
	root := gm.RootDir
	gm.mu.RUnlock()
	err := gm.engine.GitCheckout(root, branch)
	gm.Refresh()
	if gm.OnNeedRedraw != nil {
		gm.OnNeedRedraw()
	}
	return err
}

// CreateBranch creates and checks out a new branch.
func (gm *GitManager) CreateBranch(name string) error {
	if gm.engine == nil {
		return fmt.Errorf("no engine available")
	}
	gm.mu.RLock()
	root := gm.RootDir
	gm.mu.RUnlock()
	err := gm.engine.GitCreateBranch(root, name)
	gm.Refresh()
	if gm.OnNeedRedraw != nil {
		gm.OnNeedRedraw()
	}
	return err
}

// GenerateAICommit invokes the AI conventional commit message generator.
func (gm *GitManager) GenerateAICommit(instruction string) {
	if gm.engine == nil {
		return
	}

	// Capture the callback on the calling thread: the UI wires it during the
	// first render of the changes pane (before this method can be invoked),
	// and the go statement that launched us establishes happens-before, so
	// the goroutine below reads only this local copy.
	cb := gm.OnAICommitResult

	gm.mu.Lock()
	gm.IsGeneratingAI = true
	gm.StatusMsg = "Generating AI commit message..."
	gm.mu.Unlock()

	if gm.OnNeedRedraw != nil {
		gm.OnNeedRedraw()
	}

	gm.mu.RLock()
	root := gm.RootDir
	eng := gm.engine
	gm.mu.RUnlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		msg, err := eng.GenerateAICommit(ctx, root, instruction)

		// Split the cleaned message into summary/description up front so the
		// result is a plain value, not a write into widget-bound fields.
		var cleanSummary, cleanDesc string
		clean := strings.TrimSpace(msg)
		if err == nil && clean != "" {
			if nl := strings.IndexByte(clean, '\n'); nl >= 0 {
				cleanSummary = strings.TrimSpace(clean[:nl])
				cleanDesc = strings.TrimSpace(clean[nl+1:])
			} else {
				cleanSummary = clean
			}
		}

		gm.mu.Lock()
		gm.IsGeneratingAI = false
		if err != nil {
			gm.StatusMsg = fmt.Sprintf("AI Commit error: %v", err)
		} else if cleanSummary != "" {
			// Never write CommitMsg/CommitSummary/CommitDesc from this
			// goroutine: the UI binds text-input widgets to those fields.
			// Deliver through OnAICommitResult (its implementation hops to
			// the UI thread); without a UI attached, fall back to the locked
			// write so headless callers keep the previous behavior.
			if cb != nil {
				gm.StatusMsg = "AI commit message generated"
			} else {
				gm.CommitMsg = clean
				gm.CommitSummary = cleanSummary
				gm.CommitDesc = cleanDesc
				gm.StatusMsg = "AI commit message generated"
			}
		}
		gm.mu.Unlock()

		if cb != nil && err == nil && cleanSummary != "" {
			cb(cleanSummary, cleanDesc)
		}

		if gm.OnNeedRedraw != nil {
			gm.OnNeedRedraw()
		}
	}()
}
