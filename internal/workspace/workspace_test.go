package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewTemporary(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir) // relative input must resolve against the process cwd

	ws := NewTemporary("proj")
	abs := filepath.Join(dir, "proj")

	if ws.Version != Version {
		t.Errorf("Version = %d; want %d", ws.Version, Version)
	}
	if ws.Version != 1 {
		t.Errorf("Version = %d; want the literal 1 (file format contract)", ws.Version)
	}
	if ws.Name != "proj" {
		t.Errorf("Name = %q; want %q", ws.Name, "proj")
	}
	if len(ws.Folders) != 1 || ws.Folders[0] != abs {
		t.Errorf("Folders = %v; want [%s]", ws.Folders, abs)
	}
	if !ws.IsTemporary {
		t.Error("IsTemporary = false; want true")
	}
	if ws.Settings.Theme != "dark" {
		t.Errorf("Theme = %q; want \"dark\"", ws.Settings.Theme)
	}
	if !ws.Git.AutoFetch {
		t.Error("AutoFetch = false; want true")
	}
	if ws.Agents == nil || len(ws.Agents) != 0 {
		t.Error("Agents must be a non-nil empty map")
	}
	if ws.Terminals == nil || len(ws.Terminals) != 0 {
		t.Error("Terminals must be a non-nil empty map")
	}
}

func TestNew(t *testing.T) {
	folders := []string{"/a", "/b"}
	ws := New("mixed", folders)

	if ws.Version != Version {
		t.Errorf("Version = %d; want %d", ws.Version, Version)
	}
	if ws.Name != "mixed" {
		t.Errorf("Name = %q; want \"mixed\"", ws.Name)
	}
	if len(ws.Folders) != 2 || ws.Folders[0] != "/a" || ws.Folders[1] != "/b" {
		t.Errorf("Folders = %v; want [/a /b] (passed through as-is)", ws.Folders)
	}
	if !ws.IsTemporary {
		t.Error("IsTemporary = false; want true")
	}
	if ws.Agents == nil || ws.Terminals == nil {
		t.Error("Agents/Terminals must be non-nil")
	}
}

func TestLoad(t *testing.T) {
	t.Run("resolves relative folders against the file dir", func(t *testing.T) {
		dir := t.TempDir()
		absOther := filepath.Join(t.TempDir(), "elsewhere")
		file := filepath.Join(dir, "proj.workspace")
		yaml := fmt.Sprintf("version: 1\nname: team-ws\nfolders:\n  - sub\n  - %s\n", absOther)
		if err := os.WriteFile(file, []byte(yaml), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}

		ws, err := Load(file)
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}
		if ws.FilePath != file {
			t.Errorf("FilePath = %q; want %q", ws.FilePath, file)
		}
		if ws.IsTemporary {
			t.Error("IsTemporary = true; want false for a loaded workspace")
		}
		wantFolders := []string{filepath.Join(dir, "sub"), absOther}
		if len(ws.Folders) != 2 || ws.Folders[0] != wantFolders[0] || ws.Folders[1] != wantFolders[1] {
			t.Errorf("Folders = %v; want %v", ws.Folders, wantFolders)
		}
		if ws.Agents == nil || ws.Terminals == nil {
			t.Fatal("Agents/Terminals must be non-nil after Load even when absent from the YAML")
		}
		if len(ws.Agents) != 0 || len(ws.Terminals) != 0 {
			t.Error("Agents/Terminals should be empty maps when absent from the YAML")
		}
	})

	t.Run("malformed YAML errors", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "bad.workspace")
		if err := os.WriteFile(file, []byte("\tinvalid: [yaml\n"), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		if _, err := Load(file); err == nil {
			t.Fatal("Load(malformed YAML) should error")
		}
	})

	t.Run("missing file errors", func(t *testing.T) {
		if _, err := Load(filepath.Join(t.TempDir(), "missing.workspace")); err == nil {
			t.Fatal("Load(missing file) should error")
		}
	})
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	other := t.TempDir() // outside dir, exercises "../" style relative output

	ws := New("rt", []string{sub, other})
	file := filepath.Join(dir, "proj.workspace")
	if err := ws.SaveAs(file); err != nil {
		t.Fatalf("SaveAs error: %v", err)
	}

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if !strings.Contains(string(data), "./sub") {
		t.Errorf("saved YAML should store folders relative (./sub), got:\n%s", data)
	}

	reloaded, err := Load(file)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if got := reloaded.GetFolders(); len(got) != 2 || got[0] != sub || got[1] != other {
		t.Errorf("reloaded Folders = %v; want [%s %s] (absolute again)", got, sub, other)
	}
	if reloaded.IsTemporary {
		t.Error("reloaded IsTemporary = true; want false")
	}
	if reloaded.FilePath != file {
		t.Errorf("reloaded FilePath = %q; want %q", reloaded.FilePath, file)
	}
	if reloaded.Version != Version {
		t.Errorf("reloaded Version = %d; want %d", reloaded.Version, Version)
	}
}

