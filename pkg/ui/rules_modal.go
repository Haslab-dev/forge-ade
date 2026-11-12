package ui

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/icons"
)

// StandardRuleFile represents a known agent guideline file.
type StandardRuleFile struct {
	Path   string
	Exists bool
	Size   int64
}

var supportedRuleFiles = []string{
	"AGENTS.md",
	".cursorrules",
	"CLAUDE.md",
	".github/copilot-instructions.md",
}

// OpenRulesModal initializes the rules list and opens the Unified Rules Manager.
func (a *AppUI) OpenRulesModal() {
	if a.SelectedRulePath == "" {
		a.SelectedRulePath = "AGENTS.md"
	}
	a.loadSelectedRuleContent()
	a.ShowRulesModal = true
}

func (a *AppUI) loadSelectedRuleContent() {
	fullPath := filepath.Join(a.Workspace.RootDir, a.SelectedRulePath)
	data, err := os.ReadFile(fullPath)
	if err == nil {
		a.RuleEditorContent = string(data)
	} else {
		a.RuleEditorContent = ""
	}
}

// SaveActiveRule writes the editor text to the active rule file on disk.
func (a *AppUI) SaveActiveRule(c *ui.Context) {
	if a.SelectedRulePath == "" {
		return
	}
	fullPath := filepath.Join(a.Workspace.RootDir, a.SelectedRulePath)
	dir := filepath.Dir(fullPath)
	_ = os.MkdirAll(dir, 0755)
	err := os.WriteFile(fullPath, []byte(a.RuleEditorContent), 0644)
	if err != nil {
		c.AddToast(ui.Toast{Title: "Failed to Save", Description: err.Error(), Type: "error"})
	} else {
		c.AddToast(ui.Toast{Title: "Saved Rule", Description: fmt.Sprintf("Successfully saved %s", a.SelectedRulePath), Type: "success"})
		a.Workspace.Refresh()
	}
}

// SyncRuleStandard copies instructions from source rule to target rule standard.
func (a *AppUI) SyncRuleStandard(c *ui.Context, src, dst string) {
	srcPath := filepath.Join(a.Workspace.RootDir, src)
	data, err := os.ReadFile(srcPath)
	if err != nil {
		c.AddToast(ui.Toast{Title: "Sync Failed", Description: fmt.Sprintf("Source file %s does not exist", src), Type: "error"})
		return
	}
	dstPath := filepath.Join(a.Workspace.RootDir, dst)
	_ = os.MkdirAll(filepath.Dir(dstPath), 0755)
	err = os.WriteFile(dstPath, data, 0644)
	if err != nil {
		c.AddToast(ui.Toast{Title: "Sync Failed", Description: err.Error(), Type: "error"})
		return
	}
	if a.SelectedRulePath == dst {
		a.RuleEditorContent = string(data)
	}
	c.AddToast(ui.Toast{Title: "Synced Standard", Description: fmt.Sprintf("Copied %s ➔ %s", src, dst), Type: "success"})
	a.Workspace.Refresh()
}

