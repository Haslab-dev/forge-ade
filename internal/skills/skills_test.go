package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSkillsLifecycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "forge-skills-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mgr := NewManager()
	mgr.SetWorkspace(tempDir)

	// 1. Create a skill
	req := CreateSkillRequest{
		Name:        "test-playbook",
		Description: "A test playbook for verification",
		Body:        "Step 1: check test.\nStep 2: pass test.",
		Scope:       "workspace",
		Scripts: map[string]string{
			"run.sh": "echo 'running test script'",
		},
	}

	created, err := mgr.CreateSkill(req, tempDir)
	if err != nil {
		t.Fatalf("failed to create skill: %v", err)
	}
	if created.Name != "test-playbook" {
		t.Fatalf("expected name 'test-playbook', got %s", created.Name)
	}

	// 2. Fetch skill
	s, ok := mgr.Get("test-playbook")
	if !ok {
		t.Fatalf("expected to find created skill")
	}
	if len(s.Scripts) == 0 {
		t.Fatalf("expected scripts to be registered")
	}

	// 3. Catalog check
	cat := mgr.CatalogMarkdown()
	if cat == "" {
		t.Fatalf("expected non-empty catalog markdown")
	}

	// 4. Flat file discovery
	flatPath := filepath.Join(tempDir, ".forge", "skills", "flat-skill.md")
	flatContent := "---\nname: flat-skill\ndescription: Flat markdown skill\n---\nFlat body content"
	_ = os.WriteFile(flatPath, []byte(flatContent), 0644)
	mgr.Reload()

	flat, ok := mgr.Get("flat-skill")
	if !ok || flat.Description != "Flat markdown skill" {
		t.Fatalf("expected flat skill to be discovered")
	}

	// 5. Delete skill
	if err := mgr.DeleteSkill("test-playbook"); err != nil {
		t.Fatalf("failed to delete skill: %v", err)
	}
	if _, ok := mgr.Get("test-playbook"); ok {
		t.Fatalf("expected skill to be deleted")
	}
}
