package studio

import (
	"encoding/json"
)

// StoryboardShot represents a single timed shot inside a Storyboard / Plan node.
type StoryboardShot struct {
	TimeRange string `json:"timeRange"`
	Title     string `json:"title"`
	Prompt    string `json:"prompt"`
	Camera    string `json:"camera,omitempty"`
}

// MediaAsset represents a reference, generated image, or generated video in the Media Board.
type MediaAsset struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // "reference", "image", "video"
	Title    string `json:"title"`
	URL      string `json:"url"`
	FilePath string `json:"filePath,omitempty"`
}

// NodeWorkflowTheme carries semantic CSS colors resolved from the active ForgeADE theme.
type NodeWorkflowTheme struct {
	IsDark              bool   `json:"isDark"`
	BgApp               string `json:"bgApp"`
	BgSidebar           string `json:"bgSidebar"`
	BgPanel             string `json:"bgPanel"`
	BgCard              string `json:"bgCard"`
	BgInput             string `json:"bgInput"`
	HudBg               string `json:"hudBg"`
	Border              string `json:"border"`
	BorderStrong        string `json:"borderStrong"`
	GridDot             string `json:"gridDot"`
	TextPrimary         string `json:"textPrimary"`
	TextSecondary       string `json:"textSecondary"`
	TextMuted           string `json:"textMuted"`
	Accent              string `json:"accent"`
	AccentFg            string `json:"accentFg"`
	AccentSoft          string `json:"accentSoft"`
	AccentGlow          string `json:"accentGlow"`
	ToolbarControl      string `json:"toolbarControl"`
	ToolbarControlHover string `json:"toolbarControlHover"`
	Success             string `json:"success"`
	Warning             string `json:"warning"`
	Error               string `json:"error"`
	Info                string `json:"info"`
	Secondary           string `json:"secondary"`
}

// NodeWorkflowInitialState holds state passed into the webview on initial render.
type NodeWorkflowInitialState struct {
	PipelineType    string                 `json:"pipelineType"` // "image", "video", or "flow"
	PositivePrompt  string                 `json:"positivePrompt"`
	NegativePrompt  string                 `json:"negativePrompt"`
	Model           string                 `json:"model"`
	Sampler         string                 `json:"sampler"`
	Steps           int                    `json:"steps"`
	CFGScale        float32                `json:"cfgScale"`
	Seed            int64                  `json:"seed"`
	AspectRatio     string                 `json:"aspectRatio"`
	Duration        float32                `json:"duration"`
	FPS             int                    `json:"fps"`
	MotionBucket    int                    `json:"motionBucket"`
	ResultURL       string                 `json:"resultUrl"`
	ResultFormat    string                 `json:"resultFormat"`
	ResultWidth     int                    `json:"resultWidth"`
	ResultHeight    int                    `json:"resultHeight"`
	IsGenerating    bool                   `json:"isGenerating"`
	Progress        float32                `json:"progress"`
	StatusText      string                 `json:"statusText"`
	ReferenceImages []string               `json:"referenceImages,omitempty"`
	StoryboardShots []StoryboardShot       `json:"storyboardShots,omitempty"`
	MediaLibrary    []MediaAsset           `json:"mediaLibrary,omitempty"`
	Theme           *NodeWorkflowTheme     `json:"theme,omitempty"`
	Values          map[string]interface{} `json:"values,omitempty"`
}

// ComfyUIInitialState is an alias for backward compatibility.
type ComfyUIInitialState = NodeWorkflowInitialState

