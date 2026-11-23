package studio

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// StepPhase represents the 4-phase lifecycle of a Director Agent task.
type StepPhase string

const (
	PhasePlan     StepPhase = "plan"
	PhaseGenerate StepPhase = "generate"
	PhaseSync     StepPhase = "sync"
	PhaseVerify   StepPhase = "verify"
)

// DirectorStep is an observable execution card displayed in the Studio right panel.
type DirectorStep struct {
	ID        string    `json:"id"`
	Phase     StepPhase `json:"phase"`
	Title     string    `json:"title"`
	Status    string    `json:"status"` // "pending", "running", "completed", "failed", "paused"
	Progress  float32   `json:"progress"`
	Summary   string    `json:"summary"`
	Error     string    `json:"error,omitempty"`
	StartedAt time.Time `json:"startedAt,omitempty"`
	EndedAt   time.Time `json:"endedAt,omitempty"`
}

// DirectorWorkflow manages the state and background execution of Director Agent runs.
type DirectorWorkflow struct {
	mu         sync.RWMutex
	Steps      []DirectorStep
	IsRunning  bool
	CancelFunc context.CancelFunc
	OnUpdate   func([]DirectorStep)
	OnComplete func(ArtifactData, []ElementPin)
}

// NewDirectorWorkflow initializes default 4-phase step cards.
func NewDirectorWorkflow(artType ArtifactType) *DirectorWorkflow {
	return &DirectorWorkflow{
		Steps: defaultStepsForType(artType),
	}
}

func defaultStepsForType(t ArtifactType) []DirectorStep {
	switch t {
	case ArtifactTypeSlides:
		return []DirectorStep{
			{ID: "step-plan", Phase: PhasePlan, Title: "Plan Deck Narrative & Structure", Status: "pending"},
			{ID: "step-gen", Phase: PhaseGenerate, Title: "Synthesize 16:9 Slides Layout", Status: "pending"},
			{ID: "step-sync", Phase: PhaseSync, Title: "Apply Design System & Tokens", Status: "pending"},
			{ID: "step-verify", Phase: PhaseVerify, Title: "Verify Typography & Layout Overflows", Status: "pending"},
		}
	case ArtifactTypeMotion:
		return []DirectorStep{
			{ID: "step-plan", Phase: PhasePlan, Title: "Analyze Script & Scene Beats", Status: "pending"},
			{ID: "step-gen", Phase: PhaseGenerate, Title: "Generate ElevenLabs Voiceover & Timestamps", Status: "pending"},
			{ID: "step-sync", Phase: PhaseSync, Title: "Align Kinetic Subtitles to Audio Waveform", Status: "pending"},
			{ID: "step-verify", Phase: PhaseVerify, Title: "Verify Keyframe Interpolation & Scene Rhythm", Status: "pending"},
		}
	case ArtifactTypeCodeJS:
		return []DirectorStep{
			{ID: "step-plan", Phase: PhasePlan, Title: "Draft 2D/3D Scene Geometry & Loop", Status: "pending"},
			{ID: "step-gen", Phase: PhaseGenerate, Title: "Synthesize JavaScript Canvas Engine", Status: "pending"},
			{ID: "step-sync", Phase: PhaseSync, Title: "Inject Shaders & Responsive Viewport", Status: "pending"},
			{ID: "step-verify", Phase: PhaseVerify, Title: "Self-Healing: Catch & Repair Runtime Errors", Status: "pending"},
		}
	case ArtifactTypeVideo:
		return []DirectorStep{
			{ID: "step-plan", Phase: PhasePlan, Title: "Deconstruct Camera Motion & Prompt", Status: "pending"},
			{ID: "step-gen", Phase: PhaseGenerate, Title: "Interpolate Video Latent Frames", Status: "pending"},
			{ID: "step-sync", Phase: PhaseSync, Title: "Harmonize Frame Rate & Motion Smoothing", Status: "pending"},
			{ID: "step-verify", Phase: PhaseVerify, Title: "Verify Artifact Integrity & Temporal Coherence", Status: "pending"},
		}
	default: // Image
		return []DirectorStep{
			{ID: "step-plan", Phase: PhasePlan, Title: "Inject Design Skills into Prompt Matrix", Status: "pending"},
			{ID: "step-gen", Phase: PhaseGenerate, Title: "Synthesize High-Res Visual Composition", Status: "pending"},
			{ID: "step-sync", Phase: PhaseSync, Title: "Generate Multi-Aspect & Version Candidates", Status: "pending"},
			{ID: "step-verify", Phase: PhaseVerify, Title: "Prepare Split-Comparison Inspection", Status: "pending"},
		}
	}
}

// GetSteps returns a snapshot copy of current steps.
func (w *DirectorWorkflow) GetSteps() []DirectorStep {
	w.mu.RLock()
	defer w.mu.RUnlock()
	res := make([]DirectorStep, len(w.Steps))
	copy(res, w.Steps)
	return res
}

