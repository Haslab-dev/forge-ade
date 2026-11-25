package ui

import (
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/engine"
	corestudio "github.com/hasdev/forge-ade/core/studio"
)

func TestStudioFullNativeWorkflowWithoutWebView(t *testing.T) {
	dir := t.TempDir()
	eng := engine.NewEngine(dir)
	app := NewAppUI(eng, dir)
	app.setMode(ModeStudio)

	// Switch to Image Studio
	app.studioEnsureProjectForType(corestudio.ArtifactTypeImage)
	proj := app.Studio.ActiveProject()
	if proj == nil || proj.Type != corestudio.ArtifactTypeImage {
		t.Fatalf("expected active image studio project, got %+v", proj)
	}

	wf := app.activeStudioWorkflow(proj)
	if wf == nil || wf.Mode != "pipeline" || len(wf.Steps) == 0 {
		t.Fatalf("expected initialized native image pipeline, got %+v", wf)
	}
	if app.StudioNodeWebView != nil {
		t.Fatal("expected StudioNodeWebView to remain nil in full-native mode")
	}

	// Inject a reference path and verify Reference step is prepended/updated
	app.studioInjectReferencePath(wf, "/tmp/ref_frame.png")
	if wf.Steps[0].Type != corestudio.StepTypeReference || wf.Steps[0].GetString("path", "") != "/tmp/ref_frame.png" {
		t.Fatalf("expected Reference step with injected path, got %+v", wf.Steps[0])
	}

	// Execute native pipeline via studioQueuePrompt
	app.studioQueuePrompt()
	deadline := time.Now().Add(5 * time.Second)
	for app.StudioIsGenerating && time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
	}
	if app.StudioIsGenerating {
		t.Fatal("expected native workflow generation to finish")
	}
	if app.StudioGeneratedImagePath == "" {
		t.Fatal("expected StudioGeneratedImagePath to be populated")
	}
	if _, err := os.Stat(app.StudioGeneratedImagePath); err != nil {
		t.Fatalf("expected generated asset to exist: %v", err)
	}
	if len(app.StudioWorkflowLogs) == 0 {
		t.Fatal("expected StudioWorkflowLogs to record executed stages")
	}

	// Switch to Injectable JSON Template preset and run
	tpl := corestudio.PresetImageJSONTemplate()
	app.setActiveStudioWorkflow(proj, &tpl)
	app.studioQueuePrompt()
	deadline = time.Now().Add(5 * time.Second)
	for app.StudioIsGenerating && time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
	}
	if app.StudioWorkflowLastInjectedJSON == "" {
		t.Fatal("expected StudioWorkflowLastInjectedJSON after running JSON template workflow")
	}

	// Verify strict domain separation of presets
	imgPresets := corestudio.PresetsForPipelineType("image")
	for _, p := range imgPresets {
		if p.PipelineType != "image" {
			t.Fatalf("unexpected non-image preset %s in image domain", p.ID)
		}
	}
	vidPresets := corestudio.PresetsForPipelineType("video")
	for _, p := range vidPresets {
		if p.PipelineType != "video" {
			t.Fatalf("unexpected non-video preset %s in video domain", p.ID)
		}
	}

	// Verify 1-click "Animate in Video" handoff
	app.studioSendImageToVideo(app.StudioGeneratedImagePath)
	if app.Studio.ActiveProject().Type != corestudio.ArtifactTypeVideo {
		t.Fatalf("expected active project switched to video, got %s", app.Studio.ActiveProject().Type)
	}
	if app.StudioVideoWorkflow == nil || app.StudioVideoWorkflow.Steps[0].GetString("path", "") != app.StudioGeneratedImagePath {
		t.Fatalf("expected generated image injected into Video Studio keyframe, got %+v", app.StudioVideoWorkflow)
	}

	// Verify revamped Image & Video workflow UI renders cleanly (both expanded and collapsed sidebar)
	app.studioEnsureProjectForType(corestudio.ArtifactTypeImage)
	twImg := ui.NewTesterWith(func(c *ui.Context) { app.View(c) }, 1280, 800, ui.TesterOptions{Dark: true})
	if _, ok := twImg.Find("IMAGE WORKFLOWS"); !ok {
		t.Fatal("expected IMAGE WORKFLOWS sidebar when expanded")
	}
	if _, ok := twImg.Find("Generate"); !ok {
		t.Fatal("expected Generate CTA in workflow topbar")
	}

	app.StudioWorkflowLeftCollapsed = true
	twCollapsed := ui.NewTesterWith(func(c *ui.Context) { app.View(c) }, 1280, 800, ui.TesterOptions{Dark: true})
	if _, ok := twCollapsed.Find("IMAGE WORKFLOWS"); ok {
		t.Fatal("expected IMAGE WORKFLOWS sidebar hidden when collapsed")
	}
}
