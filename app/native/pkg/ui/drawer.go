package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Haslab-dev/RakitKode/llm"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
	mygoeditor "github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/git"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
	internalgit "github.com/hasdev/forge-ade/core/git"
)

// gitOpsBusy guards git mutations (commit, stage/unstage all) triggered from
// the render path: only one runs at a time, and buttons render a busy state
// until the background goroutine publishes completion via runOnUI.
var gitOpsBusy atomic.Bool

// renderRightDrawer renders ZCode's full-height collapsible Right Sidebar with
// the 42px multi-tool tab strip and the New tab launcher / active tool view.
func (a *AppUI) renderRightDrawer(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	// Register the active pane as a tab BEFORE the header renders its pills.
	a.drawerTabReconcile()

	// Responsive clamping for small displays. The max matches the resize
	// handle's clamp (min(750, winW*0.6)) so dragging feels 1:1; the 280
	// floor keeps controls usable on tiny windows.
	drawerWidth := a.RightDrawerWidth
	if winW, _ := c.Size(); winW > 0 {
		maxAllowed := minF(750, winW*0.6)
		if drawerWidth > maxAllowed {
			drawerWidth = maxAllowed
		}
		if drawerWidth < 280 {
			drawerWidth = 280
		}
	}

	bar := c.TitleBar()
	rightPad := float32(10)
	if bar.Right > 0 {
		rightPad = max(bar.Right+10, 135)
	} else if bar.Left == 0 && (runtime.GOOS == "linux" || runtime.GOOS == "windows") {
		rightPad = 135
	}

	outer := ui.Box(c).Width(drawerWidth).Shrink(0).FillHeight()
	if a.glassBlurActive() {
		outer.Material(glass.Blur{Radius: 18})
	}
	outer.Children(func() {
		ui.Column(c).Fill().Background(tc.Sidebar).
			BorderWidth(0, 0, 0, 1).BorderColor(tc.SidebarBorder).Children(func() {
			// Right Sidebar Header Bar (42px, aligned with WorkspaceTopBarHeight)
			ui.Row(c).Height(WorkspaceTopBarHeight).Padding(0, rightPad, 0, 8).Gap(6).AlignItems(ui.Center).
				Background(tc.Chrome).BorderWidth(0, 0, 1, 0).BorderColor(tc.ToolbarBorder).DragWindow().Children(func() {
				// Open pane tabs (several panes co-exist) + the "+" launcher shortcut inside a horizontal strip.
				ui.ScrollHorizontal(c).Grow(1).Scrollbars(false).Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						for ti := range a.DrawerTabs {
							tabI := ti
							inst := a.DrawerTabs[tabI]
							active := tabI == a.drawerActiveIdx
							pill := ui.Row(c.Key(fmt.Sprintf("rtab-%d", tabI))).Height(26).Radius(6).
								Padding(0, 8).Gap(6).AlignItems(ui.Center).Cursor(ui.CursorPointer)
							if active {
								pill.Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
							}
							pill.Children(func() {
								ic := drawerTabIcon(inst.Kind)
								title := inst.Title
								if inst.Kind == RightTabSubagent {
									if n := a.subagentSessionCount(); n > 0 {
										title = fmt.Sprintf("%s (%d)", title, n)
									}
								}
								ui.Icon(c, ic).Width(13).Height(13).TextColor(tc.IconMuted)
								txt := ui.Text(c, title).FontSize(12).FontWeight(500).TextColor(tc.Text).MaxWidth(150).SingleLine()
								if !active {
									txt.TextColor(tc.TextMuted)
								}
								// ZCode parity: the close ✕ renders on the ACTIVE tab
								// only.
								if active {
									tabX := ui.Row(c.Key(fmt.Sprintf("rtab-x-%d", tabI))).Size(16, 16).Radius(4).
										AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
									tabX.Children(func() {
										ui.Icon(c, icons.TablerX).Width(10).Height(10).TextColor(tc.TextMuted)
									})
									if tabX.Hovered() {
										tabX.Background(tc.Text.Alpha(0.1))
									}
									if tabX.Clicked() {
										switch inst.Kind {
										case RightTabSideFile:
											a.closeSideFile(a.ActiveSideFile)
										case RightTabBrowser:
											a.closeBrowserTab()
										}
										a.closeDrawerTab(tabI)
									}
								}
							})
							if pill.Hovered() && !active {
								pill.Background(tc.Text.Alpha(0.05))
							}
							if pill.Clicked() {
								a.activateDrawerTab(tabI)
							}
							// Right-click: close-scope actions (ZCode tab parity).
							pill.ContextMenu(func(m *ui.Menu) {
								if active && m.Item("Close Tab").Chosen() {
									switch inst.Kind {
									case RightTabSideFile:
										a.closeSideFile(a.ActiveSideFile)
									case RightTabBrowser:
										a.closeBrowserTab()
									}
									a.closeDrawerTab(tabI)
								}
								if tabI > 0 && m.Item("Close Tabs to the Left").Chosen() {
									a.closeDrawerTabsLeft(tabI)
								}
								if tabI < len(a.DrawerTabs)-1 && m.Item("Close Tabs to the Right").Chosen() {
									a.closeDrawerTabsRight(tabI)
								}
								if m.Item("Close All Tabs").Chosen() {
									a.closeDrawerTabsAll()
								}
							})
						}

						plusBtn := ui.Row(c.Key("rtab-plus")).Size(26, 26).Radius(6).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
						plusBtn.Tooltip("New tab")
						plusBtn.Children(func() {
							ui.Icon(c, icons.TablerPlus).Width(13).Height(13).TextColor(tc.IconMuted)
						})
						if plusBtn.Hovered() {
							plusBtn.Background(tc.Text.Alpha(0.06))
						}
						if plusBtn.Clicked() {
							// "+" opens a NEW LAUNCHER TAB; picking a pane REPLACES the
							// launcher tab with that pane.
							a.openNewLauncherTab()
						}

						// Close-scope menu for the ACTIVE tab.
						moreBtn := ui.Row(c.Key("rtab-more")).Size(26, 26).Radius(6).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
						moreBtn.Tooltip("Tab actions")
						moreBtn.Children(func() {
							ui.Icon(c, icons.TablerDots).Width(13).Height(13).TextColor(tc.IconMuted)
						})
						if moreBtn.Hovered() {
							moreBtn.Background(tc.Text.Alpha(0.06))
						}
						moreBtn.Menu(func(m *ui.Menu) {
							if m.Item("Close Tab").Chosen() && a.drawerActiveIdx >= 0 && a.drawerActiveIdx < len(a.DrawerTabs) {
								switch a.DrawerTabs[a.drawerActiveIdx].Kind {
								case RightTabSideFile:
									a.closeSideFile(a.ActiveSideFile)
								case RightTabBrowser:
									a.closeBrowserTab()
								}
								a.closeDrawerTab(a.drawerActiveIdx)
							}
							m.Separator()
							if m.Item("Close Tabs to the Left").Chosen() {
								a.closeDrawerTabsLeft(a.drawerActiveIdx)
							}
							if m.Item("Close Tabs to the Right").Chosen() {
								a.closeDrawerTabsRight(a.drawerActiveIdx)
							}
							m.Separator()
							if m.Item("Close All Tabs").Chosen() {
								a.closeDrawerTabsAll()
							}
						})
					})
				})

				expandIcon := icons.TablerMaximize2
				expandTip := "Expand drawer"
				if a.RightDrawerWidth > 420 {
					expandIcon = icons.TablerMinimize2
					expandTip = "Default drawer width"
				}
				if IconButton(c, tc, expandIcon, 13).Shrink(0).Tooltip(expandTip).Clicked() {
					if a.RightDrawerWidth > 420 {
						a.RightDrawerWidth = RightDrawerDefault
					} else {
						a.RightDrawerWidth = 540
					}
				}

				if IconButton(c, tc, icons.TablerPanelRight, 14).Shrink(0).Tooltip("Close Right Drawer (⌘J)").Clicked() {
					a.ShowRightDrawer = false
				}
			})

			// Drawer Content
			ui.Column(c).Grow(1).FillHeight().Children(func() {
				switch a.RightDrawerTab {
				case RightTabChanges:
					a.renderChangesPane(c)
				case RightTabTerminal:
					if a.isBottomTerminalVisible() {
						ui.Column(c).Grow(1).Center().Padding(16).Gap(8).Children(func() {
							ui.Icon(c, icons.TablerTerminal).Width(24).Height(24).TextColor(tc.IconMuted)
							ui.Text(c, "Terminal open in bottom panel").FontSize(12).FontWeight(500).TextColor(tc.Text)
							moveBtn := ui.Row(c.Key("move-term-drawer")).Height(26).Radius(6).Padding(0, 10).AlignItems(ui.Center).
								Background(tc.Accent.Alpha(0.14)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.35)).Cursor(ui.CursorPointer)
							moveBtn.Children(func() {
								ui.Text(c, "Move to Right Drawer").FontSize(11).FontWeight(600).TextColor(tc.Accent)
							})
							if moveBtn.Clicked() {
								a.setBottomTerminalVisible(false)
							}
						})
						break
					}
					if ts := a.activeTerminal(); ts != nil {
						// Pane actions row. Terminal SESSIONS are the header
						// tabs themselves (1 pane = 1 session = 1 tab); use the
						// launcher or "+" to add another session.
						ui.Row(c).Height(32).Padding(0, 10).Gap(6).AlignItems(ui.Center).
							BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
							askBtn := ui.Row(c.Key("drawer-term-ask")).Height(22).Radius(4).Padding(0, 6).Gap(4).AlignItems(ui.Center).Shrink(0).
								Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.35)).Cursor(ui.CursorPointer)
							askBtn.Children(func() {
								ui.Icon(c, icons.TablerSparkles).Width(10).Height(10).TextColor(tc.Accent)
								ui.Text(c, "Ask Agent").FontSize(10).FontWeight(600).TextColor(tc.Accent)
							})
							if askBtn.Hovered() {
								askBtn.Background(tc.Accent.Alpha(0.2))
							}
							if askBtn.Clicked() {
								a.askAgentFromTerminal()
							}

							snipBtn := ui.Row(c.Key("drawer-term-snip")).Height(22).Radius(4).Padding(0, 6).Gap(4).AlignItems(ui.Center).Shrink(0).
								Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Cursor(ui.CursorPointer)
							snipBtn.Children(func() {
								ui.Icon(c, icons.TablerBook).Width(10).Height(10).TextColor(tc.IconMuted)
								ui.Text(c, "Snippets").FontSize(10).TextColor(tc.Text)
							})
							if snipBtn.Hovered() {
								snipBtn.Background(tc.SurfaceRaised)
							}
							if snipBtn.Clicked() {
								a.OpenTerminalSnippetsModal(ts.ActiveSessionID, "", "")
							}

							if IconButton(c, tc, icons.TablerArrowUp, 11).Tooltip("Scroll Up (Page)").Clicked() && ts != nil {
								ts.ScrollPageUp()
							}
							if IconButton(c, tc, icons.TablerArrowDownToLine, 11).Tooltip("Scroll to Bottom").Clicked() && ts != nil {
								ts.ScrollToBottom()
							}
							if IconButton(c, tc, icons.TablerEraser, 11).Tooltip("Clear Terminal").Clicked() {
								ts.SendInput("\x0c")
							}
							if IconButton(c, tc, icons.TablerRefresh, 11).Tooltip("Restart Terminal").Clicked() {
								_ = ts.Restart()
							}
							ui.Box(c).Grow(1)
							a.renderTerminalFocusBadge(c, tc, ts)
						})
						a.renderTerminal(c, ts)
					}
				case RightTabChat:
					a.renderSideChatPane(c)
				case RightTabTasks:
					a.renderDrawerPlansPane(c, tc)
				case RightTabGit:
					a.renderGitPane(c)
				case RightTabDatabase:
					a.renderDatabasePane(c)
				case RightTabArtifacts:
					a.renderStudioArtifactsPane(c, tc)
				case RightTabFiles:
					a.renderFilesPane(c)
				case RightTabSideFile:
					a.renderSideFilePane(c)
				case RightTabBrowser:
					a.renderBrowserPane(c)
				case RightTabSubagent:
					a.renderSubagentPane(c)
				case RightTabDeck:
					a.renderDeckPane(c)
				default:
					a.renderRightDrawerLauncher(c, tc)
				}
			})
		})
	})
}

