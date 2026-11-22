package studio

import (
	"strings"
	"testing"
)

func TestGenerateSlidesHTMLDemoFallback(t *testing.T) {
	html := GenerateSlidesHTML("Deck", ArtifactData{})
	if !strings.Contains(html, "Autonomous Generative Systems") {
		t.Fatal("expected demo deck fallback when slides data is empty")
	}
	if !strings.Contains(html, HTMLVersionMarker) {
		t.Fatal("expected version marker in generated deck")
	}
}

func TestGenerateMotionHTMLPauseSupport(t *testing.T) {
	html := GenerateMotionHTML("Motion", ArtifactData{})
	if !strings.Contains(html, "animation-play-state: paused") {
		t.Fatal("expected motion page to pause animations via body.paused")
	}
	if !strings.Contains(html, "setPlaying") {
		t.Fatal("expected setPlaying helper in motion page")
	}
}

func TestGenerate3DSceneHTMLErrorOverlay(t *testing.T) {
	html := Generate3DSceneHTML("Scene", "")
	if !strings.Contains(html, "od-scene-error") {
		t.Fatal("expected error overlay in the 3D scene page")
	}
	if !strings.Contains(html, "__sceneReady") {
		t.Fatal("expected default scene code to signal readiness")
	}
}
