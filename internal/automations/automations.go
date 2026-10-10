package automations

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Run is one execution of an automation. Runs are created when the user
// launches an automation; the session that carries the run is linked by ID so
// the UI can jump from run history to the conversation.
type Run struct {
	ID        string    `json:"id"`
	SessionID string    `json:"sessionId,omitempty"`
	Workspace string    `json:"workspace,omitempty"`
	StartedAt time.Time `json:"startedAt"`
}

// Automation is a saved, re-runnable prompt workflow scoped to a workspace
// (ZCode "Automations"): a named prompt the user can launch into a new agent
// session with one click, with recent run history kept per automation.
type Automation struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Prompt      string     `json:"prompt"`
	Workspace   string     `json:"workspace,omitempty"`
	CronExpr    string     `json:"cronExpr,omitempty"` // 5-field cron, local time; empty = manual-only
	Enabled     bool       `json:"enabled"`            // scheduled automations can be paused
	NextRunAt   *time.Time `json:"nextRunAt,omitempty"`
	// PendingRun holds a fire token handed to the frontend (automations:fire
	// event). The window that claims it runs the automation; others ignore it.
	PendingRun string     `json:"pendingRun,omitempty"`
	LastRunAt  *time.Time `json:"lastRunAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	Runs       []Run      `json:"runs,omitempty"`
}

// scheduleActive reports whether the automation is cron-scheduled and running.
func (a *Automation) scheduleActive() bool {
	return a.Enabled && a.CronExpr != ""
}

// computeNextRun returns the next fire time for the automation's schedule, or
// zero when unscheduled/unparseable.
func (a *Automation) computeNextRun(after time.Time) time.Time {
	if a.CronExpr == "" {
		return time.Time{}
	}
	expr, err := parseCron(a.CronExpr)
	if err != nil {
		return time.Time{}
	}
	return expr.next(after)
}

const maxRunsPerAutomation = 50

// Manager persists automations as a single JSON document in the app data dir
// (same pattern as the agent session store). All mutations rewrite the file;
// read volume is tiny so no fancier storage is warranted.
type Manager struct {
	mu    sync.RWMutex
	path  string
	items []Automation
}

func NewManager(dataDir string) (*Manager, error) {
	path := filepath.Join(dataDir, "automations.json")
	m := &Manager{path: path}
	data, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(data, &m.items)
	}
	if m.items == nil {
		m.items = []Automation{}
	}
	return m, nil
}

func (m *Manager) persistLocked() {
	data, err := json.MarshalIndent(m.items, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(m.path, data, 0644)
}

// List returns automations, newest-updated first.
func (m *Manager) List() []Automation {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Automation, len(m.items))
	copy(out, m.items)
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out
}

// Get returns one automation by ID.
func (m *Manager) Get(id string) (Automation, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, a := range m.items {
		if a.ID == id {
			return a, true
		}
	}
	return Automation{}, false
}

// AutomationSaveInput is the Wails-facing create/update payload. Timestamps
// and run history are owned by the store and deliberately absent: Wails fails
// the whole binding call when it cannot parse a client-provided time.Time
// (e.g. an empty string for createdAt).
type AutomationSaveInput struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
	Workspace   string `json:"workspace"`
	CronExpr    string `json:"cronExpr"`
	Enabled     bool   `json:"enabled"`
}

// ToAutomation maps the wire input into the stored record.
func (in AutomationSaveInput) ToAutomation() Automation {
	return Automation{
		ID:          in.ID,
		Name:        in.Name,
		Description: in.Description,
		Prompt:      in.Prompt,
		Workspace:   in.Workspace,
		CronExpr:    in.CronExpr,
		Enabled:     in.Enabled,
	}
}

// Save upserts an automation. Empty ID creates one. Name and prompt are
// required; the run history is preserved on update.
func (m *Manager) Save(a Automation) (Automation, error) {
	if a.Name == "" {
		return Automation{}, fmt.Errorf("automation name is required")
	}
	if a.Prompt == "" {
		return Automation{}, fmt.Errorf("automation prompt is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	if a.ID == "" {
		a.ID = uuid.New().String()
		a.CreatedAt = now
		a.UpdatedAt = now
		if a.CronExpr != "" {
			a.Enabled = true // scheduled automations start active
		}
		a.NextRunAt = a.nextRunAtPtr(now)
		m.items = append([]Automation{a}, m.items...)
		m.persistLocked()
		return a, nil
	}
	for i := range m.items {
		if m.items[i].ID == a.ID {
			runs := m.items[i].Runs
			createdAt := m.items[i].CreatedAt
			scheduleChanged := m.items[i].CronExpr != a.CronExpr
			a.UpdatedAt = now
			a.CreatedAt = createdAt
			a.Runs = runs
			// A new/changed schedule recomputes the fire time; pausing drops it.
			if a.scheduleActive() && (scheduleChanged || a.NextRunAt == nil) {
				a.NextRunAt = a.nextRunAtPtr(now)
			} else if !a.scheduleActive() {
				a.NextRunAt = nil
			}
			m.items[i] = a
			m.persistLocked()
			return a, nil
		}
	}
	return Automation{}, fmt.Errorf("automation %s not found", a.ID)
}

// Delete removes an automation by ID.
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.items {
		if m.items[i].ID == id {
			m.items = append(m.items[:i], m.items[i+1:]...)
			m.persistLocked()
			return nil
		}
	}
	return fmt.Errorf("automation %s not found", id)
}

// RecordRun appends a run entry to an automation's history (newest first,
// capped), stamps LastRunAt, and bumps UpdatedAt so it sorts to the top of
// the list.
func (m *Manager) RecordRun(id string, run Run) (Automation, error) {
	if run.ID == "" {
		run.ID = uuid.New().String()
	}
	run.StartedAt = time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.items {
		if m.items[i].ID == id {
			m.items[i].Runs = append([]Run{run}, m.items[i].Runs...)
			if len(m.items[i].Runs) > maxRunsPerAutomation {
				m.items[i].Runs = m.items[i].Runs[:maxRunsPerAutomation]
			}
			last := run.StartedAt
			m.items[i].LastRunAt = &last
			m.items[i].UpdatedAt = time.Now()
			m.persistLocked()
			return m.items[i], nil
		}
	}
	return Automation{}, fmt.Errorf("automation %s not found", id)
}

// SetEnabled pauses (or resumes) a scheduled automation. Pausing drops the
// pending fire time; resuming recomputes it.
func (m *Manager) SetEnabled(id string, enabled bool) (Automation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.items {
		if m.items[i].ID == id {
			m.items[i].Enabled = enabled
			if enabled {
				m.items[i].NextRunAt = m.items[i].nextRunAtPtr(time.Now())
			} else {
				m.items[i].NextRunAt = nil
			}
			m.items[i].UpdatedAt = time.Now()
			m.persistLocked()
			return m.items[i], nil
		}
	}
	return Automation{}, fmt.Errorf("automation %s not found", id)
}

// BeginFire records a fire token for a due automation so exactly one
// frontend window claims and executes it.
func (m *Manager) BeginFire(id string, token string) (Automation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.items {
		if m.items[i].ID == id {
			m.items[i].PendingRun = token
			m.persistLocked()
			return m.items[i], nil
		}
	}
	return Automation{}, fmt.Errorf("automation %s not found", id)
}

// ClaimFire consumes a pending fire token. Returns true only for the first
// caller presenting the exact token — everyone else must ignore the fire.
func (m *Manager) ClaimFire(id string, token string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.items {
		if m.items[i].ID == id && m.items[i].PendingRun != "" && m.items[i].PendingRun == token {
			m.items[i].PendingRun = ""
			m.persistLocked()
			return true
		}
	}
	return false
}

// nextRunAtPtr recomputes the next fire time; zero result becomes nil so it
// is omitted from JSON.
func (a *Automation) nextRunAtPtr(after time.Time) *time.Time {
	next := a.computeNextRun(after)
	if next.IsZero() {
		return nil
	}
	return &next
}

// StartScheduler runs the host fire loop until ctx is cancelled: every tick
// it fires due automations (fire is called outside the lock) and recomputes
// their next fire time. On startup, schedules whose next run is already in
// the past (app was closed) fire once immediately — one catch-up run, never
// one per missed interval.
func (m *Manager) StartScheduler(ctx context.Context, fire func(Automation)) {
	go func() {
		m.tick(fire)
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.tick(fire)
			}
		}
	}()
}

// tick finds due automations and fires them. All schedule bookkeeping happens
// under the lock in one pass; fire callbacks run outside it.
func (m *Manager) tick(fire func(Automation)) {
	now := time.Now()
	var due []Automation

	m.mu.Lock()
	for i := range m.items {
		a := &m.items[i]
		if !a.scheduleActive() {
			continue
		}
		if a.NextRunAt == nil {
			// First observation of a schedule (e.g. imported): compute going forward.
			a.NextRunAt = a.nextRunAtPtr(now)
			continue
		}
		if !now.Before(*a.NextRunAt) {
			due = append(due, *a)
			last := now
			a.LastRunAt = &last
			a.NextRunAt = a.nextRunAtPtr(now)
		}
	}
	if len(due) > 0 {
		m.persistLocked()
	}
	m.mu.Unlock()

	for _, a := range due {
		fire(a)
	}
}
