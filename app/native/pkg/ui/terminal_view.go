package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	termplugin "github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/terminal"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
)

// renderTerminalTabsStrip draws the clickable tab strip for a TerminalState.
func (a *AppUI) renderTerminalTabsStrip(c *ui.Context, tc theme.ThemeColors, ts *terminal.TerminalState, keyPrefix string, onSelect ...func(snap terminal.TerminalTabSnapshot)) {
	if ts == nil {
		return
	}
	tabs, activeIdx := ts.SnapshotTabs()
	if len(tabs) == 0 {
		title := ts.GetTitle()
		if title == "" {
			title = "shell"
		}
		tabs = []terminal.TerminalTabSnapshot{{Index: 0, ID: 0, Title: title, Exited: ts.IsExited()}}
		activeIdx = 0
	}
	ui.ScrollHorizontal(c).Grow(1).Scrollbars(false).Children(func() {
		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			for _, tabSnap := range tabs {
				tabSnap := tabSnap
				isActive := tabSnap.Index == activeIdx
				title := tabSnap.Title
				if title == "" {
					title = "Terminal"
				}
				tab := ui.Row(c.Key(fmt.Sprintf("%s-tab-%d-%d", keyPrefix, tabSnap.ID, tabSnap.Index))).
					Height(24).Radius(6).Padding(0, 8).Gap(6).AlignItems(ui.Center).
					BorderWidth(1).Cursor(ui.CursorPointer)
				if isActive {
					tab.Background(tc.SurfaceRaised).BorderColor(tc.Border)
				} else {
					tab.Background(tc.Surface.Alpha(0.5)).BorderColor(tc.Border.Alpha(0.4))
					if tab.Hovered() {
						tab.Background(tc.SurfaceRaised.Alpha(0.7))
					}
				}

				tab.ContextMenu(func(m *ui.Menu) {
					if m.Item("Terminal Snippets...").Shortcut(ui.Alt, ui.KeyS).Chosen() {
						sessID := ""
						if ts != nil {
							sessID = ts.ActiveSessionID
						}
						a.OpenTerminalSnippetsModal(sessID, "", "")
					}
					m.Separator()
					if m.Item("New Tab").Chosen() {
						if ts != nil {
							_ = ts.NewTab("")
							tabsAfter, activeAfter := ts.SnapshotTabs()
							if len(tabsAfter) > 0 && activeAfter >= 0 && activeAfter < len(tabsAfter) {
								if len(onSelect) > 0 && onSelect[0] != nil {
									onSelect[0](tabsAfter[activeAfter])
								}
							}
							if a.Window != nil {
								a.Window.Invalidate()
							}
						}
					}
					if m.Item("Clear Terminal").Chosen() {
						if ts != nil {
							ts.SendInput("\x0c")
						}
					}
					if m.Item("Restart Shell").Chosen() {
						if ts != nil {
							_ = ts.Restart()
						}
					}
					m.Separator()
					if m.Item("Close Tab").Chosen() {
						if ts != nil {
							if ts.TabCount() > 0 {
								ts.CloseTab(tabSnap.Index)
								tabsAfter, activeAfter := ts.SnapshotTabs()
								if len(tabsAfter) > 0 && activeAfter >= 0 && activeAfter < len(tabsAfter) {
									if len(onSelect) > 0 && onSelect[0] != nil {
										onSelect[0](tabsAfter[activeAfter])
									}
								}
							} else {
								_ = ts.Restart()
							}
							if a.Window != nil {
								a.Window.Invalidate()
							}
						}
					}
				})

				closedThisTab := false
				tab.Children(func() {
					labelColor := tc.TextMuted
					if isActive {
						labelColor = tc.Text
					}
					ui.Text(c, title).FontSize(11).FontWeight(500).TextColor(labelColor).SingleLine().MaxWidth(120)
					closeTab := ui.Row(c.Key(fmt.Sprintf("%s-tab-x-%d-%d", keyPrefix, tabSnap.ID, tabSnap.Index))).
						Size(14, 14).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
					closeTab.Children(func() {
						ui.Icon(c, icons.TablerX).Width(9).Height(9).TextColor(tc.IconMuted)
					})
					if closeTab.Hovered() {
						closeTab.Background(tc.Text.Alpha(0.08))
					}
					if closeTab.Clicked() {
						closedThisTab = true
						if ts.TabCount() > 0 {
							ts.CloseTab(tabSnap.Index)
							tabsAfter, activeAfter := ts.SnapshotTabs()
							if len(tabsAfter) > 0 && activeAfter >= 0 && activeAfter < len(tabsAfter) {
								if len(onSelect) > 0 && onSelect[0] != nil {
									onSelect[0](tabsAfter[activeAfter])
								}
							}
						} else {
							_ = ts.Restart()
						}
						if a.Window != nil {
							a.Window.Invalidate()
						}
					}
				})
				if !closedThisTab && tab.Clicked() && !isActive && ts.TabCount() > 0 {
					ts.SelectTab(tabSnap.Index)
					if len(onSelect) > 0 && onSelect[0] != nil {
						onSelect[0](tabSnap)
					}
					if a.Window != nil {
						a.Window.Invalidate()
					}
				}
			}
		})
	})
}