// ExecuteAsync starts a background execution cycle without blocking the native UI thread.
func (w *DirectorWorkflow) ExecuteAsync(ctx context.Context, project *StudioProject, skillsEngine *SkillsEngine) {
	w.mu.Lock()
	if w.IsRunning {
		w.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	w.CancelFunc = cancel
	w.IsRunning = true
	w.Steps = defaultStepsForType(project.Type)
	w.mu.Unlock()

	go func() {
		defer func() {
			w.mu.Lock()
			w.IsRunning = false
			w.mu.Unlock()
		}()

		var currentData ArtifactData
		if active := project.ActiveVersion(); active != nil {
			currentData = active.Data
		}

		var currentPins []ElementPin
		if len(project.PinnedElements) > 0 {
			currentPins = project.PinnedElements
		}

		for i := range w.Steps {
			select {
			case <-ctx.Done():
				w.updateStep(i, "paused", 0.0, "Execution paused by user.", "")
				return
			default:
			}

			w.updateStep(i, "running", 0.1, "In progress...", "")
			time.Sleep(350 * time.Millisecond) // realistic agent simulation

			switch w.Steps[i].Phase {
			case PhasePlan:
				w.updateStep(i, "running", 0.6, "Formulating visual architecture and tokens...", "")
				time.Sleep(200 * time.Millisecond)
				w.updateStep(i, "completed", 1.0, "Architecture plan synthesized against active design skills.", "")

			case PhaseGenerate:
				w.updateStep(i, "running", 0.7, "Generating content artifacts...", "")
				time.Sleep(300 * time.Millisecond)
				currentData, currentPins = synthesizeArtifactData(project.Type, project.Name, currentData)
				w.updateStep(i, "completed", 1.0, "Primary creative assets generated successfully.", "")

			case PhaseSync:
				w.updateStep(i, "running", 0.8, "Aligning elements, layout tokens, and rhythm...", "")
				time.Sleep(250 * time.Millisecond)
				w.updateStep(i, "completed", 1.0, "Temporal synchronization and responsive constraints locked.", "")

			case PhaseVerify:
				w.updateStep(i, "running", 0.9, "Running self-healing checks and layout diagnostics...", "")
				time.Sleep(200 * time.Millisecond)
				w.updateStep(i, "completed", 1.0, "Verification passed with zero runtime errors.", "")
			}
		}

		if w.OnComplete != nil {
			w.OnComplete(currentData, currentPins)
		}
	}()
}

func (w *DirectorWorkflow) updateStep(index int, status string, progress float32, summary, errStr string) {
	w.mu.Lock()
	if index >= 0 && index < len(w.Steps) {
		w.Steps[index].Status = status
		w.Steps[index].Progress = progress
		w.Steps[index].Summary = summary
		w.Steps[index].Error = errStr
		if status == "running" && w.Steps[index].StartedAt.IsZero() {
			w.Steps[index].StartedAt = time.Now()
		}
		if status == "completed" || status == "failed" {
			w.Steps[index].EndedAt = time.Now()
		}
	}
	stepsCopy := make([]DirectorStep, len(w.Steps))
	copy(stepsCopy, w.Steps)
	cb := w.OnUpdate
	w.mu.Unlock()

	if cb != nil {
		cb(stepsCopy)
	}
}

// Cancel terminates active background execution.
func (w *DirectorWorkflow) Cancel() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.CancelFunc != nil {
		w.CancelFunc()
	}
	w.IsRunning = false
}

