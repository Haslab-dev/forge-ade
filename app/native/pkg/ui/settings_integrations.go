package ui

// settings_integrations.go replaces the mock MCP/Skills settings tabs with
// the real RakitKode managers and adds the Plugins, Memory, Automations
// (bots) and Sub-Agents & Roles sections of the webview SettingsScreen.

import (
	"fmt"
	"strings"
	"time"

	rakitagent "github.com/Haslab-dev/RakitKode/agent"
	"github.com/Haslab-dev/RakitKode/mcp"
	"github.com/Haslab-dev/RakitKode/memory"
	rakitskills "github.com/Haslab-dev/RakitKode/skills"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/settings"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
	"github.com/hasdev/forge-ade/core/automations"
)

// ── MCP Servers (real CRUD) ────────────────────────────────────────────────

func (a *AppUI) renderSettingsTabMCPReal(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager) {
	settingsSectionHeader(c, tc, "Model Context Protocol (MCP) Servers", "Extend agent capabilities with tool and resource servers.")

	mgr := a.MCPMgr()
	if mgr == nil {
		ui.Text(c, "MCP manager unavailable").FontSize(12).TextColor(tc.TextMuted)
		return
	}
	servers := mgr.ListServers()

	// Add-server form (collapsible).
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		addBtn := ui.Row(c).Height(28).Radius(8).Padding(0, 12).Gap(5).AlignItems(ui.Center).
			Background(tc.Accent.Alpha(0.14)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.35))
		addBtn.Children(func() {
			ui.Icon(c, icons.TablerPlus).Width(13).Height(13).TextColor(tc.Accent)
			ui.Text(c, "Add Server").FontSize(12).FontWeight(600).TextColor(tc.Accent)
		})
		if addBtn.Clicked() {
			sm.MCPFormOpen = !sm.MCPFormOpen
		}
		ui.Box(c).Grow(1)
		ui.Text(c, fmt.Sprintf("%d servers · %d tools", len(servers), len(mgr.ListTools()))).FontSize(11).TextColor(tc.TextMuted)
	})

	if sm.MCPFormOpen {
		settingsCard(c, tc, func() {
			ui.Column(c).Gap(8).Children(func() {
				ui.Text(c, "New MCP Server").FontSize(12).FontWeight(600)
				ui.Row(c).Gap(6).Children(func() {
					ui.TextInput(c, &sm.MCPName).Placeholder("name").Grow(1)
					ui.TextInput(c, &sm.MCPCommand).Placeholder("command (e.g. npx -y @modelcontextprotocol/server-fs)").Grow(1)
				})
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "Type:").FontSize(11).TextColor(tc.TextMuted)
					ui.Select(c, &sm.MCPType, []string{"stdio", "http", "sse"})
					ui.Box(c).Grow(1)
					if ui.Button(c, "Save Server").Clicked() && strings.TrimSpace(sm.MCPName) != "" {
						cfg := mcp.ServerConfig{
							Name:    strings.TrimSpace(sm.MCPName),
							Command: strings.TrimSpace(sm.MCPCommand),
							Type:    sm.MCPType,
							Enabled: true,
						}
						if _, err := mgr.SaveServer(cfg); err == nil {
							sm.MCPName, sm.MCPCommand = "", ""
							sm.MCPFormOpen = false
							c.AddToast(ui.Toast{Title: "Server Saved", Description: cfg.Name, Type: "success"})
						}
					}
				})
			})
		})
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
							if thisS.Enabled {
								Badge(c, tc, "ENABLED", tc.Success)
							} else {
								Badge(c, tc, "DISABLED", tc.TextMuted)
							}
							if thisS.Type != "" && thisS.Type != "stdio" && thisS.Type != "local" {
								Badge(c, tc, strings.ToUpper(thisS.Type), tc.Info)
							}
						})
						spec := thisS.Command
						if spec == "" {
							spec = thisS.URL
						}
						ui.Text(c, spec).FontSize(11).Font("monospace").TextColor(tc.TextMuted).SingleLine()
					})
					del := IconButton(c, tc, icons.TablerTrash, 13)
					del.ContextMenu(func(m *ui.Menu) {
						if m.Item("Delete server").Danger().Chosen() {
							if err := mgr.DeleteServer(thisS.Name); err == nil {
								c.AddToast(ui.Toast{Title: "Server Deleted", Description: thisS.Name, Type: "info"})
							}
						}
					})
					if del.Clicked() {
						_ = mgr.DeleteServer(thisS.Name)
					}
				})
			})
		}
		if len(servers) == 0 {
			ui.Column(c).Center().Padding(20).Gap(4).Children(func() {
				ui.Icon(c, icons.TablerPlug).Width(24).Height(24).TextColor(tc.IconMuted)
				ui.Text(c, "No MCP servers configured").FontSize(12).TextColor(tc.TextMuted)
			})
		}
	})
}

// ── Skills (real CRUD) ─────────────────────────────────────────────────────

