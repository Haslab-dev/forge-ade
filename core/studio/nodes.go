package studio

// NodePort represents an input or output connection pin on a node.
type NodePort struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	DataType  string `json:"dataType"` // "text", "image", "audio", "video", "code", "skill", "signal"
	Connected bool   `json:"connected"`
}

// Node represents a functional block in the monochrome ComfyUI graph.
type Node struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"` // e.g. "prompt", "skill", "generator", "tts", "timeline", "renderer", "export"
	Title    string                 `json:"title"`
	Category string                 `json:"category"`
	X        float32                `json:"x"`
	Y        float32                `json:"y"`
	Width    float32                `json:"width"`
	Height   float32                `json:"height"`
	Inputs   []NodePort             `json:"inputs"`
	Outputs  []NodePort             `json:"outputs"`
	Values   map[string]interface{} `json:"values,omitempty"`
}

// NodeLink represents a directed connection between two node ports.
type NodeLink struct {
	ID       string `json:"id"`
	FromNode string `json:"fromNode"`
	FromPort string `json:"fromPort"`
	ToNode   string `json:"toNode"`
	ToPort   string `json:"toPort"`
}

// NodeGraph represents the complete visual execution graph.
type NodeGraph struct {
	Nodes []Node     `json:"nodes"`
	Links []NodeLink `json:"links"`
}

