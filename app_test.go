package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hasdev/forge-ade/internal/explorer"
	"github.com/hasdev/forge-ade/internal/git"
)

func setupAnnotateRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "t@t")
	run("config", "user.name", "T")
	for _, p := range []string{"root.txt", "sub/inner.txt", "sub/deep/far.txt"} {
		full := filepath.Join(dir, p)
		os.MkdirAll(filepath.Dir(full), 0755)
		os.WriteFile(full, []byte("v1\n"), 0644)
	}
	run("add", ".")
	run("commit", "-q", "-m", "init")
	return dir
}

func buildTree(dir string, depth int) *explorer.FileInfo {
	exp := explorer.New(nil)
	exp.SetRoots([]string{dir})
	tree, err := exp.GetTree(depth)
	if err != nil || len(tree) == 0 {
		return nil
	}
	return tree[0]
}

func TestAnnotateNestedGitStatus(t *testing.T) {
	dir := setupAnnotateRepo(t)
	ctx := context.Background()
	engine := git.NewEngine()

	os.WriteFile(filepath.Join(dir, "sub", "inner.txt"), []byte("MODIFIED\n"), 0644)
	os.WriteFile(filepath.Join(dir, "root.txt"), []byte("ROOTMOD\n"), 0644)

	statusMap, err := engine.StatusByPath(ctx, dir)
	if err != nil {
		t.Fatalf("StatusByPath: %v", err)
	}

	root := buildTree(dir, 2)
	if root == nil {
		t.Fatal("failed to build tree")
	}
	annotateNodeGitStatus(root, dir, statusMap)

	if root.GitStatus == "" {
		t.Error("root dir should be marked dirty")
	}
	if root.Children == nil {
		t.Fatal("expected children")
	}
	find := func(name string) *explorer.FileInfo {
		for _, c := range root.Children {
			if c.Name == name {
				return c
			}
		}
		return nil
	}
	rootTxt := find("root.txt")
	if rootTxt == nil || rootTxt.GitStatus != "M" {
		t.Errorf("root.txt should be M, got %+v", rootTxt)
	}
	sub := find("sub")
	if sub == nil || sub.GitStatus == "" {
		t.Errorf("sub dir should be dirty, got %+v", sub)
	} else {
		inner := findChild(sub, "inner.txt")
		if inner == nil || inner.GitStatus != "M" {
			t.Errorf("sub/inner.txt should be M, got %+v", inner)
		}
	}
}

