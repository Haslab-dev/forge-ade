package studio

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ExportResult contains the paths and metadata of files created during handoff.
type ExportResult struct {
	PrimaryFilePath string   `json:"primaryFilePath"`
	ExportedFiles   []string `json:"exportedFiles"`
	Summary         string   `json:"summary"`
}

// ExportToWorkspace exports the project's active version directly into the workspace folder.
func ExportToWorkspace(workspaceDir string, project *StudioProject) (*ExportResult, error) {
	if workspaceDir == "" {
		return nil, fmt.Errorf("workspace directory is empty")
	}
	if project == nil {
		return nil, fmt.Errorf("project is nil")
	}

	active := project.ActiveVersion()
	if active == nil {
		return nil, fmt.Errorf("project has no versions to export")
	}

	exportDir := filepath.Join(workspaceDir, "studio_exports", sanitizeFileName(project.Name))
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create export directory: %w", err)
	}

	var primary string
	var files []string

	switch project.Type {
	case ArtifactTypeSlides:
		// 1. Export HTML Deck
		htmlPath := filepath.Join(exportDir, "deck.html")
		htmlContent := GenerateSlidesHTML(project.Name, active.Data)
		if err := os.WriteFile(htmlPath, []byte(htmlContent), 0644); err != nil {
			return nil, err
		}
		files = append(files, htmlPath)
		primary = htmlPath

		// 2. Export Slides JSON
		jsonPath := filepath.Join(exportDir, "deck.json")
		if data, err := json.MarshalIndent(active.Data.Slides, "", "  "); err == nil {
			_ = os.WriteFile(jsonPath, data, 0644)
			files = append(files, jsonPath)
		}

	case ArtifactTypeMotion:
		// 1. Export Motion HTML
		motionPath := filepath.Join(exportDir, "motion.html")
		motionContent := GenerateMotionHTML(project.Name, active.Data)
		if err := os.WriteFile(motionPath, []byte(motionContent), 0644); err != nil {
			return nil, err
		}
		files = append(files, motionPath)
		primary = motionPath

		// 2. Export Timings & Keyframes
		metaPath := filepath.Join(exportDir, "motion_timeline.json")
		if data, err := json.MarshalIndent(map[string]interface{}{
			"script":    active.Data.ScriptText,
			"tts_words": active.Data.TTSWords,
			"keyframes": active.Data.Keyframes,
		}, "", "  "); err == nil {
			_ = os.WriteFile(metaPath, data, 0644)
			files = append(files, metaPath)
		}

	case ArtifactTypeCodeJS:
		// 1. Export JS Code
		jsPath := filepath.Join(exportDir, "scene.js")
		if err := os.WriteFile(jsPath, []byte(active.Data.Code), 0644); err != nil {
			return nil, err
		}
		files = append(files, jsPath)
		primary = jsPath

		// 2. Export Entry HTML
		htmlPath := filepath.Join(exportDir, "index.html")
		entryHTML := Generate3DSceneHTML(project.Name, active.Data.Code)
		if err := os.WriteFile(htmlPath, []byte(entryHTML), 0644); err == nil {
			files = append(files, htmlPath)
		}

	default: // Image & Video
		metaPath := filepath.Join(exportDir, "asset_manifest.json")
		if data, err := json.MarshalIndent(active.Data, "", "  "); err == nil {
			_ = os.WriteFile(metaPath, data, 0644)
			files = append(files, metaPath)
			primary = metaPath
		}
	}

	project.ExportPath = primary

	return &ExportResult{
		PrimaryFilePath: primary,
		ExportedFiles:   files,
		Summary:         fmt.Sprintf("Exported %d files for %s to workspace", len(files), project.Name),
	}, nil
}

func sanitizeFileName(name string) string {
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, " ", "_")
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	res := b.String()
	if res == "" {
		res = "project"
	}
	return res
}

// HTMLVersionMarker is embedded in every generated studio viewer HTML file;
// ensureStudioAssetFile regenerates stale files that lack the current marker
// so template improvements reach existing workspaces (cache-busting).
// v3: motion pause actually stops animations, 3D scene error overlay.
const HTMLVersionMarker = "<!-- forgeade-studio-html:v3 -->"

