package studio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// WorkflowStepType identifies a stage inside a customizable native Studio pipeline.
type WorkflowStepType string

const (
	StepTypeReference  WorkflowStepType = "reference"
	StepTypePrompt     WorkflowStepType = "prompt"
	StepTypeStoryboard WorkflowStepType = "storyboard"
	StepTypeImageGen   WorkflowStepType = "image_gen"
	StepTypeVideoGen   WorkflowStepType = "video_gen"
	StepTypeUpscale    WorkflowStepType = "upscale"
	StepTypeOutput     WorkflowStepType = "output"
)

// WorkflowStep is a single editable, reorderable, bypassable stage in a native workflow.
type WorkflowStep struct {
	ID         string                 `json:"id"`
	Type       WorkflowStepType       `json:"type"`
	Title      string                 `json:"title"`
	Category   string                 `json:"category"`
	InputType  string                 `json:"inputType"`  // "none", "text", "image", "latent", "video", "any"
	OutputType string                 `json:"outputType"` // "text", "image", "latent", "video"
	Enabled    bool                   `json:"enabled"`
	Params     map[string]interface{} `json:"params,omitempty"`
}

// GetString returns a string parameter or fallback if missing.
func (s *WorkflowStep) GetString(key, fallback string) string {
	if s == nil || s.Params == nil {
		return fallback
	}
	if v, ok := s.Params[key]; ok {
		if str, ok := v.(string); ok {
			return str
		}
		return fmt.Sprintf("%v", v)
	}
	return fallback
}

// GetInt returns an integer parameter or fallback if missing.
func (s *WorkflowStep) GetInt(key string, fallback int) int {
	if s == nil || s.Params == nil {
		return fallback
	}
	v, ok := s.Params[key]
	if !ok {
		return fallback
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(n)); err == nil {
			return parsed
		}
	}
	return fallback
}

// GetFloat returns a float64 parameter or fallback if missing.
func (s *WorkflowStep) GetFloat(key string, fallback float64) float64 {
	if s == nil || s.Params == nil {
		return fallback
	}
	v, ok := s.Params[key]
	if !ok {
		return fallback
	}
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case string:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(n), 64); err == nil {
			return parsed
		}
	}
	return fallback
}

// SetParam assigns a parameter value on the step.
func (s *WorkflowStep) SetParam(key string, val interface{}) {
	if s == nil {
		return
	}
	if s.Params == nil {
		s.Params = map[string]interface{}{}
	}
	s.Params[key] = val
}

// WorkflowPipeline is ForgeADE's customizable native workflow definition.
// It supports both visual stage editing ("pipeline") and injectable JSON templates ("template").
type WorkflowPipeline struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	Description  string             `json:"description"`
	PipelineType string             `json:"pipelineType"` // "image" or "video"
	Mode         string             `json:"mode"`         // "pipeline" or "template"
	Steps        []WorkflowStep     `json:"steps"`
	TemplateJSON string             `json:"templateJson,omitempty"`
	Variables    []WorkflowVariable `json:"variables,omitempty"`
}

// AddStep appends a new step before the Output step (if present) or at the end.
func (w *WorkflowPipeline) AddStep(step WorkflowStep) {
	if w == nil {
		return
	}
	if step.ID == "" {
		step.ID = fmt.Sprintf("step-%d", time.Now().UnixNano()%1000000)
	}
	if step.Params == nil {
		step.Params = map[string]interface{}{}
	}
	if len(w.Steps) > 0 && w.Steps[len(w.Steps)-1].Type == StepTypeOutput && step.Type != StepTypeOutput {
		last := w.Steps[len(w.Steps)-1]
		w.Steps[len(w.Steps)-1] = step
		w.Steps = append(w.Steps, last)
		return
	}
	w.Steps = append(w.Steps, step)
}

// RemoveStep deletes a step by ID.
func (w *WorkflowPipeline) RemoveStep(id string) bool {
	if w == nil {
		return false
	}
	for i, s := range w.Steps {
		if s.ID == id {
			w.Steps = append(w.Steps[:i], w.Steps[i+1:]...)
			return true
		}
	}
	return false
}

