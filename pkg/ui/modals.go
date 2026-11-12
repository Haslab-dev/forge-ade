package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/icons"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/palette"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/settings"
)

// renderModals orchestrates all popup modals and overlay dialogs.
func (a *AppUI) renderModals(c *ui.Context) {
	if a.Palette != nil && a.Palette.IsOpen {
		a.renderPaletteModal(c)
	}
	if a.Settings != nil && a.Settings.IsOpen {
		a.renderSettingsModal(c)
	}
	if a.ThemeMgr != nil && a.ThemeMgr.IsOpen {
		a.renderThemePickerModal(c)
	}
	if a.ACPMgr != nil && a.ACPMgr.ActiveReq != nil {
		a.renderPermissionModal(c)
	}
	if a.ShowNewFileModal {
		a.renderNewFileModal(c)
	}
	if a.ShowNewFolderModal {
		a.renderNewFolderModal(c)
	}
	if a.ShowNewTaskModal {
		a.renderNewTaskModal(c)
	}
	if a.ShowTaskMenuModal {
		a.renderTaskActionsMenu(c)
	}
	if a.ShowRenameModal {
		a.renderRenameModal(c)
	}
	if a.ShowBranchModal {
		a.renderBranchModal(c)
	}
}

func (a *AppUI) renderPaletteModal(c *ui.Context) {
	pm := a.Palette
	tc := a.ThemeMgr.Colors()

	if c.Shortcut(0, ui.KeyEscape) {
		pm.Close()
	}

	matches := pm.FilteredFiles()
	commands := pm.FilteredCommands()
	mode := pm.Mode

	activate := func() {
		if mode == palette.PaletteFiles {
			if pm.SelectedIndex < len(matches) {
				selected := matches[pm.SelectedIndex].File.RelativePath
				pm.Close()
				full := a.Workspace.ResolveFullPath(selected)
				_, _ = a.Editor.OpenFile(full, selected)
				a.SelectedFile = selected
				a.Mode = ModeEditor
			}
		} else {
			if pm.SelectedIndex < len(commands) {
				cmd := commands[pm.SelectedIndex]
				pm.Close()
				if cmd.Run != nil {
					cmd.Run()
				}
			}
		}
	}

	ui.Modal(c, &pm.IsOpen, func() {
		panel := ui.Column(c).Width(576).Radius(16).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).
			Shadow(0, 24, 72, -20, ui.RGBA(0, 0, 0, 0.55))
		panel.Children(func() {
			// Search row
			ui.Row(c).Padding(14).Gap(8).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
				icon := icons.TablerSearch
				if mode == palette.PaletteCommands {
					icon = icons.TablerChevronRight
				}
				ui.Icon(c, icon).Width(15).Height(15).TextColor(tc.IconMuted)
				placeholder := "Search files by name..."
				if mode == palette.PaletteCommands {
					placeholder = "Type a command..."
				}
				in := ui.TextInput(c, &pm.Query).Placeholder(placeholder).Grow(1).AutoFocus()
				if in.Submitted() {
					activate()
				}
				kbd := ui.Row(c).Radius(4).Padding(1, 6).Background(tc.ToolbarControl)
				kbd.Children(func() {
					ui.Text(c, "ESC").FontSize(12)
				})
			})

			// Result list
			ui.Scroll(c).MaxHeight(320).Children(func() {
				ui.Column(c).Padding(6).Gap(2).Children(func() {
					if mode == palette.PaletteFiles {
						for i, m := range matches {
							i, path := i, m.File.RelativePath
							selected := (i == pm.SelectedIndex)
							row := ui.Row(c.Key("pal-"+path)).Radius(10).Padding(8, 12).Gap(8).AlignItems(ui.Center)
							row.Children(func() {
								ui.Icon(c, icons.FileIconSVG(path)).Width(13).Height(13).TextColor(tc.IconMuted)
								ui.Text(c, path).FontSize(13).Grow(1)
								if selected {
									ui.Icon(c, icons.TablerChevronRight).Width(12).Height(12).TextColor(tc.Accent)
								}
							})
							if selected {
								row.Background(tc.Accent.Alpha(0.12))
							} else if row.Hovered() {
								row.Background(tc.Text.Alpha(0.04))
							}
							if row.Clicked() {
								pm.SelectedIndex = i
								activate()
							}
						}
					} else {
						for i, cmd := range commands {
							i, thisCmd := i, cmd
							selected := (i == pm.SelectedIndex)
							row := ui.Row(c.Key("palcmd-"+thisCmd.ID)).Radius(10).Padding(8, 12).Gap(8).AlignItems(ui.Center)
							row.Children(func() {
								ui.Text(c, thisCmd.Label).FontSize(13).Grow(1)
								if thisCmd.Shortcut != "" {
									ui.Text(c, thisCmd.Shortcut).FontSize(11).TextColor(tc.SecondaryLabel)
								}
							})
							if selected {
								row.Background(tc.Accent.Alpha(0.12))
							} else if row.Hovered() {
								row.Background(tc.Text.Alpha(0.04))
							}
							if row.Clicked() {
								pm.SelectedIndex = i
								activate()
							}
						}
					}
				})
			})
		})
	})
}

