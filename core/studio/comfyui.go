package studio

import (
	"encoding/json"
)

// ComfyUIInitialState holds state passed into the webview on initial render.
type ComfyUIInitialState struct {
	PipelineType   string                 `json:"pipelineType"` // "image" or "video"
	PositivePrompt string                 `json:"positivePrompt"`
	NegativePrompt string                 `json:"negativePrompt"`
	Model          string                 `json:"model"`
	Sampler        string                 `json:"sampler"`
	Steps          int                    `json:"steps"`
	CFGScale       float32                `json:"cfgScale"`
	Seed           int64                  `json:"seed"`
	AspectRatio    string                 `json:"aspectRatio"`
	Duration       float32                `json:"duration"`
	FPS            int                    `json:"fps"`
	MotionBucket   int                    `json:"motionBucket"`
	ResultURL      string                 `json:"resultUrl"`
	ResultFormat   string                 `json:"resultFormat"`
	ResultWidth    int                    `json:"resultWidth"`
	ResultHeight   int                    `json:"resultHeight"`
	IsGenerating   bool                   `json:"isGenerating"`
	Progress       float32                `json:"progress"`
	StatusText     string                 `json:"statusText"`
	Values         map[string]interface{} `json:"values,omitempty"`
}

// GenerateComfyUIHTML returns an interactive, zero-dependency HTML5/Canvas/SVG ComfyUI
// visual node graph webview application tailored for Forge ADE Studio.
func GenerateComfyUIHTML(pipelineType string, state ComfyUIInitialState) string {
	if state.PipelineType == "" {
		state.PipelineType = pipelineType
	}
	if state.PositivePrompt == "" {
		if state.PipelineType == "video" {
			state.PositivePrompt = "Cinematic slow motion aerial shot of futuristic neon megalopolis at dusk, ultra realistic 4k"
		} else {
			state.PositivePrompt = "Cyberpunk neon street at dusk, cinematic volumetric fog, raytraced reflections, 8k octane render"
		}
	}
	if state.NegativePrompt == "" {
		state.NegativePrompt = "blurry, deformed, oversaturated, low quality, watermarks, bad anatomy"
	}
	if state.Model == "" {
		if state.PipelineType == "video" {
			state.Model = "veo-2-ai"
		} else {
			state.Model = "flux-1-schnell"
		}
	}
	if state.Steps <= 0 {
		state.Steps = 25
	}
	if state.CFGScale <= 0 {
		state.CFGScale = 7.5
	}
	if state.AspectRatio == "" {
		state.AspectRatio = "16:9"
	}

	stateJSON, err := json.Marshal(state)
	if err != nil {
		stateJSON = []byte("{}")
	}

	html := `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0, user-scalable=no">
<title>ComfyUI Visual Node Workflow</title>
<style>
  :root {
    --bg-canvas: #0b0d13;
    --grid-dot: rgba(255, 255, 255, 0.08);
    --node-bg: #141724;
    --node-header-bg: #1b2032;
    --node-border: #262c45;
    --node-border-active: #6366f1;
    --text-primary: #f1f5f9;
    --text-secondary: #94a3b8;
    --text-muted: #64748b;
    --accent: #6366f1;
    --accent-glow: rgba(99, 102, 241, 0.35);
    --color-model: #a855f7;
    --color-clip: #f59e0b;
    --color-latent: #ec4899;
    --color-vae: #ef4444;
    --color-image: #06b6d4;
    --color-params: #10b981;
  }

  * { box-sizing: border-box; margin: 0; padding: 0; user-select: none; }
  body, html {
    width: 100%; height: 100%; overflow: hidden;
    background-color: var(--bg-canvas);
    color: var(--text-primary);
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
  }

  #canvas-container {
    width: 100%; height: 100%; position: relative; overflow: hidden;
    cursor: grab;
    background-image: radial-gradient(var(--grid-dot) 1.2px, transparent 1.2px);
    background-size: 24px 24px;
    background-position: 0 0;
  }
  #canvas-container:active { cursor: grabbing; }

  #world {
    position: absolute; top: 0; left: 0;
    width: 100%; height: 100%;
    transform-origin: 0 0;
  }

  #cables-svg {
    position: absolute; top: 0; left: 0; width: 6000px; height: 6000px;
    pointer-events: none; z-index: 1;
  }

  .cable-path {
    fill: none; stroke-width: 3.5px;
    stroke-linecap: round;
    transition: stroke-width 0.2s;
  }
  .cable-path.animating {
    stroke-dasharray: 8 6;
    animation: cableFlow 1s linear infinite;
  }
  @keyframes cableFlow {
    from { stroke-dashoffset: 28; }
    to { stroke-dashoffset: 0; }
  }

  /* Node Card */
  .node-card {
    position: absolute;
    width: 290px;
    background: var(--node-bg);
    border: 1px solid var(--node-border);
    border-radius: 10px;
    box-shadow: 0 10px 28px rgba(0, 0, 0, 0.45);
    z-index: 10;
    cursor: default;
    transition: border-color 0.15s, box-shadow 0.15s;
  }
  .node-card.selected {
    border-color: var(--node-border-active);
    box-shadow: 0 0 0 2px var(--accent-glow), 0 12px 32px rgba(0, 0, 0, 0.6);
  }
  .node-card.executing {
    border-color: #38bdf8;
    box-shadow: 0 0 16px rgba(56, 189, 248, 0.5);
  }

  .node-header {
    height: 34px; padding: 0 12px;
    background: var(--node-header-bg);
    border-top-left-radius: 9px; border-top-right-radius: 9px;
    border-bottom: 1px solid var(--node-border);
    display: flex; align-items: center; gap: 8px;
    cursor: move;
  }
  .node-badge {
    width: 8px; height: 8px; border-radius: 4px;
    background: var(--accent);
  }
  .node-title {
    font-size: 11.5px; font-weight: 700; color: var(--text-primary);
    flex: 1; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  }
  .node-type-label {
    font-size: 9px; font-weight: 600; text-transform: uppercase;
    color: var(--text-muted);
  }

  .node-body {
    padding: 12px 14px; display: flex; flex-direction: column; gap: 10px;
  }

  /* Sockets */
  .socket-row {
    display: flex; align-items: center; justify-content: space-between;
    margin: -4px 0; font-size: 10px; font-weight: 600; color: var(--text-secondary);
  }
  .socket-slot {
    display: flex; align-items: center; gap: 6px; position: relative;
  }
  .socket-slot.output { margin-left: auto; }
  .port-circle {
    width: 11px; height: 11px; border-radius: 50%;
    border: 2px solid var(--node-bg);
    cursor: pointer;
    box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.2);
  }
  .port-circle:hover { transform: scale(1.3); }

  /* Controls & Inputs */
  .field-label {
    font-size: 10px; font-weight: 600; color: var(--text-muted);
    display: flex; justify-content: space-between; align-items: center;
  }
  .field-input, .field-textarea, .field-select {
    width: 100%; background: #0c0e17;
    border: 1px solid #1e2438; border-radius: 6px;
    color: var(--text-primary); font-size: 11px;
    padding: 6px 8px; outline: none;
    font-family: inherit;
    transition: border-color 0.15s;
  }
  .field-input:focus, .field-textarea:focus, .field-select:focus {
    border-color: var(--accent);
  }
  .field-textarea {
    resize: vertical; min-height: 54px; line-height: 1.4;
  }
  .field-slider-row {
    display: flex; align-items: center; gap: 8px;
  }
  .field-slider {
    flex: 1; accent-color: var(--accent); cursor: pointer;
  }
  .field-slider-val {
    font-size: 10.5px; font-weight: 700; color: var(--accent);
    min-width: 32px; text-align: right;
  }

  /* Style preset chips */
  .preset-chips {
    display: flex; flex-wrap: wrap; gap: 4px; margin-top: 2px;
  }
  .preset-chip {
    font-size: 9px; font-weight: 500;
    padding: 2px 7px; border-radius: 4px;
    background: #1b2033; color: var(--text-secondary);
    border: 1px solid #27304d; cursor: pointer;
    transition: all 0.15s;
  }
  .preset-chip:hover {
    background: var(--accent); color: #fff; border-color: var(--accent);
  }

  /* Preview Container */
  .preview-box {
    width: 100%; min-height: 190px; max-height: 280px;
    border-radius: 7px; overflow: hidden;
    background: #07080d; border: 1px dashed #262c44;
    display: flex; align-items: center; justify-content: center;
    position: relative;
  }
  .preview-box img, .preview-box video {
    width: 100%; height: 100%; object-fit: contain;
  }
  .preview-empty {
    font-size: 10.5px; color: var(--text-muted); text-align: center;
    padding: 20px;
  }

  /* Top Navigation & Controls Overlay */
  .hud-topbar {
    position: absolute; top: 12px; left: 16px; right: 16px;
    height: 40px; border-radius: 8px;
    background: rgba(20, 23, 36, 0.85); backdrop-filter: blur(10px);
    border: 1px solid rgba(255, 255, 255, 0.1);
    display: flex; align-items: center; padding: 0 12px; gap: 10px;
    z-index: 100;
    box-shadow: 0 4px 18px rgba(0, 0, 0, 0.4);
  }
  .hud-pill {
    padding: 3px 9px; border-radius: 5px;
    background: rgba(99, 102, 241, 0.18); border: 1px solid rgba(99, 102, 241, 0.4);
    font-size: 10px; font-weight: 700; color: #a5b4fc;
    letter-spacing: 0.5px;
  }
  .hud-btn {
    height: 28px; padding: 0 12px; border-radius: 6px;
    font-size: 11px; font-weight: 600; cursor: pointer;
    display: flex; align-items: center; gap: 6px;
    border: 1px solid transparent; transition: all 0.15s;
  }
  .hud-btn-primary {
    background: var(--accent); color: #fff;
    box-shadow: 0 2px 10px var(--accent-glow);
  }
  .hud-btn-primary:hover {
    background: #4f46e5;
  }
  .hud-btn-secondary {
    background: #1e2438; color: var(--text-secondary);
    border-color: #2e3754;
  }
  .hud-btn-secondary:hover {
    background: #28304a; color: var(--text-primary);
  }

  .hud-dock {
    position: absolute; bottom: 16px; right: 16px;
    background: rgba(20, 23, 36, 0.85); backdrop-filter: blur(10px);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 8px; padding: 4px; display: flex; gap: 4px;
    z-index: 100;
  }
  .dock-btn {
    width: 28px; height: 28px; border-radius: 5px;
    background: transparent; border: none;
    color: var(--text-secondary); font-size: 13px; font-weight: 700;
    cursor: pointer; display: flex; align-items: center; justify-content: center;
    transition: all 0.15s;
  }
  .dock-btn:hover { background: #2a314b; color: var(--text-primary); }

  .status-text {
    font-size: 10px; color: var(--text-muted);
  }
</style>
</head>
<body>

<div id="canvas-container">
  <!-- Top HUD Action Bar -->
  <div class="hud-topbar">
    <div class="hud-pill" id="pipeline-badge">IMAGE PIPELINE</div>
    <span class="status-text" id="status-display">Ready • 5 Nodes Active</span>
    <div style="flex: 1;"></div>
    <button class="hud-btn hud-btn-secondary" onclick="resetCanvasLayout()">↺ Reset Layout</button>
    <button class="hud-btn hud-btn-primary" id="btn-queue-prompt" onclick="queuePrompt()">
      <span id="queue-icon">⚡</span>
      <span id="queue-label">Queue Prompt (⌘↵)</span>
    </button>
  </div>

  <!-- Bottom Navigation Dock -->
  <div class="hud-dock">
    <button class="dock-btn" onclick="zoomDelta(0.15)" title="Zoom In">+</button>
    <button class="dock-btn" onclick="zoomDelta(-0.15)" title="Zoom Out">−</button>
    <button class="dock-btn" onclick="resetZoom()" title="Reset 100%">1:1</button>
    <button class="dock-btn" onclick="fitView()" title="Fit View">⛶</button>
  </div>

  <!-- Movable World Plane -->
  <div id="world">
    <!-- SVG Cables Layer -->
    <svg id="cables-svg"></svg>

    <!-- Interactive Draggable Nodes Container -->
    <div id="nodes-container"></div>
  </div>
</div>

<script>
  let state = ` + string(stateJSON) + `;

  let pan = { x: 40, y: 70 };
  let zoom = 1.0;
  let isPanning = false;
  let startPan = { x: 0, y: 0 };

  const container = document.getElementById('canvas-container');
  const world = document.getElementById('world');
  const cablesSvg = document.getElementById('cables-svg');
  const nodesContainer = document.getElementById('nodes-container');

  // Node definitions according to pipeline type
  let nodes = [];
  let cables = [];

  function initGraph() {
    if (state.pipelineType === 'video') {
      document.getElementById('pipeline-badge').innerText = 'VIDEO PIPELINE (VEO / WAN / KLING)';
      nodes = [
        {
          id: 'node-vmodel',
          title: '1. Video Model Loader',
          category: 'MODEL',
          color: 'var(--color-model)',
          x: 40, y: 40, width: 280,
          outputs: [{ id: 'out-vmodel', name: 'MODEL', color: 'var(--color-model)' }],
          content: ` + "`" + `
            <div class="field-label"><span>Video Checkpoint</span></div>
            <select class="field-select" id="input-model" onchange="onParamChange()">
              <option value="veo-2-ai">Veo 2 (Google DeepMind)</option>
              <option value="wan-2-14b">Wan 2.1 14B High Quality</option>
              <option value="kling-v1-5">Kling AI v1.5 Master</option>
              <option value="svd-xt">Stable Video Diffusion XT</option>
            </select>
          ` + "`" + `
        },
        {
          id: 'node-prompt',
          title: '2. Video Text Prompt',
          category: 'PROMPT',
          color: 'var(--color-clip)',
          x: 40, y: 190, width: 300,
          outputs: [{ id: 'out-prompt', name: 'CONDITIONING', color: 'var(--color-clip)' }],
          content: ` + "`" + `
            <div class="field-label"><span>Dynamic Action Prompt</span></div>
            <textarea class="field-textarea" id="input-pos-prompt" oninput="onParamChange()">${escapeHTML(state.positivePrompt)}</textarea>
            <div class="preset-chips">
              <span class="preset-chip" onclick="appendPrompt('cinematic slow motion 60fps')">SlowMo</span>
              <span class="preset-chip" onclick="appendPrompt('fpv drone flythrough cinematic lighting')">Drone FPV</span>
              <span class="preset-chip" onclick="appendPrompt('neon cyberpunk rain reflections 4k')">Cyberpunk</span>
              <span class="preset-chip" onclick="appendPrompt('hyperlapse cloud movement')">Hyperlapse</span>
            </div>
          ` + "`" + `
        },
        {
          id: 'node-dynamics',
          title: '3. Video Dynamics & Timing',
          category: 'PARAMS',
          color: 'var(--color-params)',
          x: 380, y: 40, width: 280,
          inputs: [{ id: 'in-dyn-prompt', name: 'PROMPT', color: 'var(--color-clip)' }],
          outputs: [{ id: 'out-params', name: 'VIDEO_PARAMS', color: 'var(--color-params)' }],
          content: ` + "`" + `
            <div class="field-label"><span>Duration (Seconds)</span><span class="field-slider-val" id="val-duration">5s</span></div>
            <input type="range" class="field-slider" min="2" max="10" step="1" value="5" oninput="updateSlider('duration', this.value, 's')">
            <div class="field-label" style="margin-top: 6px;"><span>Frame Rate (FPS)</span></div>
            <select class="field-select" id="input-fps" onchange="onParamChange()">
              <option value="24">24 FPS (Cinematic standard)</option>
              <option value="30" selected>30 FPS (Smooth playback)</option>
              <option value="60">60 FPS (Ultra fluid motion)</option>
            </select>
          ` + "`" + `
        },
        {
          id: 'node-generator',
          title: '4. Video Latent Sampler',
          category: 'SAMPLER',
          color: 'var(--color-latent)',
          x: 700, y: 60, width: 280,
          inputs: [
            { id: 'in-gen-model', name: 'MODEL', color: 'var(--color-model)' },
            { id: 'in-gen-params', name: 'VIDEO_PARAMS', color: 'var(--color-params)' }
          ],
          outputs: [{ id: 'out-video', name: 'VIDEO', color: 'var(--color-image)' }],
          content: ` + "`" + `
            <div class="field-label"><span>Generation Steps</span><span class="field-slider-val" id="val-steps">${state.steps}</span></div>
            <input type="range" class="field-slider" min="10" max="50" step="1" value="${state.steps}" oninput="updateSlider('steps', this.value)">
            <div class="field-label" style="margin-top: 6px;"><span>Aspect Ratio</span></div>
            <select class="field-select" id="input-aspect" onchange="onParamChange()">
              <option value="16:9" selected>16:9 Landscape</option>
              <option value="9:16">9:16 Vertical Story</option>
              <option value="1:1">1:1 Square</option>
            </select>
          ` + "`" + `
        },
        {
          id: 'node-preview',
          title: '5. Video Player Preview',
          category: 'PREVIEW',
          color: 'var(--color-image)',
          x: 1020, y: 40, width: 340,
          inputs: [{ id: 'in-preview-video', name: 'VIDEO', color: 'var(--color-image)' }],
          content: ` + "`" + `
            <div class="preview-box" id="preview-stage">
              ${state.resultUrl ? ` + "`" + `<video src="${state.resultUrl}" controls autoplay loop></video>` + "`" + ` : ` + "`" + `<div class="preview-empty">🎬 Generated Video Stream Preview</div>` + "`" + `}
            </div>
          ` + "`" + `
        }
      ];

      cables = [
        { from: 'node-vmodel:out-vmodel', to: 'node-generator:in-gen-model', color: 'var(--color-model)' },
        { from: 'node-prompt:out-prompt', to: 'node-dynamics:in-dyn-prompt', color: 'var(--color-clip)' },
        { from: 'node-dynamics:out-params', to: 'node-generator:in-gen-params', color: 'var(--color-params)' },
        { from: 'node-generator:out-video', to: 'node-preview:in-preview-video', color: 'var(--color-image)' }
      ];
    } else {
      // IMAGE PIPELINE
      document.getElementById('pipeline-badge').innerText = 'IMAGE PIPELINE (FLUX / SDXL)';
      nodes = [
        {
          id: 'node-ckpt',
          title: '1. Load Checkpoint',
          category: 'MODEL',
          color: 'var(--color-model)',
          x: 40, y: 40, width: 280,
          outputs: [
            { id: 'out-model', name: 'MODEL', color: 'var(--color-model)' },
            { id: 'out-clip', name: 'CLIP', color: 'var(--color-clip)' },
            { id: 'out-vae', name: 'VAE', color: 'var(--color-vae)' }
          ],
          content: ` + "`" + `
            <div class="field-label"><span>Diffusion Model</span></div>
            <select class="field-select" id="input-model" onchange="onParamChange()">
              <option value="flux-1-schnell" ${state.model.includes('flux') ? 'selected' : ''}>Flux.1 Schnell (Fast & Crisp)</option>
              <option value="sd_xl_base_1.0" ${state.model.includes('xl') ? 'selected' : ''}>SDXL Base 1.0 (High Precision)</option>
              <option value="sd3_medium">Stable Diffusion 3 Medium</option>
              <option value="dreamshaper_xl">DreamShaper XL Turbo</option>
            </select>
          ` + "`" + `
        },
        {
          id: 'node-prompt-pos',
          title: '2. CLIP Text (Positive)',
          category: 'PROMPT',
          color: 'var(--color-clip)',
          x: 40, y: 195, width: 300,
          inputs: [{ id: 'in-clip-pos', name: 'CLIP', color: 'var(--color-clip)' }],
          outputs: [{ id: 'out-cond-pos', name: 'CONDITIONING', color: 'var(--color-clip)' }],
          content: ` + "`" + `
            <textarea class="field-textarea" id="input-pos-prompt" oninput="onParamChange()">${escapeHTML(state.positivePrompt)}</textarea>
            <div class="preset-chips">
              <span class="preset-chip" onclick="appendPrompt('cinematic volumetric lighting')">Cinematic</span>
              <span class="preset-chip" onclick="appendPrompt('neon cyberpunk rain reflections')">Cyberpunk</span>
              <span class="preset-chip" onclick="appendPrompt('photorealistic 35mm lens')">Photoreal</span>
              <span class="preset-chip" onclick="appendPrompt('makoto shinkai aesthetic')">Anime</span>
            </div>
          ` + "`" + `
        },
        {
          id: 'node-prompt-neg',
          title: '3. CLIP Text (Negative)',
          category: 'PROMPT',
          color: 'var(--color-clip)',
          x: 40, y: 360, width: 300,
          inputs: [{ id: 'in-clip-neg', name: 'CLIP', color: 'var(--color-clip)' }],
          outputs: [{ id: 'out-cond-neg', name: 'CONDITIONING', color: 'var(--color-clip)' }],
          content: ` + "`" + `
            <textarea class="field-textarea" id="input-neg-prompt" oninput="onParamChange()">${escapeHTML(state.negativePrompt)}</textarea>
          ` + "`" + `
        },
        {
          id: 'node-latent',
          title: '4. Empty Latent Image',
          category: 'LATENT',
          color: 'var(--color-latent)',
          x: 380, y: 40, width: 260,
          outputs: [{ id: 'out-latent', name: 'LATENT', color: 'var(--color-latent)' }],
          content: ` + "`" + `
            <div class="field-label"><span>Resolution / Aspect</span></div>
            <select class="field-select" id="input-aspect" onchange="onParamChange()">
              <option value="16:9" selected>16:9 (1280x720)</option>
              <option value="1:1">1:1 Square (1024x1024)</option>
              <option value="9:16">9:16 Portrait (720x1280)</option>
              <option value="4:3">4:3 Standard (1024x768)</option>
            </select>
          ` + "`" + `
        },
        {
          id: 'node-ksampler',
          title: '5. KSampler Engine',
          category: 'SAMPLER',
          color: 'var(--color-latent)',
          x: 680, y: 60, width: 290,
          inputs: [
            { id: 'in-sam-model', name: 'MODEL', color: 'var(--color-model)' },
            { id: 'in-sam-pos', name: 'POSITIVE', color: 'var(--color-clip)' },
            { id: 'in-sam-neg', name: 'NEGATIVE', color: 'var(--color-clip)' },
            { id: 'in-sam-latent', name: 'LATENT', color: 'var(--color-latent)' }
          ],
          outputs: [{ id: 'out-sam-latent', name: 'LATENT', color: 'var(--color-latent)' }],
          content: ` + "`" + `
            <div class="field-label"><span>Steps</span><span class="field-slider-val" id="val-steps">${state.steps}</span></div>
            <input type="range" class="field-slider" min="10" max="50" step="1" value="${state.steps}" oninput="updateSlider('steps', this.value)">

            <div class="field-label" style="margin-top: 4px;"><span>CFG Scale</span><span class="field-slider-val" id="val-cfg">${state.cfgScale}</span></div>
            <input type="range" class="field-slider" min="1" max="20" step="0.5" value="${state.cfgScale}" oninput="updateSlider('cfg', this.value)">

            <div class="field-label" style="margin-top: 4px;"><span>Sampler</span></div>
            <select class="field-select" id="input-sampler" onchange="onParamChange()">
              <option value="euler" selected>Euler Ancestral</option>
              <option value="dpmpp_2m">DPM++ 2M Karras</option>
              <option value="heun">Heun</option>
              <option value="ddim">DDIM</option>
            </select>
          ` + "`" + `
        },
        {
          id: 'node-vae',
          title: '6. VAE Decode',
          category: 'DECODE',
          color: 'var(--color-vae)',
          x: 1010, y: 60, width: 220,
          inputs: [
            { id: 'in-vae-samples', name: 'SAMPLES', color: 'var(--color-latent)' },
            { id: 'in-vae-vae', name: 'VAE', color: 'var(--color-vae)' }
          ],
          outputs: [{ id: 'out-vae-img', name: 'IMAGE', color: 'var(--color-image)' }],
          content: ` + "`" + `<div class="field-label"><span>Transforms latent to pixel tensor</span></div>` + "`" + `
        },
        {
          id: 'node-preview',
          title: '7. Save & Preview Image',
          category: 'OUTPUT',
          color: 'var(--color-image)',
          x: 1270, y: 40, width: 340,
          inputs: [{ id: 'in-prev-img', name: 'IMAGE', color: 'var(--color-image)' }],
          content: ` + "`" + `
            <div class="preview-box" id="preview-stage">
              ${state.resultUrl ? ` + "`" + `<img src="${state.resultUrl}">` + "`" + ` : ` + "`" + `<div class="preview-empty">🖼️ High-Res Image Output Canvas</div>` + "`" + `}
            </div>
          ` + "`" + `
        }
      ];

      cables = [
        { from: 'node-ckpt:out-model', to: 'node-ksampler:in-sam-model', color: 'var(--color-model)' },
        { from: 'node-ckpt:out-clip', to: 'node-prompt-pos:in-clip-pos', color: 'var(--color-clip)' },
        { from: 'node-ckpt:out-clip', to: 'node-prompt-neg:in-clip-neg', color: 'var(--color-clip)' },
        { from: 'node-prompt-pos:out-cond-pos', to: 'node-ksampler:in-sam-pos', color: 'var(--color-clip)' },
        { from: 'node-prompt-neg:out-cond-neg', to: 'node-ksampler:in-sam-neg', color: 'var(--color-clip)' },
        { from: 'node-latent:out-latent', to: 'node-ksampler:in-sam-latent', color: 'var(--color-latent)' },
        { from: 'node-ksampler:out-sam-latent', to: 'node-vae:in-vae-samples', color: 'var(--color-latent)' },
        { from: 'node-ckpt:out-vae', to: 'node-vae:in-vae-vae', color: 'var(--color-vae)' },
        { from: 'node-vae:out-vae-img', to: 'node-preview:in-prev-img', color: 'var(--color-image)' }
      ];
    }

    renderNodes();
    renderCables();
    updateTransform();
  }

  function renderNodes() {
    nodesContainer.innerHTML = '';
    nodes.forEach(node => {
      const card = document.createElement('div');
      card.className = 'node-card';
      card.id = node.id;
      card.style.left = node.x + 'px';
      card.style.top = node.y + 'px';
      card.style.width = node.width + 'px';

      let inHTML = '';
      if (node.inputs) {
        node.inputs.forEach(inp => {
          inHTML += ` + "`" + `
            <div class="socket-slot input" id="${node.id}:${inp.id}">
              <div class="port-circle" style="background: ${inp.color}"></div>
              <span>${inp.name}</span>
            </div>
          ` + "`" + `;
        });
      }

      let outHTML = '';
      if (node.outputs) {
        node.outputs.forEach(out => {
          outHTML += ` + "`" + `
            <div class="socket-slot output" id="${node.id}:${out.id}">
              <span>${out.name}</span>
              <div class="port-circle" style="background: ${out.color}"></div>
            </div>
          ` + "`" + `;
        });
      }

      card.innerHTML = ` + "`" + `
        <div class="node-header" onmousedown="startDragNode(event, '${node.id}')">
          <div class="node-badge" style="background: ${node.color}"></div>
          <span class="node-title">${node.title}</span>
          <span class="node-type-label">${node.category}</span>
        </div>
        <div class="node-body">
          ${(inHTML || outHTML) ? '<div class="socket-row">' + inHTML + outHTML + '</div>' : ''}
          ${node.content || ''}
        </div>
      ` + "`" + `;

      nodesContainer.appendChild(card);
    });
  }

  function renderCables() {
    cablesSvg.innerHTML = '';
    cables.forEach(c => {
      const fromEl = document.getElementById(c.from);
      const toEl = document.getElementById(c.to);
      if (!fromEl || !toEl) return;

      const fromRect = fromEl.querySelector('.port-circle').getBoundingClientRect();
      const toRect = toEl.querySelector('.port-circle').getBoundingClientRect();

      // Convert client coordinates to world plane coordinates
      const x1 = (fromRect.left + fromRect.width / 2 - pan.x) / zoom;
      const y1 = (fromRect.top + fromRect.height / 2 - pan.y) / zoom;
      const x2 = (toRect.left + toRect.width / 2 - pan.x) / zoom;
      const y2 = (toRect.top + toRect.height / 2 - pan.y) / zoom;

      const dx = Math.abs(x2 - x1) * 0.55;
      const d = ` + "`" + `M ${x1} ${y1} C ${x1 + dx} ${y1}, ${x2 - dx} ${y2}, ${x2} ${y2}` + "`" + `;

      const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
      path.setAttribute('d', d);
      path.setAttribute('class', 'cable-path' + (state.isGenerating ? ' animating' : ''));
      path.setAttribute('stroke', c.color);
      cablesSvg.appendChild(path);
    });
  }

  function updateTransform() {
    world.style.transform = ` + "`" + `translate(${pan.x}px, ${pan.y}px) scale(${zoom})` + "`" + `;
    container.style.backgroundPosition = ` + "`" + `${pan.x}px ${pan.y}px` + "`" + `;
    container.style.backgroundSize = ` + "`" + `${24 * zoom}px ${24 * zoom}px` + "`" + `;
  }

  // Pan Canvas
  container.addEventListener('mousedown', (e) => {
    if (e.target.closest('.node-card') || e.target.closest('.hud-topbar') || e.target.closest('.hud-dock')) return;
    isPanning = true;
    startPan = { x: e.clientX - pan.x, y: e.clientY - pan.y };
  });

  window.addEventListener('mousemove', (e) => {
    if (isPanning) {
      pan.x = e.clientX - startPan.x;
      pan.y = e.clientY - startPan.y;
      updateTransform();
      renderCables();
    }
  });

  window.addEventListener('mouseup', () => { isPanning = false; });

  // Zoom Canvas
  container.addEventListener('wheel', (e) => {
    e.preventDefault();
    const factor = e.deltaY < 0 ? 1.08 : 0.92;
    const newZoom = Math.min(Math.max(0.25, zoom * factor), 2.5);

    // Zoom towards cursor
    const mouseX = e.clientX;
    const mouseY = e.clientY;
    pan.x = mouseX - (mouseX - pan.x) * (newZoom / zoom);
    pan.y = mouseY - (mouseY - pan.y) * (newZoom / zoom);
    zoom = newZoom;

    updateTransform();
    renderCables();
  }, { passive: false });

  function zoomDelta(delta) {
    zoom = Math.min(Math.max(0.25, zoom + delta), 2.5);
    updateTransform();
    renderCables();
  }

  function resetZoom() {
    zoom = 1.0;
    updateTransform();
    renderCables();
  }

  function resetCanvasLayout() {
    pan = { x: 40, y: 70 };
    zoom = 1.0;
    initGraph();
  }

  function fitView() {
    pan = { x: 20, y: 60 };
    zoom = 0.85;
    updateTransform();
    renderCables();
  }

  // Dragging Nodes
  let draggedNode = null;
  let dragOffset = { x: 0, y: 0 };

  function startDragNode(e, nodeId) {
    e.stopPropagation();
    const node = nodes.find(n => n.id === nodeId);
    if (!node) return;
    draggedNode = node;
    dragOffset = {
      x: e.clientX / zoom - node.x,
      y: e.clientY / zoom - node.y
    };

    document.querySelectorAll('.node-card').forEach(c => c.classList.remove('selected'));
    document.getElementById(nodeId).classList.add('selected');

    const onMove = (ev) => {
      if (!draggedNode) return;
      draggedNode.x = ev.clientX / zoom - dragOffset.x;
      draggedNode.y = ev.clientY / zoom - dragOffset.y;
      const el = document.getElementById(draggedNode.id);
      if (el) {
        el.style.left = draggedNode.x + 'px';
        el.style.top = draggedNode.y + 'px';
      }
      renderCables();
    };

    const onUp = () => {
      draggedNode = null;
      window.removeEventListener('mousemove', onMove);
      window.removeEventListener('mouseup', onUp);
    };

    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onUp);
  }

  // Parameter updates
  function updateSlider(type, val, unit = '') {
    if (type === 'steps') {
      state.steps = parseInt(val, 10);
      document.getElementById('val-steps').innerText = val;
    } else if (type === 'cfg') {
      state.cfgScale = parseFloat(val);
      document.getElementById('val-cfg').innerText = val;
    } else if (type === 'duration') {
      state.duration = parseFloat(val);
      document.getElementById('val-duration').innerText = val + unit;
    }
    onParamChange();
  }

  function appendPrompt(tag) {
    const el = document.getElementById('input-pos-prompt');
    if (!el) return;
    if (el.value.trim() === '') {
      el.value = tag;
    } else {
      el.value = el.value.trim() + ', ' + tag;
    }
    onParamChange();
  }

  function onParamChange() {
    const pos = document.getElementById('input-pos-prompt');
    if (pos) state.positivePrompt = pos.value;
    const neg = document.getElementById('input-neg-prompt');
    if (neg) state.negativePrompt = neg.value;
    const mod = document.getElementById('input-model');
    if (mod) state.model = mod.value;
    const asp = document.getElementById('input-aspect');
    if (asp) state.aspectRatio = asp.value;
    const sam = document.getElementById('input-sampler');
    if (sam) state.sampler = sam.value;

    syncToGo('param_change');
  }

  function queuePrompt() {
    onParamChange();
    state.isGenerating = true;
    updateUIState();
    syncToGo('queue_prompt');
  }

  function syncToGo(action) {
    const payload = encodeURIComponent(JSON.stringify({
      action: action,
      pipelineType: state.pipelineType,
      positivePrompt: state.positivePrompt,
      negativePrompt: state.negativePrompt,
      model: state.model,
      aspectRatio: state.aspectRatio,
      steps: state.steps,
      cfgScale: state.cfgScale,
      seed: state.seed
    }));
    document.title = 'forgeade:' + action + ':' + payload;
  }

  function updateUIState() {
    const btn = document.getElementById('btn-queue-prompt');
    const icon = document.getElementById('queue-icon');
    const label = document.getElementById('queue-label');
    const status = document.getElementById('status-display');

    if (state.isGenerating) {
      btn.style.opacity = '0.75';
      icon.innerText = '⏳';
      label.innerText = 'Generating...';
      status.innerText = state.statusText || 'Sampling in progress...';
    } else {
      btn.style.opacity = '1';
      icon.innerText = '⚡';
      label.innerText = 'Queue Prompt (⌘↵)';
      status.innerText = state.statusText || 'Ready • 5 Nodes Active';
    }
    renderCables();
  }

  // Window bridge methods called by Go
  window.setGenerationProgress = function(isGen, statusText, pct, resultUrl) {
    state.isGenerating = isGen;
    state.statusText = statusText;
    state.progress = pct;
    if (resultUrl) {
      state.resultUrl = resultUrl;
      const stage = document.getElementById('preview-stage');
      if (stage) {
        if (state.pipelineType === 'video') {
          stage.innerHTML = '<video src="' + resultUrl + '" controls autoplay loop></video>';
        } else {
          stage.innerHTML = '<img src="' + resultUrl + '">';
        }
      }
    }
    updateUIState();
  };

  window.setPipeline = function(pipeline) {
    state.pipelineType = pipeline;
    initGraph();
  };

  // Keyboard shortcut: Cmd+Enter
  window.addEventListener('keydown', (e) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
      e.preventDefault();
      queuePrompt();
    }
  });

  function escapeHTML(str) {
    if (!str) return '';
    return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  }

  // Initialize
  initGraph();
</script>
</body>
</html>`
	return html
}