// MoveStep shifts a step up (delta < 0) or down (delta > 0) in the pipeline.
func (w *WorkflowPipeline) MoveStep(id string, delta int) bool {
	if w == nil || delta == 0 {
		return false
	}
	idx := -1
	for i, s := range w.Steps {
		if s.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false
	}
	target := idx + delta
	if target < 0 || target >= len(w.Steps) {
		return false
	}
	w.Steps[idx], w.Steps[target] = w.Steps[target], w.Steps[idx]
	return true
}

// ToggleStep flips the Enabled flag (bypass/mute) for a step.
func (w *WorkflowPipeline) ToggleStep(id string) bool {
	if w == nil {
		return false
	}
	for i := range w.Steps {
		if w.Steps[i].ID == id {
			w.Steps[i].Enabled = !w.Steps[i].Enabled
			return true
		}
	}
	return false
}

// ExportJSON serializes the workflow to pretty-printed JSON.
func (w *WorkflowPipeline) ExportJSON() ([]byte, error) {
	if w == nil {
		return nil, fmt.Errorf("nil workflow")
	}
	return json.MarshalIndent(w, "", "  ")
}

// ImportWorkflowJSON imports a ForgeADE workflow JSON (either a direct pipeline
// definition or a reusable JSON template containing {{variable}} placeholders).
func ImportWorkflowJSON(raw []byte, defaultPipelineType string) (*WorkflowPipeline, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty workflow JSON")
	}
	if defaultPipelineType == "" {
		defaultPipelineType = "image"
	}

	rawStr := string(trimmed)
	// If the JSON contains {{variable}} placeholders, import it in injectable template mode.
	if workflowVarRe.MatchString(rawStr) {
		vars, err := ParseWorkflowVariables(rawStr)
		if err != nil {
			return nil, err
		}
		return &WorkflowPipeline{
			ID:           fmt.Sprintf("tpl-%d", time.Now().Unix()%100000),
			Name:         "Imported Workflow Template",
			Description:  fmt.Sprintf("Injectable JSON workflow (%d variables)", len(vars)),
			PipelineType: defaultPipelineType,
			Mode:         "template",
			TemplateJSON: rawStr,
			Variables:    vars,
		}, nil
	}

	var wf WorkflowPipeline
	if err := json.Unmarshal(trimmed, &wf); err != nil {
		return nil, fmt.Errorf("invalid workflow JSON: %w", err)
	}
	if wf.Mode == "" {
		if wf.TemplateJSON != "" && len(wf.Steps) == 0 {
			wf.Mode = "template"
		} else {
			wf.Mode = "pipeline"
		}
	}
	if wf.PipelineType == "" {
		wf.PipelineType = defaultPipelineType
	}
	if wf.Mode == "template" && wf.TemplateJSON != "" && len(wf.Variables) == 0 {
		if vars, err := ParseWorkflowVariables(wf.TemplateJSON); err == nil {
			wf.Variables = vars
		}
	}
	return &wf, nil
}

