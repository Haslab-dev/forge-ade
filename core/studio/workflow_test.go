package studio

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestWorkflowPipelineEditingAndRoundtrip(t *testing.T) {
	wf := PresetTextToImage()
	if len(wf.Steps) != 3 {
		t.Fatalf("expected 3 steps in PresetTextToImage, got %d", len(wf.Steps))
	}

	// Add Upscale step — should insert before trailing Output step
	up := NewDefaultStep(StepTypeUpscale, 99)
	wf.AddStep(up)
	if len(wf.Steps) != 4 {
		t.Fatalf("expected 4 steps after AddStep, got %d", len(wf.Steps))
	}
	if wf.Steps[2].ID != up.ID || wf.Steps[3].Type != StepTypeOutput {
		t.Fatalf("expected upscale inserted before output, got order: %s, %s", wf.Steps[2].Type, wf.Steps[3].Type)
	}

	// Move Upscale up by 1
	if !wf.MoveStep(up.ID, -1) {
		t.Fatal("expected MoveStep(-1) to succeed")
	}
	if wf.Steps[1].ID != up.ID {
		t.Fatalf("expected upscale at index 1, got %s", wf.Steps[1].ID)
	}

	// Toggle bypass
	if !wf.ToggleStep(up.ID) {
		t.Fatal("expected ToggleStep to succeed")
	}
	if wf.Steps[1].Enabled {
		t.Fatal("expected step to be bypassed (Enabled=false)")
	}

	// Export & re-import native pipeline JSON
	raw, err := wf.ExportJSON()
	if err != nil {
		t.Fatalf("ExportJSON failed: %v", err)
	}
	reimported, err := ImportWorkflowJSON(raw, "image")
	if err != nil {
		t.Fatalf("ImportWorkflowJSON failed: %v", err)
	}
	if reimported.Mode != "pipeline" || len(reimported.Steps) != 4 {
		t.Fatalf("unexpected reimported workflow: mode=%s steps=%d", reimported.Mode, len(reimported.Steps))
	}

	// Remove step
	if !wf.RemoveStep(up.ID) {
		t.Fatal("expected RemoveStep to succeed")
	}
	if len(wf.Steps) != 3 {
		t.Fatalf("expected 3 steps after RemoveStep, got %d", len(wf.Steps))
	}
}

func TestWorkflowJSONTemplateParseAndInject(t *testing.T) {
	tpl := PresetImageJSONTemplate()
	if tpl.Mode != "template" || len(tpl.Variables) < 4 {
		t.Fatalf("expected template preset with variables, got mode=%s vars=%d", tpl.Mode, len(tpl.Variables))
	}

	// Update variables and inject
	for i := range tpl.Variables {
		switch tpl.Variables[i].Key {
		case "positive_prompt":
			tpl.Variables[i].Value = "Neon samurai in Tokyo rain, ultra sharp 8k"
		case "model":
			tpl.Variables[i].Value = "flux-1-dev"
		case "steps":
			tpl.Variables[i].Value = "32"
		}
	}

	resolvedBytes, resolvedWf, err := InjectWorkflowJSON(tpl.TemplateJSON, tpl.Variables)
	if err != nil {
		t.Fatalf("InjectWorkflowJSON failed: %v", err)
	}
	if !strings.Contains(string(resolvedBytes), "Neon samurai in Tokyo rain, ultra sharp 8k") {
		t.Fatalf("expected injected prompt in resolved JSON, got: %s", string(resolvedBytes))
	}
	if len(resolvedWf.Steps) != 3 {
		t.Fatalf("expected 3 resolved steps, got %d", len(resolvedWf.Steps))
	}
	if got := resolvedWf.Steps[1].GetString("model", ""); got != "flux-1-dev" {
		t.Fatalf("expected injected model flux-1-dev, got %q", got)
	}
	if got := resolvedWf.Steps[1].GetInt("steps", 0); got != 32 {
		t.Fatalf("expected injected steps=32, got %d", got)
	}

	// Test ImportWorkflowJSON auto-detecting {{variables}} template mode
	imported, err := ImportWorkflowJSON([]byte(tpl.TemplateJSON), "image")
	if err != nil {
		t.Fatalf("ImportWorkflowJSON(template) failed: %v", err)
	}
	if imported.Mode != "template" || len(imported.Variables) == 0 {
		t.Fatalf("expected template mode with variables, got mode=%s vars=%d", imported.Mode, len(imported.Variables))
	}
}

func TestRunWorkflowPipelineAndTemplate(t *testing.T) {
	outDir := t.TempDir()

	// 1. Run Native Video Pipeline
	vidWf := PresetTextToVideo()
	res, err := RunWorkflow(context.Background(), &vidWf, WorkflowRunOptions{
		Config:    DefaultStudioConfig(),
		OutputDir: outDir,
	})
	if err != nil {
		t.Fatalf("RunWorkflow(video pipeline) failed: %v", err)
	}
	if res.FilePath == "" || !res.IsVideo {
		t.Fatalf("expected video output file, got %+v", res)
	}
	if _, err := os.Stat(res.FilePath); err != nil {
		t.Fatalf("expected output file on disk: %v", err)
	}
	if len(res.StepLogs) < 4 {
		t.Fatalf("expected step logs for pipeline stages, got %v", res.StepLogs)
	}

	// 2. Run Injectable JSON Template workflow
	tplWf := PresetImageJSONTemplate()
	tplRes, err := RunWorkflow(context.Background(), &tplWf, WorkflowRunOptions{
		Config:    DefaultStudioConfig(),
		OutputDir: outDir,
	})
	if err != nil {
		t.Fatalf("RunWorkflow(template) failed: %v", err)
	}
	if tplRes.FilePath == "" || tplRes.InjectedJSON == "" {
		t.Fatalf("expected output file and injected JSON, got %+v", tplRes)
	}
	if _, err := os.Stat(tplRes.FilePath); err != nil {
		t.Fatalf("expected generated file on disk: %v", err)
	}
}
