package ui

// settings_integrations.go replaces the mock MCP/Skills settings tabs with
// the real RakitKode managers and adds the Plugins, Memory, Automations
// (bots) and Sub-Agents & Roles sections of the webview SettingsScreen.

import (
	"fmt"
	"sort"
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
	sort.Slice(servers, func(i, j int) bool {
		return strings.ToLower(servers[i].Name) < strings.ToLower(servers[j].Name)
	})

	// Add-server form (collapsible) and Refresh All.
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		addLbl := "Add Server"
		addIc := icons.TablerPlus
		if sm.MCPFormOpen {
			addLbl = "Close Form"
			addIc = icons.TablerX
		}
		if settingsActionPill(c, tc, "mcp-add-btn", addLbl, addIc, !sm.MCPFormOpen).Clicked() {
			sm.MCPFormOpen = !sm.MCPFormOpen
		}

		if settingsActionPill(c, tc, "mcp-sync-all", "Refresh All", icons.TablerRotate, false).Clicked() {
			c.AddToast(ui.Toast{Title: "Connecting MCP", Description: "Refreshing all enabled servers...", Type: "info"})
			go func() {
				if a.Engine != nil {
					_ = a.Engine.SyncMCP(nil)
					a.queueToast(ui.Toast{Title: "MCP Synced", Description: "All MCP tools updated", Type: "success"})
				}
			}()
		}

		ui.Box(c).Grow(1)
		ui.Text(c, fmt.Sprintf("%d servers · %d tools", len(servers), len(mgr.ListTools()))).FontSize(11).TextColor(tc.TextMuted)
	})

	if sm.MCPFormOpen {
		settingsCard(c, tc, func() {
			ui.Column(c).Gap(10).Children(func() {
				ui.Text(c, "New MCP Server").FontSize(12.5).FontWeight(600).TextColor(tc.Text)
				ui.Row(c).Gap(8).Children(func() {
					ui.TextInput(c, &sm.MCPName).Placeholder("Server name (e.g. filesystem)").Grow(1)
					ui.TextInput(c, &sm.MCPCommand).Placeholder("Command (e.g. npx -y @modelcontextprotocol/server-fs)").Grow(2)
				})
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "Transport:").FontSize(11).TextColor(tc.TextMuted)
					ui.Select(c, &sm.MCPType, []string{"stdio", "http", "sse"})
					ui.Box(c).Grow(1)
					if ui.Button(c, "Cancel").Clicked() {
						sm.MCPFormOpen = false
					}
					if ContrastCTA(c, tc, "Save Server").Clicked() && strings.TrimSpace(sm.MCPName) != "" {
						cfg := mcp.ServerConfig{
							Name:    strings.TrimSpace(sm.MCPName),
							Command: strings.TrimSpace(sm.MCPCommand),
							Type:    sm.MCPType,
							Enabled: true,
						}
						if _, err := mgr.SaveServer(cfg); err != nil {
							c.AddToast(ui.Toast{Title: "Save Failed", Description: err.Error(), Type: "error"})
						} else {
							sm.MCPName, sm.MCPCommand = "", ""
							sm.MCPFormOpen = false
							c.AddToast(ui.Toast{Title: "Server Saved", Description: cfg.Name + " (connecting...)", Type: "success"})
							if a.Engine != nil {
								go a.Engine.SyncMCP(nil)
							}
						}
					}
				})
			})
		})
	}

	ui.Column(c).Gap(8).Children(func() {
		for _, s := range servers {
			thisS := s
			serverTools := 0
			for _, t := range mgr.ListConnectedTools() {
				if t.ServerName == thisS.Name {
					serverTools++
				}
			}
			connErr := mgr.ServerConnectionError(thisS.Name)

			settingsCard(c, tc, func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerPlug).Width(18).Height(18).TextColor(tc.Accent)
					ui.Column(c).Grow(1).Gap(2).Children(func() {
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							ui.Text(c, thisS.Name).FontSize(13).FontWeight(600)
							if serverTools > 0 {
								Badge(c, tc, fmt.Sprintf("CONNECTED (%d tools)", serverTools), tc.Success)
							} else if connErr != nil {
								Badge(c, tc, "ERROR", tc.Error)
							} else if thisS.Enabled {
								Badge(c, tc, "ENABLED", tc.Info)
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
						if connErr != nil {
							ui.Text(c, connErr.Error()).FontSize(11).TextColor(tc.Error).SingleLine()
						}
					})

					// Reconnect button
					recBtn := IconButton(c, tc, icons.TablerRotate, 13)
					if recBtn.Clicked() {
						c.AddToast(ui.Toast{Title: "Connecting MCP", Description: fmt.Sprintf("Reconnecting %s...", thisS.Name), Type: "info"})
						go func(name string) {
							if a.Engine != nil {
								_ = a.Engine.SyncMCP(nil)
								a.queueToast(ui.Toast{Title: "MCP Synced", Description: fmt.Sprintf("Refreshed tools for %s", name), Type: "success"})
							}
						}(thisS.Name)
					}

					del := IconButton(c, tc, icons.TablerTrash, 13)
					confirmDeleteServer := func() {
						// queueToast, not c.AddToast: the staged action runs in a
						// later frame's context.
						a.confirmDialogLabeled("Delete MCP Server?", fmt.Sprintf("“%s” will be removed and its tools will no longer be offered to the agent.", thisS.Name), "Delete", func() {
							if err := mgr.DeleteServer(thisS.Name); err != nil {
								a.queueToast(ui.Toast{Title: "Delete Failed", Description: err.Error(), Type: "error"})
							} else {
								if a.Engine != nil && a.Engine.ToolReg != nil {
									a.Engine.ToolReg.UnregisterMCPTools(thisS.Name)
								}
								a.queueToast(ui.Toast{Title: "Server Deleted", Description: thisS.Name, Type: "info"})
							}
						}, nil)
					}
					del.ContextMenu(func(m *ui.Menu) {
						if m.Item("Delete server").Danger().Chosen() {
							confirmDeleteServer()
						}
					})
					if del.Clicked() {
						confirmDeleteServer()
					}
				})
			})
		}
		if len(servers) == 0 {
			settingsEmptyState(c, tc, icons.TablerPlug, "No MCP servers configured", "Click Add Server above to connect external tools and data sources.")
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
	sort.Slice(list, func(i, j int) bool {
		return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
	})

	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		addLbl := "Create Skill"
		addIc := icons.TablerPlus
		if sm.SkillFormOpen {
			addLbl = "Close Form"
			addIc = icons.TablerX
		}
		if settingsActionPill(c, tc, "skill-add-btn", addLbl, addIc, !sm.SkillFormOpen).Clicked() {
			sm.SkillFormOpen = !sm.SkillFormOpen
		}
		if settingsActionPill(c, tc, "skill-reload-btn", "Reload Skills", icons.TablerRefresh, false).Clicked() {
			skillMgr.Reload()
			c.AddToast(ui.Toast{Title: "Skills Reloaded", Type: "success"})
		}
		ui.Box(c).Grow(1)
		ui.Text(c, fmt.Sprintf("%d skills", len(list))).FontSize(11).TextColor(tc.TextMuted)
	})

	if sm.SkillFormOpen {
		if sm.SkillScope == "" {
			sm.SkillScope = "workspace"
		}
		settingsCard(c, tc, func() {
			ui.Column(c).Gap(10).Children(func() {
				ui.Text(c, "Create New Skill").FontSize(12.5).FontWeight(600).TextColor(tc.Text)
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.TextInput(c, &sm.SkillName).Placeholder("Skill name (e.g. explain-code)").Grow(1)
					ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "Scope:").FontSize(11).TextColor(tc.TextMuted)
						for _, sc := range []string{"workspace", "global"} {
							s := sc
							if renderOrchOptionPill(c, tc, "skill-scope-"+s, s, strings.EqualFold(sm.SkillScope, s)).Clicked() {
								sm.SkillScope = s
							}
						}
					})
				})
				ui.TextInput(c, &sm.SkillDescription).Placeholder("Short description").Grow(1)
				ui.TextInput(c, &sm.SkillPrompt).Placeholder("Instruction prompt / body").Grow(1)
				ui.Row(c).Justify(ui.End).Gap(8).Children(func() {
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
						a.confirmDialogLabeled("Delete Skill?", fmt.Sprintf("“%s” will be removed from the agent's available skills.", thisSk.Name), "Delete", func() {
							if err := skillMgr.DeleteSkill(thisSk.Name); err != nil {
								a.queueToast(ui.Toast{Title: "Delete Failed", Description: err.Error(), Type: "error"})
								return
							}
							skillMgr.Reload()
							a.queueToast(ui.Toast{Title: "Skill Deleted", Description: thisSk.Name, Type: "info"})
						}, nil)
					}
				})
			})
		}
		if len(list) == 0 {
			settingsEmptyState(c, tc, icons.TablerWand, "No skills installed", "Create a workspace or global skill to teach the agent reusable procedures.")
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
	sort.Slice(list, func(i, j int) bool {
		ni, nj := strings.ToLower(list[i].Name), strings.ToLower(list[j].Name)
		if ni != nj {
			return ni < nj
		}
		return list[i].ID < list[j].ID
	})

	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if settingsActionPill(c, tc, "plugins-reload-btn", "Reload Plugins", icons.TablerRefresh, false).Clicked() {
			plugMgr.Reload()
			c.AddToast(ui.Toast{Title: "Plugins Reloaded", Type: "success"})
		}
		ui.Box(c).Grow(1)
		ui.Text(c, fmt.Sprintf("%d plugins", len(list))).FontSize(11).TextColor(tc.TextMuted)
	})

	ui.Column(c).Gap(8).Children(func() {
		for _, p := range list {
			thisP := p
			ui.Column(c.Key("plugin-card-" + thisP.ID)).Children(func() {
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
			})
		}
		if len(list) == 0 {
			settingsEmptyState(c, tc, icons.TablerPuzzle, "No plugins installed", "Drop plugin bundles into the plugins directory and click Reload Plugins.")
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
	sort.Slice(list, func(i, j int) bool {
		if !list[i].UpdatedAt.Equal(list[j].UpdatedAt) {
			return list[i].UpdatedAt.After(list[j].UpdatedAt)
		}
		if list[i].Key != list[j].Key {
			return list[i].Key < list[j].Key
		}
		return list[i].ID < list[j].ID
	})

	settingsCard(c, tc, func() {
		ui.Column(c).Gap(8).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Text(c, "Add Persistent Memory").FontSize(12.5).FontWeight(600).TextColor(tc.Text)
				ui.Box(c).Grow(1)
				ui.Text(c, fmt.Sprintf("%d saved", len(list))).FontSize(11).TextColor(tc.TextMuted)
				if settingsActionPill(c, tc, "mem-reload-btn", "Reload", icons.TablerRefresh, false).Clicked() {
					memMgr.Reload()
					c.AddToast(ui.Toast{Title: "Memory Reloaded", Type: "success"})
				}
			})
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.TextInput(c, &sm.MemoryNew).Placeholder("Write a preference, rule, or project fact for the agent to remember…").Grow(1)
				if ContrastCTA(c, tc, "Save Memory").Clicked() && strings.TrimSpace(sm.MemoryNew) != "" {
					content := sm.MemoryNew
					if err := memMgr.Save(memory.Entry{Key: fmt.Sprintf("note-%d", time.Now().UnixMilli()), Content: content, Scope: "global"}); err != nil {
						c.AddToast(ui.Toast{Title: "Save Failed", Description: err.Error(), Type: "error"})
					} else {
						sm.MemoryNew = ""
						c.AddToast(ui.Toast{Title: "Memory Saved", Type: "success"})
					}
				}
			})
		})
	})

	ui.Column(c).Gap(8).Children(func() {
		for _, e := range list {
			thisE := e
			ui.Column(c.Key("mem-card-" + thisE.ID)).Children(func() {
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
							a.confirmDialogLabeled("Delete Memory?", fmt.Sprintf("“%s” will no longer be recalled by the agent.", truncatePreview(thisE.Content, 60)), "Delete", func() {
								if err := memMgr.Delete(thisE.ID); err != nil {
									a.queueToast(ui.Toast{Title: "Delete Failed", Description: err.Error(), Type: "error"})
								}
							}, nil)
						}
					})
				})
			})
		}
		if len(list) == 0 {
			settingsEmptyState(c, tc, icons.TablerBrain, "No memories saved yet", "Add persistent notes above so the agent recalls your conventions across sessions.")
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

	// Action bar + collapsible create form.
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		addLbl := "New Automation"
		addIc := icons.TablerPlus
		if sm.AutoFormOpen {
			addLbl = "Close Form"
			addIc = icons.TablerX
		}
		if settingsActionPill(c, tc, "auto-add-btn", addLbl, addIc, !sm.AutoFormOpen).Clicked() {
			sm.AutoFormOpen = !sm.AutoFormOpen
		}
		ui.Box(c).Grow(1)
		ui.Text(c, fmt.Sprintf("%d automations", len(list))).FontSize(11).TextColor(tc.TextMuted)
	})

	if sm.AutoFormOpen {
		settingsCard(c, tc, func() {
			ui.Column(c).Gap(10).Children(func() {
				ui.Text(c, "New Automation").FontSize(12.5).FontWeight(600).TextColor(tc.Text)
				ui.TextInput(c, &sm.AutoName).Placeholder("Automation name (e.g. Daily PR Digest)").Grow(1)
				ui.Row(c).Gap(8).Children(func() {
					ui.TextInput(c, &sm.AutoCron).Placeholder("Cron (e.g. 0 9 * * 1-5, empty = manual)").Grow(1)
					ui.TextInput(c, &sm.AutoPrompt).Placeholder("Prompt to run").Grow(2)
				})
				ui.Row(c).Gap(6).AlignItems(ui.Center).Wrap().Children(func() {
					ui.Text(c, "Presets:").FontSize(11).TextColor(tc.TextMuted)
					cronPresets := []struct {
						label string
						expr  string
					}{
						{"Hourly", "0 * * * *"},
						{"Daily 9AM", "0 9 * * *"},
						{"Weekdays", "0 9 * * 1-5"},
						{"Weekly Mon", "0 9 * * 1"},
						{"Monthly", "0 9 1 * *"},
						{"Manual", ""},
					}
					for _, p := range cronPresets {
						preset := p
						if renderOrchOptionPill(c, tc, "cron-pre-"+preset.label, preset.label, strings.TrimSpace(sm.AutoCron) == preset.expr).Clicked() {
							sm.AutoCron = preset.expr
						}
					}
				})
				ui.Row(c).Justify(ui.End).Gap(8).Children(func() {
					if ui.Button(c, "Cancel").Clicked() {
						sm.AutoFormOpen = false
					}
					if ContrastCTA(c, tc, "Create Automation").Clicked() && strings.TrimSpace(sm.AutoName) != "" {
						_, err := autoMgr.Save(automations.Automation{
							Name:      strings.TrimSpace(sm.AutoName),
							Prompt:    sm.AutoPrompt,
							CronExpr:  strings.TrimSpace(sm.AutoCron),
							Enabled:   strings.TrimSpace(sm.AutoCron) != "",
							Workspace: a.Workspace.RootDir,
						})
						if err != nil {
							c.AddToast(ui.Toast{Title: "Create Failed", Description: err.Error(), Type: "error"})
						} else {
							sm.AutoName, sm.AutoCron, sm.AutoPrompt = "", "", ""
							sm.AutoFormOpen = false
							c.AddToast(ui.Toast{Title: "Automation Created", Type: "success"})
						}
					}
				})
			})
		})
	}

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
						a.confirmDialogLabeled("Delete Automation?", fmt.Sprintf("“%s” will stop running and its history is removed.", thisAu.Name), "Delete", func() {
							if err := autoMgr.Delete(thisAu.ID); err != nil {
								a.queueToast(ui.Toast{Title: "Delete Failed", Description: err.Error(), Type: "error"})
							}
						}, nil)
					}
				})
			})
		}
		if len(list) == 0 {
			settingsEmptyState(c, tc, icons.TablerBot, "No automations configured", "Create a scheduled cron prompt or manual bot trigger to automate recurring workflows.")
		}
	})
}

