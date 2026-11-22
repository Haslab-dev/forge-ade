package studio

import (
	"context"

	corestudio "github.com/hasdev/forge-ade/core/studio"
)

// StudioManager coordinates Studio Mode state, projects, and execution in ForgeADE Native.
type StudioManager struct {
	core    *corestudio.Manager
	RootDir string
}

// NewStudioManager creates a native StudioManager for the workspace root.
func NewStudioManager(rootDir string) *StudioManager {
	return &StudioManager{
		core:    corestudio.NewManager(rootDir),
		RootDir: rootDir,
	}
}

// Core returns the underlying core studio manager.
func (s *StudioManager) Core() *corestudio.Manager {
	if s == nil {
		return nil
	}
	return s.core
}

// SetWorkspace updates the workspace root and re-binds projects.
func (s *StudioManager) SetWorkspace(rootDir string) {
	if s == nil {
		return
	}
	s.RootDir = rootDir
	if s.core != nil {
		s.core.SetWorkspace(rootDir)
	} else {
		s.core = corestudio.NewManager(rootDir)
	}
}

// ActiveProject returns the currently active studio project.
func (s *StudioManager) ActiveProject() *corestudio.StudioProject {
	if s == nil || s.core == nil {
		return nil
	}
	return s.core.ActiveProject()
}

// Projects returns all projects.
func (s *StudioManager) Projects() []*corestudio.StudioProject {
	if s == nil || s.core == nil {
		return nil
	}
	return s.core.Projects
}

// SelectProject switches the active project by ID.
func (s *StudioManager) SelectProject(id string) {
	if s == nil || s.core == nil {
		return
	}
	s.core.SelectProject(id)
}

// CreateProject creates a new project.
func (s *StudioManager) CreateProject(name, desc string, artType corestudio.ArtifactType) *corestudio.StudioProject {
	if s == nil || s.core == nil {
		return nil
	}
	return s.core.CreateProject(name, desc, artType)
}

// TogglePinning toggles element selection mode on the canvas (V key).
func (s *StudioManager) TogglePinning() bool {
	if s == nil || s.core == nil {
		return false
	}
	return s.core.TogglePinning()
}

// IsPinning returns true if element pinning is active.
func (s *StudioManager) IsPinning() bool {
	if s == nil || s.core == nil {
		return false
	}
	return s.core.IsPinning
}

// SelectedPin returns the currently focused canvas element pin.
func (s *StudioManager) SelectedPin() *corestudio.ElementPin {
	if s == nil || s.core == nil {
		return nil
	}
	return s.core.SelectedPin
}

// SelectPin targets a specific canvas pin.
func (s *StudioManager) SelectPin(id string) {
	if s == nil || s.core == nil {
		return
	}
	s.core.SelectPin(id)
}

// Skills returns the design skills engine.
func (s *StudioManager) Skills() *corestudio.SkillsEngine {
	if s == nil || s.core == nil {
		return nil
	}
	return s.core.Skills
}

// Director returns the director workflow.
func (s *StudioManager) Director() *corestudio.DirectorWorkflow {
	if s == nil || s.core == nil {
		return nil
	}
	return s.core.Director
}

// RunDirector executes the multi-step background pipeline.
func (s *StudioManager) RunDirector(ctx context.Context, prompt string) error {
	if s == nil || s.core == nil {
		return nil
	}
	return s.core.RunDirector(ctx, prompt)
}

// ExportActive triggers Cross-Mode handoff into the workspace.
func (s *StudioManager) ExportActive() (*corestudio.ExportResult, error) {
	if s == nil || s.core == nil {
		return nil, nil
	}
	return s.core.ExportActive()
}

// ImportAgentArtifact imports an agent-authored file (md deck, html page, or
// 3D scene js) as a new version of the active project.
func (s *StudioManager) ImportAgentArtifact(absPath string) (bool, error) {
	if s == nil || s.core == nil {
		return false, nil
	}
	return s.core.ImportAgentArtifact(absPath)
}

// SetOnChanged sets the reactive notification callback.
func (s *StudioManager) SetOnChanged(cb func()) {
	if s == nil || s.core == nil {
		return
	}
	s.core.OnChanged = cb
}
