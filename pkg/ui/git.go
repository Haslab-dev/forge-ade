package ui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/internal/git"
)

// Git Sub-Tab Constants
const (
	GitTabChanges  = 0 // Staged and Unstaged Working Tree Changes
	GitTabHistory  = 1 // Commit Log & Graph
	GitTabBranches = 2 // Branch Management
)

// GitManager manages repository status, staging, unstaging, AI commits, and branches.
type GitManager struct {
	mu             sync.RWMutex
	engine         *Engine
	RootDir        string
	Branch         string
	ActiveTab      int
	Staged         []git.FileStatus
	Unstaged       []git.FileStatus
	Untracked      []git.FileStatus
	Commits        []git.CommitNode
	Branches       []string
	CommitMsg      string
	StatusMsg      string
	IsGeneratingAI bool
	OnFileSelect   func(path string, showDiff bool)
	OnNeedRedraw   func()
}

// NewGitManager creates an initialized GitManager for the repository.
func NewGitManager(engine *Engine, rootDir string, onFileSelect func(path string, showDiff bool)) *GitManager {
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
}

// Unstage unstages a single file.
func (gm *GitManager) Unstage(path string) {
	if gm.engine == nil {
		return
	}
	_ = gm.engine.GitUnstage(gm.RootDir, []string{path})
	gm.Refresh()
}

// StageAll stages all unstaged and untracked modifications.
func (gm *GitManager) StageAll() {
	if gm.engine == nil {
		return
	}
	var paths []string
	gm.mu.RLock()
	for _, f := range gm.Unstaged {
		paths = append(paths, f.Path)
	}
	for _, f := range gm.Untracked {
		paths = append(paths, f.Path)
	}
	gm.mu.RUnlock()

	if len(paths) > 0 {
		_ = gm.engine.GitStage(gm.RootDir, paths)
		gm.Refresh()
	}
}

// Commit creates a new commit from the staged files.
func (gm *GitManager) Commit() {
	msg := strings.TrimSpace(gm.CommitMsg)
	if msg == "" || gm.engine == nil {
		return
	}

	if err := gm.engine.GitCommit(gm.RootDir, msg); err != nil {
		gm.mu.Lock()
		gm.StatusMsg = fmt.Sprintf("Commit error: %v", err)
		gm.mu.Unlock()
		return
	}

	gm.mu.Lock()
	gm.CommitMsg = ""
	gm.StatusMsg = "Committed successfully"
	gm.mu.Unlock()
	gm.Refresh()
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

// RenderGit draws the complete visual Git client panel with Changes, History, and Branches.
func (gm *GitManager) RenderGit(c *ui.Context) {
	t := c.Theme()

	gm.mu.RLock()
	defer gm.mu.RUnlock()

	ui.Column(c).Fill().Children(func() {
		// Header Toolbar with Tabler Icons
		ui.Row(c).Gap(10).Padding(8, 14).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Text(c, "Source Control").Bold().FontSize(13)
			if gm.Branch != "" {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					TablerIcon(c, TablerGitBranch, 13)
					ui.Text(c, gm.Branch).FontSize(11).Bold().TextColor(t.Accent)
				})
			}
			ui.Box(c).Grow(1)
			refreshBtn := ui.Button(c, "")
			refreshBtn.Children(func() {
				TablerIcon(c, TablerRefresh, 13)
			})
			if refreshBtn.Clicked() {
				go gm.Refresh()
			}
		})

		// Sub-Tabs: [Changes] [History] [Branches] with Tabler Icons
		ui.Row(c).Gap(4).Padding(6, 12).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			subtabs := []struct {
				Tab   int
				Label string
				Icon  *ui.SVG
			}{
				{GitTabChanges, fmt.Sprintf("Changes (%d)", len(gm.Staged)+len(gm.Unstaged)+len(gm.Untracked)), TablerFileDiff},
				{GitTabHistory, "History", TablerHistory},
				{GitTabBranches, "Branches", TablerGitBranch},
			}
			for _, st := range subtabs {
				thisTab := st
				var btn ui.Element
				if gm.ActiveTab == thisTab.Tab {
					btn = ui.PrimaryButton(c, "")
				} else {
					btn = ui.Button(c, "")
				}
				btn.Children(func() {
					ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
						TablerIcon(c, thisTab.Icon, 12)
						ui.Text(c, thisTab.Label).FontSize(11).Bold()
					})
				})
				if btn.Clicked() {
					gm.ActiveTab = thisTab.Tab
				}
			}
		})

		// Tab Content
		switch gm.ActiveTab {
		case GitTabChanges:
			gm.renderChangesTab(c)
		case GitTabHistory:
			gm.renderHistoryTab(c)
		case GitTabBranches:
			gm.renderBranchesTab(c)
		}
	})
}

