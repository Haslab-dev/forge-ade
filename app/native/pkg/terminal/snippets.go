package terminal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Snippet represents a saved terminal command snippet.
type Snippet struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Command     string    `json:"command"`
	Category    string    `json:"category"` // "remote", "docker", "git", "system", "dev", "custom"
	Description string    `json:"description,omitempty"`
	IsBuiltin   bool      `json:"isBuiltin,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// HistoryItem represents an executed terminal command history entry.
type HistoryItem struct {
	ID         string    `json:"id"`
	Command    string    `json:"command"`
	Timestamp  time.Time `json:"timestamp"`
	IsFavorite bool      `json:"isFavorite"`
	Source     string    `json:"source,omitempty"` // "manual", "snippet", "remote"
}

// BuiltinSnippets returns the curated catalog of default snippets (1:1 with webview terminalSnippetStore).
var BuiltinSnippets = []Snippet{
	// Remote & SSH
	{
		ID:          "builtin-remote-service-status",
		Title:       "Check Service Status",
		Command:     "systemctl status <service>",
		Category:    "remote",
		Description: "Check active/running status of a systemd unit on remote server",
		IsBuiltin:   true,
	},
	{
		ID:          "builtin-remote-service-logs",
		Title:       "Tail Service Logs",
		Command:     "journalctl -u <service> -n 100 -f",
		Category:    "remote",
		Description: "Follow live stdout/stderr logs from a systemd service",
		IsBuiltin:   true,
	},
	{
		ID:          "builtin-remote-tail-syslog",
		Title:       "Tail Syslog",
		Command:     "tail -f /var/log/syslog",
		Category:    "remote",
		Description: "Follow general system event log stream",
		IsBuiltin:   true,
	},
	{
		ID:          "builtin-remote-tail-nginx",
		Title:       "Tail Nginx Error Log",
		Command:     "tail -f /var/log/nginx/error.log",
		Category:    "remote",
		Description: "Follow web server HTTP errors and connection drops",
		IsBuiltin:   true,
	},
	{
		ID:          "builtin-remote-ssh-port-forward",
		Title:       "SSH Local Port Forward",
		Command:     "ssh -N -L 8080:localhost:80 user@remote-host",
		Category:    "remote",
		Description: "Forward remote port 80 to your local machine at port 8080",
		IsBuiltin:   true,
	},
	{
		ID:          "builtin-remote-rsync",
		Title:       "Rsync Fast Upload",
		Command:     "rsync -avzP --exclude 'node_modules' --exclude '.git' ./ user@remote-host:/var/www/app/",
		Category:    "remote",
		Description: "Efficiently sync local directory to remote server over SSH",
		IsBuiltin:   true,
	},
	{
		ID:          "builtin-remote-curl-head",
		Title:       "Inspect HTTP Response",
		Command:     "curl -I -sS https://localhost",
		Category:    "remote",
		Description: "Fetch headers from local or remote web server",
		IsBuiltin:   true,
	},
	// Docker
	{
		ID:          "builtin-docker-ps",
		Title:       "Docker PS Formatted",
		Command:     `docker ps -a --format "table {{.ID}}\t{{.Names}}\t{{.Status}}\t{{.Ports}}"`,
		Category:    "docker",
		Description: "Clean readable list of running and stopped containers",
		IsBuiltin:   true,
	},
	{
		ID:          "builtin-docker-up",
		Title:       "Docker Compose Up",
		Command:     "docker compose up -d",
		Category:    "docker",
		Description: "Start compose service containers in detached background mode",
		IsBuiltin:   true,
	},
	{
		ID:          "builtin-docker-down",
		Title:       "Docker Compose Down",
		Command:     "docker compose down",
		Category:    "docker",
		Description: "Stop and remove containers, networks, and internal mounts",
		IsBuiltin:   true,
	},
	{
		ID:          "builtin-docker-prune",
		Title:       "Prune Dangling Resources",
		Command:     "docker system prune -f",
		Category:    "docker",
		Description: "Remove unused containers, networks, and untagged images",
		IsBuiltin:   true,
	},
	{
		ID:          "builtin-docker-exec",
		Title:       "Exec Shell into Container",
		Command:     "docker exec -it <container> /bin/sh",
		Category:    "docker",
		Description: "Open an interactive shell inside a running container",
		IsBuiltin:   true,
	},
	// Git
	{
		ID:          "builtin-git-undo-commit",
		Title:       "Undo Last Commit (Keep Files)",
		Command:     "git reset --soft HEAD~1",
		Category:    "git",
		Description: "Move HEAD back one commit while keeping changes staged",
		IsBuiltin:   true,
	},
	{
		ID:          "builtin-git-stash-pop",
		Title:       "Stash & Reapply",
		Command:     "git stash push -m 'wip' && git stash pop",
		Category:    "git",
		Description: "Save uncommitted edits to temporary stash and reapply them",
		IsBuiltin:   true,
	},
	{
		ID:          "builtin-git-clean-branches",
		Title:       "Delete Merged Branches",
		Command:     "git branch --merged | grep -v '\\*\\|main\\|master' | xargs -n 1 git branch -d",
		Category:    "git",
		Description: "Safely remove all local branches that are already merged",
		IsBuiltin:   true,
	},
	// System
	{
		ID:          "builtin-sys-disk",
		Title:       "Disk Space Usage",
		Command:     "df -h",
		Category:    "system",
		Description: "Show filesystem disk usage in human readable format",
		IsBuiltin:   true,
	},
	{
		ID:          "builtin-sys-ports",
		Title:       "Find Process on Port",
		Command:     "lsof -i :8080",
		Category:    "system",
		Description: "List processes listening on specific network port",
		IsBuiltin:   true,
	},
	{
		ID:          "builtin-sys-kill-port",
		Title:       "Kill Process on Port",
		Command:     "kill -9 $(lsof -t -i :8080)",
		Category:    "system",
		Description: "Terminate the process currently occupying port 8080",
		IsBuiltin:   true,
	},
	// Dev
	{
		ID:          "builtin-dev-deps",
		Title:       "Install Dependencies",
		Command:     "bun install || npm install || go mod tidy",
		Category:    "dev",
		Description: "Detect package manager and install project dependencies",
		IsBuiltin:   true,
	},
	{
		ID:          "builtin-dev-run-tests",
		Title:       "Run Test Suite",
		Command:     "go test -v ./... || bun test || npm test",
		Category:    "dev",
		Description: "Execute tests with verbose reporting",
		IsBuiltin:   true,
	},
}

// SnippetManager coordinates snippets and history persistence.
type SnippetManager struct {
	mu          sync.RWMutex
	dataDir     string
	customItems []Snippet
	history     []HistoryItem
}

// NewSnippetManager creates and initializes the snippet manager.
func NewSnippetManager(dataDir string) *SnippetManager {
	sm := &SnippetManager{
		dataDir:     dataDir,
		customItems: make([]Snippet, 0),
		history:     make([]HistoryItem, 0),
	}
	sm.load()
	return sm
}

func (sm *SnippetManager) load() {
	if sm.dataDir == "" {
		return
	}
	_ = os.MkdirAll(sm.dataDir, 0755)

	snipPath := filepath.Join(sm.dataDir, "terminal-snippets.json")
	if data, err := os.ReadFile(snipPath); err == nil {
		_ = json.Unmarshal(data, &sm.customItems)
	}

	histPath := filepath.Join(sm.dataDir, "terminal-history.json")
	if data, err := os.ReadFile(histPath); err == nil {
		_ = json.Unmarshal(data, &sm.history)
	}
}

func (sm *SnippetManager) save() {
	if sm.dataDir == "" {
		return
	}
	_ = os.MkdirAll(sm.dataDir, 0755)

	snipPath := filepath.Join(sm.dataDir, "terminal-snippets.json")
	if data, err := json.MarshalIndent(sm.customItems, "", "  "); err == nil {
		_ = os.WriteFile(snipPath, data, 0644)
	}

	histPath := filepath.Join(sm.dataDir, "terminal-history.json")
	if data, err := json.MarshalIndent(sm.history, "", "  "); err == nil {
		_ = os.WriteFile(histPath, data, 0644)
	}
}

// AllSnippets returns both built-in and user-custom snippets.
func (sm *SnippetManager) AllSnippets() []Snippet {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	res := make([]Snippet, 0, len(BuiltinSnippets)+len(sm.customItems))
	res = append(res, BuiltinSnippets...)
	res = append(res, sm.customItems...)
	return res
}

// FilterSnippets returns snippets filtered by category and query.
func (sm *SnippetManager) FilterSnippets(category, query string) []Snippet {
	all := sm.AllSnippets()
	q := strings.TrimSpace(strings.ToLower(query))
	cat := strings.TrimSpace(strings.ToLower(category))

	var out []Snippet
	for _, sn := range all {
		if cat != "" && cat != "all" && strings.ToLower(sn.Category) != cat {
			continue
		}
		if q != "" {
			hay := strings.ToLower(sn.Title + " " + sn.Command + " " + sn.Description + " " + sn.Category)
			if !strings.Contains(hay, q) {
				continue
			}
		}
		out = append(out, sn)
	}
	return out
}

// AddSnippet adds a new custom snippet.
func (sm *SnippetManager) AddSnippet(title, command, category, description string) Snippet {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if category == "" {
		category = "custom"
	}
	sn := Snippet{
		ID:          fmt.Sprintf("snip-%d", time.Now().UnixNano()),
		Title:       strings.TrimSpace(title),
		Command:     strings.TrimSpace(command),
		Category:    strings.ToLower(strings.TrimSpace(category)),
		Description: strings.TrimSpace(description),
		IsBuiltin:   false,
		CreatedAt:   time.Now(),
	}
	sm.customItems = append([]Snippet{sn}, sm.customItems...)
	sm.save()
	return sn
}

// UpdateSnippet modifies an existing custom snippet.
func (sm *SnippetManager) UpdateSnippet(id, title, command, category, description string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for i, it := range sm.customItems {
		if it.ID == id {
			sm.customItems[i].Title = strings.TrimSpace(title)
			sm.customItems[i].Command = strings.TrimSpace(command)
			sm.customItems[i].Category = strings.ToLower(strings.TrimSpace(category))
			sm.customItems[i].Description = strings.TrimSpace(description)
			sm.save()
			return true
		}
	}
	return false
}

// DeleteSnippet removes a custom snippet by ID.
func (sm *SnippetManager) DeleteSnippet(id string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for i, it := range sm.customItems {
		if it.ID == id {
			sm.customItems = append(sm.customItems[:i], sm.customItems[i+1:]...)
			sm.save()
			return true
		}
	}
	return false
}

// RecordHistory adds or moves a command to the front of history.
func (sm *SnippetManager) RecordHistory(command, source string) {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// De-duplicate if same command was just run
	newHist := make([]HistoryItem, 0, len(sm.history)+1)
	isFav := false
	for _, h := range sm.history {
		if h.Command == cmd {
			isFav = h.IsFavorite
			continue
		}
		newHist = append(newHist, h)
	}

	item := HistoryItem{
		ID:         fmt.Sprintf("hist-%d", time.Now().UnixNano()),
		Command:    cmd,
		Timestamp:  time.Now(),
		IsFavorite: isFav,
		Source:     source,
	}

	sm.history = append([]HistoryItem{item}, newHist...)
	if len(sm.history) > 200 {
		sm.history = sm.history[:200]
	}
	sm.save()
}

// ToggleFavorite flips the favorite status of a history item.
func (sm *SnippetManager) ToggleFavorite(id string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for i, h := range sm.history {
		if h.ID == id {
			sm.history[i].IsFavorite = !sm.history[i].IsFavorite
			sm.save()
			return sm.history[i].IsFavorite
		}
	}
	return false
}

// DeleteHistory removes an individual history item.
func (sm *SnippetManager) DeleteHistory(id string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for i, h := range sm.history {
		if h.ID == id {
			sm.history = append(sm.history[:i], sm.history[i+1:]...)
			sm.save()
			return
		}
	}
}

// ClearHistory removes non-favorite history items.
func (sm *SnippetManager) ClearHistory() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	favs := make([]HistoryItem, 0)
	for _, h := range sm.history {
		if h.IsFavorite {
			favs = append(favs, h)
		}
	}
	sm.history = favs
	sm.save()
}

// HistoryItems returns the history list, optionally filtered by query.
func (sm *SnippetManager) HistoryItems(query string) []HistoryItem {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	q := strings.TrimSpace(strings.ToLower(query))
	if q == "" {
		res := make([]HistoryItem, len(sm.history))
		copy(res, sm.history)
		return res
	}

	var out []HistoryItem
	for _, h := range sm.history {
		if strings.Contains(strings.ToLower(h.Command), q) {
			out = append(out, h)
		}
	}
	return out
}