// DemoSlides is the fallback deck used when a slides project has no version
// data yet; the native viewer renders the same list so the toolbar,
// thumbnails, and the generated page never disagree.
func DemoSlides() []SlideItem {
	return []SlideItem{
		{
			ID:       "slide-1",
			Title:    "Autonomous Generative Systems",
			Subtitle: "Next-Generation Agentic IDE Architecture",
			Layout:   "hero",
			Theme:    "dark",
			Bullets: []string{
				"Decoupled 5-layer studio engine",
				"Interactive chat agent pairing (ACP & internal)",
				"Direct workspace handoff via ⌘⇧E",
			},
		},
		{
			ID:       "slide-2",
			Title:    "ComfyUI Dataflow Graph",
			Subtitle: "Custom APIs & Visual Workflows",
			Layout:   "split",
			Theme:    "accent",
			Bullets: []string{
				"Custom endpoint & model customization",
				"Offline procedural art fallback",
				"Real-time parameter manipulation",
			},
		},
	}
}

// GenerateSlidesHTML produces a complete, standalone 16:9 presentation HTML page with interactive navigation.
func GenerateSlidesHTML(title string, data ArtifactData) string {
	if len(data.Slides) == 0 {
		// Keep the exported page in agreement with the native viewer: both
		// fall back to the same demo deck when no slides exist yet.
		data.Slides = DemoSlides()
	}

	var slidesHTML strings.Builder
	for i, slide := range data.Slides {
		activeClass := ""
		if i == 0 {
			activeClass = "active"
		}
		slidesHTML.WriteString(fmt.Sprintf(`
    <section class="slide %s %s" id="%s" data-index="%d">
      <div class="slide-content">
        <span class="slide-number">0%d</span>
        <h1>%s</h1>
        <h3>%s</h3>
        <ul>
`, slide.Theme, activeClass, slide.ID, i, i+1, slide.Title, slide.Subtitle))
		for _, b := range slide.Bullets {
			slidesHTML.WriteString(fmt.Sprintf("          <li>%s</li>\n", b))
		}
		slidesHTML.WriteString(`        </ul>
      </div>
    </section>
`)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  %s
  <title>%s - Presentation</title>
  <style>
    :root { --bg: #090a0f; --text: #f3f4f6; --accent: #3b82f6; --surface: #12141c; --border: #1f2433; }
    * { box-sizing: border-box; }
    body { margin: 0; background: var(--bg); color: var(--text); font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Inter, sans-serif; display: flex; flex-direction: column; align-items: center; justify-content: center; min-height: 100vh; overflow: hidden; user-select: none; }
    .presentation-container { position: relative; width: 90vw; max-width: 1280px; aspect-ratio: 16/9; }
    .slide { display: none; width: 100%%; height: 100%%; background: var(--surface); border-radius: 14px; border: 1px solid var(--border); padding: 52px 64px; flex-direction: column; justify-content: center; position: absolute; top: 0; left: 0; box-shadow: 0 24px 60px rgba(0,0,0,0.6); animation: fadeIn 0.25s ease-out; }
    .slide.active { display: flex; }
    .slide.accent { background: linear-gradient(135deg, #12141c 0%%, #1e3a8a 100%%); border-color: var(--accent); }
    .slide-number { position: absolute; top: 28px; right: 36px; font-size: 13px; font-weight: 700; color: #64748b; letter-spacing: 0.05em; }
    h1 { font-size: 3rem; margin: 0 0 14px 0; letter-spacing: -0.03em; font-weight: 800; line-height: 1.15; }
    h3 { font-size: 1.4rem; color: #94a3b8; font-weight: 500; margin: 0 0 28px 0; }
    ul { margin: 0; padding-left: 24px; font-size: 1.25rem; line-height: 1.8; color: #cbd5e1; }
    li { margin-bottom: 8px; }
    .nav-bar { position: absolute; bottom: 20px; right: 32px; display: flex; gap: 8px; z-index: 10; }
    .nav-btn { background: rgba(255,255,255,0.08); border: 1px solid rgba(255,255,255,0.15); color: #fff; width: 32px; height: 32px; border-radius: 6px; cursor: pointer; display: grid; place-items: center; font-size: 14px; transition: background 0.15s; }
    .nav-btn:hover { background: rgba(255,255,255,0.18); }
    @keyframes fadeIn { from { opacity: 0.6; transform: scale(0.995); } to { opacity: 1; transform: scale(1); } }
  </style>
</head>
<body>
  <div class="presentation-container">
%s
    <div class="nav-bar">
      <button class="nav-btn" onclick="prevSlide()">◀</button>
      <button class="nav-btn" onclick="nextSlide()">▶</button>
    </div>
  </div>

  <script>
    let currentIdx = 0;
    const slides = document.querySelectorAll('.slide');
    function showSlide(idx) {
      if (slides.length === 0) return;
      currentIdx = (idx + slides.length) %% slides.length;
      slides.forEach((s, i) => {
        s.classList.toggle('active', i === currentIdx);
      });
    }
    function nextSlide() { showSlide(currentIdx + 1); }
    function prevSlide() { showSlide(currentIdx - 1); }
    window.addEventListener('keydown', (e) => {
      if (e.key === 'ArrowRight' || e.key === ' ') nextSlide();
      if (e.key === 'ArrowLeft') prevSlide();
    });
    window.addEventListener('message', (e) => {
      if (!e.data || e.data.type !== 'od:slide') return;
      if (e.data.action === 'next') nextSlide();
      if (e.data.action === 'prev') prevSlide();
      if (e.data.action === 'goto' && typeof e.data.index === 'number') showSlide(e.data.index);
    });
  </script>
</body>
</html>`, HTMLVersionMarker, title, slidesHTML.String())
}

// GenerateMotionHTML produces a standalone motion composition HTML viewport.
func GenerateMotionHTML(title string, data ArtifactData) string {
	script := data.ScriptText
	if script == "" {
		script = "ForgeADE Studio introduces a fluid agentic workspace where design skills meet real-time motion."
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  %s
  <title>%s - Motion Composition</title>
  <style>
    body { margin: 0; background: #08080a; color: #fff; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Inter, sans-serif; display: grid; place-items: center; height: 100vh; overflow: hidden; user-select: none; }
    .motion-viewport { width: min(92vw, 854px); aspect-ratio: 16 / 9; background: radial-gradient(circle at 50%% 40%%, #141724 0%%, #0a0b10 100%%); border-radius: 12px; border: 1px solid #1f2438; position: relative; overflow: hidden; display: flex; flex-direction: column; align-items: center; justify-content: center; box-shadow: 0 20px 60px rgba(0,0,0,0.7); }
    .kinetic-badge { font-size: clamp(9px, 1.2vw, 11px); font-weight: 700; color: #60a5fa; letter-spacing: 0.15em; text-transform: uppercase; margin-bottom: clamp(6px, 1.4vw, 12px); padding: 4px 12px; background: rgba(59,130,246,0.12); border: 1px solid rgba(59,130,246,0.3); border-radius: 999px; animation: pulse 2.5s infinite; }
    .headline { font-size: clamp(20px, 4.4vw, 38px); font-weight: 800; letter-spacing: -0.03em; text-align: center; max-width: 84%%; line-height: 1.2; background: linear-gradient(135deg, #ffffff 0%%, #93c5fd 60%%, #3b82f6 100%%); -webkit-background-clip: text; -webkit-text-fill-color: transparent; animation: springBounce 1.2s cubic-bezier(0.175, 0.885, 0.32, 1.275); }
    .waveform-bar { display: flex; gap: 4px; align-items: center; height: clamp(18px, 3.5vw, 32px); margin-top: clamp(10px, 2.5vw, 24px); }
    .wave-line { width: 4px; background: #3b82f6; border-radius: 2px; animation: waveAnim 1s ease-in-out infinite alternate; }
    .subtitles { position: absolute; bottom: 6%%; font-size: clamp(11px, 1.8vw, 15px); font-weight: 500; color: #e2e8f0; background: rgba(15,23,42,0.85); backdrop-filter: blur(8px); padding: 8px 24px; border-radius: 24px; border: 1px solid rgba(255,255,255,0.1); max-width: 88%%; text-align: center; }
    body.paused * { animation-play-state: paused !important; }
    @keyframes springBounce { 0%% { opacity: 0; transform: translateY(30px) scale(0.92); } 100%% { opacity: 1; transform: translateY(0) scale(1); } }
    @keyframes pulse { 0%%, 100%% { opacity: 0.8; } 50%% { opacity: 1; } }
    @keyframes waveAnim { from { height: 6px; opacity: 0.4; } to { height: 28px; opacity: 1; } }
  </style>
</head>
<body>
  <div class="motion-viewport">
    <div class="kinetic-badge">KINETIC AUDIO SYNC • 1080P 60FPS</div>
    <div class="headline">%s</div>
    <div class="waveform-bar">
      <div class="wave-line" style="animation-delay: 0.1s"></div>
      <div class="wave-line" style="animation-delay: 0.3s"></div>
      <div class="wave-line" style="animation-delay: 0.2s"></div>
      <div class="wave-line" style="animation-delay: 0.5s"></div>
      <div class="wave-line" style="animation-delay: 0.4s"></div>
      <div class="wave-line" style="animation-delay: 0.25s"></div>
    </div>
    <div class="subtitles">%s</div>
  </div>
  <script>
    let isPlaying = true;
    function setPlaying(p) {
      isPlaying = p;
      document.body.classList.toggle('paused', !p);
    }
    window.addEventListener('message', (e) => {
      if (!e.data || e.data.type !== 'od:motion-control') return;
      if (e.data.action === 'play') setPlaying(true);
      if (e.data.action === 'pause') setPlaying(false);
      if (e.data.action === 'restart') {
        const headline = document.querySelector('.headline');
        headline.style.animation = 'none';
        headline.offsetHeight;
        headline.style.animation = null;
      }
    });
  </script>
</body>
</html>`, HTMLVersionMarker, title, title, script)
}

// Generate3DSceneHTML produces a Three.js / WebGL 2.0 interactive sandbox viewport.
func Generate3DSceneHTML(title, code string) string {
	if code == "" {
		code = `
import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';

const scene = new THREE.Scene();
scene.background = new THREE.Color(0x0a0c14);

const camera = new THREE.PerspectiveCamera(60, window.innerWidth / window.innerHeight, 0.1, 1000);
camera.position.set(0, 0, 4.5);

const renderer = new THREE.WebGLRenderer({ antialias: true });
renderer.setSize(window.innerWidth, window.innerHeight);
renderer.setPixelRatio(window.devicePixelRatio);
document.body.appendChild(renderer.domElement);

const controls = new OrbitControls(camera, renderer.domElement);
controls.enableDamping = true;
controls.dampingFactor = 0.05;

// Glowing Torus Knot
const geometry = new THREE.TorusKnotGeometry(1.2, 0.38, 128, 32);
const material = new THREE.MeshStandardMaterial({
  color: 0x3b82f6,
  metalness: 0.85,
  roughness: 0.18,
  wireframe: false
});
const mesh = new THREE.Mesh(geometry, material);
scene.add(mesh);

// Lights
const light1 = new THREE.PointLight(0x60a5fa, 3, 50);
light1.position.set(5, 5, 5);
scene.add(light1);

const light2 = new THREE.PointLight(0xa855f7, 2.5, 50);
light2.position.set(-5, -5, -3);
scene.add(light2);

scene.add(new THREE.AmbientLight(0x1e293b, 1.5));

function animate() {
  requestAnimationFrame(animate);
  mesh.rotation.x += 0.008;
  mesh.rotation.y += 0.012;
  controls.update();
  renderer.render(scene, camera);
}
animate();

window.addEventListener('resize', () => {
  camera.aspect = window.innerWidth / window.innerHeight;
  camera.updateProjectionMatrix();
  renderer.setSize(window.innerWidth, window.innerHeight);
});

window.__sceneReady = true;
`
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  %s
  <title>%s - 3D WebGL Sandbox</title>
  <style>
    body { margin: 0; background: #0a0c14; overflow: hidden; }
    canvas { display: block; width: 100vw; height: 100vh; }
  </style>
  <script type="importmap">
  {
    "imports": {
      "three": "https://unpkg.com/three@0.160.0/build/three.module.js",
      "three/addons/": "https://unpkg.com/three@0.160.0/examples/jsm/"
    }
  }
  </script>
</head>
<body>
<div id="od-scene-error" style="display:none; position:fixed; inset:0; z-index:99; place-items:center; background:#0a0c14; color:#f87171; font:14px -apple-system, BlinkMacSystemFont, sans-serif; text-align:center; padding:24px;"></div>
<script>
  (function () {
    var el = document.getElementById('od-scene-error');
    function fail(msg) {
      el.style.display = 'grid';
      el.textContent = '⚠ 3D scene failed to load — ' + msg + ' (Three.js loads from unpkg.com; check your connection, then click Reload in the toolbar)';
    }
    window.addEventListener('error', function (e) { fail((e && e.message) || 'script error'); });
    window.addEventListener('unhandledrejection', function (e) { fail(String((e.reason && e.reason.message) || e.reason || 'module error')); });
  })();
</script>
<script type="module">
%s
</script>
</body>
</html>`, HTMLVersionMarker, title, code)
}
