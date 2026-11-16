package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
)

type projectSwitcherEntry struct {
	Path        string
	Name        string
	Subtitle    string
	IsSSH       bool
	IsConnected bool
	IsActive    bool
	InWindow    bool
}

// buildProjectSwitcherEntry constructs metadata for a workspace item in the switcher modal.
func (a *AppUI) buildProjectSwitcherEntry(ws, current string, inWindow bool) projectSwitcherEntry {
	name := resolveProjectName(ws)
	isSSH := strings.HasPrefix(ws, "ssh://")
	isActive := sameWorkspacePath(ws, current)
	isConnected := false
	subtitle := ws

	if isSSH {
		connID, remotePath, displayHost, _ := parseSSHWorkspaceURI(ws)
		if a.Engine != nil {
			isConnected = a.Engine.IsSSHConnected(connID)
		}
		hostDesc := displayHost
		for _, cfg := range a.SSHSavedConfigs {
			if cfg.ID == connID {
				if cfg.User != "" {
					hostDesc = fmt.Sprintf("%s@%s:%d", cfg.User, cfg.Host, cfg.Port)
				} else {
					hostDesc = fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
				}
				break
			}
		}
		subtitle = fmt.Sprintf("SSH: %s (%s)", hostDesc, remotePath)
	} else {
		if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(ws, home) {
			subtitle = "~" + strings.TrimPrefix(ws, home)
		}
	}

	return projectSwitcherEntry{
		Path:        ws,
		Name:        name,
		Subtitle:    subtitle,
		IsSSH:       isSSH,
		IsConnected: isConnected,
		IsActive:    isActive,
		InWindow:    inWindow,
	}
}