func (a *AppUI) renderDrawerPlansPane(c *ui.Context, tc theme.ThemeColors) {
	ui.Scroll(c).Grow(1).Padding(16, 18).Children(func() {
		ui.Column(c).Gap(10).Children(func() {
			a.renderInspectorBody(c, tc, false)
		})
	})
}

// DrawerTabInstance is one open pane tab in the drawer header. Duplicate
// kinds are allowed for Terminal and Side Chat (multi-tab enabled);
// singleton panes (Review, Files, Browser, etc.) only have at most one tab.
type DrawerTabInstance struct {
	Kind    int    // RightTab* constant
	Title   string // pill label ("forge-ade", "Review", "New tab", …)
	TermIdx int    // RightTabTerminal: session index inside TerminalState
	ChatID  string // RightTabChat: unique session ID inside SideChatSessions
}

// drawerTabReconcile ensures RightDrawerTab is registered as an open tab:
// an already-open kind is FOCUSED, a new kind is appended. Direct
// RightDrawerTab setters — the 34 "open this pane" call sites — flow
// through here at render time. Launcher tabs are managed explicitly via
// OpenDrawerPaneNew.
func (a *AppUI) drawerTabReconcile() {
	a.drawerTabActivateKind(a.RightDrawerTab)
}

// drawerTabActivateKind focuses the first tab of kind, appending a new
// instance when none is open.
func (a *AppUI) drawerTabActivateKind(kind int) {
	if kind == RightTabLauncher {
		return // launcher instances are created explicitly
	}
	// The active tab already satisfies the kind: keep the user exactly where
	// they are (reconcile must never yank focus to an older instance — that
	// made a freshly-opened tab look "replaced").
	if a.drawerActiveIdx >= 0 && a.drawerActiveIdx < len(a.DrawerTabs) &&
		a.DrawerTabs[a.drawerActiveIdx].Kind == kind {
		return
	}
	for i := range a.DrawerTabs {
		if a.DrawerTabs[i].Kind == kind {
			a.activateDrawerTab(i)
			return
		}
	}
	if kind == RightTabTerminal {
		// 1 pane = 1 session: a directly-triggered Terminal pane gets its
		// own fresh session on first render.
		a.openTerminalSessionAndReplace()
		return
	}
	if kind == RightTabChat {
		a.pickDrawerPaneSideChat()
		return
	}
	a.openDrawerTabInstance(kind, a.drawerTabTitle(kind), -1)
}

// closeDrawerTabsExcept removes every tab except index keep, closing the
// terminal sessions of the removed Terminal tabs (session indices remapped).
func (a *AppUI) closeDrawerTabsExcept(keep int) {
	if keep < 0 || keep >= len(a.DrawerTabs) {
		return
	}
	ts := a.activeTerminal()
	removed := make([]int, 0, len(a.DrawerTabs))
	for i, t := range a.DrawerTabs {
		if i != keep && t.Kind == RightTabTerminal && t.TermIdx >= 0 && ts != nil {
			removed = append(removed, t.TermIdx)
		}
		if i != keep && t.Kind == RightTabChat && t.ChatID != "" {
			delete(a.SideChatSessions, t.ChatID)
		}
	}
	sort.Ints(removed)
	for k := len(removed) - 1; k >= 0; k-- { // close highest session index first
		ts.CloseTab(removed[k])
	}
	kept := a.DrawerTabs[keep]
	rest := make([]DrawerTabInstance, 0, len(a.DrawerTabs))
	rest = append(rest, a.DrawerTabs[:keep]...)
	rest = append(rest, a.DrawerTabs[keep+1:]...)
	a.DrawerTabs = rest
	// Remap surviving terminal session indices: each closed session with a
	// smaller index shifted the survivors down.
	for j := range a.DrawerTabs {
		if a.DrawerTabs[j].Kind != RightTabTerminal || a.DrawerTabs[j].TermIdx < 0 {
			continue
		}
		shift := 0
		for _, r := range removed {
			if r < a.DrawerTabs[j].TermIdx {
				shift++
			}
		}
		a.DrawerTabs[j].TermIdx -= shift
	}
	a.DrawerTabs = []DrawerTabInstance{kept}
	a.drawerActiveIdx = 0
	a.RightDrawerTab = kept.Kind
}

// closeDrawerTabsLeft removes every tab BEFORE index i.
func (a *AppUI) closeDrawerTabsLeft(i int) {
	if i <= 0 || i > len(a.DrawerTabs) {
		return
	}
	a.closeDrawerTabsRange(0, i)
}

// closeDrawerTabsRight removes every tab AFTER index i.
func (a *AppUI) closeDrawerTabsRight(i int) {
	if i < 0 || i >= len(a.DrawerTabs)-1 {
		return
	}
	a.closeDrawerTabsRange(i+1, len(a.DrawerTabs))
}

// closeDrawerTabsAll closes every drawer tab (terminal sessions included).
func (a *AppUI) closeDrawerTabsAll() {
	a.closeDrawerTabsRange(0, len(a.DrawerTabs))
}

// closeDrawerTabsRange removes tabs in [lo, hi), closing orphaned terminal
// sessions and remapping surviving session indices.
func (a *AppUI) closeDrawerTabsRange(lo, hi int) {
	ts := a.activeTerminal()
	removed := make([]int, 0, hi-lo)
	for _, t := range a.DrawerTabs[lo:hi] {
		if t.Kind == RightTabTerminal && t.TermIdx >= 0 && ts != nil {
			removed = append(removed, t.TermIdx)
		}
		if t.Kind == RightTabChat && t.ChatID != "" {
			delete(a.SideChatSessions, t.ChatID)
		}
	}
	sort.Ints(removed)
	rest := append(a.DrawerTabs[:lo:lo], a.DrawerTabs[hi:]...)
	a.DrawerTabs = rest
	for k := len(removed) - 1; k >= 0; k-- {
		if ts != nil {
			ts.CloseTab(removed[k])
		}
	}
	for j := range a.DrawerTabs {
		if a.DrawerTabs[j].Kind == RightTabTerminal && a.DrawerTabs[j].TermIdx >= 0 {
			shift := 0
			for _, r := range removed {
				if r < a.DrawerTabs[j].TermIdx {
					shift++
				}
			}
			a.DrawerTabs[j].TermIdx -= shift
		}
	}
	if len(a.DrawerTabs) == 0 {
		a.RightDrawerTab = RightTabLauncher
		a.drawerActiveIdx = 0
		return
	}
	if a.drawerActiveIdx >= len(a.DrawerTabs) {
		a.drawerActiveIdx = len(a.DrawerTabs) - 1
	}
	a.RightDrawerTab = a.DrawerTabs[a.drawerActiveIdx].Kind
}

// openDrawerTabInstance appends a pane instance and activates it.
func (a *AppUI) openDrawerTabInstance(kind int, title string, termIdx int) {
	a.openDrawerTabInstanceWithChat(kind, title, termIdx, "")
}

func (a *AppUI) openDrawerTabInstanceWithChat(kind int, title string, termIdx int, chatID string) {
	a.DrawerTabs = append(a.DrawerTabs, DrawerTabInstance{Kind: kind, Title: title, TermIdx: termIdx, ChatID: chatID})
	a.drawerActiveIdx = len(a.DrawerTabs) - 1
	a.RightDrawerTab = kind
}

// OpenDrawerPaneNew opens a fresh pane instance — always appends, so two
// Terminal sessions or two Side Chat sessions co-exist as separate tabs.
func (a *AppUI) OpenDrawerPaneNew(kind int, title string, termIdx int) {
	a.openDrawerTabInstance(kind, title, termIdx)
}

// replaceActiveDrawerTab TRANSFORMS the active tab in place — the launcher
// tab becomes the chosen pane (ZCode "replace" semantics: picking Terminal
// from the New-tab grid turns that launcher tab into a Terminal tab, it
// does not spawn an extra tab).
func (a *AppUI) replaceActiveDrawerTab(kind int, title string, termIdx int) {
	a.replaceActiveDrawerTabWithChat(kind, title, termIdx, "")
}

func (a *AppUI) replaceActiveDrawerTabWithChat(kind int, title string, termIdx int, chatID string) {
	if a.drawerActiveIdx < 0 || a.drawerActiveIdx >= len(a.DrawerTabs) {
		a.openDrawerTabInstanceWithChat(kind, title, termIdx, chatID)
		return
	}
	a.DrawerTabs[a.drawerActiveIdx] = DrawerTabInstance{Kind: kind, Title: title, TermIdx: termIdx, ChatID: chatID}
	a.RightDrawerTab = kind
	a.cleanLauncherTabs()
}

// cleanLauncherTabs removes any lingering RightTabLauncher tabs if other tabs exist.
func (a *AppUI) cleanLauncherTabs() {
	if len(a.DrawerTabs) <= 1 {
		return
	}
	var activeTab DrawerTabInstance
	hasActive := false
	if a.drawerActiveIdx >= 0 && a.drawerActiveIdx < len(a.DrawerTabs) {
		activeTab = a.DrawerTabs[a.drawerActiveIdx]
		hasActive = true
	}
	cleaned := make([]DrawerTabInstance, 0, len(a.DrawerTabs))
	for _, t := range a.DrawerTabs {
		if t.Kind != RightTabLauncher {
			cleaned = append(cleaned, t)
		}
	}
	if len(cleaned) == 0 {
		return
	}
	a.DrawerTabs = cleaned
	if hasActive && activeTab.Kind != RightTabLauncher {
		for i, t := range a.DrawerTabs {
			if t.Kind == activeTab.Kind && t.TermIdx == activeTab.TermIdx && t.ChatID == activeTab.ChatID && t.Title == activeTab.Title {
				a.drawerActiveIdx = i
				a.RightDrawerTab = t.Kind
				return
			}
		}
	}
	if a.drawerActiveIdx >= len(a.DrawerTabs) {
		a.drawerActiveIdx = len(a.DrawerTabs) - 1
	}
	if a.drawerActiveIdx >= 0 {
		a.RightDrawerTab = a.DrawerTabs[a.drawerActiveIdx].Kind
	}
}

// openTerminalSessionAndReplace spawns a NEW terminal session (titled
// after the project, deduped) and REPLACES the active tab with it — the
// launcher-pick path for Terminal.
func (a *AppUI) openTerminalSessionAndReplace() {
	title, idx := a.newTerminalSession()
	if idx < 0 {
		return
	}
	a.replaceActiveDrawerTab(RightTabTerminal, title, idx)
}

// openNewLauncherTab appends a fresh "New tab" launcher instance, or focuses an existing one.
func (a *AppUI) openNewLauncherTab() {
	for i, t := range a.DrawerTabs {
		if t.Kind == RightTabLauncher {
			a.activateDrawerTab(i)
			return
		}
	}
	a.OpenDrawerPaneNew(RightTabLauncher, "New tab", -1)
}

// newTerminalSession spawns a NEW terminal session at the workspace root
// (or remote host default for ssh://), titled after the project with a duplicate counter
// ("forge-ade", "forge-ade 2", …). Returns the title and session index.
func (a *AppUI) newTerminalSession() (string, int) {
	return a.newTerminalSessionInDir("")
}

