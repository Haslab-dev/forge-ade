package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
)

// OpenHandoffModal prepares and opens the Orca-inspired Session/ACP Handoff dialog.
func (a *AppUI) OpenHandoffModal() {
	if a.Agent == nil {
		return
	}
	sess := a.Agent.ActiveSession()
	if sess != nil {
		a.HandoffSourceID = sess.ID
		a.HandoffGoal = sess.Title
	} else {
		a.HandoffSourceID = ""
		a.HandoffGoal = ""
	}
	a.HandoffNextSteps = ""
	a.HandoffCreateWorktree = false
	a.HandoffWorktreeBranch = ""
	a.HandoffError = ""
	if a.HandoffTargetAgentID == "" {
		if sess != nil && sess.AgentType == "internal" {
			a.HandoffTargetAgentID = "agent-claude"
			a.HandoffTargetAgentType = "acp"
		} else {
			a.HandoffTargetAgentID = "agent-internal"
			a.HandoffTargetAgentType = "internal"
		}
	}
	a.ShowHandoffModal = true
}

func (a *AppUI) renderHandoffModal(c *ui.Context) {
	if !a.ShowHandoffModal {
		return
	}
	tc := a.ThemeMgr.Colors()

	type agentChoice struct {
		ID        string
		Name      string
		Type      string
		Model     string
		Available bool
	}
	choices := []agentChoice{
		{ID: "agent-internal", Name: "ForgeADE Internal", Type: "internal", Available: true},
	}
	if a.ACPMgr != nil {
		for _, def := range a.ACPMgr.Agents {
			if def.Type == "internal" || def.ID == "agent-internal" {
				continue
			}
			choices = append(choices, agentChoice{
				ID:        def.ID,
				Name:      def.Name,
				Type:      "acp",
				Model:     def.Model,
				Available: def.Available,
			})
		}
	}

	a.modal(c, &a.ShowHandoffModal, func() {
		modal := ui.Column(c).Width(500).Radius(14).
			Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).
			Shadow(0, 20, 56, -16, ui.RGBA(0, 0, 0, 0.55)).Clip()

		modal.Children(func() {
			// Header
			ui.Row(c).Padding(14, 18).Gap(10).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
				ui.Icon(c, icons.TablerGitFork).Width(16).Height(16).TextColor(tc.Accent)
				ui.Column(c).Grow(1).Gap(1).Children(func() {
					ui.Text(c, "Hand off session").FontSize(14).FontWeight(600).TextColor(tc.Text)
					ui.Text(c, "Transfer goal, progress & git diff to another agent or worktree").FontSize(11).TextColor(tc.TextMuted)
				})
				closeBtn := ui.Row(c.Key("ho-close")).Size(24, 24).Radius(6).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
				closeBtn.Children(func() {
					ui.Icon(c, icons.TablerX).Width(13).Height(13).TextColor(tc.IconMuted)
				})
				if closeBtn.Hovered() {
					closeBtn.Background(tc.Text.Alpha(0.08))
				}
				if closeBtn.Clicked() {
					a.ShowHandoffModal = false
				}
			})

			// Body
			ui.Column(c).Padding(16, 18).Gap(12).Children(func() {
				// Target agent grid
				ui.Text(c, "Target Agent").FontSize(11.5).FontWeight(600).TextColor(tc.TextMuted)
				ui.Row(c).Gap(6).Wrap().Children(func() {
					for _, ch := range choices {
						opt := ch
						selected := a.HandoffTargetAgentID == opt.ID
						pill := ui.Row(c.Key("ho-ag-"+opt.ID)).Height(30).Radius(7).Padding(0, 10).Gap(6).
							AlignItems(ui.Center).Cursor(ui.CursorPointer).
							BorderWidth(1)
						if selected {
							pill.Background(tc.Accent.Alpha(0.16)).BorderColor(tc.Accent.Alpha(0.55))
						} else {
							pill.Background(tc.SurfaceRaised).BorderColor(tc.Border)
						}
						pill.Children(func() {
							if opt.Type == "internal" {
								ui.Icon(c, icons.ProviderForgeADE).Width(13).Height(13)
							} else {
								icons.RenderProviderIcon(c, opt.ID+" "+opt.Name, 13)
							}
							txt := ui.Text(c, opt.Name).FontSize(12).SingleLine()
							if selected {
								txt.FontWeight(600).TextColor(tc.Text)
							} else {
								txt.FontWeight(500).TextColor(tc.TextMuted)
							}
							if !opt.Available {
								ui.Text(c, "CLI").FontSize(9.5).TextColor(tc.TextMuted.Alpha(0.6))
							}
						})
						if pill.Hovered() && !selected {
							pill.Background(tc.ToolbarControlHover)
						}
						if pill.Clicked() {
							a.HandoffTargetAgentID = opt.ID
							a.HandoffTargetAgentType = opt.Type
							a.HandoffTargetModel = opt.Model
						}
					}
				})

				// Goal input
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "Goal / Done means").FontSize(11.5).FontWeight(600).TextColor(tc.TextMuted)
					ui.TextInput(c, &a.HandoffGoal).Placeholder("What should the receiving agent accomplish?").FillWidth()
				})

				// Next instructions input
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "Next Instructions (optional)").FontSize(11.5).FontWeight(600).TextColor(tc.TextMuted)
					ui.TextAreaBase(c, &a.HandoffNextSteps).Lines(2, 4).Placeholder("Specific files, test commands, or constraints to pass along...").FillWidth()
				})

				// Optional isolated git worktree toggle
				ui.Column(c).Gap(6).Children(func() {
					wtToggle := ui.Row(c.Key("ho-wt-toggle")).Height(28).Radius(7).Padding(0, 8).Gap(8).
						AlignItems(ui.Center).Cursor(ui.CursorPointer).
						Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
					wtToggle.Children(func() {
						box := ui.Box(c).Size(14, 14).Radius(4).AlignItems(ui.Center).Justify(ui.Center).BorderWidth(1)
						if a.HandoffCreateWorktree {
							box.Background(tc.Accent).BorderColor(tc.Accent).Children(func() {
								ui.Icon(c, icons.TablerCheck).Width(10).Height(10).TextColor(tc.AccentForeground)
							})
						} else {
							box.BorderColor(tc.Border)
						}
						ui.Text(c, "Create isolated git worktree").FontSize(12).FontWeight(500).TextColor(tc.Text).Grow(1)
					})
					if wtToggle.Clicked() {
						a.HandoffCreateWorktree = !a.HandoffCreateWorktree
					}
					if a.HandoffCreateWorktree {
						ui.TextInput(c, &a.HandoffWorktreeBranch).Placeholder("Branch name (e.g. feature/handoff-review)").FillWidth()
					}
				})

				if a.HandoffError != "" {
					ui.Text(c, a.HandoffError).FontSize(11.5).TextColor(tc.Error)
				}
			})

			// Footer actions
			ui.Row(c).Padding(12, 18).Gap(8).Justify(ui.End).AlignItems(ui.Center).
				BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {
				cancelBtn := ui.Row(c.Key("ho-cancel")).Height(30).Radius(7).Padding(0, 12).
					AlignItems(ui.Center).Cursor(ui.CursorPointer).
					Background(tc.ToolbarControl).BorderWidth(1).BorderColor(tc.Border)
				cancelBtn.Children(func() {
					ui.Text(c, "Cancel").FontSize(12).FontWeight(500).TextColor(tc.Text)
				})
				if cancelBtn.Clicked() {
					a.ShowHandoffModal = false
				}

				goBtn := ui.Row(c.Key("ho-submit")).Height(30).Radius(7).Padding(0, 14).Gap(6).
					AlignItems(ui.Center).Cursor(ui.CursorPointer).
					Background(tc.MessageAction)
				goBtn.Children(func() {
					ui.Text(c, "Hand off").FontSize(12).FontWeight(600).TextColor(tc.MessageActionForeground)
					ui.Icon(c, icons.TablerExternalLink).Width(12).Height(12).TextColor(tc.MessageActionForeground)
				})
				if goBtn.Clicked() {
					sess, err := a.Agent.ExecuteHandoff(agent.HandoffRequest{
						SourceSessionID: a.HandoffSourceID,
						TargetAgentID:   a.HandoffTargetAgentID,
						TargetAgentType: a.HandoffTargetAgentType,
						TargetModel:     strings.TrimSpace(a.HandoffTargetModel),
						Goal:            strings.TrimSpace(a.HandoffGoal),
						NextSteps:       strings.TrimSpace(a.HandoffNextSteps),
						CreateWorktree:  a.HandoffCreateWorktree,
						WorktreeBranch:  strings.TrimSpace(a.HandoffWorktreeBranch),
						AutoStart:       true,
					})
					if err != nil {
						a.HandoffError = fmt.Sprintf("Handoff failed: %v", err)
					} else {
						a.ShowHandoffModal = false
						if sess != nil && sess.WorkspacePath != "" && a.Workspace != nil && !sameWorkspacePath(a.Workspace.RootDir, sess.WorkspacePath) {
							a.switchWorkspace(sess.WorkspacePath)
						}
						c.AddToast(ui.Toast{Title: "Session Handed Off", Description: "Started " + sess.Title, Type: "success"})
					}
				}
			})
		})
	})
}