// NewDefaultStep constructs a ready-to-use WorkflowStep of the requested type.
func NewDefaultStep(stepType WorkflowStepType, index int) WorkflowStep {
	id := fmt.Sprintf("%s-%d", stepType, index)
	switch stepType {
	case StepTypeReference:
		return WorkflowStep{
			ID:         id,
			Type:       StepTypeReference,
			Title:      "Reference Media",
			Category:   "Input",
			InputType:  "none",
			OutputType: "image",
			Enabled:    true,
			Params: map[string]interface{}{
				"path":     "",
				"strength": 0.65,
			},
		}
	case StepTypePrompt:
		return WorkflowStep{
			ID:         id,
			Type:       StepTypePrompt,
			Title:      "Prompt & Conditioning",
			Category:   "Conditioning",
			InputType:  "none",
			OutputType: "text",
			Enabled:    true,
			Params: map[string]interface{}{
				"positive": "Cyberpunk neon street at dusk, cinematic volumetric fog, raytraced reflections, 8k",
				"negative": "blurry, deformed, low quality, watermark, bad anatomy",
			},
		}
	case StepTypeStoryboard:
		return WorkflowStep{
			ID:         id,
			Type:       StepTypeStoryboard,
			Title:      "Storyboard & Camera Plan",
			Category:   "Director",
			InputType:  "text",
			OutputType: "text",
			Enabled:    true,
			Params: map[string]interface{}{
				"camera": "Slow Dolly In",
				"shots":  "0-3s wide establishing shot; 3-6s dynamic tracking pan; 6-8s dramatic close-up",
			},
		}
	case StepTypeImageGen:
		return WorkflowStep{
			ID:         id,
			Type:       StepTypeImageGen,
			Title:      "Image Generator",
			Category:   "Generator",
			InputType:  "text",
			OutputType: "image",
			Enabled:    true,
			Params: map[string]interface{}{
				"model":        "flux-1-schnell",
				"sampler":      "Euler a",
				"steps":        25,
				"cfg":          7.5,
				"seed":         4928172,
				"aspect_ratio": "16:9",
			},
		}
	case StepTypeVideoGen:
		return WorkflowStep{
			ID:         id,
			Type:       StepTypeVideoGen,
			Title:      "Video Generator",
			Category:   "Generator",
			InputType:  "image",
			OutputType: "video",
			Enabled:    true,
			Params: map[string]interface{}{
				"model":         "veo-2-ai",
				"duration":      6,
				"fps":           24,
				"motion_bucket": 127,
				"aspect_ratio":  "16:9",
			},
		}
	case StepTypeUpscale:
		return WorkflowStep{
			ID:         id,
			Type:       StepTypeUpscale,
			Title:      "Hi-Res Upscaler",
			Category:   "Post-Process",
			InputType:  "image",
			OutputType: "image",
			Enabled:    true,
			Params: map[string]interface{}{
				"scale":   2,
				"denoise": 0.35,
			},
		}
	case StepTypeOutput:
		return WorkflowStep{
			ID:         id,
			Type:       StepTypeOutput,
			Title:      "Output & Export",
			Category:   "Output",
			InputType:  "any",
			OutputType: "image",
			Enabled:    true,
			Params: map[string]interface{}{
				"prefix": "forge_studio",
				"format": "PNG",
			},
		}
	default:
		return WorkflowStep{
			ID:         id,
			Type:       stepType,
			Title:      "Custom Stage",
			Category:   "Custom",
			InputType:  "any",
			OutputType: "image",
			Enabled:    true,
			Params:     map[string]interface{}{},
		}
	}
}

// BuiltinWorkflowPresets returns all built-in customizable presets for Image & Video Studio.
func BuiltinWorkflowPresets() []WorkflowPipeline {
	return []WorkflowPipeline{
		PresetTextToImage(),
		PresetImageToImage(),
		PresetImageJSONTemplate(),
		PresetTextToVideoDirect(),
		PresetImageToVideo(),
		PresetTextToVideo(),
		PresetVideoJSONTemplate(),
	}
}

// PresetsForPipelineType returns only the built-in presets belonging to the
// requested domain ("image" or "video") so Image Studio and Video Studio stay cleanly separated.
func PresetsForPipelineType(pipelineType string) []WorkflowPipeline {
	all := BuiltinWorkflowPresets()
	out := make([]WorkflowPipeline, 0, len(all))
	for _, p := range all {
		if p.PipelineType == pipelineType {
			out = append(out, p)
		}
	}
	return out
}

// PresetTextToVideoDirect builds a focused 3-stage Text -> Video workflow.
func PresetTextToVideoDirect() WorkflowPipeline {
	promptStep := NewDefaultStep(StepTypePrompt, 1)
	promptStep.SetParam("positive", "Cinematic slow motion aerial shot of futuristic neon megalopolis at dusk, ultra realistic 4k")
	outStep := NewDefaultStep(StepTypeOutput, 3)
	outStep.OutputType = "video"
	outStep.SetParam("format", "MP4")
	return WorkflowPipeline{
		ID:           "preset-txt2vid-direct",
		Name:         "Text → Video",
		Description:  "Direct prompt-to-video synthesis with duration and motion controls",
		PipelineType: "video",
		Mode:         "pipeline",
		Steps: []WorkflowStep{
			promptStep,
			NewDefaultStep(StepTypeVideoGen, 2),
			outStep,
		},
	}
}

// PresetTextToImage builds a clean 3-stage Text -> Image workflow.
func PresetTextToImage() WorkflowPipeline {
	return WorkflowPipeline{
		ID:           "preset-txt2img",
		Name:         "Text → Image",
		Description:  "Prompt conditioning into native image generator",
		PipelineType: "image",
		Mode:         "pipeline",
		Steps: []WorkflowStep{
			NewDefaultStep(StepTypePrompt, 1),
			NewDefaultStep(StepTypeImageGen, 2),
			NewDefaultStep(StepTypeOutput, 3),
		},
	}
}

