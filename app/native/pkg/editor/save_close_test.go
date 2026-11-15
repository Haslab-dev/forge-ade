package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTestFile creates a file with content under a fresh temp dir.
func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestRequestCloseStagesConfirmForDirtyBuffer(t *testing.T) {
	tempDir := t.TempDir()
	filePath := writeTestFile(t, tempDir, "dirty.go", "package main\n")

	em := NewEditorManager()
	if _, err := em.OpenFile(filePath, "dirty.go"); err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	doc := em.ActiveDoc()
	doc.Editor.Buffer.Set("package main\n\nfunc changed() {}\n")
	if !doc.Editor.Buffer.Modified() {
		t.Fatalf("expected buffer to be modified after edit")
	}

	em.ConfirmBeforeDiscard = true
	var confirmed []string
	em.OnConfirmDiscard = func(paths []string, decide func(discard bool)) {
		confirmed = paths
		decide(false) // keep the buffer open
	}

	em.RequestClose(filePath)

	if len(confirmed) != 1 || confirmed[0] != filePath {
		t.Fatalf("expected callback to capture dirty path %s, got %v", filePath, confirmed)
	}
	if len(em.OpenTabPaths) != 1 || em.OpenTabPaths[0] != filePath {
		t.Fatalf("ResolveDiscard(false) must keep the tab open, tabs=%v", em.OpenTabPaths)
	}
	if len(em.PendingDiscard) != 0 {
		t.Fatalf("PendingDiscard must clear after ResolveDiscard, got %v", em.PendingDiscard)
	}

	// Accepting the discard force-closes the tab (firing OnTabClosed).
	closedPaths := make(map[string]bool, 1)
	em.OnTabClosed = func(path string) { closedPaths[path] = true }
	em.OnConfirmDiscard = func(paths []string, decide func(discard bool)) {
		confirmed = paths
		decide(true)
	}
	em.RequestClose(filePath)
	if !closedPaths[filePath] {
		t.Fatalf("OnTabClosed must fire for %s on ResolveDiscard(true)", filePath)
	}
	if len(em.OpenTabPaths) != 0 || em.GetDoc(filePath) != nil {
		t.Fatalf("ResolveDiscard(true) must close the tab, tabs=%v", em.OpenTabPaths)
	}
}

func TestRequestCloseCleanTabClosesImmediately(t *testing.T) {
	tempDir := t.TempDir()
	filePath := writeTestFile(t, tempDir, "clean.go", "package main\n")

	em := NewEditorManager()
	if _, err := em.OpenFile(filePath, "clean.go"); err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	em.ConfirmBeforeDiscard = true
	called := false
	em.OnConfirmDiscard = func(paths []string, decide func(discard bool)) {
		called = true
		decide(false)
	}

	em.RequestClose(filePath)
	if called {
		t.Fatalf("clean buffer must close without confirmation")
	}
	if len(em.OpenTabPaths) != 0 {
		t.Fatalf("clean tab must close immediately, tabs=%v", em.OpenTabPaths)
	}
}

func TestConfirmBeforeDiscardDisabledKeepsLegacyBehavior(t *testing.T) {
	tempDir := t.TempDir()
	filePath := writeTestFile(t, tempDir, "legacy.go", "package main\n")

	em := NewEditorManager()
	if _, err := em.OpenFile(filePath, "legacy.go"); err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	em.ActiveDoc().Editor.Buffer.Set("edited\n")
	em.RequestClose(filePath)
	if len(em.OpenTabPaths) != 0 {
		t.Fatalf("default (no confirm) must close dirty tabs unconditionally, tabs=%v", em.OpenTabPaths)
	}
}

