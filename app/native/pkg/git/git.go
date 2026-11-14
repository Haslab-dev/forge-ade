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

	fileDiffCache       map[string]string
	fileDiffPending     map[string]bool
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
		fileDiffPending: make(map[string]bool),
		cachedRootDir:   rootDir,
	}
}

func (gm *GitManager) redraw() {
	if gm.OnNeedRedraw != nil {
		gm.OnNeedRedraw()
	}
}

// SetWorkspace updates the repository root, resets cached state, and refreshes.
func (gm *GitManager) SetWorkspace(rootDir string) {
	gm.mu.Lock()
	gm.RootDir = rootDir
	gm.cachedRootDir = rootDir
	gm.fileDiffCache = make(map[string]string)
	gm.fileDiffPending = make(map[string]bool)
	gm.cachedRemoteSource = nil
	gm.remoteSourceLoaded = false
	gm.remoteSourceLoading = false
	gm.mu.Unlock()
	gm.Refresh()
}

// GetCachedFileDiff returns a cached working-tree unified diff for relPath if
// ready, or queues a background fetch and redraw without blocking UI renders.
func (gm *GitManager) GetCachedFileDiff(relPath string) (string, bool) {
	gm.mu.Lock()
	if gm.fileDiffCache == nil {
		gm.fileDiffCache = make(map[string]string)
	}
	if gm.fileDiffPending == nil {
		gm.fileDiffPending = make(map[string]bool)
	}
	if patch, ok := gm.fileDiffCache[relPath]; ok {
		gm.mu.Unlock()
		return patch, true
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
			}
			delete(gm.fileDiffPending, relPath)
			gm.mu.Unlock()
			gm.redraw()
		}()
		return "", false
	}
	gm.mu.Unlock()
	return "", false
}

// Refresh updates branch information, porcelain file status, history, and branches.
func (gm *GitManager) Refresh() {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	gm.fileDiffCache = make(map[string]string)
	gm.fileDiffPending = make(map[string]bool)
	if gm.RootDir != gm.cachedRootDir {
		gm.cachedRemoteSource = nil
		gm.remoteSourceLoaded = false
		gm.remoteSourceLoading = false
		gm.cachedRootDir = gm.RootDir
	}

	if gm.engine == nil {
		return
	}

	st, err := gm.engine.GetGitStatus(gm.RootDir)
	if err != nil {
		gm.StatusMsg = fmt.Sprintf("Git error: %v", err)
		return
	}

	gm.Branch = st.Branch
	gm.Staged = st.Staged
	gm.Unstaged = st.Unstaged
	gm.Untracked = st.Untracked
	gm.Conflicts = st.Conflicts
	gm.StatusMsg = fmt.Sprintf("Refreshed (Branch: %s)", st.Branch)

	// Fetch history, branches & remote source in background
	go gm.loadHistoryAndBranches()
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
	if gm.engine == nil {
		return
	}
	_ = gm.engine.GitStage(gm.RootDir, []string{path})
	gm.Refresh()
	if gm.OnNeedRedraw != nil {
		gm.OnNeedRedraw()
	}
}

// StageAll stages all modified and untracked files.
func (gm *GitManager) StageAll() {
	if gm.engine == nil {
		return
	}
	_ = gm.engine.GitStage(gm.RootDir, []string{"."})
	gm.Refresh()
	if gm.OnNeedRedraw != nil {
		gm.OnNeedRedraw()
	}
}

// Unstage unstages a staged file back to the working tree.
func (gm *GitManager) Unstage(path string) {
	if gm.engine == nil {
		return
	}
	_ = gm.engine.GitUnstage(gm.RootDir, []string{path})
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
		_ = gm.engine.GitUnstage(gm.RootDir, stagedPaths)
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
	err := gm.engine.GitFetch(gm.RootDir)
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
	err := gm.engine.GitPull(gm.RootDir)
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
	err := gm.engine.GitPush(gm.RootDir)
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
	go func() {
		diff := gm.engine.GetCommitDiff(gm.RootDir, hash)
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

	err := gm.engine.GitCommit(gm.RootDir, msg)
	gm.mu.Lock()
	if err != nil {
		gm.StatusMsg = fmt.Sprintf("Commit error: %v", err)
	} else {
		gm.CommitMsg = ""
		gm.CommitSummary = ""
		gm.CommitDesc = ""
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
	err := gm.engine.GitCheckout(gm.RootDir, branch)
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
	err := gm.engine.GitCreateBranch(gm.RootDir, name)
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

	gm.mu.Lock()
	gm.IsGeneratingAI = true
	gm.StatusMsg = "Generating AI commit message..."
	gm.mu.Unlock()

	if gm.OnNeedRedraw != nil {
		gm.OnNeedRedraw()
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		msg, err := gm.engine.GenerateAICommit(ctx, gm.RootDir, instruction)
		gm.mu.Lock()
		gm.IsGeneratingAI = false
		clean := strings.TrimSpace(msg)
		if err != nil {
			gm.StatusMsg = fmt.Sprintf("AI Commit error: %v", err)
		} else if clean != "" {
			gm.CommitMsg = clean
			if nl := strings.IndexByte(clean, '\n'); nl >= 0 {
				gm.CommitSummary = strings.TrimSpace(clean[:nl])
				gm.CommitDesc = strings.TrimSpace(clean[nl+1:])
			} else {
				gm.CommitSummary = clean
				gm.CommitDesc = ""
			}
			gm.StatusMsg = "AI commit message generated"
		}
		gm.mu.Unlock()

		if gm.OnNeedRedraw != nil {
			gm.OnNeedRedraw()
		}
	}()
}
