package ui

import (
	"fmt"
	"os/exec"

	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/icons"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/settings"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/theme"
)

// renderSettingsScreen renders ForgeADE's full Settings Screen experience.
func (a *AppUI) renderSettingsScreen(c *ui.Context) {
	sm := a.Settings
	if sm == nil || !sm.IsOpen {
		return
	}
	tc := a.ThemeMgr.Colors()

	if c.Shortcut(0, ui.KeyEscape) {
		sm.IsOpen = false
		a.Mode = ModeAgent
		return
	}

	// ForgeADE's settings is a full-screen surface with its own nav.
	{
		ui.Column(c).Fill().Background(tc.SurfaceOverlay).Children(func() {

			// 1. Header Bar: Back Button, Breadcrumb, Search Bar, Close Button
			ui.Row(c).Height(48).Padding(0, 16).Gap(12).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {

				backBtn := ui.Row(c).Height(28).Radius(6).Padding(0, 8).Gap(6).AlignItems(ui.Center).
					Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
				backBtn.Children(func() {
					ui.Icon(c, icons.TablerArrowLeft).Width(13).Height(13).TextColor(tc.IconMuted)
					ui.Text(c, "Back to workspace").FontSize(12).FontWeight(500)
				})
				if backBtn.Clicked() {
					sm.IsOpen = false
					a.Mode = ModeAgent
				}

				// Breadcrumb
				sectionName := settingsSectionTitle(sm.ActiveTab)
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "Settings").FontSize(13).TextColor(tc.TextMuted)
					ui.Icon(c, icons.TablerChevronRight).Width(11).Height(11).TextColor(tc.IconMuted)
					ui.Text(c, sectionName).FontSize(13).FontWeight(600)
				})

				ui.Box(c).Grow(1)

				// Search input
				ui.Row(c).Width(220).Height(28).Radius(6).Padding(0, 8).Gap(6).AlignItems(ui.Center).
					Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border).Children(func() {
					ui.Icon(c, icons.TablerSearch).Width(12).Height(12).TextColor(tc.IconMuted)
					ui.TextInput(c, &sm.SearchQuery).Placeholder("Search settings...").Grow(1)
				})

				closeBtn := ui.Row(c).Size(24, 24).Radius(6).AlignItems(ui.Center).Justify(ui.Center)
				closeBtn.Children(func() {
					ui.Icon(c, icons.TablerX).Width(13).Height(13).TextColor(tc.IconMuted)
				})
				if closeBtn.Clicked() {
					sm.IsOpen = false
				}
			})

			// 2. Main Body: Split Sidebar & Content Area
			ui.Row(c).Grow(1).FillWidth().Children(func() {
				// Left Navigation Sidebar (210px)
				ui.Column(c).Width(210).FillHeight().Background(tc.Sidebar).
					BorderWidth(0, 1, 0, 0).BorderColor(tc.SidebarBorder).
					Padding(12, 8).Gap(14).Children(func() {

					// Group 1: Models & Agents
					ui.Column(c).Gap(2).Children(func() {
						ui.Row(c).Padding(2, 8, 4, 8).Children(func() {
							ui.Text(c, "MODELS & AGENTS").FontSize(10).FontWeight(700).LetterSpacing(0.5).
								TextColor(tc.SidebarMutedForeground)
						})

						a.renderSettingsNavItem(c, tc, sm, settings.SettingsTabModels, "Providers & Models", icons.TablerPackage, "")
						a.renderSettingsNavItem(c, tc, sm, settings.SettingsTabAgents, "ACP & Agent CLIs", icons.TablerBot, "ACP")
						a.renderSettingsNavItem(c, tc, sm, settings.SettingsTabMCP, "MCP Servers", icons.TablerPlug, "")
						a.renderSettingsNavItem(c, tc, sm, settings.SettingsTabSkills, "Skills", icons.TablerWand, "")
					})

					// Group 2: Workspace
					ui.Column(c).Gap(2).Children(func() {
						ui.Row(c).Padding(2, 8, 4, 8).Children(func() {
							ui.Text(c, "WORKSPACE").FontSize(10).FontWeight(700).LetterSpacing(0.5).
								TextColor(tc.SidebarMutedForeground)
						})

						a.renderSettingsNavItem(c, tc, sm, settings.SettingsTabGeneral, "General", icons.TablerSettings, "")
						a.renderSettingsNavItem(c, tc, sm, settings.SettingsTabAppearance, "Appearance", icons.TablerPalette, "")
						a.renderSettingsNavItem(c, tc, sm, settings.SettingsTabShortcuts, "Shortcuts & Keys", icons.TablerKeyboard, "")
					})
				})

				// Right Content Pane
				ui.Column(c).Grow(1).FillHeight().Padding(16, 24).Children(func() {
					ui.Scroll(c).Grow(1).Children(func() {
						ui.Column(c).Gap(18).Children(func() {
							switch sm.ActiveTab {
							case settings.SettingsTabModels:
								a.renderSettingsTabModels(c, tc, sm)
							case settings.SettingsTabAgents:
								a.renderSettingsTabAgents(c, tc, sm)
							case settings.SettingsTabMCP:
								a.renderSettingsTabMCP(c, tc, sm)
							case settings.SettingsTabSkills:
								a.renderSettingsTabSkills(c, tc, sm)
							case settings.SettingsTabGeneral:
								a.renderSettingsTabGeneral(c, tc, sm)
							case settings.SettingsTabAppearance:
								a.renderSettingsTabAppearance(c, tc, sm)
							case settings.SettingsTabShortcuts:
								a.renderSettingsTabShortcuts(c, tc, sm)
							}
						})
					})
				})
			})
		})
	}
}