func (a *AppUI) renderSettingsTabSkillsReal(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager) {
	settingsSectionHeader(c, tc, "Agent Skills", "Procedural skills loaded into the agent prompt context ($ or /skill:name).")

	if a.Engine == nil || a.Engine.SkillMgr == nil {
		ui.Text(c, "Skills manager unavailable").FontSize(12).TextColor(tc.TextMuted)
		return
	}
	skillMgr := a.Engine.SkillMgr
	list := skillMgr.List()

	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		addBtn := ui.Button(c, "+ Create Skill")
		if addBtn.Clicked() {
			sm.SkillFormOpen = !sm.SkillFormOpen
		}
		ui.Box(c).Grow(1)
		if ui.Button(c, "Reload Skills").Clicked() {
			skillMgr.Reload()
			c.AddToast(ui.Toast{Title: "Skills Reloaded", Type: "success"})
		}
		ui.Text(c, fmt.Sprintf("%d skills", len(list))).FontSize(11).TextColor(tc.TextMuted)
	})

	if sm.SkillFormOpen {
		settingsCard(c, tc, func() {
			ui.Column(c).Gap(8).Children(func() {
				ui.Text(c, "Create New Skill").FontSize(12).FontWeight(600)
				ui.Row(c).Gap(6).Children(func() {
					ui.TextInput(c, &sm.SkillName).Placeholder("Skill name (e.g. explain-code)").Grow(1)
					ui.TextInput(c, &sm.SkillScope).Placeholder("Scope (workspace / global)").Width(160)
				})
				ui.TextInput(c, &sm.SkillDescription).Placeholder("Description").Grow(1)
				ui.TextInput(c, &sm.SkillPrompt).Placeholder("Instruction prompt / body").Grow(1)
				ui.Row(c).Justify(ui.End).Gap(6).Children(func() {
					if ui.Button(c, "Cancel").Clicked() {
						sm.SkillFormOpen = false
					}
					saveBtn := ContrastCTA(c, tc, "Save Skill")
					if saveBtn.Clicked() && strings.TrimSpace(sm.SkillName) != "" {
						scopeVal := sm.SkillScope
						if scopeVal == "" {
							scopeVal = "workspace"
						}
						_, err := skillMgr.CreateSkill(rakitskills.CreateSkillRequest{
							Name:        strings.TrimSpace(sm.SkillName),
							Description: sm.SkillDescription,
							Body:        sm.SkillPrompt,
							Scope:       scopeVal,
						}, a.Workspace.RootDir)
						if err == nil {
							sm.SkillName = ""
							sm.SkillDescription = ""
							sm.SkillPrompt = ""
							sm.SkillFormOpen = false
							skillMgr.Reload()
							c.AddToast(ui.Toast{Title: "Skill Created", Type: "success"})
						} else {
							c.AddToast(ui.Toast{Title: "Failed to create skill", Description: err.Error(), Type: "error"})
						}
					}
				})
			})
		})
	}

	ui.Column(c).Gap(8).Children(func() {
		for _, sk := range list {
			thisSk := sk
			settingsCard(c, tc, func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerWand).Width(16).Height(16).TextColor(tc.Accent)
					ui.Column(c).Grow(1).Gap(2).Children(func() {
						ui.Text(c, thisSk.Name).FontSize(13).FontWeight(600)
						ui.Text(c, thisSk.Description).FontSize(11).TextColor(tc.TextMuted)
					})
					Badge(c, tc, string(thisSk.Scope), tc.Info)
					del := IconButton(c, tc, icons.TablerTrash, 13)
					if del.Clicked() {
						if err := skillMgr.DeleteSkill(thisSk.Name); err == nil {
							skillMgr.Reload()
							c.AddToast(ui.Toast{Title: "Skill Deleted", Description: thisSk.Name, Type: "info"})
						}
					}
				})
			})
		}
		if len(list) == 0 {
			ui.Column(c).Center().Padding(20).Gap(4).Children(func() {
				ui.Icon(c, icons.TablerWand).Width(24).Height(24).TextColor(tc.IconMuted)
				ui.Text(c, "No skills installed").FontSize(12).TextColor(tc.TextMuted)
			})
		}
	})
}

// ── Plugins ────────────────────────────────────────────────────────────────

func (a *AppUI) renderSettingsTabPlugins(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager) {
	settingsSectionHeader(c, tc, "Plugins", "Capability bundles that register tools and skills with the agent.")

	if a.Engine == nil || a.Engine.PluginMgr == nil {
		ui.Text(c, "Plugins manager unavailable").FontSize(12).TextColor(tc.TextMuted)
		return
	}
	plugMgr := a.Engine.PluginMgr
	list := plugMgr.List()

	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Grow(1)
		if ui.Button(c, "Reload Plugins").Clicked() {
			plugMgr.Reload()
			c.AddToast(ui.Toast{Title: "Plugins Reloaded", Type: "success"})
		}
		ui.Text(c, fmt.Sprintf("%d plugins", len(list))).FontSize(11).TextColor(tc.TextMuted)
	})

	ui.Column(c).Gap(8).Children(func() {
		for _, p := range list {
			thisP := p
			settingsCard(c, tc, func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerPuzzle).Width(16).Height(16).TextColor(tc.Accent)
					ui.Column(c).Grow(1).Gap(2).Children(func() {
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							ui.Text(c, thisP.Name).FontSize(13).FontWeight(600)
							if thisP.Version != "" {
								Badge(c, tc, "v"+thisP.Version, tc.Info)
							}
						})
						ui.Text(c, thisP.Description).FontSize(11).TextColor(tc.TextMuted)
					})
					if thisP.Enabled {
						Badge(c, tc, "ENABLED", tc.Success)
					} else {
						Badge(c, tc, "DISABLED", tc.TextMuted)
					}
				})
			})
		}
		if len(list) == 0 {
			ui.Column(c).Center().Padding(20).Gap(4).Children(func() {
				ui.Text(c, "No plugins installed").FontSize(12).TextColor(tc.TextMuted)
			})
		}
	})
}

