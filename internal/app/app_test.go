package app

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Haslab-dev/RakitKode/agent"
	"github.com/Haslab-dev/RakitKode/decisions"
	"github.com/Haslab-dev/RakitKode/events"
	"github.com/Haslab-dev/RakitKode/llm"
	"github.com/Haslab-dev/RakitKode/mcp"
	"github.com/Haslab-dev/RakitKode/plugins"
	"github.com/Haslab-dev/RakitKode/search"
	"github.com/Haslab-dev/RakitKode/skills"
	"github.com/Haslab-dev/RakitKode/tools"
	"github.com/hasdev/forge-ade/internal/agentsession"
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
	var info agentsession.ProjectSessionInfo
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

func TestAppDecideAgentRole(t *testing.T) {
	app := &App{}
	// Nil manager fallback
	res, err := app.DecideAgentRole("anything")
	if err != nil {
		t.Fatalf("unexpected error with nil agentMgr: %v", err)
	}
	if res["role"] == "" {
		t.Fatalf("expected role in response, got %v", res)
	}
}

func TestDefaultAppSettings_JevDisabled(t *testing.T) {
	st := agentsession.DefaultAppSettings()
	if st.Jev.Enabled {
		t.Fatal("expected DefaultAppSettings().Jev.Enabled to be false (Pandu-Micro offline by default)")
	}
}

func TestApp_JevSettingsAndPanduMicroRouting(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	bus := events.NewBus()
	llmClient := llm.NewLLMClient(tempHome)
	si := search.NewSearchManager()
	toolReg := tools.NewRegistry(si)
	skillMgr := skills.NewManager("")
	pluginMgr := plugins.NewManager(tempHome, bus)
	mcpMgr := mcp.NewManager(tempHome)

	agentMgr := agent.NewManager(llmClient, toolReg, skillMgr, pluginMgr, mcpMgr, nil, bus, tempHome)
	settingsStore := agentsession.NewSettingsStore(tempHome)

	app := &App{
		agentMgr:    agentMgr,
		appSettings: settingsStore,
	}

	// 1. Initial status: Jev disabled, Pandu-Micro default model
	status := app.GetJevStatus()
	if status["enabled"] != false {
		t.Errorf("expected status enabled false, got %v", status["enabled"])
	}
	if status["local_model"] != "pandu-micro" {
		t.Errorf("expected local_model 'pandu-micro', got %v", status["local_model"])
	}

	adapter := agentMgr.GetDecisionAdapter()
	if adapter == nil {
		t.Fatal("expected decision adapter on agent manager")
	}
	adapterStatus := adapter.GetStatus()
	if adapterStatus.ModelVersion != "pandu-micro" {
		t.Errorf("expected adapter ModelVersion 'pandu-micro', got %q", adapterStatus.ModelVersion)
	}
	if adapterStatus.Mode != decisions.ModeLocalOnly {
		t.Errorf("expected adapter Mode %q, got %q", decisions.ModeLocalOnly, adapterStatus.Mode)
	}
	if adapterStatus.JevEnabled {
		t.Error("expected adapter JevEnabled false by default")
	}

	// 2. Enable Jev via SetJevEnabled(true)
	if err := app.SetJevEnabled(true); err != nil {
		t.Fatalf("SetJevEnabled(true) failed: %v", err)
	}
	stEnabled := app.GetJevStatus()
	if stEnabled["enabled"] != true {
		t.Errorf("expected status enabled true after toggle, got %v", stEnabled["enabled"])
	}
	adapterStEnabled := adapter.GetStatus()
	if !adapterStEnabled.JevEnabled {
		t.Error("expected adapter JevEnabled true")
	}
	if adapterStEnabled.Mode != decisions.ModeCascade {
		t.Errorf("expected adapter Mode %q, got %q", decisions.ModeCascade, adapterStEnabled.Mode)
	}

	// 3. Disable Jev via SetJevEnabled(false) -> should return to local-only Pandu-Micro
	if err := app.SetJevEnabled(false); err != nil {
		t.Fatalf("SetJevEnabled(false) failed: %v", err)
	}
	stDisabled := app.GetJevStatus()
	if stDisabled["enabled"] != false {
		t.Errorf("expected status enabled false after toggle, got %v", stDisabled["enabled"])
	}
	adapterStDisabled := adapter.GetStatus()
	if adapterStDisabled.JevEnabled {
		t.Error("expected adapter JevEnabled false")
	}
	if adapterStDisabled.Mode != decisions.ModeLocalOnly {
		t.Errorf("expected adapter Mode %q, got %q", decisions.ModeLocalOnly, adapterStDisabled.Mode)
	}
	if adapterStDisabled.ModelVersion != "pandu-micro" {
		t.Errorf("expected adapter ModelVersion 'pandu-micro', got %q", adapterStDisabled.ModelVersion)
	}
}

func TestNewApp_BootstrapAndEventConstants(t *testing.T) {
	app, err := NewApp()
	if err != nil {
		t.Fatalf("expected NewApp() to succeed, got: %v", err)
	}
	if app == nil {
		t.Fatal("expected non-nil App instance")
	}

	// Verify typed event constants
	if EventFSChanged != "fs:changed" {
		t.Errorf("expected EventFSChanged to be 'fs:changed', got %q", EventFSChanged)
	}
	if EventSessionOutput != "session:output" {
		t.Errorf("expected EventSessionOutput to be 'session:output', got %q", EventSessionOutput)
	}
	if EventSessionOpened != "session:opened" {
		t.Errorf("expected EventSessionOpened to be 'session:opened', got %q", EventSessionOpened)
	}
	if EventSessionClosed != "session:closed" {
		t.Errorf("expected EventSessionClosed to be 'session:closed', got %q", EventSessionClosed)
	}

	payload := FSChangeEvent{
		Type: FSChangeCreated,
		Path: "/test/path.txt",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal FSChangeEvent: %v", err)
	}
	if string(data) != `{"type":"created","path":"/test/path.txt"}` {
		t.Errorf("unexpected marshaled FSChangeEvent: %s", string(data))
	}
}
