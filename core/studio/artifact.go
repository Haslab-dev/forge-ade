package studio

import (
	"time"
)

// ArtifactType represents one of the 5 Studio Mode creative domains.
type ArtifactType string

const (
	ArtifactTypeImage  ArtifactType = "image"
	ArtifactTypeVideo  ArtifactType = "video"
	ArtifactTypeSlides ArtifactType = "slides"
	ArtifactTypeMotion ArtifactType = "motion"
	ArtifactTypeCodeJS ArtifactType = "code_js"
)

// SlideItem represents a single presentation slide in a 16:9 deck.
type SlideItem struct {
	ID         string            `json:"id"`
	Index      int               `json:"index"`
	Title      string            `json:"title"`
	Subtitle   string            `json:"subtitle,omitempty"`
	Body       string            `json:"body,omitempty"`
	Layout     string            `json:"layout"` // "hero", "split", "cards", "quote", "minimal"
	Theme      string            `json:"theme"`  // "dark", "light", "accent"
	Bullets    []string          `json:"bullets,omitempty"`
	MediaURL   string            `json:"mediaUrl,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
}

// TTSWord represents a word-level timing from a TTS generation (e.g. ElevenLabs).
type TTSWord struct {
	Word      string  `json:"word"`
	StartTime float64 `json:"startTime"` // in seconds
	EndTime   float64 `json:"endTime"`   // in seconds
}

// MotionKeyframe defines an animation event along the timeline.
type MotionKeyframe struct {
	Time     float64           `json:"time"`   // in seconds
	TargetID string            `json:"target"` // element selector e.g. "@hero-title"
	Action   string            `json:"action"` // "fade_in", "slide_up", "zoom_in", "pulse"
	Duration float64           `json:"duration"`
	Easing   string            `json:"easing"` // "ease_out", "spring", "linear"
	Values   map[string]string `json:"values,omitempty"`
}

// ElementPin represents an element identified on the canvas for direct manipulation (V mode).
type ElementPin struct {
	ID       string            `json:"id"`
	Selector string            `json:"selector"` // e.g. "@slide-title", "@object-3d"
	Label    string            `json:"label"`
	Category string            `json:"category"` // "text", "shape", "mesh", "media", "camera"
	X        float32           `json:"x"`
	Y        float32           `json:"y"`
	Width    float32           `json:"width"`
	Height   float32           `json:"height"`
	Props    map[string]string `json:"props,omitempty"`
}

// ArtifactData holds the specialized payload for any of the 5 artifact types.
type ArtifactData struct {
	// Common / Generative
	Prompt         string `json:"prompt,omitempty"`
	NegativePrompt string `json:"negativePrompt,omitempty"`
	Model          string `json:"model,omitempty"`
	Seed           int64  `json:"seed,omitempty"`
	OutputAsset    string `json:"outputAsset,omitempty"`

	// Image
	AspectRatio string `json:"aspectRatio,omitempty"` // "1:1", "16:9", "9:16", "4:3"
	ImageStyle  string `json:"imageStyle,omitempty"`

	// Video
	DurationSec     float64 `json:"durationSec,omitempty"`
	FPS             int     `json:"fps,omitempty"`
	MotionBucket    int     `json:"motionBucket,omitempty"`
	SourceImagePath string  `json:"sourceImagePath,omitempty"`

	// Slides
	Slides      []SlideItem `json:"slides,omitempty"`
	DeckTheme   string      `json:"deckTheme,omitempty"`
	RawMarkdown string      `json:"rawMarkdown,omitempty"`

	// Motion + TTS
	ScriptText string           `json:"scriptText,omitempty"`
	VoiceID    string           `json:"voiceId,omitempty"`
	AudioPath  string           `json:"audioPath,omitempty"`
	TTSWords   []TTSWord        `json:"ttsWords,omitempty"`
	Keyframes  []MotionKeyframe `json:"keyframes,omitempty"`

	// 2D & 3D JS
	JSEngine   string   `json:"jsEngine,omitempty"` // "canvas2d", "threejs", "webgl", "p5"
	Code       string   `json:"code,omitempty"`
	Libraries  []string `json:"libraries,omitempty"`
	EntryHTML  string   `json:"entryHtml,omitempty"`
	LastErrors []string `json:"lastErrors,omitempty"`
}

// StudioArtifactVersion tracks an immutable or stepped revision of a studio project.
type StudioArtifactVersion struct {
	Version   int          `json:"version"`
	Prompt    string       `json:"prompt"`
	Summary   string       `json:"summary"`
	Timestamp time.Time    `json:"timestamp"`
	Data      ArtifactData `json:"data"`
	Pins      []ElementPin `json:"pins,omitempty"`
}

// StudioProject is the unified project entity containing history, node graph, and skills.
type StudioProject struct {
	ID             string                  `json:"id"`
	Workspace      string                  `json:"workspace"`
	Name           string                  `json:"name"`
	Description    string                  `json:"description"`
	Type           ArtifactType            `json:"type"`
	CurrentVersion int                     `json:"currentVersion"`
	Versions       []StudioArtifactVersion `json:"versions"`
	ActiveSkillIDs []string                `json:"activeSkillIds"`
	Graph          NodeGraph               `json:"graph"`
	PinnedElements []ElementPin            `json:"pinnedElements"`
	SelectedPinID  string                  `json:"selectedPinId,omitempty"`
	ExportPath     string                  `json:"exportPath,omitempty"`
	CreatedAt      time.Time               `json:"createdAt"`
	UpdatedAt      time.Time               `json:"updatedAt"`
}

// ActiveVersion returns a pointer to the current version's data, or nil.
func (p *StudioProject) ActiveVersion() *StudioArtifactVersion {
	if len(p.Versions) == 0 {
		return nil
	}
	for i := range p.Versions {
		if p.Versions[i].Version == p.CurrentVersion {
			return &p.Versions[i]
		}
	}
	return &p.Versions[len(p.Versions)-1]
}

// AddVersion appends a new version and updates CurrentVersion.
func (p *StudioProject) AddVersion(prompt, summary string, data ArtifactData, pins []ElementPin) *StudioArtifactVersion {
	nextVer := len(p.Versions) + 1
	v := StudioArtifactVersion{
		Version:   nextVer,
		Prompt:    prompt,
		Summary:   summary,
		Timestamp: time.Now(),
		Data:      data,
		Pins:      pins,
	}
	p.Versions = append(p.Versions, v)
	p.CurrentVersion = nextVer
	p.UpdatedAt = time.Now()
	p.PinnedElements = pins
	return &p.Versions[len(p.Versions)-1]
}
