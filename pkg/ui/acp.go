package ui

import (
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/Haslab-dev/RakitKode/events"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/internal/acp"
)

// ACPAgentDefinition defines metadata and CLI execution parameters for an ACP coding agent.
type ACPAgentDefinition struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Type             string   `json:"type"` // "internal", "acp", "claude", "codex", "opencode", "commandcode", "pi", "ohmypi", "gemini"
	Description      string   `json:"description"`
	Command          string   `json:"command"`
	Args             []string `json:"args"`
	Model            string   `json:"model"`
	SupportedModels  []string `json:"supportedModels"`
	DefaultEffort    string   `json:"defaultEffort"`
	SupportedEfforts []string `json:"supportedEfforts"`
	Available        bool     `json:"available"` // Detected on PATH
	IsDefault        bool     `json:"isDefault"`
}

// PermissionRequest represents a pending permission prompt from an ACP agent.
type PermissionRequest struct {
	SessionID   string `json:"sessionId"`
	RequestID   string `json:"requestId"`
	ToolName    string `json:"toolName"`
	Description string `json:"description"`
	Command     string `json:"command,omitempty"`
	Path        string `json:"path,omitempty"`
}

// ACPManager integrates ForgeADE's internal ACP JSON-RPC manager with the native UI.
type ACPManager struct {
	mu            sync.RWMutex
	manager       *acp.Manager
	bus           *events.Bus
	Agents        []ACPAgentDefinition
	ActiveReq     *PermissionRequest
	OnTokenChunk  func(sessionID, text string)
	OnToolStep    func(sessionID, toolName, status string)
	OnTurnDone    func(sessionID string)
	OnNeedRedraw  func()
}

// NewACPManager creates and initializes the native ACP integration.
func NewACPManager(engine *Engine, onRedraw func()) *ACPManager {
	mgr := &ACPManager{
		manager:      engine.ACPMgr,
		bus:          engine.Bus,
		OnNeedRedraw: onRedraw,
		Agents:       defaultACPAgentDefinitions(),
	}

	mgr.RefreshAvailability()
	mgr.subscribeToBusEvents()

	return mgr
}

