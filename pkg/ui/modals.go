package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/agent"
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
		a.renderSettingsScreen(c)
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
	if a.ShowRenameFileModal {
		a.renderRenameFileModal(c)
	}
	if a.ShowBranchModal {
		a.renderBranchModal(c)
	}
	if a.ShowModelPalette {
		a.renderModelProviderPaletteModal(c)
	}
	if a.ShowSSHModal {
		a.renderSSHConnectModal(c)
	}
	if a.ShowSSHQuickMenu {
		a.renderSSHQuickMenuModal(c)
	}
	if a.ShowUsageModal {
		a.renderUsageQuotaModal(c)
	}
	if a.ShowRulesModal {
		a.renderUnifiedRulesModal(c)
	}
	if a.ShowNewBoardTaskModal {
		a.renderNewBoardTaskModal(c)
	}
}

func (a *AppUI) renderPaletteModal(c *ui.Context) {
	pm := a.Palette
	tc := a.ThemeMgr.Colors()

	if c.Shortcut(0, ui.KeyEscape) {
		pm.Close()
	}
	if c.Shortcut(0, ui.KeyDown) {
		pm.MoveSelection(1)
	}
	if c.Shortcut(0, ui.KeyUp) {
		pm.MoveSelection(-1)
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
					if mode == palette.PaletteFiles && len(matches) == 0 {
						ui.Text(c, "No matching files").FontSize(13).TextColor(tc.TextMuted).Padding(8, 0)
					}
					if mode == palette.PaletteCommands && len(commands) == 0 {
						ui.Text(c, "No matching commands").FontSize(13).TextColor(tc.TextMuted).Padding(8, 0)
					}
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

			// Footer: mode switches + hints
			ui.Row(c).Padding(8, 12).Gap(8).AlignItems(ui.Center).
				BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {
				modeBtn := func(label string, m int) {
					active := mode == m
					btn := ui.Row(c).Radius(4).Padding(2, 8).AlignItems(ui.Center)
					btn.Children(func() {
						txt := ui.Text(c, label).FontSize(12)
						if active {
							txt.TextColor(tc.Accent).FontWeight(500)
						} else {
							txt.TextColor(tc.TextMuted)
						}
					})
					if active {
						btn.Background(tc.Accent.Alpha(0.1))
					} else if btn.Hovered() {
						btn.Background(tc.Text.Alpha(0.04))
					}
					if btn.Clicked() {
						pm.Mode = m
						pm.SelectedIndex = 0
					}
				}
				modeBtn("Files", palette.PaletteFiles)
				modeBtn("Commands", palette.PaletteCommands)
				ui.Box(c).Grow(1)
				ui.Text(c, "↑↓ navigate · ↵ open").FontSize(12).TextColor(tc.TextMuted)
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
			summary := req.Title
			if summary == "" {
				summary = req.Description
			}
			ui.Text(c, summary).FontSize(13)
			if req.Description != "" && req.Title != "" && req.Description != req.Title {
				ui.Text(c, req.Description).FontSize(12).TextColor(tc.TextMuted)
			}
			if req.Arguments != "" {
				args := req.Arguments
				if len(args) > 600 {
					args = args[:600] + "…"
				}
				ui.Box(c).Padding(8).Radius(6).Background(tc.Canvas).Children(func() {
					ui.Text(c, args).FontSize(11).Selectable()
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
	projectName := filepath.Base(a.Workspace.RootDir)

	// Gather ACP agents
	type acpItem struct {
		ID          string
		Name        string
		Type        string
		Description string
		Command     string
		Model       string
		Available   bool
		IsInternal  bool
	}
	var acpAgents []acpItem
	// ForgeADE Internal always first
	acpAgents = append(acpAgents, acpItem{
		ID:          "agent-internal",
		Name:        "ForgeADE Internal",
		Type:        "internal",
		Description: "Internal autonomous engine — multi-turn reasoning and tool use",
		Available:   true,
		IsInternal:  true,
		Model:       a.SelectedAgentModel,
	})
	if a.ACPMgr != nil {
		for _, ag := range a.ACPMgr.Agents {
			if ag.ID == "agent-internal" {
				continue
			}
			acpAgents = append(acpAgents, acpItem{
				ID:          ag.ID,
				Name:        ag.Name,
				Type:        ag.Type,
				Description: ag.Description,
				Command:     ag.Command,
				Model:       ag.Model,
				Available:   ag.Available,
				IsInternal:  ag.Type == "internal",
			})
		}
	}

	// Gather Agent CLIs
	type cliItem struct {
		ID          string
		Name        string
		Executable  string
		Args        []string
		Description string
		Available   bool
	}
	var cliAgents []cliItem
	if a.CLIMgr != nil {
		for _, ca := range a.CLIMgr.GetAgentCLIs() {
			cliAgents = append(cliAgents, cliItem{
				ID:          ca.ID,
				Name:        ca.Name,
				Executable:  ca.Executable,
				Args:        ca.Args,
				Description: ca.Description,
				Available:   ca.Available,
			})
		}
	} else {
		for _, ca := range agent.DefaultAgentCLIs {
			cliAgents = append(cliAgents, cliItem{
				ID:          ca.ID,
				Name:        ca.Name,
				Executable:  ca.Executable,
				Args:        ca.Args,
				Description: ca.Description,
				Available:   false,
			})
		}
	}

	// Filtering
	q := strings.TrimSpace(strings.ToLower(a.NewTaskSearchQuery))
	var filteredAvailACP []acpItem
	var filteredUninstACP []acpItem
	for _, ag := range acpAgents {
		match := q == "" || strings.Contains(strings.ToLower(ag.Name), q) ||
			strings.Contains(strings.ToLower(ag.Description), q) ||
			strings.Contains(strings.ToLower(ag.Command), q) ||
			strings.Contains(strings.ToLower(ag.ID), q)
		if !match {
			continue
		}
		if ag.Available {
			filteredAvailACP = append(filteredAvailACP, ag)
		} else {
			filteredUninstACP = append(filteredUninstACP, ag)
		}
	}

	var filteredAvailCLI []cliItem
	var filteredUninstCLI []cliItem
	for _, cl := range cliAgents {
		match := q == "" || strings.Contains(strings.ToLower(cl.Name), q) ||
			strings.Contains(strings.ToLower(cl.Description), q) ||
			strings.Contains(strings.ToLower(cl.Executable), q) ||
			strings.Contains(strings.ToLower(cl.ID), q)
		if !match {
			continue
		}
		if cl.Available {
			filteredAvailCLI = append(filteredAvailCLI, cl)
		} else {
			filteredUninstCLI = append(filteredUninstCLI, cl)
		}
	}

	activeTab := a.NewTaskFilterTab
	if activeTab == "" {
		activeTab = "all"
	}

	startSession := func() {
		title := strings.TrimSpace(a.NewTaskTitle)

		if a.NewTaskSelectedKind == "cli" && a.CLIMgr != nil {
			if title == "" {
				if a.NewTaskSelectedName != "" {
					title = fmt.Sprintf("%s Task", a.NewTaskSelectedName)
				} else {
					title = "New Task"
				}
			}
			a.CLIMgr.CreateSessionWithTitle(
				a.NewTaskSelectedAgentID,
				a.NewTaskSelectedName,
				a.NewTaskSelectedExec,
				a.NewTaskSelectedArgs,
				a.Workspace.RootDir,
				title,
			)
			a.ActiveTaskKind = "cli"
			a.ShowNewTaskModal = false
			a.Mode = ModeAgent
		} else if a.NewTaskSelectedAgentID != "" && a.NewTaskSelectedAgentID != "agent-internal" {
			a.SelectedAgentID = a.NewTaskSelectedAgentID
			a.SelectedHarnessID = a.NewTaskSelectedAgentID
			if a.Agent != nil {
				a.Agent.ActiveAgentID = a.NewTaskSelectedAgentID
				a.Agent.ActiveAgentType = "acp"
			}
			if title != "" {
				if a.Agent != nil {
					a.Agent.CreateSessionWithInitialPrompt(title, a.NewTaskSelectedAgentID)
				}
			} else {
				a.NewAgentTask(a.Workspace.RootDir)
				return
			}
			a.ActiveTaskKind = "agent"
			a.ShowNewTaskModal = false
			a.Mode = ModeAgent
		} else {
			if title == "" {
				a.NewAgentTask(a.Workspace.RootDir)
				return
			}
			if a.Agent != nil {
				agentID := a.SelectedAgentID
				if agentID == "" {
					agentID = a.SelectedHarnessID
				}
				if agentID == "" {
					agentID = "agent-agy"
				}
				a.Agent.CreateSessionWithInitialPrompt(title, agentID)
			}
			a.ActiveTaskKind = "agent"
			a.ShowNewTaskModal = false
			a.Mode = ModeAgent
		}

		c.AddToast(ui.Toast{
			Title:       "Task Session Started",
			Description: fmt.Sprintf("Started %s in %s", title, projectName),
			Type:        "info",
		})
		if a.Window != nil {
			a.Window.Invalidate()
		}
	}

	modalWidth := float32(580)
	if winW, _ := c.Size(); winW > 0 && winW*0.92 < modalWidth {
		modalWidth = winW * 0.92
	}

	ui.Modal(c, &a.ShowNewTaskModal, func() {
		ui.Column(c).Width(modalWidth).Height(500).Radius(14).
			Background(tc.Sidebar).BorderWidth(1).BorderColor(tc.Border).
			Shadow(0, 24, 72, -20, ui.RGBA(0, 0, 0, 0.55)).
			Children(func() {

				// Step 1: Agent Selection
				if a.NewTaskStep != "workspace" {
					// 1. Header
					ui.Row(c).Height(46).Padding(0, 16).AlignItems(ui.Center).
						BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.6)).Children(func() {
						ui.Icon(c, icons.TablerTerminal).Width(15).Height(15).TextColor(tc.IconMuted)
						ui.Text(c, "New Task — "+projectName).FontSize(13).FontWeight(600).Grow(1).SingleLine()

						// Step indicator dots
						ui.Row(c).Gap(5).AlignItems(ui.Center).Padding(0, 8).Children(func() {
							ui.Box(c).Size(6, 6).Radius(3).Background(tc.Accent)
							ui.Box(c).Size(6, 6).Radius(3).Background(tc.Border)
						})

						if IconButton(c, tc, icons.TablerX, 13).Clicked() {
							a.ShowNewTaskModal = false
						}
					})

					// 2. Search & Segmented Filter
					ui.Column(c).Background(tc.Surface.Alpha(0.25)).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.5)).Children(func() {
						ui.Row(c).Height(36).Padding(0, 12).Gap(8).AlignItems(ui.Center).Children(func() {
							ui.Icon(c, icons.TablerSearch).Width(14).Height(14).TextColor(tc.IconMuted)
							in := ui.TextInput(c, &a.NewTaskSearchQuery).Placeholder("Filter agents by name, command, or protocol...").Grow(1).AutoFocus()
							if in.Submitted() {
								if len(filteredAvailACP) > 0 {
									first := filteredAvailACP[0]
									if first.ID == "agent-internal" || first.Type == "internal" {
										a.NewAgentTask(a.Workspace.RootDir)
										return
									}
									a.NewTaskSelectedKind = "acp"
									a.NewTaskSelectedAgentID = first.ID
									a.NewTaskSelectedName = first.Name
									a.NewTaskStep = "workspace"
								} else if len(filteredAvailCLI) > 0 {
									first := filteredAvailCLI[0]
									a.NewTaskSelectedKind = "cli"
									a.NewTaskSelectedAgentID = first.ID
									a.NewTaskSelectedName = first.Name
									a.NewTaskSelectedExec = first.Executable
									a.NewTaskSelectedArgs = first.Args
									a.NewTaskStep = "workspace"
								}
							}
							if a.NewTaskSearchQuery != "" {
								clearBtn := ui.Row(c).Size(18, 18).Radius(9).AlignItems(ui.Center).Justify(ui.Center)
								clearBtn.Children(func() {
									ui.Icon(c, icons.TablerX).Width(11).Height(11).TextColor(tc.IconMuted)
								})
								if clearBtn.Clicked() {
									a.NewTaskSearchQuery = ""
								}
							}
						})

						// Filter Tabs: All | Agent ACP | Agent CLI (Notice: NO "Interactive"!)
						ui.Row(c).Padding(0, 10, 8, 10).Gap(6).AlignItems(ui.Center).Children(func() {
							tabPill := func(key, label string, count int, onPick func()) {
								isSel := (activeTab == key)
								pill := ui.Row(c.Key("nt-tab-"+key)).Height(24).Radius(6).Padding(0, 8).Gap(5).AlignItems(ui.Center)
								pill.Children(func() {
									txt := ui.Text(c, label).FontSize(11).FontWeight(500)
									if isSel {
										txt.TextColor(tc.Text)
									} else {
										txt.TextColor(tc.TextMuted)
									}
									cntBadge := ui.Box(c).Radius(4).Padding(1, 5).AlignItems(ui.Center).Justify(ui.Center)
									if isSel {
										cntBadge.Background(tc.Accent.Alpha(0.2))
										ui.Text(c, fmt.Sprintf("%d", count)).FontSize(10).FontWeight(700).TextColor(tc.Accent)
									} else {
										cntBadge.Background(tc.Text.Alpha(0.06))
										ui.Text(c, fmt.Sprintf("%d", count)).FontSize(10).TextColor(tc.TextMuted)
									}
								})
								if isSel {
									pill.Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
								} else if pill.Hovered() {
									pill.Background(tc.Text.Alpha(0.04))
								}
								if pill.Clicked() {
									onPick()
								}
							}

							tabPill("all", "All", len(filteredAvailACP)+len(filteredAvailCLI), func() { a.NewTaskFilterTab = "all" })
							tabPill("acp", "Agent ACP", len(filteredAvailACP), func() { a.NewTaskFilterTab = "acp" })
							tabPill("cli", "Agent CLI", len(filteredAvailCLI), func() { a.NewTaskFilterTab = "cli" })
						})
					})

					// 3. Scrollable Agent Groups
					ui.Scroll(c).Grow(1).Padding(8, 12).Children(func() {
						ui.Column(c).Gap(10).Children(func() {

							// Section 1: AGENT ACP
							if activeTab == "all" || activeTab == "acp" {
								ui.Column(c).Gap(4).Children(func() {
									ui.Row(c).Padding(2, 4).Gap(6).AlignItems(ui.Center).Children(func() {
										ui.Text(c, "AGENT ACP").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.SidebarMutedForeground)
										ui.Box(c).Radius(3).Padding(1, 5).Background(tc.Accent.Alpha(0.12)).Children(func() {
											ui.Text(c, "Agent Client Protocol & Native").FontSize(9).FontWeight(600).TextColor(tc.Accent)
										})
										ui.Box(c).Grow(1)
										ui.Text(c, fmt.Sprintf("%d available", len(filteredAvailACP))).FontSize(10).TextColor(tc.TextMuted)
									})

									for _, ag := range filteredAvailACP {
										thisAg := ag
										card := ui.Row(c.Key("nt-acp-"+thisAg.ID)).Height(48).Radius(8).Padding(0, 10).Gap(10).AlignItems(ui.Center).
											Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5))
										card.Children(func() {
											// Icon
											ui.Box(c).Size(30, 30).Radius(6).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6)).
												AlignItems(ui.Center).Justify(ui.Center).Children(func() {
												if thisAg.IsInternal {
													ui.Icon(c, icons.TablerSparkles).Width(16).Height(16).TextColor(tc.Accent)
												} else {
													icons.RenderProviderIcon(c, thisAg.ID, 15)
												}
											})

											// Name & Description
											ui.Column(c).Grow(1).Gap(1).Children(func() {
												ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
													ui.Text(c, thisAg.Name).FontSize(13).FontWeight(600)
													if thisAg.IsInternal {
														ui.Box(c).Radius(3).Padding(1, 4).Background(tc.Text.Alpha(0.08)).Children(func() {
															ui.Text(c, "INTERNAL").FontSize(9).FontWeight(700).TextColor(tc.TextMuted)
														})
													} else {
														ui.Box(c).Radius(3).Padding(1, 4).Background(tc.Accent.Alpha(0.15)).Children(func() {
															ui.Text(c, "ACP").FontSize(9).FontWeight(700).TextColor(tc.Accent)
														})
													}
												})
												ui.Text(c, thisAg.Description).FontSize(11).TextColor(tc.TextMuted).SingleLine()
											})

											// Action / Chevron
											ui.Icon(c, icons.TablerChevronRight).Width(13).Height(13).TextColor(tc.IconMuted)
										})
										if card.Hovered() {
											card.Background(tc.ToolbarControlHover).BorderColor(tc.Accent.Alpha(0.4))
										}
										if card.Clicked() {
											if thisAg.ID == "agent-internal" || thisAg.Type == "internal" {
												a.NewAgentTask(a.Workspace.RootDir)
												return
											}
											a.NewTaskSelectedKind = "acp"
											a.NewTaskSelectedAgentID = thisAg.ID
											a.NewTaskSelectedName = thisAg.Name
											a.NewTaskStep = "workspace"
										}
									}

									if len(filteredAvailACP) == 0 {
										ui.Row(c).Padding(8, 12).Children(func() {
											ui.Text(c, "No ACP agents match filter.").FontSize(12).TextColor(tc.TextMuted)
										})
									}

									// Collapsible uninstalled ACP agents
									if len(filteredUninstACP) > 0 {
										tog := ui.Row(c).Padding(4, 6).Gap(5).AlignItems(ui.Center)
										tog.Children(func() {
											arr := icons.TablerChevronDown
											if a.NewTaskUninstalledOpen {
												arr = icons.TablerChevronUp
											}
											ui.Icon(c, arr).Width(12).Height(12).TextColor(tc.IconMuted)
											ui.Text(c, fmt.Sprintf("Other ACP agents (%d not installed)", len(filteredUninstACP))).FontSize(11).TextColor(tc.TextMuted)
										})
										if tog.Hovered() {
											tog.Background(tc.Text.Alpha(0.04))
										}
										if tog.Clicked() {
											a.NewTaskUninstalledOpen = !a.NewTaskUninstalledOpen
										}

										if a.NewTaskUninstalledOpen {
											for _, ag := range filteredUninstACP {
												thisAg := ag
												row := ui.Row(c.Key("nt-uninst-acp-"+thisAg.ID)).Height(38).Radius(6).Padding(0, 10).Gap(8).AlignItems(ui.Center).
													Background(tc.Surface.Alpha(0.3)).BorderWidth(1).BorderColor(tc.Border.Alpha(0.3))
												row.Children(func() {
													ui.Icon(c, icons.TablerBot).Width(14).Height(14).TextColor(tc.IconMuted)
													ui.Column(c).Grow(1).Gap(1).Children(func() {
														ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
															ui.Text(c, thisAg.Name).FontSize(12).FontWeight(500).TextColor(tc.TextMuted)
															ui.Box(c).Radius(3).Padding(0, 4).Background(tc.Border).Children(func() {
																ui.Text(c, "Not Installed").FontSize(9).TextColor(tc.TextMuted)
															})
														})
													})
												})
											}
										}
									}
								})
							}

							// Divider
							if activeTab == "all" {
								ui.Box(c).Height(1).Background(tc.Border.Alpha(0.5)).Margin(2, 0)
							}

							// Section 2: AGENT CLI (Notice: NO "Interactive"!)
							if activeTab == "all" || activeTab == "cli" {
								ui.Column(c).Gap(4).Children(func() {
									ui.Row(c).Padding(2, 4).Gap(6).AlignItems(ui.Center).Children(func() {
										ui.Text(c, "AGENT CLI").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.SidebarMutedForeground)
										ui.Box(c).Radius(3).Padding(1, 5).Background(tc.Text.Alpha(0.08)).Children(func() {
											ui.Text(c, "Terminal PTY Session").FontSize(9).FontWeight(600).TextColor(tc.TextMuted)
										})
										ui.Box(c).Grow(1)
										ui.Text(c, fmt.Sprintf("%d available", len(filteredAvailCLI))).FontSize(10).TextColor(tc.TextMuted)
									})

									for _, cl := range filteredAvailCLI {
										thisCl := cl
										card := ui.Row(c.Key("nt-cli-"+thisCl.ID)).Height(48).Radius(8).Padding(0, 10).Gap(10).AlignItems(ui.Center).
											Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border.Alpha(0.5))
										card.Children(func() {
											// Icon
											ui.Box(c).Size(30, 30).Radius(6).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6)).
												AlignItems(ui.Center).Justify(ui.Center).Children(func() {
												ui.Icon(c, icons.TablerTerminal).Width(15).Height(15).TextColor(tc.Accent)
											})

											// Name & Description
											ui.Column(c).Grow(1).Gap(1).Children(func() {
												ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
													ui.Text(c, thisCl.Name).FontSize(13).FontWeight(600)
													ui.Box(c).Radius(3).Padding(1, 4).Background(tc.Text.Alpha(0.08)).Children(func() {
														ui.Text(c, "CLI").FontSize(9).FontWeight(700).TextColor(tc.TextMuted)
													})
													if thisCl.Executable != "" {
														ui.Box(c).Radius(3).Padding(1, 4).Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6)).Children(func() {
															ui.Text(c, thisCl.Executable).FontSize(9).TextColor(tc.TextMuted)
														})
													}
												})
												ui.Text(c, thisCl.Description).FontSize(11).TextColor(tc.TextMuted).SingleLine()
											})

											// Chevron
											ui.Icon(c, icons.TablerChevronRight).Width(13).Height(13).TextColor(tc.IconMuted)
										})
										if card.Hovered() {
											card.Background(tc.ToolbarControlHover).BorderColor(tc.Accent.Alpha(0.4))
										}
										if card.Clicked() {
											a.NewTaskSelectedKind = "cli"
											a.NewTaskSelectedAgentID = thisCl.ID
											a.NewTaskSelectedName = thisCl.Name
											a.NewTaskSelectedExec = thisCl.Executable
											a.NewTaskSelectedArgs = thisCl.Args
											a.NewTaskStep = "workspace"
										}
									}

									if len(filteredAvailCLI) == 0 {
										ui.Row(c).Padding(8, 12).Children(func() {
											ui.Text(c, "No Agent CLIs match filter.").FontSize(12).TextColor(tc.TextMuted)
										})
									}

									// Collapsible uninstalled CLI agents
									if len(filteredUninstCLI) > 0 {
										tog := ui.Row(c).Padding(4, 6).Gap(5).AlignItems(ui.Center)
										tog.Children(func() {
											arr := icons.TablerChevronDown
											if a.NewTaskUninstalledOpen {
												arr = icons.TablerChevronUp
											}
											ui.Icon(c, arr).Width(12).Height(12).TextColor(tc.IconMuted)
											ui.Text(c, fmt.Sprintf("Other Agent CLIs (%d not installed)", len(filteredUninstCLI))).FontSize(11).TextColor(tc.TextMuted)
										})
										if tog.Hovered() {
											tog.Background(tc.Text.Alpha(0.04))
										}
										if tog.Clicked() {
											a.NewTaskUninstalledOpen = !a.NewTaskUninstalledOpen
										}

										if a.NewTaskUninstalledOpen {
											for _, cl := range filteredUninstCLI {
												thisCl := cl
												row := ui.Row(c.Key("nt-uninst-cli-"+thisCl.ID)).Height(38).Radius(6).Padding(0, 10).Gap(8).AlignItems(ui.Center).
													Background(tc.Surface.Alpha(0.3)).BorderWidth(1).BorderColor(tc.Border.Alpha(0.3))
												row.Children(func() {
													ui.Icon(c, icons.TablerTerminal).Width(14).Height(14).TextColor(tc.IconMuted)
													ui.Column(c).Grow(1).Gap(1).Children(func() {
														ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
															ui.Text(c, thisCl.Name).FontSize(12).FontWeight(500).TextColor(tc.TextMuted)
															ui.Box(c).Radius(3).Padding(0, 4).Background(tc.Border).Children(func() {
																ui.Text(c, "Executable not found: "+thisCl.Executable).FontSize(9).TextColor(tc.TextMuted)
															})
														})
													})
												})
											}
										}
									}
								})
							}
						})
					})

				} else {
					// Step 2: Workspace Selection & Task Title / Launch
					// 1. Header with Back button
					ui.Row(c).Height(46).Padding(0, 16).AlignItems(ui.Center).Gap(8).
						BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.6)).Children(func() {
						backBtn := ui.Row(c).Size(26, 26).Radius(6).AlignItems(ui.Center).Justify(ui.Center)
						backBtn.Children(func() {
							ui.Icon(c, icons.TablerArrowLeft).Width(14).Height(14).TextColor(tc.IconMuted)
						})
						if backBtn.Hovered() {
							backBtn.Background(tc.Text.Alpha(0.06))
						}
						if backBtn.Clicked() {
							a.NewTaskStep = "agent"
						}

						ui.Text(c, "New Task — Select Workspace").FontSize(13).FontWeight(600).Grow(1)

						// Step indicator dots
						ui.Row(c).Gap(5).AlignItems(ui.Center).Padding(0, 8).Children(func() {
							ui.Box(c).Size(6, 6).Radius(3).Background(tc.Border)
							ui.Box(c).Size(6, 6).Radius(3).Background(tc.Accent)
						})

						if IconButton(c, tc, icons.TablerX, 13).Clicked() {
							a.ShowNewTaskModal = false
						}
					})

					// 2. Body
					ui.Column(c).Grow(1).Padding(16, 20).Gap(16).Children(func() {

						// Selected Agent Banner
						ui.Row(c).Height(46).Radius(8).Padding(0, 12).Gap(10).AlignItems(ui.Center).
							Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border).Children(func() {
							ui.Box(c).Size(28, 28).Radius(6).Background(tc.SurfaceOverlay).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
								if a.NewTaskSelectedKind == "cli" {
									ui.Icon(c, icons.TablerTerminal).Width(15).Height(15).TextColor(tc.Accent)
								} else {
									ui.Icon(c, icons.TablerSparkles).Width(15).Height(15).TextColor(tc.Accent)
								}
							})
							ui.Column(c).Grow(1).Gap(1).Children(func() {
								ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
									ui.Text(c, a.NewTaskSelectedName).FontSize(13).FontWeight(600)
									ui.Box(c).Radius(3).Padding(1, 4).Background(tc.Accent.Alpha(0.15)).Children(func() {
										ui.Text(c, strings.ToUpper(a.NewTaskSelectedKind)).FontSize(9).FontWeight(700).TextColor(tc.Accent)
									})
								})
								ui.Text(c, "Agent ready to launch").FontSize(11).TextColor(tc.TextMuted)
							})

							changeBtn := ui.Row(c).Height(24).Radius(6).Padding(0, 8).AlignItems(ui.Center).Background(tc.SurfaceOverlay)
							changeBtn.Children(func() {
								ui.Text(c, "Change Agent").FontSize(11).FontWeight(500).TextColor(tc.TextMuted)
							})
							if changeBtn.Hovered() {
								changeBtn.Background(tc.ToolbarControlHover)
							}
							if changeBtn.Clicked() {
								a.NewTaskStep = "agent"
							}
						})

						// Task Objective Input
						ui.Column(c).Gap(6).Children(func() {
							ui.Text(c, "TASK OBJECTIVE / INITIAL PROMPT").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.SidebarMutedForeground)
							in := ui.TextInput(c, &a.NewTaskTitle).Placeholder("Describe task objective or initial instruction...").Grow(1).AutoFocus()
							if in.Submitted() {
								startSession()
							}
						})

						// Workspace Directory Card
						ui.Column(c).Gap(6).Children(func() {
							ui.Text(c, "TARGET WORKSPACE").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.SidebarMutedForeground)
							wsCard := ui.Row(c).Radius(8).Padding(10, 12).Gap(10).AlignItems(ui.Center).
								Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
							wsCard.Children(func() {
								ui.Icon(c, icons.TablerFolderOpen).Width(18).Height(18).TextColor(tc.Accent)
								ui.Column(c).Grow(1).Gap(2).Children(func() {
									ui.Text(c, projectName).FontSize(13).FontWeight(600)
									ui.Text(c, a.Workspace.RootDir).FontSize(11).TextColor(tc.TextMuted).SingleLine()
								})

								changeWsBtn := ui.Row(c).Height(26).Radius(6).Padding(0, 8).Gap(4).AlignItems(ui.Center).
									Background(tc.SurfaceOverlay).BorderWidth(1).BorderColor(tc.Border)
								changeWsBtn.Children(func() {
									ui.Icon(c, icons.TablerFolder).Width(12).Height(12).TextColor(tc.IconMuted)
									ui.Text(c, "Change Folder").FontSize(11).FontWeight(500)
								})
								if changeWsBtn.Hovered() {
									changeWsBtn.Background(tc.ToolbarControlHover)
								}
								if changeWsBtn.Clicked() {
									a.openWorkspaceFolder()
								}
							})
						})

						ui.Box(c).Grow(1)

						// Footer Actions
						ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
							backBtn := ui.Button(c, "Back")
							if backBtn.Clicked() {
								a.NewTaskStep = "agent"
							}

							ui.Box(c).Grow(1)

							cancelBtn := ui.Button(c, "Cancel")
							if cancelBtn.Clicked() {
								a.ShowNewTaskModal = false
							}

							startBtn := ui.PrimaryButton(c, "Start Session")
							if startBtn.Clicked() {
								startSession()
							}
						})
					})
				}
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
					var btn ui.Element
					if danger {
						btn = SidebarDangerActionButton(c, tc, icon, label)
					} else {
						btn = SidebarActionButton(c, tc, icon, label, "", false)
					}
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
						forked := a.Agent.ForkSession(sess)
						if forked != nil {
							a.Agent.SaveSessionToDisk(forked)
						}
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