// renderProjectSwitcherModal draws the Zed-style project switcher overlay
// featuring This Window active group, Recent Projects, search filtering, and close buttons.
func (a *AppUI) renderProjectSwitcherModal(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	if c.Shortcut(0, ui.KeyEscape) {
		a.ShowProjectSwitcherModal = false
	}

	q := strings.ToLower(strings.TrimSpace(a.ProjectSwitcherQuery))
	current := ""
	if a.Workspace != nil {
		current = a.Workspace.RootDir
	}

	var thisWindow []projectSwitcherEntry
	var recentProjects []projectSwitcherEntry

	windowSet := make(map[string]bool)
	for _, ws := range a.OpenWorkspaces {
		if ws == "" {
			continue
		}
		windowSet[ws] = true
		entry := a.buildProjectSwitcherEntry(ws, current, true)
		if q == "" || strings.Contains(strings.ToLower(entry.Name), q) || strings.Contains(strings.ToLower(entry.Subtitle), q) || strings.Contains(strings.ToLower(entry.Path), q) {
			thisWindow = append(thisWindow, entry)
		}
	}

	for _, ws := range a.RecentWorkspaces {
		if ws == "" || windowSet[ws] {
			continue
		}
		entry := a.buildProjectSwitcherEntry(ws, current, false)
		if q == "" || strings.Contains(strings.ToLower(entry.Name), q) || strings.Contains(strings.ToLower(entry.Subtitle), q) || strings.Contains(strings.ToLower(entry.Path), q) {
			recentProjects = append(recentProjects, entry)
		}
	}

	totalCount := len(thisWindow) + len(recentProjects)
	if totalCount > 0 {
		if c.Shortcut(0, ui.KeyDown) {
			a.projectSwitcherSelected = (a.projectSwitcherSelected + 1) % totalCount
		}
		if c.Shortcut(0, ui.KeyUp) {
			a.projectSwitcherSelected = (a.projectSwitcherSelected - 1 + totalCount) % totalCount
		}
	}

	activateEntry := func(entry projectSwitcherEntry) {
		a.ShowProjectSwitcherModal = false
		a.switchWorkspace(entry.Path)
	}

	if totalCount > 0 && c.Shortcut(0, ui.KeyEnter) {
		allEntries := append(append([]projectSwitcherEntry{}, thisWindow...), recentProjects...)
		if a.projectSwitcherSelected >= 0 && a.projectSwitcherSelected < len(allEntries) {
			activateEntry(allEntries[a.projectSwitcherSelected])
			return
		}
	}

	a.modal(c, &a.ShowProjectSwitcherModal, func() {
		panel := ui.Column(c).Width(580).Radius(14).Background(tc.SurfaceOverlay).
			BorderWidth(1).BorderColor(tc.Border).
			Shadow(0, 24, 72, -18, ui.RGBA(0, 0, 0, 0.6)).Clip()
		panel.Children(func() {
			// Search Input Header
			ui.Row(c).Padding(12, 14).Gap(10).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
				ui.Icon(c, icons.TablerSearch).Width(16).Height(16).TextColor(tc.IconMuted)
				in := ui.TextInput(c, &a.ProjectSwitcherQuery).Placeholder("Search projects...").Grow(1).AutoFocus()
				if in.Submitted() && totalCount > 0 {
					allEntries := append(append([]projectSwitcherEntry{}, thisWindow...), recentProjects...)
					if a.projectSwitcherSelected >= 0 && a.projectSwitcherSelected < len(allEntries) {
						activateEntry(allEntries[a.projectSwitcherSelected])
					}
				}
			})

			// Content List
			ui.Scroll(c).MaxHeight(380).Grow(1).Padding(8, 6).Children(func() {
				ui.Column(c).Gap(4).Children(func() {
					renderEntryRow := func(entry projectSwitcherEntry, gIdx int) {
						isSelected := gIdx == a.projectSwitcherSelected
						row := ui.Row(c.Key(fmt.Sprintf("psm-item-%s-%d", entry.Path, gIdx))).
							Height(40).Radius(8).Padding(0, 10).Gap(10).AlignItems(ui.Center).
							Cursor(ui.CursorPointer)
						if isSelected {
							row.Background(tc.Accent.Alpha(0.12))
						} else if row.Hovered() {
							row.Background(tc.Text.Alpha(0.05))
						}

						closedThis := false
						row.Children(func() {
							// Active checkmark or spacer
							if entry.IsActive {
								ui.Icon(c, icons.TablerCheck).Width(14).Height(14).TextColor(tc.Accent)
							} else {
								ui.Box(c).Width(14).Height(14)
							}

							// Type icon (Server vs Folder)
							typeIcon := icons.TablerFolder
							iconColor := tc.IconMuted
							if entry.IsSSH {
								typeIcon = icons.TablerServer
								if entry.IsConnected {
									iconColor = ui.RGBA(16, 185, 129, 1.0)
								} else {
									iconColor = tc.Accent
								}
							}
							ui.Icon(c, typeIcon).Width(16).Height(16).TextColor(iconColor)

							// Name and Subtitle Column
							ui.Column(c).Grow(1).Gap(1).Children(func() {
								nameColor := tc.Text
								if entry.IsActive {
									nameColor = tc.Accent
								}
								ui.Text(c, entry.Name).FontSize(12).FontWeight(600).TextColor(nameColor).SingleLine()
								ui.Text(c, entry.Subtitle).FontSize(10).TextColor(tc.TextMuted).SingleLine()
							})

							// Status badge for SSH
							if entry.IsSSH {
								badgeText := "SSH"
								badgeBg := tc.Accent.Alpha(0.12)
								badgeFg := tc.Accent
								if entry.IsConnected {
									badgeText = "Connected"
									badgeBg = ui.RGBA(16, 185, 129, 0.15)
									badgeFg = ui.RGBA(16, 185, 129, 1.0)
								}
								ui.Row(c).Height(18).Padding(0, 6).Radius(4).Background(badgeBg).AlignItems(ui.Center).Children(func() {
									ui.Text(c, badgeText).FontSize(9).FontWeight(600).TextColor(badgeFg)
								})
							}

							// Close button 'x' (for This Window items)
							if entry.InWindow {
								xBtn := ui.Row(c.Key(fmt.Sprintf("psm-close-%s-%d", entry.Path, gIdx))).
									Size(20, 20).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
								xBtn.Children(func() {
									ui.Icon(c, icons.TablerX).Width(11).Height(11).TextColor(tc.IconMuted)
								})
								if xBtn.Hovered() {
									xBtn.Background(tc.Text.Alpha(0.1))
								}
								if xBtn.Clicked() {
									closedThis = true
									a.closeWorkspaceFromWindow(entry.Path)
								}
							}
						})

						if !closedThis && row.Clicked() {
							activateEntry(entry)
						}
					}

					// Group 1: THIS WINDOW
					if len(thisWindow) > 0 {
						ui.Row(c).Padding(6, 8, 2, 8).Children(func() {
							ui.Text(c, "THIS WINDOW").FontSize(10).FontWeight(700).TextColor(tc.TextMuted)
						})
						for idx, entry := range thisWindow {
							renderEntryRow(entry, idx)
						}
					}

					// Group 2: RECENT PROJECTS
					if len(recentProjects) > 0 {
						offset := len(thisWindow)
						ui.Row(c).Padding(10, 8, 2, 8).Children(func() {
							ui.Text(c, "RECENT PROJECTS").FontSize(10).FontWeight(700).TextColor(tc.TextMuted)
						})
						for idx, entry := range recentProjects {
							renderEntryRow(entry, offset+idx)
						}
					}

					if totalCount == 0 {
						ui.Row(c).Padding(24, 12).Justify(ui.Center).Children(func() {
							ui.Text(c, "No matching projects found").FontSize(12).TextColor(tc.TextMuted)
						})
					}
				})
			})

			// Bottom Quick Actions Bar
			ui.Row(c).Padding(8, 12).Gap(8).AlignItems(ui.Center).
				BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Background(tc.Surface.Alpha(0.4)).Children(func() {
				// Open Local
				localBtn := ui.Row(c.Key("psm-open-local")).Height(26).Radius(6).Padding(0, 8).Gap(6).
					AlignItems(ui.Center).Cursor(ui.CursorPointer)
				localBtn.Children(func() {
					ui.Icon(c, icons.TablerFolderPlus).Width(14).Height(14).TextColor(tc.IconMuted)
					ui.Text(c, "Open Local...").FontSize(11).TextColor(tc.Text)
				})
				if localBtn.Hovered() {
					localBtn.Background(tc.Text.Alpha(0.06))
				}
				if localBtn.Clicked() {
					a.ShowProjectSwitcherModal = false
					a.openWorkspaceFolder()
				}

				// Open Remote
				remoteBtn := ui.Row(c.Key("psm-open-remote")).Height(26).Radius(6).Padding(0, 8).Gap(6).
					AlignItems(ui.Center).Cursor(ui.CursorPointer)
				remoteBtn.Children(func() {
					ui.Icon(c, icons.TablerServer).Width(14).Height(14).TextColor(tc.Accent)
					ui.Text(c, "Open Remote SSH...").FontSize(11).TextColor(tc.Text)
				})
				if remoteBtn.Hovered() {
					remoteBtn.Background(tc.Text.Alpha(0.06))
				}
				if remoteBtn.Clicked() {
					a.ShowProjectSwitcherModal = false
					a.OpenSSHModal()
				}

				// General
				genBtn := ui.Row(c.Key("psm-general")).Height(26).Radius(6).Padding(0, 8).Gap(6).
					AlignItems(ui.Center).Cursor(ui.CursorPointer)
				genBtn.Children(func() {
					ui.Icon(c, icons.TablerCompass).Width(14).Height(14).TextColor(tc.IconMuted)
					ui.Text(c, "General").FontSize(11).TextColor(tc.TextMuted)
				})
				if genBtn.Hovered() {
					genBtn.Background(tc.Text.Alpha(0.06))
				}
				if genBtn.Clicked() {
					a.ShowProjectSwitcherModal = false
					a.switchWorkspace("")
				}

				ui.Box(c).Grow(1)

				ui.Text(c, "↑↓ navigate · ↵ switch · esc close").FontSize(10).TextColor(tc.TextMuted)
			})
		})
	})
}
