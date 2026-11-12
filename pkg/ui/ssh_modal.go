package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/icons"
	"github.com/hasdev/forge-ade/internal/remotefs"
)

const sshSavedHostsFile = "ssh_saved_hosts.json"

// parseSSHWorkspaceURI parses a ssh://<connID>/<remotePath> URI into its components.
// openRemoteTerminal spawns the ssh client under the PTY for the active
// remote workspace, as forge-ade's CreateShell(ssh://…) does.
func (a *AppUI) openRemoteTerminal() {
	if a.Engine == nil || a.Engine.RemoteFS == nil {
		return
	}
	client, remotePath, isRemote := a.Engine.RemoteFS.ParseRemotePath(a.Workspace.RootDir)
	if isRemote && client != nil {
		prog, args, env := client.SSHCommand(remotePath)
		a.ShowBottomTerminal = true
		a.Terminal.Command = append([]string{prog}, args...)
		a.Terminal.Env = env
		a.Terminal.Title = "SSH"
		_ = a.Terminal.Restart()
	} else {
		a.ShowBottomTerminal = true
		a.Terminal.Cwd = a.Workspace.RootDir
		_ = a.Terminal.Restart()
	}
	if a.Window != nil {
		a.Window.Invalidate()
	}
}

func parseSSHWorkspaceURI(uri string) (connID, remotePath, displayHost, folderName string) {
	if !strings.HasPrefix(uri, "ssh://") {
		return "", uri, "", filepath.Base(uri)
	}
	trimmed := strings.TrimPrefix(uri, "ssh://")
	slashIdx := strings.Index(trimmed, "/")
	if slashIdx != -1 {
		connID = trimmed[:slashIdx]
		remotePath = trimmed[slashIdx:]
	} else {
		connID = trimmed
		remotePath = "/"
	}
	folderName = filepath.Base(remotePath)
	if folderName == "." || folderName == "/" || folderName == "" {
		folderName = "remote-root"
	}
	displayHost = connID
	return connID, remotePath, displayHost, folderName
}

// OpenSSHModal opens the SSH Connection dialog and resets/initializes form fields.
func (a *AppUI) OpenSSHModal() {
	a.ShowSSHModal = true
	a.ShowSSHQuickMenu = false
	a.SSHErrorMsg = ""
	a.SSHIsConnecting = false
	a.LoadSavedSSHConfigs()

	if len(a.SSHSavedConfigs) > 0 && a.SSHSelectedConfigID == "" {
		a.LoadSSHConfig(a.SSHSavedConfigs[0])
	} else if a.SSHHost == "" {
		a.ResetSSHForm()
	}

	if a.Window != nil {
		a.Window.Invalidate()
	}
}

// ResetSSHForm clears input fields for a new host connection.
func (a *AppUI) ResetSSHForm() {
	a.SSHSelectedConfigID = ""
	a.SSHLabel = ""
	a.SSHHost = ""
	a.SSHPort = 22
	a.SSHPortText = "22"
	a.SSHUser = ""
	a.SSHAuthType = "key_file"
	a.SSHKeyPath = "~/.ssh/id_ed25519"
	a.SSHKeyPassphrase = ""
	a.SSHPassword = ""
	a.SSHRemotePath = "/var/www"
	a.SSHErrorMsg = ""
	a.SSHIsConnecting = false
}

// LoadSSHConfig populates form fields from a saved profile.
func (a *AppUI) LoadSSHConfig(cfg remotefs.SSHConfig) {
	a.SSHSelectedConfigID = cfg.ID
	a.SSHLabel = cfg.Label
	a.SSHHost = cfg.Host
	if cfg.Port > 0 {
		a.SSHPort = cfg.Port
	} else {
		a.SSHPort = 22
	}
	a.SSHPortText = strconv.Itoa(a.SSHPort)
	a.SSHUser = cfg.User
	if cfg.AuthType != "" {
		a.SSHAuthType = cfg.AuthType
	} else {
		a.SSHAuthType = "key_file"
	}
	if cfg.KeyPath != "" {
		a.SSHKeyPath = cfg.KeyPath
	} else {
		a.SSHKeyPath = "~/.ssh/id_ed25519"
	}
	a.SSHKeyPassphrase = cfg.KeyPassphrase
	a.SSHPassword = cfg.Password
	if cfg.RemotePath != "" {
		a.SSHRemotePath = cfg.RemotePath
	} else {
		a.SSHRemotePath = "/var/www"
	}
	a.SSHErrorMsg = ""
}