// defaultACPAgentDefinitions registers the official ForgeADE ACP agent ecosystem.
func defaultACPAgentDefinitions() []ACPAgentDefinition {
	return []ACPAgentDefinition{
		{
			ID:          "agent-internal",
			Name:        "ForgeADE Internal",
			Type:        "internal",
			Description: "High-throughput autonomous engine with multi-turn planning, AST parsing, and direct LLM provider reasoning.",
			Command:     "",
			Model:       "Claude 3.7 Sonnet / gpt-4o",
			SupportedModels: []string{
				"Claude 3.7 Sonnet",
				"gpt-4o",
				"Gemini 2.5 Pro",
				"DeepSeek R1",
			},
			DefaultEffort:    "high",
			SupportedEfforts: []string{"low", "medium", "high", "max"},
			Available:        true,
			IsDefault:        true,
		},
		{
			ID:          "agent-agy",
			Name:        "Google Antigravity (AGY)",
			Type:        "acp",
			Description: "Official Google Antigravity autonomous coding agent harness connected via ACP JSON-RPC protocol.",
			Command:     "agy",
			Args:        []string{"--input-format", "stream-json", "--output-format", "stream-json"},
			Model:       "gemini-3.8-flash-high",
			SupportedModels: []string{
				"gemini-3.8-flash-high",
				"gemini-3.8-flash-medium",
				"claude-sonnet-4-6",
				"gpt-oss-120b-medium",
			},
			DefaultEffort:    "high",
			SupportedEfforts: []string{"low", "medium", "high", "max"},
		},
		{
			ID:          "agent-claude",
			Name:        "Anthropic Claude Code",
			Type:        "claude",
			Description: "Anthropic Claude Code CLI harness driving local shell, filesystem tools, and reasoning over stream-json.",
			Command:     "claude",
			Args:        []string{"-p", "--input-format", "stream-json", "--output-format", "stream-json", "--verbose"},
			Model:       "claude-3-7-sonnet",
			SupportedModels: []string{
				"claude-3-7-sonnet",
				"claude-3-5-sonnet",
				"claude-3-5-haiku",
				"claude-3-opus",
			},
			DefaultEffort:    "high",
			SupportedEfforts: []string{"low", "medium", "high", "xhigh"},
		},
		{
			ID:          "agent-codex",
			Name:        "OpenAI Codex",
			Type:        "codex",
			Description: "OpenAI Codex agent harness driven through JSON-RPC 2.0 app-server over stdio.",
			Command:     "codex",
			Args:        []string{"app-server", "--listen", "stdio://"},
			Model:       "gpt-5.6-terra",
			SupportedModels: []string{
				"gpt-5.6-terra",
				"gpt-5.6-luna",
				"deepseek-v4-1-flash",
			},
			DefaultEffort:    "medium",
			SupportedEfforts: []string{"low", "medium", "high"},
		},
		{
			ID:          "agent-opencode",
			Name:        "OpenCode",
			Type:        "opencode",
			Description: "OpenCode autonomous coding assistant connected over Agent Client Protocol (ACP).",
			Command:     "opencode",
			Args:        []string{"acp"},
			Model:       "opencode/claude-sonnet-5-5",
			SupportedModels: []string{
				"opencode/claude-sonnet-5-5",
				"deepseek/deepseek-flash",
				"Kenari/glm-5-3-flash",
			},
			DefaultEffort:    "medium",
			SupportedEfforts: []string{"low", "medium", "high"},
		},
		{
			ID:          "agent-commandcode",
			Name:        "Command Code",
			Type:        "commandcode",
			Description: "Command Code autonomous agent with taste learning connected via Agent Client Protocol (ACP).",
			Command:     "cmd",
			Args:        []string{"acp"},
			Model:       "deepseek/deepseek-v4.1-flash",
			SupportedModels: []string{
				"deepseek/deepseek-v4.1-flash",
				"claude-sonnet-5-5",
				"google/gemini-3.8-flash",
			},
			DefaultEffort:    "medium",
			SupportedEfforts: []string{"low", "medium", "high", "max"},
		},
		{
			ID:          "agent-pi",
			Name:        "Pi Agent",
			Type:        "pi",
			Description: "Autonomous core agent implementing the Pi protocol with codebase research and live terminal orchestration.",
			Command:     "pi-acp",
			Args:        []string{},
			Model:       "Kenari/glm-5-3-flash",
			SupportedModels: []string{
				"Kenari/glm-5-3-flash",
				"Kenari/claude-haiku-5-5",
			},
			DefaultEffort:    "medium",
			SupportedEfforts: []string{"minimal", "low", "medium", "high"},
		},
		{
			ID:          "agent-ohmypi",
			Name:        "Oh My Pi (omp)",
			Type:        "ohmypi",
			Description: "High-throughput agent fork with multi-turn reasoning and parallel tool execution.",
			Command:     "omp",
			Args:        []string{"acp"},
			Model:       "omp/deepseek-v4-pro",
			SupportedModels: []string{
				"omp/deepseek-v4-pro",
				"Kenari/glm-5-3-flash",
			},
			DefaultEffort:    "medium",
			SupportedEfforts: []string{"low", "medium", "high"},
		},
		{
			ID:          "agent-gemini",
			Name:        "Google Gemini CLI",
			Type:        "gemini",
			Description: "Official Google Gemini coding CLI with direct multimodal reasoning.",
			Command:     "gemini",
			Args:        []string{"--acp"},
			Model:       "gemini-2.5-pro",
			SupportedModels: []string{
				"gemini-2.5-pro",
				"gemini-2.5-flash",
			},
			DefaultEffort:    "high",
			SupportedEfforts: []string{"low", "medium", "high"},
		},
	}
}

// RefreshAvailability detects which agent CLIs are installed and on the user's PATH.
func (mgr *ACPManager) RefreshAvailability() {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	for i := range mgr.Agents {
		if mgr.Agents[i].Type == "internal" {
			mgr.Agents[i].Available = true
			continue
		}
		if mgr.Agents[i].Command != "" {
			_, err := exec.LookPath(mgr.Agents[i].Command)
			mgr.Agents[i].Available = (err == nil)
		}
	}
}

// StartSession launches an ACP agent session.
func (mgr *ACPManager) StartSession(agentID, cwd string, model string) (*acp.Session, error) {
	mgr.mu.Lock()
	var targetDef *ACPAgentDefinition
	for i := range mgr.Agents {
		if mgr.Agents[i].ID == agentID {
			targetDef = &mgr.Agents[i]
			break
		}
	}
	mgr.mu.Unlock()

	if targetDef == nil {
		return nil, fmt.Errorf("unknown agent ID: %s", agentID)
	}

	if targetDef.Type == "internal" {
		// Internal agent is handled in-process
		return &acp.Session{
			ID:        fmt.Sprintf("sess-int-%d", time.Now().UnixNano()%100000),
			Title:     targetDef.Name,
			AgentID:   targetDef.ID,
			Cwd:       cwd,
			Model:     model,
			Status:    "idle",
			CreatedAt: time.Now().Unix(),
		}, nil
	}

	opts := acp.SessionOpts{
		AgentID:        targetDef.ID,
		Command:        targetDef.Command,
		Args:           targetDef.Args,
		Cwd:            cwd,
		Model:          model,
		PermissionMode: "supervised",
	}

	if mgr.manager != nil {
		return mgr.manager.StartSession(opts)
	}
	return nil, fmt.Errorf("acp manager not initialized")
}