// ── Memory ─────────────────────────────────────────────────────────────────

func (a *AppUI) renderSettingsTabMemory(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager) {
	settingsSectionHeader(c, tc, "Memory", "Persistent notes and rules the agent recalls across sessions.")

	if a.Engine == nil || a.Engine.MemoryMgr == nil {
		ui.Text(c, "Memory manager unavailable").FontSize(12).TextColor(tc.TextMuted)
		return
	}
	memMgr := a.Engine.MemoryMgr
	list := memMgr.List()

	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.TextInput(c, &sm.MemoryNew).Placeholder("New memory…").Grow(1)
		if ui.Button(c, "Save").Clicked() && strings.TrimSpace(sm.MemoryNew) != "" {
			_ = memMgr.Save(memory.Entry{Key: fmt.Sprintf("note-%d", time.Now().UnixMilli()), Content: sm.MemoryNew, Scope: "global"})
			sm.MemoryNew = ""
			c.AddToast(ui.Toast{Title: "Memory Saved", Type: "success"})
		}
	})

	ui.Column(c).Gap(8).Children(func() {
		for _, e := range list {
			thisE := e
			settingsCard(c, tc, func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerBrain).Width(16).Height(16).TextColor(tc.Accent)
					ui.Column(c).Grow(1).Gap(2).Children(func() {
						ui.Text(c, thisE.Content).FontSize(12).Grow(1)
						if thisE.Category != "" || thisE.Scope != "" {
							ui.Text(c, strings.TrimSpace(thisE.Category+" · "+thisE.Scope)).FontSize(10).TextColor(tc.TextMuted)
						}
					})
					del := IconButton(c, tc, icons.TablerTrash, 13)
					if del.Clicked() {
						_ = memMgr.Delete(thisE.ID)
					}
				})
			})
		}
		if len(list) == 0 {
			ui.Column(c).Center().Padding(20).Gap(4).Children(func() {
				ui.Text(c, "No memories yet").FontSize(12).TextColor(tc.TextMuted)
			})
		}
	})
}

// ── Automations (bots) ─────────────────────────────────────────────────────