// newTerminalSessionInDir spawns a NEW terminal session at targetDir (or workspace root
// when targetDir is empty) inside the active mode's TerminalState.
func (a *AppUI) newTerminalSessionInDir(targetDir string) (string, int) {
	ts := a.activeTerminal()
	if ts == nil {
		return "Terminal", -1
	}
	root := targetDir
	var cmd, env []string
	if root == "" {
		if a.Workspace != nil && strings.HasPrefix(a.Workspace.RootDir, "ssh://") {
			root = a.Workspace.RootDir
		} else if a.Workspace != nil && a.Workspace.RootDir != "" {
			root = a.Workspace.RootDir
		}
	}
	if strings.HasPrefix(root, "ssh://") {
		if a.Engine != nil && a.Engine.RemoteFS != nil {
			if client, remotePath, ok := a.Engine.RemoteFS.ParseRemotePath(root); ok && client != nil {
				prog, args, rEnv := client.SSHCommand(remotePath)
				cmd = append([]string{prog}, args...)
				env = rEnv
			}
		}
	}
	base := "Terminal"
	if root != "" {
		base = resolveProjectName(root)
	}
	title := base
	snaps, _ := ts.SnapshotTabs()
	for n := 2; ; n++ {
		taken := false
		for _, t := range snaps {
			if t.Title == title {
				taken = true
				break
			}
		}
		if !taken {
			break
		}
		title = fmt.Sprintf("%s %d", base, n)
	}
	tab, err := ts.NewTabCustom(title, root, cmd, env)
	if err != nil {
		return title, -1
	}
	idx := -1
	snaps2, _ := ts.SnapshotTabs()
	for _, t := range snaps2 {
		if t.ID == tab.ID {
			idx = t.Index
			break
		}
	}
	return title, idx
}

// OpenDrawerPaneNewTerminal spawns a NEW terminal session and opens it as a
// fresh pane tab (append).
func (a *AppUI) OpenDrawerPaneNewTerminal() {
	title, idx := a.newTerminalSession()
	if idx < 0 {
		return
	}
	a.openDrawerTabInstance(RightTabTerminal, title, idx)
}

// OpenDrawerPaneTerminalInDir spawns a NEW terminal session in targetDir and
// opens it in the right drawer without moving the shell to the bottom panel.
func (a *AppUI) OpenDrawerPaneTerminalInDir(targetDir string) {
	title, idx := a.newTerminalSessionInDir(targetDir)
	if idx < 0 {
		return
	}
	a.setBottomTerminalVisible(false)
	a.ShowRightDrawer = true
	a.openDrawerTabInstance(RightTabTerminal, title, idx)
}

// pickDrawerPaneTerminal spawns a fresh terminal session and opens it as a
// new co-existing pane tab, closing the launcher instance it came from.
func (a *AppUI) pickDrawerPaneTerminal() {
	a.setBottomTerminalVisible(false)
	title, idx := a.newTerminalSession()
	if idx < 0 {
		return
	}
	a.pickDrawerPane(RightTabTerminal, title, idx)
}

// nextSideChatTitle generates a deduped title ("Side chat", "Side chat 2", …) for side chat tabs.
func (a *AppUI) nextSideChatTitle() string {
	base := "Side chat"
	taken := make(map[string]bool)
	for _, t := range a.DrawerTabs {
		if t.Kind == RightTabChat {
			taken[t.Title] = true
		}
	}
	if !taken[base] {
		return base
	}
	for n := 2; ; n++ {
		t := fmt.Sprintf("%s %d", base, n)
		if !taken[t] {
			return t
		}
	}
}

// pickDrawerPaneSideChat spawns a fresh side chat session and opens it as a
// new co-existing pane tab, closing the launcher instance it came from.
func (a *AppUI) pickDrawerPaneSideChat() {
	if a.SideChatSessions == nil {
		a.SideChatSessions = make(map[string]*SideChatSession)
	}
	id := fmt.Sprintf("sidechat-%d", time.Now().UnixNano())
	title := a.nextSideChatTitle()
	sess := &SideChatSession{
		ID:       id,
		Title:    title,
		Messages: make([]*agent.ChatMessage, 0),
	}
	a.SideChatSessions[id] = sess
	a.ActiveSideChatID = id
	a.SideChatMessages = sess.Messages
	a.SideChatInput = ""
	a.pickDrawerPaneWithChat(RightTabChat, title, -1, id)
}

// pickDrawerPaneSingleton opens a singleton pane (Review, Files, Browser, etc.)
// from the launcher. If an instance already exists, it closes the launcher and
// activates the existing tab — only Terminal and Side Chat allow multiple tabs.
func (a *AppUI) pickDrawerPaneSingleton(kind int, title string) {
	for _, t := range a.DrawerTabs {
		if t.Kind == kind {
			a.cleanLauncherTabs()
			for j, t2 := range a.DrawerTabs {
				if t2.Kind == kind {
					a.activateDrawerTab(j)
					return
				}
			}
			return
		}
	}
	a.pickDrawerPane(kind, title, -1)
}

// pickDrawerPane appends or transforms the active launcher tab into the chosen pane,
// closing the launcher instance it came from ("tambah baru + tutup launcher").
func (a *AppUI) pickDrawerPane(kind int, title string, termIdx int) {
	a.pickDrawerPaneWithChat(kind, title, termIdx, "")
}

// pickDrawerPaneWithChat replaces the launcher tab if active, or appends and cleans any launcher tabs.
func (a *AppUI) pickDrawerPaneWithChat(kind int, title string, termIdx int, chatID string) {
	if a.drawerActiveIdx >= 0 && a.drawerActiveIdx < len(a.DrawerTabs) && a.DrawerTabs[a.drawerActiveIdx].Kind == RightTabLauncher {
		a.DrawerTabs[a.drawerActiveIdx] = DrawerTabInstance{Kind: kind, Title: title, TermIdx: termIdx, ChatID: chatID}
		a.RightDrawerTab = kind
		a.cleanLauncherTabs()
		return
	}
	a.openDrawerTabInstanceWithChat(kind, title, termIdx, chatID)
	a.cleanLauncherTabs()
}

// closeDrawerTab removes tab i (closing its terminal session when it is a
// Terminal tab, or removing its side chat session) and activates a sensible neighbor;
// an empty set renders the launcher by default.
func (a *AppUI) closeDrawerTab(i int) {
	if i < 0 || i >= len(a.DrawerTabs) {
		return
	}
	closed := a.DrawerTabs[i]
	a.DrawerTabs = append(a.DrawerTabs[:i], a.DrawerTabs[i+1:]...)
	if a.drawerActiveIdx > i {
		a.drawerActiveIdx--
	}
	if closed.Kind == RightTabTerminal && closed.TermIdx >= 0 {
		if ts := a.activeTerminal(); ts != nil {
			ts.CloseTab(closed.TermIdx)
		}
		for j := range a.DrawerTabs {
			if a.DrawerTabs[j].Kind == RightTabTerminal && a.DrawerTabs[j].TermIdx > closed.TermIdx {
				a.DrawerTabs[j].TermIdx--
			}
		}
	}
	if closed.Kind == RightTabChat && closed.ChatID != "" {
		delete(a.SideChatSessions, closed.ChatID)
	}
	if len(a.DrawerTabs) == 0 {
		a.RightDrawerTab = RightTabLauncher
		a.drawerActiveIdx = 0
		return
	}
	if a.drawerActiveIdx >= len(a.DrawerTabs) {
		a.drawerActiveIdx = len(a.DrawerTabs) - 1
	}
	a.activateDrawerTab(a.drawerActiveIdx)
}

func (a *AppUI) activateDrawerTab(i int) {
	if i < 0 || i >= len(a.DrawerTabs) {
		return
	}
	a.drawerActiveIdx = i
	a.RightDrawerTab = a.DrawerTabs[i].Kind
	if a.DrawerTabs[i].Kind == RightTabTerminal {
		if ts := a.activeTerminal(); ts != nil {
			ts.SelectTab(a.DrawerTabs[i].TermIdx)
		}
	}
	if a.DrawerTabs[i].Kind == RightTabChat && a.DrawerTabs[i].ChatID != "" {
		a.ActiveSideChatID = a.DrawerTabs[i].ChatID
		if sess, ok := a.SideChatSessions[a.DrawerTabs[i].ChatID]; ok {
			a.SideChatMessages = sess.Messages
			a.SideChatInput = sess.Input
		}
	}
}

// drawerTabIcon maps a drawer tab kind to its pill icon.
func drawerTabIcon(kind int) *ui.SVG {
	switch kind {
	case RightTabChanges:
		return icons.TablerFileDiff
	case RightTabGit:
		return icons.TablerGitBranch
	case RightTabTerminal:
		return icons.TablerTerminal
	case RightTabFiles:
		return icons.TablerFolder
	case RightTabArtifacts:
		return icons.TablerFolderOpen
	case RightTabSubagent:
		return icons.TablerBot
	case RightTabTasks:
		return icons.TablerChecklist
	case RightTabChat:
		return icons.TablerMessage
	case RightTabBrowser:
		return icons.TablerWorld
	case RightTabDatabase:
		return icons.TablerDatabase
	case RightTabDeck:
		return icons.TablerPresentation
	case RightTabSideFile:
		return icons.TablerFileText
	}
	return icons.TablerSquare
}

// subagentSessionCount counts the active session's subagents for the tab
// badge.
func (a *AppUI) subagentSessionCount() int {
	if a.Agent == nil || a.Agent.ActiveSessionID == "" {
		return 0
	}
	return len(a.Agent.Subagents(a.Agent.ActiveSessionID))
}

// drawerTabTitle maps a drawer tab kind to its pill title.
func (a *AppUI) drawerTabTitle(kind int) string {
	switch kind {
	case RightTabChanges:
		return "Review"
	case RightTabGit:
		return "Source Control"
	case RightTabTerminal:
		return "Terminal"
	case RightTabFiles:
		return "Files"
	case RightTabArtifacts:
		return "Artifacts"
	case RightTabSubagent:
		return "Child Agents"
	case RightTabTasks:
		return "Plans & Progress"
	case RightTabChat:
		return "Side conversation"
	case RightTabBrowser:
		return "Browser"
	case RightTabDatabase:
		return "Database"
	case RightTabDeck:
		return "Deck"
	case RightTabSideFile:
		return "File"
	}
	return "Pane"
}

