package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/hasdev/forge-ade/internal/events"
)

// Repository wraps a git repo with its path and state.
type Repository struct {
	Path   string `json:"path"`
	Name   string `json:"name"`
	repo   *git.Repository `json:"-"`
	mu     sync.RWMutex    `json:"-"`
}

// Branch contains branch information.
type Branch struct {
	Name       string `json:"name"`
	IsHead     bool   `json:"isHead"`
	IsRemote   bool   `json:"isRemote"`
	IsActive   bool   `json:"isActive"`
	CommitHash string `json:"commitHash"`
}

// Commit represents a single commit.
type Commit struct {
	Hash      string `json:"hash"`
	Author    string `json:"author"`
	Email     string `json:"email"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
	Parents   int    `json:"parents"`
}

// StatusEntry represents a single file status entry.
type StatusEntry struct {
	Path     string `json:"path"`
	Staging  string `json:"staging"`  // ' ', 'M', 'A', 'D', 'R', 'C'
	Worktree string `json:"worktree"` // ' ', 'M', 'A', 'D', etc.
}

// Manager manages multiple git repositories.
type Manager struct {
	bus          *events.Bus
	mu           sync.RWMutex
	repositories map[string]*Repository
}

// NewManager creates a new git manager.
func NewManager(bus *events.Bus) *Manager {
	return &Manager{
		bus:          bus,
		repositories: make(map[string]*Repository),
	}
}

// Discover scans a list of directories for git repositories.
func (m *Manager) Discover(dirs []string) {
	for _, dir := range dirs {
		repoPath := m.findRepoRoot(dir)
		if repoPath != "" {
			m.addRepo(repoPath)
		}
	}
}

// AddRepo adds a repository by path.
func (m *Manager) AddRepo(path string) error {
	return m.addRepo(path)
}

func (m *Manager) addRepo(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	// Check if already tracked
	m.mu.RLock()
	_, exists := m.repositories[absPath]
	m.mu.RUnlock()
	if exists {
		return nil
	}

	repo, err := git.PlainOpen(absPath)
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.repositories[absPath] = &Repository{
		Path: absPath,
		Name: filepath.Base(absPath),
		repo: repo,
	}
	m.mu.Unlock()

	m.bus.Publish(events.Event{
		Type: events.GitUpdated,
		Data: map[string]interface{}{
			"path": absPath,
		},
	})

	return nil
}

// RemoveRepo removes a repository from tracking.
func (m *Manager) RemoveRepo(path string) {
	m.mu.Lock()
	delete(m.repositories, path)
	m.mu.Unlock()
}

// ListRepos returns all tracked repositories.
func (m *Manager) ListRepos() []*Repository {
	m.mu.RLock()
	defer m.mu.RUnlock()

	repos := make([]*Repository, 0, len(m.repositories))
	for _, r := range m.repositories {
		repos = append(repos, r)
	}
	return repos
}

// GetStatus returns the working tree status for a repository.
func (r *Repository) GetStatus() ([]StatusEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tree, err := r.repo.Worktree()
	if err != nil {
		return nil, err
	}

	status, err := tree.Status()
	if err != nil {
		return nil, err
	}

	var entries []StatusEntry
	for path, s := range status {
		entries = append(entries, StatusEntry{
			Path:     path,
			Staging:  string(s.Staging),
			Worktree: string(s.Worktree),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})

	return entries, nil
}

// GetBranches returns all branches for a repository.
func (r *Repository) GetBranches() ([]Branch, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	branches, err := r.repo.Branches()
	if err != nil {
		return nil, err
	}

	head, err := r.repo.Head()
	if err != nil {
		return nil, err
	}

	var result []Branch
	_ = branches.ForEach(func(ref *plumbing.Reference) error {
		b := Branch{
			Name:       ref.Name().Short(),
			IsHead:     ref.Hash() == head.Hash(),
			CommitHash: ref.Hash().String(),
		}
		if b.Name == head.Name().Short() {
			b.IsActive = true
		}
		result = append(result, b)
		return nil
	})

	return result, nil
}

// GetCommits returns commit history for a repository.
func (r *Repository) GetCommits(count int) ([]Commit, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	head, err := r.repo.Head()
	if err != nil {
		return nil, err
	}

	iter, err := r.repo.Log(&git.LogOptions{
		From:  head.Hash(),
		Order: git.LogOrderCommitterTime,
	})
	if err != nil {
		return nil, err
	}

	var commits []Commit
	_ = iter.ForEach(func(c *object.Commit) error {
		if count > 0 && len(commits) >= count {
			return fmt.Errorf("enough")
		}
		commits = append(commits, Commit{
			Hash:      c.Hash.String(),
			Author:    c.Author.Name,
			Email:     c.Author.Email,
			Message:   strings.Split(c.Message, "\n")[0],
			Timestamp: c.Author.When.Unix(),
			Parents:   len(c.ParentHashes),
		})
		return nil
	})

	return commits, nil
}

// Stage stages files in the repository.
func (r *Repository) Stage(paths ...string) error {
	tree, err := r.repo.Worktree()
	if err != nil {
		return err
	}

	for _, path := range paths {
		_, err := tree.Add(path)
		if err != nil {
			return err
		}
	}
	return nil
}

// Unstage unstages files in the repository.
func (r *Repository) Unstage(paths ...string) error {
	tree, err := r.repo.Worktree()
	if err != nil {
		return err
	}

	for _, path := range paths {
		_, err := tree.Remove(path)
		if err != nil && !strings.Contains(err.Error(), "file not found") {
			return err
		}
	}
	return nil
}

// Commit creates a commit with the given message.
func (r *Repository) Commit(message string) error {
	tree, err := r.repo.Worktree()
	if err != nil {
		return err
	}

	_, err = tree.Commit(message, &git.CommitOptions{})
	if err != nil {
		return err
	}

	return nil
}

// StageAll stages all changes in the working tree.
func (r *Repository) StageAll() error {
	tree, err := r.repo.Worktree()
	if err != nil {
		return err
	}

	status, err := tree.Status()
	if err != nil {
		return err
	}

	for path := range status {
		if _, err := tree.Add(path); err != nil {
			return err
		}
	}
	return nil
}

// RunGitCommand runs a raw git command in the repository directory.
func (r *Repository) RunGitCommand(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), string(output), err)
	}
	return string(output), nil
}

func (m *Manager) findRepoRoot(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}

	// Check if dir itself is a repo
	if _, err := git.PlainOpen(abs); err == nil {
		return abs
	}

	// Walk up to find .git
	current := abs
	for {
		gitDir := filepath.Join(current, ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}