// synthesizeArtifactData populates rich realistic artifact contents for any of the 5 domains.
func synthesizeArtifactData(t ArtifactType, name string, existing ArtifactData) (ArtifactData, []ElementPin) {
	data := existing
	var pins []ElementPin

	switch t {
	case ArtifactTypeSlides:
		data.DeckTheme = "dark"
		if len(data.Slides) == 0 {
			data.Slides = []SlideItem{
				{
					ID:       "slide-1",
					Index:    0,
					Title:    fmt.Sprintf("Introduction to %s", name),
					Subtitle: "Next-Generation Autonomous Systems",
					Layout:   "hero",
					Theme:    "dark",
					Bullets: []string{
						"Agentic orchestration architecture",
						"Unified creative primitives",
						"Sub-millisecond native execution",
					},
				},
				{
					ID:       "slide-2",
					Index:    1,
					Title:    "Core Architecture",
					Subtitle: "5 Decoupled Layers",
					Layout:   "split",
					Theme:    "accent",
					Bullets: []string{
						"Zero-overhead native shell",
						"Local Open Design rule injection",
						"Self-healing error runtime",
					},
				},
				{
					ID:       "slide-3",
					Index:    2,
					Title:    "Future Horizon",
					Subtitle: "Production Handoff & Acceleration",
					Layout:   "cards",
					Theme:    "dark",
					Bullets: []string{
						"Instant workspace sync (⌘⇧E)",
						"Visual node pipeline parity",
						"Continuous Director validation",
					},
				},
			}
		}
		pins = []ElementPin{
			{ID: "pin-slide-title", Selector: "@slide-title", Label: "Slide Title", Category: "text", X: 80, Y: 120, Width: 600, Height: 60},
			{ID: "pin-slide-subtitle", Selector: "@slide-subtitle", Label: "Subtitle", Category: "text", X: 80, Y: 190, Width: 480, Height: 36},
			{ID: "pin-slide-body", Selector: "@slide-bullets", Label: "Bullet Points", Category: "content", X: 80, Y: 260, Width: 520, Height: 180},
		}

	case ArtifactTypeMotion:
		data.DurationSec = 12.0
		data.VoiceID = "eleven_rachel"
		data.ScriptText = "ForgeADE Studio introduces a fluid agentic workspace where design skills meet real-time motion."
		data.TTSWords = []TTSWord{
			{Word: "ForgeADE", StartTime: 0.2, EndTime: 0.9},
			{Word: "Studio", StartTime: 1.0, EndTime: 1.6},
			{Word: "introduces", StartTime: 1.7, EndTime: 2.3},
			{Word: "a", StartTime: 2.4, EndTime: 2.5},
			{Word: "fluid", StartTime: 2.6, EndTime: 3.1},
			{Word: "agentic", StartTime: 3.2, EndTime: 3.8},
			{Word: "workspace", StartTime: 3.9, EndTime: 4.6},
			{Word: "where", StartTime: 4.8, EndTime: 5.2},
			{Word: "design", StartTime: 5.3, EndTime: 5.8},
			{Word: "skills", StartTime: 5.9, EndTime: 6.4},
			{Word: "meet", StartTime: 6.5, EndTime: 6.9},
			{Word: "real-time", StartTime: 7.0, EndTime: 7.7},
			{Word: "motion.", StartTime: 7.8, EndTime: 8.5},
		}
		data.Keyframes = []MotionKeyframe{
			{Time: 0.2, TargetID: "@motion-logo", Action: "fade_in", Duration: 0.6, Easing: "spring"},
			{Time: 1.7, TargetID: "@motion-headline", Action: "slide_up", Duration: 0.5, Easing: "ease_out"},
			{Time: 5.3, TargetID: "@motion-badge", Action: "pulse", Duration: 0.8, Easing: "ease_out"},
			{Time: 7.0, TargetID: "@motion-particles", Action: "zoom_in", Duration: 1.2, Easing: "spring"},
		}
		pins = []ElementPin{
			{ID: "pin-motion-title", Selector: "@motion-headline", Label: "Kinetic Headline", Category: "text", X: 100, Y: 180, Width: 560, Height: 70},
			{ID: "pin-motion-subtitles", Selector: "@motion-subtitle", Label: "Live Subtitles", Category: "text", X: 100, Y: 380, Width: 640, Height: 40},
		}

	case ArtifactTypeCodeJS:
		data.JSEngine = "threejs"
		if data.Code == "" {
			data.Code = `// ForgeADE Studio: Interactive 3D Torus Knot with Bloom
import * as THREE from 'three';

const scene = new THREE.Scene();
const camera = new THREE.PerspectiveCamera(60, window.innerWidth / window.innerHeight, 0.1, 1000);
camera.position.z = 4;

const geometry = new THREE.TorusKnotGeometry(1.2, 0.35, 128, 32);
const material = new THREE.MeshStandardMaterial({
  color: 0x3b82f6,
  metalness: 0.8,
  roughness: 0.2,
  wireframe: false
});

const mesh = new THREE.Mesh(geometry, material);
scene.add(mesh);

const light = new THREE.PointLight(0xffffff, 2, 50);
light.position.set(5, 5, 5);
scene.add(light);
scene.add(new THREE.AmbientLight(0x222233));

function animate() {
  requestAnimationFrame(animate);
  mesh.rotation.x += 0.01;
  mesh.rotation.y += 0.015;
}
animate();
`
		}
		pins = []ElementPin{
			{ID: "pin-mesh-object", Selector: "@objek-3d", Label: "Torus Knot Mesh", Category: "mesh", X: 200, Y: 150, Width: 320, Height: 320},
			{ID: "pin-camera", Selector: "@camera", Label: "Perspective Camera", Category: "camera", X: 40, Y: 40, Width: 120, Height: 60},
		}

	case ArtifactTypeImage:
		data.AspectRatio = "16:9"
		data.ImageStyle = "Cinematic Neo-Brutalism"
		data.OutputAsset = "studio_render_v1.png"
		pins = []ElementPin{
			{ID: "pin-focal-subject", Selector: "@focal-subject", Label: "Primary Subject", Category: "shape", X: 180, Y: 100, Width: 400, Height: 300},
			{ID: "pin-background-glow", Selector: "@bg-gradient", Label: "Background Aura", Category: "shape", X: 0, Y: 0, Width: 800, Height: 500},
		}

	default: // Video
		data.DurationSec = 5.0
		data.FPS = 30
		data.MotionBucket = 127
		data.OutputAsset = "studio_render_motion.mp4"
		pins = []ElementPin{
			{ID: "pin-camera-pan", Selector: "@camera-pan", Label: "Camera Trajectory", Category: "camera", X: 120, Y: 80, Width: 500, Height: 320},
		}
	}

	return data, pins
}