// ── Orchestration & Sub-Agent Personas ─────────────────────────────────────

func renderOrchOptionPill(c *ui.Context, tc theme.ThemeColors, key, label string, active bool) ui.Element {
	bg := tc.Surface
	fg := tc.TextMuted
	bdr := tc.Border.Alpha(0.7)
	fw := 500
	if active {
		bg = tc.Accent.Alpha(0.14)
		fg = tc.Accent
		bdr = tc.Accent.Alpha(0.45)
		fw = 600
	}
	pill := ui.Row(c.Key(key)).Height(24).Radius(6).Padding(0, 9).AlignItems(ui.Center).
		Background(bg).BorderWidth(1).BorderColor(bdr).Cursor(ui.CursorPointer)
	pill.Children(func() {
		ui.Text(c, label).FontSize(11).FontWeight(fw).TextColor(fg)
	})
	if !active && pill.Hovered() {
		pill.Background(tc.Text.Alpha(0.05))
	}
	return pill
}

func clearRoleForm(sm *settings.SettingsManager) {
	sm.RoleEditingID = ""
	sm.RoleName = ""
	sm.RoleDesc = ""
	sm.RoleFilter = ""
	sm.RoleModel = ""
	sm.RolePrompt = ""
	sm.RoleRules = ""
	sm.RoleReadOnly = false
	sm.RoleAllowBash = false
	sm.RoleAgentID = ""
	sm.RoleAgentType = ""
}

