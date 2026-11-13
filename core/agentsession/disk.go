package agentsession

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ProjectSessionInfo is the metadata record (info.json) kept alongside a
// project's sessions directory.
type ProjectSessionInfo struct {
	WorkspaceName string `json:"workspaceName"`
	ProjectPath   string `json:"projectPath"`
	ProjectName   string `json:"projectName"`
	CreatedAt     string `json:"createdAt,omitempty"`
	UpdatedAt     string `json:"updatedAt"`
	SessionCount  int    `json:"sessionCount"`
}

// resolveProjectDir maps a workspace path to the per-project session
// directory under ~/.forge/sessions/[project-name].
func resolveProjectDir(workspacePath, sessWorkspacePath string) (projectDir string, projectName string, targetWs string, homeDir string) {
	homeDir, _ = os.UserHomeDir()
	targetWs = strings.TrimSpace(sessWorkspacePath)
	if targetWs == "" {
		targetWs = strings.TrimSpace(workspacePath)
	}
	targetWsClean := filepath.Clean(targetWs)
	projectName = "General"
	if targetWsClean != "" && targetWsClean != "." && targetWsClean != "/" {
		base := filepath.Base(targetWsClean)
		if base != "" && base != "." && base != "/" {
			projectName = base
		}
	}
	if homeDir != "" {
		projectDir = filepath.Join(homeDir, ".forge", "sessions", projectName)
	}
	return projectDir, projectName, targetWsClean, homeDir
}

func countSessionsInProjectDir(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") && e.Name() != "info.json" {
			count++
		} else if e.IsDir() && e.Name() == "here" {
			subEntries, _ := os.ReadDir(filepath.Join(dir, "here"))
			for _, se := range subEntries {
				if !se.IsDir() && strings.HasSuffix(se.Name(), ".json") && se.Name() != "info.json" {
					count++
				}
			}
		}
	}
	return count
}

func updateProjectSessionInfo(projectDir, projectName, projectPath string) error {
	if projectDir == "" {
		return nil
	}
	infoPath := filepath.Join(projectDir, "info.json")
	var info ProjectSessionInfo

	if data, err := os.ReadFile(infoPath); err == nil {
		_ = json.Unmarshal(data, &info)
	}

	now := time.Now().Format(time.RFC3339)
	if info.CreatedAt == "" {
		info.CreatedAt = now
	}
	info.UpdatedAt = now

	if projectName != "" {
		info.ProjectName = projectName
		if info.WorkspaceName == "" {
			info.WorkspaceName = projectName
		}
	}
	if projectPath != "" && projectPath != "." && projectPath != "/" {
		info.ProjectPath = projectPath
	}

	info.SessionCount = countSessionsInProjectDir(projectDir)

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(infoPath, data, 0644)
}

// SaveSessionDisk persists an agent session JSON to ~/.forge/sessions/[project-name]/ with info.json
func SaveSessionDisk(sessionJSON string, workspacePath string) error {
	var sess struct {
		ID            string `json:"id"`
		WorkspacePath string `json:"workspacePath"`
	}
	if err := json.Unmarshal([]byte(sessionJSON), &sess); err != nil || sess.ID == "" {
		return fmt.Errorf("invalid session JSON: missing id")
	}

	projectDir, projectName, targetWs, homeDir := resolveProjectDir(workspacePath, sess.WorkspacePath)
	if projectDir != "" {
		_ = os.MkdirAll(projectDir, 0755)
		sessionFile := filepath.Join(projectDir, sess.ID+".json")
		_ = os.WriteFile(sessionFile, []byte(sessionJSON), 0644)
		_ = updateProjectSessionInfo(projectDir, projectName, targetWs)
	}

	// Legacy global backup mirror if directory already exists
	if homeDir != "" {
		legacyDir := filepath.Join(homeDir, ".forge-ade", "sessions")
		if info, err := os.Stat(legacyDir); err == nil && info.IsDir() {
			_ = os.WriteFile(filepath.Join(legacyDir, sess.ID+".json"), []byte(sessionJSON), 0644)
		}
	}

	// NOTE: no workspace mirror — sessions live only in the global store so
	// no .forge/ directory is created inside user workspaces. Legacy
	// workspace copies are still read (and removed) for compatibility.

	return nil
}