func settingsSectionTitle(tab int) string {
	switch tab {
	case settings.SettingsTabModels:
		return "Providers & Models"
	case settings.SettingsTabAgents:
		return "ACP & Agent CLIs"
	case settings.SettingsTabMCP:
		return "MCP Servers"
	case settings.SettingsTabSkills:
		return "Skills"
	case settings.SettingsTabGeneral:
		return "General"
	case settings.SettingsTabAppearance:
		return "Appearance"
	case settings.SettingsTabShortcuts:
		return "Shortcuts & Keys"
	default:
		return "Settings"
	}
}

func (a *AppUI) renderSettingsNavItem(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager, tab int, label string, icon *ui.SVG, badge string) {
	isActive := (sm.ActiveTab == tab)
	btn := ui.Row(c.Key("nav-tab-"+label)).Height(30).Radius(8).Padding(0, 10).Gap(8).AlignItems(ui.Center)
	btn.Children(func() {
		ic := ui.Icon(c, icon).Width(15).Height(15)
		if isActive {
			ic.TextColor(tc.Text)
		} else {
			ic.TextColor(tc.IconMuted)
		}

		txt := ui.Text(c, label).FontSize(12).Grow(1)
		if isActive {
			txt.FontWeight(600).TextColor(tc.Text)
		} else {
			txt.TextColor(tc.SidebarForeground)
		}

		if badge != "" {
			b := ui.Row(c).Radius(4).Padding(1, 5).Background(tc.Accent.Alpha(0.15))
			b.Children(func() {
				ui.Text(c, badge).FontSize(9).FontWeight(700).TextColor(tc.Accent)
			})
		}
	})

	if isActive {
		btn.Background(tc.SidebarRowSelected)
	} else if btn.Hovered() {
		btn.Background(tc.SidebarRowHover)
	}
	if btn.Clicked() {
		sm.ActiveTab = tab
	}
}

// ---------------------------------------------------------------------------
// Settings Sections
// ---------------------------------------------------------------------------

