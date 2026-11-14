package agentsession

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeWorkspacePath(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"   ", ""},
		{".", ""},
		{"/", ""},
		{"general", ""},
		{"General", ""},
		{filepath.Join(fakeHome, ".forge-ade", "general"), ""},
		{fakeHome, ""},
		{fakeHome + "/", ""},
		{"/work/alpha/", "/work/alpha"},
		{"/work/alpha/sub", "/work/alpha/sub"},
	}

	for _, tc := range cases {
		if got := NormalizeWorkspacePath(tc.in); got != tc.want {
			t.Errorf("NormalizeWorkspacePath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestTempTestPathIsolation(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	tmpWs := t.TempDir()
	sessJSON := fmt.Sprintf(`{"id":"sess-temp-1","title":"Temp task","workspacePath":%q}`, tmpWs)
	if err := SaveSessionDisk(sessJSON, tmpWs); err != nil {
		t.Fatalf("SaveSessionDisk failed: %v", err)
	}

	// Must live under <tmpWs>/.forge/sessions and NEVER under ~/.forge/sessions.
	expectedFile := filepath.Join(tmpWs, ".forge", "sessions", "sess-temp-1.json")
	if _, err := os.Stat(expectedFile); err != nil {
		t.Fatalf("expected isolated temp session file at %s: %v", expectedFile, err)
	}
	if entries, err := os.ReadDir(filepath.Join(fakeHome, ".forge", "sessions")); err == nil && len(entries) > 0 {
		t.Fatalf("expected ~/.forge/sessions to remain untouched for temp workspace, found %d entries", len(entries))
	}

	loaded, err := LoadSessionsDisk(tmpWs)
	if err != nil {
		t.Fatalf("LoadSessionsDisk failed: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 loaded session, got %d", len(loaded))
	}

	if err := DeleteSessionDisk("sess-temp-1", tmpWs); err != nil {
		t.Fatalf("DeleteSessionDisk failed: %v", err)
	}
	if _, err := os.Stat(expectedFile); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be removed after delete, err=%v", expectedFile, err)
	}
}

func TestDeleteProjectSessionsDisk(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	wsAlpha := "/non-temp-projects/alpha"
	wsBeta := "/non-temp-projects/beta"

	if err := SaveSessionDisk(fmt.Sprintf(`{"id":"s-a1","title":"A1","workspacePath":%q}`, wsAlpha), wsAlpha); err != nil {
		t.Fatalf("SaveSessionDisk alpha: %v", err)
	}
	if err := SaveSessionDisk(fmt.Sprintf(`{"id":"s-b1","title":"B1","workspacePath":%q}`, wsBeta), wsBeta); err != nil {
		t.Fatalf("SaveSessionDisk beta: %v", err)
	}

	if err := DeleteProjectSessionsDisk("alpha", wsAlpha); err != nil {
		t.Fatalf("DeleteProjectSessionsDisk failed: %v", err)
	}

	alphaDir := filepath.Join(fakeHome, ".forge", "sessions", "alpha")
	if _, err := os.Stat(alphaDir); !os.IsNotExist(err) {
		t.Fatalf("expected alpha sessions dir %s to be removed, err=%v", alphaDir, err)
	}
	betaFile := filepath.Join(fakeHome, ".forge", "sessions", "beta", "s-b1.json")
	if _, err := os.Stat(betaFile); err != nil {
		t.Fatalf("expected beta session %s to remain intact: %v", betaFile, err)
	}
}
