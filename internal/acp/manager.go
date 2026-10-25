package acp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Haslab-dev/RakitKode/events"
	"github.com/google/uuid"
)

type activeToolInfo struct {
	name  string
	title string
	args  interface{}
}

type Manager struct {
	bus *events.Bus

	mu                 sync.RWMutex
	dataDir            string
	sessions           map[string]*Session
	clients            map[string]*Client
	pendingDecisions   map[string]chan string               // requestID -> decision channel
	activeTools        map[string]map[string]activeToolInfo // sessionID -> toolCallID -> info
	sessionHasStreamed map[string]bool                      // sessionID -> whether message text was streamed
}

func NewManager(bus *events.Bus) *Manager {
	return &Manager{
		bus:                bus,
		sessions:           make(map[string]*Session),
		clients:            make(map[string]*Client),
		pendingDecisions:   make(map[string]chan string),
		activeTools:        make(map[string]map[string]activeToolInfo),
		sessionHasStreamed: make(map[string]bool),
	}
}

// SetDataDir assigns the storage directory and restores historical ACP sessions.
func (m *Manager) SetDataDir(dir string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dataDir = dir
	m.loadSessionsLocked()
}

func (m *Manager) loadSessionsLocked() {
	if m.dataDir == "" {
		return
	}
	filePath := filepath.Join(m.dataDir, "acp_sessions.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return
	}
	var loaded []*Session
	if err := json.Unmarshal(data, &loaded); err != nil {
		return
	}
	for _, sess := range loaded {
		if sess != nil && sess.ID != "" {
			if sess.Status == "running" || sess.Status == "waiting_permission" {
				sess.Status = "idle"
			}
			m.sessions[sess.ID] = sess
		}
	}
}

func (m *Manager) saveSessionsLocked() {
	if m.dataDir == "" {
		return
	}
	list := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		if s != nil {
			list = append(list, s)
		}
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return
	}
	filePath := filepath.Join(m.dataDir, "acp_sessions.json")
	_ = os.WriteFile(filePath, data, 0o644)
}

func (m *Manager) recordUsageLocked(sessionID string, u *ContextUsage) {
	if u == nil {
		return
	}
	sess, ok := m.sessions[sessionID]
	if !ok || sess == nil {
		return
	}
	if sess.Usage == nil {
		sess.Usage = &ContextUsage{}
	}
	sess.Usage.InputTokens += u.InputTokens
	sess.Usage.OutputTokens += u.OutputTokens
	sess.Usage.CachedTokens += u.CachedTokens
	total := u.TotalTokens
	if total == 0 && (u.InputTokens > 0 || u.OutputTokens > 0) {
		total = u.InputTokens + u.OutputTokens
	}
	sess.Usage.TotalTokens += total
	sess.Usage.CostUSD += u.CostUSD
	sess.UpdatedAt = time.Now().UnixMilli()
	m.saveSessionsLocked()

	m.emitEvent("agent:usage", map[string]interface{}{
		"sessionId":   sessionID,
		"session_id":  sessionID,
		"usage":       sess.Usage,
		"promptCount": sess.PromptCount,
	})
}

// resolveExecutablePath searches system PATH and standard user CLI install paths
// (e.g. ~/.local/bin, ~/.opencode/bin, /opt/homebrew/bin) so GUI apps on macOS find them reliably.
func resolveExecutablePath(cmdName string) (string, error) {
	if cmdName == "" {
		return "", fmt.Errorf("command name is empty")
	}

	// If explicit path, verify directly
	if strings.Contains(cmdName, "/") {
		if fi, err := os.Stat(cmdName); err == nil && !fi.IsDir() {
			return cmdName, nil
		}
		return "", fmt.Errorf("command not found at %s", cmdName)
	}

	home, _ := os.UserHomeDir()
	candidateDirs := []string{
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".opencode", "bin"),
		filepath.Join(home, ".commandcode", "bin"),
		filepath.Join(home, ".cmd", "bin"),
		"/opt/homebrew/bin",
		"/opt/homebrew/sbin",
		"/usr/local/bin",
		"/usr/bin",
		"/bin",
		filepath.Join(home, ".cargo", "bin"),
		filepath.Join(home, ".bun", "bin"),
	}

	nvmNodeDir := filepath.Join(home, ".nvm", "versions", "node")
	if entries, err := os.ReadDir(nvmNodeDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				candidateDirs = append(candidateDirs, filepath.Join(nvmNodeDir, entry.Name(), "bin"))
			}
		}
	}

	for _, dir := range candidateDirs {
		p := filepath.Join(dir, cmdName)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, nil
		}
	}

	if found, err := exec.LookPath(cmdName); err == nil {
		return found, nil
	}

	// If cmd or commandcode, try the alternate executable name
	if cmdName == "cmd" {
		if alt, err := resolveExecutablePath("commandcode"); err == nil {
			return alt, nil
		}
	} else if cmdName == "commandcode" {
		if alt, err := resolveExecutablePath("cmd"); err == nil {
			return alt, nil
		}
	}

	return "", fmt.Errorf("command '%s' not found on system PATH or CLI directories (~/.local/bin, /opt/homebrew/bin, etc.)", cmdName)
}

// enrichedEnv returns the environment with enriched PATH containing CLI tool directories.
func enrichedEnv(extra []string) []string {
	home, _ := os.UserHomeDir()
	extraPaths := []string{
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".opencode", "bin"),
		filepath.Join(home, ".commandcode", "bin"),
		filepath.Join(home, ".cmd", "bin"),
		"/opt/homebrew/bin",
		"/opt/homebrew/sbin",
		"/usr/local/bin",
		filepath.Join(home, ".cargo", "bin"),
		filepath.Join(home, ".bun", "bin"),
	}

	nvmNodeDir := filepath.Join(home, ".nvm", "versions", "node")
	if entries, err := os.ReadDir(nvmNodeDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				extraPaths = append(extraPaths, filepath.Join(nvmNodeDir, entry.Name(), "bin"))
			}
		}
	}
	currentPath := os.Getenv("PATH")
	newPath := strings.Join(extraPaths, ":")
	if currentPath != "" {
		newPath = newPath + ":" + currentPath
	}

	baseEnv := os.Environ()
	var res []string
	pathFound := false
	for _, env := range baseEnv {
		if strings.HasPrefix(env, "PATH=") {
			res = append(res, "PATH="+newPath)
			pathFound = true
		} else {
			res = append(res, env)
		}
	}
	if !pathFound {
		res = append(res, "PATH="+newPath)
	}
	res = append(res, extra...)
	return res
}

