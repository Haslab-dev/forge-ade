package ui

import (
	"fmt"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/internal/app"
)

// AppUI is the top-level native UI application state.
type AppUI struct {
	Backend      *app.App
	Window       *mygo.Window
	Workspace    *WorkspaceState
	Editor       *EditorManager
	Terminal     *TerminalState
	Agent        *AgentState
	ActiveTab    int
	SelectedFile string
}

// NewAppUI constructs the AppUI state tree.
func NewAppUI(backend *app.App, rootDir string) *AppUI {
	appUI := &AppUI{
		Backend:   backend,
		Workspace: NewWorkspaceState(rootDir),
		Editor:    NewEditorManager(),
		Terminal:  NewTerminalState(rootDir),
		ActiveTab: TabEditor,
	}

	appUI.Agent = NewAgentState(backend, func() {
		if appUI.Window != nil {
			// Trigger next GPU frame redraw
			appUI.Window.Invalidate()
		}
	})

	// Pre-open the first file in workspace if available
	if len(appUI.Workspace.Files) > 0 {
		first := appUI.Workspace.Files[0]
		appUI.SelectedFile = first.RelativePath
		_, _ = appUI.Editor.OpenFile(first.FullPath, first.RelativePath)
	}

	// Update Git status from backend if available
	if backend != nil {
		if st, err := backend.GetGitStatus(rootDir); err == nil && st != nil {
			appUI.Workspace.GitBranch = st.Branch
			appUI.Workspace.GitDirty = len(st.Staged)+len(st.Unstaged)+len(st.Untracked) > 0
		}
	}

	return appUI
}

// SetWindow attaches the native window reference for lifecycle and redraw callbacks.
func (a *AppUI) SetWindow(w *mygo.Window) {
	a.Window = w
}

// View builds the entire native GPU frame.
func (a *AppUI) View(c *ui.Context) {
	t := c.Theme()

	ui.Column(c).Fill().Children(func() {
		// 1. Top Global Navigation Bar
		ui.Row(c).Gap(16).Padding(8, 16).AlignItems(ui.Center).Background(t.Surface).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Text(c, "ForgeADE Native").Bold().FontSize(15)
			ui.Text(c, filepath.Base(a.Workspace.RootDir)).FontSize(12).TextColor(t.TextMuted)

			ui.Box(c).Grow(1)

			// Central Tabs
			ui.Segmented(c, &a.ActiveTab, TabNames...).Label("Views")

			ui.Box(c).Grow(1)

			// Git info & Badge
			if a.Workspace.GitBranch != "" {
				branchLabel := fmt.Sprintf("⎇ %s", a.Workspace.GitBranch)
				if a.Workspace.GitDirty {
					branchLabel += " *"
				}
				ui.Text(c, branchLabel).FontSize(12).TextColor(t.Accent)
			}
			ui.Text(c, "Pure Go · GPU").FontSize(11).TextColor(t.TextMuted)
		})

		// 2. Middle Body: Workspace Sidebar + Active Content View
		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			// Left Workspace Sidebar
			ui.Column(c).Width(240).Background(t.Surface).BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Children(func() {
				// Sidebar Header
				ui.Row(c).Padding(10, 14).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
					ui.Text(c, "WORKSPACE").FontSize(11).Bold().TextColor(t.TextMuted)
					ui.Box(c).Grow(1)
					if ui.Button(c, "↻").Clicked() {
						a.Workspace.Refresh()
					}
				})

				// File filter input
				ui.Row(c).Padding(6, 12).Children(func() {
					ui.TextInput(c, &a.Workspace.FilterQuery).Placeholder("Filter files...").Grow(1)
				})

				// File list
				filtered := a.Workspace.FilteredFiles()
				ui.Sidebar(c, &a.SelectedFile, func() {
					for _, f := range filtered {
						label := f.RelativePath
						doc := a.Editor.ActiveDoc()
						if doc != nil && doc.FullPath == f.FullPath && doc.Editor.Buffer.Modified() {
							label += " •"
						}
						item := ui.SidebarItem(c, f.RelativePath, nil, label)
						if item.Clicked() {
							a.SelectedFile = f.RelativePath
							fullPath := a.Workspace.ResolveFullPath(f.RelativePath)
							_, _ = a.Editor.OpenFile(fullPath, f.RelativePath)
							if a.ActiveTab != TabEditor && a.ActiveTab != TabSplit {
								a.ActiveTab = TabEditor
							}
						}
					}
				}).Grow(1).Label("Files")
			})

			// Main Content View
			ui.Column(c).Grow(1).FillHeight().Children(func() {
				switch a.ActiveTab {
				case TabEditor:
					a.Editor.RenderEditor(c)
				case TabDiff:
					a.Editor.RenderDiff(c)
				case TabTerminal:
					a.Terminal.RenderTerminal(c)
				case TabAgent:
					a.Agent.RenderChat(c)
				case TabSplit:
					// Split View: Editor on Left, Agent on Right
					ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
						ui.Column(c).Grow(1).FillHeight().BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Children(func() {
							a.Editor.RenderEditor(c)
						})
						ui.Column(c).Width(450).FillHeight().Children(func() {
							a.Agent.RenderChat(c)
						})
					})
				}
			})
		})

		// 3. Global Status Bar
		ui.Row(c).Gap(16).Padding(4, 16).AlignItems(ui.Center).Background(t.Surface).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
			ui.Text(c, ShortenPath(a.Workspace.RootDir, a.Workspace.RootDir)).FontSize(12).TextColor(t.TextMuted)
			if doc := a.Editor.ActiveDoc(); doc != nil {
				ui.Text(c, doc.RelativePath).FontSize(12)
			}
			ui.Box(c).Grow(1)
			ui.Text(c, a.Agent.ActiveModel).FontSize(12).TextColor(t.TextMuted)
			ui.Text(c, "MyGo v0.8.3").FontSize(11).TextColor(t.TextMuted)
		})
	})
}
