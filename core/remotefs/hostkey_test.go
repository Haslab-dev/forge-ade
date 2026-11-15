package remotefs

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func testPublicKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pub, err := ssh.NewPublicKey(priv.Public())
	if err != nil {
		t.Fatalf("wrap public key: %v", err)
	}
	return pub
}

func TestHostKeyPathSanitizesHost(t *testing.T) {
	got := hostKeyPath("/data/ssh_host_keys", "My.Server Example:com", 2222)
	if strings.Contains(got, " ") || strings.Contains(got, ":") {
		t.Fatalf("unsafe characters in path %q", got)
	}
	if filepath.Dir(got) != "/data/ssh_host_keys" || !strings.HasSuffix(got, "_2222.key") {
		t.Fatalf("unexpected path %q", got)
	}
	if got != hostKeyPath("/data/ssh_host_keys", "my.server example:com", 2222) {
		t.Error("host path must be case-insensitive stable")
	}
}

func TestTOFUHostKeyCallback(t *testing.T) {
	dir := t.TempDir()
	cb := tofuHostKeyCallback(dir, "server.example.com", 22)
	key := testPublicKey(t)

	// First contact: pins the key.
	if err := cb("server.example.com", &net.TCPAddr{}, key); err != nil {
		t.Fatalf("first connect must pin the key, got error: %v", err)
	}

	keyPath := hostKeyPath(dir, "server.example.com", 22)
	data, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("pinned key file missing: %v", err)
	}
	want := formatHostKey(key)
	if strings.TrimSpace(string(data)) != want {
		t.Fatalf("pinned content %q, want %q", strings.TrimSpace(string(data)), want)
	}
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("pinned key file mode %o, want 600", perm)
	}

	// Same key on later connects: accepted.
	if err := cb("server.example.com", &net.TCPAddr{}, key); err != nil {
		t.Fatalf("same key must be accepted, got: %v", err)
	}

	// Different key: hard error naming the file to delete.
	other := testPublicKey(t)
	err = cb("server.example.com", &net.TCPAddr{}, other)
	if err == nil {
		t.Fatal("mismatched key must fail")
	}
	if !strings.Contains(err.Error(), keyPath) {
		t.Errorf("mismatch error must name the pinned file %s: %v", keyPath, err)
	}
	if !strings.Contains(err.Error(), "delete") {
		t.Errorf("mismatch error must tell the user how to re-trust: %v", err)
	}
}

func TestTOFUPerHostIsolation(t *testing.T) {
	dir := t.TempDir()
	key := testPublicKey(t)

	if err := tofuHostKeyCallback(dir, "a.example.com", 22)("", &net.TCPAddr{}, key); err != nil {
		t.Fatal(err)
	}
	if err := tofuHostKeyCallback(dir, "b.example.com", 22)("", &net.TCPAddr{}, key); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Fatalf("expected one pinned key per host, got %d files", len(entries))
	}
}

func TestDialRefusesMissingHostKeyStore(t *testing.T) {
	cfg := SSHConfig{Host: "localhost", Port: 22, User: "u", AuthType: "password", Password: "p"}
	_, err := Dial(cfg, "")
	if err == nil || !strings.Contains(err.Error(), "host key") {
		t.Fatalf("Dial without a host key store must be refused, got: %v", err)
	}
}

func TestFormatHostKeyShape(t *testing.T) {
	key := testPublicKey(t)
	parts := strings.Fields(formatHostKey(key))
	if len(parts) != 2 {
		t.Fatalf("expected \"keytype base64key\", got %q", formatHostKey(key))
	}
	if parts[0] != key.Type() {
		t.Errorf("keytype %q, want %q", parts[0], key.Type())
	}
}