func (m *Manager) StartSession(opts SessionOpts) (*Session, error) {
	cmdName := opts.Command
	args := opts.Args

	agentKey := strings.ToLower(opts.AgentID)
	agentKey = strings.TrimPrefix(agentKey, "agent-")

	resumeID := strings.TrimSpace(opts.ResumeSessionID)
	if resumeID == "" && strings.TrimSpace(opts.SessionID) != "" {
		resumeID = strings.TrimSpace(opts.SessionID)
	}

	protocol := "acp"
	if agentKey == "agy" || agentKey == "antigravity" || opts.Command == "agy" {
		protocol = "agy-stream"
		if cmdName == "" {
			cmdName = "agy"
		}
		args = []string{"--input-format", "stream-json", "--output-format", "stream-json"}
		if resumeID != "" {
			args = append(args, "--conversation", resumeID)
		}
		if opts.Model != "" && opts.Model != "default" && opts.Model != "Default" {
			args = append(args, "--model", opts.Model)
		}
		args = append(args, "--dangerously-skip-permissions")
	} else if agentKey == "claude" || opts.Command == "claude" {
		protocol = "claude-stream"
		if cmdName == "" {
			cmdName = "claude"
		}
		args = []string{"-p", "--input-format", "stream-json", "--output-format", "stream-json", "--verbose", "--include-partial-messages"}
		if resumeID != "" {
			args = append(args, "--resume", resumeID)
		}
		if opts.Model != "" && opts.Model != "default" && opts.Model != "Default" {
			modelArg := opts.Model
			if strings.EqualFold(modelArg, "glm-5.3-flash") {
				modelArg = "sonnet"
			}
			args = append(args, "--model", modelArg)
		}
		args = append(args, "--dangerously-skip-permissions")
	} else if agentKey == "codex" || opts.Command == "codex" {
		protocol = "codex-appserver"
		if cmdName == "" {
			cmdName = "codex"
		}
		args = []string{"app-server", "--listen", "stdio://"}
	} else if agentKey == "opencode" || opts.Command == "opencode" {
		protocol = "acp"
		if cmdName == "" {
			cmdName = "opencode"
		}
		if len(args) == 0 {
			args = []string{"acp"}
		}
	} else if agentKey == "commandcode" || agentKey == "cmd" || opts.Command == "cmd" || opts.Command == "commandcode" {
		protocol = "acp"
		if cmdName == "" {
			cmdName = "cmd"
		}
		if len(args) == 0 {
			args = []string{"acp"}
		}
	} else if agentKey == "omp" || agentKey == "ohmypi" || opts.Command == "omp" {
		protocol = "acp"
		if cmdName == "" {
			cmdName = "omp"
		}
		if len(args) == 0 {
			args = []string{"acp"}
		}
	} else if agentKey == "pi" || agentKey == "pi-acp" || opts.Command == "pi-acp" || opts.Command == "pi" {
		protocol = "acp"
		if cmdName == "" {
			cmdName = "pi-acp"
		}
		if cmdName == "pi" && len(args) == 0 {
			cmdName = "pi-acp"
		}
	} else if cmdName == "" {
		return nil, fmt.Errorf("acp: no command specified for agent %s", opts.AgentID)
	}

	cwd := strings.TrimSpace(opts.Cwd)
	if cwd == "" || cwd == "." {
		if wd, err := os.Getwd(); err == nil && wd != "" {
			cwd = wd
		} else if home, err := os.UserHomeDir(); err == nil && home != "" {
			cwd = home
		} else {
			cwd = "/tmp"
		}
	}
	if abs, err := filepath.Abs(cwd); err == nil {
		cwd = abs
	}
	opts.Cwd = cwd

	// Verify command exists in enriched PATH
	resolvedCmd, err := resolveExecutablePath(cmdName)
	if err != nil {
		return nil, fmt.Errorf("acp: %w", err)
	}

	sessionID := strings.TrimSpace(opts.SessionID)
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	m.mu.RLock()
	existingSession, hasSess := m.sessions[sessionID]
	existingClient, hasCli := m.clients[sessionID]
	m.mu.RUnlock()
	if hasSess && hasCli && !existingClient.IsClosed() && existingSession.Status != "closed" && existingSession.Status != "error" {
		log.Printf("[acp] reusing already-running session %s", sessionID)
		return existingSession, nil
	}

	// Clean up any stale/dead session under the same ID
	m.mu.Lock()
	if oldCli, ok := m.clients[sessionID]; ok {
		_ = oldCli.Close()
		delete(m.clients, sessionID)
	}
	delete(m.sessions, sessionID)
	delete(m.activeTools, sessionID)
	m.mu.Unlock()

	var envPairs []string
	for k, v := range opts.Env {
		envPairs = append(envPairs, fmt.Sprintf("%s=%s", k, v))
	}
	finalEnv := enrichedEnv(envPairs)

	client, err := SpawnClient(resolvedCmd, args, finalEnv, opts.Cwd)
	if err != nil {
		return nil, fmt.Errorf("acp: spawn agent: %w", err)
	}

	now := time.Now().UnixMilli()

	permMode := opts.PermissionMode
	if permMode == "" {
		permMode = "supervised"
	}

	session := &Session{
		ID:             sessionID,
		Title:          opts.Title,
		AgentID:        opts.AgentID,
		Cwd:            opts.Cwd,
		Model:          opts.Model,
		Protocol:       protocol,
		PermissionMode: permMode,
		Status:         "idle",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	m.mu.Lock()
	if existing, exists := m.sessions[sessionID]; exists && existing != nil {
		if session.Title == "" {
			session.Title = existing.Title
		}
		session.PromptCount = existing.PromptCount
		session.Usage = existing.Usage
		if existing.CreatedAt > 0 {
			session.CreatedAt = existing.CreatedAt
		}
	}
	m.sessions[sessionID] = session
	m.clients[sessionID] = client
	m.saveSessionsLocked()
	m.mu.Unlock()

	client.OnClose = func(err error) {
		m.mu.Lock()
		if sess, ok := m.sessions[sessionID]; ok {
			sess.Status = "closed"
			sess.UpdatedAt = time.Now().UnixMilli()
			m.saveSessionsLocked()
		}
		m.mu.Unlock()
		log.Printf("[acp] session %s client closed (err: %v)", sessionID, err)
	}

	// Set up notification dispatcher
	client.OnNotification = func(method string, params json.RawMessage) {
		m.handleNotification(sessionID, method, params)
	}

	// Set up request dispatcher (e.g. permission approval)
	client.OnRequest = func(id interface{}, method string, params json.RawMessage) {
		m.handleRequest(sessionID, id, method, params)
	}

	if protocol == "agy-stream" {
		// Google Antigravity agy emits {"event":"init", ...} on startup
		initCh := make(chan struct{}, 1)

		client.OnStreamEvent = func(event string, raw []byte) {
			m.handleStreamEvent(sessionID, event, raw, initCh)
		}
		client.StartReading()

		select {
		case <-initCh:
			// Handshake complete
		case <-time.After(12 * time.Second):
			stderrLines := client.LastStderr()
			errMsg := fmt.Sprintf("stream-json agent '%s' failed to emit init within 12s", cmdName)
			if len(stderrLines) > 0 {
				errMsg += "\nStderr output:\n" + strings.Join(stderrLines, "\n")
			}
			_ = client.Close()
			m.mu.Lock()
			delete(m.sessions, sessionID)
			delete(m.clients, sessionID)
			m.mu.Unlock()
			return nil, fmt.Errorf("%s", errMsg)
		}
	} else if protocol == "claude-stream" {
		// Claude stream-json connects and waits for user turns on stdin
		client.OnStreamEvent = func(event string, raw []byte) {
			m.handleStreamEvent(sessionID, event, raw, nil)
		}
		client.StartReading()
	} else if protocol == "codex-appserver" {
		client.StartReading()
		// OpenAI Codex app-server JSON-RPC 2.0
		initCtx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()

		var initRes map[string]interface{}
		if err := client.SendRequest(initCtx, "initialize", map[string]interface{}{
			"clientInfo": map[string]string{
				"name":    "ForgeADE",
				"version": "1.0.0",
			},
		}, &initRes); err != nil {
			stderrLines := client.LastStderr()
			errMsg := fmt.Sprintf("codex app-server initialize failed: %v", err)
			if len(stderrLines) > 0 {
				errMsg += "\nStderr output:\n" + strings.Join(stderrLines, "\n")
			}
			_ = client.Close()
			m.mu.Lock()
			delete(m.sessions, sessionID)
			delete(m.clients, sessionID)
			m.mu.Unlock()
			return nil, fmt.Errorf("%s", errMsg)
		}

		var resumeSuccess bool
		if resumeID != "" {
			threadCtx, threadCancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer threadCancel()

			var threadRes struct {
				Thread struct {
					ID string `json:"id"`
				} `json:"thread"`
			}
			if err := client.SendRequest(threadCtx, "thread/resume", map[string]interface{}{"threadId": resumeID}, &threadRes); err == nil && threadRes.Thread.ID != "" {
				session.ThreadID = threadRes.Thread.ID
				session.RemoteSessionID = threadRes.Thread.ID
				resumeSuccess = true
				log.Printf("[acp] codex thread/resume succeeded: threadId=%s", session.ThreadID)
			} else {
				log.Printf("[acp] codex thread/resume notice: %v, falling back to thread/start", err)
			}
		}

		if !resumeSuccess {
			threadCtx, threadCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer threadCancel()

			var threadRes struct {
				Thread struct {
					ID string `json:"id"`
				} `json:"thread"`
			}
			threadParams := map[string]interface{}{
				"cwd": opts.Cwd,
			}
			if opts.Model != "" && opts.Model != "default" && opts.Model != "Default" {
				threadParams["model"] = opts.Model
			}
			if err := client.SendRequest(threadCtx, "thread/start", threadParams, &threadRes); err != nil {
				log.Printf("codex thread/start warning: %v", err)
			}
			if threadRes.Thread.ID != "" {
				session.ThreadID = threadRes.Thread.ID
				session.RemoteSessionID = threadRes.Thread.ID
			} else {
				session.ThreadID = sessionID
				session.RemoteSessionID = sessionID
			}
		}
	} else {
		client.StartReading()
		// JSON-RPC 2.0 ACP Handshake (e.g. OpenCode acp)
		initCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		initParams := InitializeParams{
			ProtocolVersion: ProtocolVersion,
			ClientInfo: ClientInfo{
				Name:    "ForgeADE",
				Version: "1.0.0",
			},
			Capabilities: ClientCapabilities{
				Streaming:   true,
				Permissions: true,
				Tools:       true,
			},
		}

		var initResult InitializeResult
		if err := client.SendRequest(initCtx, MethodInitialize, initParams, &initResult); err != nil {
			stderrLines := client.LastStderr()
			errMsg := fmt.Sprintf("acp initialize failed: %v", err)
			if len(stderrLines) > 0 {
				errMsg += "\nStderr output:\n" + strings.Join(stderrLines, "\n")
			}
			_ = client.Close()
			m.mu.Lock()
			delete(m.sessions, sessionID)
			delete(m.clients, sessionID)
			m.mu.Unlock()
			return nil, fmt.Errorf("%s", errMsg)
		}

		// Attempt session/resume or session/load if resumeID is present
		var resumeSuccess bool
		if resumeID != "" {
			resumeCtx, resumeCancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer resumeCancel()

			resumeParams := map[string]interface{}{
				"sessionId": resumeID,
				"cwd":       opts.Cwd,
			}
			var resumeResult struct {
				SessionID string `json:"sessionId"`
			}
			// 1. Try session/resume
			if err := client.SendRequest(resumeCtx, "session/resume", resumeParams, &resumeResult); err == nil {
				if resumeResult.SessionID != "" {
					session.RemoteSessionID = resumeResult.SessionID
				} else {
					session.RemoteSessionID = resumeID
				}
				resumeSuccess = true
				log.Printf("[acp] session/resume succeeded with remoteSessionId=%s", session.RemoteSessionID)
			} else {
				// 2. Try session/load
				loadCtx, loadCancel := context.WithTimeout(context.Background(), 8*time.Second)
				defer loadCancel()
				if err := client.SendRequest(loadCtx, "session/load", resumeParams, &resumeResult); err == nil {
					if resumeResult.SessionID != "" {
						session.RemoteSessionID = resumeResult.SessionID
					} else {
						session.RemoteSessionID = resumeID
					}
					resumeSuccess = true
					log.Printf("[acp] session/load succeeded with remoteSessionId=%s", session.RemoteSessionID)
				} else {
					log.Printf("[acp] session resume/load notice: %v, falling back to session/new", err)
				}
			}
		}

		if !resumeSuccess {
			// Create session on agent if supported
			newSessCtx, newCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer newCancel()

			newParams := SessionNewParams{
				Cwd:        opts.Cwd,
				McpServers: []interface{}{},
			}

			var newResult SessionNewResult
			if err := client.SendRequest(newSessCtx, MethodSessionNew, newParams, &newResult); err != nil {
				stderrLines := client.LastStderr()
				errMsg := fmt.Sprintf("acp session/new failed: %v", err)
				if len(stderrLines) > 0 {
					errMsg += "\nStderr output:\n" + strings.Join(stderrLines, "\n")
				}
				_ = client.Close()
				m.mu.Lock()
				delete(m.sessions, sessionID)
				delete(m.clients, sessionID)
				m.mu.Unlock()
				return nil, fmt.Errorf("%s", errMsg)
			}
			if newResult.SessionID == "" {
				_ = client.Close()
				m.mu.Lock()
				delete(m.sessions, sessionID)
				delete(m.clients, sessionID)
				m.mu.Unlock()
				return nil, fmt.Errorf("acp session/new failed: agent returned empty sessionId")
			}
			session.RemoteSessionID = newResult.SessionID
			if newResult.Meta != nil {
				if rawCmds, ok := newResult.Meta["availableCommands"]; ok {
					b, _ := json.Marshal(rawCmds)
					var cmds []AvailableCommand
					if err := json.Unmarshal(b, &cmds); err == nil && len(cmds) > 0 {
						session.AvailableCommands = cmds
					}
				}
			}
			log.Printf("[acp] session/new succeeded: remoteSessionId=%s (localSessionId=%s)", session.RemoteSessionID, sessionID)
		}

		// Configure model via session/set_config_option if model specified (e.g. omp, pi)
		if opts.Model != "" && opts.Model != "default" && opts.Model != "Default" {
			setCtx, setCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer setCancel()
			if err := client.SendRequest(setCtx, "session/set_config_option", map[string]interface{}{
				"sessionId": session.RemoteSessionID,
				"configId":  "model",
				"value":     opts.Model,
			}, nil); err != nil {
				log.Printf("[acp] session/set_config_option notice: %v", err)
			}
		}
	}

	m.emitEvent("agent:started", map[string]interface{}{
		"sessionId":  sessionID,
		"session_id": sessionID,
		"agentId":    opts.AgentID,
		"cwd":        opts.Cwd,
	})

	return session, nil
}

type extractedImage struct {
	path     string
	mimeType string
	base64   string
}

func extractImagesFromPrompt(prompt string, cwd string) []extractedImage {
	var images []extractedImage
	seen := make(map[string]bool)

	imageExts := map[string]string{
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".webp": "image/webp",
		".gif":  "image/gif",
		".svg":  "image/svg+xml",
		".bmp":  "image/bmp",
	}

	lines := strings.Split(prompt, "\n")
	inAttached := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(trimmed), "attached files:") {
			inAttached = true
			continue
		}
		if inAttached {
			if strings.HasPrefix(trimmed, "@") {
				raw := strings.TrimPrefix(trimmed, "@")
				if idx := strings.Index(raw, " ("); idx != -1 {
					raw = strings.TrimSpace(raw[:idx])
				}
				processImagePath(raw, cwd, imageExts, seen, &images)
			} else if trimmed == "" {
				continue
			} else if !strings.HasPrefix(trimmed, "-") && !strings.HasPrefix(trimmed, "*") {
				inAttached = false
			}
		}
	}

	words := strings.Fields(prompt)
	for _, word := range words {
		if strings.HasPrefix(word, "@") {
			raw := strings.TrimPrefix(word, "@")
			raw = strings.Trim(raw, `"'(),:;`)
			processImagePath(raw, cwd, imageExts, seen, &images)
		}
	}

	return images
}

