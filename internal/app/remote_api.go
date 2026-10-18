package app

import (
	"fmt"
	"log"

	"github.com/hasdev/forge-ade/internal/remotefs"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// ---------------------------------------------------------------------------
// SSH / Remote SFTP API
// ---------------------------------------------------------------------------

// ConnectSSH establishes a remote SSH/SFTP connection.
func (a *App) ConnectSSH(cfg remotefs.SSHConfig) (*remotefs.ConnectionStatus, error) {
	return a.remoteFS.Connect(cfg)
}

// DisconnectSSH closes an active remote SSH/SFTP session.
func (a *App) DisconnectSSH(connID string) error {
	return a.remoteFS.Disconnect(connID)
}

// ListSSHConnections lists all active remote SSH/SFTP sessions.
func (a *App) ListSSHConnections() []remotefs.ConnectionStatus {
	return a.remoteFS.ListConnections()
}

// OpenSSHWorkspace connects to SSH and sets the active workspace to the remote directory.
func (a *App) OpenSSHWorkspace(cfg remotefs.SSHConfig) (*remotefs.ConnectionStatus, error) {
	status, err := a.remoteFS.Connect(cfg)
	if err != nil {
		return status, err
	}
	remoteURI := fmt.Sprintf("ssh://%s%s", status.ID, status.RemotePath)
	if _, err := a.OpenFolder(remoteURI); err != nil {
		log.Printf("[ssh] connected but failed to open remote workspace %s: %v", remoteURI, err)
	}
	return status, nil
}

// UploadFileToRemote uploads a local file or directory to a remote destination URI (ssh://<connID>/<path>).
func (a *App) UploadFileToRemote(localPath, remoteURI string) error {
	return a.remoteFS.UploadFileToRemote(localPath, remoteURI)
}

// UploadFilesToRemote uploads multiple local files to a remote destination directory URI.
func (a *App) UploadFilesToRemote(localPaths []string, remoteDirURI string) ([]string, error) {
	return a.remoteFS.UploadFilesToRemote(localPaths, remoteDirURI)
}

// OpenUploadFileDialog opens a native multiple-file picker dialog and returns selected local file paths.
func (a *App) OpenUploadFileDialog() ([]string, error) {
	app := application.Get()
	if app == nil {
		return nil, fmt.Errorf("app not initialized")
	}
	return app.Dialog.OpenFile().
		CanChooseFiles(true).
		CanChooseDirectories(false).
		SetTitle("Select Files to Upload").
		PromptForMultipleSelection()
}

// OpenUploadFolderDialog opens a native directory picker dialog and returns selected local directory path.
func (a *App) OpenUploadFolderDialog() (string, error) {
	app := application.Get()
	if app == nil {
		return "", fmt.Errorf("app not initialized")
	}
	return app.Dialog.OpenFile().
		CanChooseDirectories(true).
		CanChooseFiles(false).
		SetTitle("Select Folder to Upload").
		PromptForSingleSelection()
}