func (a *AppUI) renderSettingsModal(c *ui.Context) {
	sm := a.Settings
	if sm == nil {
		return
	}
	t := c.Theme()
	tc := a.ThemeMgr.Colors()

	ui.Modal(c, &sm.IsOpen, func() {
		ui.Column(c).Width(640).Radius(12).Background(t.Surface).BorderWidth(1).BorderColor(t.Border).Padding(16, 20).Gap(12).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					TablerIcon(c, icons.TablerSettings, 16)
					ui.Text(c, "Settings & Preferences").Bold().FontSize(15)
				})
				ui.Box(c).Grow(1)
				closeBtn := ui.Button(c, "")
				closeBtn.Children(func() {
					TablerIcon(c, icons.TablerX, 13)
				})
				if closeBtn.Clicked() {
					sm.IsOpen = false
				}
			})

			// Navigation Tabs
			ui.Row(c).Gap(6).Children(func() {
				tabs := []struct {
					Idx   int
					Label string
					Icon  *ui.SVG
				}{
					{settings.SettingsTabModels, "Models & Providers", icons.TablerBrain},
					{settings.SettingsTabAppearance, "Appearance", icons.TablerPalette},
					{settings.SettingsTabAgents, "ACP Agents", icons.TablerPlug},
					{settings.SettingsTabShortcuts, "Shortcuts", icons.TablerKeyboard},
				}
				for _, tab := range tabs {
					thisTab := tab
					var btn ui.Element
					if sm.ActiveTab == thisTab.Idx {
						btn = ui.PrimaryButton(c, "")
					} else {
						btn = ui.Button(c, "")
					}
					btn.Children(func() {
						ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
							TablerIcon(c, thisTab.Icon, 12)
							ui.Text(c, thisTab.Label).FontSize(11).Bold()
						})
					})
					if btn.Clicked() {
						sm.ActiveTab = thisTab.Idx
					}
				}
			})

			// Tab 0: Models
			if sm.ActiveTab == settings.SettingsTabModels {
				ui.Column(c).Gap(10).Children(func() {
					ui.Text(c, "Provider").Bold().FontSize(12)
					ui.Select(c, &sm.Provider, settings.SupportedLLMProviders)
					ui.Text(c, "Model Name").Bold().FontSize(12)
					ui.TextInput(c, &sm.Model).Placeholder("e.g. claude-3-5-sonnet, gpt-4o, gemini-2.5-pro").Grow(1)
					ui.Text(c, "API Key").Bold().FontSize(12)
					ui.TextInput(c, &sm.APIKey).Placeholder("Enter provider API Key...").Password().Grow(1)

					ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
						ui.Box(c).Grow(1)
						if ui.PrimaryButton(c, "Save Credentials").Clicked() {
							_ = sm.Save()
						}
					})
				})
			}

			// Tab 1: Appearance
			if sm.ActiveTab == settings.SettingsTabAppearance {
				ui.Column(c).Gap(10).Children(func() {
					ui.Text(c, "Color Theme").Bold().FontSize(12)
					ui.Row(c).Gap(8).Children(func() {
						for _, p := range a.ThemeMgr.AvailablePalettes() {
							thisP := p
							btn := ui.Button(c, thisP.Name)
							if btn.Clicked() {
								a.ThemeMgr.PaletteID = thisP.ID
							}
						}
					})
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "Dark Mode").FontSize(12)
						if ui.Button(c, "Toggle Dark/Light").Clicked() {
							a.toggleAppearance()
						}
					})
				})
			}

			// Tab 2: ACP Agents
			if sm.ActiveTab == settings.SettingsTabAgents && a.ACPMgr != nil {
				ui.Scroll(c).MaxHeight(280).Children(func() {
					ui.Column(c).Gap(6).Children(func() {
						for _, ag := range a.ACPMgr.Agents {
							ui.Row(c).Padding(6, 8).Radius(6).Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border).Children(func() {
								ui.Column(c).Grow(1).Children(func() {
									ui.Text(c, ag.Name).Bold().FontSize(12)
									ui.Text(c, ag.Description).FontSize(11).TextColor(tc.TextMuted)
								})
								if ag.Available {
									Badge(c, tc, "AVAILABLE", tc.Success)
								} else {
									Badge(c, tc, "OFFLINE", tc.TextMuted)
								}
							})
						}
					})
				})
			}

			// Tab 3: Shortcuts
			if sm.ActiveTab == settings.SettingsTabShortcuts {
				shortcuts := []struct{ Key, Desc string }{
					{"⌘S", "Save current buffer to disk"},
					{"⌘P", "Quick open file by fuzzy name"},
					{"⇧⌘P", "Command palette"},
					{"⌘B", "Toggle left sidebar"},
					{"⌘J", "Toggle bottom terminal"},
					{"⌘1", "Switch to Agent mode"},
					{"⌘2", "Switch to Editor mode"},
					{"⌘N", "New task or session"},
					{"⌘W", "Close active tab"},
				}
				ui.Scroll(c).MaxHeight(280).Children(func() {
					ui.Column(c).Gap(4).Children(func() {
						for _, sc := range shortcuts {
							ui.Row(c).Padding(4, 8).AlignItems(ui.Center).Children(func() {
								ui.Text(c, sc.Key).Bold().FontSize(12).Width(80).TextColor(tc.Accent)
								ui.Text(c, sc.Desc).FontSize(12)
							})
						}
					})
				})
			}
		})
	})
}