func processImagePath(raw string, cwd string, imageExts map[string]string, seen map[string]bool, out *[]extractedImage) {
	if raw == "" {
		return
	}
	ext := strings.ToLower(filepath.Ext(raw))
	mime, isImg := imageExts[ext]
	if !isImg {
		return
	}

	candidates := []string{}
	if filepath.IsAbs(raw) {
		candidates = append(candidates, raw)
	} else if cwd != "" {
		candidates = append(candidates, filepath.Join(cwd, raw))
		candidates = append(candidates, filepath.Join(cwd, ".forge", "attachments", filepath.Base(raw)))
	}

	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".forge-ade", "attachments", filepath.Base(raw)))
		candidates = append(candidates, filepath.Join(home, ".forge-ade", "general", ".forge", "attachments", filepath.Base(raw)))
	}

	for _, p := range candidates {
		if seen[p] {
			return
		}
		info, err := os.Stat(p)
		if err == nil && !info.IsDir() && info.Size() > 0 && info.Size() <= 10*1024*1024 {
			data, err := os.ReadFile(p)
			if err == nil && len(data) > 0 {
				seen[p] = true
				b64 := base64.StdEncoding.EncodeToString(data)
				*out = append(*out, extractedImage{
					path:     p,
					mimeType: mime,
					base64:   b64,
				})

				// If cwd is set and file is outside cwd, copy it into cwd/.forge/attachments
				// so the agent's CLI workspace tools can also read it locally:
				if cwd != "" && !strings.HasPrefix(p, cwd) {
					wsAttachDir := filepath.Join(cwd, ".forge", "attachments")
					_ = os.MkdirAll(wsAttachDir, 0755)
					destPath := filepath.Join(wsAttachDir, filepath.Base(p))
					if _, statErr := os.Stat(destPath); os.IsNotExist(statErr) {
						_ = os.WriteFile(destPath, data, 0644)
					}
				}
				return
			}
		}
	}
}

const openUIGuidance = `[OpenUI Generative UI: You can render rich generative UI by outputting a fenced block tagged opui or openui (e.g. ` + "```" + `opui ... ` + "```" + `).
Example:
` + "```" + `opui
root = Card([stat, badge, note, act], "Summary", "Details")
stat = Metric("Status", "Ready", "up")
badge = Badge("Success", "success")
note = Text("Completed successfully.")
act = Button("Continue", @send_prompt("Next task"))
` + "```" + `
Components: Card(children, title, subtitle), Stack(children, direction), Text(content), Heading(content, level), Metric(label, value, trend ["up"|"down"|"flat"|"neutral"]), Badge(text, tone ["default"|"success"|"warning"|"danger"|"info"|"primary"]), Button(label, action), Table(headers, rows), List(items), Image(url, alt), Divider(). Single statement per line; root is entry point.]`

func enrichPromptWithBoardGuidance(session *Session, prompt string) string {
	if session == nil {
		return prompt
	}
	lower := strings.ToLower(prompt)
	hasBoardKeyword := strings.Contains(lower, "board") ||
		strings.Contains(lower, "kanban") ||
		strings.Contains(lower, "backlog") ||
		strings.Contains(lower, "task baru") ||
		strings.Contains(lower, "buat task") ||
		strings.Contains(lower, "create task") ||
		strings.Contains(lower, "add task") ||
		strings.Contains(lower, "update task") ||
		strings.Contains(lower, "move task")

	hasUIKeyword := strings.Contains(lower, "openui") ||
		strings.Contains(lower, "opui") ||
		strings.Contains(lower, "dashboard") ||
		strings.Contains(lower, "generative ui")

	isFirstTurn := session.PromptCount == 0
	session.PromptCount++

	res := prompt
	if isFirstTurn || hasBoardKeyword {
		res += "\n\n[System Context: Forge ADE Workflow Kanban Board tasks are stored at .forge/board_tasks.json. When creating, viewing, or updating tasks, read or edit this JSON file directly. Changes are monitored by a real-time file watcher.]"
	}
	if hasUIKeyword {
		res += "\n\n" + openUIGuidance
	}
	return res
}