// renderNewBoardTaskModal renders the task creation dialog for the Workflow Board.
func (a *AppUI) renderNewBoardTaskModal(c *ui.Context) {
	if !a.ShowNewBoardTaskModal {
		return
	}
	tc := a.ThemeMgr.Colors()

	ui.Modal(c, &a.ShowNewBoardTaskModal, func() {
		modal := ui.Column(c).Width(460).Radius(16).Background(tc.Surface).
			BorderWidth(1).BorderColor(tc.Border).Shadow(0, 24, 72, -20, ui.RGBA(0, 0, 0, 0.55)).Clip()

		modal.Children(func() {
			// Header
			ui.Row(c).Height(48).Padding(0, 18).Gap(10).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.6)).
				Background(tc.SurfaceRaised.Alpha(0.5)).Children(func() {
				ui.Box(c).Size(28, 28).Radius(6).AlignItems(ui.Center).Justify(ui.Center).
					Background(tc.Accent.Alpha(0.12)).Children(func() {
					ui.Icon(c, icons.TablerLayoutKanban).Width(14).Height(14).TextColor(tc.Accent)
				})
				ui.Text(c, "New Workflow Task").FontSize(13).FontWeight(600).TextColor(tc.Text)
				ui.Box(c).Grow(1)
				closeBtn := ui.Row(c).Size(26, 26).Radius(6).AlignItems(ui.Center).Justify(ui.Center)
				closeBtn.Children(func() {
					ui.Icon(c, icons.TablerX).Width(13).Height(13).TextColor(tc.IconMuted)
				})
				if closeBtn.Hovered() {
					closeBtn.Background(tc.Text.Alpha(0.06))
				}
				if closeBtn.Clicked() {
					a.ShowNewBoardTaskModal = false
				}
			})

			// Form body
			ui.Column(c).Padding(16, 18).Gap(12).Children(func() {
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "TITLE").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
					ui.TextInput(c, &a.NewBoardTaskTitle).Placeholder("e.g. Implement user authentication").Grow(1)
				})

				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "DESCRIPTION").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
					ui.TextInput(c, &a.NewBoardTaskDesc).Placeholder("Details, context, or acceptance criteria...").Grow(1)
				})

				// Priority selector
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "PRIORITY").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
					ui.Row(c).Gap(6).Children(func() {
						priorities := []struct {
							id    string
							label string
							color ui.Color
						}{
							{"low", "Low", tc.TextMuted},
							{"medium", "Medium", tc.Accent},
							{"high", "High", tc.Warning},
							{"urgent", "Urgent", tc.Error},
						}
						if a.NewBoardTaskPriority == "" {
							a.NewBoardTaskPriority = "medium"
						}
						for _, p := range priorities {
							thisP := p
							isSel := a.NewBoardTaskPriority == thisP.id
							pill := ui.Row(c.Key("prio-"+thisP.id)).Height(28).Radius(6).Padding(0, 10).AlignItems(ui.Center)
							pill.Children(func() {
								ui.Box(c).Size(6, 6).Radius(3).Background(thisP.color)
								ui.Text(c, thisP.label).FontSize(11).FontWeight(500)
							})
							if isSel {
								pill.Background(thisP.color.Alpha(0.18)).BorderWidth(1).BorderColor(thisP.color.Alpha(0.5))
							} else {
								pill.Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
							}
							if pill.Clicked() {
								a.NewBoardTaskPriority = thisP.id
							}
						}
					})
				})

				// Status column selector
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "COLUMN").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
					ui.Row(c).Gap(6).Children(func() {
						statuses := []struct {
							id    string
							label string
						}{
							{"todo", "To Do"},
							{"in_progress", "In Progress"},
							{"in_review", "In Review"},
							{"done", "Done"},
						}
						if a.NewBoardTaskStatus == "" {
							a.NewBoardTaskStatus = "todo"
						}
						for _, s := range statuses {
							thisS := s
							isSel := a.NewBoardTaskStatus == thisS.id
							pill := ui.Row(c.Key("col-"+thisS.id)).Height(28).Radius(6).Padding(0, 10).AlignItems(ui.Center)
							pill.Children(func() {
								ui.Text(c, thisS.label).FontSize(11).FontWeight(500)
							})
							if isSel {
								pill.Background(tc.Accent.Alpha(0.18)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.5))
							} else {
								pill.Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
							}
							if pill.Clicked() {
								a.NewBoardTaskStatus = thisS.id
							}
						}
					})
				})
			})

			// Footer actions
			ui.Row(c).Padding(12, 18).AlignItems(ui.Center).
				BorderWidth(1, 0, 0, 0).BorderColor(tc.Border.Alpha(0.6)).
				Background(tc.SurfaceRaised.Alpha(0.3)).Children(func() {
				ui.Box(c).Grow(1)
				cancelBtn := ui.Row(c).Height(30).Radius(6).Padding(0, 12).AlignItems(ui.Center).Justify(ui.Center).
					Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border)
				cancelBtn.Children(func() {
					ui.Text(c, "Cancel").FontSize(11).TextColor(tc.TextMuted)
				})
				if cancelBtn.Clicked() {
					a.ShowNewBoardTaskModal = false
				}

				createBtn := ui.Row(c).Height(30).Radius(6).Padding(0, 14).Gap(5).AlignItems(ui.Center).Justify(ui.Center).
					Background(tc.Accent).BorderWidth(1).BorderColor(tc.Accent)
				createBtn.Children(func() {
					ui.Icon(c, icons.TablerPlus).Width(12).Height(12).TextColor(ui.RGBA(255, 255, 255, 1.0))
					ui.Text(c, "Create Task").FontSize(11).FontWeight(600).TextColor(ui.RGBA(255, 255, 255, 1.0))
				})
				if createBtn.Hovered() {
					createBtn.Background(tc.Accent.Alpha(0.85))
				}
				if createBtn.Clicked() {
					if strings.TrimSpace(a.NewBoardTaskTitle) != "" {
						if a.Board == nil {
							a.RefreshBoard()
						}
						if a.Board != nil {
							_, err := a.Board.AddTask(a.NewBoardTaskTitle, a.NewBoardTaskDesc, a.NewBoardTaskStatus, a.NewBoardTaskPriority)
							if err != nil {
								c.AddToast(ui.Toast{Title: "Error", Description: err.Error(), Type: "error"})
							} else {
								c.AddToast(ui.Toast{Title: "Task Created", Description: a.NewBoardTaskTitle, Type: "success"})
								a.NewBoardTaskTitle = ""
								a.NewBoardTaskDesc = ""
								a.ShowNewBoardTaskModal = false
							}
						}
					}
				}
			})
		})
	})
}