func (gm *GitManager) renderChangesTab(c *ui.Context) {
	t := c.Theme()

	// Commit Composer
	ui.Column(c).Padding(10, 14).Gap(6).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		in := ui.TextInput(c, &gm.CommitMsg).Placeholder("Commit message summary (⌘Enter)...").Grow(1)
		if in.Submitted() && len(gm.Staged) > 0 {
			go gm.Commit()
		}

		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			// Commit Button with TablerGitCommit
			commitBtn := ui.PrimaryButton(c, "")
			commitBtn.Children(func() {
				ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
					TablerIcon(c, TablerGitCommit, 13)
					ui.Text(c, fmt.Sprintf("Commit (%d)", len(gm.Staged))).FontSize(11).Bold()
				})
			})
			if len(gm.Staged) == 0 || strings.TrimSpace(gm.CommitMsg) == "" {
				commitBtn.Disabled(true)
			}
			if commitBtn.Clicked() {
				go gm.Commit()
			}

			// AI Commit Generator Button with TablerSparkles
			aiBtn := ui.Button(c, "")
			aiBtn.Children(func() {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					TablerIcon(c, TablerSparkles, 13)
					ui.Text(c, "AI Commit").FontSize(11)
				})
			})
			if gm.IsGeneratingAI {
				aiBtn.Disabled(true)
			}
			if aiBtn.Clicked() {
				go gm.GenerateAICommit("")
			}

			// Stage All Button with TablerPlus
			if len(gm.Unstaged)+len(gm.Untracked) > 0 {
				stageAllBtn := ui.Button(c, "")
				stageAllBtn.Children(func() {
					ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
						TablerIcon(c, TablerPlus, 12)
						ui.Text(c, "Stage All").FontSize(11)
					})
				})
				if stageAllBtn.Clicked() {
					go gm.StageAll()
				}
			}

			ui.Box(c).Grow(1)
			if gm.StatusMsg != "" {
				ui.Text(c, gm.StatusMsg).FontSize(11).TextColor(t.TextMuted)
			}
		})
	})

	// Scrollable Changes Lists
	ui.Scroll(c).Grow(1).Padding(8, 14).Children(func() {
		// 1. Staged Changes
		if len(gm.Staged) > 0 {
			ui.Row(c).Padding(6, 0).AlignItems(ui.Center).Children(func() {
				ui.Text(c, fmt.Sprintf("STAGED CHANGES (%d)", len(gm.Staged))).FontSize(10).Bold().TextColor(t.TextMuted)
			})
			for _, f := range gm.Staged {
				filePath := f.Path
				ui.Row(c.Key("staged-"+filePath)).Padding(4, 8).Radius(6).AlignItems(ui.Center).Children(func() {
					tagColor := t.Accent
					if f.Status == "D" {
						tagColor = t.TextMuted
					}
					ui.Text(c, f.Status).FontSize(11).Bold().TextColor(tagColor).Width(18)
					fileRow := ui.Text(c, filePath).FontSize(12).Grow(1)
					if fileRow.Clicked() && gm.OnFileSelect != nil {
						gm.OnFileSelect(filePath, true)
					}
					if f.Additions > 0 || f.Deletions > 0 {
						ui.Text(c, fmt.Sprintf("+%d −%d", f.Additions, f.Deletions)).FontSize(10).TextColor(t.TextMuted)
					}
					unstageBtn := ui.Button(c, "")
					unstageBtn.Children(func() {
						TablerIcon(c, TablerMinus, 11)
					})
					if unstageBtn.Clicked() {
						go gm.Unstage(filePath)
					}
				})
			}
		}

		// 2. Unstaged Changes
		if len(gm.Unstaged) > 0 {
			ui.Row(c).Padding(10, 0, 4, 0).AlignItems(ui.Center).Children(func() {
				ui.Text(c, fmt.Sprintf("CHANGES (%d)", len(gm.Unstaged))).FontSize(10).Bold().TextColor(t.TextMuted)
			})
			for _, f := range gm.Unstaged {
				filePath := f.Path
				ui.Row(c.Key("unstaged-"+filePath)).Padding(4, 8).Radius(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, f.Status).FontSize(11).Bold().TextColor(t.Accent).Width(18)
					fileRow := ui.Text(c, filePath).FontSize(12).Grow(1)
					if fileRow.Clicked() && gm.OnFileSelect != nil {
						gm.OnFileSelect(filePath, true)
					}
					if f.Additions > 0 || f.Deletions > 0 {
						ui.Text(c, fmt.Sprintf("+%d −%d", f.Additions, f.Deletions)).FontSize(10).TextColor(t.TextMuted)
					}
					stageBtn := ui.Button(c, "")
					stageBtn.Children(func() {
						TablerIcon(c, TablerPlus, 11)
					})
					if stageBtn.Clicked() {
						go gm.Stage(filePath)
					}
				})
			}
		}

		// 3. Untracked Files
		if len(gm.Untracked) > 0 {
			ui.Row(c).Padding(10, 0, 4, 0).AlignItems(ui.Center).Children(func() {
				ui.Text(c, fmt.Sprintf("UNTRACKED FILES (%d)", len(gm.Untracked))).FontSize(10).Bold().TextColor(t.TextMuted)
			})
			for _, f := range gm.Untracked {
				filePath := f.Path
				ui.Row(c.Key("untracked-"+filePath)).Padding(4, 8).Radius(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "?").FontSize(11).Bold().TextColor(t.TextMuted).Width(18)
					fileRow := ui.Text(c, filePath).FontSize(12).Grow(1)
					if fileRow.Clicked() && gm.OnFileSelect != nil {
						gm.OnFileSelect(filePath, false)
					}
					stageBtn := ui.Button(c, "")
					stageBtn.Children(func() {
						TablerIcon(c, TablerPlus, 11)
					})
					if stageBtn.Clicked() {
						go gm.Stage(filePath)
					}
				})
			}
		}

		if len(gm.Staged) == 0 && len(gm.Unstaged) == 0 && len(gm.Untracked) == 0 {
			ui.Column(c).Grow(1).Center().Padding(36, 0).Gap(8).Children(func() {
				TablerIcon(c, TablerCircleCheck, 24)
				ui.Text(c, "Working tree clean").FontSize(14).Bold().TextColor(t.TextMuted)
				ui.Text(c, "No uncommitted modifications.").FontSize(12).TextColor(t.TextMuted)
			})
		}
	})
}