func TestSaveWithoutFilePath(t *testing.T) {
	ws := NewTemporary(t.TempDir())
	err := ws.Save()
	if err == nil {
		t.Fatal("Save without FilePath should error")
	}
	if !strings.Contains(err.Error(), "no file path set") {
		t.Fatalf("error = %v; want it to mention 'no file path set'", err)
	}
}

func TestAddRemoveFolders(t *testing.T) {
	dir := t.TempDir()
	other := t.TempDir()
	ws := New("folders", nil)

	ws.AddFolder(dir)
	if got := ws.GetFolders(); len(got) != 1 || got[0] != dir {
		t.Fatalf("Folders = %v; want [%s]", got, dir)
	}

	// Same folder via a different (trailing slash) form must not duplicate.
	ws.AddFolder(dir + "/")
	if got := ws.GetFolders(); len(got) != 1 {
		t.Fatalf("duplicate AddFolder not deduped: %v", got)
	}

	ws.AddFolder(other)
	if got := ws.GetFolders(); len(got) != 2 {
		t.Fatalf("Folders = %v; want 2 entries", got)
	}

	if !ws.RemoveFolder(dir) {
		t.Fatal("RemoveFolder(existing) = false; want true")
	}
	if ws.RemoveFolder(dir) {
		t.Error("RemoveFolder(already removed) = true; want false")
	}
	if ws.RemoveFolder(filepath.Join(dir, "never-added")) {
		t.Error("RemoveFolder(unknown) = true; want false")
	}
	if got := ws.GetFolders(); len(got) != 1 || got[0] != other {
		t.Fatalf("Folders = %v; want [%s]", got, other)
	}
}

func TestGetFoldersReturnsCopy(t *testing.T) {
	dir := t.TempDir()
	ws := NewTemporary(dir)

	got := ws.GetFolders()
	got[0] = "tampered"
	got = append(got, "extra")

	fresh := ws.GetFolders()
	if len(fresh) != 1 || fresh[0] != dir {
		t.Fatalf("mutating the returned slice affected the workspace: %v", fresh)
	}
}

func TestSetNameGetName(t *testing.T) {
	ws := NewTemporary(t.TempDir())
	ws.SetName("renamed")
	if got := ws.GetName(); got != "renamed" {
		t.Fatalf("GetName() = %q; want \"renamed\"", got)
	}
}