// renderUnifiedRulesModal renders the Unified Project Rules Manager modal.
func (a *AppUI) renderUnifiedRulesModal(c *ui.Context) {
	if !a.ShowRulesModal {
		return
	}

	tc := a.ThemeMgr.Colors()

	ui.Modal(c, &a.ShowRulesModal, func() {
		modal := ui.Column(c).Width(840).Height(580).Radius(16).
			Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).
			Shadow(0, 24, 72, -20, ui.RGBA(0, 0, 0, 0.55)).Clip()

		modal.Children(func() {
			// Header
			ui.Row(c).Height(52).Padding(0, 20).Gap(12).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.6)).
				Background(tc.SurfaceRaised.Alpha(0.5)).Children(func() {
				ui.Box(c).Size(32, 32).Radius(8).AlignItems(ui.Center).Justify(ui.Center).
					Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.25)).Children(func() {
					ui.Icon(c, icons.TablerFileText).Width(16).Height(16).TextColor(tc.Accent)
				})
				ui.Column(c).Grow(1).Gap(1).Children(func() {
					ui.Text(c, "Unified Project Rules Manager").FontSize(14).FontWeight(600).TextColor(tc.Text)
					ui.Text(c, "Manage and sync instruction standards (AGENTS.md, .cursorrules, CLAUDE.md)").
						FontSize(11).TextColor(tc.TextMuted)
				})
				closeBtn := ui.Row(c).Size(28, 28).Radius(6).AlignItems(ui.Center).Justify(ui.Center).
					Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
				closeBtn.Children(func() {
					ui.Icon(c, icons.TablerX).Width(14).Height(14).TextColor(tc.IconMuted)
				})
				if closeBtn.Hovered() {
					closeBtn.Background(tc.Text.Alpha(0.06))
				}
				if closeBtn.Clicked() {
					a.ShowRulesModal = false
				}
			})

			// 2-Column Body
			ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
				// Left Column: Standards list + Quick sync
				ui.Column(c).Width(260).Background(tc.SurfaceRaised.Alpha(0.4)).
					BorderWidth(0, 1, 0, 0).BorderColor(tc.Border.Alpha(0.6)).Padding(12).Gap(10).Children(func() {
					ui.Text(c, "DETECTED STANDARDS").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)

					for _, rf := range supportedRuleFiles {
						thisFile := rf
						fullP := filepath.Join(a.Workspace.RootDir, thisFile)
						fi, err := os.Stat(fullP)
						exists := err == nil && !fi.IsDir()
						isSel := a.SelectedRulePath == thisFile

						item := ui.Row(c.Key("rule-"+thisFile)).Height(34).Radius(8).Padding(0, 10).Gap(8).AlignItems(ui.Center)
						item.Children(func() {
							ui.Icon(c, icons.TablerFileText).Width(13).Height(13).TextColor(tc.IconMuted)
							ui.Text(c, thisFile).FontSize(12).FontWeight(500).Grow(1).SingleLine()
							if exists {
								ui.Row(c).Radius(4).Padding(1, 6).Background(tc.Success.Alpha(0.12)).
									BorderWidth(1).BorderColor(tc.Success.Alpha(0.3)).Children(func() {
									ui.Text(c, "Active").FontSize(9).FontWeight(600).TextColor(tc.Success)
								})
							} else {
								ui.Text(c, "Missing").FontSize(10).TextColor(tc.TextMuted)
							}
						})
						if isSel {
							item.Background(tc.Accent.Alpha(0.15)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.35))
						} else if item.Hovered() {
							item.Background(tc.Text.Alpha(0.05))
						}
						if item.Clicked() {
							a.SelectedRulePath = thisFile
							a.loadSelectedRuleContent()
						}
					}

					// Quick Sync Section
					ui.Column(c).Padding(10, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(tc.Border.Alpha(0.4)).Gap(6).Children(func() {
						ui.Text(c, "QUICK SYNC").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)

						syncCursor := ui.Row(c).Height(30).Radius(6).Padding(0, 8).Gap(6).AlignItems(ui.Center)
						syncCursor.Children(func() {
							ui.Icon(c, icons.TablerArrowsExchange).Width(12).Height(12).TextColor(tc.IconMuted)
							ui.Text(c, "AGENTS.md ➔ .cursorrules").FontSize(11).TextColor(tc.Text)
						})
						if syncCursor.Hovered() {
							syncCursor.Background(tc.Text.Alpha(0.06))
						}
						if syncCursor.Clicked() {
							a.SyncRuleStandard(c, "AGENTS.md", ".cursorrules")
						}

						syncClaude := ui.Row(c).Height(30).Radius(6).Padding(0, 8).Gap(6).AlignItems(ui.Center)
						syncClaude.Children(func() {
							ui.Icon(c, icons.TablerArrowsExchange).Width(12).Height(12).TextColor(tc.IconMuted)
							ui.Text(c, "AGENTS.md ➔ CLAUDE.md").FontSize(11).TextColor(tc.Text)
						})
						if syncClaude.Hovered() {
							syncClaude.Background(tc.Text.Alpha(0.06))
						}
						if syncClaude.Clicked() {
							a.SyncRuleStandard(c, "AGENTS.md", "CLAUDE.md")
						}
					})
				})

				// Right Column: Editor
				ui.Column(c).Grow(1).Children(func() {
					// Editor action bar
					ui.Row(c).Height(40).Padding(0, 16).Gap(8).AlignItems(ui.Center).
						BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.5)).
						Background(tc.SurfaceRaised.Alpha(0.2)).Children(func() {
						ui.Text(c, a.SelectedRulePath).FontSize(12).FontWeight(600).TextColor(tc.Text)
						ui.Box(c).Grow(1)

						saveBtn := ui.Row(c).Height(28).Radius(6).Padding(0, 12).Gap(5).
							AlignItems(ui.Center).Justify(ui.Center).
							Background(tc.Accent).BorderWidth(1).BorderColor(tc.Accent)
						saveBtn.Children(func() {
							ui.Icon(c, icons.TablerDeviceFloppy).Width(12).Height(12).TextColor(ui.RGBA(255, 255, 255, 1.0))
							ui.Text(c, "Save Rule").FontSize(11).FontWeight(600).TextColor(ui.RGBA(255, 255, 255, 1.0))
						})
						if saveBtn.Hovered() {
							saveBtn.Background(tc.Accent.Alpha(0.85))
						}
						if saveBtn.Clicked() {
							a.SaveActiveRule(c)
						}
					})

					// Editor Text Area
					ui.Column(c).Grow(1).Padding(12, 16).Children(func() {
						placeholder := fmt.Sprintf("# Rules for %s\n\nEnter guidelines, tech stack constraints, and coding standards here...", a.SelectedRulePath)
						ui.TextInput(c, &a.RuleEditorContent).Placeholder(placeholder).Grow(1).FillHeight()
					})
				})
			})
		})
	})
}