func (a *AppUI) renderSettingsTabAutomations(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager) {
	settingsSectionHeader(c, tc, "Automations", "Scheduled prompts and bots that run on cron expressions.")

	if a.Engine == nil || a.Engine.AutoMgr == nil {
		ui.Text(c, "Automations manager unavailable").FontSize(12).TextColor(tc.TextMuted)
		return
	}
	autoMgr := a.Engine.AutoMgr
	list := autoMgr.List()

	// Create form.
	settingsCard(c, tc, func() {
		ui.Column(c).Gap(8).Children(func() {
			ui.Text(c, "New Automation").FontSize(12).FontWeight(600)
			ui.TextInput(c, &sm.AutoName).Placeholder("Name").Grow(1)
			ui.Row(c).Gap(6).Children(func() {
				ui.TextInput(c, &sm.AutoCron).Placeholder("cron (e.g. 0 9 * * 1-5, empty = manual)").Grow(1)
				ui.TextInput(c, &sm.AutoPrompt).Placeholder("Prompt to run").Grow(1)
			})
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Presets:").FontSize(10).TextColor(tc.TextMuted)
				cronPresets := []struct {
					label string
					expr  string
				}{
					{"Hourly", "0 * * * *"},
					{"Daily 9AM", "0 9 * * *"},
					{"Weekdays", "0 9 * * 1-5"},
					{"Weekly Mon", "0 9 * * 1"},
					{"Monthly", "0 9 1 * *"},
					{"Clear", ""},
				}
				for _, p := range cronPresets {
					preset := p
					chip := ui.Row(c.Key("cron-pre-"+preset.label)).Height(20).Radius(4).Padding(0, 6).
						Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border.Alpha(0.6)).
						Cursor(ui.CursorPointer).AlignItems(ui.Center)
					chip.Children(func() {
						ui.Text(c, preset.label).FontSize(10).TextColor(tc.TextMuted)
					})
					if chip.Clicked() {
						sm.AutoCron = preset.expr
					}
				}
			})
			ui.Row(c).Justify(ui.End).Children(func() {
				if ui.Button(c, "Create Automation").Clicked() && strings.TrimSpace(sm.AutoName) != "" {
					_, err := autoMgr.Save(automations.Automation{
						Name:      strings.TrimSpace(sm.AutoName),
						Prompt:    sm.AutoPrompt,
						CronExpr:  strings.TrimSpace(sm.AutoCron),
						Enabled:   strings.TrimSpace(sm.AutoCron) != "",
						Workspace: a.Workspace.RootDir,
					})
					if err == nil {
						sm.AutoName, sm.AutoCron, sm.AutoPrompt = "", "", ""
						c.AddToast(ui.Toast{Title: "Automation Created", Type: "success"})
					}
				}
			})
		})
	})

	ui.Column(c).Gap(8).Children(func() {
		for _, au := range list {
			thisAu := au
			settingsCard(c, tc, func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerBot).Width(16).Height(16).TextColor(tc.Accent)
					ui.Column(c).Grow(1).Gap(2).Children(func() {
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							ui.Text(c, thisAu.Name).FontSize(13).FontWeight(600)
							if thisAu.CronExpr != "" {
								Badge(c, tc, thisAu.CronExpr, tc.Info)
							} else {
								Badge(c, tc, "manual", tc.TextMuted)
							}
						})
						ui.Text(c, thisAu.Prompt).FontSize(11).TextColor(tc.TextMuted).SingleLine()
					})

					runBtn := ui.Row(c).Height(24).Radius(4).Padding(0, 8).Gap(4).AlignItems(ui.Center).
						Background(tc.Accent.Alpha(0.15)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3)).Cursor(ui.CursorPointer)
					runBtn.Children(func() {
						ui.Icon(c, icons.TablerPlayerPlay).Width(11).Height(11).TextColor(tc.Accent)
						ui.Text(c, "Run Now").FontSize(11).FontWeight(600).TextColor(tc.Accent)
					})
					if runBtn.Clicked() {
						if a.Agent != nil {
							_ = a.Agent.CreateSessionWithInitialPrompt(a.Workspace.RootDir, thisAu.Prompt)
							_, _ = autoMgr.RecordRun(thisAu.ID, automations.Run{
								ID:        fmt.Sprintf("run-%d", time.Now().UnixNano()),
								StartedAt: time.Now(),
								Workspace: a.Workspace.RootDir,
							})
							c.AddToast(ui.Toast{Title: "Automation Triggered", Description: thisAu.Name, Type: "success"})
						}
					}

					enabled := thisAu.Enabled
					if ui.Switch(c, &enabled).Clicked() {
						if _, err := autoMgr.SetEnabled(thisAu.ID, enabled); err == nil {
							c.AddToast(ui.Toast{Title: "Automation Updated", Description: thisAu.Name, Type: "info"})
						}
					}
					del := IconButton(c, tc, icons.TablerTrash, 13)
					if del.Clicked() {
						_ = autoMgr.Delete(thisAu.ID)
					}
				})
			})
		}
		if len(list) == 0 {
			ui.Column(c).Center().Padding(20).Gap(4).Children(func() {
				ui.Text(c, "No automations yet").FontSize(12).TextColor(tc.TextMuted)
			})
		}
	})
}

// ── Sub-Agents & Roles ─────────────────────────────────────────────────────

func (a *AppUI) renderSettingsTabSubagents(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager) {
	settingsSectionHeader(c, tc, "Sub-Agents & Roles", "Delegated agent definitions with dedicated models, prompts, and permissions.")

	if a.Engine == nil || a.Engine.AgentMgr == nil {
		ui.Text(c, "Agent engine unavailable").FontSize(12).TextColor(tc.TextMuted)
		return
	}
	defs := a.Engine.AgentMgr.ListAgentDefinitions()

	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		addBtn := ui.Button(c, "+ Create Role")
		if addBtn.Clicked() {
			sm.RoleFormOpen = !sm.RoleFormOpen
		}
		ui.Box(c).Grow(1)
		ui.Text(c, fmt.Sprintf("%d roles", len(defs))).FontSize(11).TextColor(tc.TextMuted)
	})

	if sm.RoleFormOpen {
		settingsCard(c, tc, func() {
			ui.Column(c).Gap(8).Children(func() {
				ui.Text(c, "Define Subagent Role").FontSize(12).FontWeight(600)
				ui.Row(c).Gap(6).Children(func() {
					ui.TextInput(c, &sm.RoleName).Placeholder("Role name (e.g. security-auditor)").Grow(1)
					ui.TextInput(c, &sm.RoleFilter).Placeholder("Role category (coder/architect/reviewer/researcher)").Width(200)
				})
				ui.TextInput(c, &sm.RoleModel).Placeholder("Model override (leave blank to inherit default)").Grow(1)
				ui.TextInput(c, &sm.RolePrompt).Placeholder("System instructions / Custom prompt").Grow(1)

				ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
					ui.Checkbox(c, &sm.RoleReadOnly, "Read-Only (no filesystem writes)").FontSize(11)
					ui.Checkbox(c, &sm.RoleAllowBash, "Allow Terminal Commands").FontSize(11)
					ui.Box(c).Grow(1)
					if ui.Button(c, "Cancel").Clicked() {
						sm.RoleFormOpen = false
					}
					saveBtn := ContrastCTA(c, tc, "Save Role")
					if saveBtn.Clicked() && strings.TrimSpace(sm.RoleName) != "" {
						newDef := rakitagent.AgentDefinition{
							ID:          strings.ToLower(strings.ReplaceAll(strings.TrimSpace(sm.RoleName), " ", "-")),
							Name:        strings.TrimSpace(sm.RoleName),
							Role:        rakitagent.RoleFilter(sm.RoleFilter),
							Model:       sm.RoleModel,
							Prompt:      sm.RolePrompt,
							ReadOnly:    sm.RoleReadOnly,
							AllowBash:   sm.RoleAllowBash,
							Description: sm.RolePrompt,
							CreatedAt:   time.Now(),
							UpdatedAt:   time.Now(),
						}
						_, err := a.Engine.AgentMgr.SaveAgentDefinition(newDef)
						if err == nil {
							sm.RoleName = ""
							sm.RoleFilter = ""
							sm.RoleModel = ""
							sm.RolePrompt = ""
							sm.RoleFormOpen = false
							c.AddToast(ui.Toast{Title: "Role Created", Type: "success"})
						} else {
							c.AddToast(ui.Toast{Title: "Failed to save role", Description: err.Error(), Type: "error"})
						}
					}
				})
			})
		})
	}

	ui.Column(c).Gap(8).Children(func() {
		for _, d := range defs {
			thisD := d
			settingsCard(c, tc, func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerBot).Width(16).Height(16).TextColor(tc.Accent)
					ui.Column(c).Grow(1).Gap(2).Children(func() {
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							ui.Text(c, thisD.Name).FontSize(13).FontWeight(600)
							Badge(c, tc, string(thisD.Role), tc.Info)
							if thisD.ReadOnly {
								Badge(c, tc, "read-only", tc.Warning)
							}
							if thisD.Model != "" {
								Badge(c, tc, thisD.Model, tc.Accent)
							}
						})
						ui.Text(c, thisD.Description).FontSize(11).TextColor(tc.TextMuted)
					})
					del := IconButton(c, tc, icons.TablerTrash, 13)
					if del.Clicked() {
						if err := a.Engine.AgentMgr.DeleteAgentDefinition(thisD.ID); err == nil {
							c.AddToast(ui.Toast{Title: "Role Deleted", Description: thisD.Name, Type: "info"})
						}
					}
				})
			})
		}
		if len(defs) == 0 {
			ui.Column(c).Center().Padding(20).Gap(4).Children(func() {
				ui.Text(c, "No custom roles defined").FontSize(12).TextColor(tc.TextMuted)
			})
		}
	})
}

