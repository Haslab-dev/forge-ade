package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/engine"
)

func TestCustomMultiFolderProjectGroupingAndPrefs(t *testing.T) {
	tmpDir := t.TempDir()
	folderA := filepath.Join(tmpDir, "folder-a")
	folderB := filepath.Join(tmpDir, "folder-b")
	folderC := filepath.Join(tmpDir, "folder-c")
	for _, d := range []string{folderA, folderB, folderC} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	eng := engine.NewEngine(tmpDir)
	app := NewAppUI(eng, tmpDir)
	defer app.Close()

	app.CustomProjectNameInput = "MyProject"
	app.CustomProjectFoldersInput = []string{folderA, folderB, folderC}
	app.saveCustomProjectDraft()

	if len(app.CustomProjects) != 1 || app.CustomProjects[0].Name != "MyProject" {
		t.Fatalf("expected MyProject saved, got %+v", app.CustomProjects)
	}

	app.setProjectIcon("MyProject", ProjectIconConfig{
		Kind:     "icon",
		IconID:   "rocket",
		ColorKey: "accent",
	})

	// Seed sessions across folderA, folderB, folderC
	now := time.Now()
	app.Agent.Lock()
	app.Agent.Sessions = append(app.Agent.Sessions,
		&agent.AgentSession{ID: "s-a", Title: "Task A", WorkspacePath: folderA, CreatedAt: now, UpdatedAt: now},
		&agent.AgentSession{ID: "s-b", Title: "Task B", WorkspacePath: folderB, CreatedAt: now, UpdatedAt: now},
		&agent.AgentSession{ID: "s-c", Title: "Task C", WorkspacePath: folderC, CreatedAt: now, UpdatedAt: now},
	)
	app.Agent.Unlock()

	groups := app.sidebarProjectGroups()
	var myGroup *sidebarProjectGroup
	for _, g := range groups {
		if g.ProjectName == "MyProject" {
			myGroup = g
			break
		}
	}
	if myGroup == nil {
		t.Fatalf("expected MyProject group in sidebarProjectGroups, got %d groups", len(groups))
	}
	if !myGroup.IsCustom || len(myGroup.Folders) != 3 {
		t.Fatalf("expected IsCustom=true and 3 folders, got IsCustom=%v folders=%d", myGroup.IsCustom, len(myGroup.Folders))
	}
	if len(myGroup.Sessions) != 3 {
		t.Fatalf("expected 3 sessions grouped under MyProject, got %d", len(myGroup.Sessions))
	}

	// Verify prefs persistence round-trip
	app2 := NewAppUI(eng, tmpDir)
	defer app2.Close()
	if len(app2.CustomProjects) != 1 || len(app2.CustomProjects[0].Folders) != 3 {
		t.Fatalf("expected CustomProjects persisted across reload, got %+v", app2.CustomProjects)
	}
	if ic, ok := app2.ProjectIcons["MyProject"]; !ok || ic.IconID != "rocket" {
		t.Fatalf("expected ProjectIcons persisted across reload, got %+v", app2.ProjectIcons)
	}
}
