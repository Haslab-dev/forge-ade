package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
)

// ---------------------------------------------------------------------------
// File Operations (Read, Write, Create, Delete)
// ---------------------------------------------------------------------------

// GetHomeDir returns the user's home directory.
func (a *App) GetHomeDir() string {
	home, _ := os.UserHomeDir()
	return home
}

// OpenInFinder opens a path in macOS Finder (selects file or opens directory).
func (a *App) OpenInFinder(path string) error {
	cmd := exec.Command("open", "-R", path)
	return cmd.Run()
}

// BrowserOpenURL opens a URL in the system default browser.
func (a *App) BrowserOpenURL(url string) error {
	if strings.TrimSpace(url) == "" {
		return fmt.Errorf("url cannot be empty")
	}
	cmd := exec.Command("open", url)
	return cmd.Run()
}

// IsDir checks if a path is a directory.
func (a *App) IsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// ResolvePath expands ~ and relative paths to an absolute, cleaned path.
// Returns the input unchanged if it cannot be resolved.
func (a *App) ResolvePath(path string) string {
	resolved, err := ResolvePath(path)
	if err != nil {
		return path
	}
	return resolved
}

// resolveWorkspacePath resolves relative paths against the current workspace
// or explorer roots so file operations work seamlessly from any location.
func (a *App) resolveWorkspacePath(path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	if ws := a.workspaceMgr.Current(); ws != nil && len(ws.GetFolders()) > 0 {
		return filepath.Join(ws.GetFolders()[0], path)
	}
	if len(a.explorer.GetRoots()) > 0 {
		return filepath.Join(a.explorer.GetRoots()[0], path)
	}
	cwd, err := os.Getwd()
	if err == nil {
		return filepath.Join(cwd, path)
	}
	return path
}

// ReadFile reads and returns a file's content as a string.
func (a *App) ReadFile(path string) (string, error) {
	if data, isRemote, err := a.remoteFS.ReadFile(path); isRemote {
		if err != nil {
			return "", fmt.Errorf("read remote file: %w", err)
		}
		return string(data), nil
	}
	path = a.resolveWorkspacePath(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	return string(data), nil
}

// ReadFileBase64 reads a binary file and returns base64-encoded content.
func (a *App) ReadFileBase64(path string) (string, error) {
	if data, isRemote, err := a.remoteFS.ReadFile(path); isRemote {
		if err != nil {
			return "", fmt.Errorf("read remote file base64: %w", err)
		}
		return base64.StdEncoding.EncodeToString(data), nil
	}
	path = a.resolveWorkspacePath(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file base64: %w", err)
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// WriteFile writes content to a file, creating it if needed.
func (a *App) WriteFile(path string, content string) error {
	if isRemote, err := a.remoteFS.WriteFile(path, []byte(content)); isRemote {
		if err != nil {
			return fmt.Errorf("write remote file: %w", err)
		}
		return nil
	}
	path = a.resolveWorkspacePath(path)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

// CreateFile creates a new file (with content if the caller supplies any via
// WriteFile afterwards), creating parent directories as needed.
func (a *App) CreateFile(path string) error {
	if isRemote, err := a.remoteFS.CreateFile(path); isRemote {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create parent directories: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	f.Close()
	return nil
}

// CreateFolder creates a new directory (and parents if needed).
func (a *App) CreateFolder(path string) error {
	if isRemote, err := a.remoteFS.CreateFolder(path); isRemote {
		return err
	}
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("create folder: %w", err)
	}
	return nil
}

// DeleteFile removes a file or directory (recursively for directories).
func (a *App) DeleteFile(path string) error {
	if isRemote, err := a.remoteFS.DeleteFile(path); isRemote {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	if info.IsDir() {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("delete directory: %w", err)
		}
	} else {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("delete file: %w", err)
		}
	}
	return nil
}

// RenameFile renames or moves a file.
func (a *App) RenameFile(oldPath, newPath string) error {
	if isRemote, err := a.remoteFS.RenameFile(oldPath, newPath); isRemote {
		return err
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("rename file: %w", err)
	}
	return nil
}

// CopyFile copies a file from src to dst.
func (a *App) CopyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("copy file read: %w", err)
	}
	if err := os.WriteFile(dst, input, 0644); err != nil {
		return fmt.Errorf("copy file write: %w", err)
	}
	return nil
}

// MoveFile moves a file from src to dst (alias for RenameFile).
func (a *App) MoveFile(src, dst string) error {
	return os.Rename(src, dst)
}

// GetClipboardFiles returns the absolute path of a single file/folder in the
// system clipboard (e.g. copied from Finder). Empty slice when the clipboard
// holds text, nothing, or multiple files (macOS only).
func (a *App) GetClipboardFiles() []string {
	if goruntime.GOOS != "darwin" {
		return nil
	}
	script := "try\nset u to the clipboard as «class furl»\nreturn POSIX path of u\non error\nreturn \"\"\nend try"
	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	p := strings.TrimSpace(string(out))
	// Guard: furl coercion is loose — text like "a: b" can turn into a
	// bogus HFS-style path. Only accept real, existing absolute paths.
	if p == "" || !filepath.IsAbs(p) {
		return nil
	}
	if _, err := os.Stat(p); err != nil {
		return nil
	}
	return []string{p}
}

// CopyPath copies a file or folder (recursively) from src to dst. dst is the
// full destination path (file for file, dir for folder).
func (a *App) CopyPath(src, dst string) error {
	if isRemote, err := a.remoteFS.CopyPath(src, dst); isRemote {
		return err
	}
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("copy stat: %w", err)
	}
	if !info.IsDir() {
		return a.CopyFile(src, dst)
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("copy mkdir: %w", err)
	}
	return filepath.Walk(src, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if fi.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
}