// PresetImageToImage builds a Reference + Prompt -> Image -> Upscale workflow.
func PresetImageToImage() WorkflowPipeline {
	return WorkflowPipeline{
		ID:           "preset-img2img",
		Name:         "Img2Img + Upscale",
		Description:  "Reference media guided generation with 2x upscale",
		PipelineType: "image",
		Mode:         "pipeline",
		Steps: []WorkflowStep{
			NewDefaultStep(StepTypeReference, 1),
			NewDefaultStep(StepTypePrompt, 2),
			NewDefaultStep(StepTypeImageGen, 3),
			NewDefaultStep(StepTypeUpscale, 4),
			NewDefaultStep(StepTypeOutput, 5),
		},
	}
}

// PresetTextToVideo builds a Prompt -> Storyboard -> Keyframe -> Video workflow.
func PresetTextToVideo() WorkflowPipeline {
	promptStep := NewDefaultStep(StepTypePrompt, 1)
	promptStep.SetParam("positive", "Cinematic slow motion aerial shot of futuristic neon megalopolis at dusk, ultra realistic 4k")
	outStep := NewDefaultStep(StepTypeOutput, 5)
	outStep.OutputType = "video"
	outStep.SetParam("format", "MP4")
	return WorkflowPipeline{
		ID:           "preset-txt2vid",
		Name:         "Prompt → Keyframe → Video",
		Description:  "Storyboard plan, hero keyframe synthesis, then video motion",
		PipelineType: "video",
		Mode:         "pipeline",
		Steps: []WorkflowStep{
			promptStep,
			NewDefaultStep(StepTypeStoryboard, 2),
			NewDefaultStep(StepTypeImageGen, 3),
			NewDefaultStep(StepTypeVideoGen, 4),
			outStep,
		},
	}
}

// PresetImageToVideo builds a Reference -> Prompt -> Video workflow.
func PresetImageToVideo() WorkflowPipeline {
	promptStep := NewDefaultStep(StepTypePrompt, 2)
	promptStep.SetParam("positive", "Smooth cinematic camera orbit around subject, volumetric rim light, 24fps")
	outStep := NewDefaultStep(StepTypeOutput, 4)
	outStep.OutputType = "video"
	outStep.SetParam("format", "MP4")
	return WorkflowPipeline{
		ID:           "preset-img2vid",
		Name:         "Image → Video",
		Description:  "Animate a reference frame with motion guidance",
		PipelineType: "video",
		Mode:         "pipeline",
		Steps: []WorkflowStep{
			NewDefaultStep(StepTypeReference, 1),
			promptStep,
			NewDefaultStep(StepTypeVideoGen, 3),
			outStep,
		},
	}
}

// PresetImageJSONTemplate returns a reusable, injectable ForgeADE Image workflow JSON template.
func PresetImageJSONTemplate() WorkflowPipeline {
	raw := `{
  "id": "tpl-image-inject",
  "name": "Injectable Image Workflow",
  "pipelineType": "image",
  "mode": "pipeline",
  "variables": {
    "positive_prompt": "Cyberpunk neon street at dusk, cinematic volumetric fog, 8k",
    "negative_prompt": "blurry, deformed, watermark",
    "model": "flux-1-schnell",
    "aspect_ratio": "16:9",
    "steps": "25"
  },
  "steps": [
    {
      "id": "prompt-1",
      "type": "prompt",
      "title": "Prompt & Conditioning",
      "category": "Conditioning",
      "inputType": "none",
      "outputType": "text",
      "enabled": true,
      "params": {
        "positive": "{{positive_prompt}}",
        "negative": "{{negative_prompt}}"
      }
    },
    {
      "id": "image_gen-2",
      "type": "image_gen",
      "title": "Image Generator",
      "category": "Generator",
      "inputType": "text",
      "outputType": "image",
      "enabled": true,
      "params": {
        "model": "{{model}}",
        "aspect_ratio": "{{aspect_ratio}}",
        "steps": "{{steps}}",
        "cfg": 7.5
      }
    },
    {
      "id": "output-3",
      "type": "output",
      "title": "Output & Export",
      "category": "Output",
      "inputType": "image",
      "outputType": "image",
      "enabled": true,
      "params": {
        "prefix": "forge_tpl",
        "format": "PNG"
      }
    }
  ]
}`
	vars, _ := ParseWorkflowVariables(raw)
	return WorkflowPipeline{
		ID:           "preset-tpl-image",
		Name:         "JSON Template (Image)",
		Description:  "Reusable JSON workflow with {{variable}} injection",
		PipelineType: "image",
		Mode:         "template",
		TemplateJSON: raw,
		Variables:    vars,
	}
}

