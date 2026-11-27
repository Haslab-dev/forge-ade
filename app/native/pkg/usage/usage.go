package usage

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// TimeRange presets
const (
	TimeRange1H     = "1h"
	TimeRange1D     = "1d"
	TimeRange7D     = "7d"
	TimeRange1Month = "1m"
	TimeRangeCustom = "custom"
)

// UsageEntry records a discrete token usage turn or session checkpoint.
type UsageEntry struct {
	ID           string    `json:"id"`
	Timestamp    time.Time `json:"timestamp"`
	SessionID    string    `json:"sessionId"`
	SessionTitle string    `json:"sessionTitle,omitempty"`
	Model        string    `json:"model"`
	Provider     string    `json:"provider"`
	InputTokens  int       `json:"inputTokens"`
	OutputTokens int       `json:"outputTokens"`
	CachedTokens int       `json:"cachedTokens"`
	TotalTokens  int       `json:"totalTokens"`
	CostUSD      float64   `json:"costUsd,omitempty"`
}

// QueryOptions filters token usage summaries across dimensions.
type QueryOptions struct {
	TimeRange      string    // "1h", "1d", "7d", "1m", "custom"
	CustomStart    time.Time // Used when TimeRange == "custom"
	CustomEnd      time.Time // Used when TimeRange == "custom"
	FilterProvider string    // "" or "all" for no provider filter
	FilterModel    string    // "" or "all" for no model filter
}

// ModelUsage provides aggregated statistics for one model.
type ModelUsage struct {
	Model        string  `json:"model"`
	Provider     string  `json:"provider"`
	InputTokens  int     `json:"inputTokens"`
	OutputTokens int     `json:"outputTokens"`
	CachedTokens int     `json:"cachedTokens"`
	TotalTokens  int     `json:"totalTokens"`
	CacheHitRate float64 `json:"cacheHitRate"` // 0.0 - 100.0%
	PercentShare float64 `json:"percentShare"` // 0.0 - 100.0% of total filtered tokens
	CostUSD      float64 `json:"costUsd"`
	Turns        int     `json:"turns"`
	Sessions     int     `json:"sessions"`
}

// ProviderUsage provides aggregated statistics for one provider.
type ProviderUsage struct {
	Provider     string   `json:"provider"`
	ModelCount   int      `json:"modelCount"`
	Models       []string `json:"models"`
	InputTokens  int      `json:"inputTokens"`
	OutputTokens int      `json:"outputTokens"`
	CachedTokens int      `json:"cachedTokens"`
	TotalTokens  int      `json:"totalTokens"`
	CacheHitRate float64  `json:"cacheHitRate"` // 0.0 - 100.0%
	PercentShare float64  `json:"percentShare"` // 0.0 - 100.0% of total filtered tokens
	CostUSD      float64  `json:"costUsd"`
	Turns        int      `json:"turns"`
	Sessions     int      `json:"sessions"`
}

// UsageSummary aggregates total metrics and breakdowns for a given query window.
type UsageSummary struct {
	TotalInputTokens   int             `json:"totalInputTokens"`
	TotalOutputTokens  int             `json:"totalOutputTokens"`
	TotalCachedTokens  int             `json:"totalCachedTokens"`
	TotalTokens        int             `json:"totalTokens"`
	CacheHitRate       float64         `json:"cacheHitRate"` // 0.0 - 100.0%
	TotalCostUSD       float64         `json:"totalCostUsd"`
	TotalTurns         int             `json:"totalTurns"`
	TotalSessions      int             `json:"totalSessions"`
	ByModel            []ModelUsage    `json:"byModel"`
	ByProvider         []ProviderUsage `json:"byProvider"`
	AvailableProviders []string        `json:"availableProviders"`
	AvailableModels    []string        `json:"availableModels"`
	StartTime          time.Time       `json:"startTime"`
	EndTime            time.Time       `json:"endTime"`
}

// UsageManager tracks and aggregates token usage records in memory and on disk.
type UsageManager struct {
	mu           sync.RWMutex
	dataDir      string
	filePath     string
	entries      []UsageEntry
	knownIDs     map[string]bool
	scannedFiles map[string]int64
	dirty        bool
	saveTimer    *time.Timer
	provResolver func(model, provID string) string
}

// NewUsageManager initializes the token usage ledger and backfills historical sessions.
func NewUsageManager(dataDir string, provResolver func(model, provID string) string) *UsageManager {
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".forge-ade")
	}
	_ = os.MkdirAll(dataDir, 0755)

	um := &UsageManager{
		dataDir:      dataDir,
		filePath:     filepath.Join(dataDir, "usage_ledger.json"),
		knownIDs:     make(map[string]bool),
		scannedFiles: make(map[string]int64),
		provResolver: provResolver,
	}

	um.loadLedger()
	return um
}