// sshStoragePath returns the JSON storage location for saved SSH configs.
func (a *AppUI) sshStoragePath() string {
	if a.Engine != nil && a.Engine.DataDir != "" {
		return filepath.Join(a.Engine.DataDir, sshSavedHostsFile)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".forge-ade", sshSavedHostsFile)
}

// LoadSavedSSHConfigs reads saved host profiles from disk.
func (a *AppUI) LoadSavedSSHConfigs() {
	filePath := a.sshStoragePath()
	data, err := os.ReadFile(filePath)
	if err != nil {
		return
	}
	var list []remotefs.SSHConfig
	if err := json.Unmarshal(data, &list); err == nil {
		a.SSHSavedConfigs = list
	}
}

// PersistSavedSSHConfigs writes saved host profiles to disk.
func (a *AppUI) PersistSavedSSHConfigs() {
	filePath := a.sshStoragePath()
	_ = os.MkdirAll(filepath.Dir(filePath), 0755)
	data, err := json.MarshalIndent(a.SSHSavedConfigs, "", "  ")
	if err == nil {
		_ = os.WriteFile(filePath, data, 0644)
	}
}

// SaveSSHConfig adds or updates a saved profile.
func (a *AppUI) SaveSSHConfig(cfg remotefs.SSHConfig) {
	updated := make([]remotefs.SSHConfig, 0, len(a.SSHSavedConfigs)+1)
	for _, c := range a.SSHSavedConfigs {
		if c.ID != cfg.ID {
			updated = append(updated, c)
		}
	}
	updated = append([]remotefs.SSHConfig{cfg}, updated...)
	a.SSHSavedConfigs = updated
	a.PersistSavedSSHConfigs()
}

// DeleteSSHConfig removes a profile from saved hosts.
func (a *AppUI) DeleteSSHConfig(id string) {
	updated := make([]remotefs.SSHConfig, 0, len(a.SSHSavedConfigs))
	for _, c := range a.SSHSavedConfigs {
		if c.ID != id {
			updated = append(updated, c)
		}
	}
	a.SSHSavedConfigs = updated
	a.PersistSavedSSHConfigs()
	if a.SSHSelectedConfigID == id {
		a.ResetSSHForm()
	}
}

// ConnectCurrentSSH initiates connection using the current form values.
func (a *AppUI) ConnectCurrentSSH() {
	if strings.TrimSpace(a.SSHHost) == "" || strings.TrimSpace(a.SSHUser) == "" {
		a.SSHErrorMsg = "Host and Username are required."
		return
	}

	a.SSHIsConnecting = true
	a.SSHErrorMsg = ""

	configID := a.SSHSelectedConfigID
	if configID == "" {
		configID = fmt.Sprintf("ssh-%d", time.Now().UnixNano())
	}
	lbl := strings.TrimSpace(a.SSHLabel)
	if lbl == "" {
		lbl = fmt.Sprintf("%s@%s", strings.TrimSpace(a.SSHUser), strings.TrimSpace(a.SSHHost))
	}
	remPath := strings.TrimSpace(a.SSHRemotePath)
	if remPath == "" {
		remPath = "/"
	}
	port := a.SSHPort
	if port <= 0 {
		port = 22
	}

	cfg := remotefs.SSHConfig{
		ID:            configID,
		Label:         lbl,
		Host:          strings.TrimSpace(a.SSHHost),
		Port:          port,
		User:          strings.TrimSpace(a.SSHUser),
		AuthType:      a.SSHAuthType,
		Password:      a.SSHPassword,
		KeyPath:       strings.TrimSpace(a.SSHKeyPath),
		KeyPassphrase: a.SSHKeyPassphrase,
		RemotePath:    remPath,
	}

	go func() {
		status, err := a.Engine.ConnectSSH(cfg)
		if err != nil {
			a.SSHIsConnecting = false
			a.SSHErrorMsg = err.Error()
			if a.Window != nil {
				a.Window.Invalidate()
			}
			return
		}
		if status == nil || !status.Connected {
			a.SSHIsConnecting = false
			errMsg := "Failed to connect to remote SSH server."
			if status != nil && status.Error != "" {
				errMsg = status.Error
			}
			a.SSHErrorMsg = errMsg
			if a.Window != nil {
				a.Window.Invalidate()
			}
			return
		}

		// Save profile and switch workspace
		a.SaveSSHConfig(cfg)
		remoteURI := fmt.Sprintf("ssh://%s%s", status.ID, status.RemotePath)

		a.SSHIsConnecting = false
		a.ShowSSHModal = false
		a.Mode = ModeEditor
		a.switchWorkspace(remoteURI)

		if a.Window != nil {
			a.Window.Invalidate()
		}
	}()
}