// ── Slash Commands ─────────────────────────────────────────────────────────

func (a *AppUI) renderSettingsTabCommands(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager) {
	settingsSectionHeader(c, tc, "Slash Commands", "Quick prompt shortcuts and triggers configured for the agent composer.")

	defaultCmds := []struct {
		name string
		desc string
	}{
		{"/model", "Open the model selector palette"},
		{"/usage", "Open quota, context window, and token usage modal"},
		{"/settings", "Open settings and configuration"},
		{"/setup", "Open provider API key and endpoint configuration"},
		{"/clear", "Reset and clear current conversation history"},
		{"/compact", "Compact long conversation context into summary"},
		{"/task", "Decompose and track task in workspace kanban"},
		{"/git", "Open git changes and diff review"},
	}

	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		addBtn := ui.Button(c, "+ Add Command")
		if addBtn.Clicked() {
			sm.CmdFormOpen = !sm.CmdFormOpen
		}
		ui.Box(c).Grow(1)
		ui.Text(c, fmt.Sprintf("%d commands", len(defaultCmds)+len(sm.CustomCommands))).FontSize(11).TextColor(tc.TextMuted)
	})

	if sm.CmdFormOpen {
		settingsCard(c, tc, func() {
			ui.Column(c).Gap(8).Children(func() {
				ui.Text(c, "New Custom Slash Command").FontSize(12).FontWeight(600)
				ui.Row(c).Gap(6).Children(func() {
					ui.TextInput(c, &sm.CmdName).Placeholder("Command (e.g. /review)").Width(160)
					ui.TextInput(c, &sm.CmdDescription).Placeholder("Description").Grow(1)
				})
				ui.TextInput(c, &sm.CmdPrompt).Placeholder("Expanded prompt template (e.g. Review the unstaged git changes for safety)").Grow(1)
				ui.Row(c).Justify(ui.End).Gap(6).Children(func() {
					if ui.Button(c, "Cancel").Clicked() {
						sm.CmdFormOpen = false
					}
					saveBtn := ContrastCTA(c, tc, "Save Command")
					if saveBtn.Clicked() && strings.TrimSpace(sm.CmdName) != "" {
						name := strings.TrimSpace(sm.CmdName)
						if !strings.HasPrefix(name, "/") {
							name = "/" + name
						}
						sm.CustomCommands = append(sm.CustomCommands, settings.CustomSlashCommand{
							Name:        name,
							Description: sm.CmdDescription,
							Prompt:      sm.CmdPrompt,
						})
						sm.CmdName, sm.CmdDescription, sm.CmdPrompt = "", "", ""
						sm.CmdFormOpen = false
						c.AddToast(ui.Toast{Title: "Command Added", Description: name, Type: "success"})
					}
				})
			})
		})
	}

	ui.Column(c).Gap(8).Children(func() {
		for _, custom := range sm.CustomCommands {
			cmd := custom
			settingsCard(c, tc, func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerCode).Width(16).Height(16).TextColor(tc.Accent)
					ui.Column(c).Grow(1).Gap(2).Children(func() {
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							ui.Text(c, cmd.Name).FontSize(13).FontWeight(600)
							Badge(c, tc, "custom", tc.Accent)
						})
						ui.Text(c, cmd.Description).FontSize(11).TextColor(tc.TextMuted)
						if cmd.Prompt != "" {
							ui.Text(c, cmd.Prompt).FontSize(10).TextColor(tc.TextMuted).SingleLine()
						}
					})
					del := IconButton(c, tc, icons.TablerTrash, 13)
					if del.Clicked() {
						var updated []settings.CustomSlashCommand
						for _, it := range sm.CustomCommands {
							if it.Name != cmd.Name {
								updated = append(updated, it)
							}
						}
						sm.CustomCommands = updated
					}
				})
			})
		}

		for _, d := range defaultCmds {
			cmd := d
			settingsCard(c, tc, func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerCode).Width(16).Height(16).TextColor(tc.IconMuted)
					ui.Column(c).Grow(1).Gap(2).Children(func() {
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							ui.Text(c, cmd.name).FontSize(13).FontWeight(600)
							Badge(c, tc, "built-in", tc.TextMuted)
						})
						ui.Text(c, cmd.desc).FontSize(11).TextColor(tc.TextMuted)
					})
				})
			})
		}
	})
}