// SetProviderResolver updates the callback used to map profile IDs to friendly provider names.
func (um *UsageManager) SetProviderResolver(fn func(model, provID string) string) {
	um.mu.Lock()
	defer um.mu.Unlock()
	um.provResolver = fn
}

// loadLedger reads persisted usage records from disk.
func (um *UsageManager) loadLedger() {
	data, err := os.ReadFile(um.filePath)
	if err != nil || len(data) == 0 {
		return
	}

	var list []UsageEntry
	if err := json.Unmarshal(data, &list); err == nil {
		for _, e := range list {
			if e.ID != "" && !um.knownIDs[e.ID] {
				um.knownIDs[e.ID] = true
				um.entries = append(um.entries, e)
			}
		}
	}
}

// saveLedgerLocked flushes entries to disk atomically.
func (um *UsageManager) saveLedgerLocked() {
	if !um.dirty {
		return
	}
	um.dirty = false

	data, err := json.MarshalIndent(um.entries, "", "  ")
	if err != nil {
		return
	}
	tmp := um.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err == nil {
		_ = os.Rename(tmp, um.filePath)
	}
}

func (um *UsageManager) scheduleSave() {
	um.mu.Lock()
	defer um.mu.Unlock()
	um.dirty = true
	if um.saveTimer != nil {
		um.saveTimer.Stop()
	}
	um.saveTimer = time.AfterFunc(600*time.Millisecond, func() {
		um.mu.Lock()
		defer um.mu.Unlock()
		um.saveLedgerLocked()
	})
}

// RecordTurn records a discrete turn usage entry.
func (um *UsageManager) RecordTurn(sessionID, title, model, provider string, inputTokens, outputTokens, cachedTokens, totalTokens int, costUSD float64) {
	if inputTokens <= 0 && outputTokens <= 0 && totalTokens <= 0 {
		return
	}
	if totalTokens == 0 {
		totalTokens = inputTokens + outputTokens
	}

	resolvedProv := um.ResolveProvider(model, provider)
	resolvedModel := strings.TrimSpace(model)
	if resolvedModel == "" {
		resolvedModel = "default-model"
	}

	entryID := fmt.Sprintf("use-%d-%s", time.Now().UnixNano(), sessionID)
	entry := UsageEntry{
		ID:           entryID,
		Timestamp:    time.Now(),
		SessionID:    sessionID,
		SessionTitle: title,
		Model:        resolvedModel,
		Provider:     resolvedProv,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		CachedTokens: cachedTokens,
		TotalTokens:  totalTokens,
		CostUSD:      costUSD,
	}

	um.mu.Lock()
	um.entries = append(um.entries, entry)
	um.knownIDs[entryID] = true
	um.mu.Unlock()

	um.scheduleSave()
}

// ResolveProvider normalizes provider identifiers into clean user-facing brand labels.
func (um *UsageManager) ResolveProvider(model, provID string) string {
	if um.provResolver != nil {
		if resolved := um.provResolver(model, provID); resolved != "" {
			return resolved
		}
	}
	return DefaultProviderName(model, provID)
}

// DefaultProviderName applies rule-based inference for provider names.
func DefaultProviderName(model, provID string) string {
	provLower := strings.ToLower(provID)
	modelLower := strings.ToLower(model)

	switch {
	case strings.Contains(provLower, "agy") || strings.Contains(provLower, "antigravity"):
		return "Antigravity (Google)"
	case strings.Contains(provLower, "claude") || strings.Contains(modelLower, "claude"):
		return "Anthropic"
	case strings.Contains(provLower, "codex") || strings.Contains(modelLower, "o3") || strings.Contains(modelLower, "gpt"):
		return "OpenAI"
	case strings.Contains(provLower, "commandcode") || strings.Contains(provLower, "cohere") || strings.Contains(modelLower, "command"):
		return "Command Code (Cohere)"
	case strings.Contains(provLower, "deepseek") || strings.Contains(modelLower, "deepseek"):
		return "DeepSeek"
	case strings.Contains(provLower, "kenari") || strings.Contains(provLower, "zhipu") || strings.Contains(modelLower, "glm"):
		return "Zhipu AI (GLM)"
	case strings.Contains(provLower, "ollama"):
		return "Ollama (Local)"
	case strings.Contains(provLower, "ohmypi") || strings.Contains(modelLower, "ohmypi"):
		return "OhMyPi"
	case strings.Contains(provLower, "internal") || strings.Contains(provLower, "agent-internal"):
		if strings.Contains(modelLower, "deepseek") {
			return "DeepSeek"
		}
		if strings.Contains(modelLower, "glm") {
			return "Zhipu AI (GLM)"
		}
		if strings.Contains(modelLower, "gemini") {
			return "Google Gemini"
		}
		return "ForgeADE Internal"
	case strings.Contains(modelLower, "gemini"):
		return "Google Gemini"
	case provID != "":
		return provID
	default:
		return "Default Provider"
	}
}