func (m *Manager) Prompt(sessionID string, prompt string) error {
	m.mu.RLock()
	client, hasClient := m.clients[sessionID]
	session, hasSession := m.sessions[sessionID]
	m.mu.RUnlock()

	if !hasClient || !hasSession || client == nil || client.IsClosed() || session.Status == "closed" {
		return fmt.Errorf("acp: session not found: %s", sessionID)
	}

	m.mu.Lock()
	session.Status = "running"
	session.PromptCount++
	session.UpdatedAt = time.Now().UnixMilli()
	m.sessionHasStreamed[sessionID] = false
	m.saveSessionsLocked()
	effectivePrompt := enrichPromptWithBoardGuidance(session, prompt)
	m.mu.Unlock()

	m.emitEvent("agent:turn_start", map[string]interface{}{
		"sessionId":  sessionID,
		"session_id": sessionID,
		"prompt":     prompt,
	})

	images := extractImagesFromPrompt(prompt, session.Cwd)

	if session.Protocol == "agy-stream" || session.Protocol == "stream-json" {
		parts := []map[string]interface{}{
			{"text": effectivePrompt},
		}
		for _, img := range images {
			parts = append(parts, map[string]interface{}{
				"inlineData": map[string]interface{}{
					"mimeType": img.mimeType,
					"data":     img.base64,
				},
			})
		}
		inputMsg := map[string]interface{}{
			"event": "user",
			"message": map[string]interface{}{
				"content": effectivePrompt,
				"parts":   parts,
			},
		}
		data, err := json.Marshal(inputMsg)
		if err != nil {
			return fmt.Errorf("marshal stream input: %w", err)
		}

		if err := client.WriteRawLine(data); err != nil {
			m.mu.Lock()
			session.Status = "error"
			session.UpdatedAt = time.Now().UnixMilli()
			m.mu.Unlock()

			m.emitEvent("agent:turn_end", map[string]interface{}{
				"sessionId":  sessionID,
				"session_id": sessionID,
				"error":      err.Error(),
			})
			return fmt.Errorf("write stream prompt: %w", err)
		}
		return nil
	} else if session.Protocol == "claude-stream" {
		var content interface{} = effectivePrompt
		if len(images) > 0 {
			blocks := []map[string]interface{}{
				{"type": "text", "text": effectivePrompt},
			}
			for _, img := range images {
				blocks = append(blocks, map[string]interface{}{
					"type": "image",
					"source": map[string]interface{}{
						"type":       "base64",
						"media_type": img.mimeType,
						"data":       img.base64,
					},
				})
			}
			content = blocks
		}
		inputMsg := map[string]interface{}{
			"type": "user",
			"message": map[string]interface{}{
				"role":    "user",
				"content": content,
			},
		}
		data, err := json.Marshal(inputMsg)
		if err != nil {
			return fmt.Errorf("marshal claude input: %w", err)
		}

		if err := client.WriteRawLine(data); err != nil {
			m.mu.Lock()
			session.Status = "error"
			session.UpdatedAt = time.Now().UnixMilli()
			m.mu.Unlock()

			m.emitEvent("agent:turn_end", map[string]interface{}{
				"sessionId":  sessionID,
				"session_id": sessionID,
				"error":      err.Error(),
			})
			return fmt.Errorf("write claude prompt: %w", err)
		}
		return nil
	} else if session.Protocol == "codex-appserver" {
		threadID := session.ThreadID
		if threadID == "" {
			threadID = sessionID
		}
		inputs := []map[string]interface{}{
			{
				"type": "text",
				"text": effectivePrompt,
			},
		}
		for _, img := range images {
			inputs = append(inputs, map[string]interface{}{
				"type": "image",
				"url":  fmt.Sprintf("data:%s;base64,%s", img.mimeType, img.base64),
			})
		}
		params := map[string]interface{}{
			"threadId": threadID,
			"input":    inputs,
		}
		go func() {
			ctx := context.Background()
			var res json.RawMessage
			if err := client.SendRequest(ctx, "turn/start", params, &res); err != nil {
				log.Printf("codex turn/start error for %s: %v", sessionID, err)
				m.mu.Lock()
				session.Status = "error"
				session.UpdatedAt = time.Now().UnixMilli()
				m.mu.Unlock()

				m.emitEvent("agent:turn_end", map[string]interface{}{
					"sessionId":  sessionID,
					"session_id": sessionID,
					"error":      err.Error(),
				})
			}
		}()
		return nil
	}

	// JSON-RPC 2.0 ACP prompt
	targetSessID := session.RemoteSessionID
	if targetSessID == "" {
		m.mu.Lock()
		session.Status = "error"
		session.UpdatedAt = time.Now().UnixMilli()
		m.mu.Unlock()

		err := fmt.Errorf("acp: session %s has no active remote session", sessionID)
		m.emitEvent("agent:turn_end", map[string]interface{}{
			"sessionId":  sessionID,
			"session_id": sessionID,
			"error":      err.Error(),
		})
		return err
	}

	log.Printf("[acp] Prompting session %s with remoteSessionId %s", sessionID, targetSessID)
	promptBlocks := []PromptContentBlock{
		{
			Type: "text",
			Text: effectivePrompt,
		},
	}
	for _, img := range images {
		dataURI := fmt.Sprintf("data:%s;base64,%s", img.mimeType, img.base64)
		promptBlocks = append(promptBlocks, PromptContentBlock{
			Type:     "image",
			Data:     img.base64,
			MimeType: img.mimeType,
			URI:      dataURI,
			Source: &PromptImageSource{
				Type:      "base64",
				MediaType: img.mimeType,
				Data:      img.base64,
			},
		})
	}

	params := SessionPromptParams{
		SessionID: targetSessID,
		Prompt:    promptBlocks,
	}

	go func() {
		ctx := context.Background()
		var res SessionPromptResult
		if err := client.SendRequest(ctx, MethodSessionPrompt, params, &res); err != nil {
			log.Printf("acp prompt error for %s: %v", sessionID, err)
			m.mu.Lock()
			session.Status = "error"
			session.UpdatedAt = time.Now().UnixMilli()
			m.mu.Unlock()

			m.emitEvent("agent:turn_end", map[string]interface{}{
				"sessionId":  sessionID,
				"session_id": sessionID,
				"error":      err.Error(),
			})
			return
		}

		m.mu.Lock()
		session.Status = "idle"
		session.UpdatedAt = time.Now().UnixMilli()
		if res.Usage != nil {
			m.recordUsageLocked(sessionID, res.Usage)
		} else if session.Usage == nil || session.Usage.TotalTokens == 0 {
			inEst := len(effectivePrompt) / 4
			if inEst < 10 {
				inEst = 10
			}
			m.recordUsageLocked(sessionID, &ContextUsage{
				InputTokens:  inEst,
				OutputTokens: 25,
				TotalTokens:  inEst + 25,
			})
		}
		m.saveSessionsLocked()
		turnUsage := session.Usage
		hasStreamed := m.sessionHasStreamed[sessionID]
		m.sessionHasStreamed[sessionID] = false
		m.mu.Unlock()

		// If no streaming message deltas were emitted during the turn, extract text from prompt response
		if !hasStreamed {
			promptResText := extractContentText(res.Content)
			if promptResText == "" {
				promptResText = extractContentText(res.Output)
			}
			if promptResText == "" {
				promptResText = extractContentText(res.Message)
			}
			if promptResText == "" {
				promptResText = res.Text
			}
			if promptResText != "" {
				m.emitEvent("agent:message_delta", map[string]interface{}{
					"sessionId":  sessionID,
					"session_id": sessionID,
					"delta":      promptResText,
					"kind":       "text",
				})
			}
		}

		turnStatus := res.Status
		if turnStatus == "" {
			turnStatus = res.StopReason
		}
		if turnStatus == "" {
			turnStatus = "completed"
		}
		m.emitEvent("agent:turn_end", map[string]interface{}{
			"sessionId":  sessionID,
			"session_id": sessionID,
			"status":     turnStatus,
			"usage":      turnUsage,
		})
	}()

	return nil
}

func (m *Manager) Cancel(sessionID string) error {
	m.mu.RLock()
	client, hasClient := m.clients[sessionID]
	session, hasSession := m.sessions[sessionID]
	m.mu.RUnlock()

	if !hasClient || !hasSession || client == nil || client.IsClosed() || session.Status == "closed" {
		return fmt.Errorf("acp: session not found: %s", sessionID)
	}

	if session.Protocol == "agy-stream" || session.Protocol == "stream-json" {
		_ = client.WriteRawLine([]byte(`{"event":"cancel"}`))
	} else if session.Protocol == "claude-stream" {
		_ = client.WriteRawLine([]byte(`{"type":"cancel"}`))
	} else if session.Protocol == "codex-appserver" {
		threadID := session.ThreadID
		if threadID == "" {
			threadID = sessionID
		}
		_ = client.SendNotification("turn/interrupt", map[string]string{
			"threadId": threadID,
		})
	} else {
		targetSessID := session.RemoteSessionID
		if targetSessID == "" {
			targetSessID = sessionID
		}
		_ = client.SendNotification(MethodSessionCancel, map[string]string{
			"sessionId":  targetSessID,
			"session_id": targetSessID,
		})
	}

	m.mu.Lock()
	session.Status = "idle"
	session.UpdatedAt = time.Now().UnixMilli()
	m.mu.Unlock()

	m.emitEvent("agent:stopped", map[string]interface{}{
		"sessionId":  sessionID,
		"session_id": sessionID,
	})

	return nil
}

func (m *Manager) RespondPermission(sessionID, requestID string, allow bool) error {
	return m.RespondPermissionOption(sessionID, requestID, allow, "")
}