// DisconnectSSH closes the remote connection and reverts to default workspace.
func (a *AppUI) DisconnectSSH() {
	if strings.HasPrefix(a.Workspace.RootDir, "ssh://") {
		connID, _, _, _ := parseSSHWorkspaceURI(a.Workspace.RootDir)
		if connID != "" && a.Engine != nil {
			_ = a.Engine.DisconnectSSH(connID)
		}
	}
	a.ShowSSHQuickMenu = false
	home, _ := os.UserHomeDir()
	a.switchWorkspace(home)
	if a.Window != nil {
		a.Window.Invalidate()
	}
}

// renderSSHConnectModal renders the complete SSH connection dialog.
func (a *AppUI) renderSSHConnectModal(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	ui.Modal(c, &a.ShowSSHModal, func() {
		ui.Column(c).Width(640).Height(540).Radius(12).Background(tc.Surface).
			BorderWidth(1).BorderColor(tc.Border).Shadow(0, 24, 64, -20, ui.RGBA(0, 0, 0, 0.55)).Children(func() {
			// Header
			ui.Row(c).Height(58).Padding(0, 18).AlignItems(ui.Center).
				Background(tc.Canvas).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
				ui.Row(c).Size(34, 34).Radius(8).Background(tc.Accent.Alpha(0.12)).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerServer).Width(18).Height(18).TextColor(tc.Accent)
				})
				ui.Box(c).Width(10)
				ui.Column(c).Grow(1).Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "Connect to Remote Host via SSH").FontSize(13).FontWeight(600).TextColor(tc.Text)
						ui.Row(c).Padding(1, 5).Radius(4).Background(tc.Accent.Alpha(0.15)).Children(func() {
							ui.Text(c, "SFTP").FontSize(9).FontWeight(700).TextColor(tc.Accent)
						})
					})
					ui.Text(c, "Browse, edit, and sync remote files natively without local mounting").
						FontSize(11).TextColor(tc.TextMuted)
				})
				closeBtn := ui.Row(c).Size(26, 26).Radius(6).AlignItems(ui.Center).Justify(ui.Center)
				closeBtn.Children(func() {
					ui.Icon(c, icons.TablerX).Width(15).Height(15).TextColor(tc.IconMuted)
				})
				if closeBtn.Hovered() {
					closeBtn.Background(tc.Text.Alpha(0.08))
				}
				if closeBtn.Clicked() {
					a.ShowSSHModal = false
				}
			})

			// Body: Left Saved Hosts List + Right Connection Form
			ui.Row(c).Grow(1).Children(func() {
				// Left Saved Hosts Sidebar
				ui.Column(c).Width(185).Background(tc.Canvas).BorderWidth(0, 1, 0, 0).BorderColor(tc.Border).Children(func() {
					ui.Row(c).Height(34).Padding(0, 10).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
						ui.Text(c, "SAVED HOSTS").FontSize(10).FontWeight(600).TextColor(tc.TextMuted).Grow(1)
						newBtn := ui.Row(c).Size(22, 22).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
						newBtn.Children(func() {
							ui.Icon(c, icons.TablerPlus).Width(13).Height(13).TextColor(tc.Accent)
						})
						if newBtn.Hovered() {
							newBtn.Background(tc.Accent.Alpha(0.12))
						}
						if newBtn.Clicked() {
							a.ResetSSHForm()
						}
					})

					ui.Scroll(c).Grow(1).Children(func() {
						ui.Column(c).Padding(6, 6).Gap(2).Children(func() {
							if len(a.SSHSavedConfigs) == 0 {
								ui.Row(c).Padding(12, 6).Justify(ui.Center).Children(func() {
									ui.Text(c, "No saved hosts").FontSize(11).TextColor(tc.TextMuted)
								})
							} else {
								for _, cfg := range a.SSHSavedConfigs {
									itemCfg := cfg
									isSelected := (a.SSHSelectedConfigID == itemCfg.ID)
									isActive := strings.HasPrefix(a.Workspace.RootDir, "ssh://") &&
										(strings.Contains(a.Workspace.RootDir, itemCfg.ID) || strings.Contains(a.Workspace.RootDir, itemCfg.Host))

									itemRow := ui.Row(c.Key("saved-ssh-"+itemCfg.ID)).Height(42).Radius(6).
										Padding(4, 8).AlignItems(ui.Center)
									if isSelected {
										itemRow.Background(tc.Accent)
									} else if itemRow.Hovered() {
										itemRow.Background(tc.Text.Alpha(0.04))
									}

									itemRow.Children(func() {
										ui.Column(c).Grow(1).Children(func() {
											ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
												if isActive {
													ui.Box(c).Size(6, 6).Radius(3).Background(tc.Success)
												}
												txt := itemCfg.Label
												if txt == "" {
													txt = itemCfg.Host
												}
												lbl := ui.Text(c, txt).FontSize(11).FontWeight(500).SingleLine()
												if isSelected {
													lbl.TextColor(tc.Canvas)
												} else {
													lbl.TextColor(tc.Text)
												}
											})
											sub := fmt.Sprintf("%s@%s", itemCfg.User, itemCfg.Host)
											subLbl := ui.Text(c, sub).FontSize(10).SingleLine()
											if isSelected {
												subLbl.TextColor(tc.Canvas.Alpha(0.8))
											} else {
												subLbl.TextColor(tc.TextMuted)
											}
										})

										delBtn := ui.Row(c.Key("del-ssh-"+itemCfg.ID)).Size(20, 20).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
										delBtn.Children(func() {
											ic := ui.Icon(c, icons.TablerTrash).Width(12).Height(12)
											if isSelected {
												ic.TextColor(tc.Canvas.Alpha(0.9))
											} else {
												ic.TextColor(tc.IconMuted)
											}
										})
										if delBtn.Hovered() {
											delBtn.Background(tc.Error.Alpha(0.2))
										}
										if delBtn.Clicked() {
											a.DeleteSSHConfig(itemCfg.ID)
										}
									})

									if itemRow.Clicked() {
										a.LoadSSHConfig(itemCfg)
									}
								}
							}
						})
					})
				})

				// Right Form
				ui.Scroll(c).Grow(1).Children(func() {
					ui.Column(c).Padding(16, 16).Gap(12).Children(func() {
						// Error Banner
						if a.SSHErrorMsg != "" {
							ui.Row(c).Radius(6).Padding(8, 10).Gap(8).Background(tc.Error.Alpha(0.12)).
								BorderWidth(1, 1, 1, 1).BorderColor(tc.Error.Alpha(0.3)).AlignItems(ui.Center).Children(func() {
								ui.Icon(c, icons.TablerAlertTriangle).Width(16).Height(16).TextColor(tc.Error)
								ui.Text(c, a.SSHErrorMsg).FontSize(11).TextColor(tc.Error).Grow(1)
							})
						}

						// Row 1: Host + Port
						ui.Row(c).Gap(10).Children(func() {
							ui.Column(c).Grow(2).Gap(4).Children(func() {
								ui.Text(c, "Host / IP Address *").FontSize(11).FontWeight(500).TextColor(tc.TextMuted)
								ui.TextInput(c, &a.SSHHost).Placeholder("vps.example.com or 192.168.1.50").Grow(1)
							})
							ui.Column(c).Grow(1).Gap(4).Children(func() {
								ui.Text(c, "Port").FontSize(11).FontWeight(500).TextColor(tc.TextMuted)
								ui.TextInput(c, &a.SSHPortText).Placeholder("22").Grow(1)
							})
						})

						// Row 2: Username + Label
						ui.Row(c).Gap(10).Children(func() {
							ui.Column(c).Grow(1).Gap(4).Children(func() {
								ui.Text(c, "Username *").FontSize(11).FontWeight(500).TextColor(tc.TextMuted)
								ui.TextInput(c, &a.SSHUser).Placeholder("ubuntu or root").Grow(1)
							})
							ui.Column(c).Grow(1).Gap(4).Children(func() {
								ui.Text(c, "Display Label (Optional)").FontSize(11).FontWeight(500).TextColor(tc.TextMuted)
								ui.TextInput(c, &a.SSHLabel).Placeholder("e.g. Production API").Grow(1)
							})
						})

						// Row 3: Auth Method Tabs
						ui.Column(c).Gap(6).Children(func() {
							ui.Text(c, "Authentication Method").FontSize(11).FontWeight(500).TextColor(tc.TextMuted)
							ui.Row(c).Gap(6).Children(func() {
								authTab := func(id, label string, icon *ui.SVG) {
									isCur := (a.SSHAuthType == id)
									tab := ui.Row(c.Key("auth-tab-"+id)).Height(30).Radius(6).Grow(1).
										BorderWidth(1, 1, 1, 1).AlignItems(ui.Center).Justify(ui.Center).Gap(6)
									if isCur {
										tab.Background(tc.Accent.Alpha(0.12)).BorderColor(tc.Accent)
									} else {
										tab.Background(tc.Canvas).BorderColor(tc.Border)
									}
									tab.Children(func() {
										ic := ui.Icon(c, icon).Width(13).Height(13)
										if isCur {
											ic.TextColor(tc.Accent)
										} else {
											ic.TextColor(tc.IconMuted)
										}
										lbl := ui.Text(c, label).FontSize(11).FontWeight(500)
										if isCur {
											lbl.TextColor(tc.Accent)
										} else {
											lbl.TextColor(tc.TextMuted)
										}
									})
									if tab.Clicked() {
										a.SSHAuthType = id
									}
								}
								authTab("key_file", "Private Key", icons.TablerKey)
								authTab("password", "Password", icons.TablerLock)
								authTab("agent", "SSH Agent", icons.TablerServer)
							})
						})

						// Auth details box
						switch a.SSHAuthType {
						case "key_file":
							ui.Column(c).Padding(10, 10).Radius(6).Gap(8).Background(tc.Canvas).
								BorderWidth(1, 1, 1, 1).BorderColor(tc.Border).Children(func() {
								ui.Column(c).Gap(3).Children(func() {
									ui.Text(c, "Private Key Path").FontSize(10).FontWeight(500).TextColor(tc.TextMuted)
									ui.TextInput(c, &a.SSHKeyPath).Placeholder("~/.ssh/id_ed25519").Grow(1)
								})
								ui.Column(c).Gap(3).Children(func() {
									ui.Text(c, "Passphrase (Leave empty if unencrypted)").FontSize(10).FontWeight(500).TextColor(tc.TextMuted)
									ui.TextInput(c, &a.SSHKeyPassphrase).Placeholder("Optional passphrase").Password().Grow(1)
								})
							})
						case "password":
							ui.Column(c).Padding(10, 10).Radius(6).Gap(4).Background(tc.Canvas).
								BorderWidth(1, 1, 1, 1).BorderColor(tc.Border).Children(func() {
								ui.Text(c, "SSH Password").FontSize(10).FontWeight(500).TextColor(tc.TextMuted)
								ui.TextInput(c, &a.SSHPassword).Placeholder("Enter SSH password").Password().Grow(1)
							})
						case "agent":
							ui.Row(c).Padding(10, 10).Radius(6).Background(tc.Canvas).
								BorderWidth(1, 1, 1, 1).BorderColor(tc.Border).Children(func() {
								ui.Text(c, "Will authenticate via active local SSH agent ($SSH_AUTH_SOCK).").
									FontSize(11).TextColor(tc.TextMuted)
							})
						}

						// Remote Workspace Folder Path
						ui.Column(c).Gap(4).Children(func() {
							ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
								ui.Icon(c, icons.TablerFolder).Width(13).Height(13).TextColor(tc.Warning)
								ui.Text(c, "Remote Workspace Folder Path").FontSize(11).FontWeight(500).TextColor(tc.Text)
							})
							ui.TextInput(c, &a.SSHRemotePath).Placeholder("/var/www/my-project or /home/ubuntu").Grow(1)
							ui.Row(c).Gap(4).AlignItems(ui.Center).Padding(2, 0).Children(func() {
								ui.Text(c, "Quick paths:").FontSize(10).TextColor(tc.TextMuted)
								quickPaths := []string{"/var/www", "/home", "/etc", "/opt", "/root", "/"}
								for _, qp := range quickPaths {
									pathStr := qp
									isCur := (a.SSHRemotePath == pathStr)
									chip := ui.Row(c.Key("qp-"+pathStr)).Radius(3).Padding(1, 5)
									if isCur {
										chip.Background(tc.Accent)
									} else {
										chip.Background(tc.Canvas).BorderWidth(1, 1, 1, 1).BorderColor(tc.Border)
									}
									chip.Children(func() {
										txt := ui.Text(c, pathStr).FontSize(9)
										if isCur {
											txt.TextColor(tc.Canvas).FontWeight(600)
										} else {
											txt.TextColor(tc.TextMuted)
										}
									})
									if chip.Clicked() {
										a.SSHRemotePath = pathStr
									}
								}
							})
						})

						// Footer Actions
						ui.Row(c).Padding(10, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).
							AlignItems(ui.Center).Children(func() {
							if a.SSHIsConnecting {
								ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
									ui.Text(c, "Connecting to host via SFTP...").FontSize(11).TextColor(tc.Accent)
								})
							}
							ui.Box(c).Grow(1)

							cancelBtn := ui.Row(c).Height(30).Radius(6).Padding(0, 14).
								BorderWidth(1, 1, 1, 1).BorderColor(tc.Border).AlignItems(ui.Center).Justify(ui.Center)
							cancelBtn.Children(func() {
								ui.Text(c, "Cancel").FontSize(11).TextColor(tc.TextMuted)
							})
							if cancelBtn.Hovered() {
								cancelBtn.Background(tc.Text.Alpha(0.04))
							}
							if cancelBtn.Clicked() {
								a.ShowSSHModal = false
							}

							ui.Box(c).Width(8)

							connectBtn := ui.Row(c).Height(30).Radius(6).Padding(0, 16).
								Background(tc.Accent).AlignItems(ui.Center).Justify(ui.Center).Gap(6)
							connectBtn.Children(func() {
								label := "Connect & Open"
								if a.SSHIsConnecting {
									label = "Connecting..."
								}
								ui.Text(c, label).FontSize(11).FontWeight(600).TextColor(tc.Canvas)
								if !a.SSHIsConnecting {
									ui.Icon(c, icons.TablerArrowRight).Width(13).Height(13).TextColor(tc.Canvas)
								}
							})
							if connectBtn.Hovered() && !a.SSHIsConnecting {
								connectBtn.Background(tc.Accent.Alpha(0.85))
							}
							if connectBtn.Clicked() && !a.SSHIsConnecting {
								a.ConnectCurrentSSH()
							}
						})
					})
				})
			})
		})
	})
}

