package studio

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// DesignSkill encapsulates rules for typography, color, spacing, and motion.
type DesignSkill struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Category     string            `json:"category"` // "typography", "color", "layout", "motion", "deck"
	Description  string            `json:"description"`
	Enabled      bool              `json:"enabled"`
	Rules        []string          `json:"rules"`
	Typography   map[string]string `json:"typography,omitempty"`
	ColorPalette map[string]string `json:"colorPalette,omitempty"`
	Grid         string            `json:"grid,omitempty"`
	MotionRhythm string            `json:"motionRhythm,omitempty"`
	SourceFile   string            `json:"sourceFile,omitempty"`
}

// BuiltinSkills provides foundational Open Design skill sets.
func BuiltinSkills() []DesignSkill {
	return []DesignSkill{
		{
			ID:          "swiss-minimal",
			Name:        "Swiss International Typographic",
			Category:    "typography",
			Description: "High-contrast sans-serif hierarchy, asymmetric grid balance, whitespace discipline.",
			Enabled:     true,
			Rules: []string{
				"Use bold asymmetric visual hierarchy with 1.333 golden ratio type scale.",
				"Max 3 font weights across the entire composition.",
				"Generous negative space with strict 8pt alignment grid.",
				"High text-to-background contrast with minimal ornamentation.",
			},
			Typography: map[string]string{
				"fontFamily": "Inter, -apple-system, sans-serif",
				"h1":         "3.2rem font-bold tracking-tight",
				"h2":         "2.0rem font-semibold",
				"body":       "1.0rem font-normal leading-relaxed",
			},
			ColorPalette: map[string]string{
				"bg":        "#0f1115",
				"surface":   "#181b21",
				"text":      "#f3f4f6",
				"accent":    "#3b82f6",
				"highlight": "#ef4444",
			},
			Grid:         "12-column 24px gutter",
			MotionRhythm: "cubic-bezier(0.16, 1, 0.3, 1) 320ms",
		},
		{
			ID:          "editorial-dark",
			Name:        "Editorial Dark Luxury",
			Category:    "deck",
			Description: "Deep obsidian surfaces, refined serif accents, muted gold highlights, cinematic presentation.",
			Enabled:     false,
			Rules: []string{
				"Combine elegant display serif headers with clean neo-grotesque body copy.",
				"Subtle card surfaces elevated with 1px border glows.",
				"Dark mode native with luminous accent cues.",
			},
			Typography: map[string]string{
				"fontFamily": "Playfair Display, serif",
				"bodyFont":   "Inter, sans-serif",
				"h1":         "3.6rem font-serif tracking-normal",
				"body":       "1.05rem leading-loose",
			},
			ColorPalette: map[string]string{
				"bg":      "#090a0f",
				"surface": "#12141c",
				"text":    "#e5e7eb",
				"accent":  "#d4af37",
				"border":  "#1f2433",
			},
			Grid:         "8-column asymmetric 32px padding",
			MotionRhythm: "ease-out 450ms",
		},
		{
			ID:          "kinetic-motion",
			Name:        "Kinetic Rhythm & Spring Dynamics",
			Category:    "motion",
			Description: "Snappy spring physics, word-synced stagger animations, continuous focal flow.",
			Enabled:     false,
			Rules: []string{
				"Every scene entry uses staggered children reveals with 60ms offset.",
				"Match typography reveals exactly to TTS word audio timestamps.",
				"Avoid linear transitions; use overshoot damping for mechanical punch.",
			},
			MotionRhythm: "spring(mass: 1, stiffness: 180, damping: 14)",
		},
		{
			ID:          "cyber-monochrome",
			Name:        "Cyber Monochrome 3D",
			Category:    "3d",
			Description: "Wireframe geometries, high-contrast monochrome lighting, glowing vertex shaders.",
			Enabled:     false,
			Rules: []string{
				"Monochrome geometry with single neon focal accent.",
				"Clean Three.js ambient occlusion and bloom post-processing.",
				"Interactive orbit camera with smooth lerp damping.",
			},
			ColorPalette: map[string]string{
				"bg":     "#050505",
				"mesh":   "#ffffff",
				"accent": "#00ffcc",
			},
		},
	}
}