func (a *AppUI) renderRightDrawerLauncher(c *ui.Context, tc theme.ThemeColors) {
	ui.Scroll(c).Grow(1).Padding(16, 16).Children(func() {
		ui.Column(c).Gap(16).Children(func() {
			ui.Column(c).Gap(2).Children(func() {
				ui.Text(c, "Open Tool").FontSize(15).FontWeight(600).TextColor(tc.Text)
				ui.Text(c, "Launch a workspace or agent pane in this drawer.").FontSize(11).TextColor(tc.TextMuted)
			})

			launcherCard := func(key, title, desc string, icon *ui.SVG, tint ui.Color, tinted bool, onOpen func()) {
				row := ui.Row(c.Key("rlaunch-"+key)).Padding(8, 10).Radius(10).Gap(10).AlignItems(ui.Center).
					Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.65)).Cursor(ui.CursorPointer)
				if row.Hovered() {
					row.Background(tc.SurfaceRaised).BorderColor(tc.Border)
				}
				row.Children(func() {
					tileBg := tc.Canvas
					tileFg := tc.IconMuted
					tileBorder := tc.Border
					if tinted {
						tileBg = tint.Alpha(0.14)
						tileFg = tint
						tileBorder = tint.Alpha(0.28)
					}
					ui.Row(c).Size(32, 32).Radius(8).AlignItems(ui.Center).Justify(ui.Center).Shrink(0).
						Background(tileBg).BorderWidth(1).BorderColor(tileBorder).Children(func() {
						ui.Icon(c, icon).Width(15).Height(15).TextColor(tileFg)
					})
					ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
						ui.Text(c, title).FontSize(12.5).FontWeight(600).TextColor(tc.Text).SingleLine()
						ui.Text(c, desc).FontSize(11).TextColor(tc.TextMuted).SingleLine()
					})
					ui.Icon(c, icons.TablerChevronRight).Width(13).Height(13).TextColor(tc.IconMuted).Shrink(0)
				})
				if row.Clicked() {
					onOpen()
				}
			}

			launcherGroup := func(label string, items func()) {
				ui.Column(c).Gap(6).Children(func() {
					ui.Text(c, label).FontSize(10).FontWeight(700).LetterSpacing(0.6).TextColor(tc.TextMuted).Padding(0, 2)
					ui.Column(c).Gap(6).Children(items)
				})
			}

			launcherGroup("WORKSPACE TOOLS", func() {
				launcherCard("files", "Files", "Browse and preview workspace files, images, and PDFs.", icons.TablerFolder, tc.Accent, false, func() {
					a.pickDrawerPaneSingleton(RightTabFiles, "Files")
				})
				launcherCard("git", "Source Control", "Review diffs, stage changes, commit, and switch branches.", icons.TablerGitBranch, tc.Accent, false, func() {
					a.pickDrawerPaneSingleton(RightTabGit, "Source Control")
				})
				launcherCard("terminal", "Terminal", "Run shell commands and inspect output.", icons.TablerTerminal, tc.Accent, false, func() {
					a.pickDrawerPaneTerminal()
				})
				launcherCard("browser", "Browser", "Preview local apps and browse web pages.", icons.TablerWorld, tc.Accent, false, func() {
					a.pickDrawerPaneSingleton(RightTabBrowser, "Browser")
					a.openBrowserTab("")
				})
				launcherCard("database", "Database", "Inspect SQLite & DuckDB tables and run queries.", icons.TablerDatabase, tc.Info, true, func() {
					a.pickDrawerPaneSingleton(RightTabDatabase, "Database")
				})
			})

			launcherGroup("AGENT & SESSION", func() {
				launcherCard("sidechat", "Side Conversation", "Ask quick questions without interrupting your main task.", icons.TablerMessage, tc.Accent, true, func() {
					a.pickDrawerPaneSideChat()
				})
				launcherCard("plans", "Plans & Progress", "Track active plans, checklists, and task progress.", icons.TablerChecklist, tc.Success, true, func() {
					a.pickDrawerPaneSingleton(RightTabTasks, "Plans")
				})
				launcherCard("subagents", "Child Agents", "Inspect and steer spawned sub-agents.", icons.TablerBot, tc.Warning, true, func() {
					a.pickDrawerPaneSingleton(RightTabSubagent, "Child Agents")
				})
				launcherCard("artifacts", "Artifacts", "Browse generated slides, scenes, and media.", icons.TablerFolderOpen, tc.Accent, true, func() {
					a.pickDrawerPaneSingleton(RightTabArtifacts, "Artifacts")
				})
			})
		})
	})
}

// renderChangesPane renders working tree changes and unified diff viewer.
func (a *AppUI) renderChangesPane(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	gm := a.Git

	// No workspace opened — nothing for git to report.
	if a.Workspace.RootDir == "" {
		ui.Column(c).Grow(1).Center().Gap(8).Children(func() {
			ui.Icon(c, icons.TablerFolder).Width(22).Height(22).TextColor(tc.IconMuted)
			ui.Text(c, "No folder opened").FontSize(12).TextColor(tc.TextMuted)
		})
		return
	}

	var conflicts, staged, unstaged []internalgit.FileStatus
	var snap git.GitSnapshot
	if gm != nil {
		// Lazy, render-thread wiring (set-once, plain nil check is enough
		// because render is single-threaded): GenerateAICommit's goroutine
		// delivers the AI message through this callback instead of writing
		// the widget-bound CommitSummary/CommitDesc fields from off-thread.
		// SetCommitMessage locks, so the write is safe once it lands on the
		// UI thread via runOnUI.
		if gm.OnAICommitResult == nil {
			gm.OnAICommitResult = func(summary, desc string) {
				a.runOnUI(func() {
					msg := summary
					if strings.TrimSpace(desc) != "" {
						msg += "\n\n" + desc
					}
					gm.SetCommitMessage(msg, summary, desc)
				})
			}
		}
		// Render from a point-in-time snapshot: Refresh/Stage/Commit goroutines
		// write the exported fields under the manager's lock.
		snap = gm.Snapshot()
		conflicts = snap.Conflicts
		staged = snap.Staged
		unstaged = append(unstaged, snap.Unstaged...)
		unstaged = append(unstaged, snap.Untracked...)
	}

	totalFiles := len(conflicts) + len(staged) + len(unstaged)

	ui.Row(c).Height(36).Padding(0, 12).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
		ui.Text(c, "Working Tree").FontSize(12).FontWeight(600)
		ui.Box(c).Grow(1)
		if gm != nil && gitOpsBusy.Load() {
			ui.Spinner(c).Size(12, 12).Label("Git")
		} else if IconButton(c, tc, icons.TablerRefresh, 12).Clicked() && gm != nil {
			gm.RefreshAsync()
		}
		ui.Text(c, fmt.Sprintf("%d changes", totalFiles)).FontSize(11).TextColor(tc.TextMuted)
	})

	if totalFiles == 0 {
		ui.Column(c).Grow(1).Center().Gap(8).Padding(20).Children(func() {
			ui.Icon(c, icons.TablerFileDiff).Width(32).Height(32).TextColor(tc.IconMuted)
			ui.Text(c, "No changes detected").FontSize(13).FontWeight(600).TextColor(tc.TextMuted)
			ui.Text(c, "Working tree is clean. Modified files will appear here.").FontSize(12).TextColor(tc.TextMuted).MaxWidth(240)
			if ContrastCTA(c, tc, "Refresh changes").Clicked() && gm != nil {
				gm.RefreshAsync()
			}
		})
		return
	}

	if a.SelectedDiffPath == "" {
		if len(conflicts) > 0 {
			a.SelectedDiffPath = conflicts[0].Path
		} else if len(staged) > 0 {
			a.SelectedDiffPath = staged[0].Path
		} else if len(unstaged) > 0 {
			a.SelectedDiffPath = unstaged[0].Path
		}
	}

	ui.Column(c).Grow(1).FillHeight().Children(func() {
		// File lists (Conflicts, Staged & Unstaged)
		ui.Scroll(c).Height(minF(200, float32(totalFiles)*28+44)).Children(func() {
			ui.Column(c).Padding(4, 6).Gap(4).Children(func() {
				// 0. Merge Conflicts section
				if len(conflicts) > 0 {
					ui.Row(c).Padding(2, 4).AlignItems(ui.Center).Children(func() {
						ui.Text(c, fmt.Sprintf("MERGE CONFLICTS (%d)", len(conflicts))).FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.Error)
					})
					for _, f := range conflicts {
						f := f
						isSel := a.SelectedDiffPath == f.Path
						acted := false
						row := ui.Row(c.Key("conflict-"+f.Path)).Height(26).Radius(6).Padding(0, 6).Gap(4).AlignItems(ui.Center)
						row.Children(func() {
							ui.Icon(c, icons.TablerAlertTriangle).Width(12).Height(12).TextColor(tc.Error)
							ui.Text(c, filepath.Base(f.Path)).FontSize(11).Grow(1).SingleLine()
							oursBtn := ui.Row(c.Key("ours-"+f.Path)).Height(18).Radius(4).Padding(0, 5).AlignItems(ui.Center).
								Background(tc.Accent.Alpha(0.14)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.35))
							oursBtn.Children(func() {
								ui.Text(c, "Ours").FontSize(9).FontWeight(600).TextColor(tc.Accent)
							})
							if oursBtn.Clicked() && gm != nil {
								acted = true
								go func(p string) {
									_ = gm.ResolveConflict(p, "ours")
								}(f.Path)
							}
							theirsBtn := ui.Row(c.Key("theirs-"+f.Path)).Height(18).Radius(4).Padding(0, 5).AlignItems(ui.Center).
								Background(tc.Warning.Alpha(0.14)).BorderWidth(1).BorderColor(tc.Warning.Alpha(0.35))
							theirsBtn.Children(func() {
								ui.Text(c, "Theirs").FontSize(9).FontWeight(600).TextColor(tc.Warning)
							})
							if theirsBtn.Clicked() && gm != nil {
								acted = true
								go func(p string) {
									_ = gm.ResolveConflict(p, "theirs")
								}(f.Path)
							}
						})
						if isSel {
							row.Background(tc.Error.Alpha(0.14))
						} else if row.Hovered() {
							row.Background(tc.Text.Alpha(0.04))
						}
						if !acted && row.Clicked() {
							a.SelectedDiffPath = f.Path
						}
					}
				}
				// 1. Staged section
				if len(staged) > 0 {
					ui.Row(c).Padding(2, 4).AlignItems(ui.Center).Children(func() {
						ui.Text(c, fmt.Sprintf("STAGED CHANGES (%d)", len(staged))).FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
						ui.Box(c).Grow(1)
						unstageAllBtn := ui.Row(c).Radius(4).Padding(1, 6).AlignItems(ui.Center)
						unstageAllBtn.Children(func() {
							ui.Text(c, "Unstage All").FontSize(10).TextColor(tc.TextMuted)
						})
						if unstageAllBtn.Hovered() && !gitOpsBusy.Load() {
							unstageAllBtn.Background(tc.Text.Alpha(0.06))
						}
						if unstageAllBtn.Clicked() && gm != nil && gitOpsBusy.CompareAndSwap(false, true) {
							go func() {
								gm.UnstageAll()
								a.runOnUI(func() {
									gitOpsBusy.Store(false)
								})
							}()
						}
					})
					for _, f := range staged {
						f := f
						isSel := a.SelectedDiffPath == f.Path
						row := ui.Row(c.Key("staged-"+f.Path)).Height(26).Radius(6).Padding(0, 6).Gap(4).AlignItems(ui.Center)
						row.Children(func() {
							icons.RenderFileIcon(c, f.Path, 13, tc.IconMuted)
							ui.Text(c, filepath.Base(f.Path)).FontSize(11).Grow(1).SingleLine()
							DiffStatText(c, tc, int(f.Additions), int(f.Deletions))
							Badge(c, tc, "S", tc.Success)

							unstageBtn := ui.Row(c).Size(18, 18).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
							unstageBtn.Children(func() {
								ui.Icon(c, icons.TablerMinus).Width(10).Height(10).TextColor(tc.IconMuted)
							})
							if unstageBtn.Hovered() {
								unstageBtn.Background(tc.Text.Alpha(0.06))
							}
							if unstageBtn.Clicked() && gm != nil {
								go gm.Unstage(f.Path)
							}
						})
						if isSel {
							row.Background(tc.Accent.Alpha(0.14))
						} else if row.Hovered() {
							row.Background(tc.Text.Alpha(0.04))
						}
						if row.Clicked() {
							a.SelectedDiffPath = f.Path
						}
					}
				}

				// 2. Unstaged / Untracked section
				if len(unstaged) > 0 {
					ui.Row(c).Padding(4, 4, 2, 4).AlignItems(ui.Center).Children(func() {
						ui.Text(c, fmt.Sprintf("CHANGES (%d)", len(unstaged))).FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
						ui.Box(c).Grow(1)
						stageAllBtn := ui.Row(c).Radius(4).Padding(1, 6).AlignItems(ui.Center)
						stageAllBtn.Children(func() {
							ui.Text(c, "Stage All").FontSize(10).TextColor(tc.TextMuted)
						})
						if stageAllBtn.Hovered() && !gitOpsBusy.Load() {
							stageAllBtn.Background(tc.Text.Alpha(0.06))
						}
						if stageAllBtn.Clicked() && gm != nil && gitOpsBusy.CompareAndSwap(false, true) {
							go func() {
								gm.StageAll()
								a.runOnUI(func() {
									gitOpsBusy.Store(false)
								})
							}()
						}
					})
					for _, f := range unstaged {
						f := f
						isSel := a.SelectedDiffPath == f.Path
						acted := false
						row := ui.Row(c.Key("unstaged-"+f.Path)).Height(26).Radius(6).Padding(0, 6).Gap(4).AlignItems(ui.Center)
						row.Children(func() {
							icons.RenderFileIcon(c, f.Path, 13, tc.IconMuted)
							ui.Text(c, filepath.Base(f.Path)).FontSize(11).Grow(1).SingleLine()
							DiffStatText(c, tc, int(f.Additions), int(f.Deletions))
							statusLabel := string(f.Status)
							if statusLabel == "" {
								statusLabel = "M"
							}
							Badge(c, tc, statusLabel, tc.Accent)

							if gm != nil && snap.DiscardPendingPath == f.Path {
								confirmBtn := ui.Row(c.Key("discard-ok-"+f.Path)).Height(18).Radius(4).Padding(0, 5).AlignItems(ui.Center).
									Background(tc.Error.Alpha(0.18)).BorderWidth(1).BorderColor(tc.Error.Alpha(0.45))
								confirmBtn.Children(func() {
									ui.Text(c, "Sure?").FontSize(9).FontWeight(700).TextColor(tc.Error)
								})
								if confirmBtn.Clicked() {
									acted = true
									go gm.Discard(f.Path)
								}
								cancelBtn := ui.Row(c.Key("discard-no-"+f.Path)).Size(18, 18).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
								cancelBtn.Children(func() {
									ui.Icon(c, icons.TablerX).Width(9).Height(9).TextColor(tc.IconMuted)
								})
								if cancelBtn.Clicked() {
									acted = true
									// Locked setter: Discard's goroutine also
									// writes this field (under lock).
									gm.SetDiscardPending("")
								}
							} else {
								discardBtn := ui.Row(c.Key("discard-"+f.Path)).Size(18, 18).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
								discardBtn.Children(func() {
									ui.Icon(c, icons.TablerRotate).Width(10).Height(10).TextColor(tc.IconMuted)
								})
								if discardBtn.Hovered() {
									discardBtn.Background(tc.Error.Alpha(0.12))
								}
								if discardBtn.Clicked() && gm != nil {
									acted = true
									// Locked setter: Discard's goroutine also
									// writes this field (under lock).
									gm.SetDiscardPending(f.Path)
								}
							}

							stageBtn := ui.Row(c).Size(18, 18).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
							stageBtn.Children(func() {
								ui.Icon(c, icons.TablerPlus).Width(10).Height(10).TextColor(tc.IconMuted)
							})
							if stageBtn.Hovered() {
								stageBtn.Background(tc.Text.Alpha(0.06))
							}
							if stageBtn.Clicked() && gm != nil {
								acted = true
								go gm.Stage(f.Path)
							}
						})
						if isSel {
							row.Background(tc.Accent.Alpha(0.14))
						} else if row.Hovered() {
							row.Background(tc.Text.Alpha(0.04))
						}
						if !acted && row.Clicked() {
							a.SelectedDiffPath = f.Path
						}
					}
				}
			})
		})

		// Commit Form Card
		if gm != nil {
			ui.Column(c).Padding(8, 10).Gap(6).
				BorderWidth(1, 0, 1, 0).BorderColor(tc.Border.Alpha(0.6)).
				Background(tc.SurfaceRaised.Alpha(0.4)).Children(func() {
				ui.TextInput(c, &gm.CommitSummary).Placeholder("Summary (required)").Grow(1)
				ui.TextInput(c, &gm.CommitDesc).Placeholder("Description (optional)").Grow(1)

				ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
					// AI commit button
					aiBtn := ui.Row(c).Height(26).Radius(6).Padding(0, 8).Gap(4).AlignItems(ui.Center).
						Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Cursor(ui.CursorPointer)
					aiBtn.Children(func() {
						ui.Icon(c, icons.TablerSparkles).Width(12).Height(12).TextColor(tc.Accent)
						ui.Text(c, "AI Message").FontSize(10).FontWeight(500).TextColor(tc.Text)
					})
					if aiBtn.Hovered() {
						aiBtn.Background(tc.Text.Alpha(0.06))
					}
					if aiBtn.Clicked() {
						go gm.GenerateAICommit("")
					}

					ui.Box(c).Grow(1)

					// Commit button. Stage/commit run on a background goroutine
					// (they can take seconds) guarded by gitOpsBusy; results
					// and toasts publish back through runOnUI.
					busy := gitOpsBusy.Load()
					canCommit := strings.TrimSpace(snap.CommitSummary) != "" && totalFiles > 0
					branchName := snap.Branch
					if branchName == "" {
						branchName = "main"
					}
					btnText := fmt.Sprintf("Commit to %s", branchName)
					commitBtn := ui.Row(c).Height(26).Radius(6).Padding(0, 10).AlignItems(ui.Center).Justify(ui.Center)
					switch {
					case busy:
						commitBtn.Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5))
						commitBtn.Children(func() {
							ui.Spinner(c).Size(11, 11).Label("Committing…")
						})
					case canCommit:
						commitBtn.Background(tc.Accent).BorderWidth(1).BorderColor(tc.Accent).Cursor(ui.CursorPointer)
						commitBtn.Children(func() {
							ui.Icon(c, icons.TablerCheck).Width(11).Height(11).TextColor(tc.AccentForeground)
							ui.Text(c, btnText).FontSize(11).FontWeight(600).TextColor(tc.AccentForeground)
						})
						if commitBtn.Clicked() && gitOpsBusy.CompareAndSwap(false, true) {
							summary, desc := snap.CommitSummary, snap.CommitDesc
							stageFirst := len(staged) == 0 && len(unstaged) > 0
							go func() {
								if stageFirst {
									gm.StageAll()
								}
								fullMsg := summary
								if strings.TrimSpace(desc) != "" {
									fullMsg += "\n\n" + desc
								}
								err := gm.Commit(fullMsg)
								a.runOnUI(func() {
									gitOpsBusy.Store(false)
									if err != nil {
										a.queueToast(ui.Toast{Title: "Commit Error", Description: err.Error(), Type: "error"})
									} else {
										a.queueToast(ui.Toast{Title: "Committed", Description: summary, Type: "success"})
										// ClearCommitDraft locks; called on the
										// UI thread where the draft fields are
										// bound to the text-input widgets.
										gm.ClearCommitDraft()
										// Drop the stale selection so the next change
										// set auto-selects a file that still exists.
										a.SelectedDiffPath = ""
									}
								})
							}()
						}
					default:
						commitBtn.Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5))
						commitBtn.Children(func() {
							ui.Text(c, btnText).FontSize(11).TextColor(tc.TextMuted)
						})
					}
				})
			})
		}

		// Diff viewer for selected file
		ui.Column(c).Grow(1).FillHeight().Clip().Children(func() {
			diffTitle := a.SelectedDiffPath
			header := ui.Row(c).Height(30).Padding(0, 8).Gap(6).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Background(tc.Surface)
			header.Children(func() {
				ui.Icon(c, icons.TablerGitCompare).Width(12).Height(12).TextColor(tc.IconMuted)
				ui.Text(c, filepath.Base(diffTitle)).FontSize(11).FontWeight(500).Grow(1).SingleLine()
				if a.Editor != nil {
					layoutLbl := "Unified"
					if a.Editor.DiffLayout == int(mygoeditor.DiffSplit) {
						layoutLbl = "Split"
					}
					layoutBtn := ui.Row(c.Key("chg-diff-layout")).Radius(4).Padding(2, 6).Gap(3).AlignItems(ui.Center).
						Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border).Cursor(ui.CursorPointer).Tooltip("Switch Diff Layout (Unified / Split)")
					layoutBtn.Children(func() {
						ui.Text(c, layoutLbl).FontSize(10).FontWeight(500).TextColor(tc.TextMuted)
					})
					if layoutBtn.Hovered() {
						layoutBtn.Background(tc.SurfaceRaised)
					}
					if layoutBtn.Clicked() {
						if a.Editor.DiffLayout == int(mygoeditor.DiffSplit) {
							a.Editor.DiffLayout = int(mygoeditor.DiffUnified)
						} else {
							a.Editor.DiffLayout = int(mygoeditor.DiffSplit)
						}
					}
				}
				openBtn := ui.Row(c).Radius(4).Padding(2, 6).Gap(3).AlignItems(ui.Center).Cursor(ui.CursorPointer).Tooltip("Open in Editor")
				openBtn.Children(func() {
					ui.Icon(c, icons.TablerCode).Width(11).Height(11).TextColor(tc.TextMuted)
					ui.Text(c, "Open").FontSize(11).TextColor(tc.TextMuted)
				})
				if openBtn.Hovered() {
					openBtn.Background(tc.Text.Alpha(0.05))
				}
				if openBtn.Clicked() {
					fullPath := filepath.Join(a.Workspace.RootDir, diffTitle)
					_, _ = a.Editor.OpenFile(fullPath, diffTitle)
					a.Mode = ModeEditor
				}
			})

			repoDir := a.Workspace.RootDir
			if repoDir == "" && gm != nil {
				repoDir = snap.RootDir
			}
			if repoDir == "" && a.Agent != nil {
				repoDir = a.Agent.WorkspacePath()
			}
			patch := ""
			diffReady := true
			if diffTitle != "" {
				if gm != nil {
					patch, diffReady = gm.GetCachedFileDiff(diffTitle)
				} else if a.Engine != nil {
					patch = a.Engine.GetGitDiff(repoDir, diffTitle)
				}
			}
			if !diffReady {
				ui.Column(c).Grow(1).Center().Gap(6).Children(func() {
					ui.Spinner(c).Size(14, 14).Label("Loading diff")
				})
			} else if patch == "" {
				ui.Column(c).Grow(1).Center().Children(func() {
					ui.Text(c, "No diff available for this file").FontSize(12).TextColor(tc.TextMuted)
				})
			} else {
				a.renderHunkRevertStrip(c, tc, gm, diffTitle, patch)
				d := a.Editor.OpenPatchDiff(diffTitle, patch, a.ThemeMgr)
				if d != nil {
					d.Layout = mygoeditor.DiffLayout(a.Editor.DiffLayout)
					mygoeditor.DiffView(c, d).Grow(1).FillWidth()
				}
			}
		})
	})
}

