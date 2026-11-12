package git

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hasdev/forge-ade/forge-ade-native/pkg/engine"
	internalgit "github.com/hasdev/forge-ade/internal/git"
)

// Git Sub-Tab Constants
const (
	GitTabChanges  = 0 // Staged and Unstaged Working Tree Changes
	GitTabHistory  = 1 // Commit Log & Graph
	GitTabBranches = 2 // Branch Management
)

// GitManager manages repository status, staging, unstaging, AI commits, and branches.
type GitManager struct {
	mu                 sync.RWMutex
	engine             *engine.Engine
	RootDir            string
	Branch             string
	ActiveTab          int
	Staged             []internalgit.FileStatus
	Unstaged           []internalgit.FileStatus
	Untracked          []internalgit.FileStatus
	Commits            []internalgit.CommitNode
	Branches           []string
	CommitMsg          string
	CommitSummary      string
	CommitDesc         string
	StatusMsg          string
	IsGeneratingAI     bool
	IsFetching         bool
	IsPulling          bool
	IsPushing          bool
	SelectedCommitHash string
	SelectedCommitDiff string
	OnFileSelect       func(path string, showDiff bool)
	OnNeedRedraw       func()
}

// NewGitManager creates an initialized GitManager for the repository.
func NewGitManager(engine *engine.Engine, rootDir string, onFileSelect func(path string, showDiff bool)) *GitManager {
	gm := &GitManager{
		engine:       engine,
		RootDir:      rootDir,
		ActiveTab:    GitTabChanges,
		OnFileSelect: onFileSelect,
	}
	gm.Refresh()
	return gm
}

// Refresh updates branch information, porcelain file status, history, and branches.
func (gm *GitManager) Refresh() {
	gm.mu.Lock()
	defer gm.mu.Unlock()

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
	gm.StatusMsg = fmt.Sprintf("Refreshed (Branch: %s)", st.Branch)

	// Fetch history & branches in background
	go gm.loadHistoryAndBranches()
}

func (gm *GitManager) loadHistoryAndBranches() {
	if gm.engine == nil {
		return
	}

	graph, err := gm.engine.GetGitCommitGraph(gm.RootDir, 30)
	branches, bErr := gm.engine.GetGitBranches(gm.RootDir)

	gm.mu.Lock()
	if err == nil && graph != nil {
		gm.Commits = graph.Commits
	}
	if bErr == nil {
		gm.Branches = branches
	}
	gm.mu.Unlock()

	if gm.OnNeedRedraw != nil {
		gm.OnNeedRedraw()
	}
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
func (gm *GitManager) SelectCommit(hash string) {
	if gm.engine == nil {
		return
	}
	gm.mu.Lock()
	gm.SelectedCommitHash = hash
	gm.mu.Unlock()
	go func() {
		diff := gm.engine.GetCommitDiff(gm.RootDir, hash)
		gm.mu.Lock()
		if gm.SelectedCommitHash == hash {
			gm.SelectedCommitDiff = diff
		}
		gm.mu.Unlock()
		if gm.OnNeedRedraw != nil {
			gm.OnNeedRedraw()
		}
	}()
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
		if err != nil {
			gm.StatusMsg = fmt.Sprintf("AI Commit error: %v", err)
		} else if strings.TrimSpace(msg) != "" {
			gm.CommitMsg = strings.TrimSpace(msg)
			gm.StatusMsg = "AI commit message generated"
		}
		gm.mu.Unlock()

		if gm.OnNeedRedraw != nil {
			gm.OnNeedRedraw()
		}
	}()
}
