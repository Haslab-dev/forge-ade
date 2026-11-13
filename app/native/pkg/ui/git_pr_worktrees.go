package ui

// git_pr_worktrees.go ports GitPullRequestsPane.tsx and GitWorktreesPane.tsx
// onto the shared core/git PR client: PR list/detail/diff/comments/reviews,
// review + merge + lifecycle actions, PR creation, and the linked-worktrees
// manager pane.

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/hasdev/forge-ade/app/native/pkg/git"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
	internalgit "github.com/hasdev/forge-ade/core/git"
)

// prPaneState holds the PRs tab's data and busy flags (GitPullRequestsPane).
type prPaneState struct {
	Loaded      bool
	Loading     bool
	State       string // "open" | "closed" | "all"
	Items       []internalgit.PullRequestItem
	Selected    int // index into Items, -1 = none
	Detail      *internalgit.PullRequestDetail
	DetailDiff  string
	LoadingDiff bool
	Merging     bool
	Reviewing   bool
	CreateOpen  bool
	CreateTitle string
	CreateBody  string
	CreateBase  string
	CreateHead  string
	CreateDraft bool
	Creating    bool
	ReviewBody  string
	MergeMethod string // "merge" | "squash" | "rebase"
	SearchQuery string
	Error       string
}

// worktreePaneState holds the Worktrees tab state (GitWorktreesPane).
type worktreePaneState struct {
	Loading     bool
	Items       []internalgit.GitWorktreeInfo
	CreateOpen  bool
	CreatePath  string
	CreateBr    string
	CreateNewBr bool
	Busy        bool
	Error       string
}

// prPane lazily creates the PR pane state on the AppUI.
func (a *AppUI) prPane() *prPaneState {
	if a.PRPane == nil {
		a.PRPane = &prPaneState{State: "open", Selected: -1, MergeMethod: "merge"}
	}
	return a.PRPane
}

func (a *AppUI) wtPane() *worktreePaneState {
	if a.WTPane == nil {
		a.WTPane = &worktreePaneState{}
	}
	return a.WTPane
}

// loadPRs fetches the PR/MR list off the render path.
func (a *AppUI) loadPRs(state string) {
	pp := a.prPane()
	gm := a.Git
	if gm == nil {
		return
	}
	pp.Loading = true
	pp.Error = ""
	if a.Window != nil {
		a.Window.Invalidate()
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		items, err := gm.Engine().GitEngine.GetPullRequests(ctx, gm.RootDir, state)
		pp.Loading = false
		if err != nil {
			pp.Error = err.Error()
		} else {
			pp.Items = items
			pp.Loaded = true
			pp.Selected = -1
			pp.Detail = nil
		}
		if a.Window != nil {
			a.Window.Invalidate()
		}
	}()
}

// selectPR loads the selected PR's detail and unified diff.
func (a *AppUI) selectPR(index int) {
	pp := a.prPane()
	gm := a.Git
	if gm == nil || index < 0 || index >= len(pp.Items) {
		return
	}
	pp.Selected = index
	pp.Detail = nil
	pp.DetailDiff = ""
	pp.LoadingDiff = true
	pr := pp.Items[index]
	if a.Window != nil {
		a.Window.Invalidate()
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		detail, err := gm.Engine().GitEngine.GetPullRequestDetail(ctx, gm.RootDir, pr.Number)
		pp.Detail = detail
		if err == nil {
			if diff, derr := gm.Engine().GitEngine.GetPullRequestDiff(ctx, gm.RootDir, pr.Number); derr == nil {
				pp.DetailDiff = diff
			}
		}
		pp.LoadingDiff = false
		if a.Window != nil {
			a.Window.Invalidate()
		}
	}()
}