func TestManagerOpenFolder(t *testing.T) {
	root := t.TempDir()
	folder := filepath.Join(root, "project")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	m, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager error: %v", err)
	}

	if m.Current() != nil {
		t.Error("Current() = non-nil before any open; want nil")
	}
	if m.IsOpen() {
		t.Error("IsOpen() = true before any open; want false")
	}

	ws, err := m.OpenFolder(folder)
	if err != nil {
		t.Fatalf("OpenFolder error: %v", err)
	}
	if !ws.IsTemporary {
		t.Error("opened workspace IsTemporary = false; want true")
	}
	if m.Current() != ws {
		t.Error("Current() != the workspace returned by OpenFolder")
	}
	if !m.IsOpen() {
		t.Error("IsOpen() = false after open; want true")
	}

	recents := m.GetRecent()
	if len(recents) != 1 {
		t.Fatalf("recents = %d entries; want 1", len(recents))
	}
	e := recents[0]
	if e.Path != folder || e.IsWorkspace || e.Name != "project" {
		t.Errorf("recent entry = %+v; want path %s, IsWorkspace false, name project", e, folder)
	}

	t.Run("file path errors", func(t *testing.T) {
		file := filepath.Join(root, "notes.txt")
		if err := os.WriteFile(file, []byte("hi"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
		_, err := m.OpenFolder(file)
		if err == nil || !strings.Contains(err.Error(), "not a directory") {
			t.Fatalf("OpenFolder(file) err = %v; want 'not a directory'", err)
		}
	})

	t.Run("missing path errors", func(t *testing.T) {
		if _, err := m.OpenFolder(filepath.Join(root, "ghost")); err == nil {
			t.Fatal("OpenFolder(missing path) should error")
		}
	})

	t.Run("close and save-current", func(t *testing.T) {
		if err := m.SaveCurrent(); err == nil {
			t.Fatal("SaveCurrent() with a FilePath-less temp workspace should error")
		}

		m.Close()
		if m.Current() != nil || m.IsOpen() {
			t.Error("Close() did not clear the current workspace")
		}
		if err := m.SaveCurrent(); err == nil || !strings.Contains(err.Error(), "no workspace open") {
			t.Fatalf("SaveCurrent() with none open err = %v; want 'no workspace open'", err)
		}
	})
}

func TestManagerOpenWorkspace(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "myproj.workspace")
	if err := New("myproj", []string{dir}).SaveAs(file); err != nil {
		t.Fatalf("SaveAs error: %v", err)
	}

	m, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager error: %v", err)
	}

	ws, err := m.OpenWorkspace(file)
	if err != nil {
		t.Fatalf("OpenWorkspace error: %v", err)
	}
	if m.Current() != ws {
		t.Error("Current() != the workspace returned by OpenWorkspace")
	}

	recents := m.GetRecent()
	if len(recents) != 1 {
		t.Fatalf("recents = %d entries; want 1", len(recents))
	}
	e := recents[0]
	if !e.IsWorkspace {
		t.Errorf("recent entry IsWorkspace = false; want true")
	}
	if e.Name != "myproj" {
		t.Errorf("recent entry Name = %q; want \"myproj\" (.workspace extension stripped)", e.Name)
	}
	if e.Path != file {
		t.Errorf("recent entry Path = %q; want %q", e.Path, file)
	}
}

func TestRecentsUpdateAndOrdering(t *testing.T) {
	root := t.TempDir()
	var dirs []string
	for _, name := range []string{"alpha", "bravo", "charlie"} {
		d := filepath.Join(root, name)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		dirs = append(dirs, d)
	}

	m, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager error: %v", err)
	}

	for _, d := range dirs {
		if _, err := m.OpenFolder(d); err != nil {
			t.Fatalf("OpenFolder(%s) error: %v", d, err)
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Newest first.
	if got := m.GetRecent()[0].Path; got != dirs[2] {
		t.Fatalf("newest recent = %s; want %s", got, dirs[2])
	}

	// Re-opening updates LastOpened in place instead of duplicating.
	firstBravo := m.GetRecent()[1]
	time.Sleep(5 * time.Millisecond)
	if _, err := m.OpenFolder(dirs[1]); err != nil {
		t.Fatalf("re-open error: %v", err)
	}
	recents := m.GetRecent()
	if len(recents) != 3 {
		t.Fatalf("re-open created a duplicate: %d entries", len(recents))
	}
	if recents[0].Path != dirs[1] {
		t.Fatalf("re-opened entry should sort first, got %+v", recents[0])
	}
	if !recents[0].LastOpened.After(firstBravo.LastOpened) {
		t.Error("re-open did not bump LastOpened")
	}

	// Pinned entries sort first, regardless of recency.
	m.PinRecent(dirs[0], true)
	recents = m.GetRecent()
	wantOrder := []string{dirs[0], dirs[1], dirs[2]}
	for i, want := range wantOrder {
		if recents[i].Path != want {
			t.Fatalf("GetRecent()[%d].Path = %s; want %s (pinned first, then last-opened desc)", i, recents[i].Path, want)
		}
	}
}

func TestRecentsPersistAcrossManagers(t *testing.T) {
	dataDir := t.TempDir()
	d := t.TempDir()

	m1, err := NewManager(dataDir)
	if err != nil {
		t.Fatalf("NewManager error: %v", err)
	}
	if _, err := m1.OpenFolder(d); err != nil {
		t.Fatalf("OpenFolder error: %v", err)
	}
	m1.PinRecent(d, true)
	m1.FavoriteRecent(d, true)

	m2, err := NewManager(dataDir)
	if err != nil {
		t.Fatalf("NewManager error: %v", err)
	}
	recents := m2.GetRecent()
	if len(recents) != 1 {
		t.Fatalf("reloaded recents = %d entries; want 1", len(recents))
	}
	if !recents[0].Pinned || !recents[0].Favorite || recents[0].Path != d {
		t.Errorf("reloaded entry = %+v; want pinned+favorite %s", recents[0], d)
	}

	m2.RemoveRecent(d)
	m3, err := NewManager(dataDir)
	if err != nil {
		t.Fatalf("NewManager error: %v", err)
	}
	if got := m3.GetRecent(); len(got) != 0 {
		t.Errorf("recents after RemoveRecent = %+v; want empty", got)
	}
}

func TestRecentsCapAtFifty(t *testing.T) {
	root := t.TempDir()
	m, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager error: %v", err)
	}

	const total = 55
	paths := make([]string, total)
	for i := 0; i < total; i++ {
		d := filepath.Join(root, fmt.Sprintf("ws-%02d", i))
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		paths[i] = d
		if _, err := m.OpenFolder(d); err != nil {
			t.Fatalf("OpenFolder error: %v", err)
		}
	}

	recents := m.GetRecent()
	if len(recents) != 50 {
		t.Fatalf("recents = %d entries; want capped at 50", len(recents))
	}

	seen := make(map[string]bool, len(recents))
	for _, e := range recents {
		seen[e.Path] = true
	}
	for i := 0; i < 5; i++ { // the five oldest must have been dropped
		if seen[paths[i]] {
			t.Errorf("%s survived the 50-entry cap; oldest entries should be dropped", paths[i])
		}
	}
	if !seen[paths[5]] || !seen[paths[total-1]] {
		t.Error("expected the 6th-oldest and newest entries to be present")
	}
}

