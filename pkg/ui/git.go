package ui

import (
	"fmt"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/internal/app"
	"github.com/hasdev/forge-ade/internal/git"
)

// GitManager manages repository status, staging, unstaging, and commit actions.
type GitManager struct {
	mu           sync.RWMutex
	backend      *app.App
	RootDir      string
	Branch       string
	Staged       []git.FileStatus
	Unstaged     []git.FileStatus
	Untracked    []git.FileStatus
	CommitMsg    string
	StatusMsg    string
	OnFileSelect func(path string, showDiff bool)
}

// NewGitManager creates an initialized GitManager for the repository.
func NewGitManager(backend *app.App, rootDir string, onFileSelect func(path string, showDiff bool)) *GitManager {
	gm := &GitManager{
		backend:      backend,
		RootDir:      rootDir,
		OnFileSelect: onFileSelect,
	}
	gm.Refresh()
	return gm
}

// Refresh updates branch information and porcelain file status.
func (gm *GitManager) Refresh() {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	if gm.backend == nil {
		return
	}

	st, err := gm.backend.GetGitStatus(gm.RootDir)
	if err != nil {
		gm.StatusMsg = fmt.Sprintf("Git error: %v", err)
		return
	}

	gm.Branch = st.Branch
	gm.Staged = st.Staged
	gm.Unstaged = st.Unstaged
	gm.Untracked = st.Untracked
	gm.StatusMsg = fmt.Sprintf("Refreshed (Branch: %s)", st.Branch)
}

// Stage stages a single file.
func (gm *GitManager) Stage(path string) {
	if gm.backend == nil {
		return
	}
	_ = gm.backend.GitStage(gm.RootDir, []string{path})
	gm.Refresh()
}

// Unstage unstages a single file.
func (gm *GitManager) Unstage(path string) {
	if gm.backend == nil {
		return
	}
	_ = gm.backend.GitUnstage(gm.RootDir, []string{path})
	gm.Refresh()
}

// StageAll stages all unstaged and untracked modifications.
func (gm *GitManager) StageAll() {
	if gm.backend == nil {
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
		_ = gm.backend.GitStage(gm.RootDir, paths)
		gm.Refresh()
	}
}