func (a *AppUI) getActiveSideChat() *SideChatSession {
	if a.SideChatSessions == nil {
		a.SideChatSessions = make(map[string]*SideChatSession)
	}
	if a.drawerActiveIdx >= 0 && a.drawerActiveIdx < len(a.DrawerTabs) {
		t := a.DrawerTabs[a.drawerActiveIdx]
		if t.Kind == RightTabChat && t.ChatID != "" {
			if sess, ok := a.SideChatSessions[t.ChatID]; ok {
				return sess
			}
		}
	}
	if a.ActiveSideChatID != "" {
		if sess, ok := a.SideChatSessions[a.ActiveSideChatID]; ok {
			return sess
		}
	}
	defaultID := "default"
	if sess, ok := a.SideChatSessions[defaultID]; ok {
		return sess
	}
	sess := &SideChatSession{
		ID:       defaultID,
		Title:    "Side chat",
		Messages: a.SideChatMessages,
		Input:    a.SideChatInput,
	}
	a.SideChatSessions[defaultID] = sess
	a.ActiveSideChatID = defaultID
	return sess
}

// renderSideChatPane renders a secondary chat thread for quick questions.
func (a *AppUI) renderSideChatPane(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	a.SideChatMd.Theme = a.ThemeMgr.MarkdownTheme()
	a.SideChatMd.Reveal = &markdown.Reveal{Unit: markdown.AtOnce, Fade: -1}
	a.SideChatMd.OnLink = a.openMarkdownLink

	sess := a.getActiveSideChat()

	ui.Row(c).Height(40).Padding(0, 12).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
		ui.Text(c, sess.Title).FontSize(13).FontWeight(600)
		ui.Box(c).Grow(1)
		ui.Text(c, a.SideChatModel).FontSize(10).TextColor(tc.TextMuted)
	})

	if len(sess.Messages) == 0 {
		ui.Column(c).Grow(1).Center().Gap(8).Padding(20).Children(func() {
			ui.Icon(c, icons.TablerMessage).Width(28).Height(28).TextColor(tc.IconMuted)
			ui.Text(c, "Side Conversation").FontSize(13).FontWeight(600).TextColor(tc.TextMuted)
			ui.Text(c, "Run quick queries or ask fast code questions without interrupting your main task session.").FontSize(12).TextColor(tc.TextMuted).MaxWidth(240)
			// Quick-start prompts, as the webview side chat's empty state.
			ui.Column(c).Gap(4).Padding(6, 0, 0, 0).Children(func() {
				for _, qp := range []string{
					"Explain all changed files",
					"Check for potential edge cases",
					"How can I test the modified code?",
				} {
					qp := qp
					chip := ui.Row(c.Key("scq-"+qp)).Height(26).Radius(6).Padding(0, 9).AlignItems(ui.Center).
						Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Cursor(ui.CursorPointer)
					chip.Children(func() {
						ui.Text(c, qp).FontSize(11)
					})
					if chip.Hovered() {
						chip.Background(tc.Text.Alpha(0.05))
					}
					if chip.Clicked() {
						sess.Input = qp
						a.SideChatInput = qp
						a.sendSideChat(qp)
					}
				}
			})
		})
	} else {
		ui.List(c, &ui.ListState{FollowEnd: true}, len(sess.Messages), func(i int) {
			if i >= len(sess.Messages) {
				return
			}
			m := sess.Messages[i]
			if m.Role == "user" {
				ui.Row(c.Key("side-user-"+fmt.Sprint(i))).Padding(3, 8).Justify(ui.End).Children(func() {
					bubble := ui.Column(c).Padding(8, 12).Radius(14).
						Background(tc.MessageSurface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5)).
						MaxWidth(maxF(180, a.RightDrawerWidth-48))
					bubble.Children(func() {
						ui.Text(c, m.Text).FontSize(13).TextColor(tc.MessageForeground)
					})
				})
			} else {
				ui.Column(c.Key("side-asst-"+fmt.Sprint(i))).Padding(3, 8).Children(func() {
					if m.Done {
						markdown.View(c, m.Text, &a.SideChatMd)
					} else {
						markdown.Stream(c, m.Text, &a.SideChatMd)
					}
				})
			}
		}).Grow(1).Padding(8)
	}

	ui.Row(c).Padding(8, 10).Gap(6).AlignItems(ui.Center).
		BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {
		side := ui.TextAreaBase(c, &sess.Input).Lines(1, 5).Placeholder("Ask side chat...").Grow(1)
		side.OnTextKey(func(k ui.TextKey) bool {
			if k.Key == ui.KeyEnter && k.Mods == 0 {
				if strings.TrimSpace(sess.Input) != "" {
					text := strings.TrimSpace(sess.Input)
					sess.Input = ""
					a.SideChatInput = ""
					a.sendSideChat(text)
				}
				return true
			}
			return false
		})
		sendBtn := ui.Row(c).Size(28, 28).Radius(999).AlignItems(ui.Center).Justify(ui.Center).
			Background(tc.MessageAction).Cursor(ui.CursorPointer)
		sendBtn.Children(func() {
			ui.Icon(c, icons.TablerSend).Width(13).Height(13).TextColor(tc.MessageActionForeground)
		})
		if sendBtn.Clicked() && strings.TrimSpace(sess.Input) != "" {
			text := strings.TrimSpace(sess.Input)
			sess.Input = ""
			a.SideChatInput = ""
			a.sendSideChat(text)
		}
	})
}