// renderSSHQuickMenuModal renders the popup remote actions drawer/menu for active SSH sessions.
func (a *AppUI) renderSSHQuickMenuModal(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	connID, remotePath, displayHost, _ := parseSSHWorkspaceURI(a.Workspace.RootDir)
	activeConns := a.Engine.ListSSHConnections()
	labelDisplay := displayHost
	for _, ac := range activeConns {
		if ac.ID == connID {
			if ac.Label != "" {
				labelDisplay = ac.Label
			}
			displayHost = fmt.Sprintf("%s@%s:%d", ac.User, ac.Host, ac.Port)
			break
		}
	}

	ui.Modal(c, &a.ShowSSHQuickMenu, func() {
		ui.Column(c).Width(480).Radius(12).Background(tc.Surface).
			BorderWidth(1).BorderColor(tc.Border).Shadow(0, 24, 64, -20, ui.RGBA(0, 0, 0, 0.55)).Children(func() {
			// Header
			ui.Row(c).Height(62).Padding(0, 16).AlignItems(ui.Center).
				Background(tc.Canvas).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
				ui.Row(c).Size(38, 38).Radius(8).Background(tc.Accent.Alpha(0.15)).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerServer).Width(20).Height(20).TextColor(tc.Accent)
				})
				ui.Box(c).Width(10)
				ui.Column(c).Grow(1).Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, labelDisplay).FontSize(13).FontWeight(600).TextColor(tc.Text)
						ui.Row(c).Padding(1, 5).Radius(4).Background(tc.Success.Alpha(0.15)).Gap(4).AlignItems(ui.Center).Children(func() {
							ui.Box(c).Size(5, 5).Radius(2.5).Background(tc.Success)
							ui.Text(c, "Connected").FontSize(9).FontWeight(600).TextColor(tc.Success)
						})
					})
					subText := fmt.Sprintf("%s • %s", displayHost, remotePath)
					ui.Text(c, subText).FontSize(11).TextColor(tc.TextMuted).SingleLine()
				})
				closeBtn := ui.Row(c).Size(26, 26).Radius(6).AlignItems(ui.Center).Justify(ui.Center)
				closeBtn.Children(func() {
					ui.Icon(c, icons.TablerX).Width(15).Height(15).TextColor(tc.IconMuted)
				})
				if closeBtn.Hovered() {
					closeBtn.Background(tc.Text.Alpha(0.08))
				}
				if closeBtn.Clicked() {
					a.ShowSSHQuickMenu = false
				}
			})

			// Change Directory Sub-panel (if toggled)
			if a.SSHIsChangingDir {
				ui.Column(c).Padding(12, 16).Background(tc.Canvas).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Gap(8).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, icons.TablerFolder).Width(13).Height(13).TextColor(tc.Accent)
						ui.Box(c).Width(6)
						ui.Text(c, "Change Remote Directory").FontSize(11).FontWeight(600).TextColor(tc.Text).Grow(1)
						cancelChange := ui.Row(c).Padding(2, 6).Radius(4)
						cancelChange.Children(func() {
							ui.Text(c, "Cancel").FontSize(10).TextColor(tc.TextMuted)
						})
						if cancelChange.Hovered() {
							cancelChange.Background(tc.Text.Alpha(0.05))
						}
						if cancelChange.Clicked() {
							a.SSHIsChangingDir = false
						}
					})
					ui.Row(c).Gap(6).Children(func() {
						ui.TextInput(c, &a.SSHNewDirPath).Placeholder("/var/www/my-project").Grow(1)
						switchBtn := ui.Row(c).Height(30).Radius(6).Padding(0, 12).
							Background(tc.Accent).AlignItems(ui.Center).Justify(ui.Center).Gap(4)
						switchBtn.Children(func() {
							ui.Text(c, "Switch").FontSize(11).FontWeight(600).TextColor(tc.Canvas)
							ui.Icon(c, icons.TablerArrowRight).Width(12).Height(12).TextColor(tc.Canvas)
						})
						if switchBtn.Clicked() {
							clean := strings.TrimSpace(a.SSHNewDirPath)
							if clean != "" {
								if !strings.HasPrefix(clean, "/") {
									clean = "/" + clean
								}
								newURI := fmt.Sprintf("ssh://%s%s", connID, clean)
								a.ShowSSHQuickMenu = false
								a.SSHIsChangingDir = false
								a.switchWorkspace(newURI)
							}
						}
					})
					ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "Quick jump:").FontSize(10).TextColor(tc.TextMuted)
						quickPaths := []string{"/var/www", "/home", "/etc", "/opt", "/root", "/"}
						for _, qp := range quickPaths {
							pathStr := qp
							chip := ui.Row(c.Key("qjump-"+pathStr)).Radius(3).Padding(1, 5).
								Background(tc.Surface).BorderWidth(1, 1, 1, 1).BorderColor(tc.Border)
							chip.Children(func() {
								ui.Text(c, pathStr).FontSize(9).TextColor(tc.TextMuted)
							})
							if chip.Clicked() {
								a.SSHNewDirPath = pathStr
							}
						}
					})
				})
			}

			// Action list
			ui.Column(c).Padding(6, 8).Gap(2).Children(func() {
				actionItem := func(icon *ui.SVG, title, subtitle, shortcut string, isDanger bool, onPick func()) {
					item := ui.Row(c).Height(46).Radius(6).Padding(0, 10).AlignItems(ui.Center).Gap(10)
					if isDanger {
						if item.Hovered() {
							item.Background(tc.Error.Alpha(0.12))
						}
					} else {
						if item.Hovered() {
							item.Background(tc.Text.Alpha(0.04))
						}
					}
					item.Children(func() {
						iconBox := ui.Row(c).Size(30, 30).Radius(6).AlignItems(ui.Center).Justify(ui.Center)
						if isDanger {
							iconBox.Background(tc.Error.Alpha(0.15))
							ui.Icon(c, icon).Width(15).Height(15).TextColor(tc.Error)
						} else {
							iconBox.Background(tc.Text.Alpha(0.06))
							ui.Icon(c, icon).Width(15).Height(15).TextColor(tc.Accent)
						}
						ui.Column(c).Grow(1).Children(func() {
							txt := ui.Text(c, title).FontSize(11).FontWeight(500)
							if isDanger {
								txt.TextColor(tc.Error)
							} else {
								txt.TextColor(tc.Text)
							}
							ui.Text(c, subtitle).FontSize(10).TextColor(tc.TextMuted)
						})
						if shortcut != "" {
							ui.Text(c, shortcut).FontSize(10).TextColor(tc.TextMuted)
						}
					})
					if item.Clicked() {
						onPick()
					}
				}

				actionItem(icons.TablerTerminal, "Open Remote Terminal", "Launch an interactive terminal in this directory", "⌘`", false, func() {
					a.ShowSSHQuickMenu = false
					a.openRemoteTerminal()
				})

				actionItem(icons.TablerFolderOpen, "Change Remote Directory", fmt.Sprintf("Switch to another path on %s", displayHost), "", false, func() {
					a.SSHIsChangingDir = !a.SSHIsChangingDir
					a.SSHNewDirPath = remotePath
				})

				actionItem(icons.TablerRefresh, "Refresh Remote Explorer", "Reload remote file and directory tree from server", "", false, func() {
					a.ShowSSHQuickMenu = false
					a.Workspace.Refresh()
					if a.Window != nil {
						a.Window.Invalidate()
					}
				})

				actionItem(icons.TablerGlobe, "Connect to Another Host...", "Open saved SSH hosts or connect to a new server", "", false, func() {
					a.ShowSSHQuickMenu = false
					a.OpenSSHModal()
				})

				actionItem(icons.TablerUnplug, "Disconnect SSH Remote", "Close remote workspace and return to local files", "", true, func() {
					a.DisconnectSSH()
				})
			})

			// Footer
			ui.Row(c).Height(30).Padding(0, 16).AlignItems(ui.Center).
				Background(tc.Canvas).BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {
				ui.Text(c, "ForgeADE Remote SSH SFTP").FontSize(10).TextColor(tc.TextMuted).Grow(1)
				ui.Text(c, "Press Esc to close").FontSize(10).TextColor(tc.TextMuted)
			})
		})
	})
}
