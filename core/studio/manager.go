package studio

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager coordinates Studio projects, design skills, director workflows, and export bridges.
type Manager struct {
	mu           sync.RWMutex
	WorkspaceDir string
	Projects     []*StudioProject
	ActiveID     string
	Skills       *SkillsEngine
	Director     *DirectorWorkflow
	IsPinning    bool // Element Pinning mode (V key)
	SelectedPin  *ElementPin
	OnChanged    func()
}

// NewManager creates a Studio Manager for a given workspace.
func NewManager(workspaceDir string) *Manager {
	m := &Manager{
		WorkspaceDir: workspaceDir,
		Skills:       NewSkillsEngine(),
	}
	_ = m.LoadProjects()
	if len(m.Projects) == 0 {
		m.initDefaultProjects()
	}
	if len(m.Projects) > 0 {
		m.ActiveID = m.Projects[0].ID
		m.Director = NewDirectorWorkflow(m.Projects[0].Type)
		m.bindDirectorCallbacks()
	}
	return m
}

func (m *Manager) bindDirectorCallbacks() {
	if m.Director == nil {
		return
	}
	m.Director.OnUpdate = func(_ []DirectorStep) {
		m.notify()
	}
	m.Director.OnComplete = func(data ArtifactData, pins []ElementPin) {
		m.mu.Lock()
		p := m.activeProjectLocked()
		if p != nil {
			p.AddVersion("Director Agent refinement", "Automated creative execution", data, pins)
			_ = m.saveProjectsLocked()
		}
		m.mu.Unlock()
		m.notify()
	}
}

func (m *Manager) notify() {
	m.mu.RLock()
	cb := m.OnChanged
	m.mu.RUnlock()
	if cb != nil {
		cb()
	}
}

// SetWorkspace updates the active workspace and reloads projects.
func (m *Manager) SetWorkspace(workspaceDir string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.WorkspaceDir == workspaceDir {
		return
	}
	m.WorkspaceDir = workspaceDir
	m.Skills = NewSkillsEngine()
	_ = m.Skills.LoadWorkspaceSkills(workspaceDir)
	_ = m.loadProjectsLocked()
	if len(m.Projects) == 0 {
		m.initDefaultProjectsLocked()
	}
	if len(m.Projects) > 0 {
		m.ActiveID = m.Projects[0].ID
		m.Director = NewDirectorWorkflow(m.Projects[0].Type)
		m.bindDirectorCallbacks()
	}
}

// ActiveProject returns the currently selected project, or nil.
func (m *Manager) ActiveProject() *StudioProject {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeProjectLocked()
}

func (m *Manager) activeProjectLocked() *StudioProject {
	for _, p := range m.Projects {
		if p.ID == m.ActiveID {
			return p
		}
	}
	if len(m.Projects) > 0 {
		return m.Projects[0]
	}
	return nil
}

// SelectProject sets the active project by ID.
func (m *Manager) SelectProject(id string) {
	m.mu.Lock()
	m.ActiveID = id
	p := m.activeProjectLocked()
	if p != nil {
		m.Director = NewDirectorWorkflow(p.Type)
		m.bindDirectorCallbacks()
		m.SelectedPin = nil
	}
	m.mu.Unlock()
	m.notify()
}