// Prompt sends a user turn to an active ACP session.
func (mgr *ACPManager) Prompt(sessionID, text string) error {
	if mgr.manager != nil {
		return mgr.manager.Prompt(sessionID, text)
	}
	return fmt.Errorf("acp manager not initialized")
}

// Cancel stops the currently running turn in the ACP session.
func (mgr *ACPManager) Cancel(sessionID string) error {
	if mgr.manager != nil {
		return mgr.manager.Cancel(sessionID)
	}
	return nil
}

// RespondPermission responds to an interactive tool execution request.
func (mgr *ACPManager) RespondPermission(sessionID, requestID string, allow bool) error {
	mgr.mu.Lock()
	mgr.ActiveReq = nil
	mgr.mu.Unlock()

	if mgr.manager != nil {
		return mgr.manager.RespondPermission(sessionID, requestID, allow)
	}
	return nil
}

// subscribeToBusEvents listens to ACP events on the event bus.
func (mgr *ACPManager) subscribeToBusEvents() {
	if mgr.bus == nil {
		return
	}

	mgr.bus.Subscribe(events.AgentMessageDelta, func(e events.Event) {
		if e.Data == nil {
			return
		}
		sessionID, _ := e.Data["sessionId"].(string)
		text, _ := e.Data["text"].(string)
		if sessionID != "" && text != "" && mgr.OnTokenChunk != nil {
			mgr.OnTokenChunk(sessionID, text)
		}
	})

	mgr.bus.Subscribe(events.AgentPermission, func(e events.Event) {
		if e.Data == nil {
			return
		}
		reqID, _ := e.Data["requestId"].(string)
		sessID, _ := e.Data["sessionId"].(string)
		tool, _ := e.Data["toolName"].(string)
		desc, _ := e.Data["description"].(string)
		cmd, _ := e.Data["command"].(string)
		path, _ := e.Data["path"].(string)
		if reqID != "" {
			mgr.mu.Lock()
			mgr.ActiveReq = &PermissionRequest{
				SessionID:   sessID,
				RequestID:   reqID,
				ToolName:    tool,
				Description: desc,
				Command:     cmd,
				Path:        path,
			}
			mgr.mu.Unlock()
			if mgr.OnNeedRedraw != nil {
				mgr.OnNeedRedraw()
			}
		}
	})
}

// RenderPermissionModal renders the ACP security approval dialog.
func (mgr *ACPManager) RenderPermissionModal(c *ui.Context) {
	mgr.mu.RLock()
	req := mgr.ActiveReq
	mgr.mu.RUnlock()

	if req == nil {
		return
	}

	t := c.Theme()
	open := true

	ui.Modal(c, &open, func() {
		ui.Column(c).Width(440).Radius(12).Background(t.Surface).BorderWidth(1).BorderColor(t.Border).Padding(16, 20).Gap(12).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				TablerIcon(c, TablerShieldAlert, 18)
				ui.Text(c, "Permission Requested").Bold().FontSize(15)
			})
			ui.Text(c, fmt.Sprintf("Agent requested permission to execute: %s", req.ToolName)).FontSize(12).TextColor(t.TextMuted)

			if req.Command != "" {
				ui.Row(c).Padding(8, 12).Radius(6).Background(t.Background).Children(func() {
					ui.Text(c, req.Command).FontSize(11).Bold()
				})
			}
			if req.Path != "" {
				ui.Text(c, fmt.Sprintf("Target Path: %s", req.Path)).FontSize(11).TextColor(t.TextMuted)
			}

			ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Box(c).Grow(1)
				if ui.Button(c, "Deny").Clicked() {
					_ = mgr.RespondPermission(req.SessionID, req.RequestID, false)
				}
				if ui.PrimaryButton(c, "Allow").Clicked() {
					_ = mgr.RespondPermission(req.SessionID, req.RequestID, true)
				}
			})
		})
	})
}
