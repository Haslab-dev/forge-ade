package agent

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Haslab-dev/RakitKode/events"
	"github.com/hasdev/forge-ade/app/native/pkg/engine"
	"github.com/hasdev/forge-ade/core/acp"
	internalgit "github.com/hasdev/forge-ade/core/git"
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

// PermissionOption is one choice of an ACP permission request.
type PermissionOption struct {
	OptionID string `json:"optionId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"` // "allow_once", "allow_always", "reject_once", ...
}

// PermissionRequest represents a pending permission prompt from an ACP agent,
// as emitted by forge-ade's internal manager on the "agent:ask" event.
type PermissionRequest struct {
	SessionID   string             `json:"sessionId"`
	RequestID   string             `json:"requestId"`
	ToolName    string             `json:"toolName"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Arguments   string             `json:"arguments,omitempty"`
	Options     []PermissionOption `json:"options,omitempty"`
}

// ToolEvent is one ACP tool call update (forge-ade emits agent:tool_start /
// agent:tool_end with these payload fields).
type ToolEvent struct {
	SessionID  string
	ToolCallID string
	Name       string
	Args       string
	Status     string // "running", "completed", "failed"
	Output     string
	IsError    bool
}

// TurnEndInfo carries the agent:turn_end payload fields.
type TurnEndInfo struct {
	Status          string // "completed", "stopped", "closed", "error", ...
	Error           string
	TotalSteps      int
	StepCount       int
	RemoteSessionID string
}

// UsageInfo carries the token usage of one turn (agent:usage / turn_end).
type UsageInfo struct {
	InputTokens  int
	OutputTokens int
	CachedTokens int
	TotalTokens  int
	CostUSD      float64
}

// ACPManager integrates ForgeADE's internal ACP JSON-RPC manager with the native UI.
type ACPManager struct {
	mu             sync.RWMutex
	manager        *acp.Manager
	bus            *events.Bus
	Agents         []ACPAgentDefinition
	ActiveReq      *PermissionRequest
	activeReqs     map[string]*PermissionRequest
	OnTokenChunk   func(sessionID, text string)
	OnThoughtChunk func(sessionID, text string)
	OnToolEvent    func(ToolEvent)
	OnTurnStart    func(sessionID, prompt string)
	OnTurnDone     func(sessionID string, info TurnEndInfo)
	OnAsk          func(*PermissionRequest)
	OnQuestions    func(sessionID string, questions []AgentQuestion)
	OnUsage        func(sessionID string, usage UsageInfo)
	OnCommands     func(sessionID string, commands []string)
	OnNeedRedraw   func()
	eventQueue     chan func()

	// Backpressure bookkeeping. eventQueue is bounded; postEvent NEVER blocks
	// on it: the publishers run on the agent streaming callbacks (which hold
	// the RakitKode manager lock per token delta), so a blocking send here
	// deadlocks the whole engine once the consumer stalls (P0-1).
	droppedEvents atomic.Int64
	lastDropLog   atomic.Int64 // unix nano of last saturation log (rate limit)

	// Coalescing for streaming deltas: a burst of token deltas for one
	// session occupies at most one queue slot at a time (postCoalesced).
	coalesceMu    sync.Mutex
	pendingChunks map[string]*deltaChunk

	// Queue lifecycle (Close): sends are guarded by queueMu so closing the
	// channel can never race a publisher into a send-on-closed panic.
	queueMu   sync.RWMutex
	queueDead bool
	closeOnce sync.Once
}

// deltaChunk accumulates same-kind deltas for one session until the queued
// drain closure runs on the consumer goroutine.
type deltaChunk struct {
	mu     sync.Mutex
	buf    string
	queued bool
}

// NewACPManager creates and initializes the native ACP integration.
func NewACPManager(engine *engine.Engine, onRedraw func()) *ACPManager {
	mgr := newACPManagerBase(engine, onRedraw)
	mgr.RefreshAvailability()
	return mgr
}

// NewACPManagerLazy creates the native ACP integration and probes CLI binary
// availability asynchronously so UI startup never blocks on filesystem scans.
func NewACPManagerLazy(engine *engine.Engine, onRedraw func()) *ACPManager {
	mgr := newACPManagerBase(engine, onRedraw)
	go func() {
		mgr.RefreshAvailability()
		if mgr.OnNeedRedraw != nil {
			mgr.OnNeedRedraw()
		}
	}()
	return mgr
}

func newACPManagerBase(engine *engine.Engine, onRedraw func()) *ACPManager {
	mgr := &ACPManager{
		manager:      engine.ACPMgr,
		bus:          engine.Bus,
		activeReqs:   make(map[string]*PermissionRequest),
		OnNeedRedraw: onRedraw,
		Agents:       defaultACPAgentDefinitions(),
		eventQueue:   make(chan func(), 4096),
	}
	go mgr.processEventQueue()
	mgr.subscribeToBusEvents()
	return mgr
}

func (mgr *ACPManager) processEventQueue() {
	for fn := range mgr.eventQueue {
		// A panicking handler must kill neither the consumer loop nor the
		// app (P1-1): log and keep draining.
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[acp] event handler panicked: %v\n%s", r, debugStack())
				}
			}()
			fn()
		}()
	}
}

// debugStack returns the current goroutine stack for panic logs.
func debugStack() string {
	buf := make([]byte, 8192)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// noteDropped records a dropped event and logs saturation at most once a
// minute. Dropping is always preferable to blocking: the publisher holds the
// agent engine's streaming lock.
func (mgr *ACPManager) noteDropped() {
	n := mgr.droppedEvents.Add(1)
	now := time.Now().UnixNano()
	last := mgr.lastDropLog.Load()
	if now-last > int64(time.Minute) && mgr.lastDropLog.CompareAndSwap(last, now) {
		log.Printf("[acp] event queue saturated: dropped %d events total (streaming continues, stale UI events discarded)", n)
	}
}

func (mgr *ACPManager) postEvent(fn func()) {
	if mgr.eventQueue == nil {
		fn()
		return
	}
	// Non-blocking send: a full queue must NEVER stall the publisher
	// goroutine — RakitKode publishes token deltas inline while holding its
	// manager lock, and a blocked publisher deadlocks every as.mu→RK.mu
	// caller (the verified P0-1 three-way deadlock).
	mgr.queueMu.RLock()
	if mgr.queueDead {
		mgr.queueMu.RUnlock()
		return
	}
	select {
	case mgr.eventQueue <- fn:
	default:
		mgr.noteDropped()
	}
	mgr.queueMu.RUnlock()
}

// Close stops the event consumer goroutine. Events published after Close are
// dropped. Safe to call multiple times and from any goroutine.
func (mgr *ACPManager) Close() {
	if mgr == nil {
		return
	}
	mgr.closeOnce.Do(func() {
		if mgr.eventQueue == nil {
			return
		}
		mgr.queueMu.Lock()
		mgr.queueDead = true
		close(mgr.eventQueue)
		mgr.queueMu.Unlock()
	})
}

// postDeltaCoalesced merges rapid same-kind streaming deltas for a session:
// while a drain closure for (kind, session) is queued, further deltas just
// append to its buffer, so a burst of N chunks costs one queue slot instead
// of N. Text is never lost — the closure delivers everything accumulated by
// the time it runs, in order (the queue is FIFO).
func (mgr *ACPManager) postDeltaCoalesced(sessionID, kind, text string, deliver func(sessionID, text string)) {
	if deliver == nil || sessionID == "" || text == "" {
		return
	}
	key := kind + ":" + sessionID
	mgr.coalesceMu.Lock()
	if mgr.pendingChunks == nil {
		mgr.pendingChunks = make(map[string]*deltaChunk)
	}
	ch := mgr.pendingChunks[key]
	if ch == nil {
		ch = &deltaChunk{}
		mgr.pendingChunks[key] = ch
	}
	ch.mu.Lock()
	ch.buf += text
	alreadyQueued := ch.queued
	ch.queued = true
	ch.mu.Unlock()
	mgr.coalesceMu.Unlock()

	if alreadyQueued {
		return
	}

	for attempt := 0; ; attempt++ {
		mgr.queueMu.RLock()
		if mgr.queueDead {
			mgr.queueMu.RUnlock()
			ch.mu.Lock()
			ch.queued = false
			ch.mu.Unlock()
			return
		}
		select {
		case mgr.eventQueue <- func() {
			ch.mu.Lock()
			buf := ch.buf
			ch.buf = ""
			ch.queued = false
			ch.mu.Unlock()
			if buf != "" {
				deliver(sessionID, buf)
			}
		}:
			mgr.queueMu.RUnlock()
			return
		default:
			mgr.queueMu.RUnlock()
		}
		if attempt >= 2 {
			// Queue still full after a few yields: release the queued claim
			// (the buffered text stays and is flushed by the next delta's
			// drain) and count the drop. Never block or spin the publisher.
			ch.mu.Lock()
			ch.queued = false
			ch.mu.Unlock()
			mgr.noteDropped()
			return
		}
		runtime.Gosched()
	}
}

// defaultACPAgentDefinitions registers the official ForgeADE ACP agent ecosystem.
func defaultACPAgentDefinitions() []ACPAgentDefinition {
	return []ACPAgentDefinition{
		{
			ID:               "agent-internal",
			Name:             "ForgeADE Internal",
			Type:             "internal",
			Description:      "High-throughput autonomous engine with multi-turn planning, AST parsing, and direct LLM provider reasoning.",
			Command:          "",
			Model:            "",
			SupportedModels:  []string{},
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
				"gemini-3.8-flash-low",
				"claude-sonnet-4-6",
				"claude-opus-4-6-thinking",
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
			Args:        []string{"-p", "--input-format", "stream-json", "--output-format", "stream-json", "--verbose", "--include-partial-messages"},
			Model:       "sonnet",
			SupportedModels: []string{
				"sonnet",
				"glm-5.3-flash",
				"opus",
				"haiku",
				"claude-opus-5-5",
				"claude-fable-5-1[1m]",
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
			Model:       "deepseek-v4-1-flash",
			SupportedModels: []string{
				"deepseek-v4-1-flash",
				"gpt-6.1-sol",
				"gpt-6-luna",
				"gpt-5.6-terra",
				"gpt-5.6-luna",
				"gpt-5.5",
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
			Model:       "Kenari/glm-5-3-flash",
			SupportedModels: []string{
				"Kenari/glm-5-3-flash",
				"Kenari/deepseek-v4-1-flash",
				"Kenari/claude-haiku-5-5",
				"Kenari/gpt-6-luna",
				"inadigital/inadigital-flash",
				"inadigital/inadigital-pro",
				"deepseek/deepseek-flash",
				"deepseek/deepseek-v4-pro",
				"opencode/claude-sonnet-5-5",
				"opencode/deepseek-v4.1-flash",
				"opencode/gpt-6-astra",
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
				"deepseek/deepseek-v4-flash",
				"claude-sonnet-5-5",
				"claude-opus-5-5",
				"gpt-6-astra",
				"gpt-6.1-sol",
				"google/gemini-3.8-flash",
				"qwen/qwen3.8-max",
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
				"Kenari/mimo-v2-5",
				"Kenari/deepseek-v4-1-flash",
				"Kenari/gpt-6-luna",
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
			Model:       "kenari/gpt-6-luna",
			SupportedModels: []string{
				"kenari/gpt-6-luna",
				"kenari/claude-haiku-5-5",
				"kenari/mimo-v2.6-flash",
				"kenari/glm-5.3-flash",
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
				"gemini-3.8-flash-high",
			},
			DefaultEffort:    "high",
			SupportedEfforts: []string{"low", "medium", "high"},
		},
	}
}

func normalizeACPAgentLookupID(agentID string) string {
	switch strings.ToLower(strings.TrimSpace(agentID)) {
	case "agy", "antigravity", "agent-agy":
		return "agent-agy"
	case "claude", "claude-code", "agent-claude":
		return "agent-claude"
	case "codex", "agent-codex":
		return "agent-codex"
	case "opencode", "agent-opencode":
		return "agent-opencode"
	case "commandcode", "cmd", "agent-commandcode":
		return "agent-commandcode"
	case "pi", "pi-acp", "agent-pi":
		return "agent-pi"
	case "omp", "ohmypi", "agent-ohmypi":
		return "agent-ohmypi"
	case "gemini", "agent-gemini":
		return "agent-gemini"
	default:
		return strings.TrimSpace(agentID)
	}
}

func findAgentInCatalog(agents []ACPAgentDefinition, agentID string) *ACPAgentDefinition {
	norm := normalizeACPAgentLookupID(agentID)
	for i := range agents {
		if strings.EqualFold(agents[i].ID, agentID) || strings.EqualFold(agents[i].ID, norm) || strings.EqualFold(agents[i].Type, norm) {
			cp := agents[i]
			cp.Args = append([]string(nil), agents[i].Args...)
			cp.SupportedModels = append([]string(nil), agents[i].SupportedModels...)
			cp.SupportedEfforts = append([]string(nil), agents[i].SupportedEfforts...)
			return &cp
		}
	}
	return nil
}

func parseEffortLevel(s string) ReasoningEffort {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "off", "minimal":
		return EffortOff
	case "low":
		return EffortLow
	case "medium", "med":
		return EffortMedium
	case "high", "xhigh":
		return EffortHigh
	case "max":
		return EffortMax
	default:
		return ""
	}
}

func resolveAgentDefaults(ag *ACPAgentDefinition, agentID string) (model string, agentType string, effort ReasoningEffort) {
	if ag != nil {
		model = ag.Model
		if model == "" && len(ag.SupportedModels) > 0 {
			model = ag.SupportedModels[0]
		}
		agentType = ag.Type
		if agentType == "" {
			if ag.ID == "agent-internal" {
				agentType = "internal"
			} else {
				agentType = "acp"
			}
		}
		effort = parseEffortLevel(ag.DefaultEffort)
		return model, agentType, effort
	}
	switch strings.ToLower(strings.TrimSpace(agentID)) {
	case "agent-kimi", "kimi":
		return "kimi-k2", "acp", EffortMedium
	case "agent-qwen", "qwen":
		return "qwen3-coder-plus", "acp", EffortMedium
	}
	return "", "", ""
}

// FindAgent returns a copy of the ACPAgentDefinition matching agentID, or nil.
func (mgr *ACPManager) FindAgent(agentID string) *ACPAgentDefinition {
	if mgr == nil {
		return nil
	}
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	return findAgentInCatalog(mgr.Agents, agentID)
}

// ResolveDefaultModel returns the default model, agentType, and default effort for an agentID.
func (mgr *ACPManager) ResolveDefaultModel(agentID string) (model string, agentType string, effort ReasoningEffort) {
	if mgr != nil {
		if ag := mgr.FindAgent(agentID); ag != nil {
			return resolveAgentDefaults(ag, agentID)
		}
	}
	return DefaultModelForACPAgentWithType(agentID)
}

// EnsureValidModel returns currentModel if supported by agentID, or agentID's default model otherwise.
func (mgr *ACPManager) EnsureValidModel(agentID, currentModel string) string {
	ag := (*ACPAgentDefinition)(nil)
	if mgr != nil {
		ag = mgr.FindAgent(agentID)
	}
	if ag == nil {
		ag = findAgentInCatalog(defaultACPAgentDefinitions(), agentID)
	}
	return ensureModelForDefinition(ag, agentID, currentModel)
}

// DefaultModelForACPAgentWithType resolves the default model, agent type, and reasoning effort
// from the static ACP catalog.
func DefaultModelForACPAgentWithType(agentID string) (model string, agentType string, effort ReasoningEffort) {
	ag := findAgentInCatalog(defaultACPAgentDefinitions(), agentID)
	return resolveAgentDefaults(ag, agentID)
}

// DefaultModelForACPAgent resolves the default model and reasoning effort for an ACP agent ID.
func DefaultModelForACPAgent(agentID string) (model string, effort ReasoningEffort) {
	m, _, ef := DefaultModelForACPAgentWithType(agentID)
	return m, ef
}

// EnsureValidModelForACPAgent returns currentModel if supported by agentID in the default ACP
// catalog, or the agent's default model otherwise.
func EnsureValidModelForACPAgent(agentID, currentModel string) string {
	ag := findAgentInCatalog(defaultACPAgentDefinitions(), agentID)
	return ensureModelForDefinition(ag, agentID, currentModel)
}

func ensureModelForDefinition(ag *ACPAgentDefinition, agentID, currentModel string) string {
	cur := strings.TrimSpace(currentModel)
	if ag == nil {
		if cur != "" {
			return cur
		}
		def, _ := DefaultModelForACPAgent(agentID)
		return def
	}
	if ag.ID == "agent-internal" || ag.Type == "internal" {
		return cur
	}
	if cur != "" {
		for _, sm := range ag.SupportedModels {
			if strings.EqualFold(sm, cur) {
				return sm
			}
		}
	}
	def, _, _ := resolveAgentDefaults(ag, agentID)
	if def != "" {
		return def
	}
	return cur
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
			if resolved, err := lookPathEnriched(mgr.Agents[i].Command); err == nil {
				mgr.Agents[i].Available = true
				// Absolute path so spawning works in the GUI environment.
				mgr.Agents[i].Command = resolved
			} else {
				mgr.Agents[i].Available = false
			}
		}
	}
}

// UpdateAgentSupportedModels updates the supported models for a registered ACP agent.
func (mgr *ACPManager) UpdateAgentSupportedModels(agentID string, models []string) {
	if len(models) == 0 {
		return
	}
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	norm := normalizeACPAgentLookupID(agentID)
	for i := range mgr.Agents {
		if strings.EqualFold(mgr.Agents[i].ID, agentID) || strings.EqualFold(mgr.Agents[i].ID, norm) {
			mgr.Agents[i].SupportedModels = append([]string(nil), models...)
			if mgr.Agents[i].Model == "" && len(models) > 0 {
				mgr.Agents[i].Model = models[0]
			}
			break
		}
	}
}

// StartSession launches an ACP agent session.
func (mgr *ACPManager) StartSession(agentID, cwd string, model string) (*acp.Session, error) {
	return mgr.StartSessionWithID("", agentID, cwd, model, "supervised")
}

// StartSessionWithID launches or attaches an ACP agent session with a specific
// session ID. permissionMode is one of forge-ade's SessionOpts values:
// "supervised" | "auto" | "auto_edits" | "full". Optional resumeSessionID
// specifies a remote conversation ID to resume.
func (mgr *ACPManager) StartSessionWithID(sessionID, agentID, cwd, model, permissionMode string, resumeSessionID ...string) (*acp.Session, error) {
	mgr.mu.Lock()
	var targetDef *ACPAgentDefinition
	for i := range mgr.Agents {
		if mgr.Agents[i].ID == agentID {
			targetDef = &mgr.Agents[i]
			break
		}
	}
	// Fallback to first available ACP agent if unknown or legacy monolith
	if targetDef == nil || targetDef.ID == "agent-monolith" {
		for i := range mgr.Agents {
			if mgr.Agents[i].Available && mgr.Agents[i].Type != "internal" {
				targetDef = &mgr.Agents[i]
				break
			}
		}
	}
	mgr.mu.Unlock()

	if targetDef == nil {
		return nil, fmt.Errorf("no available ACP agent found for ID: %s", agentID)
	}

	if sessionID == "" {
		sessionID = fmt.Sprintf("sess-acp-%d", time.Now().UnixNano()%100000)
	}

	if targetDef.Type == "internal" {
		return &acp.Session{
			ID:        sessionID,
			Title:     targetDef.Name,
			AgentID:   targetDef.ID,
			Cwd:       cwd,
			Model:     model,
			Status:    "idle",
			CreatedAt: time.Now().Unix(),
		}, nil
	}

	// Normalize model against agent's supported models
	if len(targetDef.SupportedModels) > 0 {
		isSupported := false
		for _, sm := range targetDef.SupportedModels {
			if strings.EqualFold(sm, model) {
				isSupported = true
				model = sm
				break
			}
		}
		if !isSupported {
			model = targetDef.Model
		}
	} else if model == "" {
		model = targetDef.Model
	}

	if permissionMode == "" {
		permissionMode = "supervised"
	}

	var resumeID string
	if len(resumeSessionID) > 0 {
		resumeID = strings.TrimSpace(resumeSessionID[0])
	}

	opts := acp.SessionOpts{
		SessionID:       sessionID,
		AgentID:         targetDef.ID,
		Command:         targetDef.Command,
		Args:            targetDef.Args,
		Cwd:             cwd,
		Model:           model,
		PermissionMode:  permissionMode,
		ResumeSessionID: resumeID,
	}

	if mgr.manager != nil {
		return mgr.manager.StartSession(opts)
	}
	return nil, fmt.Errorf("acp manager not initialized")
}

// RestartSession restarts the underlying ACP process for a session, keeping its conversation ID intact.
func (mgr *ACPManager) RestartSession(sessionID string) (*acp.Session, error) {
	if mgr.manager != nil {
		return mgr.manager.RestartSession(sessionID)
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

// Cancel stops the currently running turn in the ACP session (interrupts the
// CLI process group for stream agents, protocol cancel for standard ACP).
func (mgr *ACPManager) Cancel(sessionID string) error {
	if mgr.manager != nil {
		return mgr.manager.Cancel(sessionID)
	}
	return nil
}

// RespondPermission answers a permission request with allow/deny; allow picks
// the best allow option of the request, as forge-ade's UI does.
func (mgr *ACPManager) RespondPermission(sessionID, requestID string, allow bool) error {
	mgr.mu.Lock()
	var req *PermissionRequest
	if mgr.activeReqs != nil {
		req = mgr.activeReqs[requestID]
		delete(mgr.activeReqs, requestID)
	}
	if req == nil {
		req = mgr.ActiveReq
	}
	if mgr.ActiveReq != nil && (mgr.ActiveReq.RequestID == requestID || len(mgr.activeReqs) == 0) {
		mgr.ActiveReq = nil
	}
	mgr.mu.Unlock()

	if mgr.manager == nil {
		return nil
	}
	if !allow {
		return mgr.manager.RespondPermissionOption(sessionID, requestID, false, "deny")
	}
	// Pick the first allow-kind option; the manager falls back sanely when
	// given an empty option id.
	optionID := ""
	if req != nil && req.RequestID == requestID {
		for _, opt := range req.Options {
			if strings.Contains(opt.Kind, "allow") || opt.OptionID != "" {
				optionID = opt.OptionID
				if strings.Contains(opt.Kind, "allow") {
					break
				}
			}
		}
	}
	if optionID != "" {
		return mgr.manager.RespondPermissionOption(sessionID, requestID, true, optionID)
	}
	return mgr.manager.RespondPermission(sessionID, requestID, true)
}

// RespondPermissionOption answers a permission request with an explicit
// option id ("allow"/"deny"/a specific ACP option).
func (mgr *ACPManager) RespondPermissionOption(sessionID, requestID, optionID string) error {
	mgr.mu.Lock()
	if mgr.activeReqs != nil {
		delete(mgr.activeReqs, requestID)
	}
	if mgr.ActiveReq != nil && (mgr.ActiveReq.RequestID == requestID || len(mgr.activeReqs) == 0) {
		mgr.ActiveReq = nil
	}
	mgr.mu.Unlock()
	if mgr.manager != nil {
		return mgr.manager.RespondPermissionOption(sessionID, requestID, true, optionID)
	}
	return nil
}

// GetUsageSummary returns forge-ade's aggregated ACP usage report for the
// quota/usage modal.
func (mgr *ACPManager) GetUsageSummary() map[string]interface{} {
	if mgr.manager != nil {
		return mgr.manager.GetUsageSummary()
	}
	return map[string]interface{}{}
}

// ListSessions returns the live ACP sessions known to the internal manager.
func (mgr *ACPManager) ListSessions() []*acp.Session {
	if mgr.manager != nil {
		return mgr.manager.ListSessions()
	}
	return nil
}

// subscribeToBusEvents listens to the agent:* events forge-ade's internal ACP
// manager emits on the event bus. Payload keys mirror internal/acp/manager.go.
func (mgr *ACPManager) subscribeToBusEvents() {
	if mgr.bus == nil {
		return
	}

	sessionKey := func(data map[string]interface{}) string {
		id, _ := data["sessionId"].(string)
		if id == "" {
			id, _ = data["session_id"].(string)
		}
		return id
	}
	str := func(data map[string]interface{}, keys ...string) string {
		for _, k := range keys {
			if v, ok := data[k].(string); ok && v != "" {
				return v
			}
		}
		return ""
	}
	num := func(data map[string]interface{}, keys ...string) int {
		for _, k := range keys {
			if v, ok := data[k].(float64); ok {
				return int(v)
			}
		}
		return 0
	}
	stringify := func(v interface{}) string {
		switch t := v.(type) {
		case string:
			return t
		case nil:
			return ""
		default:
			if b, err := json.Marshal(t); err == nil {
				return string(b)
			}
			return fmt.Sprintf("%v", t)
		}
	}

	// Text streaming: {sessionId, delta, kind:"text"}
	mgr.bus.Subscribe(events.AgentMessageDelta, func(e events.Event) {
		if e.Data == nil {
			return
		}
		sessionID := sessionKey(e.Data)
		if kind := str(e.Data, "kind"); kind != "" && kind != "text" && kind != "message" {
			return // thinking and other kinds stream via their own events
		}
		text := str(e.Data, "delta", "text", "content")
		if sessionID != "" && text != "" {
			// Token deltas are the highest-volume events: coalesce per
			// session so a saturating queue drops bursts, never the turn
			// lifecycle events around them (and never blocks the publisher).
			mgr.postDeltaCoalesced(sessionID, "token", text, func(sid, txt string) {
				if mgr.OnTokenChunk != nil {
					mgr.OnTokenChunk(sid, txt)
				}
			})
		}
	})

	// Thinking: {sessionId, delta}
	mgr.bus.Subscribe("agent:thinking_delta", func(e events.Event) {
		if e.Data == nil {
			return
		}
		sessionID := sessionKey(e.Data)
		text := str(e.Data, "delta", "text", "content", "thinking")
		if sessionID != "" && text != "" {
			mgr.postDeltaCoalesced(sessionID, "thought", text, func(sid, txt string) {
				if mgr.OnThoughtChunk != nil {
					mgr.OnThoughtChunk(sid, txt)
				}
			})
		}
	})

	// Tool calls: start {tool_call_id, name, title?, args}; end {tool_call_id,
	// name?, title?, args?, result, is_error}
	handleTool := func(e events.Event, status string) {
		if e.Data == nil {
			return
		}
		ev := ToolEvent{
			SessionID:  sessionKey(e.Data),
			ToolCallID: str(e.Data, "tool_call_id", "toolCallId", "id"),
			Name:       str(e.Data, "name", "toolName", "title"),
			Args:       stringify(e.Data["args"]),
			Status:     status,
			Output:     str(e.Data, "result", "output"),
			IsError:    boolOf(e.Data["is_error"]) || boolOf(e.Data["isError"]),
		}
		if ev.Status == "completed" && ev.Output == "" {
			ev.Output = stringify(e.Data["result"])
		}
		if ev.SessionID != "" {
			mgr.postEvent(func() {
				if mgr.OnToolEvent != nil {
					mgr.OnToolEvent(ev)
				}
			})
		}
	}
	mgr.bus.Subscribe("agent:tool_start", func(e events.Event) { handleTool(e, "running") })
	mgr.bus.Subscribe("agent:tool_end", func(e events.Event) {
		if e.Data == nil {
			return
		}
		status := "completed"
		if boolOf(e.Data["is_error"]) || boolOf(e.Data["isError"]) {
			status = "failed"
		}
		handleTool(e, status)
	})

	// Turn lifecycle
	mgr.bus.Subscribe("agent:turn_start", func(e events.Event) {
		if e.Data == nil {
			return
		}
		sessionID := sessionKey(e.Data)
		prompt := str(e.Data, "prompt")
		if sessionID != "" {
			mgr.postEvent(func() {
				if mgr.OnTurnStart != nil {
					mgr.OnTurnStart(sessionID, prompt)
				}
			})
		}
	})
	mgr.bus.Subscribe("agent:turn_end", func(e events.Event) {
		if e.Data == nil {
			return
		}
		sessionID := sessionKey(e.Data)
		if sessionID == "" {
			return
		}
		remID := str(e.Data, "remoteSessionId", "remote_session_id", "conversation_id")
		if remID == "" && mgr.manager != nil {
			if s, ok := mgr.manager.GetSession(sessionID); ok && s != nil {
				remID = s.RemoteSessionID
			}
		}
		info := TurnEndInfo{
			Status:          str(e.Data, "status"),
			Error:           str(e.Data, "error"),
			TotalSteps:      num(e.Data, "total_steps", "totalSteps"),
			StepCount:       num(e.Data, "step_count", "stepCount"),
			RemoteSessionID: remID,
		}
		u, hasUsage := e.Data["usage"].(map[string]interface{})
		mgr.postEvent(func() {
			if hasUsage {
				mgr.deliverUsage(sessionID, u, num, stringify)
			}
			if mgr.OnTurnDone != nil {
				mgr.OnTurnDone(sessionID, info)
			}
		})
	})

	// Permission prompts: forge-ade emits "agent:ask" in supervised mode with
	// {requestId, title, desc, tool, arguments, options}. The internal agent
	// engine instead emits {questions: [...]} for structured ask-tool
	// questions (no requestId) — routed to OnQuestions.
	askHandler := func(e events.Event) {
		if e.Data == nil {
			return
		}
		sessionID := sessionKey(e.Data)
		if rawQs, ok := e.Data["questions"].([]interface{}); ok && len(rawQs) > 0 {
			var questions []AgentQuestion
			for _, rq := range rawQs {
				qm, ok := rq.(map[string]interface{})
				if !ok {
					continue
				}
				q := AgentQuestion{
					ID:          str(qm, "id"),
					Question:    str(qm, "question"),
					Header:      str(qm, "header"),
					Multi:       boolOf(qm["multi"]),
					Recommended: -1,
				}
				if rec, ok := qm["recommended"].(float64); ok {
					q.Recommended = int(rec)
				}
				if opts, ok := qm["options"].([]interface{}); ok {
					for _, o := range opts {
						if s, ok := o.(string); ok {
							q.Options = append(q.Options, s)
						}
					}
				}
				if q.ID != "" && q.Question != "" {
					questions = append(questions, q)
				}
			}
			if len(questions) > 0 && sessionID != "" {
				log.Printf("[acp] agent:ask for session %q (%d questions)", sessionID, len(questions))
				mgr.postEvent(func() {
					if mgr.OnQuestions != nil {
						mgr.OnQuestions(sessionID, questions)
					}
					if mgr.OnNeedRedraw != nil {
						mgr.OnNeedRedraw()
					}
				})
				return
			}
			log.Printf("[acp] agent:ask without routable questions (session=%q)", sessionID)
		}
		reqID := str(e.Data, "requestId", "request_id")
		if reqID == "" {
			return
		}
		req := &PermissionRequest{
			SessionID:   sessionKey(e.Data),
			RequestID:   reqID,
			ToolName:    str(e.Data, "tool", "toolName"),
			Title:       str(e.Data, "title"),
			Description: str(e.Data, "desc", "description"),
			Arguments:   stringify(e.Data["arguments"]),
		}
		if req.Title == "" {
			req.Title = req.Description
		}
		if opts, ok := e.Data["options"].([]interface{}); ok {
			for _, o := range opts {
				om, ok := o.(map[string]interface{})
				if !ok {
					continue
				}
				req.Options = append(req.Options, PermissionOption{
					OptionID: str(om, "optionId", "option_id", "id"),
					Name:     str(om, "name", "label"),
					Kind:     str(om, "kind"),
				})
			}
		}
		mgr.postEvent(func() {
			mgr.mu.Lock()
			mgr.ActiveReq = req
			if mgr.activeReqs == nil {
				mgr.activeReqs = make(map[string]*PermissionRequest)
			}
			mgr.activeReqs[req.RequestID] = req
			mgr.mu.Unlock()
			if mgr.OnAsk != nil {
				mgr.OnAsk(req)
			}
			if mgr.OnNeedRedraw != nil {
				mgr.OnNeedRedraw()
			}
		})
	}
	mgr.bus.Subscribe("agent:ask", askHandler)
	// Kept for forward-compatibility with a plain permission event.
	mgr.bus.Subscribe(events.AgentPermission, askHandler)

	// Slash commands advertised by the agent: {commands: [{name, description}]}
	mgr.bus.Subscribe("agent:available_commands", func(e events.Event) {
		if e.Data == nil {
			return
		}
		sessionID := sessionKey(e.Data)
		cmds, _ := e.Data["commands"].([]interface{})
		var names []string
		for _, cmd := range cmds {
			cm, ok := cmd.(map[string]interface{})
			if !ok {
				continue
			}
			if name := str(cm, "name", "command"); name != "" {
				names = append(names, name)
			}
		}
		if sessionID != "" && len(names) > 0 {
			mgr.postEvent(func() {
				if mgr.OnCommands != nil {
					mgr.OnCommands(sessionID, names)
				}
			})
		}
	})

	// Usage: {usage: {inputTokens, outputTokens, ...}}
	mgr.bus.Subscribe("agent:usage", func(e events.Event) {
		if e.Data == nil {
			return
		}
		sessionID := sessionKey(e.Data)
		if u, ok := e.Data["usage"].(map[string]interface{}); ok && sessionID != "" {
			mgr.postEvent(func() {
				mgr.deliverUsage(sessionID, u, num, stringify)
			})
		}
	})
}

func boolOf(v interface{}) bool {
	b, _ := v.(bool)
	return b
}

// deliverUsage parses a usage map into UsageInfo and dispatches it.
func (mgr *ACPManager) deliverUsage(sessionID string, u map[string]interface{}, num func(map[string]interface{}, ...string) int, stringify func(interface{}) string) {
	usage := UsageInfo{
		InputTokens:  num(u, "inputTokens", "input_tokens", "prompt_tokens", "promptTokens"),
		OutputTokens: num(u, "outputTokens", "output_tokens", "completion_tokens", "completionTokens"),
		CachedTokens: num(u, "cachedTokens", "cached_read_input_tokens", "cache_read_input_tokens", "cached_input_tokens", "prompt_cache_hit_tokens", "cached_tokens"),
		TotalTokens:  num(u, "totalTokens", "total_tokens"),
	}
	if cost := stringify(u["costUsd"]); cost != "" {
		fmt.Sscanf(cost, "%f", &usage.CostUSD)
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	}
	if mgr.OnUsage != nil {
		mgr.OnUsage(sessionID, usage)
	}
}

var (
	enrichedDirsOnce sync.Once
	enrichedDirsList []string
)

func cachedEnrichedPathDirs() []string {
	enrichedDirsOnce.Do(func() {
		dirs := filepath.SplitList(internalgit.EnrichedPath())
		// Version-manager bins (nvm/volta) that GUI apps never inherit.
		if home, err := os.UserHomeDir(); err == nil {
			if matches, _ := filepath.Glob(filepath.Join(home, ".nvm", "versions", "node", "*", "bin")); len(matches) > 0 {
				dirs = append(dirs, matches...)
			}
			dirs = append(dirs, filepath.Join(home, ".volta", "bin"), filepath.Join(home, ".deno", "bin"))
		}
		enrichedDirsList = dirs
	})
	return enrichedDirsList
}

// lookPathEnriched finds an executable on the ENRICHED PATH (Homebrew,
// ~/.local/bin, go/bin, cargo, nix, ...). GUI-launched apps on macOS don't
// inherit the shell PATH, so a plain exec.LookPath marks installed ACP
// harnesses and agent CLIs as unavailable — the webview app avoids this
// with the same enrichment (git.EnrichedPath).
func lookPathEnriched(exe string) (string, error) {
	if exe == "" {
		return "", exec.ErrNotFound
	}
	if strings.ContainsRune(exe, os.PathSeparator) {
		if st, err := os.Stat(exe); err == nil && !st.IsDir() {
			return exe, nil
		}
	}
	for _, dir := range cachedEnrichedPathDirs() {
		if dir == "" {
			continue
		}
		cand := filepath.Join(dir, exe)
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			return cand, nil
		}
	}
	return exec.LookPath(exe)
}