func (a *AppUI) sendSideChat(text string) {
	sess := a.getActiveSideChat()
	if sess.Streaming {
		return
	}
	sess.Messages = append(sess.Messages, &agent.ChatMessage{Role: "user", Text: text})
	reply := &agent.ChatMessage{Role: "assistant", Text: "", Done: false}
	sess.Messages = append(sess.Messages, reply)
	sess.Streaming = true
	a.SideChatMessages = sess.Messages
	a.SideChatStreaming = true
	if a.Window != nil {
		a.Window.Invalidate()
	}

	engine := a.Engine
	root := a.Workspace.RootDir
	// The stream callback runs on an LLM goroutine: buffer the text locally
	// and publish every delta through runOnUI so render never races reply.Text.
	go func() {
		streamed := ""
		defer func() {
			final := streamed
			a.runOnUI(func() {
				reply.Text = final
				reply.Done = true
				sess.Streaming = false
				a.SideChatStreaming = false
				a.SideChatMessages = sess.Messages
			})
		}()
		if engine == nil || engine.LLMClient == nil {
			streamed = "No LLM provider configured. Set one up in Settings → Models & Providers."
			return
		}
		msgs := []llm.LLMMessage{{
			Role:    "user",
			Content: "Workspace: " + root + "\n\n" + text,
		}}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		_, err := engine.LLMClient.ChatWithStream(ctx, msgs, nil, func(deltaContent, deltaReasoning string) {
			if deltaContent != "" {
				streamed += deltaContent
				partial := streamed
				a.runOnUI(func() {
					reply.Text = partial
					a.SideChatMessages = sess.Messages
				})
			}
		})
		if err != nil && streamed == "" {
			streamed = fmt.Sprintf("LLM request failed: %v", err)
		}
	}()
}

// renderGitPane renders the full Source Control pane with Changes, History, and Branches tabs.
func (a *AppUI) renderGitPane(c *ui.Context) {
	gm := a.Git
	if gm == nil {
		return
	}
	tc := a.ThemeMgr.Colors()

	// Git operations run against the local working tree only, exactly as
	// forge-ade skips git for ssh:// workspaces.
	if strings.HasPrefix(a.Workspace.RootDir, "ssh://") {
		ui.Column(c).Grow(1).Center().Gap(8).Padding(20).Children(func() {
			ui.Icon(c, icons.TablerGitBranch).Width(32).Height(32).TextColor(tc.IconMuted)
			ui.Text(c, "Git is unavailable over SSH").FontSize(13).FontWeight(600).TextColor(tc.TextMuted)
			ui.Text(c, "Source control works on local workspaces only. Disconnect to use git.").FontSize(12).TextColor(tc.TextMuted).MaxWidth(240)
		})
		return
	}

	ui.Column(c).Fill().Children(func() {
		// Render from a point-in-time snapshot; Refresh/Fetch/Pull/Push run on
		// goroutines and publish under the manager's lock.
		snap := gm.Snapshot()

		// Header row with Branch pill + Remotes (Fetch, Pull, Push) + Refresh
		ui.Row(c).Height(40).Padding(0, 10).Gap(6).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
			ui.Text(c, "Source Control").FontSize(12).FontWeight(600)

			if snap.Branch != "" {
				branchPill := ui.Row(c).Height(24).Radius(4).Padding(0, 6).Gap(4).AlignItems(ui.Center).
					Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3)).Cursor(ui.CursorPointer)
				branchPill.Children(func() {
					ui.Icon(c, icons.TablerGitBranch).Width(11).Height(11).TextColor(tc.Accent)
					ui.Text(c, snap.Branch).FontSize(11).FontWeight(600).TextColor(tc.Accent)
				})
				if branchPill.Clicked() {
					a.ShowBranchModal = true
				}
			}

			ui.Box(c).Grow(1)

			// Remote op buttons: spinner while their goroutine is running.
			remoteBtn := func(label string, icon *ui.SVG, busy bool, run func() error, doneTitle, doneType string) {
				b := ui.Row(c).Height(24).Radius(4).Padding(0, 6).Gap(3).AlignItems(ui.Center).
					Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6)).Cursor(ui.CursorPointer)
				b.Children(func() {
					if busy {
						ui.Spinner(c).Size(10, 10).Label(label)
					} else {
						ui.Icon(c, icon).Width(10).Height(10).TextColor(tc.IconMuted)
					}
					ui.Text(c, label).FontSize(10).TextColor(tc.Text)
				})
				if b.Hovered() && !busy {
					b.Background(tc.Text.Alpha(0.06))
				}
				if b.Clicked() && !busy {
					go func() {
						err := run()
						if err != nil {
							a.queueToast(ui.Toast{Title: label + " Error", Description: err.Error(), Type: "error"})
						} else {
							a.queueToast(ui.Toast{Title: doneTitle, Type: doneType})
						}
					}()
				}
			}
			remoteBtn("Fetch", icons.TablerArrowDown, snap.IsFetching, func() error { return gm.Fetch() }, "Fetched Remotes", "info")
			remoteBtn("Pull", icons.TablerArrowDown, snap.IsPulling, func() error { return gm.Pull() }, "Pull Completed", "success")
			remoteBtn("Push", icons.TablerArrowUp, snap.IsPushing, func() error { return gm.Push() }, "Pushed to Remote", "success")

			// Refresh button
			refreshBtn := ui.Row(c).Size(24, 24).Radius(4).AlignItems(ui.Center).Justify(ui.Center).
				Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6))
			refreshBtn.Children(func() {
				ui.Icon(c, icons.TablerRefresh).Width(11).Height(11).TextColor(tc.IconMuted)
			})
			if refreshBtn.Hovered() {
				refreshBtn.Background(tc.Text.Alpha(0.06))
			}
			if refreshBtn.Clicked() {
				gm.RefreshAsync()
			}
		})

		// Subtabs: Changes, History, Branches
		ui.Row(c).Height(34).Padding(0, 8).Gap(4).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.6)).Children(func() {
			subtabs := []struct {
				Tab   int
				Label string
				Icon  *ui.SVG
			}{
				{git.GitTabChanges, fmt.Sprintf("Changes (%d)", len(snap.Conflicts)+len(snap.Staged)+len(snap.Unstaged)+len(snap.Untracked)), icons.TablerFileDiff},
				{git.GitTabHistory, "History", icons.TablerHistory},
				{git.GitTabBranches, "Branches", icons.TablerGitBranch},
			}
			for _, st := range subtabs {
				thisTab := st
				isSel := gm.ActiveTab == thisTab.Tab
				tabBtn := ui.Row(c.Key("subtab-"+thisTab.Label)).Height(24).Radius(4).Padding(0, 8).Gap(4).AlignItems(ui.Center)
				tabBtn.Children(func() {
					ui.Icon(c, thisTab.Icon).Width(11).Height(11).TextColor(tc.IconMuted)
					ui.Text(c, thisTab.Label).FontSize(11).FontWeight(500)
				})
				if isSel {
					tabBtn.Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
				}
				if tabBtn.Clicked() {
					gm.ActiveTab = thisTab.Tab
				}
			}
		})

		switch gm.ActiveTab {
		case git.GitTabChanges:
			a.renderChangesPane(c)
		case git.GitTabHistory:
			ui.Column(c).Grow(1).FillHeight().Children(func() {
				// Commit List
				ui.Scroll(c).Height(minF(220, float32(len(snap.Commits))*38+10)).Children(func() {
					ui.Column(c).Padding(4, 6).Gap(2).Children(func() {
						for _, commit := range snap.Commits {
							cm := commit
							isSel := snap.SelectedCommitHash == cm.Hash
							row := ui.Column(c.Key("commit-"+cm.Hash)).Radius(6).Padding(6, 8).Gap(2)
							row.Children(func() {
								ui.Text(c, cm.Message).FontSize(11).FontWeight(600).SingleLine().TextColor(tc.Text)
								ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
									ui.Text(c, cm.AuthorName).FontSize(10).TextColor(tc.TextMuted)
									ui.Box(c).Grow(1)
									ui.Text(c, cm.ShortHash).FontSize(10).FontWeight(600).TextColor(tc.Accent)
								})
							})
							if isSel {
								row.Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3))
							} else if row.Hovered() {
								row.Background(tc.Text.Alpha(0.04))
							}
							if row.Clicked() {
								gm.SelectCommit(cm.Hash)
							}
						}
					})
				})

				// Commit Diff Viewer (when a commit is selected)
				if snap.SelectedCommitHash != "" {
					ui.Column(c).Grow(1).FillHeight().Children(func() {
						ui.Row(c).Height(28).Padding(0, 8).Gap(6).AlignItems(ui.Center).
							Background(tc.SurfaceRaised).BorderWidth(1, 0, 1, 0).BorderColor(tc.Border.Alpha(0.5)).Children(func() {
							ui.Icon(c, icons.TablerGitCommit).Width(11).Height(11).TextColor(tc.Accent)
							shortH := snap.SelectedCommitHash
							if len(shortH) > 8 {
								shortH = shortH[:8]
							}
							ui.Text(c, fmt.Sprintf("Commit Diff (%s)", shortH)).FontSize(11).FontWeight(600).TextColor(tc.Text)
						})

						if snap.LoadingCommitDiff {
							ui.Column(c).Grow(1).Center().Children(func() {
								ui.Text(c, "Loading commit diff...").FontSize(11).TextColor(tc.TextMuted)
							})
						} else if snap.SelectedCommitDiff != "" {
							d := a.Editor.OpenPatchDiff("commit.diff", snap.SelectedCommitDiff, a.ThemeMgr)
							if d != nil {
								mygoeditor.DiffView(c, d).Grow(1).FillWidth()
							}
						} else {
							ui.Column(c).Grow(1).Center().Children(func() {
								ui.Text(c, "No diff in this commit").FontSize(11).TextColor(tc.TextMuted)
							})
						}
					})
				}
			})
		case git.GitTabBranches:
			ui.Column(c).Grow(1).Children(func() {
				// Create branch, as forge-ade's BranchSelector does
				ui.Row(c).Padding(8, 10, 4, 10).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.TextInput(c, &gm.NewBranchName).Placeholder("New branch name…").Grow(1)
					createBtn := ui.Row(c).Height(26).Radius(6).Padding(0, 10).Gap(4).AlignItems(ui.Center).
						Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3))
					createBtn.Children(func() {
						ui.Icon(c, icons.TablerPlus).Width(11).Height(11).TextColor(tc.Accent)
						ui.Text(c, "Create").FontSize(10).FontWeight(600).TextColor(tc.Accent)
					})
					if createBtn.Clicked() && strings.TrimSpace(gm.NewBranchName) != "" {
						name := strings.TrimSpace(gm.NewBranchName)
						gm.NewBranchName = ""
						go func() {
							if err := gm.CreateBranch(name); err != nil {
								a.queueToast(ui.Toast{Title: "Branch Error", Description: err.Error(), Type: "error"})
							} else {
								a.queueToast(ui.Toast{Title: "Branch Created", Description: name, Type: "success"})
							}
						}()
					}
				})
				ui.Scroll(c).Grow(1).Padding(8).Children(func() {
					ui.Column(c).Gap(2).Children(func() {
						for _, b := range snap.Branches {
							branch := b
							isCurrent := branch == snap.Branch
							row := ui.Row(c.Key("branch-"+branch)).Height(30).Radius(6).Padding(0, 8).Gap(6).AlignItems(ui.Center)
							row.Children(func() {
								ui.Icon(c, icons.TablerGitBranch).Width(12).Height(12).TextColor(tc.IconMuted)
								ui.Text(c, branch).FontSize(11).FontWeight(500).Grow(1).SingleLine()
								if isCurrent {
									Badge(c, tc, "HEAD", tc.Accent)
								}
							})
							if isCurrent {
								row.Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3))
							} else if row.Hovered() {
								row.Background(tc.Text.Alpha(0.04))
							}
							if row.Clicked() && !isCurrent {
								a.checkoutBranch(branch)
							}
						}
					})
				})
			})
		}
	})
}