func (a *AppUI) renderSettingsTabSubagents(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager) {
	settingsSectionHeader(c, tc, "Agent Personas", "Configure the lead orchestrator, model tiering, concurrency, and specialist personas.")

	if a.Engine == nil || a.Engine.AgentMgr == nil {
		ui.Text(c, "Agent engine unavailable").FontSize(12).TextColor(tc.TextMuted)
		return
	}

	orchCfg := a.Engine.AgentMgr.GetOrchestrationConfig()
	saveOrch := func(next rakitagent.OrchestrationConfig) {
		if err := a.Engine.AgentMgr.SaveOrchestrationConfig(next); err != nil {
			c.AddToast(ui.Toast{Title: "Failed to save orchestration config", Description: err.Error(), Type: "error"})
			return
		}
		orchCfg = next
	}

	// 1. Orchestrator Harness Card
	settingsCard(c, tc, func() {
		ui.Column(c).Gap(10).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons.TablerCpu).Width(15).Height(15).TextColor(tc.Accent)
				ui.Column(c).Grow(1).Gap(1).Children(func() {
					ui.Text(c, "Multi-Agent Orchestrator Harness").FontSize(13).FontWeight(600)
					ui.Text(c, "Lead orchestrator delegates specialized work across planning, research, coding, review, and QA personas.").FontSize(11).TextColor(tc.TextMuted)
				})
				if orchCfg.Enabled {
					Badge(c, tc, "Active", tc.Success)
				} else {
					Badge(c, tc, "Direct Mode", tc.TextMuted)
				}
			})

			ui.Row(c).Gap(16).AlignItems(ui.Center).Wrap().Children(func() {
				enabled := orchCfg.Enabled
				if ui.Checkbox(c, &enabled, "Enable Multi-Agent Orchestration").FontSize(12).Clicked() {
					orchCfg.Enabled = enabled
					saveOrch(orchCfg)
				}
				autoRoute := orchCfg.AutoRoute
				if ui.Checkbox(c, &autoRoute, "Auto-Route Tasks to Sub-Agents").FontSize(12).Clicked() {
					orchCfg.AutoRoute = autoRoute
					saveOrch(orchCfg)
				}
				parallel := orchCfg.ParallelExploration
				if ui.Checkbox(c, &parallel, "Parallel Exploration").FontSize(12).Clicked() {
					orchCfg.ParallelExploration = parallel
					saveOrch(orchCfg)
				}
			})

			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, "Default Lead Agent Role").FontSize(11).FontWeight(600).TextColor(tc.TextMuted)
				leadRoles := []struct {
					label string
					role  rakitagent.RoleFilter
				}{
					{"Orchestrator", rakitagent.RoleOrchestrator},
					{"Coding", rakitagent.RoleCoding},
					{"Planning", rakitagent.RolePlanning},
					{"Research", rakitagent.RoleResearch},
					{"Reviewer", rakitagent.RoleReviewer},
					{"Testing", rakitagent.RoleTesting},
				}
				ui.Row(c).Gap(6).AlignItems(ui.Center).Wrap().Children(func() {
					for _, lr := range leadRoles {
						active := orchCfg.DefaultRole == lr.role || (orchCfg.DefaultRole == "" && lr.role == rakitagent.RoleOrchestrator)
						if renderOrchOptionPill(c, tc, "lead-"+string(lr.role), lr.label, active).Clicked() {
							orchCfg.DefaultRole = lr.role
							saveOrch(orchCfg)
						}
					}
				})
			})

			ui.Row(c).Gap(18).AlignItems(ui.Center).Wrap().Children(func() {
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "Sub-Agent Model Tiering").FontSize(11).FontWeight(600).TextColor(tc.TextMuted)
					ui.Row(c).Gap(5).Children(func() {
						tiers := []string{"auto", "weak", "strong", "inherit"}
						for _, t := range tiers {
							cur := strings.ToLower(strings.TrimSpace(orchCfg.DefaultSubagentTier))
							if cur == "" {
								cur = "auto"
							}
							if renderOrchOptionPill(c, tc, "tier-"+t, strings.ToUpper(t[:1])+t[1:], cur == t).Clicked() {
								orchCfg.DefaultSubagentTier = t
								saveOrch(orchCfg)
							}
						}
					})
				})

				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "Max Concurrent").FontSize(11).FontWeight(600).TextColor(tc.TextMuted)
					ui.Row(c).Gap(5).Children(func() {
						for _, n := range []int{1, 2, 4, 6, 8} {
							if renderOrchOptionPill(c, tc, fmt.Sprintf("conc-%d", n), fmt.Sprintf("%d", n), orchCfg.MaxConcurrent == n).Clicked() {
								orchCfg.MaxConcurrent = n
								saveOrch(orchCfg)
							}
						}
					})
				})

				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "Max Depth").FontSize(11).FontWeight(600).TextColor(tc.TextMuted)
					ui.Row(c).Gap(5).Children(func() {
						for _, d := range []int{1, 2, 3, 4, 5} {
							if renderOrchOptionPill(c, tc, fmt.Sprintf("depth-%d", d), fmt.Sprintf("%d", d), orchCfg.MaxDepth == d).Clicked() {
								orchCfg.MaxDepth = d
								saveOrch(orchCfg)
							}
						}
					})
				})
			})
		})
	})

	defs := a.Engine.AgentMgr.ListAgentDefinitions()

	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		addLabel := "Create Persona"
		addIc := icons.TablerPlus
		if sm.RoleFormOpen {
			addLabel = "Close Form"
			addIc = icons.TablerX
		}
		if settingsActionPill(c, tc, "role-add-btn", addLabel, addIc, !sm.RoleFormOpen).Clicked() {
			if sm.RoleFormOpen {
				clearRoleForm(sm)
				sm.RoleFormOpen = false
			} else {
				clearRoleForm(sm)
				sm.RoleFormOpen = true
			}
		}
		ui.Box(c).Grow(1)
		ui.Text(c, fmt.Sprintf("%d personas configured", len(defs))).FontSize(11).TextColor(tc.TextMuted)
	})

	// 2. Create / Edit Persona Form
	if sm.RoleFormOpen {
		settingsCard(c, tc, func() {
			ui.Column(c).Gap(8).Children(func() {
				formTitle := "Create Sub-Agent Persona"
				saveLabel := "Save Persona"
				if sm.RoleEditingID != "" {
					formTitle = "Edit Sub-Agent Persona"
					saveLabel = "Save Changes"
				}
				ui.Row(c).AlignItems(ui.Center).Children(func() {
					ui.Text(c, formTitle).FontSize(12).FontWeight(600)
					if sm.RoleEditingID != "" {
						ui.Box(c).Width(8)
						Badge(c, tc, sm.RoleEditingID, tc.Info)
					}
				})

				// Quick Role Presets
				presets := []struct {
					label     string
					role      string
					desc      string
					prompt    string
					readOnly  bool
					allowBash bool
				}{
					{"Orchestrator", "orchestrator", "Coordinates specialized sub-agents and synthesizes final results", "Decompose complex requests, delegate exploration/planning/coding/review/testing to specialists, and verify outcomes before responding.", false, false},
					{"Planner", "planning", "Designs implementation plans, architecture, and verification steps", "Analyze requirements and architecture, then produce an ordered step-by-step plan without modifying files.", true, false},
					{"Researcher", "research", "Explores codebases, traces call paths, and gathers facts quickly", "Perform fast read-only codebase research and report exact file paths, symbols, and behavior.", true, false},
					{"Coder", "coding", "Implements focused code changes cleanly and runs targeted tests", "Implement the scoped code changes following repository conventions and verify with relevant tests.", false, true},
					{"Reviewer", "reviewer", "Audits diffs for correctness, regressions, edge cases, and security", "Review changes critically for bugs, race conditions, security flaws, and missing test coverage.", true, true},
					{"QA & Test", "testing", "Writes and executes verification suites and reproduction scripts", "Design and execute tests and verification commands to validate behavior end-to-end.", false, true},
				}
				ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "Presets:").FontSize(11).TextColor(tc.TextMuted)
					for _, p := range presets {
						p := p
						if renderOrchOptionPill(c, tc, "preset-"+p.role, p.label, strings.EqualFold(strings.TrimSpace(sm.RoleFilter), p.role)).Clicked() {
							sm.RoleFilter = p.role
							if strings.TrimSpace(sm.RoleName) == "" {
								sm.RoleName = p.label + " Agent"
							}
							sm.RoleDesc = p.desc
							sm.RolePrompt = p.prompt
							sm.RoleReadOnly = p.readOnly
							sm.RoleAllowBash = p.allowBash
						}
					}
				})

				ui.Row(c).Gap(8).Children(func() {
					ui.Column(c).Gap(3).Grow(1).Children(func() {
						ui.Text(c, "Persona Name").FontSize(10.5).FontWeight(600).TextColor(tc.TextMuted)
						ui.TextInput(c, &sm.RoleName).Placeholder("e.g. Security Auditor").FillWidth()
					})
					ui.Column(c).Gap(3).Width(200).Children(func() {
						ui.Text(c, "Role").FontSize(10.5).FontWeight(600).TextColor(tc.TextMuted)
						ui.TextInput(c, &sm.RoleFilter).Placeholder("orchestrator/planning/coding/reviewer").FillWidth()
					})
				})
				ui.Column(c).Gap(3).Children(func() {
					ui.Text(c, "Description").FontSize(10.5).FontWeight(600).TextColor(tc.TextMuted)
					ui.TextInput(c, &sm.RoleDesc).Placeholder("Short description of what this persona specializes in").FillWidth()
				})
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "Default Harness").FontSize(10.5).FontWeight(600).TextColor(tc.TextMuted)
					ui.Row(c).Gap(5).AlignItems(ui.Center).Wrap().Children(func() {
						type harnessOpt struct {
							id, label, typ string
						}
						hOpts := []harnessOpt{
							{"", "Auto (Inherit)", ""},
							{"agent-internal", "Internal", "internal"},
						}
						if a.ACPMgr != nil {
							for _, ag := range a.ACPMgr.Agents {
								if ag.ID == "agent-internal" || !ag.Available {
									continue
								}
								hOpts = append(hOpts, harnessOpt{ag.ID, shortHarnessLabel(ag.Name), "acp"})
							}
						}
						for _, ho := range hOpts {
							opt := ho
							if renderOrchOptionPill(c, tc, "form-harness-"+opt.id, opt.label, sm.RoleAgentID == opt.id).Clicked() {
								sm.RoleAgentID = opt.id
								sm.RoleAgentType = opt.typ
							}
						}
					})
				})
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "Default Model (auto-loaded from selected harness)").FontSize(10.5).FontWeight(600).TextColor(tc.TextMuted)
					ui.Row(c).Gap(5).AlignItems(ui.Center).Wrap().Children(func() {
						tiers := []string{"", "auto", "weak", "strong"}
						for _, tier := range tiers {
							t := tier
							label := t
							if label == "" {
								label = "Inherit"
							}
							active := strings.EqualFold(strings.TrimSpace(sm.RoleModel), t)
							if renderOrchOptionPill(c, tc, "form-tier-"+label, label, active).Clicked() {
								sm.RoleModel = t
							}
						}
						for _, mID := range a.modelsForHarness(sm.RoleAgentID) {
							m := mID
							active := strings.TrimSpace(sm.RoleModel) == m
							if renderOrchOptionPill(c, tc, "form-model-"+m, m, active).Clicked() {
								sm.RoleModel = m
							}
						}
					})
				})
				ui.Column(c).Gap(3).Children(func() {
					ui.Text(c, "System Instructions").FontSize(10.5).FontWeight(600).TextColor(tc.TextMuted)
					ui.TextAreaBase(c, &sm.RolePrompt).Lines(2, 4).Placeholder("System instructions / Persona prompt").FillWidth()
				})
				ui.Column(c).Gap(3).Children(func() {
					ui.Text(c, "Rules & Guardrails (optional)").FontSize(10.5).FontWeight(600).TextColor(tc.TextMuted)
					ui.TextInput(c, &sm.RoleRules).Placeholder("Additional rules / constraints").FillWidth()
				})

				ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
					ui.Checkbox(c, &sm.RoleReadOnly, "Read-Only (no filesystem writes)").FontSize(11)
					ui.Checkbox(c, &sm.RoleAllowBash, "Allow Terminal Commands").FontSize(11)
					ui.Box(c).Grow(1)
					if ui.Button(c, "Cancel").Clicked() {
						clearRoleForm(sm)
						sm.RoleFormOpen = false
					}
					saveBtn := ContrastCTA(c, tc, saveLabel)
					if saveBtn.Clicked() && strings.TrimSpace(sm.RoleName) != "" {
						defID := sm.RoleEditingID
						if defID == "" {
							defID = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(sm.RoleName), " ", "-"))
						}
						desc := strings.TrimSpace(sm.RoleDesc)
						if desc == "" {
							desc = strings.TrimSpace(sm.RolePrompt)
						}
						roleVal := rakitagent.RoleFilter(strings.TrimSpace(sm.RoleFilter))
						if roleVal == "" {
							roleVal = rakitagent.RoleCustom
						}
						newDef := rakitagent.AgentDefinition{
							ID:          defID,
							Name:        strings.TrimSpace(sm.RoleName),
							Description: desc,
							Role:        roleVal,
							Model:       strings.TrimSpace(sm.RoleModel),
							Prompt:      sm.RolePrompt,
							Rules:       sm.RoleRules,
							ReadOnly:    sm.RoleReadOnly,
							AllowBash:   sm.RoleAllowBash,
							UpdatedAt:   time.Now(),
						}
						if sm.RoleEditingID != "" {
							if existing, ok := a.Engine.AgentMgr.FindDefinition(sm.RoleEditingID); ok {
								newDef.ID = existing.ID
								newDef.Builtin = existing.Builtin
								newDef.Tools = existing.Tools
								newDef.MCPServers = existing.MCPServers
								newDef.Skills = existing.Skills
								newDef.CreatedAt = existing.CreatedAt
							}
						}
						if newDef.CreatedAt.IsZero() {
							newDef.CreatedAt = time.Now()
						}
						_, err := a.Engine.AgentMgr.SaveAgentDefinition(newDef)
						if err == nil {
							sm.SetPersonaHarness(newDef.ID, sm.RoleAgentID, sm.RoleAgentType)
							toastTitle := "Persona Created"
							if sm.RoleEditingID != "" {
								toastTitle = "Persona Updated"
							}
							clearRoleForm(sm)
							sm.RoleFormOpen = false
							c.AddToast(ui.Toast{Title: toastTitle, Description: newDef.Name, Type: "success"})
						} else {
							c.AddToast(ui.Toast{Title: "Failed to save persona", Description: err.Error(), Type: "error"})
						}
					}
				})
			})
		})
	}

	// 3. Configured Personas List
	ui.Column(c).Gap(8).Children(func() {
		for _, d := range defs {
			thisD := d
			isLead := orchCfg.Enabled && thisD.Role != "" && (thisD.Role == orchCfg.DefaultRole || (orchCfg.DefaultRole == "" && thisD.Role == rakitagent.RoleOrchestrator))
			settingsCard(c, tc, func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					botColor := tc.TextMuted
					if isLead {
						botColor = tc.Accent
					}
					ui.Icon(c, icons.TablerBot).Width(16).Height(16).TextColor(botColor)
					ui.Column(c).Grow(1).Gap(4).Children(func() {
						ui.Row(c).Gap(5).AlignItems(ui.Center).Wrap().Children(func() {
							ui.Text(c, thisD.Name).FontSize(13).FontWeight(600)
							if isLead {
								Badge(c, tc, "Default Lead", tc.Success)
							}
							if thisD.Builtin {
								Badge(c, tc, "builtin", tc.TextMuted)
							}
							if thisD.Role != "" {
								Badge(c, tc, string(thisD.Role), tc.Info)
							}
							if thisD.ReadOnly {
								Badge(c, tc, "read-only", tc.Warning)
							} else {
								Badge(c, tc, "read-write", tc.Success)
							}
							if thisD.AllowBash {
								Badge(c, tc, "bash", tc.Info)
							} else {
								Badge(c, tc, "no bash", tc.TextMuted)
							}
							if thisD.Model != "" {
								Badge(c, tc, thisD.Model, tc.Accent)
							}
							if hb := sm.GetPersonaHarness(thisD.ID); hb.AgentID != "" {
								Badge(c, tc, shortHarnessLabel(hb.AgentID), tc.Info)
							}
						})
						if thisD.Description != "" {
							ui.Text(c, thisD.Description).FontSize(11).TextColor(tc.TextMuted)
						}
					})

					if !isLead && thisD.Role != "" && thisD.Role != rakitagent.RoleCustom {
						if renderOrchOptionPill(c, tc, "setlead-"+thisD.ID, "Set Lead", false).Clicked() {
							orchCfg.Enabled = true
							orchCfg.DefaultRole = thisD.Role
							saveOrch(orchCfg)
							if a.Agent != nil {
								a.Agent.SetSessionPersona(a.Agent.ActiveSessionID, thisD.ID)
							}
							c.AddToast(ui.Toast{Title: "Default Lead Updated", Description: thisD.Name, Type: "success"})
						}
					}

					editBtn := IconButton(c, tc, icons.TablerPencil, 13)
					if editBtn.Clicked() {
						sm.RoleEditingID = thisD.ID
						sm.RoleName = thisD.Name
						sm.RoleDesc = thisD.Description
						sm.RoleFilter = string(thisD.Role)
						sm.RoleModel = thisD.Model
						sm.RolePrompt = thisD.Prompt
						sm.RoleRules = thisD.Rules
						sm.RoleReadOnly = thisD.ReadOnly
						sm.RoleAllowBash = thisD.AllowBash
						hb := sm.GetPersonaHarness(thisD.ID)
						sm.RoleAgentID = hb.AgentID
						sm.RoleAgentType = hb.AgentType
						sm.RoleFormOpen = true
					}

					if thisD.Builtin {
						rst := IconButton(c, tc, icons.TablerRefresh, 13)
						if rst.Clicked() {
							if err := a.Engine.AgentMgr.ResetBuiltinDefinition(thisD.ID); err == nil {
								c.AddToast(ui.Toast{Title: "Builtin Persona Reset", Description: thisD.Name, Type: "info"})
							} else {
								c.AddToast(ui.Toast{Title: "Failed to reset persona", Description: err.Error(), Type: "error"})
							}
						}
					} else {
						del := IconButton(c, tc, icons.TablerTrash, 13)
						if del.Clicked() {
							a.confirmDialogLabeled("Delete Persona?", fmt.Sprintf("“%s” will be removed from the orchestration roster.", thisD.Name), "Delete", func() {
								if err := a.Engine.AgentMgr.DeleteAgentDefinition(thisD.ID); err != nil {
									a.queueToast(ui.Toast{Title: "Delete Failed", Description: err.Error(), Type: "error"})
								} else {
									a.queueToast(ui.Toast{Title: "Persona Deleted", Description: thisD.Name, Type: "info"})
								}
							}, nil)
						}
					}
				})
			})
		}
		if len(defs) == 0 {
			settingsEmptyState(c, tc, icons.TablerUser, "No personas defined", "Create a specialist persona to delegate research, coding, review, or testing.")
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

	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		addLbl := "Add Command"
		addIc := icons.TablerPlus
		if sm.CmdFormOpen {
			addLbl = "Close Form"
			addIc = icons.TablerX
		}
		if settingsActionPill(c, tc, "cmd-add-btn", addLbl, addIc, !sm.CmdFormOpen).Clicked() {
			sm.CmdFormOpen = !sm.CmdFormOpen
		}
		ui.Box(c).Grow(1)
		ui.Text(c, fmt.Sprintf("%d custom · %d built-in", len(sm.CustomCommands), len(defaultCmds))).FontSize(11).TextColor(tc.TextMuted)
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
						updated := append(append([]settings.CustomSlashCommand(nil), sm.CustomCommands...), settings.CustomSlashCommand{
							Name:        name,
							Description: sm.CmdDescription,
							Prompt:      sm.CmdPrompt,
						})
						// Persist through the manager (round-trips to disk) and
						// re-publish palette commands so ⌘K can run it.
						sm.SetCustomCommands(updated)
						if a.Palette != nil {
							a.Palette.SetCommands(a.paletteCommands())
						}
						sm.CmdName, sm.CmdDescription, sm.CmdPrompt = "", "", ""
						sm.CmdFormOpen = false
						c.AddToast(ui.Toast{Title: "Command Added", Description: name, Type: "success"})
					}
				})
			})
		})
	}

	ui.Column(c).Gap(8).Children(func() {
		if len(sm.CustomCommands) > 0 {
			ui.Text(c, "CUSTOM COMMANDS").FontSize(10).FontWeight(700).LetterSpacing(0.6).TextColor(tc.TextMuted).Padding(4, 2, 0, 2)
		}
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
						targetName := cmd.Name
						a.confirmDialogLabeled("Delete Slash Command?", fmt.Sprintf("“%s” will be removed from your custom commands and command palette.", targetName), "Delete", func() {
							var updated []settings.CustomSlashCommand
							for _, it := range sm.CustomCommands {
								if it.Name != targetName {
									updated = append(updated, it)
								}
							}
							sm.SetCustomCommands(updated)
							if a.Palette != nil {
								a.Palette.SetCommands(a.paletteCommands())
							}
							a.queueToast(ui.Toast{Title: "Command Deleted", Description: targetName, Type: "info"})
						}, nil)
					}
				})
			})
		}

		ui.Text(c, "BUILT-IN COMMANDS").FontSize(10).FontWeight(700).LetterSpacing(0.6).TextColor(tc.TextMuted).Padding(6, 2, 0, 2)
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
	settingsSectionHeader(c, tc, "Browser Use", "Built-in browser engine for agent web inspection, DOM snapshots, and screenshots.")

	activeEngine := "Chromium CDP (Standalone)"
	if a.Engine != nil && a.Engine.BrowserUse != nil {
		if a.Engine.BrowserUse.EngineName() == "in-app" {
			activeEngine = "macOS Native WKWebView (Zero-Chrome)"
		}
	}
	settingsNoticeBanner(c, tc, icons.TablerCircleCheck, "Active Engine: "+activeEngine,
		"Agent browser tools (forge_browser_open, snapshot, screenshot) run through this engine automatically via forge-mcp.", tc.Success)

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
				ui.Switch(c, &sm.BrowserHeadless).Disabled(true).DimDisabled(true)
			})

			settingsDivider(c, tc)

			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, "Execute JavaScript in Web Pages").FontSize(12).FontWeight(600)
					ui.Text(c, "Allow web pages and scraped DOM scripts to evaluate dynamically.").FontSize(11).TextColor(tc.TextMuted)
				})
				ui.Switch(c, &sm.BrowserAllowJS).Disabled(true).DimDisabled(true)
			})

			settingsDivider(c, tc)

			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, "Default Search Engine").FontSize(12).FontWeight(600)
				ui.Text(c, "Fallback search provider used for unqualified queries in the URL bar.").FontSize(11).TextColor(tc.TextMuted)
				providers := []string{"DuckDuckGo", "Google", "Bing", "Brave"}
				ui.Row(c).Gap(6).Wrap().Children(func() {
					for _, p := range providers {
						prov := p
						isSel := sm.BrowserEngine == prov
						btn := ui.Row(c.Key("search-eng-"+prov)).Height(24).Radius(6).Padding(0, 10).
							BorderWidth(1).AlignItems(ui.Center)
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
						// Control disabled: no browser automation reads this yet.
					}
				})
			})

			settingsDivider(c, tc)

			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, "Default Viewport Size").FontSize(12).FontWeight(600)
				ui.Text(c, "Emulated browser window dimensions for page layout and screenshots.").FontSize(11).TextColor(tc.TextMuted)
				viewports := []string{"1280x800", "1440x900", "1920x1080", "390x844 (Mobile)"}
				ui.Row(c).Gap(6).Wrap().Children(func() {
					for _, vp := range viewports {
						viewport := vp
						isSel := sm.BrowserViewport == viewport
						btn := ui.Row(c.Key("viewport-"+viewport)).Height(24).Radius(6).Padding(0, 10).
							BorderWidth(1).AlignItems(ui.Center)
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
						// Control disabled: no browser automation reads this yet.
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

	// Honesty: no desktop automation reads these preferences yet.
	settingsNoticeBanner(c, tc, icons.TablerAlertTriangle, "Preview Only — Not Wired Up Yet",
		"Desktop GUI control settings are currently read-only until computer use automation is enabled.", tc.Warning)

	settingsCard(c, tc, func() {
		ui.Column(c).Gap(12).Children(func() {
			ui.Column(c).Gap(6).Children(func() {
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
					// Selection display only — control disabled until computer
					// use is actually wired up.
					card := ui.Row(c.Key("comp-mode-"+mode.name)).Radius(8).Padding(9, 12).Gap(10).
						BorderWidth(1).AlignItems(ui.Center)
					if isSel {
						card.Background(tc.Accent.Alpha(0.1)).BorderColor(tc.Accent)
					} else {
						card.Background(tc.Canvas).BorderColor(tc.Border.Alpha(0.6))
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
							ui.Text(c, mode.desc).FontSize(10.5).TextColor(tc.TextMuted)
						})
					})
				}
			})

			settingsDivider(c, tc)

			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, "Display Resolution & Scaling").FontSize(12).FontWeight(600)
					ui.Text(c, "Display coordinate bridge for desktop automation — reported once computer use is wired up.").FontSize(11).TextColor(tc.TextMuted)
				})
				Badge(c, tc, "Unavailable", tc.TextMuted)
			})

			settingsDivider(c, tc)

			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, "System Safety Sandbox").FontSize(12).FontWeight(600)
					ui.Text(c, "Prevent automation actions on system keychain, password dialogues, and /System directories — enforced once computer use is wired up.").FontSize(11).TextColor(tc.TextMuted)
				})
				Badge(c, tc, "Unavailable", tc.TextMuted)
			})
		})
	})
}