// prAction runs one review/merge/lifecycle action off the render path.
func (a *AppUI) prAction(label string, run func(ctx context.Context) error) {
	pp := a.prPane()
	gm := a.Git
	if gm == nil {
		return
	}
	pr := pp.Items[pp.Selected]
	pp.Merging = true
	if a.Window != nil {
		a.Window.Invalidate()
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		err := run(ctx)
		pp.Merging = false
		if err != nil {
			pp.Error = err.Error()
		} else {
			pp.Error = ""
		}
		if a.Window != nil {
			a.Window.Invalidate()
		}
		_ = pr
	}()
}

// renderGitPRsSurface is the Pull Requests / Merge Requests tab.
func (a *AppUI) renderGitPRsSurface(c *ui.Context, tc theme.ThemeColors, gm *git.GitManager) {
	pp := a.prPane()

	// Load once per activation.
	if !pp.Loaded && !pp.Loading && pp.Error == "" {
		a.loadPRs(pp.State)
	}

	headerLabel := "Pull Requests"
	src, srcErr := gm.RemoteSource()
	if srcErr == nil && src != nil && src.IsGitLab {
		headerLabel = "Merge Requests"
	}

	// Filter row: state segmented + refresh + new PR.
	ui.Row(c).Height(40).Padding(0, 12).Gap(6).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
		ui.Text(c, headerLabel).FontSize(12).FontWeight(600)
		for _, st := range []struct{ id, label string }{{"open", "Open"}, {"closed", "Closed"}, {"all", "All"}} {
			st := st
			active := pp.State == st.id
			b := ui.Row(c.Key("prst-"+st.id)).Height(24).Radius(6).Padding(0, 8).AlignItems(ui.Center)
			b.Children(func() {
				ui.Text(c, st.label).FontSize(11)
			})
			if active {
				b.Background(tc.Accent.Alpha(0.14))
			} else if b.Hovered() {
				b.Background(tc.Text.Alpha(0.05))
			}
			if b.Clicked() {
				pp.State = st.id
				pp.Loaded = false
				a.loadPRs(st.id)
			}
		}
		ui.Box(c).Grow(1)
		if pp.Loading {
			ui.Spinner(c).Size(12, 12).Label("Loading")
		}
		if IconButton(c, tc, icons.TablerRefresh, 12).Clicked() {
			a.loadPRs(pp.State)
		}
		newBtn := ui.Row(c).Height(26).Radius(6).Padding(0, 10).Gap(4).AlignItems(ui.Center).
			Background(tc.Accent.Alpha(0.14)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.35))
		newBtn.Children(func() {
			ui.Text(c, "New "+singularPR(headerLabel)).FontSize(11).FontWeight(600).TextColor(tc.Accent)
		})
		if newBtn.Clicked() {
			pp.CreateOpen = !pp.CreateOpen
		}
	})

	if pp.Error != "" {
		errBox := ui.Column(c).Padding(10, 14).Gap(4).Background(tc.Error.Alpha(0.08))
		errBox.Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons.TablerAlertTriangle).Width(13).Height(13).TextColor(tc.Error)
				ui.Text(c, pp.Error).FontSize(11).TextColor(tc.Error).Grow(1)
			})
			ui.Row(c).Gap(6).Children(func() {
				retry := ui.Text(c, "Retry").FontSize(11).FontWeight(600).TextColor(tc.Accent)
				if retry.Clicked() {
					pp.Error = ""
					a.loadPRs(pp.State)
				}
			})
		})
	}

	// Create form (collapsible).
	if pp.CreateOpen {
		createForm := ui.Column(c).Padding(10, 14).Gap(6).Background(tc.SurfaceRaised.Alpha(0.5)).
			BorderWidth(1, 0, 0, 0).BorderColor(tc.Border)
		createForm.Children(func() {
			ui.Text(c, "New "+singularPR(headerLabel)).FontSize(12).FontWeight(600)
			ui.TextInput(c, &pp.CreateTitle).Placeholder("Title").Grow(1)
			bodyArea := ui.TextAreaBase(c, &pp.CreateBody).Lines(2, 4).Placeholder("Description")
			_ = bodyArea
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.TextInput(c, &pp.CreateBase).Placeholder("base branch").Grow(1)
				ui.Text(c, "←").FontSize(11).TextColor(tc.TextMuted)
				ui.TextInput(c, &pp.CreateHead).Placeholder("head branch").Grow(1)
				draft := ui.Checkbox(c, &pp.CreateDraft, "Draft").FontSize(11)
				_ = draft
			})
			ui.Row(c).Gap(6).Justify(ui.End).AlignItems(ui.Center).Children(func() {
				cancel := composerControlChip(c, tc, icons.TablerX, "Cancel", "", false)
				if cancel.Clicked() {
					pp.CreateOpen = false
				}
				submit := ui.Row(c).Height(28).Radius(8).Padding(0, 14).AlignItems(ui.Center).Justify(ui.Center).Background(tc.Accent)
				submit.Children(func() {
					if pp.Creating {
						ui.Spinner(c).Size(12, 12).Label("Creating")
					} else {
						ui.Text(c, "Create").FontSize(12).FontWeight(600).TextColor(tc.AccentForeground)
					}
				})
				if submit.Clicked() && !pp.Creating && strings.TrimSpace(pp.CreateTitle) != "" {
					pp.Creating = true
					title, body, base, head, draft := pp.CreateTitle, pp.CreateBody, pp.CreateBase, pp.CreateHead, pp.CreateDraft
					go func() {
						ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
						defer cancel()
						_, err := gm.Engine().GitEngine.CreatePullRequest(ctx, gm.RootDir, title, body, base, head, draft)
						pp.Creating = false
						if err != nil {
							pp.Error = err.Error()
						} else {
							pp.CreateOpen = false
							pp.CreateTitle, pp.CreateBody, pp.CreateBase, pp.CreateHead = "", "", "", ""
						}
						a.loadPRs(pp.State)
					}()
				}
			})
		})
	}

	// Two-pane: list + detail. Columns inside a Row need an explicit
	// FillHeight cross-size, or their Scroll grows to content height.
	ui.Row(c).Grow(1).Children(func() {
		// List column
		listCol := ui.Column(c).Width(380).FillHeight().BorderWidth(0, 1, 0, 0).BorderColor(tc.Border)
		listCol.Children(func() {
			scroll := ui.Scroll(c)
			scroll.Children(func() {
				lst := ui.Column(c).Gap(4).Padding(8)
				lst.Children(func() {
					for i, pr := range pp.Items {
						i, pr := i, pr
						active := pp.Selected == i
						row := ui.Row(c.Key(fmt.Sprintf("pr-%d", pr.Number))).Radius(8).Padding(8, 10).Gap(8).AlignItems(ui.Center)
						row.Children(func() {
							stateIcon, stateColor := prStateGlyph(pr.State, pr.IsDraft, tc)
							ui.Icon(c, stateIcon).Width(14).Height(14).TextColor(stateColor)
							col := ui.Column(c).Gap(1)
							col.Children(func() {
								ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
									ui.Text(c, fmt.Sprintf("#%d", pr.Number)).FontSize(11).FontWeight(600).TextColor(tc.TextMuted)
									ui.Text(c, pr.Title).FontSize(12).FontWeight(500).SingleLine().Grow(1)
								})
								ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
									ui.Text(c, pr.Author.Login).FontSize(10).TextColor(tc.TextMuted)
									if pr.HeadRefName != "" {
										ui.Text(c, pr.HeadRefName+" → "+pr.BaseRefName).FontSize(10).Font("monospace").TextColor(tc.SecondaryLabel)
									}
									if pr.Additions > 0 || pr.Deletions > 0 {
										DiffStatText(c, tc, pr.Additions, pr.Deletions)
									}
								})
							})
							ui.Box(c).Grow(1)
						})
						if active {
							row.Background(tc.Accent.Alpha(0.1)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3))
						} else if row.Hovered() {
							row.Background(tc.Text.Alpha(0.05))
						}
						if row.Clicked() {
							a.selectPR(i)
						}
					}
					if len(pp.Items) == 0 && !pp.Loading {
						ui.Column(c).Center().Padding(24).Gap(6).Children(func() {
							ui.Icon(c, icons.TablerGitPullRequest).Width(26).Height(26).TextColor(tc.IconMuted)
							ui.Text(c, "No "+strings.ToLower(headerLabel)).FontSize(12).TextColor(tc.TextMuted)
						})
					}
				})
			}).Grow(1)
		})

		// Detail column
		detailCol := ui.Column(c).Grow(1).FillHeight()
		detailCol.Children(func() {
			if pp.Selected < 0 || pp.Items == nil || pp.Selected >= len(pp.Items) {
				ui.Column(c).Grow(1).Center().Gap(8).Children(func() {
					ui.Icon(c, icons.TablerGitPullRequest).Width(28).Height(28).TextColor(tc.IconMuted)
					ui.Text(c, "Select a "+strings.ToLower(singularPR(headerLabel))).FontSize(12).TextColor(tc.TextMuted)
				})
				return
			}
			pr := pp.Items[pp.Selected]
			if pp.Detail == nil {
				ui.Column(c).Grow(1).Center().Children(func() {
					ui.Spinner(c).Size(16, 16).Label("Loading details")
				})
				return
			}
			a.renderPRDetail(c, tc, gm, pr)
		})
	})
}