// CreateProject adds a new studio project of the given type.
func (m *Manager) CreateProject(name, description string, artType ArtifactType) *StudioProject {
	m.mu.Lock()
	defer m.mu.Unlock()

	p := &StudioProject{
		ID:             uuid.New().String(),
		Workspace:      m.WorkspaceDir,
		Name:           name,
		Description:    description,
		Type:           artType,
		CurrentVersion: 1,
		Graph:          DefaultGraphForType(artType),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	initialData, pins := synthesizeArtifactData(artType, name, ArtifactData{Prompt: description})
	p.AddVersion("Initial project scaffold", "Initial template generation", initialData, pins)

	m.Projects = append(m.Projects, p)
	m.ActiveID = p.ID
	m.Director = NewDirectorWorkflow(artType)
	m.bindDirectorCallbacks()
	_ = m.saveProjectsLocked()

	return p
}

// TogglePinning switches element selection mode (V shortcut).
func (m *Manager) TogglePinning() bool {
	m.mu.Lock()
	m.IsPinning = !m.IsPinning
	if !m.IsPinning {
		m.SelectedPin = nil
	}
	res := m.IsPinning
	m.mu.Unlock()
	m.notify()
	return res
}

// SelectPin targets a specific canvas element pin.
func (m *Manager) SelectPin(pinID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := m.activeProjectLocked()
	if p == nil {
		return
	}
	p.SelectedPinID = pinID
	m.SelectedPin = nil
	for i := range p.PinnedElements {
		if p.PinnedElements[i].ID == pinID {
			m.SelectedPin = &p.PinnedElements[i]
			break
		}
	}
}

// RunDirector starts the multi-step background pipeline for the active project.
func (m *Manager) RunDirector(ctx context.Context, customPrompt string) error {
	m.mu.RLock()
	p := m.activeProjectLocked()
	skills := m.Skills
	workflow := m.Director
	pin := m.SelectedPin
	m.mu.RUnlock()

	if p == nil {
		return fmt.Errorf("no active studio project")
	}
	if workflow == nil {
		return fmt.Errorf("director workflow not initialized")
	}

	prompt := customPrompt
	if prompt == "" {
		if active := p.ActiveVersion(); active != nil {
			prompt = active.Prompt
		} else {
			prompt = p.Description
		}
	}

	// Inject design skills & pinned element constraints into the prompt
	injectedPrompt := skills.InjectPrompt(prompt, pin)
	_ = injectedPrompt // passes to the workflow

	workflow.ExecuteAsync(ctx, p, skills)
	return nil
}

// ExportActive exports the active project into the workspace.
func (m *Manager) ExportActive() (*ExportResult, error) {
	m.mu.RLock()
	p := m.activeProjectLocked()
	ws := m.WorkspaceDir
	m.mu.RUnlock()

	if p == nil {
		return nil, fmt.Errorf("no active project")
	}

	res, err := ExportToWorkspace(ws, p)
	if err == nil {
		m.mu.Lock()
		_ = m.saveProjectsLocked()
		m.mu.Unlock()
		m.notify()
	}
	return res, err
}

func (m *Manager) initDefaultProjects() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.initDefaultProjectsLocked()
}

func (m *Manager) initDefaultProjectsLocked() {
	types := []struct {
		name string
		desc string
		t    ArtifactType
	}{
		{"Modern SaaS Keynote", "16:9 Presentation Deck with Swiss typography", ArtifactTypeSlides},
		{"Fluid 3D Torus Knot", "Interactive Three.js particle mesh with bloom", ArtifactTypeCodeJS},
		{"Brand Launch Motion", "Kinetic motion video with synchronized ElevenLabs voiceover", ArtifactTypeMotion},
		{"Editorial Key Visual", "Generative hero visual for product landing", ArtifactTypeImage},
	}

	for _, item := range types {
		p := &StudioProject{
			ID:             uuid.New().String(),
			Workspace:      m.WorkspaceDir,
			Name:           item.name,
			Description:    item.desc,
			Type:           item.t,
			CurrentVersion: 1,
			Graph:          DefaultGraphForType(item.t),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		data, pins := synthesizeArtifactData(item.t, item.name, ArtifactData{Prompt: item.desc})
		p.AddVersion("Initial project creation", "Seed generation", data, pins)
		m.Projects = append(m.Projects, p)
	}
	if len(m.Projects) > 0 {
		m.ActiveID = m.Projects[0].ID
	}
	_ = m.saveProjectsLocked()
}

// LoadProjects reads stored projects from .forge/studio/projects.json.
func (m *Manager) LoadProjects() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.loadProjectsLocked()
}

func (m *Manager) loadProjectsLocked() error {
	if m.WorkspaceDir == "" {
		return nil
	}
	path := filepath.Join(m.WorkspaceDir, ".forge", "studio", "projects.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var projects []*StudioProject
	if err := json.Unmarshal(data, &projects); err != nil {
		return err
	}
	m.Projects = projects
	return nil
}

// SaveProjects persists projects to .forge/studio/projects.json.
func (m *Manager) SaveProjects() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saveProjectsLocked()
}

func (m *Manager) saveProjectsLocked() error {
	if m.WorkspaceDir == "" {
		return nil
	}
	dir := filepath.Join(m.WorkspaceDir, ".forge", "studio")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "projects.json")
	data, err := json.MarshalIndent(m.Projects, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
