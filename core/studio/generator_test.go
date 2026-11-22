package studio

import (
	"context"
	"os"
	"testing"
)

func TestGenerateImageProceduralFallback(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "studio-gen-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	req := GenerationRequest{
		Prompt:      "Cyberpunk street with neon reflections",
		Model:       "flux-1-schnell",
		AspectRatio: "16:9",
		Steps:       25,
		CFGScale:    7.5,
		Seed:        12345,
		OutputDir:   tmpDir,
	}

	res, err := GenerateImage(context.Background(), req)
	if err != nil {
		t.Fatalf("GenerateImage failed: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil GenerationResult")
	}
	if res.FilePath == "" {
		t.Error("expected non-empty FilePath")
	}
	if _, err := os.Stat(res.FilePath); err != nil {
		t.Errorf("generated file does not exist at %s: %v", res.FilePath, err)
	}
	if res.Width != 1280 || res.Height != 720 {
		t.Errorf("expected 1280x720, got %dx%d", res.Width, res.Height)
	}
}