func TestCorruptRecentFileStartsEmpty(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "recent.json"), []byte("not json"), 0o644); err != nil {
		t.Fatalf("write corrupt recent.json: %v", err)
	}

	m, err := NewManager(dataDir)
	if err != nil {
		t.Fatalf("NewManager with corrupt recent.json error: %v", err)
	}
	if got := m.GetRecent(); len(got) != 0 {
		t.Errorf("recents = %+v; want empty after corrupt file", got)
	}
}

// TestSaveKeepsFoldersAbsoluteUnderConcurrency documents FORGE-134.
//
// Workspace.Save() snapshots relFolders under the read lock, then briefly
// swaps the shared w.Folders slice for the relative ("./...") forms under the
// write lock before restoring it. Lock-obeying readers are fine, but anything
// that reads the exported Folders field directly (e.g. json.Marshal of the
// workspace in the bindings layer) races with the swap, and the pre-lock
// snapshot means a concurrent AddFolder can be silently dropped from the
// written file.
func TestSaveKeepsFoldersAbsoluteUnderConcurrency(t *testing.T) {
	t.Skip("known limitation, see QA doc FORGE-134: Save() mutates w.Folders in place under the write lock and snapshots relFolders before the lock upgrade; re-enable once Save marshals a copy without mutating shared state")

	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	ws := NewTemporary(sub)
	file := filepath.Join(dir, "proj.workspace")
	if err := ws.SaveAs(file); err != nil {
		t.Fatalf("SaveAs error: %v", err)
	}

	stop := make(chan struct{})
	errCh := make(chan error, 1)
	go func() {
		for {
			select {
			case <-stop:
				errCh <- nil
				return
			default:
			}
			if err := ws.Save(); err != nil {
				errCh <- err
				return
			}
		}
	}()

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		// The invariant: no reader ever observes a non-absolute folder path.
		for _, f := range ws.Folders {
			if !filepath.IsAbs(f) {
				close(stop)
				t.Fatalf("observed non-absolute folder %q while saving", f)
			}
		}
		_ = ws.GetFolders()
	}
	close(stop)
	if err := <-errCh; err != nil {
		t.Fatalf("concurrent Save error: %v", err)
	}
}