// DefaultGraphForType generates an initial ComfyUI-style graph for a project type.
func DefaultGraphForType(t ArtifactType) NodeGraph {
	switch t {
	case ArtifactTypeImage:
		return NodeGraph{
			Nodes: []Node{
				{
					ID:       "node-prompt",
					Type:     "prompt",
					Title:    "Creative Prompt",
					Category: "Input",
					X:        40,
					Y:        60,
					Width:    180,
					Height:   90,
					Outputs:  []NodePort{{ID: "out-text", Name: "Prompt", DataType: "text"}},
				},
				{
					ID:       "node-skill",
					Type:     "skill",
					Title:    "Design Skill",
					Category: "Style",
					X:        40,
					Y:        180,
					Width:    180,
					Height:   90,
					Outputs:  []NodePort{{ID: "out-skill", Name: "Style Tokens", DataType: "skill"}},
				},
				{
					ID:       "node-gen",
					Type:     "generator",
					Title:    "Image Engine",
					Category: "Generative",
					X:        280,
					Y:        100,
					Width:    200,
					Height:   120,
					Inputs: []NodePort{
						{ID: "in-prompt", Name: "Prompt", DataType: "text"},
						{ID: "in-skill", Name: "Tokens", DataType: "skill"},
					},
					Outputs: []NodePort{{ID: "out-img", Name: "Image (v1/v2)", DataType: "image"}},
				},
				{
					ID:       "node-compare",
					Type:     "compare",
					Title:    "Split Compare",
					Category: "Review",
					X:        540,
					Y:        80,
					Width:    180,
					Height:   100,
					Inputs:   []NodePort{{ID: "in-img", Name: "Image", DataType: "image"}},
					Outputs:  []NodePort{{ID: "out-approved", Name: "Approved", DataType: "image"}},
				},
				{
					ID:       "node-export",
					Type:     "export",
					Title:    "Send to Editor",
					Category: "Output",
					X:        780,
					Y:        100,
					Width:    160,
					Height:   80,
					Inputs:   []NodePort{{ID: "in-art", Name: "Asset", DataType: "image"}},
				},
			},
			Links: []NodeLink{
				{ID: "link-1", FromNode: "node-prompt", FromPort: "out-text", ToNode: "node-gen", ToPort: "in-prompt"},
				{ID: "link-2", FromNode: "node-skill", FromPort: "out-skill", ToNode: "node-gen", ToPort: "in-skill"},
				{ID: "link-3", FromNode: "node-gen", FromPort: "out-img", ToNode: "node-compare", ToPort: "in-img"},
				{ID: "link-4", FromNode: "node-compare", FromPort: "out-approved", ToNode: "node-export", ToPort: "in-art"},
			},
		}

	case ArtifactTypeSlides:
		return NodeGraph{
			Nodes: []Node{
				{
					ID:       "node-outline",
					Type:     "prompt",
					Title:    "Deck Outline",
					Category: "Input",
					X:        40,
					Y:        80,
					Width:    180,
					Height:   90,
					Outputs:  []NodePort{{ID: "out-text", Name: "Outline", DataType: "text"}},
				},
				{
					ID:       "node-skill",
					Type:     "skill",
					Title:    "Presentation Tokens",
					Category: "Style",
					X:        40,
					Y:        200,
					Width:    180,
					Height:   90,
					Outputs:  []NodePort{{ID: "out-tokens", Name: "Typography/Grid", DataType: "skill"}},
				},
				{
					ID:       "node-slides-gen",
					Type:     "generator",
					Title:    "Slide Synthesizer",
					Category: "Director",
					X:        280,
					Y:        120,
					Width:    200,
					Height:   110,
					Inputs: []NodePort{
						{ID: "in-outline", Name: "Outline", DataType: "text"},
						{ID: "in-tokens", Name: "Tokens", DataType: "skill"},
					},
					Outputs: []NodePort{{ID: "out-deck", Name: "Slides JSON", DataType: "text"}},
				},
				{
					ID:       "node-preview",
					Type:     "renderer",
					Title:    "16:9 Canvas Stage",
					Category: "Viewport",
					X:        540,
					Y:        100,
					Width:    190,
					Height:   100,
					Inputs:   []NodePort{{ID: "in-deck", Name: "Slides", DataType: "text"}},
					Outputs:  []NodePort{{ID: "out-view", Name: "Rendered Deck", DataType: "code"}},
				},
				{
					ID:       "node-export",
					Type:     "export",
					Title:    "Export (.pptx / HTML)",
					Category: "Output",
					X:        780,
					Y:        120,
					Width:    170,
					Height:   80,
					Inputs:   []NodePort{{ID: "in-export", Name: "Deck", DataType: "code"}},
				},
			},
			Links: []NodeLink{
				{ID: "link-1", FromNode: "node-outline", FromPort: "out-text", ToNode: "node-slides-gen", ToPort: "in-outline"},
				{ID: "link-2", FromNode: "node-skill", FromPort: "out-tokens", ToNode: "node-slides-gen", ToPort: "in-tokens"},
				{ID: "link-3", FromNode: "node-slides-gen", FromPort: "out-deck", ToNode: "node-preview", ToPort: "in-deck"},
				{ID: "link-4", FromNode: "node-preview", FromPort: "out-view", ToNode: "node-export", ToPort: "in-export"},
			},
		}

	case ArtifactTypeMotion:
		return NodeGraph{
			Nodes: []Node{
				{
					ID:       "node-script",
					Type:     "prompt",
					Title:    "Voiceover Script",
					Category: "Input",
					X:        40,
					Y:        60,
					Width:    180,
					Height:   90,
					Outputs:  []NodePort{{ID: "out-script", Name: "Script Text", DataType: "text"}},
				},
				{
					ID:       "node-tts",
					Type:     "tts",
					Title:    "ElevenLabs TTS",
					Category: "Audio",
					X:        280,
					Y:        60,
					Width:    190,
					Height:   100,
					Inputs:   []NodePort{{ID: "in-script", Name: "Script", DataType: "text"}},
					Outputs:  []NodePort{{ID: "out-audio", Name: "Waveform & Timestamps", DataType: "audio"}},
				},
				{
					ID:       "node-timeline",
					Type:     "timeline",
					Title:    "Timeline & Keyframes",
					Category: "Sync",
					X:        530,
					Y:        100,
					Width:    200,
					Height:   110,
					Inputs:   []NodePort{{ID: "in-audio", Name: "Audio/Word Timings", DataType: "audio"}},
					Outputs:  []NodePort{{ID: "out-motion", Name: "Motion Stream", DataType: "video"}},
				},
				{
					ID:       "node-export",
					Type:     "export",
					Title:    "Export Video (MP4)",
					Category: "Output",
					X:        780,
					Y:        110,
					Width:    170,
					Height:   80,
					Inputs:   []NodePort{{ID: "in-video", Name: "Motion Video", DataType: "video"}},
				},
			},
			Links: []NodeLink{
				{ID: "link-1", FromNode: "node-script", FromPort: "out-script", ToNode: "node-tts", ToPort: "in-script"},
				{ID: "link-2", FromNode: "node-tts", FromPort: "out-audio", ToNode: "node-timeline", ToPort: "in-audio"},
				{ID: "link-3", FromNode: "node-timeline", FromPort: "out-motion", ToNode: "node-export", ToPort: "in-video"},
			},
		}

	case ArtifactTypeCodeJS:
		return NodeGraph{
			Nodes: []Node{
				{
					ID:       "node-spec",
					Type:     "prompt",
					Title:    "3D/2D Specification",
					Category: "Input",
					X:        40,
					Y:        80,
					Width:    180,
					Height:   90,
					Outputs:  []NodePort{{ID: "out-spec", Name: "Specification", DataType: "text"}},
				},
				{
					ID:       "node-js-sandbox",
					Type:     "generator",
					Title:    "JS Code Synthesizer",
					Category: "Code",
					X:        280,
					Y:        80,
					Width:    200,
					Height:   100,
					Inputs:   []NodePort{{ID: "in-spec", Name: "Spec", DataType: "text"}},
					Outputs:  []NodePort{{ID: "out-code", Name: "JavaScript", DataType: "code"}},
				},
				{
					ID:       "node-self-heal",
					Type:     "renderer",
					Title:    "Live View & Self-Heal",
					Category: "Verification",
					X:        540,
					Y:        80,
					Width:    200,
					Height:   100,
					Inputs:   []NodePort{{ID: "in-code", Name: "Code", DataType: "code"}},
					Outputs:  []NodePort{{ID: "out-tested", Name: "Clean Runtime", DataType: "code"}},
				},
				{
					ID:       "node-export",
					Type:     "export",
					Title:    "Send to Editor",
					Category: "Output",
					X:        790,
					Y:        90,
					Width:    160,
					Height:   80,
					Inputs:   []NodePort{{ID: "in-code", Name: "Code", DataType: "code"}},
				},
			},
			Links: []NodeLink{
				{ID: "link-1", FromNode: "node-spec", FromPort: "out-spec", ToNode: "node-js-sandbox", ToPort: "in-spec"},
				{ID: "link-2", FromNode: "node-js-sandbox", FromPort: "out-code", ToNode: "node-self-heal", ToPort: "in-code"},
				{ID: "link-3", FromNode: "node-self-heal", FromPort: "out-tested", ToNode: "node-export", ToPort: "in-code"},
			},
		}

	default: // Video
		return NodeGraph{
			Nodes: []Node{
				{
					ID:       "node-prompt",
					Type:     "prompt",
					Title:    "Motion Prompt",
					Category: "Input",
					X:        40,
					Y:        80,
					Width:    180,
					Height:   90,
					Outputs:  []NodePort{{ID: "out-prompt", Name: "Prompt", DataType: "text"}},
				},
				{
					ID:       "node-video-engine",
					Type:     "generator",
					Title:    "Video Engine",
					Category: "Generative",
					X:        280,
					Y:        80,
					Width:    190,
					Height:   100,
					Inputs:   []NodePort{{ID: "in-prompt", Name: "Prompt", DataType: "text"}},
					Outputs:  []NodePort{{ID: "out-video", Name: "Video Frames", DataType: "video"}},
				},
				{
					ID:       "node-export",
					Type:     "export",
					Title:    "Export MP4",
					Category: "Output",
					X:        540,
					Y:        90,
					Width:    160,
					Height:   80,
					Inputs:   []NodePort{{ID: "in-video", Name: "Video", DataType: "video"}},
				},
			},
			Links: []NodeLink{
				{ID: "link-1", FromNode: "node-prompt", FromPort: "out-prompt", ToNode: "node-video-engine", ToPort: "in-prompt"},
				{ID: "link-2", FromNode: "node-video-engine", FromPort: "out-video", ToNode: "node-export", ToPort: "in-video"},
			},
		}
	}
}