func (a *AppUI) renderThemePickerModal(c *ui.Context) {
	tm := a.ThemeMgr
	if tm == nil {
		return
	}
	tc := tm.Colors()

	ui.Modal(c, &tm.IsOpen, func() {
		ui.Column(c).Width(400).Radius(14).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).Padding(16).Gap(10).
			Shadow(0, 24, 72, -20, ui.RGBA(0, 0, 0, 0.55)).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerPalette).Width(16).Height(16).TextColor(tc.Accent)
					ui.Text(c, "Theme & Appearance").FontSize(14).FontWeight(600)
				})
				ui.Box(c).Grow(1)
				if IconButton(c, tc, icons.TablerX, 13).Clicked() {
					tm.IsOpen = false
				}
			})

			for _, p := range tm.AvailablePalettes() {
				thisP := p
				isSel := (tm.PaletteID == thisP.ID)
				row := ui.Row(c.Key("pal-pick-"+thisP.ID)).Radius(8).Padding(8, 12).Gap(8).AlignItems(ui.Center)
				row.Children(func() {
					ui.Text(c, thisP.Name).FontSize(13).FontWeight(500).Grow(1)
					if isSel {
						ui.Icon(c, icons.TablerCheck).Width(14).Height(14).TextColor(tc.Accent)
					}
				})
				if isSel {
					row.Background(tc.Accent.Alpha(0.12))
				} else if row.Hovered() {
					row.Background(tc.Text.Alpha(0.04))
				}
				if row.Clicked() {
					tm.PaletteID = thisP.ID
					tm.IsOpen = false
				}
			}
		})
	})
}

