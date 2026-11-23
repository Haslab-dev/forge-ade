package studio

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ImportAgentArtifact imports a file the Studio Agent wrote during a turn as
// a new version of the active project, so the viewer shows the agent's real
// output instead of the seeded placeholder:
//
//   - slides project + .md: parsed into slides (ParseMarkdownDeck)
//   - any creative project + .html/.htm: shown directly as the viewer page
//   - 3D project + .js: becomes the live scene code
//
// Returns whether a version was added.
func (m *Manager) ImportAgentArtifact(absPath string) (bool, error) {
	p, err := func() (*StudioProject, error) {
		m.mu.RLock()
		defer m.mu.RUnlock()
		p := m.activeProjectLocked()
		if p == nil {
			return nil, fmt.Errorf("no active studio project")
		}
		switch p.Type {
		case ArtifactTypeImage, ArtifactTypeVideo:
			return nil, fmt.Errorf("image/video projects import through the ComfyUI pipeline")
		}
		return p, nil
	}()
	if err != nil {
		return false, err
	}

	ext := strings.ToLower(filepath.Ext(absPath))
	src, err := os.ReadFile(absPath)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", filepath.Base(absPath), err)
	}

	data := ArtifactData{}
	if active := p.ActiveVersion(); active != nil {
		data.Prompt = active.Data.Prompt
	}
	imported := false
	switch {
	case p.Type == ArtifactTypeCodeJS && ext == ".js":
		data.JSEngine = "threejs"
		data.Code = string(src)
		imported = true
	case ext == ".html" || ext == ".htm" || ext == ".pdf" || ext == ".pptx":
		// Agent-authored page or document: the viewer opens it as-is (custom toolbar).
		data.EntryHTML = absPath
		imported = true
	case p.Type == ArtifactTypeSlides && ext == ".md":
		data.DeckTheme = "dark"
		data.RawMarkdown = string(src)
		data.Slides = ParseMarkdownDeck(string(src))
		if len(data.Slides) == 0 {
			return false, fmt.Errorf("no slides found in %s", filepath.Base(absPath))
		}
		imported = true
	default:
		return false, fmt.Errorf("no studio importer for %s files in %s projects", ext, p.Type)
	}

	p.AddVersion(fmt.Sprintf("Agent: %s", filepath.Base(absPath)), "Imported from Studio Agent turn", data, nil)

	m.mu.Lock()
	_ = m.saveProjectsLocked()
	m.mu.Unlock()
	m.notify()
	return imported, nil
}