// PresetVideoJSONTemplate returns a reusable, injectable ForgeADE Video workflow JSON template.
func PresetVideoJSONTemplate() WorkflowPipeline {
	raw := `{
  "id": "tpl-video-inject",
  "name": "Injectable Video Workflow",
  "pipelineType": "video",
  "mode": "pipeline",
  "variables": {
    "positive_prompt": "Cinematic drone shot flying through misty alpine mountains at golden hour, 4k",
    "camera_motion": "Slow Dolly In",
    "video_model": "veo-2-ai",
    "duration": "6",
    "fps": "24"
  },
  "steps": [
    {
      "id": "prompt-1",
      "type": "prompt",
      "title": "Prompt & Conditioning",
      "category": "Conditioning",
      "inputType": "none",
      "outputType": "text",
      "enabled": true,
      "params": {
        "positive": "{{positive_prompt}}",
        "negative": "flicker, blurry, lowres, watermark"
      }
    },
    {
      "id": "storyboard-2",
      "type": "storyboard",
      "title": "Storyboard & Camera Plan",
      "category": "Director",
      "inputType": "text",
      "outputType": "text",
      "enabled": true,
      "params": {
        "camera": "{{camera_motion}}",
        "shots": "0-3s wide establishing; 3-6s smooth push-in"
      }
    },
    {
      "id": "video_gen-3",
      "type": "video_gen",
      "title": "Video Generator",
      "category": "Generator",
      "inputType": "text",
      "outputType": "video",
      "enabled": true,
      "params": {
        "model": "{{video_model}}",
        "duration": "{{duration}}",
        "fps": "{{fps}}",
        "aspect_ratio": "16:9"
      }
    },
    {
      "id": "output-4",
      "type": "output",
      "title": "Output & Export",
      "category": "Output",
      "inputType": "video",
      "outputType": "video",
      "enabled": true,
      "params": {
        "prefix": "forge_video_tpl",
        "format": "MP4"
      }
    }
  ]
}`
	vars, _ := ParseWorkflowVariables(raw)
	return WorkflowPipeline{
		ID:           "preset-tpl-video",
		Name:         "JSON Template (Video)",
		Description:  "Reusable video JSON workflow with {{variable}} injection",
		PipelineType: "video",
		Mode:         "template",
		TemplateJSON: raw,
		Variables:    vars,
	}
}

// WorkflowRunOptions configures execution of a native WorkflowPipeline.
type WorkflowRunOptions struct {
	Config     StudioConfig
	OutputDir  string
	OnProgress func(stepIdx, total int, status string, progress float32)
}

// WorkflowRunResult captures the final output and per-step execution trace.
type WorkflowRunResult struct {
	FilePath     string        `json:"filePath"`
	Format       string        `json:"format"`
	Width        int           `json:"width"`
	Height       int           `json:"height"`
	Seed         int64         `json:"seed"`
	Duration     time.Duration `json:"duration"`
	FromAPI      bool          `json:"fromApi"`
	IsVideo      bool          `json:"isVideo"`
	Summary      string        `json:"summary"`
	StepLogs     []string      `json:"stepLogs"`
	InjectedJSON string        `json:"injectedJson,omitempty"`
}

