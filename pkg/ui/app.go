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
	Backend          *app.App
	Window           *mygo.Window
	Workspace        *WorkspaceState
	Editor           *EditorManager
	Terminal         *TerminalState
	Agent            *AgentState
	Git              *GitManager
	Database         *DatabaseManager
	Palette          *PaletteManager
	Settings         *SettingsManager
	ActiveTab        int
	SelectedFile     string
	ShowSidebar      bool
	NewFileName      string
	ShowNewFileModal bool
}

// NewAppUI constructs the AppUI state tree.
func NewAppUI(backend *app.App, rootDir string) *AppUI {
	appUI := &AppUI{
		Backend:     backend,
		Workspace:   NewWorkspaceState(rootDir),
		Editor:      NewEditorManager(),
		Terminal:    NewTerminalState(rootDir),
		ActiveTab:   TabEditor,
		ShowSidebar: true,
	}

	appUI.Agent = NewAgentState(backend, func() {
		if appUI.Window != nil {
			appUI.Window.Invalidate()
		}
	})

	appUI.Settings = NewSettingsManager(backend, func(provider, model string) {
		appUI.Agent.ActiveModel = model
	})

	appUI.Database = NewDatabaseManager(backend, appUI.Workspace)

	appUI.Git = NewGitManager(backend, rootDir, func(path string, showDiff bool) {
		fullPath := appUI.Workspace.ResolveFullPath(path)
		_, _ = appUI.Editor.OpenFile(fullPath, path)
		appUI.SelectedFile = path
		if showDiff {
			appUI.ActiveTab = TabDiff
		} else {
			appUI.ActiveTab = TabEditor
		}
	})

	appUI.Palette = NewPaletteManager(appUI.Workspace, func(path string) {
		fullPath := appUI.Workspace.ResolveFullPath(path)
		_, _ = appUI.Editor.OpenFile(fullPath, path)
		appUI.SelectedFile = path
		appUI.ActiveTab = TabEditor
	})

	// Pre-open first file in workspace
	if len(appUI.Workspace.Files) > 0 {
		first := appUI.Workspace.Files[0]
		appUI.SelectedFile = first.RelativePath
		_, _ = appUI.Editor.OpenFile(first.FullPath, first.RelativePath)
	}

	// Initial Git status sync
	if appUI.Git != nil && appUI.Git.Branch != "" {
		appUI.Workspace.GitBranch = appUI.Git.Branch
		appUI.Workspace.GitDirty = len(appUI.Git.Staged)+len(appUI.Git.Unstaged)+len(appUI.Git.Untracked) > 0
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

	// Global Keyboard Shortcuts
	if c.Shortcut(ui.Cmd, ui.KeyS) {
		if err := a.Editor.SaveActiveFile(); err == nil {
			doc := a.Editor.ActiveDoc()
			name := "File"
			if doc != nil {
				name = doc.RelativePath
			}
			c.AddToast(ui.Toast{
				Title:       "Saved",
				Description: fmt.Sprintf("%s saved to disk", name),
				Type:        "success",
			})
		}
	}
	if c.Shortcut(ui.Cmd, ui.KeyP) {
		a.Palette.Toggle()
	}
	if c.Shortcut(ui.Cmd, ui.KeyComma) {
		a.Settings.Toggle()
	}
	if c.Shortcut(ui.Cmd, ui.KeyB) {
		a.ShowSidebar = !a.ShowSidebar
	}
	if c.Shortcut(ui.Cmd, ui.Key1) {
		a.ActiveTab = TabEditor
	}
	if c.Shortcut(ui.Cmd, ui.Key2) {
		a.ActiveTab = TabDiff
	}
	if c.Shortcut(ui.Cmd, ui.Key3) {
		a.ActiveTab = TabTerminal
	}
	if c.Shortcut(ui.Cmd, ui.Key4) {
		a.ActiveTab = TabAgent
	}
	if c.Shortcut(ui.Cmd, ui.Key5) {
		a.ActiveTab = TabGit
	}
	if c.Shortcut(ui.Cmd, ui.Key6) {
		a.ActiveTab = TabDatabase
	}
	if c.Shortcut(ui.Cmd, ui.Key7) {
		a.ActiveTab = TabSplit
	}

	ui.Column(c).Fill().Children(func() {
		// 1. Top Global Navigation Bar
		ui.Row(c).Gap(10).Padding(8, 16).AlignItems(ui.Center).Background(t.Surface).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			// Sidebar toggle button
			sidebarToggle := ui.Button(c, map[bool]string{true: "◨", false: "◻"}[a.ShowSidebar]).FontSize(12)
			if sidebarToggle.Clicked() {
				a.ShowSidebar = !a.ShowSidebar
			}

			ui.Text(c, "ForgeADE Native").Bold().FontSize(15)
			ui.Text(c, filepath.Base(a.Workspace.RootDir)).FontSize(12).TextColor(t.TextMuted)

			// Quick Open Search Button
			searchBtn := ui.Button(c, "🔍 Search (⌘P)").FontSize(12)
			if searchBtn.Clicked() {
				a.Palette.Toggle()
			}

			ui.Box(c).Grow(1)

			// Central Navigation Tabs
			ui.Segmented(c, &a.ActiveTab, TabNames...).Label("Views")

			ui.Box(c).Grow(1)

			// Git Branch Badge
			if a.Git != nil && a.Git.Branch != "" {
				dirtyMarker := ""
				if len(a.Git.Staged)+len(a.Git.Unstaged)+len(a.Git.Untracked) > 0 {
					dirtyMarker = " *"
				}
				branchBtn := ui.Button(c, fmt.Sprintf("⎇ %s%s", a.Git.Branch, dirtyMarker)).FontSize(12)
				if branchBtn.Clicked() {
					a.ActiveTab = TabGit
				}
			}

			// Settings Button
			settingsBtn := ui.Button(c, "⚙ Settings (⌘,)").FontSize(12)
			if settingsBtn.Clicked() {
				a.Settings.Toggle()
			}

			ui.Text(c, "Pure Go · GPU").FontSize(11).TextColor(t.TextMuted)
		})

		// 2. Middle Body: Workspace Sidebar + Active Content View
		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			// Optional Workspace Sidebar
			if a.ShowSidebar {
				ui.Column(c).Width(240).Background(t.Surface).BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Children(func() {
					// Sidebar Header & Actions
					ui.Row(c).Padding(10, 14).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
						ui.Text(c, "FILES").FontSize(11).Bold().TextColor(t.TextMuted)
						ui.Box(c).Grow(1)
						if ui.Button(c, "+ File").FontSize(11).Clicked() {
							a.ShowNewFileModal = true
							a.NewFileName = ""
						}
						if ui.Button(c, "↻").FontSize(11).Clicked() {
							a.Workspace.Refresh()
							if a.Git != nil {
								go a.Git.Refresh()
							}
							if a.Database != nil {
								go a.Database.ScanDatabases()
							}
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

								// If database file clicked, jump to database tab!
								lower := f.RelativePath
								if filepath.Ext(lower) == ".sqlite" || filepath.Ext(lower) == ".db" || filepath.Ext(lower) == ".duckdb" {
									if a.Database != nil {
										a.Database.Inspect(fullPath)
										a.ActiveTab = TabDatabase
									}
									return
								}

								_, _ = a.Editor.OpenFile(fullPath, f.RelativePath)
								if a.ActiveTab != TabEditor && a.ActiveTab != TabSplit {
									a.ActiveTab = TabEditor
								}
							}
						}
					}).Grow(1).Label("Files")
				})
			}

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
				case TabGit:
					if a.Git != nil {
						a.Git.RenderGit(c)
					}
				case TabDatabase:
					if a.Database != nil {
						a.Database.RenderDatabase(c)
					}
				case TabSplit:
					// Split View: Editor on Left, Agent on Right
					ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
						ui.Column(c).Grow(1).FillHeight().BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Children(func() {
							a.Editor.RenderEditor(c)
						})
						ui.Column(c).Width(460).FillHeight().Children(func() {
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
			ui.Text(c, "⌘S Save · ⌘P Open · ⌘, Settings · ⌘B Sidebar · ⌘1-7 Tabs").FontSize(11).TextColor(t.TextMuted)
		})

		// 4. Floating Quick Open Palette Modal Overlay
		if a.Palette != nil {
			a.Palette.RenderPalette(c)
		}

		// 5. Settings Modal Overlay
		if a.Settings != nil {
			a.Settings.RenderSettings(c)
		}

		// 6. New File Modal Overlay
		if a.ShowNewFileModal {
			ui.Modal(c, &a.ShowNewFileModal, func() {
				ui.Column(c).Width(400).Radius(10).Background(t.Surface).BorderWidth(1).BorderColor(t.Border).Padding(16, 18).Gap(10).Children(func() {
					ui.Text(c, "Create New File").Bold().FontSize(14)
					in := ui.TextInput(c, &a.NewFileName).Placeholder("filename.go (or path/to/file.ext)...").Grow(1).AutoFocus()
					if in.Submitted() && a.NewFileName != "" {
						_ = a.Workspace.CreateFile(a.NewFileName)
						a.ShowNewFileModal = false
						full := a.Workspace.ResolveFullPath(a.NewFileName)
						_, _ = a.Editor.OpenFile(full, a.NewFileName)
						a.SelectedFile = a.NewFileName
						a.ActiveTab = TabEditor
					}
					ui.Row(c).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Grow(1)
						if ui.Button(c, "Cancel").Clicked() {
							a.ShowNewFileModal = false
						}
						if ui.PrimaryButton(c, "Create").Clicked() && a.NewFileName != "" {
							_ = a.Workspace.CreateFile(a.NewFileName)
							a.ShowNewFileModal = false
							full := a.Workspace.ResolveFullPath(a.NewFileName)
							_, _ = a.Editor.OpenFile(full, a.NewFileName)
							a.SelectedFile = a.NewFileName
							a.ActiveTab = TabEditor
						}
					})
				})
			})
		}
	})
}
