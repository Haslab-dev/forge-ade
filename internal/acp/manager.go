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

type Manager struct {
	bus *events.Bus

	mu               sync.RWMutex
	sessions         map[string]*Session
	clients          map[string]*Client
	pendingDecisions map[string]chan string // requestID -> decision channel
}

func NewManager(bus *events.Bus) *Manager {
	return &Manager{
		bus:              bus,
		sessions:         make(map[string]*Session),
		clients:          make(map[string]*Client),
		pendingDecisions: make(map[string]chan string),
	}
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
	m.sessions[sessionID] = session
	m.clients[sessionID] = client
	m.mu.Unlock()

	client.OnClose = func(err error) {
		m.mu.Lock()
		if sess, ok := m.sessions[sessionID]; ok {
			sess.Status = "closed"
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

	isFirstTurn := session.PromptCount == 0
	session.PromptCount++

	if isFirstTurn || hasBoardKeyword {
		return prompt + "\n\n[System Context: Forge ADE Workflow Kanban Board tasks are stored at .forge/board_tasks.json. When creating, viewing, or updating tasks, read or edit this JSON file directly. Changes are monitored by a real-time file watcher.]"
	}
	return prompt
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
	session.UpdatedAt = time.Now().UnixMilli()
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
		m.mu.Unlock()

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
		decision = "allow"
	}
	ch <- decision

	return nil
}

func (m *Manager) CloseSession(sessionID string) error {
	m.mu.Lock()
	client, hasClient := m.clients[sessionID]
	session, hasSession := m.sessions[sessionID]
	delete(m.clients, sessionID)
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

	case "content_block_delta":
		// Claude partial delta
		var deltaMsg struct {
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
		}
		if err := json.Unmarshal(raw, &deltaMsg); err == nil && deltaMsg.Delta.Text != "" {
			m.emitEvent("agent:message_delta", map[string]interface{}{
				"sessionId":  sessionID,
				"session_id": sessionID,
				"delta":      deltaMsg.Delta.Text,
				"kind":       "text",
			})
		}

	case "result":
		var resPayload struct {
			SessionID string `json:"session_id"`
			UUID      string `json:"uuid"`
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
		}
		m.mu.Unlock()

		m.emitEvent("agent:turn_end", map[string]interface{}{
			"sessionId":  sessionID,
			"session_id": sessionID,
			"status":     "completed",
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
		m.mu.Lock()
		if sess, ok := m.sessions[sessionID]; ok {
			sess.Status = "idle"
			sess.UpdatedAt = time.Now().UnixMilli()
		}
		m.mu.Unlock()

		m.emitEvent("agent:turn_end", map[string]interface{}{
			"sessionId":  sessionID,
			"session_id": sessionID,
			"status":     "completed",
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

	if method != MethodSessionUpdate {
		return
	}

	// 1. Try standard ACP session/update (params.update object from ACP SDK e.g. omp, pi-acp)
	var rawNotification struct {
		SessionID string                 `json:"sessionId"`
		Update    map[string]interface{} `json:"update"`
	}
	if err := json.Unmarshal(params, &rawNotification); err == nil && rawNotification.Update != nil {
		up := rawNotification.Update
		sessionUpdate, _ := up["sessionUpdate"].(string)
		switch sessionUpdate {
		case "agent_message_chunk", "agent_message_delta":
			var text string
			if content, ok := up["content"].(map[string]interface{}); ok {
				if t, ok := content["text"].(string); ok {
					text = t
				} else if c, ok := content["content"].(string); ok {
					text = c
				}
			} else if s, ok := up["content"].(string); ok {
				text = s
			} else if delta, ok := up["delta"].(map[string]interface{}); ok {
				if c, ok := delta["content"].(string); ok {
					text = c
				} else if t, ok := delta["text"].(string); ok {
					text = t
				}
			} else if s, ok := up["delta"].(string); ok {
				text = s
			} else if s, ok := up["text"].(string); ok {
				text = s
			}
			if text != "" {
				m.emitEvent("agent:message_delta", map[string]interface{}{
					"sessionId":  sessionID,
					"session_id": sessionID,
					"delta":      text,
					"kind":       "text",
				})
			}
		case "agent_thought_chunk", "agent_thought_delta":
			var text string
			if content, ok := up["content"].(map[string]interface{}); ok {
				if t, ok := content["text"].(string); ok {
					text = t
				}
			} else if s, ok := up["content"].(string); ok {
				text = s
			} else if delta, ok := up["delta"].(map[string]interface{}); ok {
				if c, ok := delta["content"].(string); ok {
					text = c
				} else if t, ok := delta["text"].(string); ok {
					text = t
				}
			} else if s, ok := up["delta"].(string); ok {
				text = s
			}
			if text != "" {
				m.emitEvent("agent:thinking_delta", map[string]interface{}{
					"sessionId":  sessionID,
					"session_id": sessionID,
					"delta":      text,
				})
			}
		case "tool_call", "tool_call_update":
			toolID, _ := up["toolCallId"].(string)
			if toolID == "" {
				toolID, _ = up["id"].(string)
			}
			title, _ := up["title"].(string)
			status, _ := up["status"].(string)
			rawInput := up["rawInput"]
			if rawInput == nil {
				rawInput = up["input"]
			}
			rawOutput := up["rawOutput"]
			if rawOutput == nil {
				rawOutput = up["output"]
			}
			if status == "pending" || status == "running" || status == "" {
				m.emitEvent("agent:tool_start", map[string]interface{}{
					"sessionId":    sessionID,
					"session_id":   sessionID,
					"tool_call_id": toolID,
					"name":         title,
					"args":         rawInput,
				})
			} else if status == "success" || status == "error" || status == "completed" {
				outStr := ""
				if rawOutput != nil {
					b, _ := json.Marshal(rawOutput)
					outStr = string(b)
				}
				m.emitEvent("agent:tool_end", map[string]interface{}{
					"sessionId":    sessionID,
					"session_id":   sessionID,
					"tool_call_id": toolID,
					"name":         title,
					"result":       outStr,
					"is_error":     status == "error",
				})
			}
		}
		return
	}

	var update SessionUpdateParams
	if err := json.Unmarshal(params, &update); err != nil {
		return
	}

	m.mu.Lock()
	session, ok := m.sessions[sessionID]
	if ok && update.Usage != nil {
		session.Usage = update.Usage
	}
	m.mu.Unlock()

	switch update.UpdateType {
	case "content":
		m.emitEvent("agent:message_delta", map[string]interface{}{
			"sessionId":  sessionID,
			"session_id": sessionID,
			"delta":      update.Content,
			"kind":       "text",
		})
	case "thinking":
		m.emitEvent("agent:thinking_delta", map[string]interface{}{
			"sessionId":  sessionID,
			"session_id": sessionID,
			"delta":      update.Thinking,
		})
	case "tool_call":
		if update.ToolCall != nil {
			eventType := "agent:tool_delta"
			isError := false
			if update.ToolCall.Status == "pending" || update.ToolCall.Status == "running" {
				eventType = "agent:tool_start"
			} else if update.ToolCall.Status == "success" || update.ToolCall.Status == "error" {
				eventType = "agent:tool_end"
				if update.ToolCall.Status == "error" {
					isError = true
				}
			}
			toolName := update.ToolCall.Name
			if toolName == "" {
				toolName = update.ToolCall.Title
			}
			m.emitEvent(events.EventType(eventType), map[string]interface{}{
				"sessionId":    sessionID,
				"session_id":   sessionID,
				"tool_call_id": update.ToolCall.ID,
				"name":         toolName,
				"args":         update.ToolCall.Input,
				"result":       update.ToolCall.Output,
				"is_error":     isError,
				"tool":         update.ToolCall,
			})
		}
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
	})

	go func() {
		select {
		case decision := <-decisionCh:
			if decision == "allow" {
				_ = client.SendResponse(id, makeAllowResponse(), nil)
			} else {
				_ = client.SendResponse(id, makeDenyResponse(), nil)
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