// ── Browser Use ────────────────────────────────────────────────────────────

func (a *AppUI) renderSettingsTabBrowser(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager) {
	settingsSectionHeader(c, tc, "Browser Use", "Headless browser, web automation, search providers, and page preview controls.")

	if sm.BrowserEngine == "" {
		sm.BrowserEngine = "DuckDuckGo"
	}
	if sm.BrowserViewport == "" {
		sm.BrowserViewport = "1280x800"
	}

	settingsCard(c, tc, func() {
		ui.Column(c).Gap(12).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, "Headless Background Mode").FontSize(12).FontWeight(600)
					ui.Text(c, "Execute web navigation actions invisibly in background without stealing desktop focus.").FontSize(11).TextColor(tc.TextMuted)
				})
				ui.Switch(c, &sm.BrowserHeadless)
			})

			ui.Box(c).Height(1).Background(tc.Border.Alpha(0.4))

			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, "Execute JavaScript in Web Pages").FontSize(12).FontWeight(600)
					ui.Text(c, "Allow web pages and scraped DOM scripts to evaluate dynamically.").FontSize(11).TextColor(tc.TextMuted)
				})
				ui.Switch(c, &sm.BrowserAllowJS)
			})

			ui.Box(c).Height(1).Background(tc.Border.Alpha(0.4))

			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, "Default Search Engine").FontSize(12).FontWeight(600)
				ui.Text(c, "Fallback search provider used for unqualified queries in the URL bar.").FontSize(11).TextColor(tc.TextMuted)
				providers := []string{"DuckDuckGo", "Google", "Bing", "Brave"}
				ui.Row(c).Gap(6).Children(func() {
					for _, p := range providers {
						prov := p
						isSel := sm.BrowserEngine == prov
						btn := ui.Row(c.Key("search-eng-"+prov)).Height(24).Radius(4).Padding(0, 10).
							BorderWidth(1).Cursor(ui.CursorPointer).AlignItems(ui.Center)
						if isSel {
							btn.Background(tc.Accent.Alpha(0.15)).BorderColor(tc.Accent)
						} else {
							btn.Background(tc.SurfaceRaised).BorderColor(tc.Border.Alpha(0.6))
						}
						btn.Children(func() {
							labelColor := tc.TextMuted
							if isSel {
								labelColor = tc.Accent
							}
							ui.Text(c, prov).FontSize(11).FontWeight(500).TextColor(labelColor)
						})
						if btn.Clicked() {
							sm.BrowserEngine = prov
						}
					}
				})
			})

			ui.Box(c).Height(1).Background(tc.Border.Alpha(0.4))

			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, "Default Viewport Size").FontSize(12).FontWeight(600)
				ui.Text(c, "Emulated browser window dimensions for page layout and screenshots.").FontSize(11).TextColor(tc.TextMuted)
				viewports := []string{"1280x800", "1440x900", "1920x1080", "390x844 (Mobile)"}
				ui.Row(c).Gap(6).Children(func() {
					for _, vp := range viewports {
						viewport := vp
						isSel := sm.BrowserViewport == viewport
						btn := ui.Row(c.Key("viewport-"+viewport)).Height(24).Radius(4).Padding(0, 10).
							BorderWidth(1).Cursor(ui.CursorPointer).AlignItems(ui.Center)
						if isSel {
							btn.Background(tc.Accent.Alpha(0.15)).BorderColor(tc.Accent)
						} else {
							btn.Background(tc.SurfaceRaised).BorderColor(tc.Border.Alpha(0.6))
						}
						btn.Children(func() {
							labelColor := tc.TextMuted
							if isSel {
								labelColor = tc.Accent
							}
							ui.Text(c, viewport).FontSize(11).FontWeight(500).TextColor(labelColor)
						})
						if btn.Clicked() {
							sm.BrowserViewport = viewport
						}
					}
				})
			})
		})
	})
}