// renderBottomTerminal renders the bottom collapsible terminal drawer.
func (a *AppUI) renderBottomTerminal(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	ts := a.activeTerminal()

	ui.Column(c).Height(a.BottomTerminalHeight).Background(tc.Canvas).BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {
		// Terminal Drawer Header — ZCode dock style: bold label, shell name,
		// session tab strip, then compact actions menu + new/close on the right.
		hdr := ui.Row(c).Height(TerminalHeaderHeight).Padding(0, 14).Gap(10).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(tc.Border)
		hdr.ContextMenu(func(m *ui.Menu) {
			if m.Item("Add Snippet...").Chosen() {
				sessID := ""
				if ts != nil {
					sessID = ts.ActiveSessionID
				}
				clipText := mygo.Clipboard.ReadText()
				a.OpenAddSnippetModal(sessID, clipText)
			}
			if m.Item("Terminal Snippets...").Shortcut(ui.Alt, ui.KeyS).Chosen() {
				sessID := ""
				if ts != nil {
					sessID = ts.ActiveSessionID
				}
				a.OpenTerminalSnippetsModal(sessID, "", "")
			}
			if m.Item("Ask Agent Diagnostic").Chosen() {
				a.askAgentFromTerminal()
			}
			m.Separator()
			if m.Item("New Tab").Chosen() && ts != nil {
				cwd := ""
				if a.Workspace != nil && a.Workspace.RootDir != "" && !strings.HasPrefix(a.Workspace.RootDir, "ssh://") {
					cwd = a.Workspace.RootDir
				}
				_ = ts.NewTab(cwd)
			}
			if m.Item("Clear Terminal").Chosen() && ts != nil {
				ts.SendInput("\x0c")
			}
			if m.Item("Restart Shell").Chosen() && ts != nil {
				_ = ts.Restart()
			}
			m.Separator()
			if m.Item("Close Dock").Shortcut(ui.Cmd, ui.KeyBackquote).Chosen() {
				a.setBottomTerminalVisible(false)
			}
		})
		hdr.Children(func() {
			termTitle := "Terminal"
			if a.Workspace != nil && a.Workspace.RootDir != "" {
				termTitle = resolveProjectName(a.Workspace.RootDir)
			}
			ui.Text(c, termTitle).FontSize(13).FontWeight(600)
			shellName := "zsh"
			if a.Workspace != nil && strings.HasPrefix(a.Workspace.RootDir, "ssh://") {
				connID, _, displayHost, _ := parseSSHWorkspaceURI(a.Workspace.RootDir)
				host := a.resolveSSHDisplayHost(connID)
				if host == "" {
					host = displayHost
				}
				shellName = fmt.Sprintf("ssh (%s)", host)
			} else if a.Settings != nil && a.Settings.DefaultShell != "" {
				shellName = filepath.Base(a.Settings.DefaultShell)
			}
			ui.Text(c, shellName).FontSize(12).TextColor(tc.TextMuted)
			a.renderTerminalFocusBadge(c, tc, ts)

			a.renderTerminalTabsStrip(c, tc, ts, "bottom-term")

			moreBtn := ui.Row(c.Key("term-actions-menu")).Size(24, 24).Radius(6).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
			moreBtn.Children(func() {
				ui.Icon(c, icons.TablerDots).Width(14).Height(14).TextColor(tc.IconMuted)
			})
			if moreBtn.Hovered() {
				moreBtn.Background(tc.Text.Alpha(0.06))
			}
			moreBtn.Menu(func(m *ui.Menu) {
				if m.Item("Ask Agent Diagnostic").Chosen() {
					a.askAgentFromTerminal()
				}
				if m.Item("Add Snippet...").Chosen() {
					sessID := ""
					if ts != nil {
						sessID = ts.ActiveSessionID
					}
					clipText := mygo.Clipboard.ReadText()
					a.OpenAddSnippetModal(sessID, clipText)
				}
				if m.Item("Terminal Snippets").Shortcut(ui.Alt, ui.KeyS).Chosen() {
					sessID := ""
					if ts != nil {
						sessID = ts.ActiveSessionID
					}
					a.OpenTerminalSnippetsModal(sessID, "", "")
				}
				if ts != nil {
					m.Separator()
					if m.Item("Scroll Up (Page)").Shortcut(ui.Shift, ui.KeyPageUp).Chosen() {
						ts.ScrollPageUp()
					}
					if m.Item("Scroll to Bottom").Shortcut(ui.Cmd, ui.KeyEnd).Chosen() {
						ts.ScrollToBottom()
					}
					if m.Item("Scroll to Top").Shortcut(ui.Cmd, ui.KeyHome).Chosen() {
						ts.ScrollToTop()
					}
					m.Separator()
					if m.Item("Clear Terminal").Chosen() {
						ts.SendInput("\x0c")
					}
					if m.Item("Restart Shell").Chosen() {
						_ = ts.Restart()
					}
				}
			})

			if IconButton(c, tc, icons.TablerArrowUp, 13).Tooltip("Scroll Up (Page)").Clicked() && ts != nil {
				ts.ScrollPageUp()
			}
			if IconButton(c, tc, icons.TablerArrowDownToLine, 13).Tooltip("Scroll to Bottom").Clicked() && ts != nil {
				ts.ScrollToBottom()
			}
			if IconButton(c, tc, icons.TablerPlus, 13).Clicked() && ts != nil {
				cwd := ""
				if a.Workspace != nil && a.Workspace.RootDir != "" && !strings.HasPrefix(a.Workspace.RootDir, "ssh://") {
					cwd = a.Workspace.RootDir
				}
				_ = ts.NewTab(cwd)
			}
			if IconButton(c, tc, icons.TablerX, 13).Clicked() {
				a.setBottomTerminalVisible(false)
			}
		})

		// Terminal Viewport
		if ts != nil {
			a.renderTerminal(c, ts)
		}
	})
}

