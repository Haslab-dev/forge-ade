package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/terminal"
	"github.com/hasdev/forge-ade/core/remotefs"
)

const sshSavedHostsFile = "ssh_saved_hosts.json"

// parseSSHWorkspaceURI parses a ssh://<connID>/<remotePath> URI into its components.
// openRemoteTerminal spawns the ssh client under the PTY for the active
// remote workspace, as forge-ade's CreateShell(ssh://…) does.
func (a *AppUI) openRemoteTerminal() {
	a.openRemoteTerminalPath("")
}

func (a *AppUI) openRemoteTerminalPath(targetURI string) {
	if a.Engine == nil || a.Engine.RemoteFS == nil {
		return
	}
	uri := targetURI
	if uri == "" && a.Workspace != nil {
		uri = a.Workspace.RootDir
	}
	if a.Mode == ModeEditor {
		a.openEditorTerminalTabForURI("", uri)
		return
	}
	ts := a.activeTerminal()
	if ts == nil {
		return
	}
	client, remotePath, isRemote := a.Engine.RemoteFS.ParseRemotePath(uri)
	if isRemote && client != nil {
		prog, args, env := client.SSHCommand(remotePath)
		a.setBottomTerminalVisible(true)
		ts.Command = append([]string{prog}, args...)
		ts.Env = env
		_, _, _, folder := parseSSHWorkspaceURI(uri)
		if folder == "" || folder == "remote-root" {
			folder = filepath.Base(remotePath)
		}
		if folder == "" || folder == "." || folder == "/" {
			folder = "remote"
		}
		ts.Title = folder
		_ = ts.Restart()
	} else {
		a.setBottomTerminalVisible(true)
		_ = ts.ResetShell(uri)
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
	// Reopening (or resetting) the modal invalidates any in-flight connect:
	// its goroutine abandons save/switch when the generation moved on.
	a.sshConnectGen++
	a.sshSFTPBusyID = ""
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
	// Starting over abandons any in-flight connect for the old form values.
	a.sshConnectGen++
	a.sshSFTPBusyID = ""
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
	// The store holds plaintext passwords and key passphrases: repair files
	// written by older builds with world-readable permissions.
	if info, statErr := os.Stat(filePath); statErr == nil && info.Mode().Perm() != 0600 {
		_ = os.Chmod(filePath, 0600)
	}
}

// PersistSavedSSHConfigs writes saved host profiles to disk. The JSON stores
// plaintext secrets (passwords, key passphrases), so it is written
// owner-only: 0600 on the file, 0700 on the directory.
func (a *AppUI) PersistSavedSSHConfigs() {
	filePath := a.sshStoragePath()
	_ = os.MkdirAll(filepath.Dir(filePath), 0700)
	data, err := json.MarshalIndent(a.SSHSavedConfigs, "", "  ")
	if err == nil {
		_ = os.WriteFile(filePath, data, 0600)
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

// DeleteSSHConfig removes a profile from saved hosts and cleans up any
// sidebar workspaces/sessions bound to that host so it does not reappear.
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
	if id != "" {
		hostPrefix := "ssh://" + id
		matchesHost := func(ws string) bool {
			return ws == hostPrefix || strings.HasPrefix(ws, hostPrefix+"/")
		}
		var toRemove []string
		for _, p := range a.RecentWorkspaces {
			if matchesHost(p) {
				toRemove = append(toRemove, p)
			}
		}
		for _, p := range a.OpenWorkspaces {
			if matchesHost(p) {
				toRemove = append(toRemove, p)
			}
		}
		if a.Agent != nil {
			for _, s := range a.Agent.AllSessions() {
				if s != nil && matchesHost(s.WorkspacePath) {
					toRemove = append(toRemove, s.WorkspacePath)
				}
			}
		}
		seen := make(map[string]bool, len(toRemove))
		for _, ws := range toRemove {
			if seen[ws] {
				continue
			}
			seen[ws] = true
			a.deleteWorkspaceGroup(nil, resolveProjectName(ws), ws)
		}
	}
}

// ConnectCurrentSSH initiates connection using the current form values.
// A generation counter guards the goroutine: cancelling or reopening the
// modal invalidates in-flight results (no save, no workspace switch), and a
// re-entrant click while a connect is running is ignored.
func (a *AppUI) ConnectCurrentSSH() {
	if a.SSHIsConnecting {
		return // a connect is already in flight
	}
	if strings.TrimSpace(a.SSHHost) == "" || strings.TrimSpace(a.SSHUser) == "" {
		a.SSHErrorMsg = "Host and Username are required."
		return
	}

	// Port validation: an unparsable port is surfaced instead of silently
	// keeping the previous value.
	port := 22
	if txt := strings.TrimSpace(a.SSHPortText); txt != "" {
		parsed, parseErr := strconv.Atoi(txt)
		if parseErr != nil || parsed <= 0 || parsed > 65535 {
			a.queueToast(ui.Toast{
				Title:       "Invalid Port",
				Description: fmt.Sprintf("“%s” is not a valid port (1–65535). Fix it and connect again.", txt),
				Type:        "error",
			})
			return
		}
		port = parsed
		a.SSHPort = parsed
	}

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

	a.startSSHConnect(cfg, func(status *remotefs.ConnectionStatus) {
		// Save profile and switch workspace on success.
		a.SaveSSHConfig(cfg)
		remoteURI := fmt.Sprintf("ssh://%s%s", status.ID, status.RemotePath)
		a.ShowSSHModal = false
		a.Mode = ModeEditor
		a.switchWorkspace(remoteURI)
	})
}

// startSSHConnect runs Engine.ConnectSSH off the UI thread and publishes the
// result through runOnUI. onConnected (UI thread) runs only when the connect
// succeeded AND the generation captured at start is still current.
func (a *AppUI) startSSHConnect(cfg remotefs.SSHConfig, onConnected func(status *remotefs.ConnectionStatus)) {
	if a.SSHIsConnecting {
		return
	}
	if a.Engine == nil {
		a.SSHErrorMsg = "Engine unavailable — cannot connect."
		a.invalidateUI()
		return
	}
	a.SSHIsConnecting = true
	a.SSHErrorMsg = ""
	a.sshConnectGen++
	gen := a.sshConnectGen
	a.invalidateUI()

	go func() {
		status, err := a.Engine.ConnectSSH(cfg)
		a.runOnUI(func() {
			if gen != a.sshConnectGen {
				// Modal was cancelled/reopened while connecting: abandon the
				// result entirely (no save, no switch, no error surface).
				return
			}
			a.SSHIsConnecting = false
			switch {
			case err != nil:
				// Includes TOFU host-key mismatch text from core/remotefs,
				// which names the pinned-key file to delete.
				a.SSHErrorMsg = err.Error()
			case status == nil || !status.Connected:
				errMsg := "Failed to connect to remote SSH server."
				if status != nil && status.Error != "" {
					errMsg = status.Error
				}
				a.SSHErrorMsg = errMsg
			default:
				if onConnected != nil {
					onConnected(status)
				}
			}
			a.invalidateUI()
		})
	}()
}

// connectAndOpenSFTP backs the saved-host SFTP buttons: uploads always run
// against the ACTIVE connection, so a non-active host connects first and the
// uploader only opens on success.
func (a *AppUI) connectAndOpenSFTP(cfg remotefs.SSHConfig) {
	if a.sshSFTPBusyID != "" {
		return // a connect-then-upload chain is already running
	}
	if a.sshHostActive(cfg) {
		a.OpenRemoteFileUploader(cfg.RemotePath)
		return
	}
	if a.SSHIsConnecting || a.Engine == nil {
		return
	}
	a.sshSFTPBusyID = cfg.ID
	a.LoadSSHConfig(cfg)
	a.startSSHConnect(cfg, func(status *remotefs.ConnectionStatus) {
		a.sshSFTPBusyID = ""
		a.SaveSSHConfig(cfg)
		remoteURI := fmt.Sprintf("ssh://%s%s", status.ID, status.RemotePath)
		a.ShowSSHModal = false
		a.Mode = ModeEditor
		a.switchWorkspace(remoteURI)
		a.OpenRemoteFileUploader(cfg.RemotePath)
	})
}

// sshHostActive reports whether the saved host owns the active remote
// workspace connection.
func (a *AppUI) sshHostActive(cfg remotefs.SSHConfig) bool {
	root := ""
	if a.Workspace != nil {
		root = a.Workspace.RootDir
	}
	if !strings.HasPrefix(root, "ssh://") {
		return false
	}
	connID, _, _, _ := parseSSHWorkspaceURI(root)
	return connID == cfg.ID || connID == cfg.Host || strings.Contains(root, cfg.Host)
}

// cancelSSHModal dismisses the connection dialog and abandons any in-flight
// connect by bumping the generation counter.
func (a *AppUI) cancelSSHModal() {
	a.ShowSSHModal = false
	a.sshConnectGen++
	a.sshSFTPBusyID = ""
	if !a.SSHIsConnecting {
		return
	}
	// The in-flight goroutine will abandon its result; clear the spinner now.
	a.SSHIsConnecting = false
	a.invalidateUI()
}

// quoteRemotePath single-quotes a remote path for POSIX shells so spaces and
// metacharacters survive the ssh command line ('\' escaping included).
func quoteRemotePath(p string) string {
	return "'" + strings.ReplaceAll(p, "'", `'\''`) + "'"
}

// ReconnectSSH re-establishes the active remote SSH workspace connection
// if the network was dropped or disconnected, refreshing the explorer
// and restarting any dead terminal tabs.
func (a *AppUI) ReconnectSSH() {
	if a.Engine == nil || a.Workspace == nil || !strings.HasPrefix(a.Workspace.RootDir, "ssh://") {
		return
	}
	connID, _, displayHost, _ := parseSSHWorkspaceURI(a.Workspace.RootDir)
	if connID == "" {
		return
	}
	a.queueToast(ui.Toast{Title: "Reconnecting SSH...", Description: fmt.Sprintf("Connecting to %s", displayHost), Type: "info"})
	go func() {
		status, err := a.Engine.ReconnectSSH(connID)
		a.runOnUI(func() {
			if err != nil {
				a.queueToast(ui.Toast{Title: "SSH Reconnect Failed", Description: err.Error(), Type: "error"})
				return
			}
			a.queueToast(ui.Toast{Title: "SSH Reconnected", Description: fmt.Sprintf("Connected to %s — resuming terminal session", status.Host), Type: "success"})
			if a.Workspace != nil {
				go a.Workspace.Refresh()
			}

			// Re-attach any open remote terminals (Editor terminal tabs and Bottom
			// terminal) using the fresh SSHCommand so tmux resumes the latest session.
			if a.Engine != nil && a.Engine.RemoteFS != nil {
				if client, remotePath, ok := a.Engine.RemoteFS.ParseRemotePath(a.Workspace.RootDir); ok && client != nil {
					prog, args, env := client.SSHCommand(remotePath)
					freshCmd := append([]string{prog}, args...)
					restarted := map[*terminal.TerminalState]bool{}
					reattach := func(ts *terminal.TerminalState) {
						if ts == nil || restarted[ts] {
							return
						}
						restarted[ts] = true
						ts.Command = append([]string(nil), freshCmd...)
						ts.Env = append([]string(nil), env...)
						_ = ts.Restart()
					}

					// 1. Editor main-pane terminal tabs (if any terminal:// tab is open).
					hasEditorTermTab := false
					if a.Editor != nil {
						for _, tabPath := range a.Editor.OpenTabs() {
							if strings.HasPrefix(tabPath, "terminal://") {
								hasEditorTermTab = true
								break
							}
						}
					}
					if hasEditorTermTab && a.Terminal != nil {
						reattach(a.Terminal)
					}

					// 2. Active mode terminal (Bottom terminal / Right drawer terminal).
					if ts := a.activeTerminal(); ts != nil {
						if ts.IsExited() || a.isBottomTerminalVisible() || (a.ShowRightDrawer && a.RightDrawerTab == RightTabTerminal) {
							reattach(ts)
						}
					}
				}
			}
			if a.Window != nil {
				a.Window.Invalidate()
			}
		})
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

// connectSSHShell launches an interactive SSH session in the active mode terminal.
func (a *AppUI) connectSSHShell(cfg remotefs.SSHConfig) {
	if a.Mode == ModeEditor {
		a.openEditorTerminalTab("")
		return
	}
	ts := a.activeTerminal()
	if ts == nil {
		return
	}
	if a.Engine != nil && a.Engine.RemoteFS != nil {
		if client := a.Engine.RemoteFS.GetClient(cfg.ID); client != nil {
			prog, args, env := client.SSHCommand(cfg.RemotePath)
			a.setBottomTerminalVisible(true)
			ts.Command = append([]string{prog}, args...)
			ts.Env = env
			ts.Title = fmt.Sprintf("SSH: %s", cfg.Host)
			_ = ts.Restart()
			if a.Window != nil {
				a.Window.Invalidate()
			}
			return
		}
	}
	userHost := cfg.Host
	if cfg.User != "" {
		userHost = fmt.Sprintf("%s@%s", cfg.User, cfg.Host)
	}
	portStr := strconv.Itoa(cfg.Port)
	if cfg.Port <= 0 {
		portStr = "22"
	}
	args := []string{"ssh", "-p", portStr}
	if cfg.KeyPath != "" && cfg.AuthType == "key_file" {
		kp := cfg.KeyPath
		if strings.HasPrefix(kp, "~/") {
			home, _ := os.UserHomeDir()
			kp = filepath.Join(home, kp[2:])
		}
		args = append(args, "-i", kp)
	}
	args = append(args, userHost)
	if cfg.RemotePath != "" && cfg.RemotePath != "/" {
		// Single-quote the remote path so spaces and shell metacharacters in
		// it survive the ssh command line intact.
		args = append(args, "-t", fmt.Sprintf("cd %s && exec $SHELL -l", quoteRemotePath(cfg.RemotePath)))
	}
	a.setBottomTerminalVisible(true)
	ts.Command = args
	ts.Title = fmt.Sprintf("SSH: %s", cfg.Host)
	_ = ts.Restart()
	if a.Window != nil {
		a.Window.Invalidate()
	}
}

// renderSSHConnectModal renders the Termius-style SSH connection & SFTP management dialog.
func (a *AppUI) renderSSHConnectModal(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	a.modal(c, &a.ShowSSHModal, func() {
		ui.Column(c).Width(720).Height(520).Radius(12).Background(tc.Surface).
			BorderWidth(1).BorderColor(tc.Border).Shadow(0, 24, 64, -20, ui.RGBA(0, 0, 0, 0.55)).Clip().Children(func() {
			// Header (Fixed)
			ui.Row(c).Height(52).Padding(0, 18).AlignItems(ui.Center).
				Background(tc.Canvas).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
				ui.Row(c).Size(32, 32).Radius(8).Background(tc.Accent.Alpha(0.12)).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerServer).Width(18).Height(18).TextColor(tc.Accent)
				})
				ui.Box(c).Width(10)
				ui.Column(c).Grow(1).Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "Remote Server Connections").FontSize(13).FontWeight(600).TextColor(tc.Text)
						ui.Row(c).Padding(1, 5).Radius(4).Background(tc.Accent.Alpha(0.15)).Children(func() {
							ui.Text(c, "SSH / SFTP").FontSize(9).FontWeight(700).TextColor(tc.Accent)
						})
					})
					ui.Text(c, "Browse, edit, sync remote files and manage shells natively").
						FontSize(11).TextColor(tc.TextMuted)
				})
				closeBtn := ui.Row(c.Key("ssh-modal-close-btn")).Size(26, 26).Radius(6).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
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
			ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
				// Left Saved Hosts Sidebar (Termius-like host list)
				ui.Column(c).Width(230).FillHeight().Background(tc.Canvas).BorderWidth(0, 1, 0, 0).BorderColor(tc.Border).Children(func() {
					// Sidebar Top: Header + New Host button
					ui.Row(c).Height(36).Padding(0, 10).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
						ui.Text(c, "SAVED HOSTS").FontSize(10).FontWeight(700).LetterSpacing(0.5).TextColor(tc.TextMuted)
						if len(a.SSHSavedConfigs) > 0 {
							ui.Box(c).Width(4)
							ui.Row(c).Padding(1, 5).Radius(4).Background(tc.Text.Alpha(0.06)).Children(func() {
								ui.Text(c, fmt.Sprintf("%d", len(a.SSHSavedConfigs))).FontSize(9).TextColor(tc.TextMuted)
							})
						}
						ui.Box(c).Grow(1)
						newBtn := ui.Row(c.Key("new-ssh-host")).Height(22).Radius(4).Padding(0, 8).Gap(4).
							Background(tc.Accent.Alpha(0.12)).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
						newBtn.Children(func() {
							ui.Icon(c, icons.TablerPlus).Width(11).Height(11).TextColor(tc.Accent)
							ui.Text(c, "New").FontSize(10).FontWeight(600).TextColor(tc.Accent)
						})
						if newBtn.Hovered() {
							newBtn.Background(tc.Accent.Alpha(0.2))
						}
						if newBtn.Clicked() {
							a.ResetSSHForm()
						}
					})

					// Search Filter Input
					ui.Row(c).Height(34).Padding(0, 8).Gap(6).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.5)).Children(func() {
						ui.Icon(c, icons.TablerSearch).Width(12).Height(12).TextColor(tc.IconMuted)
						ui.TextInput(c, &a.SSHHostSearch).Placeholder("Search hosts...").Grow(1)
						if a.SSHHostSearch != "" {
							clrBtn := ui.Row(c.Key("clear-host-search")).Size(16, 16).Radius(8).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
							clrBtn.Children(func() {
								ui.Icon(c, icons.TablerX).Width(10).Height(10).TextColor(tc.IconMuted)
							})
							if clrBtn.Clicked() {
								a.SSHHostSearch = ""
							}
						}
					})

					// Filtered Host list
					q := strings.ToLower(strings.TrimSpace(a.SSHHostSearch))
					var matchedConfigs []remotefs.SSHConfig
					for _, cfg := range a.SSHSavedConfigs {
						if q == "" ||
							strings.Contains(strings.ToLower(cfg.Label), q) ||
							strings.Contains(strings.ToLower(cfg.Host), q) ||
							strings.Contains(strings.ToLower(cfg.User), q) ||
							strings.Contains(strings.ToLower(cfg.RemotePath), q) {
							matchedConfigs = append(matchedConfigs, cfg)
						}
					}

					ui.Scroll(c).Grow(1).Children(func() {
						ui.Column(c).Padding(6, 6).Gap(4).Children(func() {
							if len(matchedConfigs) == 0 {
								ui.Column(c).Padding(24, 10).AlignItems(ui.Center).Gap(6).Children(func() {
									ui.Icon(c, icons.TablerServer).Width(24).Height(24).TextColor(tc.IconMuted.Alpha(0.4))
									ui.Text(c, "No hosts found").FontSize(11).FontWeight(500).TextColor(tc.TextMuted)
									if q != "" {
										ui.Text(c, "Try a different search term").FontSize(10).TextColor(tc.TextMuted.Alpha(0.8))
									}
								})
							} else {
								for _, cfg := range matchedConfigs {
									itemCfg := cfg
									isSelected := (a.SSHSelectedConfigID == itemCfg.ID)
									isActive := strings.HasPrefix(a.Workspace.RootDir, "ssh://") &&
										(strings.Contains(a.Workspace.RootDir, itemCfg.ID) || strings.Contains(a.Workspace.RootDir, itemCfg.Host))

									card := ui.Column(c.Key("saved-ssh-"+itemCfg.ID)).Radius(6).
										Padding(7, 8).Gap(3).BorderWidth(1).Cursor(ui.CursorPointer)
									if isSelected {
										card.Background(tc.SurfaceRaised).BorderColor(tc.Accent)
									} else {
										card.Background(tc.Surface.Alpha(0.4)).BorderColor(tc.Border.Alpha(0.5))
										if card.Hovered() {
											card.Background(tc.SurfaceRaised.Alpha(0.6)).BorderColor(tc.Border)
										}
									}

									card.Children(func() {
										// Top line: Server icon + Label + Online dot
										ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
											iconBox := ui.Row(c).Size(20, 20).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
											if isActive {
												iconBox.Background(tc.Success.Alpha(0.18))
											} else {
												iconBox.Background(tc.Accent.Alpha(0.1))
											}
											iconBox.Children(func() {
												icColor := tc.Accent
												if isActive {
													icColor = tc.Success
												}
												ui.Icon(c, icons.TablerServer).Width(12).Height(12).TextColor(icColor)
											})

											titleTxt := itemCfg.Label
											if titleTxt == "" {
												titleTxt = itemCfg.Host
											}
											ui.Text(c, titleTxt).FontSize(11).FontWeight(600).TextColor(tc.Text).Grow(1).SingleLine()

											if isActive {
												ui.Box(c).Size(6, 6).Radius(3).Background(tc.Success)
											}
										})

										// Second line: user@host:port
										hostDisplay := fmt.Sprintf("%s@%s:%d", itemCfg.User, itemCfg.Host, itemCfg.Port)
										ui.Text(c, hostDisplay).FontSize(10).TextColor(tc.TextMuted).SingleLine()

										// Third line: Remote path badge if set
										if itemCfg.RemotePath != "" {
											ui.Row(c).Gap(3).AlignItems(ui.Center).Children(func() {
												ui.Icon(c, icons.TablerFolder).Width(10).Height(10).TextColor(tc.IconMuted)
												ui.Text(c, itemCfg.RemotePath).FontSize(9).TextColor(tc.TextMuted).SingleLine()
											})
										}

										// Termius 1-Click Action Buttons
										ui.Row(c).Gap(4).Margin(2, 0, 0, 0).AlignItems(ui.Center).Children(func() {
											connectBtn := ui.Row(c.Key("act-conn-"+itemCfg.ID)).Height(20).Radius(4).Padding(0, 6).Gap(3).
												Background(tc.Accent.Alpha(0.14)).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
											connectBtn.Children(func() {
												ui.Icon(c, icons.TablerPlug).Width(9).Height(9).TextColor(tc.Accent)
												ui.Text(c, "Connect").FontSize(9).FontWeight(600).TextColor(tc.Accent)
											})
											if connectBtn.Hovered() {
												connectBtn.Background(tc.Accent.Alpha(0.24))
											}
											if connectBtn.Clicked() {
												a.LoadSSHConfig(itemCfg)
												a.ConnectCurrentSSH()
											}

											shellBtn := ui.Row(c.Key("act-shell-"+itemCfg.ID)).Size(20, 20).Radius(4).
												Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border).
												AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
											shellBtn.Children(func() {
												ui.Icon(c, icons.TablerTerminal).Width(10).Height(10).TextColor(tc.TextMuted)
											})
											if shellBtn.Hovered() {
												shellBtn.Background(tc.Text.Alpha(0.08))
											}
											if shellBtn.Clicked() {
												a.connectSSHShell(itemCfg)
											}

											sftpBtn := ui.Row(c.Key("act-sftp-"+itemCfg.ID)).Size(20, 20).Radius(4).
												Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border).
												AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
											sftpBtn.Children(func() {
												// Uploads always use the ACTIVE connection, so a
												// non-active host first connects (spinner on the
												// button) and the uploader only opens on success.
												if a.sshSFTPBusyID == itemCfg.ID {
													ui.Spinner(c).Size(10, 10)
												} else {
													ui.Icon(c, icons.TablerUpload).Width(10).Height(10).TextColor(tc.TextMuted)
												}
											})
											if sftpBtn.Hovered() && a.sshSFTPBusyID != itemCfg.ID {
												sftpBtn.Background(tc.Text.Alpha(0.08))
											}
											if sftpBtn.Clicked() && a.sshSFTPBusyID != itemCfg.ID {
												a.connectAndOpenSFTP(itemCfg)
											}

											ui.Box(c).Grow(1)

											delBtn := ui.Row(c.Key("del-ssh-"+itemCfg.ID)).Size(20, 20).Radius(4).
												AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
											delBtn.Children(func() {
												ui.Icon(c, icons.TablerTrash).Width(10).Height(10).TextColor(tc.IconMuted)
											})
											if delBtn.Hovered() {
												delBtn.Background(tc.Error.Alpha(0.18))
											}
											if delBtn.Clicked() {
												// Destructive: confirm before removing the saved profile.
												title := itemCfg.Label
												if title == "" {
													title = itemCfg.Host
												}
												a.confirmDialogLabeled("Delete Saved Host?", fmt.Sprintf("“%s” will be removed from saved SSH hosts (including its stored credentials).", title), "Delete", func() {
													a.DeleteSSHConfig(itemCfg.ID)
												}, nil)
											}
										})
									})

									if card.Clicked() {
										a.LoadSSHConfig(itemCfg)
									}
								}
							}
						})
					})
				})

				// Right Form Container (Termius-like server configuration with fixed footer)
				ui.Column(c).Grow(1).FillHeight().Children(func() {
					// Form Top Bar
					ui.Row(c).Height(36).Padding(0, 18).AlignItems(ui.Center).
						Background(tc.Surface).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border.Alpha(0.6)).Children(func() {
						headerTitle := "New Remote Host"
						if a.SSHSelectedConfigID != "" {
							lbl := a.SSHLabel
							if lbl == "" {
								lbl = a.SSHHost
							}
							headerTitle = fmt.Sprintf("Config: %s", lbl)
						}
						ui.Text(c, headerTitle).FontSize(11).FontWeight(600).TextColor(tc.TextMuted).Grow(1)

						if a.SSHHost != "" {
							quickShellBtn := ui.Row(c.Key("quick-shell-btn")).Height(22).Radius(4).Padding(0, 8).Gap(4).
								Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border).AlignItems(ui.Center).Cursor(ui.CursorPointer)
							quickShellBtn.Children(func() {
								ui.Icon(c, icons.TablerTerminal).Width(11).Height(11).TextColor(tc.Accent)
								ui.Text(c, "Shell").FontSize(10).TextColor(tc.Text)
							})
							if quickShellBtn.Clicked() {
								p, _ := strconv.Atoi(a.SSHPortText)
								a.connectSSHShell(remotefs.SSHConfig{
									Host:       a.SSHHost,
									Port:       p,
									User:       a.SSHUser,
									AuthType:   a.SSHAuthType,
									KeyPath:    a.SSHKeyPath,
									RemotePath: a.SSHRemotePath,
								})
							}

							ui.Box(c).Width(6)

							quickSftpBtn := ui.Row(c.Key("quick-sftp-btn")).Height(22).Radius(4).Padding(0, 8).Gap(4).
								Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border).AlignItems(ui.Center).Cursor(ui.CursorPointer)
							quickSftpBtn.Children(func() {
								ui.Icon(c, icons.TablerUpload).Width(11).Height(11).TextColor(tc.Accent)
								ui.Text(c, "SFTP Upload").FontSize(10).TextColor(tc.Text)
							})
							if quickSftpBtn.Clicked() {
								a.OpenRemoteFileUploader(a.SSHRemotePath)
							}
						}
					})

					// Scrollable Form Body
					ui.Scroll(c).Grow(1).Children(func() {
						ui.Column(c).Padding(14, 18).Gap(10).Children(func() {
							// Error Banner
							if a.SSHErrorMsg != "" {
								ui.Row(c).Radius(6).Padding(8, 10).Gap(8).Background(tc.Error.Alpha(0.12)).
									BorderWidth(1).BorderColor(tc.Error.Alpha(0.3)).AlignItems(ui.Center).Children(func() {
									ui.Icon(c, icons.TablerAlertTriangle).Width(15).Height(15).TextColor(tc.Error)
									ui.Text(c, a.SSHErrorMsg).FontSize(11).TextColor(tc.Error).Grow(1)
								})
							}

							// Row 1: Host + Port
							ui.Row(c).Gap(10).Children(func() {
								ui.Column(c).Grow(3).Gap(4).Children(func() {
									ui.Text(c, "Host / IP Address *").FontSize(11).FontWeight(500).TextColor(tc.TextMuted)
									ui.TextInput(c, &a.SSHHost).Placeholder("vps.example.com or 192.168.1.50").Grow(1)
								})
								ui.Column(c).Width(90).Gap(4).Children(func() {
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
										tab := ui.Row(c.Key("auth-tab-" + id)).Height(28).Radius(6).Grow(1).
											BorderWidth(1).AlignItems(ui.Center).Justify(ui.Center).Gap(6).Cursor(ui.CursorPointer)
										if isCur {
											tab.Background(tc.Accent.Alpha(0.12)).BorderColor(tc.Accent)
										} else {
											tab.Background(tc.Canvas).BorderColor(tc.Border)
											if tab.Hovered() {
												tab.Background(tc.SurfaceRaised)
											}
										}
										tab.Children(func() {
											ic := ui.Icon(c, icon).Width(12).Height(12)
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
								ui.Column(c).Padding(10, 12).Radius(8).Gap(8).Background(tc.Canvas).
									BorderWidth(1).BorderColor(tc.Border).Children(func() {
									ui.Column(c).Gap(3).Children(func() {
										ui.Row(c).AlignItems(ui.Center).Children(func() {
											ui.Text(c, "Private Key Path").FontSize(10).FontWeight(600).TextColor(tc.TextMuted).Grow(1)
											browseKeyBtn := ui.Row(c.Key("browse-ssh-key")).Height(20).Radius(4).Padding(0, 6).Gap(3).
												Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border).AlignItems(ui.Center).Cursor(ui.CursorPointer)
											browseKeyBtn.Children(func() {
												ui.Icon(c, icons.TablerFolder).Width(10).Height(10).TextColor(tc.IconMuted)
												ui.Text(c, "Browse…").FontSize(9).TextColor(tc.Text)
											})
											if browseKeyBtn.Clicked() && a.Window != nil {
												win := a.Window
												go func() {
													paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
														Parent:   win,
														Title:    "Select SSH Private Key",
														Multiple: false,
													})
													if err == nil && len(paths) > 0 {
														// SSHKeyPath is rendered state: publish on the UI thread.
														a.runOnUI(func() {
															a.SSHKeyPath = paths[0]
															a.invalidateUI()
														})
													}
												}()
											}
										})
										ui.TextInput(c, &a.SSHKeyPath).Placeholder("~/.ssh/id_ed25519").Grow(1)
										ui.Row(c).Gap(4).Padding(2, 0).Children(func() {
											for _, kp := range []string{"~/.ssh/id_ed25519", "~/.ssh/id_rsa"} {
												val := kp
												chip := ui.Row(c.Key("key-chip-"+val)).Radius(3).Padding(1, 5).
													Background(tc.SurfaceRaised).BorderWidth(1).BorderColor(tc.Border).Cursor(ui.CursorPointer)
												chip.Children(func() {
													ui.Text(c, val).FontSize(9).TextColor(tc.TextMuted)
												})
												if chip.Clicked() {
													a.SSHKeyPath = val
												}
											}
										})
									})
									ui.Column(c).Gap(3).Children(func() {
										ui.Text(c, "Passphrase (Leave empty if unencrypted)").FontSize(10).FontWeight(600).TextColor(tc.TextMuted)
										ui.TextInput(c, &a.SSHKeyPassphrase).Placeholder("Optional passphrase").Password().Grow(1)
									})
								})
							case "password":
								ui.Column(c).Padding(10, 12).Radius(8).Gap(4).Background(tc.Canvas).
									BorderWidth(1).BorderColor(tc.Border).Children(func() {
									ui.Text(c, "SSH Password").FontSize(10).FontWeight(600).TextColor(tc.TextMuted)
									ui.TextInput(c, &a.SSHPassword).Placeholder("Enter SSH password").Password().Grow(1)
								})
							case "agent":
								ui.Row(c).Padding(12, 12).Radius(8).Background(tc.Canvas).
									BorderWidth(1).BorderColor(tc.Border).AlignItems(ui.Center).Gap(8).Children(func() {
									ui.Icon(c, icons.TablerServer).Width(16).Height(16).TextColor(tc.Accent)
									ui.Text(c, "Authenticates via local SSH agent ($SSH_AUTH_SOCK) with your unlocked keys.").
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
										chip := ui.Row(c.Key("qp-"+pathStr)).Radius(3).Padding(1, 5).Cursor(ui.CursorPointer)
										if isCur {
											chip.Background(tc.Accent)
										} else {
											chip.Background(tc.Canvas).BorderWidth(1).BorderColor(tc.Border)
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
						})
					})

					// FIXED FOOTER (PINNED AT BOTTOM OUTSIDE SCROLL)
					ui.Row(c).Height(50).Padding(0, 18).Background(tc.Surface).
						BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).AlignItems(ui.Center).Children(func() {
						if a.SSHIsConnecting {
							ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
								ui.Spinner(c).Size(12, 12).Label("connecting")
								ui.Text(c, "Connecting to host via SFTP...").FontSize(11).TextColor(tc.Accent)
							})
						}
						ui.Spacer(c)

						cancelBtn := ui.Row(c.Key("ssh-cancel-btn")).Height(30).Radius(6).Padding(0, 14).
							BorderWidth(1).BorderColor(tc.Border).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
						cancelBtn.Children(func() {
							ui.Text(c, "Cancel").FontSize(11).TextColor(tc.TextMuted)
						})
						if cancelBtn.Hovered() {
							cancelBtn.Background(tc.Text.Alpha(0.04))
						}
						if cancelBtn.Clicked() {
							// Cancelling abandons any in-flight connect (generation bump),
							// so it can no longer save or switch the workspace.
							a.cancelSSHModal()
						}

						ui.Box(c).Width(8)

						connectBtn := ui.Row(c.Key("ssh-connect-btn")).Height(30).Radius(6).Padding(0, 16).
							Background(tc.Accent).AlignItems(ui.Center).Justify(ui.Center).Gap(6).Cursor(ui.CursorPointer)
						connectBtn.Children(func() {
							label := "Connect & Open Workspace"
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
}

// renderSSHQuickMenuModal renders the popup remote actions drawer/menu for active SSH sessions.
func (a *AppUI) renderSSHQuickMenuModal(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	connID, remotePath, displayHost, _ := parseSSHWorkspaceURI(a.Workspace.RootDir)
	var activeConns []remotefs.ConnectionStatus
	if a.Engine != nil {
		activeConns = a.Engine.ListSSHConnections()
	}
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

	a.modal(c, &a.ShowSSHQuickMenu, func() {
		ui.Column(c).Width(480).Radius(12).Background(tc.Surface).
			BorderWidth(1).BorderColor(tc.Border).Shadow(0, 24, 64, -20, ui.RGBA(0, 0, 0, 0.55)).Clip().Children(func() {
			// Header
			ui.Row(c).Height(62).Padding(0, 16).AlignItems(ui.Center).
				Background(tc.Canvas).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
				ui.Row(c).Size(38, 38).Radius(8).Background(tc.Accent.Alpha(0.15)).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerServer).Width(20).Height(20).TextColor(tc.Accent)
				})
				ui.Box(c).Width(10)
				ui.Column(c).Grow(1).Children(func() {
					isConnected := a.Engine != nil && a.Engine.IsSSHConnected(connID)
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, labelDisplay).FontSize(13).FontWeight(600).TextColor(tc.Text)
						if isConnected {
							ui.Row(c).Padding(1, 5).Radius(4).Background(tc.Success.Alpha(0.15)).Gap(4).AlignItems(ui.Center).Children(func() {
								ui.Box(c).Size(5, 5).Radius(2.5).Background(tc.Success)
								ui.Text(c, "Connected").FontSize(9).FontWeight(600).TextColor(tc.Success)
							})
						} else {
							ui.Row(c).Padding(1, 5).Radius(4).Background(tc.Warning.Alpha(0.15)).Gap(4).AlignItems(ui.Center).Children(func() {
								ui.Box(c).Size(5, 5).Radius(2.5).Background(tc.Warning)
								ui.Text(c, "Disconnected").FontSize(9).FontWeight(600).TextColor(tc.Warning)
							})
						}
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

				actionItem(icons.TablerRefresh, "Reconnect SSH Connection", "Restore connection if network was dropped or interrupted", "", false, func() {
					a.ShowSSHQuickMenu = false
					a.ReconnectSSH()
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

				actionItem(icons.TablerUpload, "Upload Files to Remote", "Pick local files and upload them over SFTP", "", false, func() {
					a.ShowSSHQuickMenu = false
					a.OpenRemoteFileUploader(remotePath)
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

// ── Remote upload queue ────────────────────────────────────────────────────
// The queue is appended to by Browse dialog goroutines and rewritten by the
// upload goroutine; every access goes through these mutex-guarded helpers.

func (a *AppUI) enqueueRemoteUpload(item RemoteUploadItem) {
	a.uploadQueueMu.Lock()
	a.RemoteUploadQueue = append(a.RemoteUploadQueue, item)
	a.uploadQueueMu.Unlock()
}

func (a *AppUI) remoteUploadQueueSnapshot() []RemoteUploadItem {
	a.uploadQueueMu.Lock()
	defer a.uploadQueueMu.Unlock()
	out := make([]RemoteUploadItem, len(a.RemoteUploadQueue))
	copy(out, a.RemoteUploadQueue)
	return out
}

func (a *AppUI) remoteUploadQueueLen() int {
	a.uploadQueueMu.Lock()
	defer a.uploadQueueMu.Unlock()
	return len(a.RemoteUploadQueue)
}

func (a *AppUI) setRemoteUploadStatus(id, status, errMsg string) {
	a.uploadQueueMu.Lock()
	for i := range a.RemoteUploadQueue {
		if a.RemoteUploadQueue[i].ID == id {
			a.RemoteUploadQueue[i].Status = status
			a.RemoteUploadQueue[i].ErrorMsg = errMsg
			break
		}
	}
	a.uploadQueueMu.Unlock()
}

func (a *AppUI) removeRemoteUpload(id string) {
	a.uploadQueueMu.Lock()
	for i := range a.RemoteUploadQueue {
		if a.RemoteUploadQueue[i].ID == id {
			a.RemoteUploadQueue = append(a.RemoteUploadQueue[:i], a.RemoteUploadQueue[i+1:]...)
			break
		}
	}
	a.uploadQueueMu.Unlock()
}

func (a *AppUI) clearRemoteUploadQueue() {
	a.uploadQueueMu.Lock()
	a.RemoteUploadQueue = nil
	a.uploadQueueMu.Unlock()
}

// renderRemoteFileUploaderModal renders the full SFTP file/folder upload modal (RemoteFileUploaderModal parity).
func (a *AppUI) renderRemoteFileUploaderModal(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	if c.Shortcut(0, ui.KeyEscape) {
		a.ShowRemoteUploadModal = false
	}

	a.modal(c, &a.ShowRemoteUploadModal, func() {
		ui.Column(c).Width(600).Radius(12).Background(tc.Surface).
			BorderWidth(1).BorderColor(tc.Border).Shadow(0, 24, 64, -20, ui.RGBA(0, 0, 0, 0.55)).Clip().Children(func() {
			// Header
			ui.Row(c).Height(56).Padding(0, 16).AlignItems(ui.Center).
				Background(tc.Canvas).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
				ui.Row(c).Size(32, 32).Radius(8).Background(tc.Accent.Alpha(0.12)).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerUpload).Width(16).Height(16).TextColor(tc.Accent)
				})
				ui.Box(c).Width(10)
				ui.Column(c).Grow(1).Children(func() {
					ui.Text(c, "Remote File Uploader").FontSize(13).FontWeight(600).TextColor(tc.Text)
					ui.Text(c, "Upload local files or directories to remote server via SFTP").FontSize(10).TextColor(tc.TextMuted)
				})
				closeBtn := ui.Row(c).Size(24, 24).Radius(6).AlignItems(ui.Center).Justify(ui.Center)
				closeBtn.Children(func() {
					ui.Icon(c, icons.TablerX).Width(14).Height(14).TextColor(tc.IconMuted)
				})
				if closeBtn.Hovered() {
					closeBtn.Background(tc.Text.Alpha(0.08))
				}
				if closeBtn.Clicked() {
					a.ShowRemoteUploadModal = false
				}
			})

			// Target Directory input
			ui.Column(c).Padding(12, 16).Gap(4).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
				ui.Text(c, "Remote Target Directory").FontSize(11).FontWeight(600).TextColor(tc.Text)
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerFolder).Width(14).Height(14).TextColor(tc.Accent)
					ui.TextInput(c, &a.RemoteUploadTargetDir).Placeholder("/remote/path/destination").Grow(1)
				})
			})

			// Live upload status line (progress from the active SFTP run).
			if a.SSHUploadStatus != "" {
				ui.Row(c).Padding(0, 16, 8, 16).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.TablerUpload).Width(12).Height(12).TextColor(tc.Accent)
					ui.Text(c, a.SSHUploadStatus).FontSize(11).TextColor(tc.TextMuted).Grow(1)
				})
			}

			// Add Files / Browse area
			ui.Column(c).Padding(12, 16).Gap(8).Children(func() {
				dropBox := ui.Column(c).Radius(8).Padding(16, 20).Background(tc.Canvas.Alpha(0.5)).
					BorderWidth(1).BorderColor(tc.Border.Alpha(0.8)).AlignItems(ui.Center).Gap(8)
				dropBox.Children(func() {
					ui.Icon(c, icons.TablerUpload).Width(24).Height(24).TextColor(tc.IconMuted)
					ui.Text(c, "Select local files or whole directories to upload").FontSize(11).TextColor(tc.TextMuted)
					ui.Row(c).Gap(8).Children(func() {
						addFilesBtn := ui.Row(c).Height(28).Radius(6).Padding(0, 10).Background(tc.SurfaceRaised).
							BorderWidth(1).BorderColor(tc.Border).AlignItems(ui.Center).Gap(6)
						addFilesBtn.Children(func() {
							ui.Icon(c, icons.TablerFileText).Width(12).Height(12).TextColor(tc.Text)
							ui.Text(c, "Browse Files…").FontSize(11).FontWeight(500).TextColor(tc.Text)
						})
						if addFilesBtn.Hovered() {
							addFilesBtn.Background(tc.Text.Alpha(0.06))
						}
						if addFilesBtn.Clicked() && a.Window != nil {
							win := a.Window
							go func() {
								paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
									Parent:   win,
									Title:    "Select Files to Upload",
									Multiple: true,
								})
								if err == nil && len(paths) > 0 {
									for _, p := range paths {
										info, _ := os.Stat(p)
										var sz int64
										if info != nil {
											sz = info.Size()
										}
										name := filepath.Base(p)
										dest := filepath.Join(a.RemoteUploadTargetDir, name)
										a.enqueueRemoteUpload(RemoteUploadItem{
											ID:        fmt.Sprintf("up-%d-%s", time.Now().UnixNano(), name),
											LocalPath: p,
											Name:      name,
											Size:      sz,
											DestPath:  dest,
											Status:    "pending",
										})
									}
									win.Invalidate()
								}
							}()
						}

						addFolderBtn := ui.Row(c).Height(28).Radius(6).Padding(0, 10).Background(tc.SurfaceRaised).
							BorderWidth(1).BorderColor(tc.Border).AlignItems(ui.Center).Gap(6)
						addFolderBtn.Children(func() {
							ui.Icon(c, icons.TablerFolder).Width(12).Height(12).TextColor(tc.Text)
							ui.Text(c, "Browse Folder…").FontSize(11).FontWeight(500).TextColor(tc.Text)
						})
						if addFolderBtn.Hovered() {
							addFolderBtn.Background(tc.Text.Alpha(0.06))
						}
						if addFolderBtn.Clicked() && a.Window != nil {
							win := a.Window
							go func() {
								dirPaths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
									Parent:    win,
									Title:     "Select Folder to Upload",
									Directory: true,
								})
								if err == nil && len(dirPaths) > 0 && dirPaths[0] != "" {
									dirPath := dirPaths[0]
									name := filepath.Base(dirPath)
									dest := filepath.Join(a.RemoteUploadTargetDir, name)
									a.enqueueRemoteUpload(RemoteUploadItem{
										ID:        fmt.Sprintf("up-%d-%s", time.Now().UnixNano(), name),
										LocalPath: dirPath,
										Name:      name,
										Size:      0,
										DestPath:  dest,
										Status:    "pending",
									})
									win.Invalidate()
								}
							}()
						}
					})
				})

				// Queue items (rendered from a mutex-guarded snapshot so Browse
				// dialogs appending concurrently can never race the render).
				queueSnap := a.remoteUploadQueueSnapshot()
				if len(queueSnap) == 0 {
					ui.Row(c).Padding(12, 0).Justify(ui.Center).Children(func() {
						ui.Text(c, "Upload queue is empty").FontSize(11).TextColor(tc.TextMuted)
					})
				} else {
					ui.Row(c).AlignItems(ui.Center).Children(func() {
						ui.Text(c, fmt.Sprintf("Queue (%d items)", len(queueSnap))).FontSize(11).FontWeight(600).TextColor(tc.Text).Grow(1)
						clearBtn := ui.Row(c).Padding(2, 6).Radius(4)
						clearBtn.Children(func() {
							ui.Text(c, "Clear all").FontSize(10).TextColor(tc.TextMuted)
						})
						if clearBtn.Hovered() {
							clearBtn.Background(tc.Text.Alpha(0.06))
						}
						if clearBtn.Clicked() && !a.RemoteUploadIsUploading {
							a.clearRemoteUploadQueue()
						}
					})

					scroll := ui.Scroll(c).MaxHeight(200)
					scroll.Children(func() {
						ui.Column(c).Gap(4).Children(func() {
							for _, item := range queueSnap {
								it := item
								row := ui.Row(c.Key("upq-"+it.ID)).Height(32).Padding(0, 8).Radius(6).
									Background(tc.Canvas.Alpha(0.6)).AlignItems(ui.Center).Gap(8)
								row.Children(func() {
									icons.RenderFileIcon(c, it.Name, 14, tc.IconMuted)
									ui.Text(c, it.Name).FontSize(11).Font("monospace").TextColor(tc.Text).SingleLine().Grow(1)
									if it.Size > 0 {
										ui.Text(c, formatBytes(it.Size)).FontSize(10).TextColor(tc.TextMuted)
									}
									// Status pill
									switch it.Status {
									case "uploading":
										ui.Box(c).Radius(4).Padding(1, 5).Background(tc.Accent.Alpha(0.15)).Children(func() {
											ui.Text(c, "Uploading…").FontSize(9).FontWeight(600).TextColor(tc.Accent)
										})
									case "completed":
										ui.Box(c).Radius(4).Padding(1, 5).Background(tc.Success.Alpha(0.15)).Children(func() {
											ui.Text(c, "Uploaded").FontSize(9).FontWeight(600).TextColor(tc.Success)
										})
									case "error":
										ui.Box(c).Radius(4).Padding(1, 5).Background(tc.Error.Alpha(0.15)).Children(func() {
											ui.Text(c, "Failed").FontSize(9).FontWeight(600).TextColor(tc.Error)
										})
									default:
										ui.Box(c).Radius(4).Padding(1, 5).Background(tc.Text.Alpha(0.06)).Children(func() {
											ui.Text(c, "Pending").FontSize(9).TextColor(tc.TextMuted)
										})
									}
									if !a.RemoteUploadIsUploading {
										rmBtn := ui.Row(c).Size(18, 18).Radius(3).AlignItems(ui.Center).Justify(ui.Center)
										rmBtn.Children(func() {
											ui.Icon(c, icons.TablerX).Width(11).Height(11).TextColor(tc.IconMuted)
										})
										if rmBtn.Hovered() {
											rmBtn.Background(tc.Text.Alpha(0.08))
										}
										if rmBtn.Clicked() {
											a.removeRemoteUpload(it.ID)
										}
									}
								})
							}
						})
					})
				}
			})

			// Footer Actions
			ui.Row(c).Height(48).Padding(0, 16).AlignItems(ui.Center).
				Background(tc.Canvas).BorderWidth(1, 0, 0, 0).BorderColor(tc.Border).Children(func() {
				cancelBtn := ui.Row(c).Height(30).Radius(6).Padding(0, 12).
					BorderWidth(1).BorderColor(tc.Border).AlignItems(ui.Center).Justify(ui.Center)
				cancelBtn.Children(func() {
					ui.Text(c, "Close").FontSize(11).TextColor(tc.Text)
				})
				if cancelBtn.Hovered() {
					cancelBtn.Background(tc.Text.Alpha(0.05))
				}
				if cancelBtn.Clicked() {
					a.ShowRemoteUploadModal = false
				}

				ui.Box(c).Grow(1)

				queueSnap := a.remoteUploadQueueSnapshot()
				if len(queueSnap) > 0 {
					uploadBtn := ui.Row(c).Height(30).Radius(6).Padding(0, 14).
						Background(tc.Accent).AlignItems(ui.Center).Justify(ui.Center).Gap(6)
					uploadBtn.Children(func() {
						if a.RemoteUploadIsUploading {
							ui.Spinner(c).Size(12, 12).Label("Uploading")
							ui.Text(c, "Uploading…").FontSize(11).FontWeight(600).TextColor(tc.Canvas)
						} else {
							ui.Icon(c, icons.TablerUpload).Width(12).Height(12).TextColor(tc.Canvas)
							ui.Text(c, fmt.Sprintf("Upload All (%d)", len(queueSnap))).FontSize(11).FontWeight(600).TextColor(tc.Canvas)
						}
					})
					if !a.RemoteUploadIsUploading && uploadBtn.Clicked() && a.Engine != nil && a.Engine.RemoteFS != nil {
						a.RemoteUploadIsUploading = true
						a.SSHUploadStatus = fmt.Sprintf("Uploading %d item(s)…", len(queueSnap))
						targetDir := a.RemoteUploadTargetDir
						// Iterate a snapshot: Browse dialogs may append to the
						// live queue while this goroutine runs.
						pending := a.remoteUploadQueueSnapshot()
						go func() {
							okCount, errCount := 0, 0
							for _, item := range pending {
								a.setRemoteUploadStatus(item.ID, "uploading", "")
								a.invalidateUI()
								dest := path.Join(targetDir, item.Name)
								err := a.Engine.RemoteFS.UploadFileToRemote(item.LocalPath, dest)
								if err != nil {
									a.setRemoteUploadStatus(item.ID, "error", err.Error())
									errCount++
								} else {
									a.setRemoteUploadStatus(item.ID, "completed", "")
									okCount++
								}
								done := okCount + errCount
								total := len(pending)
								a.runOnUI(func() {
									a.SSHUploadStatus = fmt.Sprintf("Uploading… %d/%d done", done, total)
									a.invalidateUI()
								})
							}
							a.runOnUI(func() {
								a.RemoteUploadIsUploading = false
								if errCount > 0 {
									a.SSHUploadStatus = fmt.Sprintf("Uploaded %d item(s), %d failed", okCount, errCount)
								} else {
									a.SSHUploadStatus = fmt.Sprintf("Uploaded %d item(s)", okCount)
								}
								a.Workspace.Refresh()
								a.invalidateUI()
							})
						}()
					}
				}
			})
		})
	})
}

func formatBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	} else if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(bytes)/(1024*1024*1024))
}