// renderDatabasePane renders SQLite / DuckDB inspector and query console.
// All manager state is read through one per-frame Snapshot: ScanDatabases,
// Inspect, SelectTable and ExecuteQuery run on goroutines and mutate the
// fields under the manager's lock (they can nil out Info / QueryResult
// between a direct check and its use, which crashes the frame).
func (a *AppUI) renderDatabasePane(c *ui.Context) {
	dm := a.Database
	if dm == nil {
		return
	}
	tc := a.ThemeMgr.Colors()
	snap := dm.Snapshot()

	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Height(40).Gap(8).Padding(0, 12).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons.TablerDatabase).Width(14).Height(14).TextColor(tc.Accent)
				ui.Text(c, "Database").FontSize(12.5).FontWeight(600).TextColor(tc.Text)
			})
			if snap.Info != nil {
				ui.Text(c, fmt.Sprintf("%s (%s)", snap.Info.FileName, snap.Info.Type)).FontSize(11).TextColor(tc.Accent).SingleLine()
			}
			ui.Box(c).Grow(1)
			rescanBtn := ui.Row(c.Key("db-rescan")).Height(24).Radius(6).Padding(0, 8).Gap(4).AlignItems(ui.Center).
				Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Cursor(ui.CursorPointer)
			rescanBtn.Children(func() {
				ui.Icon(c, icons.TablerRefresh).Width(11).Height(11).TextColor(tc.IconMuted)
				ui.Text(c, "Rescan").FontSize(11).TextColor(tc.Text)
			})
			if rescanBtn.Hovered() {
				rescanBtn.Background(tc.SurfaceRaised)
			}
			if rescanBtn.Clicked() {
				go dm.ScanDatabases()
			}
		})

		if len(snap.Databases) == 0 {
			ui.Column(c).Grow(1).Center().Padding(20).Gap(8).Children(func() {
				ui.Icon(c, icons.TablerDatabase).Width(28).Height(28).TextColor(tc.IconMuted)
				ui.Text(c, "No SQLite / DuckDB databases detected").FontSize(13).FontWeight(600).TextColor(tc.Text)
				ui.Text(c, "Add a .sqlite, .sqlite3, .db, or .duckdb file to your workspace.").FontSize(11.5).TextColor(tc.TextMuted)
			})
			return
		}

		// Database file switcher strip when workspace contains database files.
		ui.Row(c).Height(32).Padding(0, 12).Gap(6).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Background(tc.Surface).Children(func() {
			ui.Text(c, "DB:").FontSize(10).FontWeight(700).TextColor(tc.TextMuted)
			ui.ScrollHorizontal(c).Grow(1).Scrollbars(false).Children(func() {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					for _, dbPath := range snap.Databases {
						p := dbPath
						isSel := (p == snap.ActiveDBPath)
						pill := ui.Row(c.Key("db-switch-"+p)).Height(22).Radius(4).Padding(0, 8).AlignItems(ui.Center).Cursor(ui.CursorPointer)
						pill.Children(func() {
							txt := ui.Text(c, filepath.Base(p)).FontSize(11)
							if isSel {
								txt.FontWeight(600).TextColor(tc.AccentForeground)
							} else {
								txt.TextColor(tc.Text)
							}
						})
						if isSel {
							pill.Background(tc.Accent)
						} else if pill.Hovered() {
							pill.Background(tc.SurfaceRaised)
						}
						pill.Tooltip(p)
						if pill.Clicked() && !isSel {
							go dm.Inspect(p)
						}
					}
				})
			})
		})

		renderSQLConsole := func() {
			// The text input binds to a frame-local copy of the query; edits
			// sync back through SetSQLQuery (locked) so the ExecuteQuery
			// goroutine's locked read of dm.SQLQuery stays race-free.
			sqlQuery := snap.SQLQuery
			ui.Column(c).Grow(1).FillHeight().Children(func() {
				ui.Column(c).Padding(8, 12).Gap(6).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
					in := ui.TextInput(c, &sqlQuery).Placeholder("SELECT * FROM table LIMIT 25;").Grow(1)
					if in.Submitted() && !snap.IsExecuting {
						dm.SetSQLQuery(sqlQuery)
						go dm.ExecuteQuery()
					}

					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						runBtn := ui.Row(c.Key("db-exec")).Height(26).Radius(6).Padding(0, 10).Gap(5).AlignItems(ui.Center).
							Background(tc.Accent).Cursor(ui.CursorPointer)
						runBtn.Children(func() {
							ui.Icon(c, icons.TablerPlayerPlay).Width(11).Height(11).TextColor(tc.AccentForeground)
							ui.Text(c, "Execute").FontSize(11).FontWeight(600).TextColor(tc.AccentForeground)
						})
						if runBtn.Hovered() {
							runBtn.Background(tc.Accent.Alpha(0.85))
						}
						if runBtn.Clicked() {
							dm.SetSQLQuery(sqlQuery)
							go dm.ExecuteQuery()
						}
						ui.Box(c).Grow(1)
						if snap.StatusMsg != "" {
							ui.Text(c, snap.StatusMsg).FontSize(11).TextColor(tc.TextMuted)
						}
					})
				})

				if qr := snap.QueryResult; qr != nil && len(qr.Columns) > 0 {
					ui.ScrollBoth(c).Grow(1).Padding(6, 10).Children(func() {
						ui.Row(c).Padding(4, 6).Background(tc.Surface).Radius(4).Children(func() {
							for _, col := range qr.Columns {
								ui.Text(c, col).FontWeight(600).FontSize(11).TextColor(tc.Text).Width(100)
							}
						})

						for i, r := range qr.Rows {
							if r == nil {
								continue
							}
							ui.Row(c.Key(fmt.Sprintf("dbrow-%d", i))).Padding(3, 6).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.5)).Children(func() {
								for _, col := range qr.Columns {
									valStr := fmt.Sprintf("%v", r[col])
									if len(valStr) > 40 {
										valStr = valStr[:37] + "…"
									}
									ui.Text(c, valStr).FontSize(11).TextColor(tc.Text).Width(100)
								}
							})
						}
					})
				}
			})
			// Persist any in-frame edits back to the manager under lock.
			if sqlQuery != snap.SQLQuery {
				dm.SetSQLQuery(sqlQuery)
			}
		}

		if a.RightDrawerWidth < 460 {
			// Stacked: table pills at top + SQL console below
			if snap.Info != nil && len(snap.Info.Tables) > 0 {
				ui.Row(c).Height(32).Padding(0, 8).AlignItems(ui.Center).Gap(6).
					BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Background(tc.Surface).Children(func() {
					ui.Text(c, "TABLE:").FontSize(9).FontWeight(700).TextColor(tc.TextMuted)
					ui.ScrollHorizontal(c).Grow(1).Gap(3).AlignItems(ui.Center).Children(func() {
						for _, tb := range snap.Info.Tables {
							tName := tb.Name
							isSel := (tName == snap.ActiveTable)
							pill := ui.Row(c.Key("db-pill-"+tName)).Height(22).Radius(4).Padding(0, 6).Gap(4).AlignItems(ui.Center).Cursor(ui.CursorPointer)
							pill.Children(func() {
								txt := ui.Text(c, tName).FontSize(11)
								cnt := ui.Text(c, fmt.Sprintf("%d", tb.RowCount)).FontSize(9)
								if isSel {
									txt.FontWeight(600).TextColor(tc.AccentForeground)
									cnt.TextColor(tc.AccentForeground.Alpha(0.85))
								} else {
									txt.TextColor(tc.Text)
									cnt.TextColor(tc.TextMuted)
								}
							})
							if isSel {
								pill.Background(tc.Accent)
							} else if pill.Hovered() {
								pill.Background(tc.SurfaceRaised)
							}
							if pill.Clicked() {
								go dm.SelectTable(tName)
							}
						}
					})
				})
			}
			renderSQLConsole()
		} else {
			// Side by side with compact tables column (136px)
			ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
				ui.Column(c).Width(136).Background(tc.Surface).BorderWidth(0, 1, 0, 0).BorderColor(tc.Border).Children(func() {
					ui.Row(c).Padding(8, 10).Children(func() {
						ui.Text(c, "TABLES").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
					})

					if snap.Info != nil {
						ui.Scroll(c).Grow(1).Padding(2, 6).Children(func() {
							for _, tb := range snap.Info.Tables {
								tName := tb.Name
								isSel := tName == snap.ActiveTable
								row := ui.Row(c.Key("db-table-"+tName)).Padding(4, 6).Radius(5).Gap(4).AlignItems(ui.Center).Cursor(ui.CursorPointer)
								row.Children(func() {
									icColor := tc.IconMuted
									if isSel {
										icColor = tc.AccentForeground
									}
									ui.Icon(c, icons.TablerTable).Width(12).Height(12).TextColor(icColor)
									txt := ui.Text(c, tName).FontSize(11).Grow(1).SingleLine()
									cnt := ui.Text(c, fmt.Sprintf("%d", tb.RowCount)).FontSize(10)
									if isSel {
										txt.FontWeight(600).TextColor(tc.AccentForeground)
										cnt.TextColor(tc.AccentForeground.Alpha(0.85))
									} else {
										txt.TextColor(tc.Text)
										cnt.TextColor(tc.TextMuted)
									}
								})
								if isSel {
									row.Background(tc.Accent)
								} else if row.Hovered() {
									row.Background(tc.SurfaceRaised)
								}
								if row.Clicked() {
									go dm.SelectTable(tName)
								}
							}
						})
					}
				})

				renderSQLConsole()
			})
		}
	})
}