func (a *AppUI) renderPermissionModal(c *ui.Context) {
	mgr := a.ACPMgr
	if mgr == nil || mgr.ActiveReq == nil {
		return
	}
	req := mgr.ActiveReq
	tc := a.ThemeMgr.Colors()

	ui.Modal(c, nil, func() {
		ui.Column(c).Width(480).Radius(12).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).Padding(16, 18).Gap(10).
			Shadow(0, 24, 64, -24, ui.RGBA(0, 0, 0, 0.45)).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons.TablerShieldAlert).Width(18).Height(18).TextColor(tc.Warning)
				ui.Text(c, "Agent Permission Request").FontSize(14).FontWeight(600)
			})
			ui.Text(c, req.Description).FontSize(12).TextColor(tc.TextMuted)
			if req.Command != "" {
				ui.Box(c).Padding(8).Radius(6).Background(tc.Canvas).Children(func() {
					ui.Text(c, req.Command).FontSize(11)
				})
			}
			ui.Row(c).AlignItems(ui.Center).Gap(6).Justify(ui.End).Children(func() {
				ui.Box(c).Grow(1)
				if ui.Button(c, "Deny").Clicked() {
					_ = mgr.RespondPermission(req.SessionID, req.RequestID, false)
				}
				if ui.PrimaryButton(c, "Allow").Clicked() {
					_ = mgr.RespondPermission(req.SessionID, req.RequestID, true)
				}
			})
		})
	})
}

func (a *AppUI) renderNewFileModal(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	ui.Modal(c, &a.ShowNewFileModal, func() {
		ui.Column(c).Width(420).Radius(12).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).Padding(16, 18).Gap(10).
			Shadow(0, 24, 64, -24, ui.RGBA(0, 0, 0, 0.45)).
			Children(func() {
				ui.Text(c, "Create New File").FontSize(14).FontWeight(600)
				in := ui.TextInput(c, &a.NewFileName).Placeholder("filename.go (or path/to/file.ext)...").Grow(1).AutoFocus()
				create := func() {
					if a.NewFileName == "" {
						return
					}
					_ = a.Workspace.CreateFile(a.NewFileName)
					a.ShowNewFileModal = false
					full := a.Workspace.ResolveFullPath(a.NewFileName)
					_, _ = a.Editor.OpenFile(full, a.NewFileName)
					a.SelectedFile = a.NewFileName
					a.Mode = ModeEditor
				}
				if in.Submitted() {
					create()
				}
				ui.Row(c).AlignItems(ui.Center).Gap(6).Justify(ui.End).Children(func() {
					ui.Box(c).Grow(1)
					if ui.Button(c, "Cancel").Clicked() {
						a.ShowNewFileModal = false
					}
					if ui.PrimaryButton(c, "Create").Clicked() {
						create()
					}
				})
			})
	})
}

func (a *AppUI) renderNewFolderModal(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	ui.Modal(c, &a.ShowNewFolderModal, func() {
		ui.Column(c).Width(420).Radius(12).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).Padding(16, 18).Gap(10).
			Shadow(0, 24, 64, -24, ui.RGBA(0, 0, 0, 0.45)).
			Children(func() {
				ui.Text(c, "Create New Folder").FontSize(14).FontWeight(600)
				in := ui.TextInput(c, &a.NewFolderName).Placeholder("folder_name (or path/to/folder)...").Grow(1).AutoFocus()
				if in.Submitted() && a.NewFolderName != "" {
					_ = a.Workspace.CreateFolder(a.NewFolderName)
					a.ShowNewFolderModal = false
				}
				ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
					ui.Box(c).Grow(1)
					if ui.Button(c, "Cancel").Clicked() {
						a.ShowNewFolderModal = false
					}
					if ui.PrimaryButton(c, "Create").Clicked() && a.NewFolderName != "" {
						_ = a.Workspace.CreateFolder(a.NewFolderName)
						a.ShowNewFolderModal = false
					}
				})
			})
	})
}