// GenerateNodeWorkflowHTML returns an interactive, zero-dependency HTML5/Canvas/SVG
// hybrid Google Flow + ComfyUI visual workflow studio tailored for Forge ADE Studio.
func GenerateNodeWorkflowHTML(pipelineType string, state NodeWorkflowInitialState) string {
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
	if state.Duration <= 0 {
		state.Duration = 8
	}
	if state.FPS <= 0 {
		state.FPS = 30
	}
	if state.MotionBucket <= 0 {
		state.MotionBucket = 127
	}
	if len(state.StoryboardShots) == 0 {
		state.StoryboardShots = []StoryboardShot{
			{TimeRange: "0-2s", Title: "Opening", Prompt: "Wide establishing shot introducing subject and atmosphere", Camera: "Slow Dolly In"},
			{TimeRange: "2-5s", Title: "Action", Prompt: "Dynamic medium shot tracking key movement and lighting", Camera: "Tracking Pan"},
			{TimeRange: "5-8s", Title: "Climax", Prompt: "Dramatic close-up with crisp focal depth and motion blur", Camera: "Handheld Push"},
			{TimeRange: "8-10s", Title: "Closing", Prompt: "Hero framing resolving the scene with golden/neon rim light", Camera: "Crane Pullback"},
		}
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
<title>ForgeADE Flow &amp; Node Studio</title>
<style>
  :root {
    --bg-app: #0c0d11;
    --bg-sidebar: #11131a;
    --bg-panel: #151822;
    --bg-card: #191d2a;
    --bg-input: #0e1017;
    --hud-bg: rgba(20, 23, 33, 0.88);
    --border: rgba(255, 255, 255, 0.08);
    --border-strong: rgba(255, 255, 255, 0.14);
    --grid-dot: rgba(255, 255, 255, 0.075);

    --text-primary: #f3f5f9;
    --text-secondary: #a1a9bd;
    --text-muted: #6b7388;

    --accent: #6366f1;
    --accent-fg: #ffffff;
    --accent-soft: rgba(99, 102, 241, 0.16);
    --accent-glow: rgba(99, 102, 241, 0.35);

    --toolbar-control: rgba(255, 255, 255, 0.05);
    --toolbar-control-hover: rgba(255, 255, 255, 0.1);
    --shadow-card: 0 12px 32px rgba(0, 0, 0, 0.42);
    --shadow-hud: 0 6px 24px rgba(0, 0, 0, 0.35);

    --color-ref: #10b981;
    --color-prompt: #f59e0b;
    --color-plan: #a855f7;
    --color-image: #06b6d4;
    --color-video: #f43f5e;
    --color-model: #818cf8;
    --color-output: #38bdf8;
  }

  * { box-sizing: border-box; margin: 0; padding: 0; user-select: none; }
  html, body {
    width: 100%; height: 100%; overflow: hidden;
    background: var(--bg-app);
    color: var(--text-primary);
    font-family: -apple-system, BlinkMacSystemFont, "Inter", "Segoe UI", Roboto, sans-serif;
    font-size: 12px;
  }

  /* Tabler Icon Baseline */
  .ti {
    display: inline-block; vertical-align: middle; flex-shrink: 0;
  }
  .ti-spin {
    animation: tiSpin 0.9s linear infinite;
  }
  @keyframes tiSpin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }

  /* 3-Pane Google Flow Shell */
  .app-shell {
    display: flex; width: 100%; height: 100%; overflow: hidden;
  }

  /* Left Sidebar */
  .left-sidebar {
    width: 224px; flex-shrink: 0;
    background: var(--bg-sidebar);
    border-right: 1px solid var(--border);
    display: flex; flex-direction: column;
    transition: width 0.2s ease;
    overflow: hidden;
    z-index: 40;
  }
  .left-sidebar.collapsed { width: 48px; }
  .left-sidebar.collapsed .nav-label,
  .left-sidebar.collapsed .sidebar-section-title,
  .left-sidebar.collapsed .node-palette-list,
  .left-sidebar.collapsed .template-list,
  .left-sidebar.collapsed .upload-btn-text,
  .left-sidebar.collapsed .nav-badge { display: none; }

  .sidebar-scroll {
    flex: 1; overflow-y: auto; padding: 10px 8px;
    display: flex; flex-direction: column; gap: 4px;
  }
  .sidebar-scroll::-webkit-scrollbar { width: 5px; }
  .sidebar-scroll::-webkit-scrollbar-thumb { background: var(--border-strong); border-radius: 3px; }

  .nav-item {
    display: flex; align-items: center; gap: 9px;
    height: 32px; padding: 0 10px; border-radius: 8px;
    color: var(--text-secondary); cursor: pointer;
    font-size: 12px; font-weight: 500;
    transition: background 0.14s, color 0.14s;
    border: 1px solid transparent;
  }
  .nav-item:hover { background: var(--toolbar-control-hover); color: var(--text-primary); }
  .nav-item.active {
    background: var(--accent-soft);
    border-color: var(--accent-glow);
    color: var(--text-primary);
  }
  .nav-item.active .nav-icon { color: var(--accent); }
  .nav-icon {
    width: 16px; height: 16px; display: flex; align-items: center; justify-content: center;
    color: var(--text-muted); flex-shrink: 0;
  }
  .nav-label { flex: 1; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .nav-badge {
    font-size: 10px; font-weight: 600; color: var(--text-muted);
    background: var(--toolbar-control); padding: 1px 6px; border-radius: 10px;
  }

  .sidebar-divider { height: 1px; background: var(--border); margin: 8px 6px; }
  .sidebar-section-title {
    font-size: 10px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.6px;
    color: var(--text-muted); padding: 4px 10px; display: flex; align-items: center; justify-content: space-between;
  }

  .upload-ref-btn {
    margin: 4px 4px 6px; height: 32px; border-radius: 8px;
    background: var(--accent-soft); border: 1px dashed var(--accent);
    color: var(--accent); font-size: 11px; font-weight: 600;
    display: flex; align-items: center; justify-content: center; gap: 6px;
    cursor: pointer; transition: all 0.15s;
  }
  .upload-ref-btn:hover { background: var(--accent-glow); }

  .node-palette-list { display: flex; flex-direction: column; gap: 3px; padding: 0 2px; }
  .palette-item {
    display: flex; align-items: center; gap: 8px;
    padding: 6px 8px; border-radius: 7px;
    background: var(--toolbar-control); border: 1px solid var(--border);
    cursor: pointer; transition: all 0.14s;
  }
  .palette-item:hover {
    background: var(--toolbar-control-hover); border-color: var(--border-strong);
    transform: translateX(2px);
  }
  .palette-dot { width: 8px; height: 8px; border-radius: 4px; flex-shrink: 0; }
  .palette-text { flex: 1; min-width: 0; }
  .palette-name { font-size: 11px; font-weight: 600; color: var(--text-primary); }
  .palette-sub { font-size: 9.5px; color: var(--text-muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .palette-plus { color: var(--text-muted); display: flex; align-items: center; }

  .template-list { display: flex; flex-direction: column; gap: 4px; padding: 0 2px; }
  .template-btn {
    text-align: left; padding: 7px 9px; border-radius: 7px;
    background: var(--toolbar-control); border: 1px solid var(--border);
    color: var(--text-primary); font-size: 10.5px; font-weight: 600;
    cursor: pointer; transition: all 0.14s;
    display: flex; flex-direction: column; gap: 2px;
  }
  .template-btn:hover { background: var(--accent-soft); border-color: var(--accent); }
  .template-btn-head { display: flex; align-items: center; gap: 6px; }
  .template-btn small { display: block; font-weight: 400; font-size: 9.5px; color: var(--text-muted); }

  .sidebar-footer {
    padding: 8px; border-top: 1px solid var(--border);
  }

  /* Center Workspace */
  .center-stage {
    flex: 1; min-width: 0; height: 100%; position: relative;
    display: flex; flex-direction: column; overflow: hidden;
    background: var(--bg-app);
  }

  /* Floating Top HUD */
  .hud-topbar {
    position: absolute; top: 10px; left: 12px; right: 12px;
    height: 40px; border-radius: 10px;
    background: var(--hud-bg); backdrop-filter: blur(12px);
    border: 1px solid var(--border-strong);
    display: flex; align-items: center; padding: 0 10px; gap: 8px;
    z-index: 60; box-shadow: var(--shadow-hud);
  }
  .view-seg {
    display: flex; background: var(--toolbar-control);
    border-radius: 7px; padding: 2px; border: 1px solid var(--border);
  }
  .view-seg-btn {
    height: 24px; padding: 0 9px; border-radius: 5px;
    background: transparent; border: none; color: var(--text-secondary);
    font-size: 11px; font-weight: 600; cursor: pointer; transition: all 0.14s;
    display: flex; align-items: center; gap: 5px;
  }
  .view-seg-btn.active { background: var(--accent); color: var(--accent-fg); }
  .hud-pill {
    padding: 3px 8px; border-radius: 5px;
    background: var(--accent-soft); border: 1px solid var(--accent-glow);
    font-size: 9.5px; font-weight: 700; color: var(--accent);
    letter-spacing: 0.4px; white-space: nowrap;
  }
  .status-text {
    font-size: 10.5px; color: var(--text-muted);
    white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  }
  .hud-btn {
    height: 26px; padding: 0 10px; border-radius: 6px;
    font-size: 11px; font-weight: 600; cursor: pointer;
    display: flex; align-items: center; gap: 5px;
    border: 1px solid var(--border-strong); background: var(--toolbar-control);
    color: var(--text-secondary); transition: all 0.14s; white-space: nowrap;
  }
  .hud-btn:hover { background: var(--toolbar-control-hover); color: var(--text-primary); }
  .hud-btn-primary {
    background: var(--accent); color: var(--accent-fg); border-color: transparent;
    box-shadow: 0 2px 12px var(--accent-glow);
  }
  .hud-btn-primary:hover { filter: brightness(1.08); }

  /* Infinite Node Graph Canvas */
  #canvas-container {
    width: 100%; height: 100%; position: relative; overflow: hidden;
    cursor: grab;
    background-image: radial-gradient(var(--grid-dot) 1.25px, transparent 1.25px);
    background-size: 24px 24px;
    background-position: 0 0;
  }
  #canvas-container.panning { cursor: grabbing; }
  #canvas-container.linking { cursor: crosshair; }

  #world {
    position: absolute; top: 0; left: 0; width: 100%; height: 100%;
    transform-origin: 0 0;
  }

  #cables-svg {
    position: absolute; top: 0; left: 0; width: 8000px; height: 8000px;
    overflow: visible; z-index: 2; pointer-events: none;
  }
  .cable-path {
    fill: none; stroke-width: 3px; stroke-linecap: round;
    transition: stroke-width 0.15s, opacity 0.15s;
    pointer-events: none;
  }
  .cable-hit {
    fill: none; stroke: transparent; stroke-width: 14px;
    pointer-events: stroke; cursor: pointer;
  }
  .cable-group:hover .cable-path {
    stroke-width: 4.5px; filter: drop-shadow(0 0 6px currentColor);
  }
  .cable-path.animating {
    stroke-dasharray: 8 6;
    animation: cableFlow 0.85s linear infinite;
  }
  .cable-temp {
    fill: none; stroke-width: 3px; stroke-dasharray: 6 5; stroke-linecap: round;
    pointer-events: none;
  }
  @keyframes cableFlow {
    from { stroke-dashoffset: 28; }
    to { stroke-dashoffset: 0; }
  }

  /* Node Cards */
  .node-card {
    position: absolute; width: 290px;
    background: var(--bg-panel);
    border: 1px solid var(--border-strong);
    border-radius: 12px;
    box-shadow: var(--shadow-card);
    z-index: 10; cursor: default;
    transition: border-color 0.15s, box-shadow 0.15s;
  }
  .node-card.selected {
    border-color: var(--accent);
    box-shadow: 0 0 0 2px var(--accent-glow), var(--shadow-card);
    z-index: 15;
  }
  .node-card.executing {
    border-color: var(--accent);
    box-shadow: 0 0 18px var(--accent-glow);
  }

  .node-header {
    height: 36px; padding: 0 10px 0 12px;
    background: var(--bg-card);
    border-top-left-radius: 11px; border-top-right-radius: 11px;
    border-bottom: 1px solid var(--border);
    display: flex; align-items: center; gap: 7px; cursor: move;
  }
  .node-badge { width: 8px; height: 8px; border-radius: 4px; flex-shrink: 0; }
  .node-title {
    font-size: 11.5px; font-weight: 700; color: var(--text-primary);
    flex: 1; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  }
  .node-cat {
    font-size: 8.5px; font-weight: 700; text-transform: uppercase;
    letter-spacing: 0.5px; padding: 2px 5px; border-radius: 4px;
    background: var(--toolbar-control); color: var(--text-muted);
  }
  .node-hdr-btn {
    width: 22px; height: 22px; border-radius: 5px; border: none;
    background: transparent; color: var(--text-muted);
    cursor: pointer; display: flex; align-items: center; justify-content: center;
  }
  .node-hdr-btn:hover { background: var(--toolbar-control-hover); color: var(--text-primary); }
  .node-hdr-btn.danger:hover { background: rgba(244, 63, 94, 0.18); color: var(--color-video); }

  .node-body { padding: 10px 12px 12px; display: flex; flex-direction: column; gap: 8px; }

  /* Sockets */
  .ports-grid {
    display: flex; justify-content: space-between; gap: 8px;
    padding-bottom: 6px; border-bottom: 1px solid var(--border);
  }
  .ports-col { display: flex; flex-direction: column; gap: 6px; }
  .ports-col.out { align-items: flex-end; margin-left: auto; }
  .socket-slot {
    display: flex; align-items: center; gap: 6px;
    font-size: 9.5px; font-weight: 700; letter-spacing: 0.3px; color: var(--text-secondary);
  }
  .port-circle {
    width: 12px; height: 12px; border-radius: 50%;
    border: 2px solid var(--bg-panel);
    box-shadow: 0 0 0 1.5px var(--border-strong);
    cursor: crosshair; transition: transform 0.14s, box-shadow 0.14s;
    flex-shrink: 0;
  }
  .port-circle:hover, .port-circle.active-port {
    transform: scale(1.35);
    box-shadow: 0 0 0 2px var(--text-primary), 0 0 10px currentColor;
  }

  /* Controls */
  .field-label {
    font-size: 10px; font-weight: 600; color: var(--text-muted);
    display: flex; justify-content: space-between; align-items: center;
  }
  .field-input, .field-textarea, .field-select {
    width: 100%; background: var(--bg-input);
    border: 1px solid var(--border-strong); border-radius: 7px;
    color: var(--text-primary); font-size: 11px;
    padding: 6px 8px; outline: none; font-family: inherit;
    user-select: text;
  }
  .field-input:focus, .field-textarea:focus, .field-select:focus { border-color: var(--accent); }
  .field-textarea { resize: vertical; min-height: 54px; line-height: 1.4; }
  .field-row { display: flex; gap: 6px; align-items: center; }
  .field-slider { flex: 1; accent-color: var(--accent); cursor: pointer; }
  .field-val { font-size: 10.5px; font-weight: 700; color: var(--accent); min-width: 28px; text-align: right; }

  .preset-chips { display: flex; flex-wrap: wrap; gap: 4px; }
  .preset-chip {
    font-size: 9.5px; font-weight: 600; padding: 2px 7px; border-radius: 5px;
    background: var(--toolbar-control); color: var(--text-secondary);
    border: 1px solid var(--border); cursor: pointer; transition: all 0.12s;
  }
  .preset-chip:hover { background: var(--accent); color: var(--accent-fg); border-color: var(--accent); }

  /* Reference Dropzone */
  .ref-dropzone {
    width: 100%; min-height: 110px; max-height: 170px;
    border-radius: 8px; border: 1px dashed var(--color-ref);
    background: var(--bg-input);
    display: flex; flex-direction: column; align-items: center; justify-content: center;
    gap: 6px; cursor: pointer; overflow: hidden; position: relative;
    transition: all 0.15s;
  }
  .ref-dropzone:hover { background: var(--toolbar-control-hover); }
  .ref-dropzone img { width: 100%; height: 150px; object-fit: cover; display: block; }
  .ref-placeholder {
    font-size: 10.5px; color: var(--text-secondary); text-align: center; padding: 14px;
    display: flex; flex-direction: column; align-items: center; gap: 6px;
  }

  /* Storyboard / Plan Shots */
  .shots-list { display: flex; flex-direction: column; gap: 5px; max-height: 210px; overflow-y: auto; }
  .shot-card {
    background: var(--bg-input); border: 1px solid var(--border);
    border-radius: 7px; padding: 6px 8px; display: flex; flex-direction: column; gap: 4px;
  }
  .shot-top { display: flex; align-items: center; gap: 6px; }
  .shot-time {
    font-size: 9px; font-weight: 700; color: var(--color-plan);
    background: var(--accent-soft); padding: 1px 5px; border-radius: 4px;
  }
  .shot-title { font-size: 10.5px; font-weight: 700; color: var(--text-primary); flex: 1; }
  .shot-cam { font-size: 9px; color: var(--text-muted); }
  .shot-input {
    width: 100%; background: transparent; border: none; color: var(--text-secondary);
    font-size: 10px; outline: none; font-family: inherit; user-select: text;
  }
  .shot-input:focus { color: var(--text-primary); }

  /* Linked Status Pills */
  .linked-pills { display: flex; flex-wrap: wrap; gap: 4px; }
  .linked-pill {
    font-size: 9px; font-weight: 700; padding: 2px 6px; border-radius: 4px;
    background: var(--toolbar-control); border: 1px solid var(--border); color: var(--text-muted);
    display: inline-flex; align-items: center; gap: 3px;
  }
  .linked-pill.on {
    background: var(--accent-soft); border-color: var(--color-ref); color: var(--color-ref);
  }

  .node-run-btn {
    width: 100%; height: 28px; border-radius: 7px; border: none;
    background: var(--accent); color: var(--accent-fg); font-size: 11px; font-weight: 600;
    cursor: pointer; display: flex; align-items: center; justify-content: center; gap: 5px;
    transition: all 0.14s;
  }
  .node-run-btn:hover { filter: brightness(1.08); }
  .node-run-btn.video { background: var(--color-video); color: #ffffff; }
  .node-mini-btn {
    height: 24px; padding: 0 8px; border-radius: 6px;
    background: var(--toolbar-control); border: 1px solid var(--border);
    color: var(--text-secondary); font-size: 10px; font-weight: 600; cursor: pointer;
    display: inline-flex; align-items: center; justify-content: center; gap: 4px;
  }
  .node-mini-btn:hover { background: var(--toolbar-control-hover); color: var(--text-primary); }

  /* Preview Box */
  .preview-box {
    width: 100%; min-height: 150px; max-height: 240px;
    border-radius: 8px; overflow: hidden;
    background: var(--bg-input); border: 1px dashed var(--border-strong);
    display: flex; align-items: center; justify-content: center; position: relative;
  }
  .preview-box img, .preview-box video { width: 100%; height: 100%; object-fit: contain; display: block; }
  .preview-empty {
    font-size: 10.5px; color: var(--text-muted); text-align: center; padding: 18px;
    display: flex; flex-direction: column; align-items: center; gap: 6px;
  }

  /* Bottom Zoom Dock */
  .hud-dock {
    position: absolute; bottom: 14px; right: 14px;
    background: var(--hud-bg); backdrop-filter: blur(10px);
    border: 1px solid var(--border-strong); border-radius: 8px;
    padding: 3px; display: flex; gap: 3px; z-index: 60;
  }
  .dock-btn {
    width: 28px; height: 26px; border-radius: 5px; border: none;
    background: transparent; color: var(--text-secondary);
    font-size: 11px; font-weight: 700; cursor: pointer;
    display: flex; align-items: center; justify-content: center;
  }
  .dock-btn:hover { background: var(--toolbar-control-hover); color: var(--text-primary); }
  .hud-hint {
    position: absolute; bottom: 14px; left: 14px;
    background: var(--hud-bg); border: 1px solid var(--border);
    border-radius: 7px; padding: 5px 10px; font-size: 10px; color: var(--text-muted);
    z-index: 60; pointer-events: none;
  }

  /* Media Board / Gallery View */
  #gallery-view {
    display: none; width: 100%; height: 100%;
    padding: 62px 20px 20px; overflow-y: auto;
  }
  #gallery-view.active { display: block; }
  .gallery-header {
    display: flex; align-items: center; gap: 10px; margin-bottom: 16px;
  }
  .gallery-search {
    flex: 1; max-width: 420px; height: 34px; border-radius: 17px;
    background: var(--bg-panel); border: 1px solid var(--border-strong);
    color: var(--text-primary); padding: 0 14px; font-size: 12px; outline: none;
    user-select: text;
  }
  .gallery-grid {
    display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 14px;
  }
  .media-card {
    background: var(--bg-panel); border: 1px solid var(--border);
    border-radius: 12px; overflow: hidden; display: flex; flex-direction: column;
    transition: transform 0.15s, border-color 0.15s;
  }
  .media-card:hover { transform: translateY(-2px); border-color: var(--border-strong); }
  .media-thumb {
    width: 100%; height: 165px; background: var(--bg-input);
    display: flex; align-items: center; justify-content: center; overflow: hidden; position: relative;
  }
  .media-thumb img, .media-thumb video { width: 100%; height: 100%; object-fit: cover; display: block; }
  .media-type-tag {
    position: absolute; top: 8px; left: 8px;
    font-size: 9px; font-weight: 700; text-transform: uppercase;
    padding: 2px 6px; border-radius: 4px;
    background: var(--hud-bg); color: var(--text-primary); border: 1px solid var(--border);
  }
  .media-meta { padding: 10px 12px; display: flex; flex-direction: column; gap: 6px; }
  .media-title { font-size: 11.5px; font-weight: 600; color: var(--text-primary); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .media-actions { display: flex; gap: 6px; }

  /* Right Sidebar: Flow AI Assistant */
  .right-sidebar {
    width: 286px; flex-shrink: 0;
    background: var(--bg-sidebar);
    border-left: 1px solid var(--border);
    display: flex; flex-direction: column;
    transition: width 0.2s ease;
    z-index: 40;
  }
  .right-sidebar.collapsed { width: 0; border-left: none; overflow: hidden; }
  .flow-header {
    height: 44px; padding: 0 14px;
    border-bottom: 1px solid var(--border);
    display: flex; align-items: center; gap: 8px;
  }
  .flow-title { font-size: 12.5px; font-weight: 600; flex: 1; color: var(--text-primary); }
  .flow-body {
    flex: 1; overflow-y: auto; padding: 16px 14px;
    display: flex; flex-direction: column; gap: 12px;
  }
  .flow-hero { text-align: center; padding: 12px 6px 6px; }
  .flow-hero h2 { font-size: 15px; font-weight: 600; color: var(--text-primary); margin-bottom: 4px; }
  .flow-hero p { font-size: 11.5px; color: var(--text-muted); line-height: 1.4; }
  .flow-starter {
    display: flex; align-items: center; gap: 10px;
    padding: 10px 12px; border-radius: 12px;
    background: var(--bg-card); border: 1px solid var(--border);
    cursor: pointer; transition: all 0.15s; text-align: left;
  }
  .flow-starter:hover { border-color: var(--accent); background: var(--accent-soft); }
  .flow-starter-icon {
    width: 34px; height: 34px; border-radius: 9px;
    background: var(--toolbar-control); color: var(--accent);
    display: flex; align-items: center; justify-content: center; flex-shrink: 0;
  }
  .flow-starter-title { font-size: 11.5px; font-weight: 600; color: var(--text-primary); }
  .flow-starter-sub { font-size: 10px; color: var(--text-muted); margin-top: 2px; }

  .flow-log { display: flex; flex-direction: column; gap: 8px; }
  .flow-msg {
    padding: 8px 10px; border-radius: 9px; font-size: 11px; line-height: 1.4;
    background: var(--toolbar-control); border: 1px solid var(--border); color: var(--text-secondary);
  }
  .flow-msg.user { background: var(--accent-soft); border-color: var(--accent-glow); color: var(--text-primary); }

  .flow-composer {
    padding: 10px 12px 12px; border-top: 1px solid var(--border);
  }
  .flow-composer-box {
    background: var(--bg-card); border: 1px solid var(--border-strong);
    border-radius: 14px; padding: 8px 10px;
    display: flex; flex-direction: column; gap: 6px;
  }
  .flow-composer-input {
    width: 100%; background: transparent; border: none; color: var(--text-primary);
    font-size: 11.5px; outline: none; resize: none; min-height: 36px; font-family: inherit;
    user-select: text;
  }
  .flow-composer-actions { display: flex; align-items: center; justify-content: space-between; }
  .flow-round-btn {
    width: 26px; height: 26px; border-radius: 50%; border: 1px solid var(--border-strong);
    background: var(--toolbar-control); color: var(--text-secondary);
    cursor: pointer; display: flex; align-items: center; justify-content: center;
  }
  .flow-round-btn:hover { background: var(--toolbar-control-hover); color: var(--text-primary); }
  .flow-round-btn.send { background: var(--accent); border-color: transparent; color: var(--accent-fg); }
  .flow-round-btn.send:hover { filter: brightness(1.08); }
</style>
</head>
<body>

<input type="file" id="global-file-input" accept="image/*" multiple style="display:none">

<div class="app-shell">
  <!-- LEFT SIDEBAR: Media Library + Node Palette -->
  <aside class="left-sidebar" id="left-sidebar">
    <div class="sidebar-scroll">
      <div class="nav-item active" id="nav-all" onclick="setMediaFilter('all')">
        <span class="nav-icon" data-icon="layout-grid" data-size="15"></span><span class="nav-label">All media</span><span class="nav-badge" id="count-all">0</span>
      </div>
      <div class="nav-item" id="nav-ref" onclick="setMediaFilter('reference')">
        <span class="nav-icon" data-icon="paperclip" data-size="15"></span><span class="nav-label">References</span><span class="nav-badge" id="count-ref">0</span>
      </div>
      <div class="nav-item" id="nav-image" onclick="setMediaFilter('image')">
        <span class="nav-icon" data-icon="photo" data-size="15"></span><span class="nav-label">Images</span><span class="nav-badge" id="count-image">0</span>
      </div>
      <div class="nav-item" id="nav-video" onclick="setMediaFilter('video')">
        <span class="nav-icon" data-icon="movie" data-size="15"></span><span class="nav-label">Videos</span><span class="nav-badge" id="count-video">0</span>
      </div>
      <div class="nav-item" id="nav-plan" onclick="setMediaFilter('plan')">
        <span class="nav-icon" data-icon="checklist" data-size="15"></span><span class="nav-label">Storyboards</span><span class="nav-badge" id="count-plan">1</span>
      </div>

      <button class="upload-ref-btn" onclick="triggerReferenceUpload()">
        <span data-icon="upload" data-size="14"></span><span class="upload-btn-text">Upload Reference</span>
      </button>

      <div class="sidebar-divider"></div>
      <div class="sidebar-section-title"><span>Add Workflow Node</span></div>
      <div class="node-palette-list">
        <div class="palette-item" onclick="addNodeToCanvas('reference')">
          <div class="palette-dot" style="background:var(--color-ref)"></div>
          <div class="palette-text"><div class="palette-name">Reference Media</div><div class="palette-sub">Character / Scene / Style</div></div>
          <span class="palette-plus" data-icon="plus" data-size="13"></span>
        </div>
        <div class="palette-item" onclick="addNodeToCanvas('prompt')">
          <div class="palette-dot" style="background:var(--color-prompt)"></div>
          <div class="palette-text"><div class="palette-name">Prompt &amp; Concept</div><div class="palette-sub">Positive / Negative text</div></div>
          <span class="palette-plus" data-icon="plus" data-size="13"></span>
        </div>
        <div class="palette-item" onclick="addNodeToCanvas('plan')">
          <div class="palette-dot" style="background:var(--color-plan)"></div>
          <div class="palette-text"><div class="palette-name">Storyboard / Plan</div><div class="palette-sub">Timed shots &amp; camera</div></div>
          <span class="palette-plus" data-icon="plus" data-size="13"></span>
        </div>
        <div class="palette-item" onclick="addNodeToCanvas('image_gen')">
          <div class="palette-dot" style="background:var(--color-image)"></div>
          <div class="palette-text"><div class="palette-name">Image Generator</div><div class="palette-sub">Flux / SDXL / KSampler</div></div>
          <span class="palette-plus" data-icon="plus" data-size="13"></span>
        </div>
        <div class="palette-item" onclick="addNodeToCanvas('video_gen')">
          <div class="palette-dot" style="background:var(--color-video)"></div>
          <div class="palette-text"><div class="palette-name">Video Generator</div><div class="palette-sub">Veo 2 / Kling / Wan 2.1</div></div>
          <span class="palette-plus" data-icon="plus" data-size="13"></span>
        </div>
        <div class="palette-item" onclick="addNodeToCanvas('preview')">
          <div class="palette-dot" style="background:var(--color-output)"></div>
          <div class="palette-text"><div class="palette-name">Output Preview</div><div class="palette-sub">Review &amp; Export</div></div>
          <span class="palette-plus" data-icon="plus" data-size="13"></span>
        </div>
      </div>

      <div class="sidebar-divider"></div>
      <div class="sidebar-section-title"><span>Flow Templates</span></div>
      <div class="template-list">
        <button class="template-btn" onclick="loadTemplate('flow')">
          <span class="template-btn-head"><span data-icon="sparkles" data-size="13" style="color:var(--accent)"></span>Full Video Flow</span>
          <small>Ref + Prompt + Plan + Img &rarr; Video</small>
        </button>
        <button class="template-btn" onclick="loadTemplate('image')">
          <span class="template-btn-head"><span data-icon="photo" data-size="13" style="color:var(--color-image)"></span>Image Pipeline</span>
          <small>Ref + Prompt &rarr; KSampler &rarr; Image</small>
        </button>
        <button class="template-btn" onclick="loadTemplate('video')">
          <span class="template-btn-head"><span data-icon="movie" data-size="13" style="color:var(--color-video)"></span>Video Pipeline</span>
          <small>Prompt + Plan + Keyframe &rarr; Video</small>
        </button>
      </div>
    </div>

    <div class="sidebar-footer">
      <div class="nav-item" onclick="toggleLeftSidebar()">
        <span class="nav-icon" data-icon="panel-left" data-size="15"></span><span class="nav-label">Collapse</span>
      </div>
    </div>
  </aside>

  <!-- CENTER WORKSPACE: Node Graph + Media Board -->
  <main class="center-stage">
    <div class="hud-topbar">
      <div class="view-seg">
        <button class="view-seg-btn active" id="btn-view-graph" onclick="setCenterView('graph')">
          <span data-icon="git-branch" data-size="13"></span><span>Nodes</span>
        </button>
        <button class="view-seg-btn" id="btn-view-gallery" onclick="setCenterView('gallery')">
          <span data-icon="layout-grid" data-size="13"></span><span>Media Board</span>
        </button>
      </div>
      <div class="hud-pill" id="pipeline-badge">IMAGE PIPELINE (FLUX / SDXL)</div>
      <span class="status-text" id="status-display">Ready</span>
      <div style="flex:1;"></div>
      <button class="hud-btn" onclick="resetCanvasLayout()" title="Reset Layout">
        <span data-icon="refresh" data-size="13"></span><span>Reset</span>
      </button>
      <button class="hud-btn" onclick="toggleRightSidebar()" title="Toggle Flow Assistant">
        <span data-icon="sparkles" data-size="13"></span><span>Flow</span>
      </button>
      <button class="hud-btn hud-btn-primary" id="btn-queue-prompt" onclick="queuePrompt()">
        <span id="queue-icon" data-icon="bolt" data-size="13"></span>
        <span id="queue-label">Queue Generation</span>
      </button>
    </div>

    <!-- Interactive Node Graph Canvas -->
    <div id="canvas-container">
      <div id="world">
        <svg id="cables-svg"></svg>
        <div id="nodes-container"></div>
      </div>

      <div class="hud-hint">Drag output port to input port to link &bull; Click cable to disconnect &bull; Scroll to zoom</div>

      <div class="hud-dock">
        <button class="dock-btn" onclick="zoomDelta(0.12)" title="Zoom In"><span data-icon="plus" data-size="13"></span></button>
        <button class="dock-btn" onclick="zoomDelta(-0.12)" title="Zoom Out"><span data-icon="minus" data-size="13"></span></button>
        <button class="dock-btn" onclick="resetZoom()" title="Reset 100%">1:1</button>
        <button class="dock-btn" onclick="fitView()" title="Fit View"><span data-icon="maximize" data-size="13"></span></button>
      </div>
    </div>

    <!-- Media Board / Gallery View -->
    <div id="gallery-view">
      <div class="gallery-header">
        <input type="text" class="gallery-search" id="gallery-search" placeholder="Search references, images, storyboards, videos..." oninput="renderGallery()">
        <button class="hud-btn" onclick="openOutputsFolder()"><span data-icon="folder-open" data-size="13"></span><span>Open Folder</span></button>
        <button class="hud-btn" onclick="triggerReferenceUpload()"><span data-icon="upload" data-size="13"></span><span>Upload Reference</span></button>
        <button class="hud-btn hud-btn-primary" onclick="setCenterView('graph')"><span data-icon="git-branch" data-size="13"></span><span>Back to Nodes</span></button>
      </div>
      <div class="gallery-grid" id="gallery-grid"></div>
    </div>
  </main>

  <!-- RIGHT SIDEBAR: Flow AI Assistant -->
  <aside class="right-sidebar" id="right-sidebar">
    <div class="flow-header">
      <span data-icon="sparkles" data-size="15" style="color:var(--accent)"></span>
      <span class="flow-title">Flow Assistant</span>
      <button class="node-hdr-btn" onclick="toggleRightSidebar()" title="Hide panel"><span data-icon="x" data-size="14"></span></button>
    </div>

    <div class="flow-body" id="flow-body">
      <div class="flow-hero">
        <h2>What would you like to create?</h2>
        <p>Link references, prompts, storyboard plans, and keyframes into a unified Image &amp; Video workflow.</p>
      </div>

      <button class="flow-starter" onclick="applyStarter('concept_plan')">
        <div class="flow-starter-icon"><span data-icon="movie" data-size="18"></span></div>
        <div>
          <div class="flow-starter-title">Turn a concept into prompts &amp; plan</div>
          <div class="flow-starter-sub">Auto-build 4-shot storyboard linked to Video</div>
        </div>
      </button>

      <button class="flow-starter" onclick="applyStarter('full_flow')">
        <div class="flow-starter-icon"><span data-icon="link" data-size="18"></span></div>
        <div>
          <div class="flow-starter-title">Reference + Image &rarr; Video Flow</div>
          <div class="flow-starter-sub">Wire Reference, Prompt, Plan &amp; Keyframe</div>
        </div>
      </button>

      <button class="flow-starter" onclick="applyStarter('keyframe_img')">
        <div class="flow-starter-icon"><span data-icon="photo" data-size="18"></span></div>
        <div>
          <div class="flow-starter-title">Generate Styled Keyframe Image</div>
          <div class="flow-starter-sub">Reference + CLIP Prompt &rarr; KSampler Engine</div>
        </div>
      </button>

      <div class="flow-log" id="flow-log"></div>
    </div>

    <div class="flow-composer">
      <div class="flow-composer-box">
        <textarea class="flow-composer-input" id="flow-chat-input" placeholder="Describe your scene or video idea..." onkeydown="onFlowInputKey(event)"></textarea>
        <div class="flow-composer-actions">
          <button class="flow-round-btn" onclick="triggerReferenceUpload()" title="Attach reference image"><span data-icon="plus" data-size="14"></span></button>
          <button class="flow-round-btn send" onclick="submitFlowIdea()" title="Apply to workflow"><span data-icon="arrow-right" data-size="14"></span></button>
        </div>
      </div>
    </div>
  </aside>
</div>

<script>
  // Tabler Icon SVG paths (matches app/native/pkg/icons/icons.go)
  const TABLER_PATHS = {
    'layout-grid': '<path d="M4 4m0 1a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v4a1 1 0 0 1 -1 1h-4a1 1 0 0 1 -1 -1z"/><path d="M14 4m0 1a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v4a1 1 0 0 1 -1 1h-4a1 1 0 0 1 -1 -1z"/><path d="M4 14m0 1a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v4a1 1 0 0 1 -1 1h-4a1 1 0 0 1 -1 -1z"/><path d="M14 14m0 1a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v4a1 1 0 0 1 -1 1h-4a1 1 0 0 1 -1 -1z"/>',
    'paperclip': '<path d="M15 7l-6.5 6.5a1.5 1.5 0 0 0 3 3l6.5 -6.5a3 3 0 0 0 -6 -6l-6.5 6.5a4.5 4.5 0 0 0 9 9l6.5 -6.5"/>',
    'photo': '<path d="M15 8h.01"/><path d="M3 6a3 3 0 0 1 3 -3h12a3 3 0 0 1 3 3v12a3 3 0 0 1 -3 3h-12a3 3 0 0 1 -3 -3v-12z"/><path d="M3 16l5 -5c.928 -.893 2.072 -.893 3 0l5 5"/><path d="M14 14l1 -1c.928 -.893 2.072 -.893 3 0l3 3"/>',
    'movie': '<path d="M4 4m0 2a2 2 0 0 1 2 -2h12a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-12a2 2 0 0 1 -2 -2z"/><path d="M8 4l0 16"/><path d="M16 4l0 16"/><path d="M4 8l4 0"/><path d="M4 16l4 0"/><path d="M4 12l16 0"/><path d="M16 8l4 0"/><path d="M16 16l4 0"/>',
    'checklist': '<path d="M9.615 20h-2.615a2 2 0 0 1 -2 -2v-12a2 2 0 0 1 2 -2h8a2 2 0 0 1 2 2v8"/><path d="M14 19l2 2l4 -4"/><path d="M9 8h4"/><path d="M9 12h2"/>',
    'upload': '<path d="M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2 -2v-2"/><path d="M7 9l5 -5l5 5"/><path d="M12 4l0 12"/>',
    'plus': '<path d="M12 5l0 14"/><path d="M5 12l14 0"/>',
    'minus': '<path d="M5 12l14 0"/>',
    'sparkles': '<path d="M16 18a2 2 0 0 1 2 2a2 2 0 0 1 2 -2a2 2 0 0 1 -2 -2a2 2 0 0 1 -2 2zm0 -12a2 2 0 0 1 2 2a2 2 0 0 1 2 -2a2 2 0 0 1 -2 -2a2 2 0 0 1 -2 2zm-7 1a6 6 0 0 1 6 6a6 6 0 0 1 -6 6a6 6 0 0 1 -6 -6a6 6 0 0 1 6 -6z"/>',
    'panel-left': '<path d="M4 4m0 2a2 2 0 0 1 2 -2h12a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-12a2 2 0 0 1 -2 -2z"/><path d="M9 4v16"/>',
    'git-branch': '<path d="M7 18m-2 0a2 2 0 1 0 4 0a2 2 0 1 0 -4 0"/><path d="M7 6m-2 0a2 2 0 1 0 4 0a2 2 0 1 0 -4 0"/><path d="M17 6m-2 0a2 2 0 1 0 4 0a2 2 0 1 0 -4 0"/><path d="M7 8l0 8"/><path d="M9 18h1a4 4 0 0 0 4 -4v-2a4 4 0 0 1 4 -4h1"/>',
    'refresh': '<path d="M20 11a8.1 8.1 0 0 0 -15.5 -2m-.5 -5v5h5"/><path d="M4 13a8.1 8.1 0 0 0 15.5 2m.5 5v-5h-5"/>',
    'bolt': '<path d="M13 3l0 7l6 0l-8 11l0 -7l-6 0z"/>',
    'loader': '<path d="M12 3a9 9 0 1 0 9 9"/>',
    'maximize': '<path d="M15 3h6v6"/><path d="M9 21h-6v-6"/><path d="M21 3l-7 7"/><path d="M3 21l7 -7"/>',
    'link': '<path d="M9 15l6 -6"/><path d="M11 6l.463 -.536a5 5 0 0 1 7.071 7.072l-.534 .464"/><path d="M13 18l-.397 .534a5.068 5.068 0 0 1 -7.127 0a4.972 4.972 0 0 1 0 -7.071l.524 -.463"/>',
    'x': '<path d="M18 6l-12 12"/><path d="M6 6l12 12"/>',
    'copy': '<path d="M8 8m0 2a2 2 0 0 1 2 -2h8a2 2 0 0 1 2 2v8a2 2 0 0 1 -2 2h-8a2 2 0 0 1 -2 -2z"/><path d="M16 8v-2a2 2 0 0 0 -2 -2h-8a2 2 0 0 0 -2 2v8a2 2 0 0 0 2 2h2"/>',
    'arrow-right': '<path d="M5 12l14 0"/><path d="M13 18l6 -6"/><path d="M13 6l6 6"/>',
    'player-play': '<path d="M7 4v16l13 -8z"/>',
    'check': '<path d="M5 12l5 5l10 -10"/>',
    'circle': '<path d="M12 12m-9 0a9 9 0 1 0 18 0a9 9 0 1 0 -18 0"/>',
    'external-link': '<path d="M12 6h-6a2 2 0 0 0 -2 2v10a2 2 0 0 0 2 2h10a2 2 0 0 0 2 -2v-6"/><path d="M11 13l9 -9"/><path d="M15 4h5v5"/>',
    'folder-open': '<path d="M5 19l2.757 -7.351a1 1 0 0 1 .936 -.649h12.307a1 1 0 0 1 .986 1.164l-.996 5.211a2 2 0 0 1 -1.964 1.625h-14.026a2 2 0 0 1 -2 -2v-11a2 2 0 0 1 2 -2h4l3 3h7a2 2 0 0 1 2 2v1.5"/>',
    'download': '<path d="M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2 -2v-2"/><path d="M7 11l5 5l5 -5"/><path d="M12 4l0 12"/>'
  };

  function ti(name, size, extraClass) {
    const sz = size || 14;
    const p = TABLER_PATHS[name] || '';
    const cls = extraClass ? 'ti ' + extraClass : 'ti';
    return '<svg class="' + cls + '" xmlns="http://www.w3.org/2000/svg" width="' + sz + '" height="' + sz + '" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">' + p + '</svg>';
  }

  function hydrateStaticIcons() {
    document.querySelectorAll('[data-icon]').forEach(function(el) {
      const name = el.getAttribute('data-icon');
      const sz = parseInt(el.getAttribute('data-size') || '14', 10);
      el.innerHTML = ti(name, sz);
    });
  }

  // Dynamic Theme Application (syncs with ForgeADE ThemeManager)
  window.applyTheme = function(t) {
    if (!t) return;
    const r = document.documentElement.style;
    if (t.bgApp) r.setProperty('--bg-app', t.bgApp);
    if (t.bgSidebar) r.setProperty('--bg-sidebar', t.bgSidebar);
    if (t.bgPanel) r.setProperty('--bg-panel', t.bgPanel);
    if (t.bgCard) r.setProperty('--bg-card', t.bgCard);
    if (t.bgInput) r.setProperty('--bg-input', t.bgInput);
    if (t.hudBg) r.setProperty('--hud-bg', t.hudBg);
    if (t.border) r.setProperty('--border', t.border);
    if (t.borderStrong) r.setProperty('--border-strong', t.borderStrong);
    if (t.gridDot) r.setProperty('--grid-dot', t.gridDot);
    if (t.textPrimary) r.setProperty('--text-primary', t.textPrimary);
    if (t.textSecondary) r.setProperty('--text-secondary', t.textSecondary);
    if (t.textMuted) r.setProperty('--text-muted', t.textMuted);
    if (t.accent) r.setProperty('--accent', t.accent);
    if (t.accentFg) r.setProperty('--accent-fg', t.accentFg);
    if (t.accentSoft) r.setProperty('--accent-soft', t.accentSoft);
    if (t.accentGlow) r.setProperty('--accent-glow', t.accentGlow);
    if (t.toolbarControl) r.setProperty('--toolbar-control', t.toolbarControl);
    if (t.toolbarControlHover) r.setProperty('--toolbar-control-hover', t.toolbarControlHover);
    if (t.success) r.setProperty('--color-ref', t.success);
    if (t.warning) r.setProperty('--color-prompt', t.warning);
    if (t.secondary) r.setProperty('--color-plan', t.secondary);
    if (t.info) {
      r.setProperty('--color-image', t.info);
      r.setProperty('--color-output', t.info);
    }
    if (t.error) r.setProperty('--color-video', t.error);
    if (t.isDark === false) {
      r.setProperty('--shadow-card', '0 8px 24px rgba(15, 23, 42, 0.10)');
      r.setProperty('--shadow-hud', '0 4px 16px rgba(15, 23, 42, 0.08)');
    } else {
      r.setProperty('--shadow-card', '0 12px 32px rgba(0, 0, 0, 0.42)');
      r.setProperty('--shadow-hud', '0 6px 24px rgba(0, 0, 0, 0.35)');
    }
  };

  let state = ` + string(stateJSON) + `;

  let pan = { x: 24, y: 66 };
  let zoom = 0.86;
  let isPanning = false;
  let startPan = { x: 0, y: 0 };
  let centerView = 'graph';
  let activeMediaFilter = 'all';
  let pendingUploadNodeId = null;
  let nodeSeq = 100;

  let linkingFrom = null;
  let tempMouse = { x: 0, y: 0 };
  let mediaItems = [];

  const container = document.getElementById('canvas-container');
  const world = document.getElementById('world');
  const cablesSvg = document.getElementById('cables-svg');
  const nodesContainer = document.getElementById('nodes-container');
  const fileInput = document.getElementById('global-file-input');

  let nodes = [];
  let cables = [];

  function escapeHTML(str) {
    if (!str) return '';
    return String(str).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  }

  function initMediaLibrary() {
    mediaItems = [];
    const seen = {};
    if (Array.isArray(state.mediaLibrary)) {
      state.mediaLibrary.forEach(function(item) {
        if (!item || !item.url) return;
        const key = item.filePath || item.url;
        if (seen[key]) return;
        seen[key] = true;
        mediaItems.push({
          id: item.id || ('media-' + mediaItems.length),
          type: item.type || 'image',
          title: item.title || 'Generated Asset',
          url: item.url,
          filePath: item.filePath || ''
        });
      });
    }
    if (Array.isArray(state.referenceImages)) {
      state.referenceImages.forEach(function(url, idx) {
        if (seen[url]) return;
        seen[url] = true;
        mediaItems.push({ id: 'ref-' + idx, type: 'reference', title: 'Reference #' + (idx + 1), url: url, filePath: '' });
      });
    }
    if (state.resultUrl && !seen[state.resultUrl]) {
      seen[state.resultUrl] = true;
      mediaItems.unshift({
        id: 'gen-initial',
        type: state.pipelineType === 'video' ? 'video' : 'image',
        title: 'Latest Output',
        url: state.resultUrl,
        filePath: ''
      });
    }
    updateMediaCounts();
  }

  function updateMediaCounts() {
    const refC = mediaItems.filter(function(m) { return m.type === 'reference'; }).length;
    const imgC = mediaItems.filter(function(m) { return m.type === 'image'; }).length;
    const vidC = mediaItems.filter(function(m) { return m.type === 'video'; }).length;
    const planC = nodes.filter(function(n) { return n.kind === 'plan'; }).length;
    document.getElementById('count-all').innerText = mediaItems.length + planC;
    document.getElementById('count-ref').innerText = refC;
    document.getElementById('count-image').innerText = imgC;
    document.getElementById('count-video').innerText = vidC;
    document.getElementById('count-plan').innerText = planC;
  }

  function loadTemplate(tpl) {
    state.pipelineType = tpl;
    initGraph();
    addFlowLog('Loaded ' + tpl.toUpperCase() + ' workflow template.', false);
  }

  function initGraph() {
    const shots = Array.isArray(state.storyboardShots) && state.storyboardShots.length > 0
      ? JSON.parse(JSON.stringify(state.storyboardShots))
      : [
          { timeRange: '0-2s', title: 'Opening', prompt: 'Wide establishing shot setting mood and lighting', camera: 'Slow Dolly In' },
          { timeRange: '2-5s', title: 'Action', prompt: 'Dynamic medium shot following main subject', camera: 'Tracking Pan' },
          { timeRange: '5-8s', title: 'Climax', prompt: 'Close-up hero moment with cinematic depth', camera: 'Push In' },
          { timeRange: '8-10s', title: 'Closing', prompt: 'Final wide pullback resolving the story', camera: 'Crane Out' }
        ];

    const firstRef = (Array.isArray(state.referenceImages) && state.referenceImages[0]) ? state.referenceImages[0] : '';

    if (state.pipelineType === 'video' || state.pipelineType === 'flow') {
      document.getElementById('pipeline-badge').innerText =
        state.pipelineType === 'flow' ? 'FLOW PIPELINE (REF + PROMPT + PLAN + IMG -> VIDEO)' : 'VIDEO PIPELINE (VEO / WAN / KLING)';

      nodes = [
        {
          id: 'node-ref',
          kind: 'reference',
          title: '1. Reference Media',
          category: 'REF',
          color: 'var(--color-ref)',
          x: 20, y: 20, width: 255,
          inputs: [],
          outputs: [{ id: 'out-ref', name: 'REFERENCE', dataType: 'ref', color: 'var(--color-ref)' }],
          data: { role: 'Character / Scene', weight: 0.85, imageUrl: firstRef }
        },
        {
          id: 'node-prompt',
          kind: 'prompt',
          title: '2. Concept & Prompt',
          category: 'PROMPT',
          color: 'var(--color-prompt)',
          x: 20, y: 315, width: 280,
          inputs: [],
          outputs: [{ id: 'out-prompt', name: 'PROMPT', dataType: 'prompt', color: 'var(--color-prompt)' }],
          data: { positive: state.positivePrompt, negative: state.negativePrompt }
        },
        {
          id: 'node-plan',
          kind: 'plan',
          title: '3. Storyboard & Shot Plan',
          category: 'PLAN',
          color: 'var(--color-plan)',
          x: 345, y: 295, width: 310,
          inputs: [
            { id: 'in-plan-ref', name: 'REFERENCE', dataType: 'ref', color: 'var(--color-ref)' },
            { id: 'in-plan-prompt', name: 'PROMPT', dataType: 'prompt', color: 'var(--color-prompt)' }
          ],
          outputs: [{ id: 'out-plan', name: 'PLAN', dataType: 'plan', color: 'var(--color-plan)' }],
          data: { shots: shots }
        },
        {
          id: 'node-image-gen',
          kind: 'image_gen',
          title: '4. Keyframe Image (KSampler Engine)',
          category: 'IMAGE',
          color: 'var(--color-image)',
          x: 345, y: 15, width: 290,
          inputs: [
            { id: 'in-img-ref', name: 'REFERENCE', dataType: 'ref', color: 'var(--color-ref)' },
            { id: 'in-img-prompt', name: 'PROMPT', dataType: 'prompt', color: 'var(--color-prompt)' }
          ],
          outputs: [{ id: 'out-image', name: 'IMAGE', dataType: 'image', color: 'var(--color-image)' }],
          data: { model: 'flux-1-schnell', aspect: state.aspectRatio, steps: state.steps, cfg: state.cfgScale, sampler: state.sampler || 'euler', previewUrl: state.pipelineType === 'image' ? state.resultUrl : '' }
        },
        {
          id: 'node-video-gen',
          kind: 'video_gen',
          title: '5. Video Latent Sampler',
          category: 'VIDEO',
          color: 'var(--color-video)',
          x: 705, y: 80, width: 300,
          inputs: [
            { id: 'in-vid-ref', name: 'REFERENCE', dataType: 'ref', color: 'var(--color-ref)' },
            { id: 'in-vid-prompt', name: 'PROMPT', dataType: 'prompt', color: 'var(--color-prompt)' },
            { id: 'in-vid-image', name: 'IMAGE', dataType: 'image', color: 'var(--color-image)' },
            { id: 'in-vid-plan', name: 'PLAN', dataType: 'plan', color: 'var(--color-plan)' }
          ],
          outputs: [{ id: 'out-video', name: 'VIDEO', dataType: 'video', color: 'var(--color-video)' }],
          data: { model: state.model || 'veo-2-ai', duration: state.duration || 8, fps: state.fps || 30, motion: state.motionBucket || 127, aspect: state.aspectRatio }
        },
        {
          id: 'node-preview',
          kind: 'preview',
          title: '6. Video Output Preview',
          category: 'OUTPUT',
          color: 'var(--color-output)',
          x: 1055, y: 80, width: 310,
          inputs: [
            { id: 'in-prev-video', name: 'VIDEO', dataType: 'video', color: 'var(--color-video)' },
            { id: 'in-prev-image', name: 'IMAGE', dataType: 'image', color: 'var(--color-image)' }
          ],
          outputs: [],
          data: { url: state.resultUrl || '', isVideo: state.pipelineType === 'video' }
        }
      ];

      cables = [
        { id: 'c1', fromNode: 'node-ref', fromPort: 'out-ref', toNode: 'node-image-gen', toPort: 'in-img-ref', color: 'var(--color-ref)' },
        { id: 'c2', fromNode: 'node-prompt', fromPort: 'out-prompt', toNode: 'node-image-gen', toPort: 'in-img-prompt', color: 'var(--color-prompt)' },
        { id: 'c3', fromNode: 'node-ref', fromPort: 'out-ref', toNode: 'node-plan', toPort: 'in-plan-ref', color: 'var(--color-ref)' },
        { id: 'c4', fromNode: 'node-prompt', fromPort: 'out-prompt', toNode: 'node-plan', toPort: 'in-plan-prompt', color: 'var(--color-prompt)' },
        { id: 'c5', fromNode: 'node-ref', fromPort: 'out-ref', toNode: 'node-video-gen', toPort: 'in-vid-ref', color: 'var(--color-ref)' },
        { id: 'c6', fromNode: 'node-prompt', fromPort: 'out-prompt', toNode: 'node-video-gen', toPort: 'in-vid-prompt', color: 'var(--color-prompt)' },
        { id: 'c7', fromNode: 'node-image-gen', fromPort: 'out-image', toNode: 'node-video-gen', toPort: 'in-vid-image', color: 'var(--color-image)' },
        { id: 'c8', fromNode: 'node-plan', fromPort: 'out-plan', toNode: 'node-video-gen', toPort: 'in-vid-plan', color: 'var(--color-plan)' },
        { id: 'c9', fromNode: 'node-video-gen', fromPort: 'out-video', toNode: 'node-preview', toPort: 'in-prev-video', color: 'var(--color-video)' }
      ];
    } else {
      document.getElementById('pipeline-badge').innerText = 'IMAGE PIPELINE (FLUX / SDXL)';
      nodes = [
        {
          id: 'node-ref',
          kind: 'reference',
          title: '1. Reference Image',
          category: 'REF',
          color: 'var(--color-ref)',
          x: 24, y: 24, width: 260,
          inputs: [],
          outputs: [{ id: 'out-ref', name: 'REFERENCE', dataType: 'ref', color: 'var(--color-ref)' }],
          data: { role: 'Style / Character', weight: 0.75, imageUrl: firstRef }
        },
        {
          id: 'node-prompt',
          kind: 'prompt',
          title: '2. CLIP Text Prompt',
          category: 'PROMPT',
          color: 'var(--color-prompt)',
          x: 24, y: 310, width: 290,
          inputs: [],
          outputs: [{ id: 'out-prompt', name: 'PROMPT', dataType: 'prompt', color: 'var(--color-prompt)' }],
          data: { positive: state.positivePrompt, negative: state.negativePrompt }
        },
        {
          id: 'node-image-gen',
          kind: 'image_gen',
          title: '3. KSampler Engine',
          category: 'SAMPLER',
          color: 'var(--color-image)',
          x: 375, y: 80, width: 300,
          inputs: [
            { id: 'in-img-ref', name: 'REFERENCE', dataType: 'ref', color: 'var(--color-ref)' },
            { id: 'in-img-prompt', name: 'PROMPT', dataType: 'prompt', color: 'var(--color-prompt)' },
            { id: 'in-img-plan', name: 'PLAN', dataType: 'plan', color: 'var(--color-plan)' }
          ],
          outputs: [{ id: 'out-image', name: 'IMAGE', dataType: 'image', color: 'var(--color-image)' }],
          data: { model: state.model || 'flux-1-schnell', aspect: state.aspectRatio, steps: state.steps, cfg: state.cfgScale, sampler: state.sampler || 'euler', previewUrl: state.resultUrl || '' }
        },
        {
          id: 'node-preview',
          kind: 'preview',
          title: '4. Save & Preview Image',
          category: 'OUTPUT',
          color: 'var(--color-output)',
          x: 735, y: 80, width: 320,
          inputs: [
            { id: 'in-prev-image', name: 'IMAGE', dataType: 'image', color: 'var(--color-image)' },
            { id: 'in-prev-video', name: 'VIDEO', dataType: 'video', color: 'var(--color-video)' }
          ],
          outputs: [],
          data: { url: state.resultUrl || '', isVideo: false }
        }
      ];

      cables = [
        { id: 'c1', fromNode: 'node-ref', fromPort: 'out-ref', toNode: 'node-image-gen', toPort: 'in-img-ref', color: 'var(--color-ref)' },
        { id: 'c2', fromNode: 'node-prompt', fromPort: 'out-prompt', toNode: 'node-image-gen', toPort: 'in-img-prompt', color: 'var(--color-prompt)' },
        { id: 'c3', fromNode: 'node-image-gen', fromPort: 'out-image', toNode: 'node-preview', toPort: 'in-prev-image', color: 'var(--color-image)' }
      ];
    }

    renderNodes();
    renderCables();
    updateTransform();
    updateUIState();
    updateMediaCounts();
  }

  function hasIncomingLink(nodeId, dataType) {
    return cables.some(function(c) {
      if (c.toNode !== nodeId) return false;
      const targetNode = nodes.find(function(n) { return n.id === nodeId; });
      if (!targetNode || !targetNode.inputs) return false;
      const port = targetNode.inputs.find(function(p) { return p.id === c.toPort; });
      return port && port.dataType === dataType;
    });
  }

  function buildNodeBodyHTML(node) {
    if (node.kind === 'reference') {
      const d = node.data || {};
      const imgHtml = d.imageUrl
        ? '<img src="' + escapeHTML(d.imageUrl) + '" alt="Reference">'
        : '<div class="ref-placeholder">' + ti('upload', 20) + '<span>Click to upload image</span><span style="opacity:0.7;font-size:9.5px">Character / Scene / Style Ref</span></div>';
      return (
        '<div class="ref-dropzone" onclick="uploadForNode(\'' + node.id + '\')">' + imgHtml + '</div>' +
        '<div class="field-row">' +
          '<select class="field-select" style="flex:1" onchange="updateNodeField(\'' + node.id + '\', \'role\', this.value)">' +
            '<option ' + (d.role === 'Character / Scene' ? 'selected' : '') + '>Character / Scene</option>' +
            '<option ' + (d.role === 'Character' ? 'selected' : '') + '>Character</option>' +
            '<option ' + (d.role === 'Scene' ? 'selected' : '') + '>Scene</option>' +
            '<option ' + (d.role === 'Style / Character' ? 'selected' : '') + '>Style / Character</option>' +
            '<option ' + (d.role === 'Style' ? 'selected' : '') + '>Style</option>' +
          '</select>' +
          '<button class="node-mini-btn" onclick="uploadForNode(\'' + node.id + '\')">' + ti('upload', 12) + '<span>Upload</span></button>' +
        '</div>' +
        '<div class="field-label"><span>Reference Weight</span><span class="field-val" id="val-w-' + node.id + '">' + (d.weight || 0.8) + '</span></div>' +
        '<input type="range" class="field-slider" min="0.1" max="1.0" step="0.05" value="' + (d.weight || 0.8) + '" oninput="updateRefWeight(\'' + node.id + '\', this.value)">'
      );
    }

    if (node.kind === 'prompt') {
      const d = node.data || {};
      return (
        '<div class="field-label"><span>Positive Prompt</span></div>' +
        '<textarea class="field-textarea" id="input-pos-' + node.id + '" oninput="updatePromptNode(\'' + node.id + '\', \'positive\', this.value)">' + escapeHTML(d.positive || '') + '</textarea>' +
        '<div class="preset-chips">' +
          '<span class="preset-chip" onclick="appendNodePrompt(\'' + node.id + '\', \'cinematic lighting 4k\')">Cinematic</span>' +
          '<span class="preset-chip" onclick="appendNodePrompt(\'' + node.id + '\', \'photorealistic 35mm lens\')">Photoreal</span>' +
          '<span class="preset-chip" onclick="appendNodePrompt(\'' + node.id + '\', \'slow motion drone shot\')">Drone</span>' +
          '<span class="preset-chip" onclick="appendNodePrompt(\'' + node.id + '\', \'neon cyberpunk rain\')">Cyberpunk</span>' +
        '</div>' +
        '<div class="field-label"><span>Negative Prompt</span></div>' +
        '<input type="text" class="field-input" value="' + escapeHTML(d.negative || '') + '" oninput="updatePromptNode(\'' + node.id + '\', \'negative\', this.value)">'
      );
    }

    if (node.kind === 'plan') {
      const shots = (node.data && node.data.shots) ? node.data.shots : [];
      let shotsHTML = '';
      shots.forEach(function(s, idx) {
        shotsHTML += (
          '<div class="shot-card">' +
            '<div class="shot-top">' +
              '<span class="shot-time">' + escapeHTML(s.timeRange) + '</span>' +
              '<span class="shot-title">' + escapeHTML(s.title) + '</span>' +
              '<span class="shot-cam">' + escapeHTML(s.camera || '') + '</span>' +
            '</div>' +
            '<input class="shot-input" value="' + escapeHTML(s.prompt) + '" oninput="updateShotPrompt(\'' + node.id + '\',' + idx + ',this.value)">' +
          '</div>'
        );
      });
      return (
        '<div class="shots-list">' + shotsHTML + '</div>' +
        '<div class="field-row">' +
          '<button class="node-mini-btn" style="flex:1" onclick="addShotToPlan(\'' + node.id + '\')">' + ti('plus', 12) + '<span>Add Shot</span></button>' +
          '<button class="node-mini-btn" style="flex:1;color:var(--accent);border-color:var(--accent-glow)" onclick="autoPlanShots(\'' + node.id + '\')">' + ti('sparkles', 12) + '<span>Auto-Plan</span></button>' +
        '</div>'
      );
    }

    if (node.kind === 'image_gen') {
      const d = node.data || {};
      const hasRef = hasIncomingLink(node.id, 'ref');
      const hasPrompt = hasIncomingLink(node.id, 'prompt');
      const hasPlan = hasIncomingLink(node.id, 'plan');
      return (
        '<div class="linked-pills">' +
          '<span class="linked-pill ' + (hasRef ? 'on' : '') + '">' + ti(hasRef ? 'check' : 'circle', 10) + '<span>Ref</span></span>' +
          '<span class="linked-pill ' + (hasPrompt ? 'on' : '') + '">' + ti(hasPrompt ? 'check' : 'circle', 10) + '<span>Prompt</span></span>' +
          '<span class="linked-pill ' + (hasPlan ? 'on' : '') + '">' + ti(hasPlan ? 'check' : 'circle', 10) + '<span>Plan</span></span>' +
        '</div>' +
        '<div class="field-label"><span>Image Model</span></div>' +
        '<select class="field-select" onchange="updateGenParam(\'' + node.id + '\', \'model\', this.value)">' +
          '<option value="flux-1-schnell" ' + ((d.model || '').indexOf('flux') >= 0 ? 'selected' : '') + '>Flux.1 Schnell (Fast &amp; Crisp)</option>' +
          '<option value="imagen-3" ' + (d.model === 'imagen-3' ? 'selected' : '') + '>Imagen 3 (Photoreal)</option>' +
          '<option value="sd_xl_base_1.0" ' + ((d.model || '').indexOf('xl') >= 0 ? 'selected' : '') + '>SDXL Base 1.0</option>' +
          '<option value="dreamshaper_xl" ' + (d.model === 'dreamshaper_xl' ? 'selected' : '') + '>DreamShaper XL Turbo</option>' +
        '</select>' +
        '<div class="field-row">' +
          '<select class="field-select" style="flex:1" onchange="updateGenParam(\'' + node.id + '\', \'aspect\', this.value)">' +
            '<option value="16:9" ' + (d.aspect === '16:9' ? 'selected' : '') + '>16:9 Landscape</option>' +
            '<option value="9:16" ' + (d.aspect === '9:16' ? 'selected' : '') + '>9:16 Vertical</option>' +
            '<option value="1:1" ' + (d.aspect === '1:1' ? 'selected' : '') + '>1:1 Square</option>' +
            '<option value="4:3" ' + (d.aspect === '4:3' ? 'selected' : '') + '>4:3 Classic</option>' +
          '</select>' +
          '<select class="field-select" style="flex:1" onchange="updateGenParam(\'' + node.id + '\', \'sampler\', this.value)">' +
            '<option value="euler" ' + (d.sampler === 'euler' ? 'selected' : '') + '>Euler a</option>' +
            '<option value="dpmpp_2m" ' + (d.sampler === 'dpmpp_2m' ? 'selected' : '') + '>DPM++ 2M</option>' +
            '<option value="ddim" ' + (d.sampler === 'ddim' ? 'selected' : '') + '>DDIM</option>' +
          '</select>' +
        '</div>' +
        '<div class="field-label"><span>Steps</span><span class="field-val" id="val-steps-' + node.id + '">' + (d.steps || 25) + '</span></div>' +
        '<input type="range" class="field-slider" min="10" max="50" step="1" value="' + (d.steps || 25) + '" oninput="updateGenSlider(\'' + node.id + '\', \'steps\', this.value)">' +
        '<div class="field-label"><span>CFG Scale</span><span class="field-val" id="val-cfg-' + node.id + '">' + (d.cfg || 7.5) + '</span></div>' +
        '<input type="range" class="field-slider" min="1" max="20" step="0.5" value="' + (d.cfg || 7.5) + '" oninput="updateGenSlider(\'' + node.id + '\', \'cfg\', this.value)">' +
        '<button class="node-run-btn" onclick="runSingleNode(\'' + node.id + '\', \'image\')">' + ti('player-play', 12) + '<span>Generate Image</span></button>'
      );
    }

    if (node.kind === 'video_gen') {
      const d = node.data || {};
      const hasRef = hasIncomingLink(node.id, 'ref');
      const hasPrompt = hasIncomingLink(node.id, 'prompt');
      const hasImg = hasIncomingLink(node.id, 'image');
      const hasPlan = hasIncomingLink(node.id, 'plan');
      return (
        '<div class="linked-pills">' +
          '<span class="linked-pill ' + (hasRef ? 'on' : '') + '">' + ti(hasRef ? 'check' : 'circle', 10) + '<span>Ref</span></span>' +
          '<span class="linked-pill ' + (hasPrompt ? 'on' : '') + '">' + ti(hasPrompt ? 'check' : 'circle', 10) + '<span>Prompt</span></span>' +
          '<span class="linked-pill ' + (hasImg ? 'on' : '') + '">' + ti(hasImg ? 'check' : 'circle', 10) + '<span>Image</span></span>' +
          '<span class="linked-pill ' + (hasPlan ? 'on' : '') + '">' + ti(hasPlan ? 'check' : 'circle', 10) + '<span>Plan</span></span>' +
        '</div>' +
        '<div class="field-label"><span>Video Engine</span></div>' +
        '<select class="field-select" onchange="updateGenParam(\'' + node.id + '\', \'model\', this.value)">' +
          '<option value="veo-2-ai" ' + (d.model === 'veo-2-ai' ? 'selected' : '') + '>Veo 2 (Google DeepMind)</option>' +
          '<option value="kling-v1-5" ' + (d.model === 'kling-v1-5' ? 'selected' : '') + '>Kling AI v1.5 Master</option>' +
          '<option value="wan-2-14b" ' + (d.model === 'wan-2-14b' ? 'selected' : '') + '>Wan 2.1 14B</option>' +
          '<option value="runway-gen3" ' + (d.model === 'runway-gen3' ? 'selected' : '') + '>Runway Gen-3 Alpha</option>' +
        '</select>' +
        '<div class="field-row">' +
          '<select class="field-select" style="flex:1" onchange="updateGenParam(\'' + node.id + '\', \'aspect\', this.value)">' +
            '<option value="16:9" ' + (d.aspect === '16:9' ? 'selected' : '') + '>16:9 Cinema</option>' +
            '<option value="9:16" ' + (d.aspect === '9:16' ? 'selected' : '') + '>9:16 Reels</option>' +
            '<option value="1:1" ' + (d.aspect === '1:1' ? 'selected' : '') + '>1:1 Square</option>' +
          '</select>' +
          '<select class="field-select" style="flex:1" onchange="updateGenParam(\'' + node.id + '\', \'fps\', this.value)">' +
            '<option value="24" ' + (Number(d.fps) === 24 ? 'selected' : '') + '>24 FPS</option>' +
            '<option value="30" ' + (Number(d.fps) === 30 ? 'selected' : '') + '>30 FPS</option>' +
            '<option value="60" ' + (Number(d.fps) === 60 ? 'selected' : '') + '>60 FPS</option>' +
          '</select>' +
        '</div>' +
        '<div class="field-label"><span>Duration</span><span class="field-val" id="val-dur-' + node.id + '">' + (d.duration || 8) + 's</span></div>' +
        '<input type="range" class="field-slider" min="2" max="15" step="1" value="' + (d.duration || 8) + '" oninput="updateGenSlider(\'' + node.id + '\', \'duration\', this.value)">' +
        '<button class="node-run-btn video" onclick="runSingleNode(\'' + node.id + '\', \'video\')">' + ti('movie', 13) + '<span>Render Video Flow</span></button>'
      );
    }

    if (node.kind === 'preview') {
      const d = node.data || {};
      const url = d.url || state.resultUrl || '';
      let mediaHtml = '<div class="preview-empty">' + ti('photo', 24) + '<span>Output Canvas</span><span style="opacity:0.7;font-size:9.5px">Run workflow to render</span></div>';
      if (url) {
        if (url.toLowerCase().endsWith('.mp4') || url.toLowerCase().endsWith('.webm')) {
          mediaHtml = '<video src="' + escapeHTML(url) + '" controls autoplay loop></video>';
        } else {
          mediaHtml = '<img src="' + escapeHTML(url) + '" alt="Output">';
        }
      }
      return (
        '<div class="preview-box" id="preview-stage">' + mediaHtml + '</div>' +
        '<div class="field-row">' +
          '<button class="node-mini-btn" style="flex:1" onclick="syncToGo(\'export_editor\')">' + ti('external-link', 12) + '<span>Editor</span></button>' +
          '<button class="node-mini-btn" style="flex:1" onclick="syncToGo(\'open_finder\')">' + ti('folder-open', 12) + '<span>Finder</span></button>' +
        '</div>'
      );
    }

    return '';
  }

  function renderNodes() {
    nodesContainer.innerHTML = '';
    nodes.forEach(function(node) {
      const card = document.createElement('div');
      card.className = 'node-card' + (state.isGenerating ? ' executing' : '');
      card.id = node.id;
      card.style.left = node.x + 'px';
      card.style.top = node.y + 'px';
      card.style.width = node.width + 'px';

      let inHTML = '';
      if (node.inputs && node.inputs.length > 0) {
        inHTML = '<div class="ports-col in">';
        node.inputs.forEach(function(inp) {
          const isTargetActive = linkingFrom !== null;
          inHTML += (
            '<div class="socket-slot input" id="' + node.id + ':' + inp.id + '">' +
              '<div class="port-circle in ' + (isTargetActive ? 'active-port' : '') + '" style="background:' + inp.color + ';color:' + inp.color + '" ' +
                'onmouseup="finishLink(event, \'' + node.id + '\', \'' + inp.id + '\')" ' +
                'onclick="clickInputPort(event, \'' + node.id + '\', \'' + inp.id + '\')" title="Connect to ' + inp.name + '"></div>' +
              '<span>' + inp.name + '</span>' +
            '</div>'
          );
        });
        inHTML += '</div>';
      }

      let outHTML = '';
      if (node.outputs && node.outputs.length > 0) {
        outHTML = '<div class="ports-col out">';
        node.outputs.forEach(function(out) {
          const isSrcActive = linkingFrom && linkingFrom.nodeId === node.id && linkingFrom.portId === out.id;
          outHTML += (
            '<div class="socket-slot output" id="' + node.id + ':' + out.id + '">' +
              '<span>' + out.name + '</span>' +
              '<div class="port-circle out ' + (isSrcActive ? 'active-port' : '') + '" style="background:' + out.color + ';color:' + out.color + '" ' +
                'onmousedown="startLink(event, \'' + node.id + '\', \'' + out.id + '\', \'' + out.color + '\')" ' +
                'title="Drag to connect ' + out.name + '"></div>' +
            '</div>'
          );
        });
        outHTML += '</div>';
      }

      const portsBlock = (inHTML || outHTML) ? '<div class="ports-grid">' + inHTML + outHTML + '</div>' : '';

      card.innerHTML = (
        '<div class="node-header" onmousedown="startDragNode(event, \'' + node.id + '\')">' +
          '<div class="node-badge" style="background:' + node.color + '"></div>' +
          '<span class="node-title">' + escapeHTML(node.title) + '</span>' +
          '<span class="node-cat">' + escapeHTML(node.category) + '</span>' +
          '<button class="node-hdr-btn" onclick="duplicateNode(event, \'' + node.id + '\')" title="Duplicate">' + ti('copy', 12) + '</button>' +
          '<button class="node-hdr-btn danger" onclick="deleteNode(event, \'' + node.id + '\')" title="Delete">' + ti('x', 12) + '</button>' +
        '</div>' +
        '<div class="node-body">' +
          portsBlock +
          buildNodeBodyHTML(node) +
        '</div>'
      );

      nodesContainer.appendChild(card);
    });
  }

  function getPortCenterWorld(nodeId, portId) {
    const slot = document.getElementById(nodeId + ':' + portId);
    if (!slot) return null;
    const dot = slot.querySelector('.port-circle');
    if (!dot) return null;
    const rect = dot.getBoundingClientRect();
    const contRect = container.getBoundingClientRect();
    return {
      x: (rect.left - contRect.left + rect.width / 2 - pan.x) / zoom,
      y: (rect.top - contRect.top + rect.height / 2 - pan.y) / zoom
    };
  }

  function renderCables() {
    cablesSvg.innerHTML = '';
    cables.forEach(function(c) {
      const p1 = getPortCenterWorld(c.fromNode, c.fromPort);
      const p2 = getPortCenterWorld(c.toNode, c.toPort);
      if (!p1 || !p2) return;

      const dx = Math.max(48, Math.abs(p2.x - p1.x) * 0.55);
      const d = 'M ' + p1.x + ' ' + p1.y + ' C ' + (p1.x + dx) + ' ' + p1.y + ', ' + (p2.x - dx) + ' ' + p2.y + ', ' + p2.x + ' ' + p2.y;

      const g = document.createElementNS('http://www.w3.org/2000/svg', 'g');
      g.setAttribute('class', 'cable-group');

      const hit = document.createElementNS('http://www.w3.org/2000/svg', 'path');
      hit.setAttribute('d', d);
      hit.setAttribute('class', 'cable-hit');
      hit.addEventListener('click', function(ev) {
        ev.stopPropagation();
        disconnectCable(c.id);
      });

      const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
      path.setAttribute('d', d);
      path.setAttribute('class', 'cable-path' + (state.isGenerating ? ' animating' : ''));
      path.setAttribute('stroke', c.color || 'var(--accent)');

      g.appendChild(hit);
      g.appendChild(path);
      cablesSvg.appendChild(g);
    });

    if (linkingFrom) {
      const p1 = getPortCenterWorld(linkingFrom.nodeId, linkingFrom.portId);
      if (p1) {
        const dx = Math.max(40, Math.abs(tempMouse.x - p1.x) * 0.5);
        const d = 'M ' + p1.x + ' ' + p1.y + ' C ' + (p1.x + dx) + ' ' + p1.y + ', ' + (tempMouse.x - dx) + ' ' + tempMouse.y + ', ' + tempMouse.x + ' ' + tempMouse.y;
        const temp = document.createElementNS('http://www.w3.org/2000/svg', 'path');
        temp.setAttribute('d', d);
        temp.setAttribute('class', 'cable-temp');
        temp.setAttribute('stroke', linkingFrom.color || 'var(--accent)');
        cablesSvg.appendChild(temp);
      }
    }
  }

  function startLink(e, nodeId, portId, color) {
    e.stopPropagation();
    e.preventDefault();
    const contRect = container.getBoundingClientRect();
    linkingFrom = { nodeId: nodeId, portId: portId, color: color };
    tempMouse = {
      x: (e.clientX - contRect.left - pan.x) / zoom,
      y: (e.clientY - contRect.top - pan.y) / zoom
    };
    container.classList.add('linking');
    renderCables();
  }

  function finishLink(e, toNodeId, toPortId) {
    if (!linkingFrom) return;
    e.stopPropagation();
    createLink(linkingFrom.nodeId, linkingFrom.portId, toNodeId, toPortId, linkingFrom.color);
    linkingFrom = null;
    container.classList.remove('linking');
    renderNodes();
    renderCables();
    updateUIState();
  }

  function clickInputPort(e, toNodeId, toPortId) {
    if (linkingFrom) finishLink(e, toNodeId, toPortId);
  }

  function createLink(fromNode, fromPort, toNode, toPort, color) {
    if (fromNode === toNode) return;
    const exists = cables.some(function(c) {
      return c.fromNode === fromNode && c.fromPort === fromPort && c.toNode === toNode && c.toPort === toPort;
    });
    if (exists) return;
    cables.push({
      id: 'c-' + Date.now() + '-' + Math.floor(Math.random() * 1000),
      fromNode: fromNode, fromPort: fromPort, toNode: toNode, toPort: toPort, color: color
    });
    addFlowLog('Linked ' + fromNode + ' -> ' + toNode, false);
  }

  function disconnectCable(cableId) {
    cables = cables.filter(function(c) { return c.id !== cableId; });
    renderNodes();
    renderCables();
    updateUIState();
  }

  function addNodeToCanvas(kind) {
    nodeSeq++;
    const id = 'node-' + kind + '-' + nodeSeq;
    const contRect = container.getBoundingClientRect();
    const cx = Math.round(((contRect.width / 2) - pan.x) / zoom - 140 + (Math.random() * 40 - 20));
    const cy = Math.round(((contRect.height / 2) - pan.y) / zoom - 90 + (Math.random() * 40 - 20));

    let node = null;
    if (kind === 'reference') {
      node = {
        id: id, kind: 'reference', title: 'Reference Media', category: 'REF', color: 'var(--color-ref)',
        x: cx, y: cy, width: 255, inputs: [],
        outputs: [{ id: 'out-ref', name: 'REFERENCE', dataType: 'ref', color: 'var(--color-ref)' }],
        data: { role: 'Character / Scene', weight: 0.85, imageUrl: '' }
      };
    } else if (kind === 'prompt') {
      node = {
        id: id, kind: 'prompt', title: 'Prompt & Concept', category: 'PROMPT', color: 'var(--color-prompt)',
        x: cx, y: cy, width: 280, inputs: [],
        outputs: [{ id: 'out-prompt', name: 'PROMPT', dataType: 'prompt', color: 'var(--color-prompt)' }],
        data: { positive: state.positivePrompt, negative: state.negativePrompt }
      };
    } else if (kind === 'plan') {
      node = {
        id: id, kind: 'plan', title: 'Storyboard / Plan', category: 'PLAN', color: 'var(--color-plan)',
        x: cx, y: cy, width: 310,
        inputs: [
          { id: 'in-plan-ref', name: 'REFERENCE', dataType: 'ref', color: 'var(--color-ref)' },
          { id: 'in-plan-prompt', name: 'PROMPT', dataType: 'prompt', color: 'var(--color-prompt)' }
        ],
        outputs: [{ id: 'out-plan', name: 'PLAN', dataType: 'plan', color: 'var(--color-plan)' }],
        data: { shots: JSON.parse(JSON.stringify(state.storyboardShots || [])) }
      };
    } else if (kind === 'image_gen') {
      node = {
        id: id, kind: 'image_gen', title: 'KSampler Engine', category: 'IMAGE', color: 'var(--color-image)',
        x: cx, y: cy, width: 290,
        inputs: [
          { id: 'in-img-ref', name: 'REFERENCE', dataType: 'ref', color: 'var(--color-ref)' },
          { id: 'in-img-prompt', name: 'PROMPT', dataType: 'prompt', color: 'var(--color-prompt)' },
          { id: 'in-img-plan', name: 'PLAN', dataType: 'plan', color: 'var(--color-plan)' }
        ],
        outputs: [{ id: 'out-image', name: 'IMAGE', dataType: 'image', color: 'var(--color-image)' }],
        data: { model: 'flux-1-schnell', aspect: state.aspectRatio, steps: state.steps, cfg: state.cfgScale, sampler: 'euler' }
      };
    } else if (kind === 'video_gen') {
      node = {
        id: id, kind: 'video_gen', title: 'Video Latent Sampler', category: 'VIDEO', color: 'var(--color-video)',
        x: cx, y: cy, width: 300,
        inputs: [
          { id: 'in-vid-ref', name: 'REFERENCE', dataType: 'ref', color: 'var(--color-ref)' },
          { id: 'in-vid-prompt', name: 'PROMPT', dataType: 'prompt', color: 'var(--color-prompt)' },
          { id: 'in-vid-image', name: 'IMAGE', dataType: 'image', color: 'var(--color-image)' },
          { id: 'in-vid-plan', name: 'PLAN', dataType: 'plan', color: 'var(--color-plan)' }
        ],
        outputs: [{ id: 'out-video', name: 'VIDEO', dataType: 'video', color: 'var(--color-video)' }],
        data: { model: 'veo-2-ai', duration: 8, fps: 30, motion: 127, aspect: state.aspectRatio }
      };
    } else {
      node = {
        id: id, kind: 'preview', title: 'Output Preview', category: 'OUTPUT', color: 'var(--color-output)',
        x: cx, y: cy, width: 310,
        inputs: [
          { id: 'in-prev-image', name: 'IMAGE', dataType: 'image', color: 'var(--color-image)' },
          { id: 'in-prev-video', name: 'VIDEO', dataType: 'video', color: 'var(--color-video)' }
        ],
        outputs: [],
        data: { url: state.resultUrl || '' }
      };
    }

    nodes.push(node);
    setCenterView('graph');
    renderNodes();
    renderCables();
    updateUIState();
    updateMediaCounts();
  }

  function duplicateNode(e, nodeId) {
    e.stopPropagation();
    const src = nodes.find(function(n) { return n.id === nodeId; });
    if (!src) return;
    nodeSeq++;
    const clone = JSON.parse(JSON.stringify(src));
    clone.id = 'node-' + src.kind + '-' + nodeSeq;
    clone.x += 36;
    clone.y += 36;
    nodes.push(clone);
    renderNodes();
    renderCables();
    updateUIState();
    updateMediaCounts();
  }

  function deleteNode(e, nodeId) {
    e.stopPropagation();
    nodes = nodes.filter(function(n) { return n.id !== nodeId; });
    cables = cables.filter(function(c) { return c.fromNode !== nodeId && c.toNode !== nodeId; });
    renderNodes();
    renderCables();
    updateUIState();
    updateMediaCounts();
  }

  function triggerReferenceUpload() {
    pendingUploadNodeId = null;
    fileInput.click();
  }
  function uploadForNode(nodeId) {
    pendingUploadNodeId = nodeId;
    fileInput.click();
  }

  fileInput.addEventListener('change', function(e) {
    const files = Array.from(e.target.files || []);
    files.forEach(function(file) {
      const reader = new FileReader();
      reader.onload = function(ev) {
        const dataUrl = ev.target.result;
        mediaItems.unshift({
          id: 'ref-' + Date.now() + '-' + Math.floor(Math.random() * 999),
          type: 'reference',
          title: file.name,
          url: dataUrl
        });
        let targetNode = null;
        if (pendingUploadNodeId) {
          targetNode = nodes.find(function(n) { return n.id === pendingUploadNodeId; });
        } else {
          targetNode = nodes.find(function(n) { return n.kind === 'reference'; });
        }
        if (targetNode) {
          targetNode.data = targetNode.data || {};
          targetNode.data.imageUrl = dataUrl;
          targetNode.title = 'Ref: ' + file.name;
        }
        updateMediaCounts();
        renderNodes();
        renderCables();
        renderGallery();
        addFlowLog('Attached reference: ' + file.name, false);
      };
      reader.readAsDataURL(file);
    });
    fileInput.value = '';
  });

  function updateNodeField(nodeId, key, val) {
    const n = nodes.find(function(x) { return x.id === nodeId; });
    if (n) { n.data[key] = val; }
  }
  function updateRefWeight(nodeId, val) {
    updateNodeField(nodeId, 'weight', parseFloat(val));
    const el = document.getElementById('val-w-' + nodeId);
    if (el) el.innerText = val;
  }
  function updatePromptNode(nodeId, key, val) {
    const n = nodes.find(function(x) { return x.id === nodeId; });
    if (n) n.data[key] = val;
    if (key === 'positive') state.positivePrompt = val;
    if (key === 'negative') state.negativePrompt = val;
    syncToGo('param_change');
  }
  function appendNodePrompt(nodeId, tag) {
    const el = document.getElementById('input-pos-' + nodeId);
    if (!el) return;
    el.value = el.value.trim() ? (el.value.trim() + ', ' + tag) : tag;
    updatePromptNode(nodeId, 'positive', el.value);
  }
  function updateShotPrompt(nodeId, idx, val) {
    const n = nodes.find(function(x) { return x.id === nodeId; });
    if (n && n.data && n.data.shots && n.data.shots[idx]) {
      n.data.shots[idx].prompt = val;
    }
  }
  function addShotToPlan(nodeId) {
    const n = nodes.find(function(x) { return x.id === nodeId; });
    if (!n) return;
    n.data.shots = n.data.shots || [];
    const idx = n.data.shots.length;
    n.data.shots.push({
      timeRange: (idx * 2) + '-' + ((idx + 1) * 2) + 's',
      title: 'Shot ' + (idx + 1),
      prompt: 'Describe camera action and visual transition...',
      camera: 'Smooth Pan'
    });
    renderNodes();
    renderCables();
  }
  function autoPlanShots(nodeId) {
    const n = nodes.find(function(x) { return x.id === nodeId; });
    if (!n) return;
    const base = (state.positivePrompt || 'Cinematic visual story').slice(0, 80);
    n.data.shots = [
      { timeRange: '0-2s', title: 'Opening', prompt: 'Establishing wide shot: ' + base, camera: 'Slow Dolly In' },
      { timeRange: '2-5s', title: 'Build-up', prompt: 'Medium tracking shot highlighting subject details & motion', camera: 'Tracking Pan' },
      { timeRange: '5-8s', title: 'Peak Moment', prompt: 'Dramatic close-up with volumetric lighting and kinetic energy', camera: 'Push In' },
      { timeRange: '8-10s', title: 'Resolution', prompt: 'Hero wide frame holding final composition smoothly', camera: 'Crane Out' }
    ];
    renderNodes();
    renderCables();
    addFlowLog('Generated 4-shot storyboard plan from prompt.', false);
  }
  function updateGenParam(nodeId, key, val) {
    const n = nodes.find(function(x) { return x.id === nodeId; });
    if (n) n.data[key] = val;
    if (key === 'model') state.model = val;
    if (key === 'aspect') state.aspectRatio = val;
    if (key === 'sampler') state.sampler = val;
    if (key === 'fps') state.fps = parseInt(val, 10);
    syncToGo('param_change');
  }
  function updateGenSlider(nodeId, key, val) {
    const n = nodes.find(function(x) { return x.id === nodeId; });
    if (n) n.data[key] = parseFloat(val);
    if (key === 'steps') {
      state.steps = parseInt(val, 10);
      const el = document.getElementById('val-steps-' + nodeId);
      if (el) el.innerText = val;
    } else if (key === 'cfg') {
      state.cfgScale = parseFloat(val);
      const el = document.getElementById('val-cfg-' + nodeId);
      if (el) el.innerText = val;
    } else if (key === 'duration') {
      state.duration = parseFloat(val);
      const el = document.getElementById('val-dur-' + nodeId);
      if (el) el.innerText = val + 's';
    }
    syncToGo('param_change');
  }

  function updateTransform() {
    world.style.transform = 'translate(' + pan.x + 'px, ' + pan.y + 'px) scale(' + zoom + ')';
    container.style.backgroundPosition = pan.x + 'px ' + pan.y + 'px';
    container.style.backgroundSize = (24 * zoom) + 'px ' + (24 * zoom) + 'px';
  }

  container.addEventListener('mousedown', function(e) {
    if (linkingFrom && !e.target.closest('.port-circle')) {
      linkingFrom = null;
      container.classList.remove('linking');
      renderCables();
      return;
    }
    if (e.target.closest('.node-card') || e.target.closest('.hud-topbar') || e.target.closest('.hud-dock')) return;
    isPanning = true;
    container.classList.add('panning');
    startPan = { x: e.clientX - pan.x, y: e.clientY - pan.y };
  });

  window.addEventListener('mousemove', function(e) {
    if (isPanning) {
      pan.x = e.clientX - startPan.x;
      pan.y = e.clientY - startPan.y;
      updateTransform();
      renderCables();
    } else if (linkingFrom) {
      const contRect = container.getBoundingClientRect();
      tempMouse = {
        x: (e.clientX - contRect.left - pan.x) / zoom,
        y: (e.clientY - contRect.top - pan.y) / zoom
      };
      renderCables();
    }
  });

  window.addEventListener('mouseup', function() {
    isPanning = false;
    container.classList.remove('panning');
  });

  container.addEventListener('wheel', function(e) {
    e.preventDefault();
    const factor = e.deltaY < 0 ? 1.07 : 0.93;
    const newZoom = Math.min(Math.max(0.3, zoom * factor), 2.2);
    const contRect = container.getBoundingClientRect();
    const mx = e.clientX - contRect.left;
    const my = e.clientY - contRect.top;
    pan.x = mx - (mx - pan.x) * (newZoom / zoom);
    pan.y = my - (my - pan.y) * (newZoom / zoom);
    zoom = newZoom;
    updateTransform();
    renderCables();
  }, { passive: false });

  function zoomDelta(d) {
    zoom = Math.min(Math.max(0.3, zoom + d), 2.2);
    updateTransform();
    renderCables();
  }
  function resetZoom() { zoom = 1.0; updateTransform(); renderCables(); }
  function fitView() { pan = { x: 20, y: 62 }; zoom = 0.78; updateTransform(); renderCables(); }
  function resetCanvasLayout() { pan = { x: 24, y: 66 }; zoom = 0.86; initGraph(); }

  function startDragNode(e, nodeId) {
    if (e.target.closest('button')) return;
    e.stopPropagation();
    const node = nodes.find(function(n) { return n.id === nodeId; });
    if (!node) return;
    const dragOffset = { x: e.clientX / zoom - node.x, y: e.clientY / zoom - node.y };
    document.querySelectorAll('.node-card').forEach(function(c) { c.classList.remove('selected'); });
    const cardEl = document.getElementById(nodeId);
    if (cardEl) cardEl.classList.add('selected');

    function onMove(ev) {
      node.x = ev.clientX / zoom - dragOffset.x;
      node.y = ev.clientY / zoom - dragOffset.y;
      if (cardEl) {
        cardEl.style.left = node.x + 'px';
        cardEl.style.top = node.y + 'px';
      }
      renderCables();
    }
    function onUp() {
      window.removeEventListener('mousemove', onMove);
      window.removeEventListener('mouseup', onUp);
    }
    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onUp);
  }

  function setCenterView(v) {
    centerView = v;
    document.getElementById('btn-view-graph').classList.toggle('active', v === 'graph');
    document.getElementById('btn-view-gallery').classList.toggle('active', v === 'gallery');
    document.getElementById('canvas-container').style.display = v === 'graph' ? 'block' : 'none';
    document.getElementById('gallery-view').classList.toggle('active', v === 'gallery');
    if (v === 'gallery') renderGallery();
    else renderCables();
  }

  function setMediaFilter(filter) {
    activeMediaFilter = filter;
    ['all', 'ref', 'image', 'video', 'plan'].forEach(function(k) {
      const el = document.getElementById('nav-' + k);
      if (el) el.classList.toggle('active', (k === 'ref' && filter === 'reference') || k === filter);
    });
    if (centerView !== 'gallery') {
      setCenterView('gallery');
    } else {
      renderGallery();
    }
  }

  function openOutputsFolder() {
    syncToGo('open_media_folder', { filePath: '' });
  }

  function openMediaFolder(itemId) {
    const item = mediaItems.find(function(m) { return m.id === itemId; });
    syncToGo('open_media_folder', { filePath: (item && item.filePath) ? item.filePath : '' });
  }

  function downloadMedia(itemId) {
    const item = mediaItems.find(function(m) { return m.id === itemId; });
    if (!item) return;
    if (item.filePath) {
      syncToGo('download_media', { filePath: item.filePath, fileName: item.title || 'studio-asset' });
      addFlowLog('Saving copy of ' + (item.title || 'asset') + '...', false);
      return;
    }
    if (item.url) {
      const a = document.createElement('a');
      a.href = item.url;
      a.download = item.title || 'reference.png';
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      addFlowLog('Downloaded ' + (item.title || 'reference'), false);
    }
  }

  function renderGallery() {
    const grid = document.getElementById('gallery-grid');
    if (!grid) return;
    const q = (document.getElementById('gallery-search').value || '').toLowerCase();
    let html = '';

    mediaItems.forEach(function(item) {
      if (activeMediaFilter !== 'all' && item.type !== activeMediaFilter) return;
      if (q && (item.title || '').toLowerCase().indexOf(q) < 0) return;
      const isVid = item.type === 'video' || (item.url && (item.url.toLowerCase().endsWith('.mp4') || item.url.toLowerCase().endsWith('.webm') || item.url.toLowerCase().endsWith('.mov')));
      const thumbEl = isVid
        ? '<video src="' + escapeHTML(item.url) + '" muted loop playsinline onmouseover="this.play()" onmouseout="this.pause()"></video>'
        : '<img src="' + escapeHTML(item.url) + '" alt="' + escapeHTML(item.title) + '">';
      html += (
        '<div class="media-card">' +
          '<div class="media-thumb">' +
            '<span class="media-type-tag">' + escapeHTML(item.type) + '</span>' +
            thumbEl +
          '</div>' +
          '<div class="media-meta">' +
            '<div class="media-title" title="' + escapeHTML(item.filePath || item.title) + '">' + escapeHTML(item.title) + '</div>' +
            '<div class="media-actions">' +
              '<button class="node-mini-btn" style="flex:1" onclick="useMediaAsRef(\'' + item.id + '\')" title="Use as Reference in Graph">' + ti('link', 11) + '<span>Use Ref</span></button>' +
              '<button class="node-mini-btn" onclick="downloadMedia(\'' + item.id + '\')" title="Download / Save Copy">' + ti('download', 12) + '</button>' +
              '<button class="node-mini-btn" onclick="openMediaFolder(\'' + item.id + '\')" title="Open File Folder">' + ti('folder-open', 12) + '</button>' +
            '</div>' +
          '</div>' +
        '</div>'
      );
    });

    if (activeMediaFilter === 'all' || activeMediaFilter === 'plan') {
      nodes.filter(function(n) { return n.kind === 'plan'; }).forEach(function(pn) {
        const shots = (pn.data && pn.data.shots) || [];
        let list = shots.map(function(s) { return '<div style="font-size:10px;color:var(--text-secondary);margin-bottom:3px"><b>' + escapeHTML(s.timeRange) + ' ' + escapeHTML(s.title) + ':</b> ' + escapeHTML(s.prompt) + '</div>'; }).join('');
        html += (
          '<div class="media-card">' +
            '<div class="media-thumb" style="padding:12px;align-items:flex-start;justify-content:flex-start;flex-direction:column;overflow-y:auto">' +
              '<span class="media-type-tag" style="position:static;margin-bottom:6px">STORYBOARD</span>' +
              list +
            '</div>' +
            '<div class="media-meta">' +
              '<div class="media-title">' + escapeHTML(pn.title) + '</div>' +
              '<div class="media-actions"><button class="node-mini-btn" style="flex:1" onclick="setCenterView(\'graph\')">Edit in Graph</button></div>' +
            '</div>' +
          '</div>'
        );
      });
    }

    if (!html) {
      html = '<div style="grid-column:1/-1;text-align:center;padding:60px 20px;color:var(--text-muted)">No media assets yet. Upload a reference image or run the workflow to populate your Media Board.</div>';
    }
    grid.innerHTML = html;
  }

  function useMediaAsRef(itemId) {
    const item = mediaItems.find(function(m) { return m.id === itemId; });
    if (!item) return;
    let refNode = nodes.find(function(n) { return n.kind === 'reference'; });
    if (!refNode) {
      addNodeToCanvas('reference');
      refNode = nodes[nodes.length - 1];
    }
    refNode.data.imageUrl = item.url;
    refNode.title = 'Ref: ' + item.title;
    setCenterView('graph');
    renderNodes();
    renderCables();
  }

  function toggleLeftSidebar() {
    document.getElementById('left-sidebar').classList.toggle('collapsed');
    setTimeout(renderCables, 220);
  }
  function toggleRightSidebar() {
    document.getElementById('right-sidebar').classList.toggle('collapsed');
    setTimeout(renderCables, 220);
  }

  function addFlowLog(text, isUser) {
    const log = document.getElementById('flow-log');
    if (!log) return;
    const div = document.createElement('div');
    div.className = 'flow-msg' + (isUser ? ' user' : '');
    div.innerText = text;
    log.appendChild(div);
    const body = document.getElementById('flow-body');
    if (body) body.scrollTop = body.scrollHeight;
  }

  function applyStarter(kind) {
    if (kind === 'concept_plan') {
      state.pipelineType = 'flow';
      initGraph();
      const planNode = nodes.find(function(n) { return n.kind === 'plan'; });
      if (planNode) autoPlanShots(planNode.id);
      addFlowLog('Built Concept -> Storyboard Plan -> Video workflow.', false);
    } else if (kind === 'full_flow') {
      loadTemplate('flow');
    } else if (kind === 'keyframe_img') {
      loadTemplate('image');
    }
  }

  function onFlowInputKey(e) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      submitFlowIdea();
    }
  }

  function submitFlowIdea() {
    const inp = document.getElementById('flow-chat-input');
    const txt = (inp.value || '').trim();
    if (!txt) return;
    inp.value = '';
    addFlowLog(txt, true);

    state.positivePrompt = txt;
    const promptNode = nodes.find(function(n) { return n.kind === 'prompt'; });
    if (promptNode) promptNode.data.positive = txt;
    const planNode = nodes.find(function(n) { return n.kind === 'plan'; });
    if (planNode) {
      autoPlanShots(planNode.id);
    } else {
      renderNodes();
      renderCables();
    }
    addFlowLog('Updated Prompt & Storyboard Plan nodes. Click Queue Generation to render.', false);
    syncToGo('param_change');
  }

  function composeWorkflowPrompt() {
    const parts = [];
    const pNode = nodes.find(function(n) { return n.kind === 'prompt'; });
    if (pNode && pNode.data && pNode.data.positive) parts.push(pNode.data.positive);
    else if (state.positivePrompt) parts.push(state.positivePrompt);

    const planNode = nodes.find(function(n) { return n.kind === 'plan'; });
    if (planNode && planNode.data && Array.isArray(planNode.data.shots) && planNode.data.shots.length > 0) {
      const summary = planNode.data.shots.map(function(s) { return s.timeRange + ' ' + s.title + ': ' + s.prompt; }).join(' | ');
      parts.push('[Storyboard: ' + summary + ']');
    }
    return parts.join(' ');
  }

  function runSingleNode(nodeId, targetKind) {
    state.isGenerating = true;
    state.statusText = 'Executing ' + (targetKind === 'video' ? 'Video Latent Sampler' : 'KSampler Engine') + '...';
    updateUIState();
    addFlowLog('Running ' + targetKind.toUpperCase() + ' stage...', false);
    syncToGo('queue_prompt');
  }

  function queuePrompt() {
    state.positivePrompt = composeWorkflowPrompt();
    state.isGenerating = true;
    state.statusText = 'Executing linked workflow (' + nodes.length + ' nodes, ' + cables.length + ' links)...';
    updateUIState();
    syncToGo('queue_prompt');
  }

  function syncToGo(action, extra) {
    const obj = {
      action: action,
      pipelineType: state.pipelineType,
      positivePrompt: state.positivePrompt,
      negativePrompt: state.negativePrompt,
      model: state.model,
      aspectRatio: state.aspectRatio,
      steps: state.steps,
      cfgScale: state.cfgScale,
      seed: state.seed
    };
    if (extra && typeof extra === 'object') {
      Object.keys(extra).forEach(function(k) { obj[k] = extra[k]; });
    }
    const payload = encodeURIComponent(JSON.stringify(obj));
    document.title = 'forgeade:' + action + ':' + payload;
  }

  function updateUIState() {
    const btn = document.getElementById('btn-queue-prompt');
    const icon = document.getElementById('queue-icon');
    const label = document.getElementById('queue-label');
    const status = document.getElementById('status-display');

    if (state.isGenerating) {
      btn.style.opacity = '0.78';
      icon.innerHTML = ti('loader', 13, 'ti-spin');
      label.innerText = 'Generating...';
      status.innerText = state.statusText || 'Sampling linked workflow...';
    } else {
      btn.style.opacity = '1';
      icon.innerHTML = ti('bolt', 13);
      label.innerText = 'Queue Generation';
      status.innerText = state.statusText || ('Ready • ' + nodes.length + ' Nodes • ' + cables.length + ' Links');
    }
    renderCables();
  }

  window.setGenerationProgress = function(isGen, statusText, pct, resultUrl, filePath) {
    state.isGenerating = isGen;
    state.statusText = statusText;
    state.progress = pct;
    if (resultUrl) {
      state.resultUrl = resultUrl;
      const fp = filePath || '';
      const baseName = fp ? fp.split('/').pop() : ('Render ' + new Date().toLocaleTimeString());
      const exists = mediaItems.some(function(m) { return (fp && m.filePath === fp) || m.url === resultUrl; });
      if (!exists) {
        const lower = (fp || resultUrl).toLowerCase();
        const itemType = (lower.endsWith('.mp4') || lower.endsWith('.webm') || lower.endsWith('.mov')) ? 'video' : 'image';
        mediaItems.unshift({
          id: 'out-' + Date.now(),
          type: itemType,
          title: baseName,
          url: resultUrl,
          filePath: fp
        });
      }
      nodes.forEach(function(n) {
        if (n.kind === 'preview') n.data.url = resultUrl;
      });
      updateMediaCounts();
      renderNodes();
      if (centerView === 'gallery') renderGallery();
      addFlowLog('Generation complete! Added output to Media Board.', false);
    }
    updateUIState();
  };

  window.setMediaLibrary = function(items) {
    if (!Array.isArray(items)) return;
    state.mediaLibrary = items;
    initMediaLibrary();
    if (centerView === 'gallery') renderGallery();
  };

  window.setPipeline = function(pipeline) {
    state.pipelineType = pipeline;
    initGraph();
  };

  window.addEventListener('keydown', function(e) {
    if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
      e.preventDefault();
      queuePrompt();
    }
  });

  // Boot
  hydrateStaticIcons();
  if (state.theme) window.applyTheme(state.theme);
  initMediaLibrary();
  initGraph();
</script>
</body>
</html>`
	return html
}

// GenerateComfyUIHTML is an alias for GenerateNodeWorkflowHTML for backward compatibility.
func GenerateComfyUIHTML(pipelineType string, state ComfyUIInitialState) string {
	return GenerateNodeWorkflowHTML(pipelineType, state)
}