// renderPRDetail draws the selected PR: header, actions, body, files, diff,
// comments, reviews.
func (a *AppUI) renderPRDetail(c *ui.Context, tc theme.ThemeColors, gm *git.GitManager, pr internalgit.PullRequestItem) {
	d := a.prPane().Detail

	header := ui.Column(c).Padding(10, 14).Gap(8).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border)
	header.Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			stateIcon, stateColor := prStateGlyph(pr.State, pr.IsDraft, tc)
			ui.Icon(c, stateIcon).Width(16).Height(16).TextColor(stateColor)
			ui.Text(c, fmt.Sprintf("#%d %s", pr.Number, pr.Title)).FontSize(13).FontWeight(600).Grow(1)
			ui.Text(c, pr.State).FontSize(10).FontWeight(600).TextColor(stateColor)
		})
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "@"+pr.Author.Login).FontSize(11).TextColor(tc.TextMuted)
			if d.HeadRefName != "" {
				ui.Text(c, d.HeadRefName+" → "+d.BaseRefName).FontSize(11).Font("monospace").TextColor(tc.SecondaryLabel)
			}
			ui.Box(c).Grow(1)
			// Review + lifecycle actions
			methods := a.prPane()
			mergeRow := ui.Row(c).Height(26).Radius(6).Padding(0, 8).Gap(4).AlignItems(ui.Center).
				Background(tc.Success.Alpha(0.14)).BorderWidth(1).BorderColor(tc.Success.Alpha(0.35))
			mergeRow.Children(func() {
				ui.Text(c, "Merge ("+methods.MergeMethod+")").FontSize(11).FontWeight(600).TextColor(tc.Success)
			})
			mergeRow.Menu(func(m *ui.Menu) {
				for _, mm := range []string{"merge", "squash", "rebase"} {
					if m.Item(mm).Chosen() {
						methods.MergeMethod = mm
					}
				}
			})
			if mergeRow.Clicked() {
				num := pr.Number
				method := methods.MergeMethod
				a.prAction("Merge", func(ctx context.Context) error {
					_, err := gm.Engine().GitEngine.MergePullRequest(ctx, gm.RootDir, num, method)
					return err
				})
			}
			if uiButton(c, tc, "Approve", tc.Success).Clicked() {
				num := pr.Number
				body := a.prPane().ReviewBody
				a.prAction("Approve", func(ctx context.Context) error {
					return gm.Engine().GitEngine.ReviewPullRequest(ctx, gm.RootDir, num, "approve", body)
				})
			}
			if uiButton(c, tc, "Request Changes", tc.Warning).Clicked() {
				num := pr.Number
				body := a.prPane().ReviewBody
				a.prAction("Request Changes", func(ctx context.Context) error {
					return gm.Engine().GitEngine.ReviewPullRequest(ctx, gm.RootDir, num, "request-changes", body)
				})
			}
			if uiButton(c, tc, "Checkout", tc.Accent).Clicked() {
				num := pr.Number
				a.prAction("Checkout", func(ctx context.Context) error {
					_, err := gm.Engine().GitEngine.CheckoutPullRequest(ctx, gm.RootDir, num)
					return err
				})
			}
			if pr.State == "OPEN" {
				if uiButton(c, tc, "Close", tc.Error).Clicked() {
					num := pr.Number
					a.prAction("Close", func(ctx context.Context) error {
						return gm.Engine().GitEngine.ClosePullRequest(ctx, gm.RootDir, num)
					})
				}
			} else {
				if uiButton(c, tc, "Reopen", tc.Success).Clicked() {
					num := pr.Number
					a.prAction("Reopen", func(ctx context.Context) error {
						return gm.Engine().GitEngine.ReopenPullRequest(ctx, gm.RootDir, num)
					})
				}
			}
		})
		// Review comment box
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.TextInput(c, &a.prPane().ReviewBody).Placeholder("Review comment (optional)").Grow(1)
		})
	})

	scroll := ui.Scroll(c)
	scroll.Children(func() {
		col := ui.Column(c).Gap(10).Padding(12)
		col.Children(func() {
			if d.Body != "" {
				markdown.View(c, d.Body, &a.Agent.MdOpts)
			}
			// Files + diff
			if len(d.Files) > 0 {
				ui.Text(c, fmt.Sprintf("Files changed (%d)", len(d.Files))).FontSize(12).FontWeight(600)
				for _, f := range d.Files {
					f := f
					ui.Row(c.Key("prf-"+f.Path)).Gap(6).AlignItems(ui.Center).Padding(2, 0).Children(func() {
						ui.Icon(c, icons.FileIconSVG(f.Path)).Width(12).Height(12).TextColor(tc.IconMuted)
						ui.Text(c, f.Path).FontSize(11).Font("monospace").Grow(1)
						DiffStatText(c, tc, f.Additions, f.Deletions)
					})
				}
			}
			if a.prPane().DetailDiff != "" {
				ui.Text(c, "Diff").FontSize(12).FontWeight(600)
				renderToolOutputBox(c, tc, a.prPane().DetailDiff, false)
			}
			// Comments
			if len(d.Comments) > 0 {
				ui.Text(c, fmt.Sprintf("Comments (%d)", len(d.Comments))).FontSize(12).FontWeight(600)
				for _, cm := range d.Comments {
					cm := cm
					cbox := ui.Column(c.Key("prc-"+cm.ID)).Padding(8, 10).Radius(8).Gap(3).
						Background(tc.SurfaceRaised.Alpha(0.5)).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5))
					cbox.Children(func() {
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							ui.Text(c, "@"+cm.Author.Login).FontSize(11).FontWeight(600)
							ui.Text(c, relativeTime(parseRFC3339(cm.CreatedAt))).FontSize(10).TextColor(tc.TextMuted)
						})
						markdown.View(c, cm.Body, &a.Agent.MdOpts)
					})
				}
			}
			// Reviews
			if len(d.Reviews) > 0 {
				ui.Text(c, fmt.Sprintf("Reviews (%d)", len(d.Reviews))).FontSize(12).FontWeight(600)
				for _, rv := range d.Reviews {
					rv := rv
					color := tc.TextMuted
					switch rv.State {
					case "APPROVED":
						color = tc.Success
					case "CHANGES_REQUESTED":
						color = tc.Warning
					}
					ui.Row(c.Key("prr-"+rv.ID)).Gap(6).AlignItems(ui.Center).Padding(2, 0).Children(func() {
						ui.Box(c).Size(7, 7).Radius(4).Background(color)
						ui.Text(c, "@"+rv.Author.Login).FontSize(11).FontWeight(500)
						ui.Text(c, rv.State).FontSize(10).FontWeight(600).TextColor(color)
					})
				}
			}
		})
	}).Grow(1)
}

