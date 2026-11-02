package git

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// CheckpointRecord represents a single point-in-time snapshot of the workspace.
type CheckpointRecord struct {
	ID          string   `json:"id"`
	SessionID   string   `json:"sessionId"`
	TurnID      string   `json:"turnId"`
	Description string   `json:"description"`
	CreatedAt   int64    `json:"createdAt"`
	CommitHash  string   `json:"commitHash,omitempty"`
	Files       []string `json:"files,omitempty"`
}

var (
	checkpointMu sync.Mutex
)

// IsGitRepo reports whether the given directory is inside a git repository.
func IsGitRepo(ctx context.Context, repoPath string) bool {
	gitDir := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		return true
	}
	cmd := gitCmd(ctx, repoPath, "rev-parse", "--git-dir")
	return cmd.Run() == nil
}

// checkpointsMetaPath returns the path to the checkpoints JSON index for a workspace.
func checkpointsMetaPath(repoPath string) string {
	return filepath.Join(repoPath, ".forge", "checkpoints.json")
}

func readCheckpointsMeta(repoPath string) ([]CheckpointRecord, error) {
	p := checkpointsMetaPath(repoPath)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var records []CheckpointRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	return records, nil
}

func writeCheckpointsMeta(repoPath string, records []CheckpointRecord) error {
	dir := filepath.Join(repoPath, ".forge")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(checkpointsMetaPath(repoPath), data, 0644)
}

// CreateCheckpoint snapshots the current state of repoPath for a given session and turn.
// It uses git stash create / update-ref if git is available, with file-level fallback.
func CreateCheckpoint(ctx context.Context, repoPath string, sessionID string, turnID string, description string) (*CheckpointRecord, error) {
	checkpointMu.Lock()
	defer checkpointMu.Unlock()

	if repoPath == "" {
		return nil, fmt.Errorf("repoPath cannot be empty")
	}
	if sessionID == "" {
		sessionID = "general"
	}
	if turnID == "" {
		turnID = fmt.Sprintf("turn_%d", time.Now().UnixMilli())
	}
	id := fmt.Sprintf("chk_%d_%s", time.Now().UnixMilli(), turnID)

	rec := CheckpointRecord{
		ID:          id,
		SessionID:   sessionID,
		TurnID:      turnID,
		Description: description,
		CreatedAt:   time.Now().UnixMilli(),
	}

	// Try Git-based snapshot
	isGit := IsGitRepo(ctx, repoPath)
	if isGit {
		// git stash create creates a commit object representing current state without modifying HEAD/index
		stashHashOut, err := gitCmd(ctx, repoPath, "stash", "create").Output()
		hash := strings.TrimSpace(string(stashHashOut))
		if err == nil && hash != "" {
			rec.CommitHash = hash
		} else {
			// If working directory is completely clean, HEAD is the current state
			headOut, headErr := gitCmd(ctx, repoPath, "rev-parse", "HEAD").Output()
			if headErr == nil {
				rec.CommitHash = strings.TrimSpace(string(headOut))
			}
		}

		if rec.CommitHash != "" {
			refName := fmt.Sprintf("refs/forge/checkpoints/%s/%s", sessionID, turnID)
			_ = gitCmd(ctx, repoPath, "update-ref", refName, rec.CommitHash).Run()

			// List modified/tracked files
			diffOut, _ := gitCmd(ctx, repoPath, "diff", "--name-only", "HEAD").Output()
			lines := strings.Split(strings.TrimSpace(string(diffOut)), "\n")
			for _, l := range lines {
				if l = strings.TrimSpace(l); l != "" {
					rec.Files = append(rec.Files, l)
				}
			}
		}
	}

	// If not a git repo or git commit creation didn't yield a hash, create snapshot directory
	if rec.CommitHash == "" {
		backupDir := filepath.Join(repoPath, ".forge", "snapshots", sessionID, turnID)
		_ = os.MkdirAll(backupDir, 0755)
		rec.CommitHash = "local_snapshot"
	}

	// Save to metadata index
	records, _ := readCheckpointsMeta(repoPath)
	records = append(records, rec)
	_ = writeCheckpointsMeta(repoPath, records)

	return &rec, nil
}

// RestoreCheckpoint restores the workspace files to the state captured in the checkpoint.
func RestoreCheckpoint(ctx context.Context, repoPath string, sessionID string, turnID string) error {
	checkpointMu.Lock()
	defer checkpointMu.Unlock()

	records, err := readCheckpointsMeta(repoPath)
	if err != nil {
		return fmt.Errorf("read checkpoints: %w", err)
	}

	var target *CheckpointRecord
	for i := len(records) - 1; i >= 0; i-- {
		if records[i].SessionID == sessionID && records[i].TurnID == turnID {
			target = &records[i]
			break
		}
	}

	if target == nil {
		return fmt.Errorf("checkpoint not found for session %q turn %q", sessionID, turnID)
	}

	if target.CommitHash != "" && target.CommitHash != "local_snapshot" && IsGitRepo(ctx, repoPath) {
		// Restore working tree files from the saved commit/stash hash
		out, err := gitCmd(ctx, repoPath, "checkout", target.CommitHash, "--", ".").CombinedOutput()
		if err != nil {
			// Fallback: try restore command
			out, err = gitCmd(ctx, repoPath, "restore", "--source", target.CommitHash, "--worktree", ".").CombinedOutput()
			if err != nil {
				return fmt.Errorf("git checkout checkpoint failed: %s (%w)", string(out), err)
			}
		}
		return nil
	}

	// Local snapshot fallback
	backupDir := filepath.Join(repoPath, ".forge", "snapshots", sessionID, turnID)
	if info, err := os.Stat(backupDir); err == nil && info.IsDir() {
		return copyDir(backupDir, repoPath)
	}

	return fmt.Errorf("unable to restore checkpoint: no valid git ref or snapshot found")
}

// ListCheckpoints returns all checkpoints for a workspace and optionally filtered by sessionID.
func ListCheckpoints(repoPath string, sessionID string) ([]CheckpointRecord, error) {
	checkpointMu.Lock()
	defer checkpointMu.Unlock()

	records, err := readCheckpointsMeta(repoPath)
	if err != nil {
		return nil, err
	}
	if sessionID == "" {
		return records, nil
	}
	var filtered []CheckpointRecord
	for _, r := range records {
		if r.SessionID == sessionID {
			filtered = append(filtered, r)
		}
	}
	return filtered, nil
}

func copyDir(src string, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