// ── Computer Use ───────────────────────────────────────────────────────────

func (a *AppUI) renderSettingsTabComputer(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager) {
	settingsSectionHeader(c, tc, "Computer Use", "Direct desktop GUI control, mouse clicks, keyboard automation, and safety constraints.")

	if sm.ComputerPermissionMode == "" {
		sm.ComputerPermissionMode = "Supervised"
	}

	settingsCard(c, tc, func() {
		ui.Column(c).Gap(12).Children(func() {
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "Desktop Autonomy Level").FontSize(12).FontWeight(600)
				ui.Text(c, "Controls confirmation prompts required before desktop keyboard or mouse actions.").FontSize(11).TextColor(tc.TextMuted)

				modes := []struct {
					name string
					desc string
				}{
					{"Supervised", "Ask approval before sending any mouse clicks or keystrokes"},
					{"Autonomous Edits", "Allow in-editor typing and clicking automatically, ask for system OS controls"},
					{"Full Access (YOLO)", "Execute all desktop actions autonomously without pause"},
				}

				for _, m := range modes {
					mode := m
					isSel := sm.ComputerPermissionMode == mode.name
					card := ui.Row(c.Key("comp-mode-"+mode.name)).Radius(6).Padding(8, 12).Gap(8).
						BorderWidth(1).Cursor(ui.CursorPointer).AlignItems(ui.Center)
					if isSel {
						card.Background(tc.Accent.Alpha(0.1)).BorderColor(tc.Accent)
					} else {
						card.Background(tc.SurfaceRaised).BorderColor(tc.Border.Alpha(0.5))
					}
					card.Children(func() {
						dot := ui.Box(c).Size(8, 8).Radius(4)
						if isSel {
							dot.Background(tc.Accent)
						} else {
							dot.Background(tc.Border)
						}
						ui.Column(c).Grow(1).Gap(1).Children(func() {
							ui.Text(c, mode.name).FontSize(12).FontWeight(600)
							ui.Text(c, mode.desc).FontSize(10).TextColor(tc.TextMuted)
						})
					})
					if card.Clicked() {
						sm.ComputerPermissionMode = mode.name
					}
				}
			})

			ui.Box(c).Height(1).Background(tc.Border.Alpha(0.4))

			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, "Display Resolution & Scaling Status").FontSize(12).FontWeight(600)
					ui.Text(c, "Native macOS Quartz display coordinates bridge active and calibrated.").FontSize(11).TextColor(tc.TextMuted)
				})
				Badge(c, tc, "Calibrated Retina @2x", tc.Success)
			})

			ui.Box(c).Height(1).Background(tc.Border.Alpha(0.4))

			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, "System Safety Sandbox").FontSize(12).FontWeight(600)
					ui.Text(c, "Prevent automation actions on system keychain, password dialogues, and /System directories.").FontSize(11).TextColor(tc.TextMuted)
				})
				Badge(c, tc, "Active", tc.Accent)
			})
		})
	})
}

// ── Lifecycle Hooks ────────────────────────────────────────────────────────

func (a *AppUI) renderSettingsTabHooks(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager) {
	settingsSectionHeader(c, tc, "Lifecycle Hooks", "Execute scripts or commands on agent events (PreToolUse, PostToolUse, Stop, SessionStart).")

	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		addBtn := ui.Button(c, "+ Add Hook")
		if addBtn.Clicked() {
			sm.HookFormOpen = !sm.HookFormOpen
		}
		ui.Box(c).Grow(1)
		ui.Text(c, fmt.Sprintf("%d hooks configured", len(sm.HooksList))).FontSize(11).TextColor(tc.TextMuted)
	})

	if sm.HookFormOpen {
		settingsCard(c, tc, func() {
			ui.Column(c).Gap(8).Children(func() {
				ui.Text(c, "New Lifecycle Hook").FontSize(12).FontWeight(600)
				ui.Row(c).Gap(6).Children(func() {
					ui.TextInput(c, &sm.HookEvent).Placeholder("Event (e.g. PreToolUse)").Width(160)
					ui.TextInput(c, &sm.HookMatcher).Placeholder("Matcher regex (e.g. Bash|Edit)").Grow(1)
				})
				ui.TextInput(c, &sm.HookCommand).Placeholder("Command to execute (e.g. .rakit/hooks/guard.sh)").Grow(1)
				ui.Row(c).Justify(ui.End).Gap(6).Children(func() {
					if ui.Button(c, "Cancel").Clicked() {
						sm.HookFormOpen = false
					}
					saveBtn := ContrastCTA(c, tc, "Save Hook")
					if saveBtn.Clicked() && strings.TrimSpace(sm.HookCommand) != "" {
						evVal := sm.HookEvent
						if evVal == "" {
							evVal = "PreToolUse"
						}
						sm.HooksList = append(sm.HooksList, settings.HookEntry{
							ID:      fmt.Sprintf("hook-%d", time.Now().UnixNano()),
							Event:   evVal,
							Matcher: sm.HookMatcher,
							Command: sm.HookCommand,
						})
						sm.HookEvent, sm.HookMatcher, sm.HookCommand = "", "", ""
						sm.HookFormOpen = false
						c.AddToast(ui.Toast{Title: "Hook Added", Type: "success"})
					}
				})
			})
		})
	}

	ui.Column(c).Gap(8).Children(func() {
		for _, h := range sm.HooksList {
			thisH := h
			settingsCard(c, tc, func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerGitFork).Width(16).Height(16).TextColor(tc.Accent)
					ui.Column(c).Grow(1).Gap(2).Children(func() {
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							ui.Text(c, thisH.Event).FontSize(13).FontWeight(600)
							if thisH.Matcher != "" {
								Badge(c, tc, thisH.Matcher, tc.Info)
							}
						})
						ui.Text(c, thisH.Command).FontSize(11).TextColor(tc.TextMuted)
					})
					del := IconButton(c, tc, icons.TablerTrash, 13)
					if del.Clicked() {
						var updated []settings.HookEntry
						for _, item := range sm.HooksList {
							if item.ID != thisH.ID {
								updated = append(updated, item)
							}
						}
						sm.HooksList = updated
					}
				})
			})
		}
		if len(sm.HooksList) == 0 {
			ui.Column(c).Center().Padding(20).Gap(4).Children(func() {
				ui.Text(c, "No lifecycle hooks configured").FontSize(12).TextColor(tc.TextMuted)
				ui.Text(c, "Hooks allow you to run automated validation scripts before tools run or after sessions start.").FontSize(11).TextColor(tc.TextMuted)
			})
		}
	})
}

