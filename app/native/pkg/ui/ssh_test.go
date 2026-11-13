package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hasdev/forge-ade/app/native/pkg/engine"
	"github.com/hasdev/forge-ade/core/remotefs"
)

func TestParseSSHWorkspaceURI(t *testing.T) {
	connID, remotePath, displayHost, folderName := parseSSHWorkspaceURI("ssh://vps-prod/var/www/my-app")
	if connID != "vps-prod" {
		t.Fatalf("expected connID vps-prod, got %s", connID)
	}
	if remotePath != "/var/www/my-app" {
		t.Fatalf("expected remotePath /var/www/my-app, got %s", remotePath)
	}
	if folderName != "my-app" {
		t.Fatalf("expected folderName my-app, got %s", folderName)
	}
	if displayHost != "vps-prod" {
		t.Fatalf("expected displayHost vps-prod, got %s", displayHost)
	}

	// Root path
	connID, remotePath, _, folderName = parseSSHWorkspaceURI("ssh://host-123/")
	if connID != "host-123" {
		t.Fatalf("expected connID host-123, got %s", connID)
	}
	if remotePath != "/" {
		t.Fatalf("expected remotePath /, got %s", remotePath)
	}
	if folderName != "remote-root" {
		t.Fatalf("expected folderName remote-root, got %s", folderName)
	}
}

func TestSSHSavedConfigsPersistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "forge-ssh-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	eng := engine.NewEngine(tmpDir)
	app := NewAppUI(eng, tmpDir)

	if len(app.SSHSavedConfigs) != 0 {
		t.Fatalf("expected 0 initial configs, got %d", len(app.SSHSavedConfigs))
	}

	cfg := remotefs.SSHConfig{
		ID:         "ssh-test-1",
		Label:      "Staging VPS",
		Host:       "192.168.1.100",
		Port:       2222,
		User:       "deploy",
		AuthType:   "key_file",
		KeyPath:    "~/.ssh/staging",
		RemotePath: "/srv/app",
	}

	app.SaveSSHConfig(cfg)
	if len(app.SSHSavedConfigs) != 1 {
		t.Fatalf("expected 1 saved config, got %d", len(app.SSHSavedConfigs))
	}

	// Verify file was written to disk
	storagePath := filepath.Join(tmpDir, sshSavedHostsFile)
	if _, err := os.Stat(storagePath); os.IsNotExist(err) {
		t.Fatalf("storage file %s was not written", storagePath)
	}

	// Load into a new AppUI
	app2 := NewAppUI(eng, tmpDir)
	app2.LoadSavedSSHConfigs()
	if len(app2.SSHSavedConfigs) != 1 {
		t.Fatalf("expected 1 loaded config in app2, got %d", len(app2.SSHSavedConfigs))
	}
	if app2.SSHSavedConfigs[0].Label != "Staging VPS" {
		t.Fatalf("expected Staging VPS, got %s", app2.SSHSavedConfigs[0].Label)
	}

	// Delete config
	app.DeleteSSHConfig("ssh-test-1")
	if len(app.SSHSavedConfigs) != 0 {
		t.Fatalf("expected 0 configs after delete, got %d", len(app.SSHSavedConfigs))
	}
}

func TestSSHModalControlsAndForm(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "forge-ssh-modal-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	eng := engine.NewEngine(tmpDir)
	app := NewAppUI(eng, tmpDir)

	app.OpenSSHModal()
	if !app.ShowSSHModal {
		t.Fatal("expected ShowSSHModal to be true")
	}

	// Form defaults
	if app.SSHPort != 22 {
		t.Fatalf("expected default port 22, got %d", app.SSHPort)
	}
	if app.SSHAuthType != "key_file" {
		t.Fatalf("expected default authType key_file, got %s", app.SSHAuthType)
	}

	// Load config
	app.LoadSSHConfig(remotefs.SSHConfig{
		ID:         "test-id",
		Label:      "Dev Server",
		Host:       "dev.local",
		Port:       2200,
		User:       "ubuntu",
		AuthType:   "password",
		Password:   "secret",
		RemotePath: "/home/ubuntu/app",
	})

	if app.SSHHost != "dev.local" || app.SSHPort != 2200 || app.SSHAuthType != "password" {
		t.Fatalf("form fields not populated correctly: %+v", app)
	}

	// Disconnect SSH
	app.switchWorkspace("ssh://test-conn/var/www")
	if !app.Workspace.IsRemote() && app.Workspace.RootDir != "ssh://test-conn/var/www" {
		t.Fatalf("expected remote workspace root, got %s", app.Workspace.RootDir)
	}

	app.DisconnectSSH()
	if app.Workspace.RootDir == "ssh://test-conn/var/www" {
		t.Fatalf("expected local workspace after DisconnectSSH, got %s", app.Workspace.RootDir)
	}
}