func TestRequestCloseOthersAllLeftRight(t *testing.T) {
	tempDir := t.TempDir()
	fileA := writeTestFile(t, tempDir, "a.go", "package main\n")
	fileB := writeTestFile(t, tempDir, "b.go", "package main\n")
	fileC := writeTestFile(t, tempDir, "c.go", "package main\n")

	em := NewEditorManager()
	for _, p := range []string{fileA, fileB, fileC} {
		if _, err := em.OpenFile(p, filepath.Base(p)); err != nil {
			t.Fatalf("OpenFile %s: %v", p, err)
		}
	}
	dirtyDoc := em.GetDoc(fileB)
	dirtyDoc.Editor.Buffer.Set("dirty\n")
	em.ConfirmBeforeDiscard = true

	var captured []string
	decided := false
	em.OnConfirmDiscard = func(paths []string, decide func(discard bool)) {
		captured = paths
		decided = true
		decide(true)
	}

	em.RequestCloseOthers(fileA)
	if !decided || len(captured) != 1 || captured[0] != fileB {
		t.Fatalf("RequestCloseOthers should confirm only the dirty target, got %v", captured)
	}
	// keepPath survives Close Others; the dirty B closes via ResolveDiscard
	// and clean C closes immediately.
	if len(em.OpenTabPaths) != 1 || em.OpenTabPaths[0] != fileA {
		t.Fatalf("expected keepPath %s to remain, tabs=%v", fileA, em.OpenTabPaths)
	}
}

func TestSaveMarksBufferClean(t *testing.T) {
	tempDir := t.TempDir()
	filePath := writeTestFile(t, tempDir, "save.go", "package main\n")

	em := NewEditorManager()
	doc, err := em.OpenFile(filePath, "save.go")
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	doc.Editor.Buffer.Set("package main\n\nfunc saved() {}\n")
	if !doc.Editor.Buffer.Modified() {
		t.Fatalf("expected dirty buffer before save")
	}
	if err := em.SaveActiveFile(); err != nil {
		t.Fatalf("SaveActiveFile: %v", err)
	}
	if doc.Editor.Buffer.Modified() {
		t.Fatalf("buffer must be unmodified after a successful save")
	}
	onDisk, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(onDisk) != "package main\n\nfunc saved() {}\n" {
		t.Fatalf("unexpected disk content: %q", string(onDisk))
	}
}

func TestSavePreservesCRLFAndBOM(t *testing.T) {
	tempDir := t.TempDir()

	crlfPath := writeTestFile(t, tempDir, "crlf.txt", "alpha\r\nbeta\r\n")
	em := NewEditorManager()
	doc, err := em.OpenFile(crlfPath, "crlf.txt")
	if err != nil {
		t.Fatalf("OpenFile crlf: %v", err)
	}
	if !doc.HasCRLF {
		t.Fatalf("expected CRLF detection")
	}
	doc.Editor.Buffer.Set("alpha edited\nbeta edited\n")
	if err := em.SaveActiveFile(); err != nil {
		t.Fatalf("SaveActiveFile: %v", err)
	}
	onDisk, _ := os.ReadFile(crlfPath)
	if string(onDisk) != "alpha edited\r\nbeta edited\r\n" {
		t.Fatalf("CRLF must be restored on save, got %q", string(onDisk))
	}

	bomPath := writeTestFile(t, tempDir, "bom.json", "\uFEFF{\"a\": 1}\n")
	doc2, err := em.OpenFile(bomPath, "bom.json")
	if err != nil {
		t.Fatalf("OpenFile bom: %v", err)
	}
	if !doc2.HasBOM {
		t.Fatalf("expected BOM detection")
	}
	if strings.HasPrefix(doc2.Editor.Buffer.String(), "\uFEFF") {
		t.Fatalf("BOM must be stripped from the buffer at open")
	}
	doc2.Editor.Buffer.Set("{\"a\": 2}\n")
	if err := em.SaveActiveFile(); err != nil {
		t.Fatalf("SaveActiveFile bom: %v", err)
	}
	onDisk2, _ := os.ReadFile(bomPath)
	if string(onDisk2) != "\uFEFF{\"a\": 2}\n" {
		t.Fatalf("BOM must be re-added on save, got %q", string(onDisk2))
	}
}

