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

// WriteFileAtomic replaces path with data via temp-file + rename so a crash
// mid-write never truncates an existing store (sessions, stash, steer queues
// were plain os.WriteFile — a kill during the write produced a corrupt or
// empty JSON file). The existing file's mode is preserved; perm is used for
// new files. Same package-external callers (pkg/agent) use it too.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if fi, err := os.Stat(path); err == nil {
		perm = fi.Mode().Perm()
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// IsTempTestPath reports whether p points inside an OS temporary directory
// (such as go test's t.TempDir() or os.MkdirTemp), so test runs stay
// isolated from the user's global ~/.forge/sessions store.
func IsTempTestPath(p string) bool {
	clean := strings.TrimSpace(p)
	if clean == "" {
		return false
	}
	clean = filepath.Clean(clean)
	sep := string(filepath.Separator)
	prefixes := []string{"/tmp", "/private/tmp", "/var/folders", "/private/var/folders"}
	if sysTmp := strings.TrimSpace(os.TempDir()); sysTmp != "" {
		prefixes = append(prefixes, filepath.Clean(sysTmp))
	}
	for _, prefix := range prefixes {
		prefix = strings.TrimSuffix(prefix, sep)
		if prefix == "" {
			continue
		}
		if clean == prefix || strings.HasPrefix(clean, prefix+sep) {
			return true
		}
	}
	return false
}

// NormalizeWorkspacePath normalizes a workspace folder path so "general"
// aliases (~/.forge-ade/general, "general", ".", "/", empty string, or the
// bare home directory) all collapse deterministically to "".
func NormalizeWorkspacePath(folderPath string) string {
	clean := strings.TrimSpace(folderPath)
	if clean == "" || clean == "." || clean == "/" || strings.EqualFold(clean, "general") {
		return ""
	}
	clean = filepath.Clean(clean)
	if clean == "." || clean == "/" {
		return ""
	}
	if strings.HasSuffix(clean, string(filepath.Separator)+".forge-ade"+string(filepath.Separator)+"general") {
		return ""
	}
	if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
		homeClean := filepath.Clean(homeDir)
		if clean == homeClean || clean == filepath.Join(homeClean, ".forge-ade", "general") {
			return ""
		}
	}
	return clean
}

// resolveProjectDir maps a workspace path to the per-project session
// directory under ~/.forge/sessions/[project-name].
func resolveProjectDir(workspacePath, sessWorkspacePath string) (projectDir string, projectName string, targetWs string, homeDir string) {
	homeDir, _ = os.UserHomeDir()
	targetWs = NormalizeWorkspacePath(sessWorkspacePath)
	if targetWs == "" {
		targetWs = NormalizeWorkspacePath(workspacePath)
	}
	projectName = "General"
	if targetWs != "" {
		base := filepath.Base(targetWs)
		if base != "" && base != "." && base != "/" {
			projectName = base
		}
	}
	if homeDir != "" {
		projectDir = filepath.Join(homeDir, ".forge", "sessions", projectName)
	}
	return projectDir, projectName, targetWs, homeDir
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
	return WriteFileAtomic(infoPath, data, 0644)
}

type diskSessionRef struct {
	ID            string `json:"id"`
	WorkspacePath string `json:"workspacePath"`
	Folder        string `json:"folder"`
}

func extractDiskSessionRef(content []byte) (id, sessWs string, ok bool) {
	var sess diskSessionRef
	if err := json.Unmarshal(content, &sess); err != nil || sess.ID == "" {
		return "", "", false
	}
	ws := strings.TrimSpace(sess.WorkspacePath)
	if ws == "" {
		ws = strings.TrimSpace(sess.Folder)
	}
	return sess.ID, ws, true
}

// internalSessionProbe detects engine-internal sub-agent session files.
// RakitKode agent.Session dumps use a snake_case schema with
// parent_session_id / role_filter and store timestamps as created_at /
// updated_at — none of which the user-task session schema reads.
type internalSessionProbe struct {
	ParentSessionID string `json:"parent_session_id"`
	RoleFilter      string `json:"role_filter"`
}

// isInternalSubagentSessionFile reports whether content is an internal
// sub-agent session rather than a user task session. Loading them as user
// sessions pollutes the task list with "Coding Agent Sub-agent" rows whose
// timestamps derive as "now" on every launch.
func isInternalSubagentSessionFile(content []byte) bool {
	var probe internalSessionProbe
	if err := json.Unmarshal(content, &probe); err != nil {
		return false
	}
	return probe.ParentSessionID != "" || probe.RoleFilter != ""
}