// LoadSessionsDisk retrieves all agent sessions from global ~/.forge/sessions/[project-name]/,
// workspace, and auto-migrates legacy ~/.forge-ade/sessions/
func LoadSessionsDisk(workspacePath string) ([]string, error) {
	seen := make(map[string]bool)
	var result []string

	homeDir, _ := os.UserHomeDir()

	// 1. Auto-migrate any unmigrated legacy sessions from ~/.forge-ade/sessions/ to ~/.forge/sessions/[project-name]/
	if homeDir != "" {
		legacyGlobalDir := filepath.Join(homeDir, ".forge-ade", "sessions")
		if entries, err := os.ReadDir(legacyGlobalDir); err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") && e.Name() != "info.json" {
					content, rerr := os.ReadFile(filepath.Join(legacyGlobalDir, e.Name()))
					if rerr == nil && len(content) > 0 {
						var sess struct {
							ID            string `json:"id"`
							WorkspacePath string `json:"workspacePath"`
						}
						if json.Unmarshal(content, &sess) == nil && sess.ID != "" {
							projDir, projName, tws, _ := resolveProjectDir(workspacePath, sess.WorkspacePath)
							if projDir != "" {
								_ = os.MkdirAll(projDir, 0755)
								targetFile := filepath.Join(projDir, e.Name())
								if _, serr := os.Stat(targetFile); os.IsNotExist(serr) {
									_ = os.WriteFile(targetFile, content, 0644)
									_ = updateProjectSessionInfo(projDir, projName, tws)
								}
							}
						}
					}
				}
			}
		}
	}

	readDir := func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") && e.Name() != "info.json" {
				id := strings.TrimSuffix(e.Name(), ".json")
				if seen[id] {
					continue
				}
				content, err := os.ReadFile(filepath.Join(dir, e.Name()))
				if err == nil && len(content) > 0 {
					seen[id] = true
					result = append(result, string(content))
				}
			}
		}
	}

	// 2. Read all project sessions from ~/.forge/sessions/*/
	if homeDir != "" {
		globalSessionsRoot := filepath.Join(homeDir, ".forge", "sessions")
		if entries, err := os.ReadDir(globalSessionsRoot); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					projDir := filepath.Join(globalSessionsRoot, e.Name())
					readDir(projDir)
					// Also check any subfolder 'here' inside project dir
					readDir(filepath.Join(projDir, "here"))
				} else if strings.HasSuffix(e.Name(), ".json") && e.Name() != "info.json" {
					// In case a loose session file exists directly in root
					id := strings.TrimSuffix(e.Name(), ".json")
					if !seen[id] {
						content, err := os.ReadFile(filepath.Join(globalSessionsRoot, e.Name()))
						if err == nil && len(content) > 0 {
							seen[id] = true
							result = append(result, string(content))
						}
					}
				}
			}
		}
	}

	// 3. Read workspace-specific sessions
	if workspacePath != "" && workspacePath != "/" {
		readDir(filepath.Join(workspacePath, ".forge", "sessions"))
		readDir(filepath.Join(workspacePath, ".forge-ade", "sessions"))
	}

	// 4. Fallback check legacy global sessions (~/.forge-ade/sessions/)
	if homeDir != "" {
		readDir(filepath.Join(homeDir, ".forge-ade", "sessions"))
	}

	return result, nil
}

// DeleteSessionDisk removes session JSON from ~/.forge/sessions/[project-name]/, workspace, and legacy paths
func DeleteSessionDisk(sessionID string, workspacePath string) error {
	if sessionID == "" {
		return nil
	}

	targetFileName := sessionID + ".json"

	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		globalSessionsRoot := filepath.Join(homeDir, ".forge", "sessions")
		_ = os.Remove(filepath.Join(globalSessionsRoot, targetFileName))

		// Walk through all project folders in ~/.forge/sessions/
		if entries, err := os.ReadDir(globalSessionsRoot); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					projDir := filepath.Join(globalSessionsRoot, e.Name())
					_ = os.Remove(filepath.Join(projDir, targetFileName))
					_ = os.Remove(filepath.Join(projDir, "here", targetFileName))
					_ = updateProjectSessionInfo(projDir, e.Name(), "")
				}
			}
		}

		_ = os.Remove(filepath.Join(homeDir, ".forge-ade", "sessions", targetFileName))
	}

	if workspacePath != "" && workspacePath != "/" {
		_ = os.Remove(filepath.Join(workspacePath, ".forge", "sessions", targetFileName))
		_ = os.Remove(filepath.Join(workspacePath, ".forge-ade", "sessions", targetFileName))
	}

	return nil
}