func (a *AppUI) renderNewTaskModal(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	ui.Modal(c, &a.ShowNewTaskModal, func() {
		ui.Column(c).Width(580).Radius(12).Background(tc.Sidebar).BorderWidth(1).BorderColor(tc.Border).Padding(0, 0, 12, 0).Gap(10).
			Shadow(0, 24, 72, -20, ui.RGBA(0, 0, 0, 0.5)).
			Children(func() {
				ui.Row(c).Height(48).Padding(0, 16).AlignItems(ui.Center).
					BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.6)).Children(func() {
					ui.Text(c, "New Task").FontSize(14).FontWeight(500)
					ui.Box(c).Grow(1)
					if IconButton(c, tc, icons.TablerX, 13).Clicked() {
						a.ShowNewTaskModal = false
					}
				})

				ui.Column(c).Padding(4, 16).Gap(10).Children(func() {
					ui.Text(c, "TASK OBJECTIVE").FontSize(10).FontWeight(600).TextColor(tc.TextMuted)
					ui.TextInput(c, &a.NewTaskTitle).Placeholder("Describe task objective...").Grow(1).AutoFocus()

					ui.Row(c).Gap(6).Children(func() {
						protoTab := func(label string, icon *ui.SVG, active bool, onPick func()) {
							btn := ui.Row(c).Height(28).Radius(6).Padding(0, 10).Gap(5).AlignItems(ui.Center)
							btn.Children(func() {
								ui.Icon(c, icon).Width(13).Height(13).TextColor(tc.IconMuted)
								ui.Text(c, label).FontSize(12).FontWeight(500)
							})
							if active {
								btn.Background(tc.Text.Alpha(0.1))
							} else if btn.Hovered() {
								btn.Background(tc.Text.Alpha(0.05))
							}
							if btn.Clicked() {
								onPick()
							}
						}
						protoTab("ACP Agents", icons.TablerSparkles, a.NewTaskAgentType == "acp", func() { a.NewTaskAgentType = "acp" })
						protoTab("Interactive Agent CLIs", icons.TablerTerminal, a.NewTaskAgentType == "cli", func() { a.NewTaskAgentType = "cli" })
					})

					ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
						ui.Box(c).Grow(1)
						if ui.Button(c, "Cancel").Clicked() {
							a.ShowNewTaskModal = false
						}
						if ui.PrimaryButton(c, "Start Session").Clicked() {
							title := a.NewTaskTitle
							if title == "" {
								title = "New Task"
							}
							if a.NewTaskAgentType == "cli" && a.CLIMgr != nil {
								a.CLIMgr.CreateSession(a.SelectedAgentID, a.SelectedAgentID, a.SelectedAgentID, []string{}, a.Workspace.RootDir)
								a.ActiveTaskKind = "cli"
							} else {
								sess := a.Agent.NewSession(title)
								sess.AgentID = a.SelectedAgentID
								sess.Model = a.SelectedAgentModel
								a.Agent.SaveSessionToDisk(sess)
								a.ActiveTaskKind = "agent"
							}
							a.ShowNewTaskModal = false
							a.Mode = ModeAgent
						}
					})
				})
			})
	})
}