// renderGitWorktreesSurface is the Worktrees management tab.
func (a *AppUI) renderGitWorktreesSurface(c *ui.Context, tc theme.ThemeColors, gm *git.GitManager) {
	wt := a.wtPane()
	if len(wt.Items) == 0 && !wt.Loading {
		wt.Loading = true
		go func() {
			items, err := gm.ListWorktrees()
			wt.Loading = false
			if err != nil {
				wt.Error = err.Error()
			} else {
				wt.Items = items
				wt.Error = ""
			}
			if a.Window != nil {
				a.Window.Invalidate()
			}
		}()
	}

	ui.Row(c).Height(40).Padding(0, 12).Gap(6).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
		ui.Text(c, "Worktrees").FontSize(12).FontWeight(600)
		ui.Text(c, "Linked checkouts sharing one repository").FontSize(11).TextColor(tc.TextMuted)
		ui.Box(c).Grow(1)
		if wt.Loading {
			ui.Spinner(c).Size(12, 12).Label("Loading")
		}
		newBtn := ui.Row(c).Height(26).Radius(6).Padding(0, 10).AlignItems(ui.Center).
			Background(tc.Accent.Alpha(0.14)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.35))
		newBtn.Children(func() {
			ui.Text(c, "New Worktree").FontSize(11).FontWeight(600).TextColor(tc.Accent)
		})
		if newBtn.Clicked() {
			wt.CreateOpen = !wt.CreateOpen
		}
		if IconButton(c, tc, icons.TablerRefresh, 12).Clicked() {
			wt.Items = nil
		}
	})

	if wt.Error != "" {
		ui.Row(c).Padding(8, 14).Background(tc.Error.Alpha(0.08)).Children(func() {
			ui.Text(c, wt.Error).FontSize(11).TextColor(tc.Error).Grow(1)
		})
	}

	if wt.CreateOpen {
		form := ui.Column(c).Padding(10, 14).Gap(6).Background(tc.SurfaceRaised.Alpha(0.5)).
			BorderWidth(1, 0, 0, 0).BorderColor(tc.Border)
		form.Children(func() {
			ui.Text(c, "New worktree").FontSize(12).FontWeight(600)
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.TextInput(c, &wt.CreatePath).Placeholder("path (e.g. ../repo-feature)").Grow(1)
				ui.TextInput(c, &wt.CreateBr).Placeholder("branch").Grow(1)
			})
			ui.Row(c).Gap(6).Justify(ui.End).AlignItems(ui.Center).Children(func() {
				newBr := ui.Checkbox(c, &wt.CreateNewBr, "create branch").FontSize(11)
				_ = newBr
				submit := ui.Row(c).Height(26).Radius(6).Padding(0, 12).AlignItems(ui.Center).Background(tc.Accent)
				submit.Children(func() {
					ui.Text(c, "Create").FontSize(11).FontWeight(600).TextColor(tc.AccentForeground)
				})
				if submit.Clicked() && !wt.Busy && strings.TrimSpace(wt.CreatePath) != "" {
					wt.Busy = true
					path, br, createNew := wt.CreatePath, wt.CreateBr, wt.CreateNewBr
					go func() {
						err := gm.CreateWorktree(path, br, createNew)
						wt.Busy = false
						if err != nil {
							wt.Error = err.Error()
						} else {
							wt.CreateOpen = false
							wt.CreatePath, wt.CreateBr = "", ""
							wt.Items = nil // reload
						}
						if a.Window != nil {
							a.Window.Invalidate()
						}
					}()
				}
			})
		})
	}

	scroll := ui.Scroll(c)
	scroll.Children(func() {
		col := ui.Column(c).Gap(4).Padding(10)
		col.Children(func() {
			for _, w := range wt.Items {
				w := w
				row := ui.Row(c.Key("wt-"+w.Path)).Radius(8).Padding(8, 10).Gap(8).AlignItems(ui.Center).
					Background(tc.SurfaceRaised.Alpha(0.4)).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5))
				row.Children(func() {
					ui.Icon(c, icons.TablerFolder).Width(13).Height(13).TextColor(tc.IconMuted)
					col := ui.Column(c).Gap(1)
					col.Children(func() {
						ui.Text(c, w.Path).FontSize(12).FontWeight(500).SingleLine()
						ui.Text(c, w.Branch).FontSize(10).Font("monospace").TextColor(tc.TextMuted)
					})
					ui.Box(c).Grow(1)
					del := IconButton(c, tc, icons.TablerTrash, 12)
					if del.Clicked() {
						wt.Busy = true
						path := w.Path
						go func() {
							err := gm.RemoveWorktree(path, false)
							wt.Busy = false
							if err != nil {
								wt.Error = err.Error()
							} else {
								wt.Items = nil
							}
							if a.Window != nil {
								a.Window.Invalidate()
							}
						}()
					}
					del.ContextMenu(func(m *ui.Menu) {
						if m.Item("Remove worktree (force)").Danger().Chosen() {
							wt.Busy = true
							path := w.Path
							go func() {
								err := gm.RemoveWorktree(path, true)
								wt.Busy = false
								if err != nil {
									wt.Error = err.Error()
								} else {
									wt.Items = nil
								}
								if a.Window != nil {
									a.Window.Invalidate()
								}
							}()
						}
					})
				})
			}
			if len(wt.Items) == 0 && !wt.Loading {
				ui.Column(c).Center().Padding(24).Gap(6).Children(func() {
					ui.Icon(c, icons.TablerFolder).Width(26).Height(26).TextColor(tc.IconMuted)
					ui.Text(c, "No linked worktrees").FontSize(12).TextColor(tc.TextMuted)
				})
			}
		})
	}).Grow(1)
}