func (a *AppUI) renderSettingsTabModels(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager) {
	settingsSectionHeader(c, tc, "Providers & Models", "Configure LLM provider API credentials, endpoints, and default model selection.")

	// Provider Configuration Card
	settingsCard(c, tc, func() {
		ui.Column(c).Gap(12).Children(func() {
			ui.Text(c, "Active Provider").FontSize(12).FontWeight(600)
			ui.Select(c, &sm.Provider, settings.SupportedLLMProviders)

			ui.Text(c, "Default Model Name").FontSize(12).FontWeight(600)
			ui.TextInput(c, &sm.Model).Placeholder("e.g. claude-3-7-sonnet-20250219, gpt-4o, gemini-2.5-pro").Grow(1)

			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "API Key").FontSize(12).FontWeight(600)
				ui.Box(c).Grow(1)
				toggleBtn := ui.Row(c).Gap(4).AlignItems(ui.Center)
				toggleBtn.Children(func() {
					icon := icons.TablerEye
					if sm.ShowAPIKey {
						icon = icons.TablerEyeOff
					}
					ui.Icon(c, icon).Width(12).Height(12).TextColor(tc.IconMuted)
					txt := "Show"
					if sm.ShowAPIKey {
						txt = "Hide"
					}
					ui.Text(c, txt).FontSize(11).TextColor(tc.TextMuted)
				})
				if toggleBtn.Clicked() {
					sm.ShowAPIKey = !sm.ShowAPIKey
				}
			})

			in := ui.TextInput(c, &sm.APIKey).Placeholder("Enter API key (stored securely)...").Grow(1)
			if !sm.ShowAPIKey {
				in.Password()
			}

			ui.Text(c, "Custom Base URL (Optional)").FontSize(12).FontWeight(600)
			ui.TextInput(c, &sm.BaseURL).Placeholder("https://api.openai.com/v1 (or Ollama/Local URL)").Grow(1)

			ui.Row(c).AlignItems(ui.Center).Padding(4, 0, 0, 0).Children(func() {
				if sm.StatusMsg != "" {
					ui.Text(c, sm.StatusMsg).FontSize(12).TextColor(tc.Success)
				}
				ui.Box(c).Grow(1)
				saveBtn := ui.PrimaryButton(c, "Save Credentials")
				if saveBtn.Clicked() {
					if err := sm.Save(); err == nil {
						c.AddToast(ui.Toast{
							Title:       "Settings Saved",
							Description: fmt.Sprintf("Credentials saved for %s (%s)", sm.Provider, sm.Model),
							Type:        "success",
						})
					}
				}
			})
		})
	})

	// Local LLMs & Ollama Status
	settingsCard(c, tc, func() {
		ui.Column(c).Gap(6).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Local LLMs (Ollama / LM Studio)").FontSize(12).FontWeight(600)
				ui.Box(c).Grow(1)
				Badge(c, tc, "ONLINE", tc.Success)
			})
			ui.Text(c, "ForgeADE automatically routes local prompts to Ollama (http://localhost:11434) and LM Studio (http://localhost:1234) when active.").
				FontSize(11).TextColor(tc.TextMuted)
		})
	})
}