// BackfillHistoricalSessions scans saved session files under provided directories
// (or ~/.forge-ade/sessions/ if none given) and registers past usage if not already tracked in the ledger.
func (um *UsageManager) BackfillHistoricalSessions(customDirs ...string) {
	var searchDirs []string
	if len(customDirs) > 0 {
		searchDirs = customDirs
	} else {
		home, _ := os.UserHomeDir()
		searchDirs = []string{
			filepath.Join(home, ".forge-ade", "sessions"),
			filepath.Join(um.dataDir, "sessions"),
		}
	}

	type diskUsageRaw struct {
		InputTokens  int     `json:"inputTokens"`
		OutputTokens int     `json:"outputTokens"`
		CachedTokens int     `json:"cachedTokens"`
		TotalTokens  int     `json:"totalTokens"`
		CostUSD      float64 `json:"costUsd"`
	}

	type diskMsgRaw struct {
		Role      string          `json:"role"`
		Content   json.RawMessage `json:"content"`
		Text      string          `json:"text"`
		Thinking  string          `json:"thinking"`
		Thoughts  []string        `json:"thoughts"`
		Timestamp string          `json:"timestamp"`
		Tools     []struct {
			Output string `json:"output"`
		} `json:"toolExecutions"`
	}

	type diskSessRaw struct {
		ID        string        `json:"id"`
		Title     string        `json:"title"`
		Model     string        `json:"model"`
		Provider  string        `json:"provider"`
		AgentID   string        `json:"agentId"`
		CreatedAt string        `json:"createdAt"`
		UpdatedAt string        `json:"updatedAt"`
		LastUsage *diskUsageRaw `json:"lastUsage"`
		Messages  []diskMsgRaw  `json:"messages"`
	}

	usageRegex := regexp.MustCompile(`total_tokens:\s*(\d+)`)

	var imported int
	for _, sDir := range searchDirs {
		if _, err := os.Stat(sDir); os.IsNotExist(err) {
			continue
		}
		_ = filepath.Walk(sDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(info.Name(), ".json") || info.Name() == "info.json" {
				return nil
			}

			// Fast-path skip BEFORE reading/unmarshaling JSON from disk:
			// 1. Session files are named "<sessionID>.json" -> check knownIDs by filename stem.
			// 2. Skip unchanged files already scanned in this process.
			stem := strings.TrimSuffix(info.Name(), ".json")
			modNano := info.ModTime().UnixNano()
			um.mu.RLock()
			skipKnown := um.knownIDs["sess-"+stem] || um.knownIDs[stem]
			skipScanned := um.scannedFiles != nil && um.scannedFiles[path] == modNano
			um.mu.RUnlock()
			if skipKnown || skipScanned {
				return nil
			}

			data, rerr := os.ReadFile(path)
			if rerr != nil {
				return nil
			}

			um.mu.Lock()
			if um.scannedFiles == nil {
				um.scannedFiles = make(map[string]int64)
			}
			um.scannedFiles[path] = modNano
			um.mu.Unlock()

			var s diskSessRaw
			if json.Unmarshal(data, &s) != nil || s.ID == "" {
				return nil
			}

			// Check if session is already recorded
			ledgerKey := "sess-" + s.ID
			um.mu.RLock()
			alreadyKnown := um.knownIDs[ledgerKey]
			um.mu.RUnlock()
			if alreadyKnown {
				return nil
			}

			// Determine timestamp
			ts := time.Now()
			if s.UpdatedAt != "" {
				if t, terr := time.Parse(time.RFC3339, s.UpdatedAt); terr == nil {
					ts = t
				}
			} else if s.CreatedAt != "" {
				if t, terr := time.Parse(time.RFC3339, s.CreatedAt); terr == nil {
					ts = t
				}
			}

			modelName := s.Model
			if modelName == "" {
				modelName = s.AgentID
			}
			if modelName == "" {
				modelName = "agent-model"
			}

			provID := s.Provider
			if provID == "" {
				provID = s.AgentID
			}
			resolvedProv := um.ResolveProvider(modelName, provID)

			var inputTokens, outputTokens, cachedTokens, totalTokens int
			var costUSD float64

			if s.LastUsage != nil && (s.LastUsage.TotalTokens > 0 || s.LastUsage.InputTokens > 0) {
				inputTokens = s.LastUsage.InputTokens
				outputTokens = s.LastUsage.OutputTokens
				cachedTokens = s.LastUsage.CachedTokens
				totalTokens = s.LastUsage.TotalTokens
				costUSD = s.LastUsage.CostUSD
				if totalTokens == 0 {
					totalTokens = inputTokens + outputTokens
				}
			} else {
				// Estimate token volume from message contents & tool outputs
				var chars int
				for _, m := range s.Messages {
					chars += len(m.Text) + len(m.Thinking)
					for _, th := range m.Thoughts {
						chars += len(th)
					}
					for _, t := range m.Tools {
						chars += len(t.Output)
						if m := usageRegex.FindStringSubmatch(t.Output); len(m) > 1 {
							if n, _ := strconv.Atoi(m[1]); n > totalTokens {
								totalTokens = n
							}
						}
					}
				}

				if totalTokens == 0 {
					totalTokens = max(250, chars/4)
				}
				inputTokens = int(float64(totalTokens) * 0.72)
				outputTokens = totalTokens - inputTokens
				cachedTokens = int(float64(inputTokens) * 0.38)
			}

			entry := UsageEntry{
				ID:           ledgerKey,
				Timestamp:    ts,
				SessionID:    s.ID,
				SessionTitle: s.Title,
				Model:        modelName,
				Provider:     resolvedProv,
				InputTokens:  inputTokens,
				OutputTokens: outputTokens,
				CachedTokens: cachedTokens,
				TotalTokens:  totalTokens,
				CostUSD:      costUSD,
			}

			um.mu.Lock()
			um.entries = append(um.entries, entry)
			um.knownIDs[ledgerKey] = true
			um.mu.Unlock()
			imported++
			return nil
		})
	}

	if imported > 0 {
		um.mu.Lock()
		// Sort entries chronologically
		sort.Slice(um.entries, func(i, j int) bool {
			return um.entries[i].Timestamp.Before(um.entries[j].Timestamp)
		})
		um.mu.Unlock()
		um.scheduleSave()
	}
}