// Commit creates a new commit from the staged files.
func (gm *GitManager) Commit() {
	msg := strings.TrimSpace(gm.CommitMsg)
	if msg == "" || gm.backend == nil {
		return
	}

	if err := gm.backend.GitCommit(gm.RootDir, msg); err != nil {
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

// RenderGit draws the visual Git client panel.
func (gm *GitManager) RenderGit(c *ui.Context) {
	t := c.Theme()

	gm.mu.RLock()
	defer gm.mu.RUnlock()

	ui.Column(c).Fill().Children(func() {
		// Header Toolbar
		ui.Row(c).Gap(12).Padding(8, 16).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Text(c, "Source Control").Bold().FontSize(14)
			if gm.Branch != "" {
				ui.Text(c, fmt.Sprintf("⎇ %s", gm.Branch)).FontSize(12).TextColor(t.Accent)
			}
			ui.Box(c).Grow(1)
			if ui.Button(c, "↻ Refresh").Clicked() {
				go gm.Refresh()
			}
			if len(gm.Unstaged)+len(gm.Untracked) > 0 {
				if ui.Button(c, "+ Stage All").Clicked() {
					go gm.StageAll()
				}
			}
		})

		// Commit Composer
		ui.Column(c).Padding(12, 16).Gap(8).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			in := ui.TextInput(c, &gm.CommitMsg).Placeholder("Commit message (Cmd+Enter to commit)...").Grow(1)
			if in.Submitted() && len(gm.Staged) > 0 {
				go gm.Commit()
			}

			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				commitBtn := ui.PrimaryButton(c, fmt.Sprintf("Commit to %s (%d files)", gm.Branch, len(gm.Staged)))
				if len(gm.Staged) == 0 || strings.TrimSpace(gm.CommitMsg) == "" {
					commitBtn.Disabled(true)
				}
				if commitBtn.Clicked() {
					go gm.Commit()
				}
				ui.Box(c).Grow(1)
				if gm.StatusMsg != "" {
					ui.Text(c, gm.StatusMsg).FontSize(12).TextColor(t.TextMuted)
				}
			})
		})

		// Changes Lists
		ui.Scroll(c).Grow(1).Padding(8, 16).Children(func() {
			// 1. Staged Changes
			if len(gm.Staged) > 0 {
				ui.Row(c).Padding(8, 0).AlignItems(ui.Center).Children(func() {
					ui.Text(c, fmt.Sprintf("STAGED CHANGES (%d)", len(gm.Staged))).FontSize(11).Bold().TextColor(t.TextMuted)
				})
				for _, f := range gm.Staged {
					filePath := f.Path
					ui.Row(c.Key("staged-"+filePath)).Padding(4, 8).Radius(6).AlignItems(ui.Center).Children(func() {
						tagColor := t.Accent
						if f.Status == "D" {
							tagColor = t.TextMuted
						}
						ui.Text(c, f.Status).FontSize(11).Bold().TextColor(tagColor).Width(20)
						fileRow := ui.Text(c, filePath).Grow(1)
						if fileRow.Clicked() && gm.OnFileSelect != nil {
							gm.OnFileSelect(filePath, true)
						}
						if f.Additions > 0 || f.Deletions > 0 {
							ui.Text(c, fmt.Sprintf("+%d −%d", f.Additions, f.Deletions)).FontSize(11).TextColor(t.TextMuted)
						}
						if ui.Button(c, "− Unstage").FontSize(11).Clicked() {
							go gm.Unstage(filePath)
						}
					})
				}
			}

			// 2. Unstaged Changes
			if len(gm.Unstaged) > 0 {
				ui.Row(c).Padding(12, 0, 4, 0).AlignItems(ui.Center).Children(func() {
					ui.Text(c, fmt.Sprintf("CHANGES (%d)", len(gm.Unstaged))).FontSize(11).Bold().TextColor(t.TextMuted)
				})
				for _, f := range gm.Unstaged {
					filePath := f.Path
					ui.Row(c.Key("unstaged-"+filePath)).Padding(4, 8).Radius(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, f.Status).FontSize(11).Bold().TextColor(t.Accent).Width(20)
						fileRow := ui.Text(c, filePath).Grow(1)
						if fileRow.Clicked() && gm.OnFileSelect != nil {
							gm.OnFileSelect(filePath, true)
						}
						if f.Additions > 0 || f.Deletions > 0 {
							ui.Text(c, fmt.Sprintf("+%d −%d", f.Additions, f.Deletions)).FontSize(11).TextColor(t.TextMuted)
						}
						if ui.Button(c, "+ Stage").FontSize(11).Clicked() {
							go gm.Stage(filePath)
						}
					})
				}
			}

			// 3. Untracked Files
			if len(gm.Untracked) > 0 {
				ui.Row(c).Padding(12, 0, 4, 0).AlignItems(ui.Center).Children(func() {
					ui.Text(c, fmt.Sprintf("UNTRACKED FILES (%d)", len(gm.Untracked))).FontSize(11).Bold().TextColor(t.TextMuted)
				})
				for _, f := range gm.Untracked {
					filePath := f.Path
					ui.Row(c.Key("untracked-"+filePath)).Padding(4, 8).Radius(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "?").FontSize(11).Bold().TextColor(t.TextMuted).Width(20)
						fileRow := ui.Text(c, filePath).Grow(1)
						if fileRow.Clicked() && gm.OnFileSelect != nil {
							gm.OnFileSelect(filePath, false)
						}
						if ui.Button(c, "+ Track").FontSize(11).Clicked() {
							go gm.Stage(filePath)
						}
					})
				}
			}

			if len(gm.Staged) == 0 && len(gm.Unstaged) == 0 && len(gm.Untracked) == 0 {
				ui.Column(c).Grow(1).Center().Padding(40, 0).Children(func() {
					ui.Text(c, "Working tree clean").FontSize(15).Bold().TextColor(t.TextMuted)
					ui.Text(c, "No unstaged or untracked changes in repository.").TextColor(t.TextMuted)
				})
			}
		})
	})
}