func readProjectSessionInfo(projectDir string) (ProjectSessionInfo, bool) {
	var info ProjectSessionInfo
	data, err := os.ReadFile(filepath.Join(projectDir, "info.json"))
	if err != nil {
		return info, false
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return info, false
	}
	return info, true
}

// SaveSessionDisk persists an agent session JSON to ~/.forge/sessions/[project-name]/ with info.json
func SaveSessionDisk(sessionJSON string, workspacePath string) error {
	id, sessWs, ok := extractDiskSessionRef([]byte(sessionJSON))
	if !ok {
		return fmt.Errorf("invalid session JSON: missing id")
	}

	// Keep tests / temp directories completely isolated from the user's global store.
	if IsTempTestPath(sessWs) || IsTempTestPath(workspacePath) {
		targetDir := workspacePath
		if IsTempTestPath(sessWs) {
			targetDir = sessWs
		}
		if targetDir != "" {
			dir := filepath.Join(targetDir, ".forge", "sessions")
			_ = os.MkdirAll(dir, 0755)
			_ = WriteFileAtomic(filepath.Join(dir, id+".json"), []byte(sessionJSON), 0644)
		}
		return nil
	}

	projectDir, projectName, targetWs, homeDir := resolveProjectDir(workspacePath, sessWs)
	if projectDir != "" {
		_ = os.MkdirAll(projectDir, 0755)
		sessionFile := filepath.Join(projectDir, id+".json")
		_ = WriteFileAtomic(sessionFile, []byte(sessionJSON), 0644)
		_ = updateProjectSessionInfo(projectDir, projectName, targetWs)
	}

	// Legacy global backup mirror if directory already exists
	if homeDir != "" {
		legacyDir := filepath.Join(homeDir, ".forge-ade", "sessions")
		if info, err := os.Stat(legacyDir); err == nil && info.IsDir() {
			_ = WriteFileAtomic(filepath.Join(legacyDir, id+".json"), []byte(sessionJSON), 0644)
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

	readDir := func(dir string, skipTemp bool) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || e.Name() == "info.json" {
				continue
			}
			id := strings.TrimSuffix(e.Name(), ".json")
			if seen[id] {
				continue
			}
			content, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err == nil && len(content) > 0 {
				// Internal sub-agent session files never surface as user
				// sessions, regardless of which directory they sit in.
				if isInternalSubagentSessionFile(content) {
					continue
				}
				if skipTemp {
					if _, sessWs, ok := extractDiskSessionRef(content); ok && IsTempTestPath(sessWs) {
						continue
					}
				}
				seen[id] = true
				result = append(result, string(content))
			}
		}
	}

	// Test / temp workspace isolation: load only from the temp workspace itself.
	if IsTempTestPath(workspacePath) {
		readDir(filepath.Join(workspacePath, ".forge", "sessions"), false)
		readDir(filepath.Join(workspacePath, ".forge-ade", "sessions"), false)
		return result, nil
	}

	homeDir, _ := os.UserHomeDir()

	// 1. Auto-migrate any unmigrated legacy sessions from ~/.forge-ade/sessions/
	// to ~/.forge/sessions/[project-name]/ using the session's own workspace.
	if homeDir != "" {
		legacyGlobalDir := filepath.Join(homeDir, ".forge-ade", "sessions")
		if entries, err := os.ReadDir(legacyGlobalDir); err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") && e.Name() != "info.json" {
					content, rerr := os.ReadFile(filepath.Join(legacyGlobalDir, e.Name()))
					if rerr == nil && len(content) > 0 {
						if id, sessWs, ok := extractDiskSessionRef(content); ok {
							if IsTempTestPath(sessWs) || isInternalSubagentSessionFile(content) {
								continue
							}
							projDir, projName, tws, _ := resolveProjectDir("", sessWs)
							if projDir != "" {
								_ = os.MkdirAll(projDir, 0755)
								targetFile := filepath.Join(projDir, id+".json")
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

	// 2. Read all project sessions from ~/.forge/sessions/*/, cleaning up
	// polluted temp-test directories and misplaced duplicate files.
	if homeDir != "" {
		globalSessionsRoot := filepath.Join(homeDir, ".forge", "sessions")
		if entries, err := os.ReadDir(globalSessionsRoot); err == nil {
			for _, e := range entries {
				if !e.IsDir() {
					if strings.HasSuffix(e.Name(), ".json") && e.Name() != "info.json" {
						id := strings.TrimSuffix(e.Name(), ".json")
						if !seen[id] {
							content, err := os.ReadFile(filepath.Join(globalSessionsRoot, e.Name()))
							if err == nil && len(content) > 0 {
								if _, sessWs, ok := extractDiskSessionRef(content); !ok || (!IsTempTestPath(sessWs) && !isInternalSubagentSessionFile(content)) {
									seen[id] = true
									result = append(result, string(content))
								}
							}
						}
					}
					continue
				}

				dirName := e.Name()
				projDir := filepath.Join(globalSessionsRoot, dirName)
				if info, hasInfo := readProjectSessionInfo(projDir); hasInfo && IsTempTestPath(info.ProjectPath) {
					// Rescue any real non-temp sessions before purging the polluted test directory.
					for _, sub := range []string{projDir, filepath.Join(projDir, "here")} {
						subEntries, err := os.ReadDir(sub)
						if err != nil {
							continue
						}
						for _, se := range subEntries {
							if se.IsDir() || !strings.HasSuffix(se.Name(), ".json") || se.Name() == "info.json" {
								continue
							}
							content, rerr := os.ReadFile(filepath.Join(sub, se.Name()))
							if rerr != nil || len(content) == 0 {
								continue
							}
							if id, sessWs, ok := extractDiskSessionRef(content); ok && !IsTempTestPath(sessWs) && !isInternalSubagentSessionFile(content) {
								tDir, tName, tws, _ := resolveProjectDir("", sessWs)
								if tDir != "" {
									_ = os.MkdirAll(tDir, 0755)
									tFile := filepath.Join(tDir, id+".json")
									if _, serr := os.Stat(tFile); os.IsNotExist(serr) {
										_ = os.WriteFile(tFile, content, 0644)
										_ = updateProjectSessionInfo(tDir, tName, tws)
									}
									if !seen[id] {
										seen[id] = true
										result = append(result, string(content))
									}
								}
							}
						}
					}
					_ = os.RemoveAll(projDir)
					continue
				}

				dirModified := false
				for _, sub := range []string{projDir, filepath.Join(projDir, "here")} {
					subEntries, err := os.ReadDir(sub)
					if err != nil {
						continue
					}
					for _, se := range subEntries {
						if se.IsDir() || !strings.HasSuffix(se.Name(), ".json") || se.Name() == "info.json" {
							continue
						}
						filePath := filepath.Join(sub, se.Name())
						content, rerr := os.ReadFile(filePath)
						if rerr != nil || len(content) == 0 {
							continue
						}
						id := strings.TrimSuffix(se.Name(), ".json")
						if parsedID, sessWs, ok := extractDiskSessionRef(content); ok {
							id = parsedID
							if IsTempTestPath(sessWs) {
								_ = os.Remove(filePath)
								dirModified = true
								continue
							}
							if isInternalSubagentSessionFile(content) {
								_ = os.Remove(filePath)
								dirModified = true
								continue
							}
							trueDir, trueName, tws, _ := resolveProjectDir("", sessWs)
							if trueName != "" && trueName != dirName && trueDir != "" {
								_ = os.MkdirAll(trueDir, 0755)
								targetFile := filepath.Join(trueDir, id+".json")
								if _, serr := os.Stat(targetFile); os.IsNotExist(serr) {
									_ = os.WriteFile(targetFile, content, 0644)
									_ = updateProjectSessionInfo(trueDir, trueName, tws)
								}
								_ = os.Remove(filePath)
								dirModified = true
							}
						}
						if !seen[id] {
							seen[id] = true
							result = append(result, string(content))
						}
					}
				}
				if dirModified {
					_ = updateProjectSessionInfo(projDir, dirName, "")
				}
			}
		}
	}

	// 3. Read workspace-specific sessions
	if wsClean := NormalizeWorkspacePath(workspacePath); wsClean != "" {
		readDir(filepath.Join(wsClean, ".forge", "sessions"), true)
		readDir(filepath.Join(wsClean, ".forge-ade", "sessions"), true)
	}

	// 4. Fallback check legacy global sessions (~/.forge-ade/sessions/)
	if homeDir != "" {
		readDir(filepath.Join(homeDir, ".forge-ade", "sessions"), true)
	}

	return result, nil
}

// DeleteSessionDisk removes session JSON from ~/.forge/sessions/[project-name]/, workspace, and legacy paths
func DeleteSessionDisk(sessionID string, workspacePath string) error {
	if sessionID == "" {
		return nil
	}

	targetFileName := sessionID + ".json"

	if IsTempTestPath(workspacePath) {
		_ = os.Remove(filepath.Join(workspacePath, ".forge", "sessions", targetFileName))
		_ = os.Remove(filepath.Join(workspacePath, ".forge-ade", "sessions", targetFileName))
		return nil
	}

	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		globalSessionsRoot := filepath.Join(homeDir, ".forge", "sessions")
		_ = os.Remove(filepath.Join(globalSessionsRoot, targetFileName))

		// Walk through all project folders in ~/.forge/sessions/
		if entries, err := os.ReadDir(globalSessionsRoot); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					projDir := filepath.Join(globalSessionsRoot, e.Name())
					removed := os.Remove(filepath.Join(projDir, targetFileName)) == nil
					if os.Remove(filepath.Join(projDir, "here", targetFileName)) == nil {
						removed = true
					}
					if removed {
						_ = updateProjectSessionInfo(projDir, e.Name(), "")
					}
				}
			}
		}

		_ = os.Remove(filepath.Join(homeDir, ".forge-ade", "sessions", targetFileName))
	}

	if wsClean := NormalizeWorkspacePath(workspacePath); wsClean != "" {
		_ = os.Remove(filepath.Join(wsClean, ".forge", "sessions", targetFileName))
		_ = os.Remove(filepath.Join(wsClean, ".forge-ade", "sessions", targetFileName))
	}

	return nil
}

