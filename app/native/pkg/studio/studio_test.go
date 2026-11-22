package studio

import (
	"testing"

	corestudio "github.com/hasdev/forge-ade/core/studio"
)

func TestNativeStudioManager(t *testing.T) {
	tmp := t.TempDir()
	mgr := NewStudioManager(tmp)

	if mgr == nil || mgr.Core() == nil {
		t.Fatal("expected studio manager with core")
	}

	projs := mgr.Projects()
	if len(projs) == 0 {
		t.Fatal("expected default projects")
	}

	active := mgr.ActiveProject()
	if active == nil {
		t.Fatal("expected active project")
	}

	// Create new project
	p := mgr.CreateProject("New 3D Scene", "Three.js sandbox", corestudio.ArtifactTypeCodeJS)
	if p == nil || p.Name != "New 3D Scene" {
		t.Fatalf("expected project 'New 3D Scene', got %+v", p)
	}

	if mgr.ActiveProject().ID != p.ID {
		t.Fatalf("expected newly created project to be active, got %s", mgr.ActiveProject().ID)
	}

	// Pinning
	if mgr.IsPinning() {
		t.Fatal("expected pinning to be false")
	}
	mgr.TogglePinning()
	if !mgr.IsPinning() {
		t.Fatal("expected pinning to be true")
	}

	// Export
	res, err := mgr.ExportActive()
	if err != nil {
		t.Fatalf("export active failed: %v", err)
	}
	if res == nil || res.PrimaryFilePath == "" {
		t.Fatal("expected export result")
	}
}