// RunWorkflow executes a customizable ForgeADE native workflow (either directly
// from Pipeline Builder stages or by injecting variables into a JSON template).
func RunWorkflow(ctx context.Context, wf *WorkflowPipeline, opts WorkflowRunOptions) (*WorkflowRunResult, error) {
	if wf == nil {
		return nil, fmt.Errorf("nil workflow")
	}
	start := time.Now()
	report := func(idx, total int, msg string, prog float32) {
		if opts.OnProgress != nil {
			opts.OnProgress(idx, total, msg, prog)
		}
	}

	var injectedJSON string
	active := wf
	var logs []string

	// If running in JSON Template mode, inject variables into the workflow JSON first.
	if wf.Mode == "template" {
		report(0, 2, "Injecting variables into workflow JSON...", 0.15)
		resolvedBytes, resolvedWf, err := InjectWorkflowJSON(wf.TemplateJSON, wf.Variables)
		if err != nil {
			return nil, fmt.Errorf("workflow template injection failed: %w", err)
		}
		injectedJSON = string(resolvedBytes)
		active = resolvedWf
		if active.PipelineType == "" {
			active.PipelineType = wf.PipelineType
		}
		logs = append(logs, fmt.Sprintf("Injected %d variables into workflow JSON", len(wf.Variables)))
	}

	enabledSteps := make([]WorkflowStep, 0, len(active.Steps))
	for _, s := range active.Steps {
		if s.Enabled {
			enabledSteps = append(enabledSteps, s)
		}
	}
	if len(enabledSteps) == 0 {
		return nil, fmt.Errorf("no enabled steps in workflow")
	}

	var (
		prompt      = "Cyberpunk neon city street at dusk, cinematic volumetric fog, 8k"
		negPrompt   = ""
		refPath     = ""
		model       = opts.Config.ImageModel
		steps       = 25
		cfgScale    = float32(7.5)
		seed        = int64(time.Now().UnixNano() % 10000000)
		aspectRatio = "16:9"
		upscaleMult = 1
		lastResult  *GenerationResult
		isVideo     = active.PipelineType == "video"
	)

	total := len(enabledSteps)
	for i, step := range enabledSteps {
		prog := float32(i+1) / float32(total+1)
		report(i, total, fmt.Sprintf("Step %d/%d: %s...", i+1, total, step.Title), prog)

		switch step.Type {
		case StepTypeReference:
			refPath = strings.TrimSpace(step.GetString("path", ""))
			if refPath != "" {
				logs = append(logs, fmt.Sprintf("[%s] Loaded reference: %s", step.Title, filepath.Base(refPath)))
			} else {
				logs = append(logs, fmt.Sprintf("[%s] No reference path set (skipped)", step.Title))
			}

		case StepTypePrompt:
			if p := strings.TrimSpace(step.GetString("positive", "")); p != "" {
				prompt = p
			}
			negPrompt = strings.TrimSpace(step.GetString("negative", ""))
			logs = append(logs, fmt.Sprintf("[%s] Encoded prompt (%d chars)", step.Title, len(prompt)))

		case StepTypeStoryboard:
			camera := strings.TrimSpace(step.GetString("camera", ""))
			shots := strings.TrimSpace(step.GetString("shots", ""))
			if camera != "" {
				prompt += ", camera: " + camera
			}
			if shots != "" {
				prompt += ", storyboard: " + shots
			}
			logs = append(logs, fmt.Sprintf("[%s] Applied storyboard & camera (%s)", step.Title, camera))

		case StepTypeImageGen:
			if m := strings.TrimSpace(step.GetString("model", "")); m != "" {
				model = m
			}
			steps = step.GetInt("steps", 25)
			cfgScale = float32(step.GetFloat("cfg", 7.5))
			if s := int64(step.GetInt("seed", 0)); s > 0 {
				seed = s
			}
			if ar := strings.TrimSpace(step.GetString("aspect_ratio", "")); ar != "" {
				aspectRatio = ar
			}

			req := GenerationRequest{
				Prompt:         prompt,
				NegativePrompt: negPrompt,
				Model:          model,
				Steps:          steps,
				CFGScale:       cfgScale,
				Seed:           seed,
				AspectRatio:    aspectRatio,
				Endpoint:       opts.Config.ImageEndpoint,
				APIKey:         opts.Config.ImageAPIKey,
				Provider:       opts.Config.ImageProvider,
				OutputDir:      opts.OutputDir,
			}
			res, err := GenerateImage(ctx, req)
			if err != nil {
				return nil, fmt.Errorf("step %s failed: %w", step.Title, err)
			}
			lastResult = res
			refPath = res.FilePath
			logs = append(logs, fmt.Sprintf("[%s] Generated %dx%d %s (%s)", step.Title, res.Width, res.Height, res.Format, model))

		case StepTypeVideoGen:
			isVideo = true
			vidModel := step.GetString("model", opts.Config.VideoModel)
			if vidModel == "" {
				vidModel = "veo-2-ai"
			}
			durationSec := step.GetInt("duration", 6)
			fps := step.GetInt("fps", 24)
			if ar := strings.TrimSpace(step.GetString("aspect_ratio", "")); ar != "" {
				aspectRatio = ar
			}

			if strings.TrimSpace(opts.Config.VideoEndpoint) != "" && strings.TrimSpace(opts.Config.VideoAPIKey) != "" {
				res, err := callCustomVideoAPI(ctx, opts.Config.VideoEndpoint, opts.Config.VideoAPIKey, vidModel, prompt, refPath, durationSec, fps, opts.OutputDir, seed)
				if err == nil && res != nil {
					lastResult = res
					logs = append(logs, fmt.Sprintf("[%s] Video API rendered %s (%ds @ %dfps)", step.Title, filepath.Base(res.FilePath), durationSec, fps))
					continue
				}
				logs = append(logs, fmt.Sprintf("[%s] Video API fallback (%v)", step.Title, err))
			}

			if lastResult == nil {
				req := GenerationRequest{
					Prompt:         fmt.Sprintf("[VIDEO %ds %dfps] %s", durationSec, fps, prompt),
					NegativePrompt: negPrompt,
					Model:          vidModel,
					Steps:          steps,
					CFGScale:       cfgScale,
					Seed:           seed,
					AspectRatio:    aspectRatio,
					OutputDir:      opts.OutputDir,
				}
				res, err := GenerateImage(ctx, req)
				if err != nil {
					return nil, fmt.Errorf("step %s failed: %w", step.Title, err)
				}
				lastResult = res
			}
			logs = append(logs, fmt.Sprintf("[%s] Prepared %ds @ %dfps video keyframe (%s)", step.Title, durationSec, fps, vidModel))

		case StepTypeUpscale:
			upscaleMult = step.GetInt("scale", 2)
			denoise := step.GetFloat("denoise", 0.35)
			logs = append(logs, fmt.Sprintf("[%s] Upscale pass %dx (denoise %.2f)", step.Title, upscaleMult, denoise))

		case StepTypeOutput:
			prefix := step.GetString("prefix", "forge_studio")
			logs = append(logs, fmt.Sprintf("[%s] Finalized output (prefix: %s)", step.Title, prefix))
		}
	}

	if lastResult == nil {
		res, err := GenerateImage(ctx, GenerationRequest{
			Prompt:      prompt,
			Model:       model,
			AspectRatio: aspectRatio,
			Seed:        seed,
			OutputDir:   opts.OutputDir,
		})
		if err != nil {
			return nil, err
		}
		lastResult = res
	}

	w, h := lastResult.Width, lastResult.Height
	if upscaleMult > 1 {
		w *= upscaleMult
		h *= upscaleMult
	}

	report(total, total, "Workflow complete", 1.0)
	return &WorkflowRunResult{
		FilePath:     lastResult.FilePath,
		Format:       lastResult.Format,
		Width:        w,
		Height:       h,
		Seed:         lastResult.Seed,
		Duration:     time.Since(start),
		FromAPI:      lastResult.FromAPI,
		IsVideo:      isVideo,
		Summary:      lastResult.Summary,
		StepLogs:     logs,
		InjectedJSON: injectedJSON,
	}, nil
}

func callCustomVideoAPI(ctx context.Context, endpoint, apiKey, model, prompt, refPath string, durationSec, fps int, outDir string, seed int64) (*GenerationResult, error) {
	bodyMap := map[string]interface{}{
		"prompt":   prompt,
		"model":    model,
		"duration": durationSec,
		"fps":      fps,
	}
	if refPath != "" {
		bodyMap["image_path"] = refPath
	}
	bodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("video API HTTP %d", resp.StatusCode)
	}
	var parsed struct {
		URL  string `json:"url"`
		Path string `json:"path"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	if parsed.Path != "" {
		if _, err := os.Stat(parsed.Path); err == nil {
			return &GenerationResult{
				FilePath: parsed.Path,
				Format:   "MP4",
				Seed:     seed,
				FromAPI:  true,
				Summary:  fmt.Sprintf("Generated video via %s", model),
			}, nil
		}
	}
	return nil, fmt.Errorf("video API did not return a local file path")
}