// DeleteProjectSessionsDisk removes persisted session storage for the named
// non-General project directory under ~/.forge/sessions/<projectName> (and any
// matching legacy sessions), leaving workspace source files untouched.
func DeleteProjectSessionsDisk(projectName, workspacePath string) error {
	if IsTempTestPath(workspacePath) {
		_ = os.RemoveAll(filepath.Join(workspacePath, ".forge", "sessions"))
		_ = os.RemoveAll(filepath.Join(workspacePath, ".forge-ade", "sessions"))
		return nil
	}

	if projectName == "" || strings.EqualFold(projectName, "General") {
		return nil
	}

	homeDir, _ := os.UserHomeDir()
	if homeDir == "" {
		return nil
	}

	_ = os.RemoveAll(filepath.Join(homeDir, ".forge", "sessions", projectName))

	normWs := NormalizeWorkspacePath(workspacePath)
	globalSessionsRoot := filepath.Join(homeDir, ".forge", "sessions")
	if entries, err := os.ReadDir(globalSessionsRoot); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			projDir := filepath.Join(globalSessionsRoot, e.Name())
			if info, ok := readProjectSessionInfo(projDir); ok && normWs != "" && NormalizeWorkspacePath(info.ProjectPath) == normWs {
				_ = os.RemoveAll(projDir)
				continue
			}
			if normWs == "" {
				continue
			}
			for _, sub := range []string{projDir, filepath.Join(projDir, "here")} {
				subEntries, serr := os.ReadDir(sub)
				if serr != nil {
					continue
				}
				for _, se := range subEntries {
					if se.IsDir() || !strings.HasSuffix(se.Name(), ".json") || se.Name() == "info.json" {
						continue
					}
					fp := filepath.Join(sub, se.Name())
					content, rerr := os.ReadFile(fp)
					if rerr != nil {
						continue
					}
					if _, sessWs, ok := extractDiskSessionRef(content); ok && NormalizeWorkspacePath(sessWs) == normWs {
						_ = os.Remove(fp)
					}
				}
			}
		}
	}

	// Guard: if the workspace IS the home directory (or an ancestor of it),
	// the paths below ARE the global session stores — removing them would
	// wipe every project's sessions, not just this workspace's.
	if normWs != "" && normWs != NormalizeWorkspacePath(homeDir) &&
		!strings.HasPrefix(NormalizeWorkspacePath(homeDir)+string(filepath.Separator), normWs+string(filepath.Separator)) &&
		!strings.HasPrefix(NormalizeWorkspacePath(homeDir), normWs) {
		_ = os.RemoveAll(filepath.Join(normWs, ".forge", "sessions"))
		_ = os.RemoveAll(filepath.Join(normWs, ".forge-ade", "sessions"))
	}

	// Also prune matching sessions in legacy ~/.forge-ade/sessions/ so they do
	// not re-migrate back on next launch.
	legacyDir := filepath.Join(homeDir, ".forge-ade", "sessions")
	if entries, err := os.ReadDir(legacyDir); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || e.Name() == "info.json" {
				continue
			}
			fp := filepath.Join(legacyDir, e.Name())
			content, err := os.ReadFile(fp)
			if err != nil {
				continue
			}
			_, sessWs, ok := extractDiskSessionRef(content)
			if !ok {
				continue
			}
			_, trueName, tws, _ := resolveProjectDir("", sessWs)
			if trueName == projectName || (normWs != "" && tws == normWs) {
				_ = os.Remove(fp)
			}
		}
	}

	return nil
}