func (a *AppUI) renderTaskActionsMenu(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	sess := a.Agent.ActiveSession()

	ui.Modal(c, &a.ShowTaskMenuModal, func() {
		ui.Column(c).Width(280).Radius(12).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).Padding(6).Gap(2).
			Shadow(0, 24, 64, -24, ui.RGBA(0, 0, 0, 0.45)).
			Children(func() {
				menuItem := func(icon *ui.SVG, label string, danger bool, onPick func()) {
					btn := SidebarActionButton(c, tc, icon, label, "", danger)
					if btn.Clicked() {
						onPick()
					}
				}
				divider := func() {
					ui.Box(c).Height(1).FillWidth().Background(tc.Border.Alpha(0.6)).Padding(0)
				}

				menuItem(icons.TablerPencil, "Rename task", false, func() {
					a.ShowTaskMenuModal = false
					a.ShowRenameModal = true
					if sess != nil {
						a.RenameSessionTitle = sess.Title
					}
				})

				archiveLabel := "Archive task"
				if sess != nil && sess.Archived {
					archiveLabel = "Unarchive task"
				}
				menuItem(icons.TablerArchive, archiveLabel, false, func() {
					a.ShowTaskMenuModal = false
					if sess != nil {
						a.Agent.ToggleArchive(sess.ID)
					}
				})

				menuItem(icons.TablerTerminal, "Toggle terminal", false, func() {
					a.ShowTaskMenuModal = false
					a.ShowBottomTerminal = !a.ShowBottomTerminal
				})

				divider()

				menuItem(icons.TablerExternalLink, "Open in Finder", false, func() {
					a.ShowTaskMenuModal = false
					if a.Workspace != nil && a.Workspace.RootDir != "" {
						mygo.Shell.ShowItemInFolder(a.Workspace.RootDir)
					}
				})

				menuItem(icons.TablerFolder, "Open sandbox folder", false, func() {
					a.ShowTaskMenuModal = false
					sandboxDir := filepath.Join(a.Workspace.RootDir, ".forge", "sandbox")
					if _, err := os.Stat(sandboxDir); err == nil {
						mygo.Shell.ShowItemInFolder(sandboxDir)
					} else {
						mygo.Shell.ShowItemInFolder(a.Workspace.RootDir)
					}
				})

				menuItem(icons.TablerCopy, "Copy sandbox path", false, func() {
					a.ShowTaskMenuModal = false
					sandboxDir := filepath.Join(a.Workspace.RootDir, ".forge", "sandbox")
					mygo.Clipboard.WriteText(sandboxDir)
					c.AddToast(ui.Toast{Title: "Copied!", Description: "Sandbox path copied to clipboard", Type: "success"})
				})

				menuItem(icons.TablerGitFork, "Fork conversation", false, func() {
					a.ShowTaskMenuModal = false
					if sess != nil {
						a.Agent.NewSession(fmt.Sprintf("%s (fork)", sess.Title))
					}
				})

				menuItem(icons.TablerCopy, "Copy path", false, func() {
					a.ShowTaskMenuModal = false
					if a.Workspace != nil {
						mygo.Clipboard.WriteText(a.Workspace.RootDir)
						c.AddToast(ui.Toast{Title: "Copied!", Description: "Workspace path copied to clipboard", Type: "success"})
					}
				})

				menuItem(icons.TablerCopy, "Copy log path", false, func() {
					a.ShowTaskMenuModal = false
					if sess != nil && a.Workspace != nil {
						logPath := filepath.Join(a.Workspace.RootDir, ".forge", "sessions", sess.ID+".json")
						mygo.Clipboard.WriteText(logPath)
						c.AddToast(ui.Toast{Title: "Copied!", Description: "Log path copied to clipboard", Type: "success"})
					}
				})

				menuItem(icons.TablerCopy, "Copy session ID", false, func() {
					a.ShowTaskMenuModal = false
					if sess != nil {
						mygo.Clipboard.WriteText(sess.ID)
						c.AddToast(ui.Toast{Title: "Copied!", Description: "Session ID copied to clipboard", Type: "success"})
					}
				})

				divider()

				menuItem(icons.TablerHelpCircle, "Report issue", false, func() {
					a.ShowTaskMenuModal = false
					mygo.Shell.OpenExternal("https://github.com/hasdev/forge-ade/issues")
				})

				menuItem(icons.TablerTrash, "Delete task", true, func() {
					a.ShowTaskMenuModal = false
					if sess != nil {
						a.Agent.DeleteSession(sess.ID)
					}
				})
			})
	})
}

func (a *AppUI) renderRenameModal(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	ui.Modal(c, &a.ShowRenameModal, func() {
		ui.Column(c).Width(380).Radius(12).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).Padding(16, 18).Gap(10).
			Shadow(0, 24, 64, -24, ui.RGBA(0, 0, 0, 0.45)).
			Children(func() {
				ui.Text(c, "Rename Task").FontSize(14).FontWeight(600)
				in := ui.TextInput(c, &a.RenameSessionTitle).Grow(1).AutoFocus()
				save := func() {
					if a.RenameSessionTitle == "" {
						return
					}
					if sess := a.Agent.ActiveSession(); sess != nil {
						sess.Title = a.RenameSessionTitle
						a.Agent.SaveSessionToDisk(sess)
					}
					a.ShowRenameModal = false
				}
				if in.Submitted() {
					save()
				}
				ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
					ui.Box(c).Grow(1)
					if ui.Button(c, "Cancel").Clicked() {
						a.ShowRenameModal = false
					}
					if ui.PrimaryButton(c, "Save").Clicked() {
						save()
					}
				})
			})
	})
}

