package studio

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStudioProjectVersioning(t *testing.T) {
	p := &StudioProject{
		ID:        "test-proj",
		Name:      "Test Keynote",
		Type:      ArtifactTypeSlides,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	data1, pins1 := synthesizeArtifactData(ArtifactTypeSlides, "Test Keynote", ArtifactData{})
	v1 := p.AddVersion("Initial prompt", "v1 created", data1, pins1)
	if v1.Version != 1 {
		t.Fatalf("expected version 1, got %d", v1.Version)
	}
	if p.CurrentVersion != 1 {
		t.Fatalf("expected current version 1, got %d", p.CurrentVersion)
	}

	// Add second version
	data2 := data1
	data2.DeckTheme = "accent"
	v2 := p.AddVersion("Update theme", "v2 theme updated", data2, pins1)
	if v2.Version != 2 {
		t.Fatalf("expected version 2, got %d", v2.Version)
	}
	if p.CurrentVersion != 2 {
		t.Fatalf("expected current version 2, got %d", p.CurrentVersion)
	}

	active := p.ActiveVersion()
	if active == nil || active.Version != 2 || active.Data.DeckTheme != "accent" {
		t.Fatalf("expected active version 2 with accent theme, got %+v", active)
	}
}

func TestSkillsEnginePromptInjection(t *testing.T) {
	engine := NewSkillsEngine()
	skills := engine.List()
	if len(skills) == 0 {
		t.Fatal("expected builtin skills to be registered")
	}

	// Enable Swiss Minimalist
	engine.SetEnabled("swiss-minimal", true)

	// Test prompt injection with pinned element
	pin := &ElementPin{
		ID:       "pin-1",
		Selector: "@slide-title",
		Label:    "Slide Title",
		Category: "text",
		X:        100,
		Y:        80,
		Width:    400,
		Height:   50,
	}

	injected := engine.InjectPrompt("Make the typography bolder with higher contrast", pin)

	if !strings.Contains(injected, "[DIRECT-MANIPULATION TARGET: @slide-title") {
		t.Fatalf("expected direct-manipulation selector in injected prompt, got: %s", injected)
	}
	if !strings.Contains(injected, "Swiss International Typographic") {
		t.Fatalf("expected active design skill in injected prompt, got: %s", injected)
	}
	if !strings.Contains(injected, "Make the typography bolder with higher contrast") {
		t.Fatalf("expected user prompt in injected prompt, got: %s", injected)
	}
}

func TestDirectorWorkflow(t *testing.T) {
	p := &StudioProject{
		ID:   "proj-motion",
		Name: "Test Motion",
		Type: ArtifactTypeMotion,
	}
	data, pins := synthesizeArtifactData(ArtifactTypeMotion, "Test Motion", ArtifactData{})
	p.AddVersion("Initial", "v1", data, pins)

	wf := NewDirectorWorkflow(ArtifactTypeMotion)
	steps := wf.GetSteps()
	if len(steps) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(steps))
	}

	skills := NewSkillsEngine()

	completedCh := make(chan bool, 1)
	wf.OnComplete = func(resData ArtifactData, resPins []ElementPin) {
		completedCh <- true
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wf.ExecuteAsync(ctx, p, skills)

	select {
	case <-completedCh:
		finalSteps := wf.GetSteps()
		for _, s := range finalSteps {
			if s.Status != "completed" {
				t.Fatalf("expected step %s to be completed, got %s", s.ID, s.Status)
			}
		}
	case <-time.After(4 * time.Second):
		t.Fatal("timed out waiting for director workflow to complete")
	}
}

func TestExportToWorkspace(t *testing.T) {
	tmpDir := t.TempDir()

	p := &StudioProject{
		ID:   "proj-slides",
		Name: "Pitch Deck",
		Type: ArtifactTypeSlides,
	}
	data, pins := synthesizeArtifactData(ArtifactTypeSlides, "Pitch Deck", ArtifactData{})
	p.AddVersion("v1", "Initial deck", data, pins)

	res, err := ExportToWorkspace(tmpDir, p)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	if res.PrimaryFilePath == "" {
		t.Fatal("expected primary file path")
	}

	if _, err := os.Stat(res.PrimaryFilePath); err != nil {
		t.Fatalf("exported primary file not found on disk: %v", err)
	}

	// Verify HTML content
	content, err := os.ReadFile(res.PrimaryFilePath)
	if err != nil {
		t.Fatalf("could not read exported file: %v", err)
	}
	if !strings.Contains(string(content), "Pitch Deck - Presentation") {
		t.Fatalf("expected presentation title in exported HTML, got: %s", string(content))
	}
}

func TestManagerLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	if len(mgr.Projects) == 0 {
		t.Fatal("expected default projects to be initialized")
	}

	active := mgr.ActiveProject()
	if active == nil {
		t.Fatal("expected active project")
	}

	// Test Element Pinning Toggle
	if mgr.IsPinning {
		t.Fatal("expected pinning mode to be false initially")
	}
	mgr.TogglePinning()
	if !mgr.IsPinning {
		t.Fatal("expected pinning mode to be active after toggle")
	}

	// Select a pin
	if len(active.PinnedElements) > 0 {
		pinID := active.PinnedElements[0].ID
		mgr.SelectPin(pinID)
		if mgr.SelectedPin == nil || mgr.SelectedPin.ID != pinID {
			t.Fatalf("expected selected pin %s, got %+v", pinID, mgr.SelectedPin)
		}
	}

	// Export active
	expRes, err := mgr.ExportActive()
	if err != nil {
		t.Fatalf("export active failed: %v", err)
	}
	if expRes == nil || expRes.PrimaryFilePath == "" {
		t.Fatal("expected export result with primary file")
	}

	// Verify persistence in .forge/studio/projects.json
	projJSON := filepath.Join(tmpDir, ".forge", "studio", "projects.json")
	if _, err := os.Stat(projJSON); err != nil {
		t.Fatalf("expected projects.json to be saved at %s", projJSON)
	}
}

func TestStudioConfig(t *testing.T) {
	cfg := DefaultStudioConfig()
	if cfg.ImageModel != "flux-1-schnell" {
		t.Fatalf("expected default model flux-1-schnell, got %s", cfg.ImageModel)
	}
	cfg.ImageEndpoint = "http://my-custom-api:8000/v1/generate"
	cfg.ImageAPIKey = "custom-key-123"

	if err := SaveStudioConfig(cfg); err != nil {
		t.Fatalf("failed to save studio config: %v", err)
	}

	loaded := LoadStudioConfig()
	if loaded.ImageEndpoint != "http://my-custom-api:8000/v1/generate" {
		t.Fatalf("expected loaded endpoint, got %s", loaded.ImageEndpoint)
	}
	if loaded.ImageAPIKey != "custom-key-123" {
		t.Fatalf("expected loaded key, got %s", loaded.ImageAPIKey)
	}
}