func (a *AppUI) checkoutBranch(branch string) {
	gm := a.Git
	if gm == nil {
		return
	}
	go func() {
		if err := gm.SwitchBranch(branch); err == nil {
			// Publish state mutations on the UI thread; the workspace rescan
			// (potentially slow over remote file systems) stays off-thread.
			a.runOnUI(func() {
				a.Workspace.GitBranch = branch
			})
			a.Workspace.Refresh()
			if a.Window != nil {
				a.Window.Invalidate()
			}
		}
	}()
	a.ShowBranchModal = false
}

func (a *AppUI) createBranch(branch string) {
	gm := a.Git
	if gm == nil || strings.TrimSpace(branch) == "" {
		return
	}
	go func() {
		if err := gm.CreateBranch(strings.TrimSpace(branch)); err == nil {
			a.runOnUI(func() {
				a.Workspace.GitBranch = branch
			})
			a.Workspace.Refresh()
			if a.Window != nil {
				a.Window.Invalidate()
			}
		}
	}()
	a.ShowBranchModal = false
}

func minF(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func maxF(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// renderSideFilePane draws the right sidebar's file preview tabs — files
// opened from the agent sidebar's explorer, as forge-ade's side preview:
// a tab strip plus the file in the MyGo editor view.
func (a *AppUI) renderSideFilePane(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	if len(a.SideFileTabs) == 0 {
		ui.Column(c).Grow(1).Center().Children(func() {
			ui.Text(c, "No file preview open").FontSize(12).TextColor(tc.TextMuted)
		})
		return
	}

	// File tab strip (forge-ade's sideFileTabs mirrored into pane tabs).
	ui.ScrollHorizontal(c).Height(34).Padding(0, 8).Gap(4).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
		for _, p := range a.SideFileTabs {
			thisPath := p
			active := a.ActiveSideFile == thisPath
			tab := ui.Row(c.Key("sidefile-"+thisPath)).Height(24).Radius(6).Padding(0, 8).Gap(5).AlignItems(ui.Center).Cursor(ui.CursorPointer)
			tab.Children(func() {
				icons.RenderFileIcon(c, thisPath, 13, tc.IconMuted)
				txt := ui.Text(c, filepath.Base(thisPath)).FontSize(11).SingleLine()
				if active {
					txt.FontWeight(600).TextColor(tc.Text)
				} else {
					txt.TextColor(tc.TextMuted)
				}
				// Right-click: close / close others / close to the right,
				// as AgentRightSidebar's tab context menu.
				tab.ContextMenu(func(m *ui.Menu) {
					if m.Item("Close").Chosen() {
						a.closeSideFile(thisPath)
					}
					if m.Item("Close Others").Chosen() {
						a.SideFileTabs = []string{thisPath}
						a.ActiveSideFile = thisPath
					}
					if m.Item("Close to the Right").Chosen() {
						for i, t := range a.SideFileTabs {
							if t == thisPath {
								a.SideFileTabs = a.SideFileTabs[:i+1]
								break
							}
						}
						found := false
						for _, t := range a.SideFileTabs {
							if t == a.ActiveSideFile {
								found = true
								break
							}
						}
						if !found {
							a.ActiveSideFile = thisPath
						}
					}
				})
				closeBtn := ui.Row(c.Key("sidefile-x-"+thisPath)).Size(14, 14).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
				closeBtn.Children(func() {
					ui.Icon(c, icons.TablerX).Width(9).Height(9).TextColor(tc.IconMuted)
				})
				if closeBtn.Hovered() {
					closeBtn.Background(tc.Text.Alpha(0.08))
				}
				if closeBtn.Clicked() {
					a.closeSideFile(thisPath)
				}
			})
			if active {
				tab.Background(tc.Accent.Alpha(0.14))
			} else if tab.Hovered() {
				tab.Background(tc.Text.Alpha(0.05))
			}
			if tab.Clicked() {
				a.ActiveSideFile = thisPath
			}
		}
	})

	// Special preview modes (markdown) render instead of the buffer.
	if a.ActiveSideFile == a.SideFilePreviewMd {
		ui.Column(c).Grow(1).FillHeight().Children(func() {
			a.renderSideFilePreviewBody(c, tc, func() {
				ui.Column(c).Grow(1).Center().Children(func() {
					ui.Text(c, "File unavailable").FontSize(12).TextColor(tc.TextMuted)
				})
			})
		})
		return
	}

	full := a.ActiveSideFile
	if a.Workspace != nil {
		full = a.Workspace.ResolveFullPath(a.ActiveSideFile)
	}

	// Image and SVG viewer preview in right panel
	if isImageOrSVG(a.ActiveSideFile) && a.SVGViewMode[full] != "code" {
		doc := a.Editor.GetDoc(full)
		a.renderImageViewer(c, tc, full, a.ActiveSideFile, doc, true)
		return
	}

	// Native PDF viewer preview in side file tabs
	if strings.HasSuffix(strings.ToLower(a.ActiveSideFile), ".pdf") {
		a.renderPdfViewerPane(c, tc, full, filepath.Base(a.ActiveSideFile))
		return
	}

	// Presentation & Motion preview in side file tabs
	if isDeckOrMotionFile(a.ActiveSideFile) {
		a.renderDeckPane(c)
		return
	}

	// Editor view of the active file.
	doc := a.Editor.GetDoc(full)
	if doc == nil && full != "" {
		if validatePreviewableFile(full) == nil {
			doc, _ = a.Editor.OpenFile(full, a.ActiveSideFile)
		}
	}
	if doc == nil {
		ui.Column(c).Grow(1).Center().Children(func() {
			ui.Text(c, "File unavailable").FontSize(12).TextColor(tc.TextMuted)
		})
		return
	}
	// Diff mode: show the file's git working-tree diff instead of the
	// content, as forge-ade's sideFileDiffMode toggle does.
	diffMode := a.SideFileDiffMode[a.ActiveSideFile]
	ui.Column(c).Grow(1).FillHeight().Children(func() {
		if diffMode {
			repoDir := a.Workspace.RootDir
			if repoDir == "" && a.Git != nil {
				repoDir = a.Git.Snapshot().RootDir
			}
			if repoDir == "" && a.Agent != nil {
				repoDir = a.Agent.WorkspacePath()
			}
			patch := ""
			diffReady := true
			if a.Git != nil {
				patch, diffReady = a.Git.GetCachedFileDiff(a.ActiveSideFile)
			} else if a.Engine != nil {
				patch = a.Engine.GetGitDiff(repoDir, a.ActiveSideFile)
			}
			if !diffReady {
				ui.Column(c).Grow(1).Center().Children(func() {
					ui.Spinner(c).Size(14, 14).Label("Loading diff")
				})
			} else if patch == "" {
				ui.Column(c).Grow(1).Center().Children(func() {
					ui.Text(c, "No uncommitted changes").FontSize(12).TextColor(tc.TextMuted)
				})
			} else if d := a.Editor.OpenPatchDiff(a.ActiveSideFile, patch, a.ThemeMgr); d != nil {
				mygoeditor.DiffView(c, d).Grow(1).FillWidth()
			}
		} else {
			edView := mygoeditor.View(c, doc.Editor).Grow(1).FillWidth()
			a.attachEditorContextMenu(c, edView, doc)
		}
		// Keep the drawer preview in sync with the editor-mode active tab.
		ui.Row(c).Height(26).Padding(0, 12).Gap(6).AlignItems(ui.Center).
			BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {
			ui.Text(c, a.ActiveSideFile).FontSize(10).TextColor(tc.TextMuted).SingleLine()
			ui.Box(c).Grow(1)
			diffToggle := ui.Text(c, "Diff").FontSize(10).Cursor(ui.CursorPointer)
			if diffMode {
				diffToggle.FontWeight(600).TextColor(tc.Accent)
			} else {
				diffToggle.TextColor(tc.TextMuted)
			}
			if diffToggle.Clicked() {
				a.SideFileDiffMode[a.ActiveSideFile] = !diffMode
			}
			if ui.Text(c, "Open in Editor").FontSize(10).TextColor(tc.Accent).Padding(0, 0, 0, 10).Cursor(ui.CursorPointer).Clicked() {
				_, _ = a.Editor.OpenFile(full, a.ActiveSideFile)
				a.Mode = ModeEditor
			}
		})
	})
}

// renderHunkRevertStrip lists the working-tree hunks of a file with per-hunk
// Revert buttons (GetGitFileDiffHunks / RevertGitHunk parity) above the diff.
func (a *AppUI) renderHunkRevertStrip(c *ui.Context, tc theme.ThemeColors, gm *git.GitManager, path, patch string) {
	if gm == nil || path == "" || patch == "" {
		return
	}
	var hunkHeaders []string
	for _, ln := range strings.Split(patch, "\n") {
		if strings.HasPrefix(ln, "@@ ") {
			hunkHeaders = append(hunkHeaders, strings.TrimSpace(ln))
		}
	}
	if len(hunkHeaders) == 0 {
		return
	}
	if len(a.HunkRevertPaths) == 0 {
		a.HunkRevertPaths = make(map[string]bool)
	}
	strip := ui.Column(c.Key("hunks-"+path)).Gap(2).Padding(4, 8).
		BorderWidth(1, 1, 0, 1).BorderColor(tc.Border.Alpha(0.5)).
		Background(tc.SurfaceRaised.Alpha(0.3))
	strip.Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, fmt.Sprintf("Hunks (%d)", len(hunkHeaders))).FontSize(10).FontWeight(700).TextColor(tc.TextMuted)
			ui.Box(c).Grow(1)
			ui.Text(c, "revert restores the hunk to HEAD").FontSize(9).TextColor(tc.SecondaryLabel)
		})
		for i, header := range hunkHeaders {
			i, header := i, header
			row := ui.Row(c.Key(fmt.Sprintf("hunk-%s-%d", path, i))).Height(24).Radius(6).Padding(0, 6).Gap(6).AlignItems(ui.Center)
			row.Children(func() {
				ui.Text(c, header).FontSize(10).Font("monospace").TextColor(tc.Accent).Grow(1).SingleLine()
				revertKey := path + "#" + fmt.Sprint(i)
				reverted := a.HunkRevertPaths[revertKey]
				if reverted {
					ui.Text(c, "reverted").FontSize(9).TextColor(tc.Success)
				} else {
					btn := ui.Row(c).Height(20).Radius(4).Padding(0, 7).AlignItems(ui.Center).
						Background(tc.Warning.Alpha(0.14)).BorderWidth(1).BorderColor(tc.Warning.Alpha(0.4)).Cursor(ui.CursorPointer)
					btn.Children(func() {
						ui.Text(c, "Revert").FontSize(9).FontWeight(600).TextColor(tc.Warning)
					})
					if btn.Clicked() {
						a.runOnUI(func() {
							a.HunkRevertPaths[revertKey] = true
						})
						go func() {
							if rerr := gm.RevertDiffHunk(path, i); rerr != nil {
								// Roll the marker back on the UI thread so the
								// render pass never races the revert goroutine.
								a.runOnUI(func() {
									a.HunkRevertPaths[revertKey] = false
									a.queueToast(ui.Toast{Title: "Revert Hunk Failed", Description: rerr.Error(), Type: "error"})
								})
							} else {
								gm.Refresh()
								a.runOnUI(func() {
									for k := range a.HunkRevertPaths {
										if strings.HasPrefix(k, path+"#") {
											delete(a.HunkRevertPaths, k)
										}
									}
									a.queueToast(ui.Toast{Title: "Hunk Reverted", Description: filepath.Base(path), Type: "info"})
								})
							}
						}()
					}
				}
			})
			if row.Hovered() {
				row.Background(tc.Text.Alpha(0.04))
			}
		}
	})
}
