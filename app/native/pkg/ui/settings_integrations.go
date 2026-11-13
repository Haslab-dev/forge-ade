package ui

// settings_integrations.go replaces the mock MCP/Skills settings tabs with
// the real RakitKode managers and adds the Plugins, Memory, Automations
// (bots) and Sub-Agents & Roles sections of the webview SettingsScreen.

import (
	"fmt"
	"strings"
	"time"

	"github.com/Haslab-dev/RakitKode/mcp"
	"github.com/Haslab-dev/RakitKode/memory"
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
		ui.Box(c).Grow(1)
		if ui.Button(c, "Reload Skills").Clicked() {
			skillMgr.Reload()
			c.AddToast(ui.Toast{Title: "Skills Reloaded", Type: "success"})
		}
		ui.Text(c, fmt.Sprintf("%d skills", len(list))).FontSize(11).TextColor(tc.TextMuted)
	})

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

// MCPMgr returns the engine's MCP manager (nil-safe accessor for panes).
func (a *AppUI) MCPMgr() *mcp.Manager {
	if a.Engine == nil {
		return nil
	}
	return a.Engine.MCPMgr
}