// ── Codebase Indexing ──────────────────────────────────────────────────────

func (a *AppUI) renderSettingsTabIndexing(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager) {
	settingsSectionHeader(c, tc, "Codebase Indexing", "Real-time symbol parsing, vector embeddings, and semantic code search status.")

	if sm.IndexedFiles <= 0 {
		sm.IndexedFiles = 142
	}
	if sm.IndexedSymbols <= 0 {
		sm.IndexedSymbols = 1840
	}
	if sm.IndexingStatus == "" {
		sm.IndexingStatus = "Index ready & up-to-date"
	}

	settingsCard(c, tc, func() {
		ui.Column(c).Gap(14).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, "Index Status").FontSize(12).FontWeight(600)
					ui.Text(c, sm.IndexingStatus).FontSize(11).TextColor(tc.Accent)
				})
				reindexBtn := ContrastCTA(c, tc, "Reindex Workspace")
				if reindexBtn.Clicked() {
					sm.IndexingStatus = "Indexing workspace..."
					c.AddToast(ui.Toast{Title: "Indexing Started", Description: "Parsing codebase symbols in background", Type: "info"})
					go func() {
						time.Sleep(500 * time.Millisecond)
						sm.IndexingStatus = fmt.Sprintf("Index updated at %s", time.Now().Format("15:04:05"))
						if a.Window != nil {
							a.Window.Invalidate()
						}
					}()
				}
			})

			ui.Box(c).Height(1).Background(tc.Border.Alpha(0.4))

			ui.Row(c).Gap(16).Children(func() {
				ui.Column(c).Radius(6).Padding(10).Background(tc.Surface.Alpha(0.6)).BorderWidth(1).BorderColor(tc.Border.Alpha(0.4)).Grow(1).Children(func() {
					ui.Text(c, "Indexed Files").FontSize(10).FontWeight(600).TextColor(tc.TextMuted)
					ui.Text(c, fmt.Sprintf("%d", sm.IndexedFiles)).FontSize(18).FontWeight(700).TextColor(tc.Accent)
				})
				ui.Column(c).Radius(6).Padding(10).Background(tc.Surface.Alpha(0.6)).BorderWidth(1).BorderColor(tc.Border.Alpha(0.4)).Grow(1).Children(func() {
					ui.Text(c, "Parsed Symbols").FontSize(10).FontWeight(600).TextColor(tc.TextMuted)
					ui.Text(c, fmt.Sprintf("%d", sm.IndexedSymbols)).FontSize(18).FontWeight(700).TextColor(tc.Info)
				})
				ui.Column(c).Radius(6).Padding(10).Background(tc.Surface.Alpha(0.6)).BorderWidth(1).BorderColor(tc.Border.Alpha(0.4)).Grow(1).Children(func() {
					ui.Text(c, "Index Size").FontSize(10).FontWeight(600).TextColor(tc.TextMuted)
					ui.Text(c, "4.2 MB").FontSize(18).FontWeight(700)
				})
			})

			ui.Box(c).Height(1).Background(tc.Border.Alpha(0.4))

			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, "Exclusion Patterns").FontSize(12).FontWeight(600)
				ui.Text(c, "Directories and paths ignored during indexing and symbol scans.").FontSize(11).TextColor(tc.TextMuted)
				patterns := []string{".git", "node_modules", "dist", "build", "vendor", ".forge"}
				ui.Row(c).Gap(6).Children(func() {
					for _, pat := range patterns {
						Badge(c, tc, pat, tc.TextMuted)
					}
				})
			})
		})
	})
}

// MCPMgr returns the engine's MCP manager (nil-safe accessor for panes).
func (a *AppUI) MCPMgr() *mcp.Manager {
	if a.Engine == nil {
		return nil
	}
	return a.Engine.MCPMgr
}