func (a *AppUI) renderBranchModal(c *ui.Context) {
	tc := a.ThemeMgr.Colors()
	ui.Modal(c, &a.ShowBranchModal, func() {
		ui.Column(c).Width(420).Radius(12).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border).Padding(16, 18).Gap(10).
			Shadow(0, 24, 64, -24, ui.RGBA(0, 0, 0, 0.45)).
			Children(func() {
				ui.Row(c).AlignItems(ui.Center).Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, icons.TablerGitBranch).Width(16).Height(16).TextColor(tc.Accent)
						ui.Text(c, "Switch Git Branch").FontSize(14).FontWeight(600)
					})
					ui.Box(c).Grow(1)
					if IconButton(c, tc, icons.TablerX, 13).Clicked() {
						a.ShowBranchModal = false
					}
				})

				in := ui.TextInput(c, &a.BranchSearchQuery).Placeholder("Search or create branch...").Grow(1).AutoFocus()
				query := strings.TrimSpace(a.BranchSearchQuery)
				if in.Submitted() && query != "" {
					found := false
					if a.Git != nil {
						for _, b := range a.Git.Branches {
							if b == query {
								found = true
								break
							}
						}
					}
					if found {
						a.checkoutBranch(query)
					} else {
						a.createBranch(query)
					}
				}

				currentBranch := ""
				var branches []string
				if a.Git != nil {
					currentBranch = a.Git.Branch
					branches = a.Git.Branches
				}

				var filtered []string
				for _, b := range branches {
					if query == "" || strings.Contains(strings.ToLower(b), strings.ToLower(query)) {
						filtered = append(filtered, b)
					}
				}

				ui.Scroll(c).Height(minF(220, float32(len(filtered))*32+40)).Children(func() {
					ui.Column(c).Gap(2).Children(func() {
						for _, b := range filtered {
							branch := b
							isCurrent := (branch == currentBranch)
							row := ui.Row(c.Key("branch-row-"+branch)).Height(30).Radius(6).Padding(0, 10).Gap(8).AlignItems(ui.Center)
							row.Children(func() {
								ui.Icon(c, icons.TablerGitBranch).Width(13).Height(13).TextColor(tc.IconMuted)
								ui.Text(c, branch).FontSize(12).FontWeight(500).Grow(1)
								if isCurrent {
									ui.Icon(c, icons.TablerCheck).Width(14).Height(14).TextColor(tc.Success)
								}
							})
							if isCurrent {
								row.Background(tc.Accent.Alpha(0.12))
							} else if row.Hovered() {
								row.Background(tc.Text.Alpha(0.04))
							}
							if row.Clicked() && !isCurrent {
								a.checkoutBranch(branch)
							}
						}

						if query != "" {
							exists := false
							for _, b := range branches {
								if b == query {
									exists = true
									break
								}
							}
							if !exists {
								createRow := ui.Row(c.Key("branch-create-"+query)).Height(32).Radius(6).Padding(0, 10).Gap(8).AlignItems(ui.Center).
									Background(tc.Accent.Alpha(0.08))
								createRow.Children(func() {
									ui.Icon(c, icons.TablerPlus).Width(13).Height(13).TextColor(tc.Accent)
									ui.Text(c, fmt.Sprintf("Create branch '%s'", query)).FontSize(12).FontWeight(500).TextColor(tc.Accent)
								})
								if createRow.Hovered() {
									createRow.Background(tc.Accent.Alpha(0.16))
								}
								if createRow.Clicked() {
									a.createBranch(query)
								}
							}
						}
					})
				})
			})
	})
}