// ── Lifecycle Hooks ────────────────────────────────────────────────────────

func (a *AppUI) renderSettingsTabHooks(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager) {
	settingsSectionHeader(c, tc, "Lifecycle Hooks", "Execute scripts or commands on agent events (PreToolUse, PostToolUse, Stop, SessionStart).")

	// Honesty: hooks are saved and reloaded today, but nothing executes them
	// yet — say so instead of implying they run.
	settingsNoticeBanner(c, tc, icons.TablerInfoCircle, "Saved Locally — Execution Not Wired Yet",
		"Hooks configured here persist to disk and will activate automatically once lifecycle hook execution is enabled.", tc.Warning)

	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		addLbl := "Add Hook"
		addIc := icons.TablerPlus
		if sm.HookFormOpen {
			addLbl = "Close Form"
			addIc = icons.TablerX
		}
		if settingsActionPill(c, tc, "hook-add-btn", addLbl, addIc, !sm.HookFormOpen).Clicked() {
			sm.HookFormOpen = !sm.HookFormOpen
		}
		ui.Box(c).Grow(1)
		ui.Text(c, fmt.Sprintf("%d hooks configured", len(sm.HooksList))).FontSize(11).TextColor(tc.TextMuted)
	})

	if sm.HookFormOpen {
		if sm.HookEvent == "" {
			sm.HookEvent = "PreToolUse"
		}
		settingsCard(c, tc, func() {
			ui.Column(c).Gap(10).Children(func() {
				ui.Text(c, "New Lifecycle Hook").FontSize(12.5).FontWeight(600).TextColor(tc.Text)
				ui.Row(c).Gap(6).AlignItems(ui.Center).Wrap().Children(func() {
					ui.Text(c, "Event:").FontSize(11).TextColor(tc.TextMuted)
					for _, ev := range []string{"PreToolUse", "PostToolUse", "Stop", "SessionStart"} {
						e := ev
						if renderOrchOptionPill(c, tc, "hook-ev-"+e, e, sm.HookEvent == e).Clicked() {
							sm.HookEvent = e
						}
					}
				})
				ui.Row(c).Gap(8).Children(func() {
					ui.TextInput(c, &sm.HookMatcher).Placeholder("Matcher regex (e.g. Bash|Edit)").Grow(1)
					ui.TextInput(c, &sm.HookCommand).Placeholder("Command to execute (e.g. .rakit/hooks/guard.sh)").Grow(2)
				})
				ui.Row(c).Justify(ui.End).Gap(8).Children(func() {
					if ui.Button(c, "Cancel").Clicked() {
						sm.HookFormOpen = false
					}
					saveBtn := ContrastCTA(c, tc, "Save Hook")
					if saveBtn.Clicked() && strings.TrimSpace(sm.HookCommand) != "" {
						evVal := sm.HookEvent
						if evVal == "" {
							evVal = "PreToolUse"
						}
						updated := append(append([]settings.HookEntry(nil), sm.HooksList...), settings.HookEntry{
							ID:      fmt.Sprintf("hook-%d", time.Now().UnixNano()),
							Event:   evVal,
							Matcher: sm.HookMatcher,
							Command: sm.HookCommand,
						})
						// Persist through the manager so hooks round-trip.
						sm.SetHooks(updated)
						sm.HookEvent, sm.HookMatcher, sm.HookCommand = "", "", ""
						sm.HookFormOpen = false
						c.AddToast(ui.Toast{Title: "Hook Saved", Description: "Stored — it will run once hook execution is wired up", Type: "info"})
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
						ui.Row(c).Gap(6).AlignItems(ui.Center).Wrap().Children(func() {
							ui.Text(c, thisH.Event).FontSize(13).FontWeight(600)
							if thisH.Matcher != "" {
								Badge(c, tc, thisH.Matcher, tc.Info)
							}
							Badge(c, tc, "not executed", tc.TextMuted)
						})
						ui.Text(c, thisH.Command).FontSize(11).Font("monospace").TextColor(tc.TextMuted)
					})
					del := IconButton(c, tc, icons.TablerTrash, 13)
					if del.Clicked() {
						targetID := thisH.ID
						targetEvent := thisH.Event
						a.confirmDialogLabeled("Delete Lifecycle Hook?", fmt.Sprintf("Remove the %s hook?", targetEvent), "Delete", func() {
							var updated []settings.HookEntry
							for _, item := range sm.HooksList {
								if item.ID != targetID {
									updated = append(updated, item)
								}
							}
							sm.SetHooks(updated)
							a.queueToast(ui.Toast{Title: "Hook Deleted", Description: targetEvent, Type: "info"})
						}, nil)
					}
				})
			})
		}
		if len(sm.HooksList) == 0 {
			settingsEmptyState(c, tc, icons.TablerGitFork, "No lifecycle hooks configured", "Hooks let you attach automated validation scripts to agent lifecycle events.")
		}
	})
}