func (a *AppUI) isTerminalFocused(ts *terminal.TerminalState) bool {
	if ts == nil || a.termFocusWasFocused == nil {
		return false
	}
	return a.termFocusWasFocused[ts]
}

func (a *AppUI) renderTerminalFocusBadge(c *ui.Context, tc theme.ThemeColors, ts *terminal.TerminalState) {
	if !a.isTerminalFocused(ts) {
		return
	}
	pill := ui.Row(c.Key("term-active-badge")).Height(18).Radius(9).Padding(0, 6).Gap(4).
		AlignItems(ui.Center).Shrink(0).
		Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3))
	pill.Children(func() {
		ui.Box(c).Size(6, 6).Radius(3).Background(tc.Success)
		ui.Text(c, "Active").FontSize(10).FontWeight(600).TextColor(tc.Accent)
	})
}

// renderTerminal renders the Ghostty terminal viewport (native blinking cursor
// activates automatically while focused).
func (a *AppUI) renderTerminal(c *ui.Context, ts *terminal.TerminalState) {
	if ts == nil {
		return
	}
	t := c.Theme()

	term := ts.Term()
	if term == nil && ts.IsExited() && ts.TabCount() <= 1 {
		_ = ts.Restart()
		term = ts.Term()
	}
	if term != nil {
		if a.termFocusWasFocused == nil {
			a.termFocusWasFocused = make(map[*terminal.TerminalState]bool)
		}
		// Key by active session: without a per-session key MyGo may reuse the
		// previous GPU surface element, so switching tabs would keep showing
		// the old shell ("switching doesn't work").
		termView := termplugin.View(c, term).Key(fmt.Sprintf("term-sess-%s", ts.ActiveSessionID)).Fill().AutoFocus().ContextMenu(func(m *ui.Menu) {
			// No "Copy" item: the terminal copies on select (copy-on-select)
			// and ⌘C is handled by the terminal itself; mygo exposes no API to
			// read back the terminal selection to copy it manually.
			if m.Item("Paste").Shortcut(ui.Cmd, ui.KeyV).Chosen() {
				if text := mygo.Clipboard.ReadText(); text != "" {
					term.Paste(text)
				}
			}
			if m.Item("Clear Terminal").Shortcut(ui.Cmd, ui.KeyK).Chosen() {
				ts.SendInput("\x0c")
			}
			m.Separator()
			if m.Item("Scroll Up (Page)").Shortcut(ui.Shift, ui.KeyPageUp).Chosen() {
				ts.ScrollPageUp()
			}
			if m.Item("Scroll to Bottom").Shortcut(ui.Cmd, ui.KeyEnd).Chosen() {
				ts.ScrollToBottom()
			}
			m.Separator()
			if m.Item("Add Snippet...").Chosen() {
				sessID := ts.ActiveSessionID
				clipText := mygo.Clipboard.ReadText()
				a.OpenAddSnippetModal(sessID, clipText)
			}
			if m.Item("Terminal Snippets...").Shortcut(ui.Alt, ui.KeyS).Chosen() {
				sessID := ts.ActiveSessionID
				a.OpenTerminalSnippetsModal(sessID, "", "")
			}
			if m.Item("Ask Agent Diagnostic").Chosen() {
				a.askAgentFromTerminal()
			}
			m.Separator()
			if m.Item("New Tab").Shortcut(ui.Cmd, ui.KeyT).Chosen() {
				_ = ts.NewTab("")
			}
			if m.Item("Restart Shell").Chosen() {
				_ = ts.Restart()
			}
		})
		a.termFocusWasFocused[ts] = termView.Focused()
	} else {
		ui.Column(c).Grow(1).Center().Children(func() {
			ui.Text(c, "Terminal not available").TextColor(t.TextMuted)
			if ui.PrimaryButton(c, "Start Terminal").Clicked() {
				_ = ts.Restart()
			}
		})
	}
}

// askAgentFromTerminal captures recent output from the active terminal and sends a diagnostic prompt to the agent composer.
func (a *AppUI) askAgentFromTerminal() {
	output := ""
	if ts := a.activeTerminal(); ts != nil {
		output = ts.RecentOutput(40)
	}
	if strings.TrimSpace(output) == "" {
		output = "No recent terminal output recorded."
	}
	prompt := fmt.Sprintf("Here is the recent terminal output:\n```\n%s\n```\n\nPlease diagnose any errors, explain what happened, and fix the issues.", strings.TrimSpace(output))
	a.setMode(ModeAgent)
	a.AgentViewMode = AgentViewChat
	a.IsGitClientOpen = false
	a.ActiveTaskKind = "agent"
	if a.Agent != nil {
		a.Agent.InputText = prompt
	}
	if a.Window != nil {
		a.Window.Invalidate()
	}
}