func (a *AppUI) renderSettingsTabAgents(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager) {
	settingsSectionHeader(c, tc, "ACP & Agent CLIs", "External autonomous agents conforming to the Agent Client Protocol (ACP).")

	if a.ACPMgr == nil {
		return
	}

	ui.Column(c).Gap(8).Children(func() {
		for _, ag := range a.ACPMgr.Agents {
			thisAg := ag
			settingsCard(c, tc, func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					icons.RenderProviderIcon(c, thisAg.Type, 20)
					ui.Column(c).Grow(1).Gap(2).Children(func() {
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							ui.Text(c, thisAg.Name).FontSize(13).FontWeight(600)
							if thisAg.Model != "" {
								ui.Text(c, thisAg.Model).FontSize(10).TextColor(tc.TextMuted)
							}
						})
						ui.Text(c, thisAg.Description).FontSize(11).TextColor(tc.TextMuted)
						if thisAg.Command != "" {
							ui.Text(c, "Command: "+thisAg.Command).FontSize(10).FontWeight(500).TextColor(tc.IconMuted)
						}
					})

					if thisAg.Available {
						Badge(c, tc, "AVAILABLE", tc.Success)
					} else {
						Badge(c, tc, "NOT FOUND", tc.TextMuted)
					}
				})
			})
		}

		ui.Row(c).Padding(6, 0).Children(func() {
			scanBtn := ui.Button(c, "Scan System PATH for Agents")
			if scanBtn.Clicked() {
				go func() {
					for _, ag := range a.ACPMgr.Agents {
						if _, err := exec.LookPath(ag.Command); err == nil {
							ag.Available = true
						}
					}
					if a.Window != nil {
						a.Window.Invalidate()
					}
				}()
				c.AddToast(ui.Toast{Title: "Scanning", Description: "Probed PATH for installed ACP binaries", Type: "info"})
			}
		})
	})
}

func (a *AppUI) renderSettingsTabMCP(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager) {
	settingsSectionHeader(c, tc, "Model Context Protocol (MCP) Servers", "Extend agent capabilities with tool and resource servers.")

	servers := []struct {
		Name  string
		Desc  string
		Tools int
	}{
		{"forge-filesystem", "Local workspace filesystem reads, writes, and tree operations", 8},
		{"forge-terminal", "Integrated terminal, PTY multiplexing, and command execution", 4},
		{"forge-git", "Git worktree inspection, staging, commits, and diff inspection", 6},
		{"forge-database", "SQLite and DuckDB schema reflection and query runner", 3},
	}

	ui.Column(c).Gap(8).Children(func() {
		for _, s := range servers {
			thisS := s
			settingsCard(c, tc, func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerPlug).Width(18).Height(18).TextColor(tc.Accent)
					ui.Column(c).Grow(1).Gap(2).Children(func() {
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							ui.Text(c, thisS.Name).FontSize(13).FontWeight(600)
							Badge(c, tc, fmt.Sprintf("%d tools", thisS.Tools), tc.Accent)
						})
						ui.Text(c, thisS.Desc).FontSize(11).TextColor(tc.TextMuted)
					})
					Badge(c, tc, "RUNNING", tc.Success)
				})
			})
		}
	})
}

func (a *AppUI) renderSettingsTabSkills(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager) {
	settingsSectionHeader(c, tc, "Agent Skills", "Built-in procedural skills loaded dynamically into the prompt context.")

	skills := []struct {
		Name string
		Desc string
	}{
		{"auto-verify", "Runs test suites and executes self-correction routines after code edits"},
		{"git-commit-helper", "Analyzes staged changes and generates conventional commits"},
		{"db-inspector", "Discovers SQL tables, schemas, and indices for query planning"},
		{"refactor-clean", "Safely renames symbols and updates cross-file imports"},
	}

	ui.Column(c).Gap(8).Children(func() {
		for _, sk := range skills {
			thisSk := sk
			settingsCard(c, tc, func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerWand).Width(16).Height(16).TextColor(tc.Accent)
					ui.Column(c).Grow(1).Gap(2).Children(func() {
						ui.Text(c, thisSk.Name).FontSize(13).FontWeight(600)
						ui.Text(c, thisSk.Desc).FontSize(11).TextColor(tc.TextMuted)
					})
					Badge(c, tc, "ENABLED", tc.Success)
				})
			})
		}
	})
}

