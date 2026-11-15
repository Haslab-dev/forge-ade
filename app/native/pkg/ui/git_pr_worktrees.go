package ui

// git_pr_worktrees.go ports GitPullRequestsPane.tsx and GitWorktreesPane.tsx
// onto the shared core/git PR client: PR list/detail/diff/comments/reviews,
// review + merge + lifecycle actions, PR creation, and the linked-worktrees
// manager pane.

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	mygoeditor "github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/hasdev/forge-ade/app/native/pkg/git"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
	internalgit "github.com/hasdev/forge-ade/core/git"
)

// prPaneState holds the PRs tab's data and busy flags (GitPullRequestsPane).
type prPaneState struct {
	Loaded           bool
	Loading          bool
	State            string // "open" | "closed" | "all"
	Items            []internalgit.PullRequestItem
	Selected         int // index into Items, -1 = none
	Detail           *internalgit.PullRequestDetail
	DetailError      string // set when GetPullRequestDetail fails; enables Retry
	DetailDiff       string
	DetailFiles      []mygoeditor.FilePatch
	DetailDiffParsed string
	DiffError        string // set when GetPullRequestDiff fails; enables Retry
	LoadingDiff      bool
	Merging          bool
	Reviewing        bool
	CreateOpen       bool
	CreateTitle      string
	CreateBody       string
	CreateBase       string
	CreateHead       string
	CreateDraft      bool
	Creating         bool
	ReviewBody       string
	MergeMethod      string // "merge" | "squash" | "rebase"
	SearchQuery      string
	Error            string
}

// worktreePaneState holds the Worktrees tab state (GitWorktreesPane).
type worktreePaneState struct {
	Loaded      bool
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

// loadPRs fetches the PR/MR list off the render path. Background goroutines
// never write pane fields directly — results publish through runOnUI so the
// render thread reads consistent state.
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
		a.runOnUI(func() {
			pp.Loading = false
			if err != nil {
				pp.Error = err.Error()
			} else {
				pp.Items = items
				pp.Loaded = true
				pp.Selected = -1
				pp.Detail = nil
				pp.DetailError = ""
				pp.DetailDiff = ""
				pp.DetailFiles = nil
				pp.DetailDiffParsed = ""
				pp.DiffError = ""
			}
		})
	}()
}

// selectPR loads the selected PR's detail and unified diff. Detail and diff
// failures set DetailError / DiffError so the pane shows an honest error
// state with a Retry button instead of an endless spinner.
func (a *AppUI) selectPR(index int) {
	pp := a.prPane()
	gm := a.Git
	if gm == nil || index < 0 || index >= len(pp.Items) {
		return
	}
	pp.Selected = index
	pp.Detail = nil
	pp.DetailError = ""
	pp.DetailDiff = ""
	pp.DetailFiles = nil
	pp.DetailDiffParsed = ""
	pp.DiffError = ""
	pp.LoadingDiff = true
	pr := pp.Items[index]
	if a.Window != nil {
		a.Window.Invalidate()
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		detail, err := gm.Engine().GitEngine.GetPullRequestDetail(ctx, gm.RootDir, pr.Number)
		if err != nil {
			a.runOnUI(func() {
				pp.Detail = nil
				pp.DetailError = err.Error()
				pp.LoadingDiff = false
			})
			return
		}
		diff, derr := gm.Engine().GitEngine.GetPullRequestDiff(ctx, gm.RootDir, pr.Number)
		a.runOnUI(func() {
			pp.Detail = detail
			if derr != nil {
				pp.DiffError = derr.Error()
			} else {
				pp.DetailDiff = diff
			}
			pp.LoadingDiff = false
		})
	}()
}