func findChild(parent *explorer.FileInfo, name string) *explorer.FileInfo {
	for _, c := range parent.Children {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestAnnotateFlatListing(t *testing.T) {
	// Simulates ExpandPath/ListDirectory output: a flat []*FileInfo of sibling
	// files AND directories (not a root folder). Top-level files must get
	// annotated too.
	dir := setupAnnotateRepo(t)
	ctx := context.Background()
	engine := git.NewEngine()

	os.WriteFile(filepath.Join(dir, "sub", "inner.txt"), []byte("MODIFIED\n"), 0644)
	os.WriteFile(filepath.Join(dir, "root.txt"), []byte("ROOTMOD\n"), 0644)

	statusMap, err := engine.StatusByPath(ctx, dir)
	if err != nil {
		t.Fatalf("StatusByPath: %v", err)
	}

	// Build a flat listing of the root dir (mirrors readDir output).
	exp := explorer.New(nil)
	listing, err := exp.ListDirectory(dir)
	if err != nil {
		t.Fatalf("ListDirectory: %v", err)
	}

	for _, n := range listing {
		annotateNodeGitStatus(n, dir, statusMap)
	}

	for _, n := range listing {
		switch n.Name {
		case "root.txt":
			if n.GitStatus != "M" {
				t.Errorf("flat root.txt should be M, got %q", n.GitStatus)
			}
		case "sub":
			if n.GitStatus == "" {
				t.Errorf("flat sub dir should be dirty")
			}
		}
	}
}

func TestAnnotateUntrackedDeep(t *testing.T) {
	dir := setupAnnotateRepo(t)
	ctx := context.Background()
	engine := git.NewEngine()

	os.WriteFile(filepath.Join(dir, "sub", "deep", "brandnew.txt"), []byte("new\n"), 0644)

	statusMap, err := engine.StatusByPath(ctx, dir)
	if err != nil {
		t.Fatalf("StatusByPath: %v", err)
	}

	root := buildTree(dir, 3)
	annotateNodeGitStatus(root, dir, statusMap)

	sub := findChild(root, "sub")
	if sub == nil || sub.GitStatus == "" {
		t.Errorf("sub should be dirty, got %+v", sub)
	}
	deep := findChild(sub, "deep")
	if deep == nil || deep.GitStatus == "" {
		t.Errorf("sub/deep should be dirty, got %+v", deep)
	}
	brand := findChild(deep, "brandnew.txt")
	if brand == nil || brand.GitStatus != "U" {
		t.Errorf("brandnew.txt should be U, got %+v", brand)
	}
}

func TestAgentSessionDiskGlobalProjectStorage(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	app := &App{}

	projectWs := filepath.Join(tempHome, "projects", "my-awesome-app")
	if err := os.MkdirAll(projectWs, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	sessionJSON := `{"id":"sess-123","workspacePath":"` + projectWs + `","title":"Initial task"}`

	// 1. Save session
	if err := app.SaveAgentSessionDisk(sessionJSON, projectWs); err != nil {
		t.Fatalf("SaveAgentSessionDisk failed: %v", err)
	}

	// Verify it saved under ~/.forge/sessions/my-awesome-app/sess-123.json
	sessFile := filepath.Join(tempHome, ".forge", "sessions", "my-awesome-app", "sess-123.json")
	if _, err := os.Stat(sessFile); err != nil {
		t.Fatalf("expected session file at %s, got: %v", sessFile, err)
	}

	// Verify info.json was generated with correct metadata
	infoFile := filepath.Join(tempHome, ".forge", "sessions", "my-awesome-app", "info.json")
	infoData, err := os.ReadFile(infoFile)
	if err != nil {
		t.Fatalf("failed to read info.json: %v", err)
	}
	var info ProjectSessionInfo
	if err := json.Unmarshal(infoData, &info); err != nil {
		t.Fatalf("failed to parse info.json: %v", err)
	}
	if info.ProjectName != "my-awesome-app" {
		t.Errorf("expected projectName 'my-awesome-app', got '%s'", info.ProjectName)
	}
	if info.ProjectPath != projectWs {
		t.Errorf("expected projectPath '%s', got '%s'", projectWs, info.ProjectPath)
	}
	if info.SessionCount != 1 {
		t.Errorf("expected sessionCount 1, got %d", info.SessionCount)
	}
	if info.CreatedAt == "" || info.UpdatedAt == "" {
		t.Errorf("expected timestamps in info.json, got createdAt: %s, updatedAt: %s", info.CreatedAt, info.UpdatedAt)
	}

	// 2. Add another session to check sessionCount increments
	sessionJSON2 := `{"id":"sess-456","workspacePath":"` + projectWs + `","title":"Second task"}`
	if err := app.SaveAgentSessionDisk(sessionJSON2, projectWs); err != nil {
		t.Fatalf("SaveAgentSessionDisk 2 failed: %v", err)
	}
	infoData2, _ := os.ReadFile(infoFile)
	_ = json.Unmarshal(infoData2, &info)
	if info.SessionCount != 2 {
		t.Errorf("expected sessionCount 2, got %d", info.SessionCount)
	}

	// 3. Test LoadAgentSessionsDisk (should ignore info.json and return both sessions)
	loaded, err := app.LoadAgentSessionsDisk(projectWs)
	if err != nil {
		t.Fatalf("LoadAgentSessionsDisk failed: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 loaded sessions, got %d", len(loaded))
	}

	// 4. Test delete session and verify info.json count decrements
	if err := app.DeleteAgentSessionDisk("sess-123", projectWs); err != nil {
		t.Fatalf("DeleteAgentSessionDisk failed: %v", err)
	}
	if _, err := os.Stat(sessFile); !os.IsNotExist(err) {
		t.Fatalf("expected sess-123.json to be deleted")
	}
	infoData3, _ := os.ReadFile(infoFile)
	_ = json.Unmarshal(infoData3, &info)
	if info.SessionCount != 1 {
		t.Errorf("expected sessionCount 1 after delete, got %d", info.SessionCount)
	}

	// 5. Test legacy migration from ~/.forge-ade/sessions/
	legacyDir := filepath.Join(tempHome, ".forge-ade", "sessions")
	_ = os.MkdirAll(legacyDir, 0755)
	legacySess := `{"id":"legacy-789","workspacePath":"` + projectWs + `","title":"Legacy migrated task"}`
	_ = os.WriteFile(filepath.Join(legacyDir, "legacy-789.json"), []byte(legacySess), 0644)

	// Load should auto-migrate into ~/.forge/sessions/my-awesome-app/
	loadedAfterMigration, err := app.LoadAgentSessionsDisk(projectWs)
	if err != nil {
		t.Fatalf("LoadAgentSessionsDisk after migration failed: %v", err)
	}
	if len(loadedAfterMigration) != 2 { // sess-456 + legacy-789
		t.Fatalf("expected 2 sessions after migration, got %d", len(loadedAfterMigration))
	}
	migratedFile := filepath.Join(tempHome, ".forge", "sessions", "my-awesome-app", "legacy-789.json")
	if _, err := os.Stat(migratedFile); err != nil {
		t.Fatalf("expected legacy session to be migrated to %s: %v", migratedFile, err)
	}
}