// Query calculates usage totals, cache hit rate %, and groupings for the requested window.
func (um *UsageManager) Query(opts QueryOptions) UsageSummary {
	um.mu.RLock()
	defer um.mu.RUnlock()

	now := time.Now()
	var startBound, endBound time.Time

	switch opts.TimeRange {
	case TimeRange1H:
		startBound = now.Add(-1 * time.Hour)
		endBound = now
	case TimeRange1D:
		startBound = now.Add(-24 * time.Hour)
		endBound = now
	case TimeRange7D:
		startBound = now.Add(-7 * 24 * time.Hour)
		endBound = now
	case TimeRange1Month:
		startBound = now.Add(-30 * 24 * time.Hour)
		endBound = now
	case TimeRangeCustom:
		if !opts.CustomStart.IsZero() {
			startBound = opts.CustomStart
		} else {
			startBound = time.Time{}
		}
		if !opts.CustomEnd.IsZero() {
			endBound = opts.CustomEnd
		} else {
			endBound = now
		}
	default:
		// Default to 7 days
		startBound = now.Add(-7 * 24 * time.Hour)
		endBound = now
	}

	modelsMap := make(map[string]*ModelUsage)
	providersMap := make(map[string]*ProviderUsage)
	provModelsMap := make(map[string]map[string]bool)

	availModelsMap := make(map[string]bool)
	availProvidersMap := make(map[string]bool)

	var totalInput, totalOutput, totalCached, totalTokens int
	var totalCost float64
	var totalTurns int
	sessionsSeen := make(map[string]bool)

	for _, e := range um.entries {
		// Populate all known options for filters
		if e.Model != "" {
			availModelsMap[e.Model] = true
		}
		if e.Provider != "" {
			availProvidersMap[e.Provider] = true
		}

		// Filter time window
		if !startBound.IsZero() && e.Timestamp.Before(startBound) {
			continue
		}
		if !endBound.IsZero() && e.Timestamp.After(endBound) {
			continue
		}

		// Filter provider
		if opts.FilterProvider != "" && opts.FilterProvider != "all" && !strings.EqualFold(e.Provider, opts.FilterProvider) {
			continue
		}

		// Filter model
		if opts.FilterModel != "" && opts.FilterModel != "all" && !strings.EqualFold(e.Model, opts.FilterModel) {
			continue
		}

		totalInput += e.InputTokens
		totalOutput += e.OutputTokens
		totalCached += e.CachedTokens
		totalTokens += e.TotalTokens
		totalCost += e.CostUSD
		totalTurns++
		if e.SessionID != "" {
			sessionsSeen[e.SessionID] = true
		}

		// Aggregate by Model
		mKey := e.Model
		if mKey == "" {
			mKey = "Unknown Model"
		}
		mu, ok := modelsMap[mKey]
		if !ok {
			mu = &ModelUsage{
				Model:    mKey,
				Provider: e.Provider,
			}
			modelsMap[mKey] = mu
		}
		mu.InputTokens += e.InputTokens
		mu.OutputTokens += e.OutputTokens
		mu.CachedTokens += e.CachedTokens
		mu.TotalTokens += e.TotalTokens
		mu.CostUSD += e.CostUSD
		mu.Turns++
		if e.SessionID != "" {
			mu.Sessions++
		}

		// Aggregate by Provider
		pKey := e.Provider
		if pKey == "" {
			pKey = "Unknown Provider"
		}
		pu, pok := providersMap[pKey]
		if !pok {
			pu = &ProviderUsage{
				Provider: pKey,
			}
			providersMap[pKey] = pu
			provModelsMap[pKey] = make(map[string]bool)
		}
		pu.InputTokens += e.InputTokens
		pu.OutputTokens += e.OutputTokens
		pu.CachedTokens += e.CachedTokens
		pu.TotalTokens += e.TotalTokens
		pu.CostUSD += e.CostUSD
		pu.Turns++
		if e.SessionID != "" {
			pu.Sessions++
		}
		if e.Model != "" {
			provModelsMap[pKey][e.Model] = true
		}
	}

	// Calculate overall cache hit rate %
	overallHitRate := computeHitRatePct(totalInput, totalCached)

	// Format ByModel slice
	byModel := make([]ModelUsage, 0, len(modelsMap))
	for _, mu := range modelsMap {
		mu.CacheHitRate = computeHitRatePct(mu.InputTokens, mu.CachedTokens)
		if totalTokens > 0 {
			mu.PercentShare = (float64(mu.TotalTokens) / float64(totalTokens)) * 100.0
		}
		byModel = append(byModel, *mu)
	}
	sort.Slice(byModel, func(i, j int) bool {
		return byModel[i].TotalTokens > byModel[j].TotalTokens
	})

	// Format ByProvider slice
	byProvider := make([]ProviderUsage, 0, len(providersMap))
	for pKey, pu := range providersMap {
		pu.CacheHitRate = computeHitRatePct(pu.InputTokens, pu.CachedTokens)
		if totalTokens > 0 {
			pu.PercentShare = (float64(pu.TotalTokens) / float64(totalTokens)) * 100.0
		}
		pu.ModelCount = len(provModelsMap[pKey])
		for m := range provModelsMap[pKey] {
			pu.Models = append(pu.Models, m)
		}
		sort.Strings(pu.Models)
		byProvider = append(byProvider, *pu)
	}
	sort.Slice(byProvider, func(i, j int) bool {
		return byProvider[i].TotalTokens > byProvider[j].TotalTokens
	})

	availModels := make([]string, 0, len(availModelsMap))
	for m := range availModelsMap {
		availModels = append(availModels, m)
	}
	sort.Strings(availModels)

	availProviders := make([]string, 0, len(availProvidersMap))
	for p := range availProvidersMap {
		availProviders = append(availProviders, p)
	}
	sort.Strings(availProviders)

	return UsageSummary{
		TotalInputTokens:   totalInput,
		TotalOutputTokens:  totalOutput,
		TotalCachedTokens:  totalCached,
		TotalTokens:        totalTokens,
		CacheHitRate:       overallHitRate,
		TotalCostUSD:       totalCost,
		TotalTurns:         totalTurns,
		TotalSessions:      len(sessionsSeen),
		ByModel:            byModel,
		ByProvider:         byProvider,
		AvailableProviders: availProviders,
		AvailableModels:    availModels,
		StartTime:          startBound,
		EndTime:            endBound,
	}
}

// computeHitRatePct calculates cache hit rate percentage safely.
func computeHitRatePct(input, cached int) float64 {
	if input <= 0 || cached <= 0 {
		return 0.0
	}
	denom := float64(input)
	if cached > input {
		denom = float64(input + cached)
	}
	rate := (float64(cached) / denom) * 100.0
	return math.Min(100.0, math.Max(0.0, rate))
}
