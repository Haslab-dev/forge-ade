package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Event is a single lightweight LLM request record. Written per request and
// aggregated incrementally into daily/hourly buckets for fast dashboards.
type Event struct {
	ID            string    `json:"id"`
	Timestamp     time.Time `json:"timestamp"`
	WorkspaceID   string    `json:"workspace_id"`
	WorkspaceName string    `json:"workspace_name"`
	SessionID     string    `json:"session_id"`
	Agent         string    `json:"agent"`
	Provider      string    `json:"provider"`
	Model         string    `json:"model"`
	InputTokens   int64     `json:"input_tokens"`
	OutputTokens  int64     `json:"output_tokens"`
	CachedTokens  int64     `json:"cached_tokens"`
	ThinkingTokens int64    `json:"thinking_tokens"`
	ToolCalls     int       `json:"tool_calls"`
	RetryCount    int       `json:"retry_count"`
	LatencyMS     int64     `json:"latency_ms"`
	CostUSD       float64   `json:"cost_usd"`
	Success       bool      `json:"success"`
}

// Store persists usage events and maintains incremental per-day aggregates.
type Store struct {
	mu      sync.Mutex
	dir     string
	events  []*Event
	byDay   map[string]*DayAgg
	byWorkspace map[string]*Agg
	byAgent     map[string]*Agg
	byProvider  map[string]*Agg
	byModel     map[string]*Agg
	byTool      map[string]*ToolAgg
}

// DayAgg is a per-day rollup (key: "2006-01-02").
type DayAgg struct {
	Requests      int64
	InputTokens   int64
	OutputTokens  int64
	CachedTokens  int64
	ThinkingTokens int64
	LatencyTotalMS int64
	ToolCalls     int64
	CostUSD       float64
	Failures      int64
}

// Agg is a generic rollup keyed by workspace/agent/provider/model.
type Agg struct {
	Requests      int64
	InputTokens   int64
	OutputTokens  int64
	CachedTokens  int64
	ThinkingTokens int64
	LatencyTotalMS int64
	ToolCalls     int64
	CostUSD       float64
	Failures      int64
}

// ToolAgg is a rollup per tool name (execution stats).
type ToolAgg struct {
	Calls      int64
	Failures   int64
	LatencyTotalMS int64
}

// NewStore creates a usage store backed by dir.
func NewStore(dir string) *Store {
	s := &Store{
		dir:         dir,
		events:      make([]*Event, 0, 4096),
		byDay:       make(map[string]*DayAgg),
		byWorkspace: make(map[string]*Agg),
		byAgent:     make(map[string]*Agg),
		byProvider:  make(map[string]*Agg),
		byModel:     make(map[string]*Agg),
		byTool:      make(map[string]*ToolAgg),
	}
	s.load()
	return s
}

func (s *Store) load() {
	data, err := os.ReadFile(filepath.Join(s.dir, "usage_events.jsonl"))
	if err != nil {
		return
	}
	// Replay the journal into the in-memory aggregates.
	var evs []*Event
	for _, line := range splitLines(data) {
		if len(line) == 0 {
			continue
		}
		var ev Event
		if json.Unmarshal(line, &ev) == nil {
			evs = append(evs, &ev)
		}
	}
	s.events = evs
	for _, ev := range evs {
		s.apply(ev)
	}
}

func splitLines(b []byte) [][]byte {
	var out [][]byte
	start := 0
	for i, c := range b {
		if c == '\n' {
			if i > start {
				out = append(out, b[start:i])
			}
			start = i + 1
		}
	}
	if start < len(b) {
		out = append(out, b[start:])
	}
	return out
}

