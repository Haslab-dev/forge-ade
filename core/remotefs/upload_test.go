package remotefs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseRemotePath(t *testing.T) {
	mgr := NewManager()

	// Unregistered / local paths
	_, _, isRemote := mgr.ParseRemotePath("/var/www/test")
	if isRemote {
		t.Fatalf("expected isRemote=false for local path")
	}

	_, _, isRemote = mgr.ParseRemotePath("ssh://non-existent/var/www")
	if isRemote {
		t.Fatalf("expected isRemote=false for non-existent connection")
	}

	// Registered connection
	client := &Client{
		config: SSHConfig{
			ID:   "conn-1",
			Host: "127.0.0.1",
			Port: 22,
			User: "test",
		},
	}
	mgr.clients["conn-1"] = client

	c, subPath, isRemote := mgr.ParseRemotePath("ssh://conn-1/var/www/site")
	if !isRemote {
		t.Fatalf("expected isRemote=true for registered connection")
	}
	if c != client {
		t.Fatalf("expected client to match registered client")
	}
	if subPath != "/var/www/site" {
		t.Fatalf("expected subPath /var/www/site, got %s", subPath)
	}
}

func TestUploadFileToRemote_LocalFileMissing(t *testing.T) {
	mgr := NewManager()
	client := &Client{
		config: SSHConfig{
			ID: "conn-1",
		},
	}
	mgr.clients["conn-1"] = client

	err := mgr.UploadFileToRemote("/non/existent/path/on/local.txt", "ssh://conn-1/remote/test.txt")
	if err == nil {
		t.Fatalf("expected error for non-existent local file")
	}
}

func TestUploadFilesToRemote_LocalFileMissing(t *testing.T) {
	mgr := NewManager()
	client := &Client{
		config: SSHConfig{
			ID: "conn-1",
		},
	}
	mgr.clients["conn-1"] = client

	_, err := mgr.UploadFilesToRemote([]string{"/non/existent/local.txt"}, "ssh://conn-1/remote")
	if err == nil {
		t.Fatalf("expected error for non-existent local file")
	}
}

func TestCopyPathRouting_LocalToRemoteMissingLocal(t *testing.T) {
	mgr := NewManager()
	client := &Client{
		config: SSHConfig{
			ID: "conn-1",
		},
	}
	mgr.clients["conn-1"] = client

	isHandled, err := mgr.CopyPath("/path/not/exist.txt", "ssh://conn-1/remote/file.txt")
	if !isHandled {
		t.Fatalf("expected isHandled=true for local-to-remote CopyPath")
	}
	if err == nil {
		t.Fatalf("expected error for missing local file")
	}
}

func TestUploadFileToRemote_ActiveClientFallback(t *testing.T) {
	mgr := NewManager()
	tmpFile := filepath.Join(t.TempDir(), "test.txt")
	_ = os.WriteFile(tmpFile, []byte("hello"), 0644)

	// No active client
	err := mgr.UploadFileToRemote(tmpFile, "/remote/path")
	if err == nil {
		t.Fatalf("expected error when no active client is set")
	}
}