func (a *AppUI) renderSettingsTabGeneral(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager) {
	settingsSectionHeader(c, tc, "General Workspace", "Workspace preferences, file persistence, and notifications.")

	settingsCard(c, tc, func() {
		ui.Column(c).Gap(10).Children(func() {
			ui.Text(c, "Workspace Root Directory").FontSize(12).FontWeight(600)
			ui.Text(c, a.Workspace.RootDir).FontSize(11).TextColor(tc.TextMuted)

			ui.Row(c).Gap(8).Children(func() {
				if ui.Button(c, "Choose Folder...").Clicked() {
					a.openWorkspaceFolder()
				}
			})
		})
	})

	settingsCard(c, tc, func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, "Auto Save Files").FontSize(13).FontWeight(600)
				ui.Text(c, "Automatically persist modified buffers to disk on focus change.").FontSize(11).TextColor(tc.TextMuted)
			})
			ui.Switch(c, &sm.AutoSave)
		})
	})
}

func (a *AppUI) renderSettingsTabAppearance(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager) {
	settingsSectionHeader(c, tc, "Appearance & Themes", "Customize UI color palettes, dark/light theme, font sizes, and layout lane width.")
	tm := a.ThemeMgr

	// 1. Dark / Light Toggle
	settingsCard(c, tc, func() {
		ui.Column(c).Gap(8).Children(func() {
			ui.Text(c, "APPEARANCE MODE").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
			ui.Row(c).Gap(8).Children(func() {
				darkBtn := func() ui.Element {
					if tm.Appearance == "dark" {
						return ui.PrimaryButton(c, "")
					}
					return ui.Button(c, "")
				}()
				darkBtn.Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, icons.TablerMoon).Width(13).Height(13)
						ui.Text(c, "Dark Mode").FontSize(12).FontWeight(600)
					})
				})
				if darkBtn.Clicked() {
					tm.Appearance = "dark"
				}

				lightBtn := func() ui.Element {
					if tm.Appearance == "light" {
						return ui.PrimaryButton(c, "")
					}
					return ui.Button(c, "")
				}()
				lightBtn.Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, icons.TablerSun).Width(13).Height(13)
						ui.Text(c, "Light Mode").FontSize(12).FontWeight(600)
					})
				})
				if lightBtn.Clicked() {
					tm.Appearance = "light"
				}
			})
		})
	})

	// 2. Palette Swatches
	settingsCard(c, tc, func() {
		ui.Column(c).Gap(8).Children(func() {
			ui.Text(c, "COLOR PALETTE (6 BUILT-IN PACKS)").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
			ui.Row(c).Gap(8).Wrap().Children(func() {
				for _, p := range tm.AvailablePalettes() {
					thisP := p
					isSel := (tm.PaletteID == thisP.ID)
					card := ui.Row(c.Key("pal-card-"+thisP.ID)).Radius(8).Padding(8, 12).Gap(8).AlignItems(ui.Center)
					card.Children(func() {
						ui.Box(c).Size(12, 12).Radius(6).Background(ui.Hex(thisP.SwatchHex))
						ui.Text(c, thisP.Name).FontSize(12).FontWeight(func() int {
							if isSel {
								return 600
							}
							return 400
						}())
						if isSel {
							ui.Icon(c, icons.TablerCheck).Width(13).Height(13).TextColor(tc.Accent)
						}
					})
					if isSel {
						card.Background(tc.Accent.Alpha(0.12)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3))
					} else {
						card.Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
						if card.Hovered() {
							card.Background(tc.Surface.Alpha(0.8))
						}
					}
					if card.Clicked() {
						tm.PaletteID = thisP.ID
					}
				}
			})
		})
	})

	// 3. Chat Lane Width & Diff Color Scheme
	settingsCard(c, tc, func() {
		ui.Row(c).Gap(24).Children(func() {
			ui.Column(c).Grow(1).Gap(6).Children(func() {
				ui.Text(c, "Chat Lane Width").FontSize(12).FontWeight(600)
				ui.Row(c).Gap(6).Children(func() {
					for _, opt := range []struct{ ID, Label string }{
						{"comfortable", "Comfortable (736px)"},
						{"wide", "Wide (928px)"},
						{"full", "Full"},
					} {
						thisOpt := opt
						btn := func() ui.Element {
							if tm.ChatWidth == thisOpt.ID {
								return ui.PrimaryButton(c, thisOpt.Label).FontSize(11)
							}
							return ui.Button(c, thisOpt.Label).FontSize(11)
						}()
						if btn.Clicked() {
							tm.ChatWidth = thisOpt.ID
						}
					}
				})
			})

			ui.Column(c).Grow(1).Gap(6).Children(func() {
				ui.Text(c, "Diff Scheme").FontSize(12).FontWeight(600)
				ui.Row(c).Gap(6).Children(func() {
					for _, opt := range []struct{ ID, Label string }{
						{"classic", "Classic (Green/Red)"},
						{"modern", "Modern (Blue/Orange)"},
					} {
						thisOpt := opt
						btn := func() ui.Element {
							if tm.DiffScheme == thisOpt.ID {
								return ui.PrimaryButton(c, thisOpt.Label).FontSize(11)
							}
							return ui.Button(c, thisOpt.Label).FontSize(11)
						}()
						if btn.Clicked() {
							tm.DiffScheme = thisOpt.ID
						}
					}
				})
			})
		})
	})
}