// ── Codebase Indexing ──────────────────────────────────────────────────────

func (a *AppUI) renderSettingsTabIndexing(c *ui.Context, tc theme.ThemeColors, sm *settings.SettingsManager) {
	settingsSectionHeader(c, tc, "Codebase Indexing", "Real-time symbol parsing, vector embeddings, and semantic code search status.")

	if sm.IndexingStatus == "" {
		sm.IndexingStatus = "Index ready & up-to-date"
	}

	// Real counts from the workspace file index — never fabricated. The
	// symbol count is omitted entirely rather than faked (no cheap parser
	// counter is wired to this screen yet).
	fileCount := 0
	var totalBytes int64
	if a.Workspace != nil {
		for _, f := range a.Workspace.FilteredFiles() {
			if !f.IsDir {
				fileCount++
				totalBytes += f.Size
			}
		}
	}

	settingsCard(c, tc, func() {
		ui.Column(c).Gap(14).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, "Index Status").FontSize(12).FontWeight(600)
					ui.Text(c, sm.IndexingStatus).FontSize(11).TextColor(tc.Accent)
				})
				reindexBtn := ContrastCTA(c, tc, "Reindex Workspace")
				if reindexBtn.Clicked() && !sm.ReindexBusy {
					sm.ReindexBusy = true
					sm.IndexingStatus = "Refreshing workspace index..."
					c.AddToast(ui.Toast{Title: "Reindex Started", Description: "Refreshing workspace file index", Type: "info"})
					go func() {
						if a.Workspace != nil {
							a.Workspace.Refresh()
						}
						a.runOnUI(func() {
							sm.ReindexBusy = false
							count := 0
							if a.Workspace != nil {
								for _, f := range a.Workspace.FilteredFiles() {
									if !f.IsDir {
										count++
									}
								}
							}
							sm.IndexingStatus = fmt.Sprintf("Index refreshed at %s — %d files", time.Now().Format("15:04:05"), count)
							a.invalidateUI()
						})
					}()
				}
			})

			settingsDivider(c, tc)

			ui.Row(c).Gap(14).Children(func() {
				ui.Column(c).Radius(10).Padding(12, 14).Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border).Grow(1).Gap(2).Children(func() {
					ui.Text(c, "INDEXED FILES").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
					ui.Text(c, fmt.Sprintf("%d", fileCount)).FontSize(20).FontWeight(700).TextColor(tc.Accent)
				})
				ui.Column(c).Radius(10).Padding(12, 14).Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border).Grow(1).Gap(2).Children(func() {
					ui.Text(c, "INDEX SIZE").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
					ui.Text(c, formatBytes(totalBytes)).FontSize(20).FontWeight(700).TextColor(tc.Text)
				})
			})

			settingsDivider(c, tc)

			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, "Exclusion Patterns").FontSize(12).FontWeight(600)
				ui.Text(c, "Directories and paths ignored during indexing and symbol scans.").FontSize(11).TextColor(tc.TextMuted)
				patterns := []string{".git", "node_modules", "dist", "build", "vendor", ".forge"}
				ui.Row(c).Gap(6).Wrap().Children(func() {
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

// truncatePreview shortens s for single-line dialog copy.
func truncatePreview(s string, max int) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}