// prStateGlyph picks the state icon + color (open/closed/merged/draft).
func prStateGlyph(state string, draft bool, tc theme.ThemeColors) (*ui.SVG, ui.Color) {
	switch strings.ToUpper(state) {
	case "MERGED":
		return icons.TablerGitMerge, tc.Success
	case "CLOSED":
		return icons.TablerGitPullRequestClosed, tc.Error
	default:
		if draft {
			return icons.TablerGitPullRequestDraft, tc.TextMuted
		}
		return icons.TablerGitPullRequest, tc.Success
	}
}

// singularPR trims a plural header label for single-item copy.
func singularPR(label string) string {
	return strings.TrimSuffix(label, "s")
}

// parseRFC3339 parses remote timestamps defensively (zero time on error).
func parseRFC3339(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// uiButton is a small tinted action button used in the PR detail toolbar.
func uiButton(c *ui.Context, tc theme.ThemeColors, label string, color ui.Color) ui.Element {
	b := ui.Row(c.Key("uibtn-"+label)).Height(26).Radius(6).Padding(0, 10).AlignItems(ui.Center).Justify(ui.Center).
		Background(color.Alpha(0.12)).BorderWidth(1).BorderColor(color.Alpha(0.35))
	b.Children(func() {
		ui.Text(c, label).FontSize(11).FontWeight(600).TextColor(color)
	})
	if b.Hovered() {
		b.Background(color.Alpha(0.2))
	}
	return b
}