// Record appends an event and updates the incremental aggregates.
func (s *Store) Record(ev *Event) {
	if ev == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.events = append(s.events, ev)
	s.apply(ev)

	// Append to the JSONL journal (best-effort).
	line, err := json.Marshal(ev)
	if err == nil {
		_ = os.MkdirAll(s.dir, 0755)
		f, err := os.OpenFile(filepath.Join(s.dir, "usage_events.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			f.Write(append(line, '\n'))
			f.Close()
		}
	}

	// Cap in-memory events to bound memory.
	if len(s.events) > 20000 {
		s.events = s.events[len(s.events)-20000:]
	}
}

func (s *Store) apply(ev *Event) {
	day := ev.Timestamp.Format("2006-01-02")
	d := s.byDay[day]
	if d == nil {
		d = &DayAgg{}
		s.byDay[day] = d
	}
	addDay(d, ev)

	addAgg(s.byWorkspace, ev.WorkspaceName, ev)
	addAgg(s.byAgent, ev.Agent, ev)
	addAgg(s.byProvider, ev.Provider, ev)
	addAgg(s.byModel, ev.Model, ev)
}

func addDay(d *DayAgg, ev *Event) {
	d.Requests++
	d.InputTokens += ev.InputTokens
	d.OutputTokens += ev.OutputTokens
	d.CachedTokens += ev.CachedTokens
	d.ThinkingTokens += ev.ThinkingTokens
	d.LatencyTotalMS += ev.LatencyMS
	d.ToolCalls += int64(ev.ToolCalls)
	d.CostUSD += ev.CostUSD
	if !ev.Success {
		d.Failures++
	}
}

func addAgg(m map[string]*Agg, key string, ev *Event) {
	if key == "" {
		key = "unknown"
	}
	a := m[key]
	if a == nil {
		a = &Agg{}
		m[key] = a
	}
	a.Requests++
	a.InputTokens += ev.InputTokens
	a.OutputTokens += ev.OutputTokens
	a.CachedTokens += ev.CachedTokens
	a.ThinkingTokens += ev.ThinkingTokens
	a.LatencyTotalMS += ev.LatencyMS
	a.ToolCalls += int64(ev.ToolCalls)
	a.CostUSD += ev.CostUSD
	if !ev.Success {
		a.Failures++
	}
}

// RecordToolCall records one tool execution (for tool analytics).
func (s *Store) RecordToolCall(toolName string, success bool, latencyMS int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if toolName == "" {
		toolName = "unknown"
	}
	t := s.byTool[toolName]
	if t == nil {
		t = &ToolAgg{}
		s.byTool[toolName] = t
	}
	t.Calls++
	t.LatencyTotalMS += latencyMS
	if !success {
		t.Failures++
	}
}

// ---------------------------------------------------------------------------
// Query helpers (frontend-facing, all read-only).
// ---------------------------------------------------------------------------

// Overview is the top-level health KPI response.
type Overview struct {
	Requests      int64   `json:"requests"`
	InputTokens   int64   `json:"input_tokens"`
	OutputTokens  int64   `json:"output_tokens"`
	CachedTokens  int64   `json:"cached_tokens"`
	CacheHitRate  float64 `json:"cache_hit_rate"`
	LatencyP95MS  int64   `json:"latency_p95_ms"`
	AvgToolCalls  float64 `json:"avg_tool_calls"`
	CostUSD       float64 `json:"cost_usd"`
}

// DayPoint is one point in a time-series chart.
type DayPoint struct {
	Date         string `json:"date"`
	Requests     int64  `json:"requests"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	CachedTokens int64  `json:"cached_tokens"`
	CostUSD      float64 `json:"cost_usd"`
	Failures     int64  `json:"failures"`
}

// RequestRow is one row in the request table.
type RequestRow struct {
	ID            string    `json:"id"`
	Timestamp     time.Time `json:"timestamp"`
	Workspace     string    `json:"workspace"`
	Agent         string    `json:"agent"`
	Provider      string    `json:"provider"`
	Model         string    `json:"model"`
	InputTokens   int64     `json:"input_tokens"`
	OutputTokens  int64     `json:"output_tokens"`
	CachedTokens  int64     `json:"cached_tokens"`
	LatencyMS     int64     `json:"latency_ms"`
	ToolCalls     int       `json:"tool_calls"`
	CostUSD       float64   `json:"cost_usd"`
	Success       bool      `json:"success"`
	RetryCount    int       `json:"retry_count"`
}

// Bucket is a generic ranked rollup (workspaces/agents/providers/models).
type Bucket struct {
	Key           string  `json:"key"`
	Requests      int64   `json:"requests"`
	InputTokens   int64   `json:"input_tokens"`
	OutputTokens  int64   `json:"output_tokens"`
	CachedTokens  int64   `json:"cached_tokens"`
	CacheHitRate  float64 `json:"cache_hit_rate"`
	CostUSD       float64 `json:"cost_usd"`
	Failures      int64   `json:"failures"`
	AvgLatencyMS  int64   `json:"avg_latency_ms"`
}

// FilterOptions lists all distinct filter values.
type FilterOptions struct {
	Workspaces []string `json:"workspaces"`
	Agents     []string `json:"agents"`
	Providers  []string `json:"providers"`
	Models     []string `json:"models"`
}

// Overview returns KPIs over events within [from, to].
func (s *Store) Overview(from, to time.Time) Overview {
	s.mu.Lock()
	defer s.mu.Unlock()
	var o Overview
	var latencies []int64
	var toolCalls int64
	for _, ev := range s.events {
		if !ev.Timestamp.After(from) || !ev.Timestamp.Before(to) {
			continue
		}
		o.Requests++
		o.InputTokens += ev.InputTokens
		o.OutputTokens += ev.OutputTokens
		o.CachedTokens += ev.CachedTokens
		o.CostUSD += ev.CostUSD
		toolCalls += int64(ev.ToolCalls)
		latencies = append(latencies, ev.LatencyMS)
	}
	if o.InputTokens+o.CachedTokens > 0 {
		o.CacheHitRate = float64(o.CachedTokens) / float64(o.InputTokens+o.CachedTokens) * 100
	}
	if o.Requests > 0 {
		o.AvgToolCalls = float64(toolCalls) / float64(o.Requests)
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	if n := len(latencies); n > 0 {
		idx := int(float64(n) * 0.95)
		if idx >= n {
			idx = n - 1
		}
		o.LatencyP95MS = latencies[idx]
	}
	return o
}

// TimeSeries returns per-day aggregates within [from, to].
func (s *Store) TimeSeries(from, to time.Time) []DayPoint {
	s.mu.Lock()
	defer s.mu.Unlock()
	var points []DayPoint
	days := make(map[string]*DayPoint)
	for _, ev := range s.events {
		if !ev.Timestamp.After(from) || !ev.Timestamp.Before(to) {
			continue
		}
		day := ev.Timestamp.Format("2006-01-02")
		p := days[day]
		if p == nil {
			p = &DayPoint{Date: day}
			days[day] = p
		}
		p.Requests++
		p.InputTokens += ev.InputTokens
		p.OutputTokens += ev.OutputTokens
		p.CachedTokens += ev.CachedTokens
		p.CostUSD += ev.CostUSD
		if !ev.Success {
			p.Failures++
		}
	}
	keys := make([]string, 0, len(days))
	for k := range days {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		points = append(points, *days[k])
	}
	return points
}

// Requests returns request rows within [from, to], newest first.
func (s *Store) Requests(from, to time.Time, limit int) []RequestRow {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 {
		limit = 200
	}
	var rows []RequestRow
	for i := len(s.events) - 1; i >= 0 && len(rows) < limit; i-- {
		ev := s.events[i]
		if !ev.Timestamp.After(from) || !ev.Timestamp.Before(to) {
			continue
		}
		rows = append(rows, RequestRow{
			ID:           ev.ID,
			Timestamp:    ev.Timestamp,
			Workspace:    ev.WorkspaceName,
			Agent:        ev.Agent,
			Provider:     ev.Provider,
			Model:        ev.Model,
			InputTokens:  ev.InputTokens,
			OutputTokens: ev.OutputTokens,
			CachedTokens: ev.CachedTokens,
			LatencyMS:    ev.LatencyMS,
			ToolCalls:    ev.ToolCalls,
			CostUSD:      ev.CostUSD,
			Success:      ev.Success,
			RetryCount:   ev.RetryCount,
		})
	}
	return rows
}

// Buckets returns ranked rollups (byDimension: workspace|agent|provider|model).
func (s *Store) Buckets(dimension string, from, to time.Time) []Bucket {
	s.mu.Lock()
	defer s.mu.Unlock()
	agg := make(map[string]*Agg)
	for _, ev := range s.events {
		if !ev.Timestamp.After(from) || !ev.Timestamp.Before(to) {
			continue
		}
		var key string
		switch dimension {
		case "workspace":
			key = ev.WorkspaceName
		case "agent":
			key = ev.Agent
		case "provider":
			key = ev.Provider
		case "model":
			key = ev.Model
		default:
			key = ev.WorkspaceName
		}
		if key == "" {
			key = "unknown"
		}
		a := agg[key]
		if a == nil {
			a = &Agg{}
			agg[key] = a
		}
		a.Requests++
		a.InputTokens += ev.InputTokens
		a.OutputTokens += ev.OutputTokens
		a.CachedTokens += ev.CachedTokens
		a.ThinkingTokens += ev.ThinkingTokens
		a.LatencyTotalMS += ev.LatencyMS
		a.ToolCalls += int64(ev.ToolCalls)
		a.CostUSD += ev.CostUSD
		if !ev.Success {
			a.Failures++
		}
	}
	var out []Bucket
	for k, a := range agg {
		b := Bucket{
			Key:          k,
			Requests:     a.Requests,
			InputTokens:  a.InputTokens,
			OutputTokens: a.OutputTokens,
			CachedTokens: a.CachedTokens,
			CostUSD:      a.CostUSD,
			Failures:     a.Failures,
		}
		if a.InputTokens+a.CachedTokens > 0 {
			b.CacheHitRate = float64(a.CachedTokens) / float64(a.InputTokens+a.CachedTokens) * 100
		}
		if a.Requests > 0 {
			b.AvgLatencyMS = a.LatencyTotalMS / a.Requests
		}
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Requests > out[j].Requests })
	return out
}

// FilterOptions returns distinct filter values.
func (s *Store) FilterOptions() FilterOptions {
	s.mu.Lock()
	defer s.mu.Unlock()
	var fo FilterOptions
	ws, ag, pr, mo := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, ev := range s.events {
		if ev.WorkspaceName != "" {
			ws[ev.WorkspaceName] = true
		}
		if ev.Agent != "" {
			ag[ev.Agent] = true
		}
		if ev.Provider != "" {
			pr[ev.Provider] = true
		}
		if ev.Model != "" {
			mo[ev.Model] = true
		}
	}
	for k := range ws {
		fo.Workspaces = append(fo.Workspaces, k)
	}
	for k := range ag {
		fo.Agents = append(fo.Agents, k)
	}
	for k := range pr {
		fo.Providers = append(fo.Providers, k)
	}
	for k := range mo {
		fo.Models = append(fo.Models, k)
	}
	sort.Strings(fo.Workspaces)
	sort.Strings(fo.Agents)
	sort.Strings(fo.Providers)
	sort.Strings(fo.Models)
	return fo
}