func (m *Manager) RespondPermissionOption(sessionID, requestID string, allow bool, optionID string) error {
	m.mu.Lock()
	ch, ok := m.pendingDecisions[requestID]
	session, hasSession := m.sessions[sessionID]
	if ok {
		delete(m.pendingDecisions, requestID)
	}

	// Remove from pending perms list in session
	if hasSession {
		filtered := make([]*PendingPerm, 0, len(session.PendingPerms))
		for _, p := range session.PendingPerms {
			if p.RequestID != requestID {
				filtered = append(filtered, p)
			}
		}
		session.PendingPerms = filtered
		if len(filtered) == 0 && session.Status == "waiting_permission" {
			session.Status = "running"
		}
	}
	m.mu.Unlock()

	if !ok || ch == nil {
		return fmt.Errorf("acp: permission request %s not found or expired", requestID)
	}

	decision := "deny"
	if allow {
		if optionID != "" {
			decision = optionID
		} else {
			decision = "allow"
		}
	}
	ch <- decision

	return nil
}

func (m *Manager) CloseSession(sessionID string) error {
	m.mu.Lock()
	client, hasClient := m.clients[sessionID]
	session, hasSession := m.sessions[sessionID]
	delete(m.clients, sessionID)
	delete(m.activeTools, sessionID)
	if hasSession {
		session.Status = "closed"
		session.UpdatedAt = time.Now().UnixMilli()
	}
	m.mu.Unlock()

	if hasClient && client != nil {
		_ = client.Close()
	}

	m.emitEvent("agent:stopped", map[string]interface{}{
		"sessionId":  sessionID,
		"session_id": sessionID,
	})

	return nil
}

func (m *Manager) ListSessions() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		list = append(list, s)
	}
	return list
}

// GetAvailableCommands returns the list of available slash commands for an ACP session.
func (m *Manager) GetAvailableCommands(sessionID string) []AvailableCommand {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if sess, ok := m.sessions[sessionID]; ok && len(sess.AvailableCommands) > 0 {
		cmds := make([]AvailableCommand, len(sess.AvailableCommands))
		copy(cmds, sess.AvailableCommands)
		return cmds
	}
	return nil
}