// prAction runs one review/merge/lifecycle action off the render path.
func (a *AppUI) prAction(label string, run func(ctx context.Context) error) {
	pp := a.prPane()
	gm := a.Git
	if gm == nil || pp.Selected < 0 || pp.Selected >= len(pp.Items) {
		return
	}
	state := pp.State
	pp.Merging = true
	if a.Window != nil {
		a.Window.Invalidate()
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		err := run(ctx)
		a.runOnUI(func() {
			pp.Merging = false
			if err != nil {
				pp.Error = err.Error()
			} else {
				pp.Error = ""
				a.loadPRs(state)
				if label == "Checkout" {
					gm.RefreshAsync()
				}
			}
		})
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
					state := pp.State
					go func() {
						ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
						defer cancel()
						_, err := gm.Engine().GitEngine.CreatePullRequest(ctx, gm.RootDir, title, body, base, head, draft)
						a.runOnUI(func() {
							pp.Creating = false
							if err != nil {
								pp.Error = err.Error()
							} else {
								pp.CreateOpen = false
								pp.CreateTitle, pp.CreateBody, pp.CreateBase, pp.CreateHead = "", "", "", ""
							}
							a.loadPRs(state)
						})
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
				if pp.DetailError != "" {
					// Honest failure state instead of an endless spinner.
					ui.Column(c).Grow(1).Center().Gap(8).Padding(20).Children(func() {
						ui.Icon(c, icons.TablerAlertTriangle).Width(24).Height(24).TextColor(tc.Error)
						ui.Text(c, "Could not load details").FontSize(12).FontWeight(600).TextColor(tc.Text)
						ui.Text(c, pp.DetailError).FontSize(11).TextColor(tc.TextMuted).MaxWidth(320)
						if uiButton(c, tc, "Retry", tc.Accent).Clicked() {
							a.selectPR(pp.Selected)
						}
					})
				} else {
					ui.Column(c).Grow(1).Center().Children(func() {
						ui.Spinner(c).Size(16, 16).Label("Loading details")
					})
				}
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
			ui.Row(c).Gap(2).AlignItems(ui.Center).Children(func() {
				mergeBtn := ui.Row(c.Key("pr-merge-btn")).Height(26).Radius(6).Padding(0, 8).AlignItems(ui.Center).
					Background(tc.Success.Alpha(0.14)).BorderWidth(1).BorderColor(tc.Success.Alpha(0.35)).Cursor(ui.CursorPointer)
				mergeBtn.Children(func() {
					ui.Text(c, "Merge ("+methods.MergeMethod+")").FontSize(11).FontWeight(600).TextColor(tc.Success)
				})
				if mergeBtn.Hovered() {
					mergeBtn.Background(tc.Success.Alpha(0.22))
				}
				if mergeBtn.Clicked() {
					num := pr.Number
					method := methods.MergeMethod
					a.prAction("Merge", func(ctx context.Context) error {
						_, err := gm.Engine().GitEngine.MergePullRequest(ctx, gm.RootDir, num, method)
						return err
					})
				}
				mergeMenuBtn := ui.Row(c.Key("pr-merge-method")).Height(26).Radius(6).Padding(0, 6).AlignItems(ui.Center).
					Background(tc.Success.Alpha(0.14)).BorderWidth(1).BorderColor(tc.Success.Alpha(0.35)).Cursor(ui.CursorPointer)
				mergeMenuBtn.Children(func() {
					ui.Icon(c, icons.TablerChevronDown).Width(11).Height(11).TextColor(tc.Success)
				})
				if mergeMenuBtn.Hovered() {
					mergeMenuBtn.Background(tc.Success.Alpha(0.22))
				}
				mergeMenuBtn.Menu(func(m *ui.Menu) {
					for _, mm := range []string{"merge", "squash", "rebase"} {
						if m.Item(mm).Chosen() {
							methods.MergeMethod = mm
						}
					}
				})
			})
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
			pp := a.prPane()
			if pp.DiffError != "" && pp.DetailDiff == "" {
				// Diff fetch failed — surface the error with a retry rather
				// than silently rendering without the diff.
				ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(4, 0).
					Background(tc.Error.Alpha(0.08)).Radius(6).Children(func() {
					ui.Icon(c, icons.TablerAlertTriangle).Width(13).Height(13).TextColor(tc.Error)
					ui.Text(c, "Diff failed to load: "+pp.DiffError).FontSize(11).TextColor(tc.Error).Grow(1)
					retry := ui.Text(c, "Retry").FontSize(11).FontWeight(600).TextColor(tc.Accent)
					if retry.Clicked() {
						a.selectPR(pp.Selected)
					}
				})
			}
			if pp.DetailDiff != "" {
				ui.Text(c, "Diff").FontSize(12).FontWeight(600)
				if pp.DetailDiffParsed != pp.DetailDiff {
					files, err := mygoeditor.ParsePatch(pp.DetailDiff)
					if err == nil {
						pp.DetailFiles = files
					} else {
						pp.DetailFiles = nil
					}
					pp.DetailDiffParsed = pp.DetailDiff
				}
				files := pp.DetailFiles
				if len(files) > 0 {
					for idx, fp := range files {
						fp := fp
						fileName := fp.NewName
						if fileName == "" {
							fileName = fp.OldName
						}
						key := fmt.Sprintf("pr-%d-f-%d-%s", d.Number, idx, fileName)
						diffState := a.Editor.GetOrCreatePatchDiff(key, fp, a.ThemeMgr)
						ui.Column(c.Key("prdiff-"+key)).Radius(6).BorderWidth(1).BorderColor(tc.Border).Clip().Margin(0, 0, 8, 0).Children(func() {
							ui.Row(c).Padding(6, 10).Background(tc.SurfaceRaised).AlignItems(ui.Center).Children(func() {
								ui.Icon(c, icons.FileIconSVG(fileName)).Width(12).Height(12).TextColor(tc.IconMuted)
								ui.Text(c, fileName).FontSize(11).Font("monospace").FontWeight(600).Margin(0, 0, 0, 6)
							})
							mygoeditor.DiffView(c, diffState).FillWidth()
						})
					}
				} else {
					renderToolOutputBox(c, tc, pp.DetailDiff, false)
				}
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

// renderGitWorktreesSurface is the Worktrees management tab. Worktree execs
// run on goroutines; every pane mutation publishes through runOnUI so the
// render thread never races the worker.
func (a *AppUI) renderGitWorktreesSurface(c *ui.Context, tc theme.ThemeColors, gm *git.GitManager) {
	wt := a.wtPane()
	if !wt.Loaded && !wt.Loading && wt.Error == "" {
		wt.Loading = true
		go func() {
			items, err := gm.ListWorktrees()
			a.runOnUI(func() {
				wt.Loading = false
				if err != nil {
					wt.Error = err.Error()
				} else {
					wt.Items = items
					wt.Loaded = true
					wt.Error = ""
				}
			})
		}()
	}

	ui.Row(c).Height(40).Padding(0, 12).Gap(6).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
		ui.Text(c, "Worktrees").FontSize(12).FontWeight(600)
		ui.Text(c, "Linked checkouts sharing one repository").FontSize(11).TextColor(tc.TextMuted)
		ui.Box(c).Grow(1)
		if wt.Loading || wt.Busy {
			ui.Spinner(c).Size(12, 12).Label("Working")
		}
		pruneBtn := ui.Row(c).Height(26).Radius(6).Padding(0, 10).AlignItems(ui.Center).
			Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border).Cursor(ui.CursorPointer)
		pruneBtn.Children(func() {
			ui.Text(c, "Prune").FontSize(11).FontWeight(500).TextColor(tc.TextMuted)
		})
		if pruneBtn.Hovered() {
			pruneBtn.Background(tc.MessageActionHover)
		}
		if pruneBtn.Clicked() && !wt.Busy {
			wt.Busy = true
			go func() {
				err := gm.PruneWorktrees()
				a.runOnUI(func() {
					wt.Busy = false
					if err != nil {
						wt.Error = err.Error()
					} else {
						wt.Error = ""
						wt.Items = nil
						wt.Loaded = false
					}
				})
			}()
		}
		newBtn := ui.Row(c).Height(26).Radius(6).Padding(0, 10).AlignItems(ui.Center).
			Background(tc.Accent.Alpha(0.14)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.35)).Cursor(ui.CursorPointer)
		newBtn.Children(func() {
			ui.Text(c, "New Worktree").FontSize(11).FontWeight(600).TextColor(tc.Accent)
		})
		if newBtn.Clicked() {
			wt.CreateOpen = !wt.CreateOpen
		}
		if IconButton(c, tc, icons.TablerRefresh, 12).Clicked() {
			wt.Items = nil
			wt.Loaded = false
			wt.Error = ""
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
						a.runOnUI(func() {
							wt.Busy = false
							if err != nil {
								wt.Error = err.Error()
							} else {
								wt.CreateOpen = false
								wt.CreatePath, wt.CreateBr = "", ""
								wt.Items = nil
								wt.Loaded = false
							}
						})
					}()
				}
			})
		})
	}

	rootClean := filepath.Clean(gm.Snapshot().RootDir)
	scroll := ui.Scroll(c)
	scroll.Children(func() {
		col := ui.Column(c).Gap(4).Padding(10)
		col.Children(func() {
			for i, w := range wt.Items {
				w := w
				wClean := filepath.Clean(w.Path)
				isCurrent := wClean == rootClean
				isMain := i == 0 || isCurrent
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
					if isCurrent {
						Badge(c, tc, "Current", tc.Accent)
					} else if isMain {
						Badge(c, tc, "Main", tc.TextMuted)
					}
					if !isCurrent {
						openBtn := ui.Row(c.Key("wt-open-"+w.Path)).Height(24).Radius(6).Padding(0, 8).AlignItems(ui.Center).
							Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3)).Cursor(ui.CursorPointer)
						openBtn.Children(func() {
							ui.Text(c, "Open").FontSize(11).FontWeight(500).TextColor(tc.Accent)
						})
						if openBtn.Hovered() {
							openBtn.Background(tc.Accent.Alpha(0.2))
						}
						if openBtn.Clicked() {
							a.switchWorkspace(w.Path)
						}
					}
					if !isMain {
						del := IconButton(c, tc, icons.TablerTrash, 12)
						if del.Clicked() && !wt.Busy {
							wt.Busy = true
							path := w.Path
							go func() {
								err := gm.RemoveWorktree(path, false)
								a.runOnUI(func() {
									wt.Busy = false
									if err != nil {
										wt.Error = err.Error()
									} else {
										wt.Items = nil
										wt.Loaded = false
									}
								})
							}()
						}
						del.ContextMenu(func(m *ui.Menu) {
							if m.Item("Remove worktree (force)").Danger().Chosen() && !wt.Busy {
								wt.Busy = true
								path := w.Path
								go func() {
									err := gm.RemoveWorktree(path, true)
									a.runOnUI(func() {
										wt.Busy = false
										if err != nil {
											wt.Error = err.Error()
										} else {
											wt.Items = nil
											wt.Loaded = false
										}
									})
								}()
							}
						})
					}
				})
			}
			if wt.Loaded && len(wt.Items) == 0 && !wt.Loading {
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
