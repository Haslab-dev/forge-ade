package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// BotProfile defines a persistent Local ForgeADE Bot (inspired by Orca Bots):
// a named autonomous persona with its own Home Chat, default agent/model,
// workspace scope, and system instructions.
type BotProfile struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Role          string    `json:"role"`
	SystemPrompt  string    `json:"systemPrompt,omitempty"`
	AgentID       string    `json:"agentId"`
	AgentType     string    `json:"agentType"` // "internal" or "acp"
	Model         string    `json:"model,omitempty"`
	WorkspacePath string    `json:"workspacePath,omitempty"`
	HomeSessionID string    `json:"homeSessionId,omitempty"`
	Paused        bool      `json:"paused,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
}

// BotStore persists Local Bot profiles to disk (<dir>/bots.json).
type BotStore struct {
	mu   sync.RWMutex
	path string
	bots []*BotProfile
}

// NewBotStore loads or initializes the local bot catalog at <dir>/bots.json.
func NewBotStore(dir string) *BotStore {
	if dir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, ".forge-ade")
		}
	}
	st := &BotStore{
		path: filepath.Join(dir, "bots.json"),
	}
	st.load()
	return st
}

func (st *BotStore) load() {
	if st == nil || st.path == "" {
		return
	}
	data, err := os.ReadFile(st.path)
	if err != nil {
		return
	}
	var list []*BotProfile
	if err := json.Unmarshal(data, &list); err == nil {
		st.mu.Lock()
		st.bots = list
		st.mu.Unlock()
	}
}

func (st *BotStore) saveLocked() {
	if st == nil || st.path == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(st.path), 0o755)
	data, err := json.MarshalIndent(st.bots, "", "  ")
	if err != nil {
		return
	}
	tmp := st.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err == nil {
		_ = os.Rename(tmp, st.path)
	}
}

// List returns a snapshot of all configured local bots.
func (st *BotStore) List() []*BotProfile {
	if st == nil {
		return nil
	}
	st.mu.RLock()
	defer st.mu.RUnlock()
	out := make([]*BotProfile, len(st.bots))
	copy(out, st.bots)
	return out
}

// Get finds a bot by ID.
func (st *BotStore) Get(id string) *BotProfile {
	if st == nil {
		return nil
	}
	st.mu.RLock()
	defer st.mu.RUnlock()
	for _, b := range st.bots {
		if b != nil && b.ID == id {
			return b
		}
	}
	return nil
}

// Create adds a new local bot profile and persists it.
func (st *BotStore) Create(b BotProfile) *BotProfile {
	if st == nil {
		return nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()

	if b.ID == "" {
		b.ID = fmt.Sprintf("bot-%d", time.Now().UnixNano())
	}
	if strings.TrimSpace(b.Name) == "" {
		b.Name = "Forge Bot"
	}
	if b.AgentID == "" {
		b.AgentID = "agent-internal"
	}
	if b.AgentType == "" {
		if b.AgentID == "agent-internal" {
			b.AgentType = "internal"
		} else {
			b.AgentType = "acp"
		}
	}
	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now()
	}
	cp := b
	st.bots = append(st.bots, &cp)
	st.saveLocked()
	return &cp
}

// UpdateHomeSession links a bot to its Home Chat session ID and persists.
func (st *BotStore) UpdateHomeSession(botID, sessionID string) {
	if st == nil || botID == "" {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	for _, b := range st.bots {
		if b != nil && b.ID == botID {
			b.HomeSessionID = sessionID
			st.saveLocked()
			return
		}
	}
}

// TogglePause flips the paused flag for a bot.
func (st *BotStore) TogglePause(botID string) {
	if st == nil {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	for _, b := range st.bots {
		if b != nil && b.ID == botID {
			b.Paused = !b.Paused
			st.saveLocked()
			return
		}
	}
}

// Delete removes a bot by ID.
func (st *BotStore) Delete(botID string) {
	if st == nil {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	out := st.bots[:0]
	for _, b := range st.bots {
		if b != nil && b.ID != botID {
			out = append(out, b)
		}
	}
	st.bots = out
	st.saveLocked()
}

// OpenBotHomeSession activates (or creates) the persistent Home Chat session
// for bot, tagging it with BotID.
func (as *AgentState) OpenBotHomeSession(store *BotStore, bot *BotProfile) *AgentSession {
	if as == nil || bot == nil {
		return nil
	}
	as.mu.Lock()
	if bot.HomeSessionID != "" {
		for _, s := range as.Sessions {
			if s != nil && s.ID == bot.HomeSessionID {
				as.ActiveSessionID = s.ID
				as.syncActiveSession()
				as.mu.Unlock()
				if as.Invalidate != nil {
					as.Invalidate()
				}
				return s
			}
		}
	}

	now := time.Now()
	id := as.nextSessionIDLocked()
	ws := bot.WorkspacePath
	if ws == "" {
		ws = as.workspacePath
	}
	model := bot.Model
	if model == "" {
		model = as.ActiveModel
	}
	sess := &AgentSession{
		ID:            id,
		Title:         bot.Name + " · Home",
		Status:        "idle",
		CreatedAt:     now,
		UpdatedAt:     now,
		Model:         model,
		Provider:      as.ActiveProvider,
		AgentID:       bot.AgentID,
		AgentType:     bot.AgentType,
		WorkspacePath: ws,
		BotID:         bot.ID,
		Tasks:         NewTaskManager(),
		nextMsgID:     1,
	}
	as.Sessions = append([]*AgentSession{sess}, as.Sessions...)
	as.ActiveSessionID = id
	as.ActiveAgentID = bot.AgentID
	as.ActiveAgentType = bot.AgentType
	if model != "" {
		as.ActiveModel = model
	}
	as.Messages = sess.Messages
	as.TaskManager = sess.Tasks
	as.mu.Unlock()

	if store != nil {
		store.UpdateHomeSession(bot.ID, id)
	}
	as.SaveSessionToDisk(sess)
	if as.Invalidate != nil {
		as.Invalidate()
	}
	return sess
}

// HandToBot hands off sourceSessionID to bot with the given goal, tagging the
// newly created session with bot.ID so it appears under the Bot in the sidebar.
func (as *AgentState) HandToBot(bot *BotProfile, sourceSessionID, goal string, autoStart bool) (*AgentSession, error) {
	if as == nil || bot == nil {
		return nil, fmt.Errorf("bot is required")
	}
	nextSteps := strings.TrimSpace(bot.SystemPrompt)
	if bot.Role != "" {
		if nextSteps != "" {
			nextSteps = fmt.Sprintf("Act as **%s** (%s).\n%s", bot.Name, bot.Role, nextSteps)
		} else {
			nextSteps = fmt.Sprintf("Act as **%s** (%s) and complete the goal autonomously.", bot.Name, bot.Role)
		}
	}
	sess, err := as.ExecuteHandoff(HandoffRequest{
		SourceSessionID: sourceSessionID,
		TargetAgentID:   bot.AgentID,
		TargetAgentType: bot.AgentType,
		TargetModel:     bot.Model,
		Goal:            goal,
		NextSteps:       nextSteps,
		AutoStart:       autoStart,
	})
	if err != nil {
		return nil, err
	}
	as.mu.Lock()
	sess.BotID = bot.ID
	as.mu.Unlock()
	as.SaveSessionToDisk(sess)
	return sess, nil
}