func (m *Manager) handleStreamEvent(sessionID string, event string, raw []byte, initCh chan struct{}) {
	switch event {
	case "init":
		var initPayload struct {
			ConversationID string `json:"conversation_id"`
			SessionID      string `json:"session_id"`
		}
		_ = json.Unmarshal(raw, &initPayload)
		if initPayload.ConversationID != "" || initPayload.SessionID != "" {
			m.mu.Lock()
			if sess, ok := m.sessions[sessionID]; ok {
				if initPayload.ConversationID != "" {
					sess.RemoteSessionID = initPayload.ConversationID
				} else if initPayload.SessionID != "" {
					sess.RemoteSessionID = initPayload.SessionID
				}
			}
			m.mu.Unlock()
		}
		if initCh != nil {
			select {
			case initCh <- struct{}{}:
			default:
			}
		}

	case "system":
		var sysPayload struct {
			Subtype string `json:"subtype"`
		}
		_ = json.Unmarshal(raw, &sysPayload)
		if sysPayload.Subtype == "init" && initCh != nil {
			select {
			case initCh <- struct{}{}:
			default:
			}
		}

	case "step_update":
		// Google Antigravity agy
		var payload struct {
			StepUpdate struct {
				StepIndex       int                    `json:"step_index"`
				State           string                 `json:"state"`
				StepType        string                 `json:"step_type"`
				ToolName        string                 `json:"tool_name"`
				TextDelta       string                 `json:"text_delta"`
				DurationSeconds float64                `json:"duration_seconds"`
				ToolInfo        map[string]interface{} `json:"tool_info"`
			} `json:"step_update"`
		}
		if err := json.Unmarshal(raw, &payload); err == nil {
			su := payload.StepUpdate
			if su.TextDelta != "" {
				m.mu.Lock()
				m.sessionHasStreamed[sessionID] = true
				m.mu.Unlock()

				m.emitEvent("agent:message_delta", map[string]interface{}{
					"sessionId":  sessionID,
					"session_id": sessionID,
					"delta":      su.TextDelta,
					"kind":       "text",
				})
			}
			if su.StepType == "tool" {
				toolID := fmt.Sprintf("agy-tool-%d", su.StepIndex)
				toolName := su.ToolName
				if toolName == "" && su.ToolInfo != nil {
					if n, ok := su.ToolInfo["name"].(string); ok {
						toolName = n
					}
				}
				if toolName == "" {
					toolName = "tool"
				}
				var params interface{}
				if su.ToolInfo != nil {
					params = su.ToolInfo["parameters"]
				}
				if su.State == "ACTIVE" {
					m.emitEvent("agent:tool_start", map[string]interface{}{
						"sessionId":    sessionID,
						"session_id":   sessionID,
						"tool_call_id": toolID,
						"name":         toolName,
						"args":         params,
					})
				} else if su.State == "DONE" || su.State == "ERROR" {
					var resultStr string
					var isErr bool
					if su.ToolInfo != nil {
						if res, ok := su.ToolInfo["result"]; ok {
							resultBytes, _ := json.Marshal(res)
							resultStr = string(resultBytes)
						}
						if errVal, ok := su.ToolInfo["error"]; ok {
							isErr = true
							errBytes, _ := json.Marshal(errVal)
							resultStr = string(errBytes)
						}
					}
					m.emitEvent("agent:tool_end", map[string]interface{}{
						"sessionId":    sessionID,
						"session_id":   sessionID,
						"tool_call_id": toolID,
						"name":         toolName,
						"result":       resultStr,
						"is_error":     isErr || su.State == "ERROR",
					})
				}
			}
		}

	case "assistant":
		// Claude assistant message
		var claudeMsg struct {
			Message struct {
				Content []struct {
					Type       string                 `json:"type"`
					Text       string                 `json:"text,omitempty"`
					Thinking   string                 `json:"thinking,omitempty"`
					ID         string                 `json:"id,omitempty"`
					Name       string                 `json:"name,omitempty"`
					Input      map[string]interface{} `json:"input,omitempty"`
					ToolUseID  string                 `json:"tool_use_id,omitempty"`
					ContentVal interface{}            `json:"content,omitempty"`
				} `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal(raw, &claudeMsg); err == nil {
			for _, c := range claudeMsg.Message.Content {
				if c.Type == "text" && c.Text != "" {
					m.mu.Lock()
					m.sessionHasStreamed[sessionID] = true
					m.mu.Unlock()

					m.emitEvent("agent:message_delta", map[string]interface{}{
						"sessionId":  sessionID,
						"session_id": sessionID,
						"delta":      c.Text,
						"kind":       "text",
					})
				} else if c.Type == "thinking" && c.Thinking != "" {
					m.emitEvent("agent:thinking_delta", map[string]interface{}{
						"sessionId":  sessionID,
						"session_id": sessionID,
						"delta":      c.Thinking,
					})
				} else if c.Type == "tool_use" {
					m.emitEvent("agent:tool_start", map[string]interface{}{
						"sessionId":    sessionID,
						"session_id":   sessionID,
						"tool_call_id": c.ID,
						"name":         c.Name,
						"args":         c.Input,
					})
				} else if c.Type == "tool_result" {
					resStr, _ := json.Marshal(c.ContentVal)
					m.emitEvent("agent:tool_end", map[string]interface{}{
						"sessionId":    sessionID,
						"session_id":   sessionID,
						"tool_call_id": c.ToolUseID,
						"result":       string(resStr),
						"is_error":     false,
					})
				}
			}
		}

	case "message_start":
		// Claude message start
		var msgStart struct {
			Message struct {
				ID    string        `json:"id"`
				Model string        `json:"model"`
				Usage *ContextUsage `json:"usage"`
			} `json:"message"`
		}
		if err := json.Unmarshal(raw, &msgStart); err == nil {
			m.mu.Lock()
			if sess, ok := m.sessions[sessionID]; ok {
				if sess.Model == "" && msgStart.Message.Model != "" {
					sess.Model = msgStart.Message.Model
				}
				if msgStart.Message.Usage != nil {
					m.recordUsageLocked(sessionID, msgStart.Message.Usage)
				}
			}
			m.mu.Unlock()
		}

	case "message_delta":
		// Claude message delta
		var deltaPayload struct {
			Usage *ContextUsage `json:"usage"`
		}
		if err := json.Unmarshal(raw, &deltaPayload); err == nil && deltaPayload.Usage != nil {
			m.mu.Lock()
			m.recordUsageLocked(sessionID, deltaPayload.Usage)
			m.mu.Unlock()
		}

	case "content_block_delta":
		// Claude partial delta
		var deltaMsg struct {
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
		}
		if err := json.Unmarshal(raw, &deltaMsg); err == nil && deltaMsg.Delta.Text != "" {
			m.mu.Lock()
			m.sessionHasStreamed[sessionID] = true
			m.mu.Unlock()

			m.emitEvent("agent:message_delta", map[string]interface{}{
				"sessionId":  sessionID,
				"session_id": sessionID,
				"delta":      deltaMsg.Delta.Text,
				"kind":       "text",
			})
		}

	case "message", "text", "content", "delta", "chunk":
		var p map[string]interface{}
		if err := json.Unmarshal(raw, &p); err == nil {
			t := extractTextOrDelta(p)
			if t != "" {
				m.mu.Lock()
				m.sessionHasStreamed[sessionID] = true
				m.mu.Unlock()

				m.emitEvent("agent:message_delta", map[string]interface{}{
					"sessionId":  sessionID,
					"session_id": sessionID,
					"delta":      t,
					"kind":       "text",
				})
			}
		}

	case "result":
		var resPayload struct {
			SessionID string        `json:"session_id"`
			UUID      string        `json:"uuid"`
			Usage     *ContextUsage `json:"usage"`
		}
		_ = json.Unmarshal(raw, &resPayload)
		m.mu.Lock()
		if sess, ok := m.sessions[sessionID]; ok {
			sess.Status = "idle"
			sess.UpdatedAt = time.Now().UnixMilli()
			if resPayload.SessionID != "" {
				sess.RemoteSessionID = resPayload.SessionID
			} else if resPayload.UUID != "" {
				sess.RemoteSessionID = resPayload.UUID
			}
			if resPayload.Usage != nil {
				m.recordUsageLocked(sessionID, resPayload.Usage)
			}
			m.saveSessionsLocked()
		}
		turnUsage := (func() *ContextUsage {
			if sess, ok := m.sessions[sessionID]; ok {
				return sess.Usage
			}
			return nil
		})()
		m.mu.Unlock()

		m.emitEvent("agent:turn_end", map[string]interface{}{
			"sessionId":  sessionID,
			"session_id": sessionID,
			"status":     "completed",
			"usage":      turnUsage,
		})
	}
}

func (m *Manager) handleNotification(sessionID string, method string, params json.RawMessage) {
	// Codex app-server notifications
	switch method {
	case "item/agentMessage/delta":
		var deltaPayload struct {
			Delta string `json:"delta"`
		}
		if err := json.Unmarshal(params, &deltaPayload); err == nil && deltaPayload.Delta != "" {
			m.emitEvent("agent:message_delta", map[string]interface{}{
				"sessionId":  sessionID,
				"session_id": sessionID,
				"delta":      deltaPayload.Delta,
				"kind":       "text",
			})
		}
		return

	case "turn/completed":
		var turnPayload struct {
			Turn struct {
				Status     string        `json:"status"`
				Usage      *ContextUsage `json:"usage"`
				TokenUsage *ContextUsage `json:"tokenUsage"`
			} `json:"turn"`
			Usage      *ContextUsage `json:"usage"`
			TokenUsage *ContextUsage `json:"tokenUsage"`
		}
		_ = json.Unmarshal(params, &turnPayload)

		m.mu.Lock()
		if sess, ok := m.sessions[sessionID]; ok {
			sess.Status = "idle"
			sess.UpdatedAt = time.Now().UnixMilli()
		}
		u := turnPayload.Turn.Usage
		if u == nil {
			u = turnPayload.Turn.TokenUsage
		}
		if u == nil {
			u = turnPayload.Usage
		}
		if u == nil {
			u = turnPayload.TokenUsage
		}
		if u != nil {
			m.recordUsageLocked(sessionID, u)
		}
		m.saveSessionsLocked()
		turnUsage := (func() *ContextUsage {
			if sess, ok := m.sessions[sessionID]; ok {
				return sess.Usage
			}
			return nil
		})()
		m.mu.Unlock()

		m.emitEvent("agent:turn_end", map[string]interface{}{
			"sessionId":  sessionID,
			"session_id": sessionID,
			"status":     "completed",
			"usage":      turnUsage,
		})
		return

	case "item/started":
		var itemPayload struct {
			Item struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				ToolName string `json:"toolName"`
				Command  string `json:"command"`
			} `json:"item"`
		}
		if err := json.Unmarshal(params, &itemPayload); err == nil {
			if itemPayload.Item.Type == "commandExecution" || itemPayload.Item.Type == "toolCall" {
				toolName := itemPayload.Item.ToolName
				if toolName == "" {
					toolName = "bash"
				}
				m.emitEvent("agent:tool_start", map[string]interface{}{
					"sessionId":    sessionID,
					"session_id":   sessionID,
					"tool_call_id": itemPayload.Item.ID,
					"name":         toolName,
					"args":         itemPayload.Item.Command,
				})
			}
		}
		return

	case "item/completed":
		var itemPayload struct {
			Item struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				ToolName string `json:"toolName"`
				Output   string `json:"output"`
			} `json:"item"`
		}
		if err := json.Unmarshal(params, &itemPayload); err == nil {
			if itemPayload.Item.Type == "commandExecution" || itemPayload.Item.Type == "toolCall" {
				m.emitEvent("agent:tool_end", map[string]interface{}{
					"sessionId":    sessionID,
					"session_id":   sessionID,
					"tool_call_id": itemPayload.Item.ID,
					"result":       itemPayload.Item.Output,
					"is_error":     false,
				})
			}
		}
		return
	}

	lowerMethod := strings.ToLower(method)
	if lowerMethod != "session/update" && lowerMethod != "sessionupdate" && lowerMethod != "session_update" &&
		lowerMethod != "notifications/message" && lowerMethod != "notifications/progress" &&
		lowerMethod != "agent/message" && lowerMethod != "agent/message_delta" {
		return
	}

	var rawParams map[string]interface{}
	if err := json.Unmarshal(params, &rawParams); err != nil {
		return
	}

	up := rawParams
	isNested := false
	if nestedUp, ok := rawParams["update"].(map[string]interface{}); ok && nestedUp != nil {
		up = nestedUp
		isNested = true
	}

	// Usage tracking from up or rawParams
	var usageCandidate interface{}
	for _, k := range []string{"usage", "tokenUsage", "token_usage"} {
		if u, ok := up[k]; ok && u != nil {
			usageCandidate = u
			break
		}
		if u, ok := rawParams[k]; ok && u != nil {
			usageCandidate = u
			break
		}
	}
	if usageCandidate != nil {
		usageBytes, _ := json.Marshal(usageCandidate)
		var u ContextUsage
		if err := json.Unmarshal(usageBytes, &u); err == nil && (u.TotalTokens > 0 || u.InputTokens > 0 || u.OutputTokens > 0) {
			m.mu.Lock()
			m.recordUsageLocked(sessionID, &u)
			m.mu.Unlock()
		}
	}

	// Determine discriminator
	rawType := ""
	for _, k := range []string{"sessionUpdate", "session_update", "updateType", "update_type", "type", "kind", "event"} {
		if v, ok := up[k].(string); ok && v != "" {
			rawType = v
			break
		}
		if v, ok := rawParams[k].(string); ok && v != "" {
			rawType = v
			break
		}
	}

	normType := strings.ToLower(rawType)
	normType = strings.ReplaceAll(normType, "_", "")
	normType = strings.ReplaceAll(normType, "-", "")

	// 1. Available Commands
	if strings.Contains(normType, "command") {
		var cmds []AvailableCommand
		rawCmds := up["availableCommands"]
		if rawCmds == nil {
			rawCmds = up["commands"]
		}
		if rawCmds == nil {
			rawCmds = rawParams["availableCommands"]
		}
		if rawCmds == nil {
			rawCmds = rawParams["commands"]
		}
		if rawCmds != nil {
			b, _ := json.Marshal(rawCmds)
			_ = json.Unmarshal(b, &cmds)
		}
		m.mu.Lock()
		if sess, ok := m.sessions[sessionID]; ok {
			sess.AvailableCommands = cmds
		}
		m.mu.Unlock()
		m.emitEvent("agent:available_commands", map[string]interface{}{
			"sessionId":  sessionID,
			"session_id": sessionID,
			"commands":   cmds,
		})
		return
	}

	// 2. Tool Calls
	if strings.Contains(normType, "tool") {
		toolID, _ := up["toolCallId"].(string)
		if toolID == "" {
			toolID, _ = up["id"].(string)
		}
		if toolID == "" {
			if tc, ok := up["toolCall"].(map[string]interface{}); ok {
				toolID, _ = tc["id"].(string)
			}
		}
		status, _ := up["status"].(string)
		rawInput := up["rawInput"]
		if rawInput == nil {
			rawInput = up["input"]
		}
		if rawInput == nil {
			rawInput = up["arguments"]
		}
		if rawInput == nil {
			rawInput = up["args"]
		}
		if rawInput == nil {
			if fields, ok := up["fields"].(map[string]interface{}); ok {
				rawInput = fields["rawInput"]
				if rawInput == nil {
					rawInput = fields["input"]
				}
				if rawInput == nil {
					rawInput = fields["arguments"]
				}
			}
		}

		toolName, title := resolveAcpToolName(up)

		m.mu.Lock()
		sessTools := m.activeTools[sessionID]
		if sessTools == nil {
			sessTools = make(map[string]activeToolInfo)
			m.activeTools[sessionID] = sessTools
		}
		if toolID != "" {
			cached, hasCached := sessTools[toolID]
			if hasCached {
				if (toolName == "tool" || toolName == "") && cached.name != "" {
					toolName = cached.name
				}
				if title == "" && cached.title != "" {
					title = cached.title
				}
				if rawInput == nil && cached.args != nil {
					rawInput = cached.args
				}
			}
			sessTools[toolID] = activeToolInfo{
				name:  toolName,
				title: title,
				args:  rawInput,
			}
		}
		m.mu.Unlock()

		lowerStatus := strings.ToLower(status)
		if lowerStatus == "pending" || lowerStatus == "running" || lowerStatus == "in_progress" || lowerStatus == "" {
			m.emitEvent("agent:tool_start", map[string]interface{}{
				"sessionId":    sessionID,
				"session_id":   sessionID,
				"tool_call_id": toolID,
				"name":         toolName,
				"title":        title,
				"args":         rawInput,
			})
		} else if lowerStatus == "success" || lowerStatus == "completed" || lowerStatus == "error" || lowerStatus == "failed" || lowerStatus == "denied" || lowerStatus == "blocked" {
			outStr := extractContentText(up["content"])
			if outStr == "" {
				rawOutput := up["rawOutput"]
				if rawOutput == nil {
					rawOutput = up["output"]
				}
				if rawOutput == nil {
					rawOutput = up["result"]
				}
				if rawOutput == nil {
					if fields, ok := up["fields"].(map[string]interface{}); ok {
						rawOutput = fields["rawOutput"]
						if rawOutput == nil {
							rawOutput = fields["output"]
						}
						if rawOutput == nil {
							rawOutput = fields["result"]
						}
					}
				}
				if rawOutput != nil {
					if s, ok := rawOutput.(string); ok {
						outStr = s
					} else {
						b, _ := json.Marshal(rawOutput)
						outStr = string(b)
					}
				}
			}

			isErr := lowerStatus == "error" || lowerStatus == "failed" || lowerStatus == "denied" || lowerStatus == "blocked"
			m.emitEvent("agent:tool_end", map[string]interface{}{
				"sessionId":    sessionID,
				"session_id":   sessionID,
				"tool_call_id": toolID,
				"name":         toolName,
				"title":        title,
				"args":         rawInput,
				"result":       outStr,
				"is_error":     isErr,
			})

			if toolID != "" {
				m.mu.Lock()
				if st := m.activeTools[sessionID]; st != nil {
					delete(st, toolID)
				}
				m.mu.Unlock()
			}
		}
		return
	}

	// 3. Thoughts / Thinking
	if strings.Contains(normType, "thought") || strings.Contains(normType, "thinking") {
		text := extractTextOrDelta(up)
		if text == "" && isNested {
			text = extractTextOrDelta(rawParams)
		}
		if text != "" {
			m.emitEvent("agent:thinking_delta", map[string]interface{}{
				"sessionId":  sessionID,
				"session_id": sessionID,
				"delta":      text,
			})
		}
		return
	}

	// 4. Plan
	if strings.Contains(normType, "plan") {
		if planObj, ok := up["plan"].(map[string]interface{}); ok {
			if entries, ok := planObj["entries"].([]interface{}); ok && len(entries) > 0 {
				var sb strings.Builder
				sb.WriteString("📋 **Plan:**\n")
				for _, entry := range entries {
					if em, ok := entry.(map[string]interface{}); ok {
						c := extractContentText(em["content"])
						st, _ := em["status"].(string)
						icon := "⏳"
						if st == "completed" || st == "done" {
							icon = "✅"
						} else if st == "in_progress" || st == "running" {
							icon = "🔄"
						}
						sb.WriteString(fmt.Sprintf("- %s %s\n", icon, c))
					}
				}
				planStr := sb.String()
				if planStr != "" {
					m.emitEvent("agent:thinking_delta", map[string]interface{}{
						"sessionId":  sessionID,
						"session_id": sessionID,
						"delta":      planStr + "\n",
					})
				}
			}
		}
		return
	}

	// 5. Message Content / Delta
	text := extractTextOrDelta(up)
	if text == "" && isNested {
		text = extractTextOrDelta(rawParams)
	}
	if text != "" {
		m.mu.Lock()
		m.sessionHasStreamed[sessionID] = true
		m.mu.Unlock()

		m.emitEvent("agent:message_delta", map[string]interface{}{
			"sessionId":  sessionID,
			"session_id": sessionID,
			"delta":      text,
			"kind":       "text",
		})
	}
}

func (m *Manager) handleRequest(sessionID string, id interface{}, method string, params json.RawMessage) {
	if method != MethodSessionRequestPerm {
		m.mu.RLock()
		client := m.clients[sessionID]
		m.mu.RUnlock()
		if client != nil {
			_ = client.SendResponse(id, nil, &RPCError{Code: -32601, Message: "Method not found"})
		}
		return
	}

	var permParams RequestPermissionParams
	if err := json.Unmarshal(params, &permParams); err != nil {
		return
	}

	m.mu.RLock()
	session := m.sessions[sessionID]
	client := m.clients[sessionID]
	m.mu.RUnlock()

	if session == nil || client == nil {
		return
	}

	reqID := permParams.RequestID
	if reqID == "" {
		reqID = fmt.Sprintf("req-%v", id)
	}

	toolName := permParams.Tool
	var toolArgs map[string]interface{} = permParams.Arguments
	title := permParams.Title

	if permParams.ToolCall != nil {
		if toolName == "" {
			toolName = permParams.ToolCall.Title
			if toolName == "" {
				toolName = permParams.ToolCall.Kind
			}
		}
		if title == "" {
			title = permParams.ToolCall.Title
		}
		if toolArgs == nil && permParams.ToolCall.RawInput != nil {
			if mInput, ok := permParams.ToolCall.RawInput.(map[string]interface{}); ok {
				toolArgs = mInput
			} else {
				toolArgs = map[string]interface{}{"input": permParams.ToolCall.RawInput}
			}
		}
	}

	// Select best allow OptionID
	allowOptionID := "allow"
	for _, opt := range permParams.Options {
		k := strings.ToLower(opt.Kind)
		idLower := strings.ToLower(opt.OptionID)
		if k == "allow_once" || k == "allow_always" || strings.Contains(idLower, "allow") {
			allowOptionID = opt.OptionID
			break
		}
	}
	if allowOptionID == "allow" && len(permParams.Options) > 0 {
		allowOptionID = permParams.Options[0].OptionID
	}

	makeAllowResponse := func() RequestPermissionResult {
		return RequestPermissionResult{
			Decision: "allow",
			Outcome: map[string]interface{}{
				"outcome":  "selected",
				"optionId": allowOptionID,
			},
		}
	}

	makeDenyResponse := func() RequestPermissionResult {
		return RequestPermissionResult{
			Decision: "deny",
			Outcome: map[string]interface{}{
				"outcome": "cancelled",
			},
		}
	}

	// Permission auto-approval logic based on mode
	if session.PermissionMode == "full" || session.PermissionMode == "auto" {
		_ = client.SendResponse(id, makeAllowResponse(), nil)
		return
	}

	toolLower := strings.ToLower(toolName)
	if session.PermissionMode == "auto_edits" && (toolLower == "write_to_file" || toolLower == "replace_file_content" || toolLower == "edit" || strings.Contains(toolLower, "edit") || strings.Contains(toolLower, "write")) {
		_ = client.SendResponse(id, makeAllowResponse(), nil)
		return
	}

	// Supervised mode: record pending permission, update state, and wait for user approval
	decisionCh := make(chan string, 1)
	m.mu.Lock()
	m.pendingDecisions[reqID] = decisionCh
	session.Status = "waiting_permission"
	session.PendingPerms = append(session.PendingPerms, &PendingPerm{
		RequestID:   reqID,
		Title:       title,
		Description: permParams.Description,
		Tool:        toolName,
		Arguments:   toolArgs,
		CreatedAt:   time.Now().UnixMilli(),
	})
	m.mu.Unlock()

	m.emitEvent("agent:ask", map[string]interface{}{
		"sessionId":  sessionID,
		"session_id": sessionID,
		"requestId":  reqID,
		"title":      title,
		"desc":       permParams.Description,
		"tool":       toolName,
		"arguments":  toolArgs,
		"options":    permParams.Options,
	})

	go func() {
		select {
		case decision := <-decisionCh:
			if decision == "deny" {
				_ = client.SendResponse(id, makeDenyResponse(), nil)
			} else if decision == "allow" {
				_ = client.SendResponse(id, makeAllowResponse(), nil)
			} else {
				// User selected a specific option ID or custom input
				_ = client.SendResponse(id, RequestPermissionResult{
					Decision: "allow",
					Outcome: map[string]interface{}{
						"outcome":  "selected",
						"optionId": decision,
					},
				}, nil)
			}
		case <-time.After(5 * time.Minute):
			// Timeout: deny by default
			_ = client.SendResponse(id, makeDenyResponse(), nil)
			m.mu.Lock()
			delete(m.pendingDecisions, reqID)
			m.mu.Unlock()
		}
	}()
}

func (m *Manager) emitEvent(eventType events.EventType, data map[string]interface{}) {
	if m.bus != nil {
		m.bus.Publish(events.Event{
			Type: eventType,
			Data: data,
		})
	}
}

func extractContentText(content interface{}) string {
	if content == nil {
		return ""
	}
	switch v := content.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case []interface{}:
		var sb strings.Builder
		for _, item := range v {
			str := extractContentText(item)
			if str != "" {
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString(str)
			}
		}
		return sb.String()
	case map[string]interface{}:
		if t, ok := v["text"].(string); ok && t != "" {
			return t
		}
		if d, ok := v["delta"]; ok && d != nil {
			if t := extractContentText(d); t != "" {
				return t
			}
		}
		if c, ok := v["content"]; ok && c != nil {
			if t := extractContentText(c); t != "" {
				return t
			}
		}
		if val, ok := v["value"].(string); ok && val != "" {
			return val
		}
		if out, ok := v["output"].(string); ok && out != "" {
			return out
		}
		if ch, ok := v["chunk"]; ok && ch != nil {
			if t := extractContentText(ch); t != "" {
				return t
			}
		}
		if m, ok := v["message"]; ok && m != nil {
			if t := extractContentText(m); t != "" {
				return t
			}
		}
	}
	return ""
}

func extractTextOrDelta(data map[string]interface{}) string {
	if data == nil {
		return ""
	}
	for _, k := range []string{"delta", "content", "text", "chunk", "message", "value", "output", "result"} {
		if val, ok := data[k]; ok && val != nil {
			if t := extractContentText(val); t != "" {
				return t
			}
		}
	}
	return ""
}

func resolveAcpToolName(up map[string]interface{}) (name string, title string) {
	if t, _ := up["title"].(string); t != "" {
		title = t
	}
	if n, _ := up["name"].(string); n != "" {
		name = n
	} else if t, _ := up["tool"].(string); t != "" {
		name = t
	} else if t, _ := up["toolName"].(string); t != "" {
		name = t
	}

	if fields, ok := up["fields"].(map[string]interface{}); ok {
		if name == "" {
			if n, _ := fields["name"].(string); n != "" {
				name = n
			} else if t, _ := fields["tool"].(string); t != "" {
				name = t
			} else if t, _ := fields["toolName"].(string); t != "" {
				name = t
			}
		}
		if title == "" {
			if t, _ := fields["title"].(string); t != "" {
				title = t
			}
		}
	}

	kind, _ := up["kind"].(string)
	if name == "" {
		switch strings.ToLower(kind) {
		case "read":
			name = "read_file"
		case "edit":
			name = "edit_file"
		case "write", "create":
			name = "write_file"
		case "execute", "terminal", "bash", "shell":
			name = "bash"
		}
	}

	if name == "" && title != "" {
		lower := strings.ToLower(title)
		if strings.HasPrefix(lower, "read:") || strings.HasPrefix(lower, "reading ") || lower == "read" {
			name = "read_file"
		} else if strings.HasPrefix(lower, "edit:") || strings.HasPrefix(lower, "editing ") || lower == "edit" {
			name = "edit_file"
		} else if strings.HasPrefix(lower, "write:") || strings.HasPrefix(lower, "writing ") || lower == "write" {
			name = "write_file"
		} else if strings.HasPrefix(lower, "shell:") || strings.HasPrefix(lower, "terminal:") || strings.HasPrefix(lower, "exec:") {
			name = "bash"
		} else if strings.HasPrefix(lower, "search:") || strings.HasPrefix(lower, "glob:") {
			name = "grep"
		} else {
			name = title
		}
	}

	if name == "" {
		name = "tool"
	}
	return name, title
}

// GetUsageSummary aggregates usage statistics across active and historical ACP sessions.
func (m *Manager) GetUsageSummary() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	type modelAgg struct {
		model            string
		provider         string
		promptTokens     int
		completionTokens int
		cachedTokens     int
		totalTokens      int
		turns            int
		sessions         int
	}

	type providerAgg struct {
		provider         string
		promptTokens     int
		completionTokens int
		cachedTokens     int
		totalTokens      int
		models           map[string]bool
		sessions         int
	}

	var totalPrompt, totalComp, totalCached, totalTokens int
	var totalTurns int
	modelsMap := make(map[string]*modelAgg)
	providersMap := make(map[string]*providerAgg)
	sessionsList := make([]map[string]interface{}, 0, len(m.sessions))

	for _, sess := range m.sessions {
		if sess == nil {
			continue
		}

		modelName := sess.Model
		if modelName == "" {
			switch strings.ToLower(sess.AgentID) {
			case "agy", "agent-agy", "antigravity":
				modelName = "gemini-2.5-pro (agy)"
			case "claude", "agent-claude", "claude-code":
				modelName = "claude-3-7-sonnet (claude)"
			case "codex", "agent-codex":
				modelName = "o3-mini (codex)"
			case "commandcode", "agent-commandcode":
				modelName = "command-r-plus (commandcode)"
			case "omp", "agent-ohmypi":
				modelName = "pi-model (omp)"
			case "opencode", "agent-opencode":
				modelName = "opencode"
			default:
				if sess.AgentID != "" {
					modelName = sess.AgentID
				} else {
					modelName = "acp-agent"
				}
			}
		}

		providerName := "acp"
		switch strings.ToLower(sess.AgentID) {
		case "agy", "agent-agy", "antigravity":
			providerName = "google (acp)"
		case "claude", "agent-claude", "claude-code":
			providerName = "anthropic (acp)"
		case "codex", "agent-codex":
			providerName = "openai (acp)"
		case "commandcode", "agent-commandcode":
			providerName = "cohere (acp)"
		default:
			if sess.AgentID != "" {
				providerName = "acp:" + strings.TrimPrefix(sess.AgentID, "agent-")
			}
		}

		var pTokens, cTokens, cached, tTokens int
		if sess.Usage != nil {
			pTokens = sess.Usage.InputTokens
			cTokens = sess.Usage.OutputTokens
			cached = sess.Usage.CachedTokens
			tTokens = sess.Usage.TotalTokens
			if tTokens == 0 && (pTokens > 0 || cTokens > 0) {
				tTokens = pTokens + cTokens
			}
		}

		turnsCount := sess.PromptCount
		if turnsCount == 0 && tTokens > 0 {
			turnsCount = 1
		}

		totalPrompt += pTokens
		totalComp += cTokens
		totalCached += cached
		totalTokens += tTokens
		totalTurns += turnsCount

		// Model aggregation
		ma, ok := modelsMap[modelName]
		if !ok {
			ma = &modelAgg{
				model:    modelName,
				provider: providerName,
			}
			modelsMap[modelName] = ma
		}
		ma.promptTokens += pTokens
		ma.completionTokens += cTokens
		ma.cachedTokens += cached
		ma.totalTokens += tTokens
		ma.turns += turnsCount
		ma.sessions++

		// Provider aggregation
		pa, ok := providersMap[providerName]
		if !ok {
			pa = &providerAgg{
				provider: providerName,
				models:   make(map[string]bool),
			}
			providersMap[providerName] = pa
		}
		pa.promptTokens += pTokens
		pa.completionTokens += cTokens
		pa.cachedTokens += cached
		pa.totalTokens += tTokens
		pa.models[modelName] = true
		pa.sessions++

		sessName := sess.Title
		if sessName == "" {
			dirName := filepath.Base(sess.Cwd)
			if dirName == "" || dirName == "." || dirName == "/" {
				dirName = "workspace"
			}
			sessName = fmt.Sprintf("ACP · %s (%s)", sess.AgentID, dirName)
		}

		cTime := time.UnixMilli(sess.CreatedAt)
		if sess.CreatedAt == 0 {
			cTime = time.Now()
		}
		uTime := time.UnixMilli(sess.UpdatedAt)
		if sess.UpdatedAt == 0 {
			uTime = cTime
		}

		sessionsList = append(sessionsList, map[string]interface{}{
			"id":                sess.ID,
			"name":              sessName,
			"role":              "acp",
			"model":             modelName,
			"provider":          providerName,
			"prompt_tokens":     pTokens,
			"completion_tokens": cTokens,
			"cached_tokens":     cached,
			"total_tokens":      tTokens,
			"context_tokens":    0,
			"cache_hit_rate":    0.0,
			"turns":             turnsCount,
			"created_at":        cTime.Format(time.RFC3339),
			"updated_at":        uTime.Format(time.RFC3339),
			"is_acp":            true,
			"agent_id":          sess.AgentID,
		})
	}

	byModel := make([]map[string]interface{}, 0, len(modelsMap))
	for _, ma := range modelsMap {
		byModel = append(byModel, map[string]interface{}{
			"model":             ma.model,
			"provider":          ma.provider,
			"prompt_tokens":     ma.promptTokens,
			"completion_tokens": ma.completionTokens,
			"cached_tokens":     ma.cachedTokens,
			"total_tokens":      ma.totalTokens,
			"turns":             ma.turns,
			"sessions":          ma.sessions,
			"is_acp":            true,
		})
	}

	byProvider := make([]map[string]interface{}, 0, len(providersMap))
	for _, pa := range providersMap {
		modelsList := make([]string, 0, len(pa.models))
		for mdl := range pa.models {
			modelsList = append(modelsList, mdl)
		}
		byProvider = append(byProvider, map[string]interface{}{
			"provider":          pa.provider,
			"prompt_tokens":     pa.promptTokens,
			"completion_tokens": pa.completionTokens,
			"cached_tokens":     pa.cachedTokens,
			"total_tokens":      pa.totalTokens,
			"models":            modelsList,
			"sessions":          pa.sessions,
			"is_acp":            true,
		})
	}

	return map[string]interface{}{
		"totals": map[string]interface{}{
			"total_tokens":      totalTokens,
			"prompt_tokens":     totalPrompt,
			"completion_tokens": totalComp,
			"cached_tokens":     totalCached,
			"session_count":     len(m.sessions),
			"turn_count":        totalTurns,
		},
		"by_model":    byModel,
		"by_provider": byProvider,
		"by_session":  sessionsList,
	}
}
