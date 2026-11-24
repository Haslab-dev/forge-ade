package studio

// NodePort represents an input or output connection pin on a node.
type NodePort struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	DataType  string `json:"dataType"` // "text", "image", "audio", "video", "code", "skill", "signal"
	Connected bool   `json:"connected"`
}

// Node represents a functional block in the node graph.
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

// DefaultGraphForType generates an initial node graph for a project type.
func DefaultGraphForType(t ArtifactType) NodeGraph {
	switch t {
	case ArtifactTypeImage:
		return NodeGraph{
			Nodes: []Node{
				{
					ID:       "node-ref",
					Type:     "reference",
					Title:    "Reference Media",
					Category: "Reference",
					X:        40,
					Y:        40,
					Width:    220,
					Height:   120,
					Outputs:  []NodePort{{ID: "out-ref", Name: "Reference", DataType: "image"}},
				},
				{
					ID:       "node-prompt",
					Type:     "prompt",
					Title:    "Creative Prompt",
					Category: "Input",
					X:        40,
					Y:        190,
					Width:    220,
					Height:   110,
					Outputs:  []NodePort{{ID: "out-text", Name: "Prompt", DataType: "text"}},
				},
				{
					ID:       "node-gen",
					Type:     "generator",
					Title:    "Image Engine (KSampler)",
					Category: "Generative",
					X:        320,
					Y:        100,
					Width:    240,
					Height:   140,
					Inputs: []NodePort{
						{ID: "in-ref", Name: "Reference", DataType: "image"},
						{ID: "in-prompt", Name: "Prompt", DataType: "text"},
					},
					Outputs: []NodePort{{ID: "out-img", Name: "Image", DataType: "image"}},
				},
				{
					ID:       "node-export",
					Type:     "export",
					Title:    "Preview & Export",
					Category: "Output",
					X:        620,
					Y:        110,
					Width:    200,
					Height:   110,
					Inputs:   []NodePort{{ID: "in-art", Name: "Image", DataType: "image"}},
				},
			},
			Links: []NodeLink{
				{ID: "link-1", FromNode: "node-ref", FromPort: "out-ref", ToNode: "node-gen", ToPort: "in-ref"},
				{ID: "link-2", FromNode: "node-prompt", FromPort: "out-text", ToNode: "node-gen", ToPort: "in-prompt"},
				{ID: "link-3", FromNode: "node-gen", FromPort: "out-img", ToNode: "node-export", ToPort: "in-art"},
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

	default: // Video (Full Flow: Reference + Prompt + Storyboard Plan + Keyframe Image -> Video)
		return NodeGraph{
			Nodes: []Node{
				{
					ID:       "node-ref",
					Type:     "reference",
					Title:    "Reference Media",
					Category: "Reference",
					X:        40,
					Y:        40,
					Width:    220,
					Height:   110,
					Outputs:  []NodePort{{ID: "out-ref", Name: "Reference", DataType: "image"}},
				},
				{
					ID:       "node-prompt",
					Type:     "prompt",
					Title:    "Concept & Prompt",
					Category: "Input",
					X:        40,
					Y:        180,
					Width:    220,
					Height:   110,
					Outputs:  []NodePort{{ID: "out-prompt", Name: "Prompt", DataType: "text"}},
				},
				{
					ID:       "node-plan",
					Type:     "plan",
					Title:    "Storyboard Plan",
					Category: "Director",
					X:        310,
					Y:        160,
					Width:    240,
					Height:   130,
					Inputs: []NodePort{
						{ID: "in-ref", Name: "Reference", DataType: "image"},
						{ID: "in-prompt", Name: "Prompt", DataType: "text"},
					},
					Outputs: []NodePort{{ID: "out-plan", Name: "Shot Plan", DataType: "text"}},
				},
				{
					ID:       "node-image-gen",
					Type:     "generator",
					Title:    "Keyframe Image Gen",
					Category: "Image",
					X:        310,
					Y:        20,
					Width:    240,
					Height:   120,
					Inputs: []NodePort{
						{ID: "in-ref", Name: "Reference", DataType: "image"},
						{ID: "in-prompt", Name: "Prompt", DataType: "text"},
					},
					Outputs: []NodePort{{ID: "out-image", Name: "Keyframe", DataType: "image"}},
				},
				{
					ID:       "node-video-engine",
					Type:     "generator",
					Title:    "Video Engine (Veo / Kling)",
					Category: "Generative",
					X:        610,
					Y:        70,
					Width:    240,
					Height:   150,
					Inputs: []NodePort{
						{ID: "in-ref", Name: "Reference", DataType: "image"},
						{ID: "in-prompt", Name: "Prompt", DataType: "text"},
						{ID: "in-image", Name: "Keyframe", DataType: "image"},
						{ID: "in-plan", Name: "Plan", DataType: "text"},
					},
					Outputs: []NodePort{{ID: "out-video", Name: "Video", DataType: "video"}},
				},
				{
					ID:       "node-export",
					Type:     "export",
					Title:    "Video Preview & Export",
					Category: "Output",
					X:        910,
					Y:        90,
					Width:    200,
					Height:   110,
					Inputs:   []NodePort{{ID: "in-video", Name: "Video", DataType: "video"}},
				},
			},
			Links: []NodeLink{
				{ID: "link-1", FromNode: "node-ref", FromPort: "out-ref", ToNode: "node-image-gen", ToPort: "in-ref"},
				{ID: "link-2", FromNode: "node-prompt", FromPort: "out-prompt", ToNode: "node-image-gen", ToPort: "in-prompt"},
				{ID: "link-3", FromNode: "node-prompt", FromPort: "out-prompt", ToNode: "node-plan", ToPort: "in-prompt"},
				{ID: "link-4", FromNode: "node-ref", FromPort: "out-ref", ToNode: "node-video-engine", ToPort: "in-ref"},
				{ID: "link-5", FromNode: "node-image-gen", FromPort: "out-image", ToNode: "node-video-engine", ToPort: "in-image"},
				{ID: "link-6", FromNode: "node-plan", FromPort: "out-plan", ToNode: "node-video-engine", ToPort: "in-plan"},
				{ID: "link-7", FromNode: "node-video-engine", FromPort: "out-video", ToNode: "node-export", ToPort: "in-video"},
			},
		}
	}
}