func (a *AppUI) renderSettingsTabShortcuts(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager) {
	settingsSectionHeader(c, tc, "Terminal & Shortcuts", "Shell configuration and keybinding shortcuts reference.")

	settingsCard(c, tc, func() {
		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, "Default Shell").FontSize(12).FontWeight(600)
			ui.Select(c, &sm.DefaultShell, []string{"/bin/bash", "/bin/zsh", "/bin/sh"})
		})
	})

	shortcuts := []struct{ Key, Desc string }{
		{"⌘S", "Save current active buffer to disk"},
		{"⌘P", "Quick open file by fuzzy name"},
		{"⇧⌘P", "Open command palette"},
		{"⌘M", "Select AI Model & Harness"},
		{"⌘1", "Switch to Autonomous Agent Mode"},
		{"⌘2", "Switch to Code Editor Mode"},
		{"⌘B", "Toggle left sidebar"},
		{"⌘J", "Toggle right drawer"},
		{"⌘`", "Toggle collapsible bottom terminal"},
		{"⌘N", "Start a new task session"},
		{"⌘,", "Open Settings"},
		{"⌘/", "Toggle Dark / Light appearance"},
		{"⌘W", "Close active editor tab"},
		{"Ctrl+Tab", "Next editor tab"},
		{"⇧Ctrl+Tab", "Previous editor tab"},
	}

	settingsCard(c, tc, func() {
		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, "KEYBOARD SHORTCUTS REFERENCE").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
			for _, sc := range shortcuts {
				s := sc
				ui.Row(c.Key("sc-"+s.Key)).Height(26).Padding(0, 8).AlignItems(ui.Center).Children(func() {
					kbd := ui.Row(c).Radius(4).Padding(2, 6).Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
					kbd.Children(func() {
						ui.Text(c, s.Key).FontSize(11).FontWeight(600).TextColor(tc.Accent)
					})
					ui.Box(c).Width(12)
					ui.Text(c, s.Desc).FontSize(12).TextColor(tc.Text)
				})
			}
		})
	})
}

func settingsSectionHeader(c *ui.Context, tc theme.ThemeColors, title, desc string) {
	ui.Column(c).Gap(3).Children(func() {
		ui.Text(c, title).FontSize(18).FontWeight(700)
		ui.Text(c, desc).FontSize(12).TextColor(tc.TextMuted)
	})
}

func settingsCard(c *ui.Context, tc theme.ThemeColors, children func()) {
	ui.Column(c).Padding(14, 16).Radius(10).Background(tc.Surface.Alpha(0.5)).
		BorderWidth(1).BorderColor(tc.Border).Children(children)
}
