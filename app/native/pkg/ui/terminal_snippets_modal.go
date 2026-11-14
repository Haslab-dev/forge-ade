package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/terminal"
)

// OpenTerminalSnippetsModal opens the snippets dialog with optional draft command.
func (a *AppUI) OpenTerminalSnippetsModal(targetSessionID, draftCommand, draftTitle string) {
	a.ShowTerminalSnippetsModal = true
	a.TerminalSnippetTab = "snippets"
	a.TerminalSnippetTargetSession = targetSessionID
	a.TerminalSnippetSearch = ""
	a.TerminalSnippetCategory = "all"
	a.TerminalSnippetDraftCmd = draftCommand
	a.TerminalSnippetDraftTitle = draftTitle
	a.TerminalSnippetDraftCat = "custom"
	a.TerminalSnippetDraftDesc = ""
	a.TerminalSnippetEditID = ""
	if draftCommand != "" {
		a.TerminalSnippetFormOpen = true
		if draftTitle == "" {
			a.TerminalSnippetDraftTitle = "New Command Snippet"
		}
	} else {
		a.TerminalSnippetFormOpen = false
	}
	if a.Window != nil {
		a.Window.Invalidate()
	}
}

// renderTerminalSnippetsModal renders the 1:1 TerminalSnippetModal dialog.
func (a *AppUI) renderTerminalSnippetsModal(c *ui.Context) {
	if !a.ShowTerminalSnippetsModal {
		return
	}
	tc := a.ThemeMgr.Colors()

	if a.Snippets == nil {
		dataDir := ""
		if a.Engine != nil {
			dataDir = a.Engine.DataDir
		}
		if dataDir == "" {
			h, _ := os.UserHomeDir()
			dataDir = h + "/.forge-ade"
		}
		a.Snippets = terminal.NewSnippetManager(dataDir)
	}
	sm := a.Snippets

	if c.Shortcut(0, ui.KeyEscape) {
		a.ShowTerminalSnippetsModal = false
		return
	}

	// Modal backdrop
	ui.Column(c).Fill().Background(ui.RGBA(0, 0, 0, 0.55)).Center().Children(func() {
		// Modal window container
		modal := ui.Column(c).Width(680).MaxHeight(600).Radius(12).
			Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border).
			Shadow(0, 16, 48, -12, ui.RGBA(0, 0, 0, 0.65)).Clip()

		modal.Children(func() {
			// 1. Header Bar: Title, Tabs ("Snippets" / "History"), Close button
			ui.Row(c).Height(48).Padding(0, 16).Gap(12).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
				ui.Icon(c, icons.TablerTerminal).Width(16).Height(16).TextColor(tc.Accent)
				ui.Text(c, "Terminal Snippets & History").FontSize(13).FontWeight(600).TextColor(tc.Text)

				ui.Box(c).Grow(1)

				// Segmented Tab switcher
				snipActive := (a.TerminalSnippetTab != "history")
				histActive := (a.TerminalSnippetTab == "history")

				tabBtn := func(label string, active bool, onPick func()) {
					pill := ui.Row(c.Key("tab-"+label)).Height(26).Radius(6).Padding(0, 10).AlignItems(ui.Center)
					pill.Children(func() {
						txt := ui.Text(c, label).FontSize(11)
						if active {
							txt.FontWeight(600).TextColor(tc.Text)
						} else {
							txt.TextColor(tc.TextMuted)
						}
					})
					if active {
						pill.Background(tc.Text.Alpha(0.1))
					} else if pill.Hovered() {
						pill.Background(tc.Text.Alpha(0.04))
					}
					if pill.Clicked() {
						onPick()
					}
				}

				tabBtn("Snippets", snipActive, func() { a.TerminalSnippetTab = "snippets" })
				tabBtn("History", histActive, func() { a.TerminalSnippetTab = "history" })

				closeBtn := ui.Row(c).Size(26, 26).Radius(6).AlignItems(ui.Center).Justify(ui.Center)
				closeBtn.Children(func() {
					ui.Icon(c, icons.TablerX).Width(14).Height(14).TextColor(tc.IconMuted)
				})
				if closeBtn.Hovered() {
					closeBtn.Background(tc.Text.Alpha(0.08))
				}
				if closeBtn.Clicked() {
					a.ShowTerminalSnippetsModal = false
				}
			})

			// 2. Toolbar & Category Bar (if Snippets tab)
			ui.Row(c).Height(40).Padding(0, 14).Gap(10).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.5)).Children(func() {
				// Search Input
				ui.Row(c).Width(240).Height(28).Radius(6).Padding(0, 8).Gap(6).AlignItems(ui.Center).
					Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border).Children(func() {
					ui.Icon(c, icons.TablerSearch).Width(12).Height(12).TextColor(tc.IconMuted)
					ui.TextInput(c, &a.TerminalSnippetSearch).Placeholder("Filter snippets & commands…").Grow(1).FontSize(11)
				})

				if a.TerminalSnippetTab != "history" {
					// Category filter pills
					categories := []string{"all", "remote", "docker", "git", "system", "dev", "custom"}
					for _, cat := range categories {
						thisCat := cat
						active := (a.TerminalSnippetCategory == thisCat || (a.TerminalSnippetCategory == "" && thisCat == "all"))
						cpill := ui.Row(c.Key("cat-"+thisCat)).Height(24).Radius(5).Padding(0, 7).AlignItems(ui.Center)
						cpill.Children(func() {
							txt := ui.Text(c, strings.ToUpper(thisCat[:1])+thisCat[1:]).FontSize(10)
							if active {
								txt.FontWeight(600).TextColor(tc.Accent)
							} else {
								txt.TextColor(tc.TextMuted)
							}
						})
						if active {
							cpill.Background(tc.Accent.Alpha(0.12))
						} else if cpill.Hovered() {
							cpill.Background(tc.Text.Alpha(0.04))
						}
						if cpill.Clicked() {
							a.TerminalSnippetCategory = thisCat
						}
					}

					ui.Box(c).Grow(1)

					// + New Snippet Button
					newBtn := ui.Row(c).Height(26).Radius(6).Padding(0, 9).Gap(4).AlignItems(ui.Center).
						Background(tc.Accent.Alpha(0.15)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.35))
					newBtn.Children(func() {
						ui.Icon(c, icons.TablerPlus).Width(11).Height(11).TextColor(tc.Accent)
						ui.Text(c, "New").FontSize(11).FontWeight(600).TextColor(tc.Accent)
					})
					if newBtn.Clicked() {
						a.TerminalSnippetFormOpen = !a.TerminalSnippetFormOpen
						a.TerminalSnippetEditID = ""
						if a.TerminalSnippetFormOpen {
							a.TerminalSnippetDraftTitle = ""
							a.TerminalSnippetDraftCmd = ""
							a.TerminalSnippetDraftDesc = ""
							a.TerminalSnippetDraftCat = "custom"
						}
					}
				} else {
					ui.Box(c).Grow(1)
					// Clear History Button
					clearBtn := ui.Row(c).Height(24).Radius(5).Padding(0, 8).AlignItems(ui.Center).
						Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
					clearBtn.Children(func() {
						ui.Text(c, "Clear History").FontSize(10).TextColor(tc.TextMuted)
					})
					if clearBtn.Clicked() {
						sm.ClearHistory()
					}
				}
			})

			// 3. New / Edit Snippet Form Drawer (collapsible)
			if a.TerminalSnippetTab != "history" && a.TerminalSnippetFormOpen {
				ui.Column(c).Padding(12, 16).Gap(8).Background(tc.Canvas).
					BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
					titleLabel := "Create Custom Snippet"
					if a.TerminalSnippetEditID != "" {
						titleLabel = "Edit Snippet"
					}
					ui.Text(c, titleLabel).FontSize(11).FontWeight(600).TextColor(tc.Accent)

					ui.Row(c).Gap(8).Children(func() {
						ui.TextInput(c, &a.TerminalSnippetDraftTitle).Placeholder("Snippet title (e.g. Docker logs)").Grow(2).FontSize(11)
						ui.TextInput(c, &a.TerminalSnippetDraftCat).Placeholder("Category (e.g. docker, git, remote)").Grow(1).FontSize(11)
					})
					ui.TextInput(c, &a.TerminalSnippetDraftCmd).Placeholder("Command string (e.g. docker logs -f <container>)").Grow(1).FontSize(11)
					ui.TextInput(c, &a.TerminalSnippetDraftDesc).Placeholder("Description (optional)").Grow(1).FontSize(11)

					ui.Row(c).Gap(6).Justify(ui.End).Children(func() {
						cancelBtn := ui.Row(c).Height(24).Radius(5).Padding(0, 8).AlignItems(ui.Center).
							Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
						cancelBtn.Children(func() {
							ui.Text(c, "Cancel").FontSize(10).TextColor(tc.TextMuted)
						})
						if cancelBtn.Clicked() {
							a.TerminalSnippetFormOpen = false
						}

						saveBtn := ui.Row(c).Height(24).Radius(5).Padding(0, 10).AlignItems(ui.Center).
							Background(tc.Accent)
						saveBtn.Children(func() {
							ui.Text(c, "Save Snippet").FontSize(10).FontWeight(600).TextColor(tc.Canvas)
						})
						if saveBtn.Clicked() && strings.TrimSpace(a.TerminalSnippetDraftTitle) != "" && strings.TrimSpace(a.TerminalSnippetDraftCmd) != "" {
							if a.TerminalSnippetEditID != "" {
								sm.UpdateSnippet(a.TerminalSnippetEditID, a.TerminalSnippetDraftTitle, a.TerminalSnippetDraftCmd, a.TerminalSnippetDraftCat, a.TerminalSnippetDraftDesc)
							} else {
								sm.AddSnippet(a.TerminalSnippetDraftTitle, a.TerminalSnippetDraftCmd, a.TerminalSnippetDraftCat, a.TerminalSnippetDraftDesc)
							}
							a.TerminalSnippetFormOpen = false
							c.AddToast(ui.Toast{Title: "Snippet Saved", Type: "success"})
						}
					})
				})
			}

			// 4. Main Body: Scrollable list of Snippets or History Items
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Padding(10, 14).Gap(6).Children(func() {
					if a.TerminalSnippetTab == "history" {
						// Render History Items
						histItems := sm.HistoryItems(a.TerminalSnippetSearch)
						if len(histItems) == 0 {
							ui.Column(c).Center().Padding(30).Gap(4).Children(func() {
								ui.Icon(c, icons.TablerHistory).Width(24).Height(24).TextColor(tc.IconMuted)
								ui.Text(c, "No command history").FontSize(12).TextColor(tc.TextMuted)
							})
						}
						for _, it := range histItems {
							thisIt := it
							row := ui.Row(c.Key("hist-"+thisIt.ID)).Radius(8).Padding(8, 10).Gap(10).AlignItems(ui.Center).
								Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6))

							row.Children(func() {
								// Favorite Star Button
								favBtn := ui.Row(c.Key("fav-"+thisIt.ID)).Size(22, 22).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
								favBtn.Children(func() {
									starIcon := icons.TablerStar
									starColor := tc.IconMuted
									if thisIt.IsFavorite {
										starColor = tc.Warning
									}
									ui.Icon(c, starIcon).Width(13).Height(13).TextColor(starColor)
								})
								if favBtn.Clicked() {
									sm.ToggleFavorite(thisIt.ID)
								}

								ui.Column(c).Grow(1).Gap(2).Children(func() {
									ui.Text(c, thisIt.Command).FontSize(11).Font("monospace").TextColor(tc.Text).SingleLine()
									timeStr := thisIt.Timestamp.Format("15:04:05")
									if !thisIt.Timestamp.IsZero() {
										dur := time.Since(thisIt.Timestamp)
										if dur < time.Minute {
											timeStr = "just now"
										} else if dur < time.Hour {
											timeStr = fmt.Sprintf("%dm ago", int(dur.Minutes()))
										}
									}
									ui.Text(c, timeStr).FontSize(9).TextColor(tc.TextMuted)
								})

								// Run Button
								runBtn := ui.Row(c.Key("run-"+thisIt.ID)).Height(24).Radius(5).Padding(0, 8).Gap(4).AlignItems(ui.Center).
									Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3))
								runBtn.Children(func() {
									ui.Icon(c, icons.TablerPlayerPlay).Width(10).Height(10).TextColor(tc.Accent)
									ui.Text(c, "Run").FontSize(10).FontWeight(600).TextColor(tc.Accent)
								})
								if runBtn.Clicked() {
									a.runTerminalCommand(thisIt.Command)
									a.ShowTerminalSnippetsModal = false
								}

								// Delete Button
								delBtn := ui.Row(c.Key("del-"+thisIt.ID)).Size(22, 22).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
								delBtn.Children(func() {
									ui.Icon(c, icons.TablerTrash).Width(12).Height(12).TextColor(tc.IconMuted)
								})
								if delBtn.Clicked() {
									sm.DeleteHistory(thisIt.ID)
								}
							})
							if row.Hovered() {
								row.Background(tc.Text.Alpha(0.04))
							}
						}
					} else {
						// Render Snippets
						snips := sm.FilterSnippets(a.TerminalSnippetCategory, a.TerminalSnippetSearch)
						if len(snips) == 0 {
							ui.Column(c).Center().Padding(30).Gap(4).Children(func() {
								ui.Icon(c, icons.TablerBook).Width(24).Height(24).TextColor(tc.IconMuted)
								ui.Text(c, "No snippets match query").FontSize(12).TextColor(tc.TextMuted)
							})
						}
						for _, sn := range snips {
							thisSn := sn
							row := ui.Row(c.Key("snip-"+thisSn.ID)).Radius(8).Padding(8, 12).Gap(10).AlignItems(ui.Center).
								Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6))

							row.Children(func() {
								// Category badge
								catColor := tc.Accent
								switch thisSn.Category {
								case "docker":
									catColor = tc.Info
								case "git":
									catColor = tc.Warning
								case "remote":
									catColor = tc.Success
								case "system":
									catColor = tc.Error
								case "dev":
									catColor = tc.Update
								}
								Badge(c, tc, strings.ToUpper(thisSn.Category), catColor)

								ui.Column(c).Grow(1).Gap(2).Children(func() {
									ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
										ui.Text(c, thisSn.Title).FontSize(12).FontWeight(600).TextColor(tc.Text)
										if thisSn.IsBuiltin {
											Badge(c, tc, "built-in", tc.TextMuted)
										}
									})
									if thisSn.Description != "" {
										ui.Text(c, thisSn.Description).FontSize(10).TextColor(tc.TextMuted).SingleLine()
									}
									ui.Text(c, thisSn.Command).FontSize(11).Font("monospace").TextColor(tc.Accent).SingleLine()
								})

								// Copy button
								cpBtn := ui.Row(c.Key("cp-"+thisSn.ID)).Height(24).Radius(5).Padding(0, 7).Gap(4).AlignItems(ui.Center).
									Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border)
								cpBtn.Children(func() {
									ui.Icon(c, icons.TablerCopy).Width(11).Height(11).TextColor(tc.TextMuted)
									ui.Text(c, "Copy").FontSize(10).TextColor(tc.TextMuted)
								})
								if cpBtn.Clicked() {
									mygo.Clipboard.WriteText(thisSn.Command)
									c.AddToast(ui.Toast{Title: "Copied to clipboard", Type: "success"})
								}

								// Run in Terminal button
								runBtn := ui.Row(c.Key("run-"+thisSn.ID)).Height(24).Radius(5).Padding(0, 8).Gap(4).AlignItems(ui.Center).
									Background(tc.Accent.Alpha(0.14)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.35))
								runBtn.Children(func() {
									ui.Icon(c, icons.TablerPlayerPlay).Width(10).Height(10).TextColor(tc.Accent)
									ui.Text(c, "Run").FontSize(10).FontWeight(600).TextColor(tc.Accent)
								})
								if runBtn.Clicked() {
									a.runTerminalCommand(thisSn.Command)
									a.ShowTerminalSnippetsModal = false
								}

								// Edit / Delete for custom snippets
								if !thisSn.IsBuiltin {
									editBtn := ui.Row(c.Key("ed-"+thisSn.ID)).Size(22, 22).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
									editBtn.Children(func() {
										ui.Icon(c, icons.TablerPencil).Width(11).Height(11).TextColor(tc.IconMuted)
									})
									if editBtn.Clicked() {
										a.TerminalSnippetFormOpen = true
										a.TerminalSnippetEditID = thisSn.ID
										a.TerminalSnippetDraftTitle = thisSn.Title
										a.TerminalSnippetDraftCmd = thisSn.Command
										a.TerminalSnippetDraftCat = thisSn.Category
										a.TerminalSnippetDraftDesc = thisSn.Description
									}

									delBtn := ui.Row(c.Key("del-"+thisSn.ID)).Size(22, 22).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
									delBtn.Children(func() {
										ui.Icon(c, icons.TablerTrash).Width(11).Height(11).TextColor(tc.IconMuted)
									})
									if delBtn.Clicked() {
										sm.DeleteSnippet(thisSn.ID)
									}
								}
							})
							if row.Hovered() {
								row.Background(tc.Text.Alpha(0.04))
							}
						}
					}
				})
			})
		})
	})
}

// runTerminalCommand delivers a command to the active terminal and records it in history.
func (a *AppUI) runTerminalCommand(cmd string) {
	if cmd == "" {
		return
	}
	if a.Snippets != nil {
		a.Snippets.RecordHistory(cmd, "snippet")
	}

	// Send to active terminal PTY
	targetSessionID := a.TerminalSnippetTargetSession
	if targetSessionID == "" && a.Terminal != nil {
		targetSessionID = a.Terminal.ActiveSessionID
	}

	if a.Terminal != nil {
		_ = a.Terminal.WriteInput(targetSessionID, cmd+"\n")
	}
}