func (gm *GitManager) renderHistoryTab(c *ui.Context) {
	t := c.Theme()

	if len(gm.Commits) == 0 {
		ui.Column(c).Grow(1).Center().Padding(40, 0).Children(func() {
			ui.Text(c, "No commit history found").FontSize(13).Bold().TextColor(t.TextMuted)
		})
		return
	}

	ui.Scroll(c).Grow(1).Padding(8, 14).Children(func() {
		ui.Column(c).Gap(6).Children(func() {
			for _, commit := range gm.Commits {
				cm := commit
				ui.Column(c.Key("commit-"+cm.Hash)).Padding(8, 10).Radius(6).Background(t.Surface).BorderWidth(1).BorderColor(t.Border).Gap(4).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Children(func() {
						ui.Text(c, cm.ShortHash).Bold().FontSize(11).TextColor(t.Accent)
						ui.Box(c).Grow(1)
						timeStr := cm.Timestamp.Format("Jan 02, 15:04")
						ui.Text(c, timeStr).FontSize(10).TextColor(t.TextMuted)
					})
					ui.Text(c, cm.Message).FontSize(12).Bold()
					ui.Text(c, cm.AuthorName).FontSize(10).TextColor(t.TextMuted)
				})
			}
		})
	})
}

func (gm *GitManager) renderBranchesTab(c *ui.Context) {
	t := c.Theme()

	ui.Scroll(c).Grow(1).Padding(8, 14).Children(func() {
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, "BRANCHES").FontSize(10).Bold().TextColor(t.TextMuted).Padding(4, 0)
			for _, b := range gm.Branches {
				branchName := b
				isCurrent := (branchName == gm.Branch)
				row := ui.Row(c.Key("branch-"+branchName)).Padding(6, 10).Radius(6).AlignItems(ui.Center)
				if isCurrent {
					row.Background(t.Border)
				}
				row.Children(func() {
					TablerIcon(c, TablerGitBranch, 13)
					ui.Text(c, branchName).FontSize(12).Bold().Grow(1)
					if isCurrent {
						ui.Text(c, "Active").FontSize(10).TextColor(t.Accent)
					}
				})
			}
		})
	})
}