// SkillsEngine manages loading, toggling, and prompt synthesis for design skills.
type SkillsEngine struct {
	mu     sync.RWMutex
	skills map[string]DesignSkill
}

// NewSkillsEngine creates an initialized skills engine with default skills.
func NewSkillsEngine() *SkillsEngine {
	e := &SkillsEngine{
		skills: make(map[string]DesignSkill),
	}
	for _, s := range BuiltinSkills() {
		e.skills[s.ID] = s
	}
	return e
}

// LoadWorkspaceSkills loads custom skill rules from .forge/skills/*.json
func (e *SkillsEngine) LoadWorkspaceSkills(workspaceDir string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	skillsDir := filepath.Join(workspaceDir, ".forge", "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil // directory may not exist yet
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(skillsDir, entry.Name()))
		if err != nil {
			continue
		}
		var skill DesignSkill
		if err := json.Unmarshal(data, &skill); err == nil && skill.ID != "" {
			skill.SourceFile = filepath.Join(skillsDir, entry.Name())
			e.skills[skill.ID] = skill
		}
	}
	return nil
}

// List returns all registered design skills.
func (e *SkillsEngine) List() []DesignSkill {
	e.mu.RLock()
	defer e.mu.RUnlock()

	res := make([]DesignSkill, 0, len(e.skills))
	for _, s := range e.skills {
		res = append(res, s)
	}
	return res
}

// SetEnabled toggles an individual skill on or off.
func (e *SkillsEngine) SetEnabled(skillID string, enabled bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if s, exists := e.skills[skillID]; exists {
		s.Enabled = enabled
		e.skills[skillID] = s
	}
}

// ActiveSkills returns currently enabled design skills.
func (e *SkillsEngine) ActiveSkills() []DesignSkill {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var active []DesignSkill
	for _, s := range e.skills {
		if s.Enabled {
			active = append(active, s)
		}
	}
	return active
}

// InjectPrompt combines the user prompt with active design guidelines and pinned element scoping.
func (e *SkillsEngine) InjectPrompt(rawPrompt string, pinnedPin *ElementPin) string {
	active := e.ActiveSkills()
	var sb strings.Builder

	// 1. Direct manipulation scope if an element is pinned
	if pinnedPin != nil && pinnedPin.Selector != "" {
		sb.WriteString(fmt.Sprintf("[DIRECT-MANIPULATION TARGET: %s (Category: %s, Current Bounds: [%.0f, %.0f, %.0f, %.0f])]\n",
			pinnedPin.Selector, pinnedPin.Category, pinnedPin.X, pinnedPin.Y, pinnedPin.Width, pinnedPin.Height))
		sb.WriteString("CRITICAL: Only modify the targeted element specified above. Retain all other existing layout, components, and structure completely intact.\n\n")
	}

	// 2. Active Design Skills injection
	if len(active) > 0 {
		sb.WriteString("[DESIGN SYSTEM CONSTRAINTS ACTIVATED]\n")
		for _, s := range active {
			sb.WriteString(fmt.Sprintf("• %s (%s):\n", s.Name, s.Category))
			for _, r := range s.Rules {
				sb.WriteString(fmt.Sprintf("   - %s\n", r))
			}
			if s.Grid != "" {
				sb.WriteString(fmt.Sprintf("   - Grid: %s\n", s.Grid))
			}
			if s.MotionRhythm != "" {
				sb.WriteString(fmt.Sprintf("   - Motion Rhythm: %s\n", s.MotionRhythm))
			}
		}
		sb.WriteString("\n")
	}

	// 3. User prompt
	sb.WriteString("[USER DIRECTIVE]\n")
	sb.WriteString(strings.TrimSpace(rawPrompt))

	return sb.String()
}