func TestSaveAtomicPreservesFileMode(t *testing.T) {
	tempDir := t.TempDir()
	filePath := writeTestFile(t, tempDir, "script.sh", "#!/bin/sh\necho hi\n")
	if err := os.Chmod(filePath, 0755); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	em := NewEditorManager()
	doc, err := em.OpenFile(filePath, "script.sh")
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	doc.Editor.Buffer.Set("#!/bin/sh\necho bye\n")
	if err := em.SaveActiveFile(); err != nil {
		t.Fatalf("SaveActiveFile: %v", err)
	}
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Fatalf("atomic save must preserve file mode, got %v", info.Mode().Perm())
	}
	// No temp leftovers in the directory.
	entries, _ := os.ReadDir(tempDir)
	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("temp files must be cleaned up, dir contents: %v", names)
	}
}

func TestSaveOverwriteConfirmFlow(t *testing.T) {
	tempDir := t.TempDir()
	filePath := writeTestFile(t, tempDir, "conflict.txt", "original\n")

	em := NewEditorManager()
	doc, err := em.OpenFile(filePath, "conflict.txt")
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	doc.Editor.Buffer.Set("my edits\n")

	// Simulate an external writer bumping the mtime.
	future := doc.ModTime.Add(2 * 60 * 60 * 1000 * 1000 * 1000) // +2h
	if err := os.Chtimes(filePath, future, future); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	em.ConfirmBeforeDiscard = false
	asked := false
	em.OnConfirmOverwrite = func(path string, decide func(proceed bool)) {
		asked = true
		if path != filePath {
			t.Errorf("overwrite confirm for wrong path: %s", path)
		}
		decide(true)
	}
	if err := em.SaveActiveFile(); err != nil {
		t.Fatalf("SaveActiveFile with confirm hook: %v", err)
	}
	if !asked {
		t.Fatalf("expected OnConfirmOverwrite to fire on mtime mismatch")
	}
	onDisk, _ := os.ReadFile(filePath)
	if string(onDisk) != "my edits\n" {
		t.Fatalf("ResolveOverwrite(true) must save, disk=%q", string(onDisk))
	}
	if doc.Editor.Buffer.Modified() {
		t.Fatalf("buffer must be clean after confirmed overwrite save")
	}
}

func TestCloseAllForWorkspaceDropsForeignPinnedTabs(t *testing.T) {
	tempDirA := t.TempDir()
	tempDirB := t.TempDir()
	fileA := writeTestFile(t, tempDirA, "a.go", "package main\n")
	fileB := writeTestFile(t, tempDirB, "b.go", "package main\n")

	em := NewEditorManager()
	if _, err := em.OpenFile(fileA, "a.go"); err != nil {
		t.Fatalf("OpenFile a: %v", err)
	}
	if _, err := em.OpenFile(fileB, "b.go"); err != nil {
		t.Fatalf("OpenFile b: %v", err)
	}
	em.TogglePin(fileA) // pinned from the previous workspace must still go

	em.CloseAllForWorkspace(tempDirB)
	for _, p := range em.OpenTabPaths {
		if p == fileA {
			t.Fatalf("foreign pinned tab %s must be closed by CloseAllForWorkspace", fileA)
		}
	}
	if em.GetDoc(fileA) != nil {
		t.Fatalf("foreign doc must be dropped")
	}
	if em.GetDoc(fileB) == nil {
		t.Fatalf("doc under the new workspace must survive")
	}
}

func TestSetDocTextAndSaveKeepsOpenBufferInSync(t *testing.T) {
	tempDir := t.TempDir()
	filePath := writeTestFile(t, tempDir, "rep.go", "value = 1\n")

	em := NewEditorManager()
	doc, err := em.OpenFile(filePath, "rep.go")
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	if err := em.SetDocTextAndSave(filePath, "value = 2\n"); err != nil {
		t.Fatalf("SetDocTextAndSave: %v", err)
	}
	onDisk, _ := os.ReadFile(filePath)
	if string(onDisk) != "value = 2\n" {
		t.Fatalf("disk must match replaced buffer text, got %q", string(onDisk))
	}
	if doc.Editor.Buffer.String() != "value = 2\n" {
		t.Fatalf("buffer must hold the replaced text, got %q", doc.Editor.Buffer.String())
	}
	if doc.Editor.Buffer.Modified() {
		t.Fatalf("buffer must be clean after SetDocTextAndSave")
	}
}
