package studio

import (
	"strings"
	"testing"
)

func TestGenerateNodeWorkflowHTML(t *testing.T) {
	// 1. Image Pipeline
	imgState := NodeWorkflowInitialState{
		PipelineType:   "image",
		PositivePrompt: "A cyberpunk city in rain",
		Model:          "flux-1-schnell",
		Steps:          25,
		CFGScale:       7.5,
	}
	htmlImg := GenerateNodeWorkflowHTML("image", imgState)
	if !strings.Contains(htmlImg, "IMAGE PIPELINE (FLUX / SDXL)") {
		t.Fatal("expected image pipeline badge in generated HTML")
	}
	if !strings.Contains(htmlImg, "KSampler Engine") {
		t.Fatal("expected KSampler node in image pipeline HTML")
	}
	if !strings.Contains(htmlImg, "A cyberpunk city in rain") {
		t.Fatal("expected positive prompt embedded in HTML")
	}
	if !strings.Contains(htmlImg, "cables-svg") {
		t.Fatal("expected cables svg layer in HTML")
	}

	// 2. Video Pipeline
	vidState := NodeWorkflowInitialState{
		PipelineType:   "video",
		PositivePrompt: "Drone shot over alpine mountains",
		Model:          "veo-2-ai",
		Duration:       5,
		FPS:            30,
	}
	htmlVid := GenerateNodeWorkflowHTML("video", vidState)
	if !strings.Contains(htmlVid, "VIDEO PIPELINE") {
		t.Fatal("expected video pipeline badge in generated HTML")
	}
	if !strings.Contains(htmlVid, "Video Latent Sampler") {
		t.Fatal("expected Video Latent Sampler node in video pipeline HTML")
	}
	if !strings.Contains(htmlVid, "Drone shot over alpine mountains") {
		t.Fatal("expected video prompt embedded in HTML")
	}
	if !strings.Contains(htmlVid, "Storyboard & Shot Plan") {
		t.Fatal("expected storyboard/plan node in video flow HTML")
	}
	if !strings.Contains(htmlVid, "Reference Media") {
		t.Fatal("expected reference media node in video flow HTML")
	}
	if !strings.Contains(htmlVid, "Flow Assistant") {
		t.Fatal("expected Google Flow assistant sidebar in HTML")
	}
	if !strings.Contains(htmlVid, "Media Board") {
		t.Fatal("expected Media Board gallery switcher in HTML")
	}
}
