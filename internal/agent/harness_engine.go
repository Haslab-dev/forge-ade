package agent

import (
	"context"
	"sync"

	"github.com/hasdev/forge-ade/internal/harness"
)

// Harness-scheduled tool batching: this wires the harness tool scheduler
// (internal/harness ScheduleTools) into ForgeADE's turn loop. Read-only /
// concurrent-safe calls run in parallel groups (bounded, mirroring the harness
// executor default of 10); mutating or unknown-classification calls run in
// sequential singleton groups, preserving ForgeADE's approval-gate semantics.

// maxParallelTools mirrors the harness executor's default maxConcurrency.
const maxParallelTools = 10

// schedRegistry exposes tool scheduling metadata (read-only / destructive /
// side-effect scope) to the harness scheduler without execution handlers.
// Classification mirrors ForgeADE's own isReadOnlyTool/isMutatingTool so the
// approval gate and the scheduler agree.
func (m *Manager) schedRegistry() *harness.ToolRegistry {
	reg := harness.NewToolRegistry()
	if m.toolReg == nil {
		return reg
	}
	for _, def := range m.toolReg.Definitions() {
		name := def.Function.Name
		if name == "" {
			continue
		}
		meta := harness.ToolMetadata{Name: name}
		switch {
		case isReadOnlyTool(name):
			meta.ReadOnly = true
			meta.SideEffectScope = "none"
		case isMutatingTool(name):
			meta.Destructive = true
			meta.SideEffectScope = "workspace"
		default:
			// Unknown classification: never schedule in parallel.
			meta.SideEffectScope = "workspace"
		}
		_ = reg.Register(&harness.ToolEntry{Metadata: meta})
	}
	return reg
}

// batchState carries dedup + budget accumulation across a scheduled batch.
type batchState struct {
	mu    sync.Mutex
	dedup map[string]dedupEntry
	spent int
	fatal bool
}

type dedupEntry struct {
	content string
	isErr   bool
}

// executeScheduledBatch runs one batch of tool calls under harness scheduler
// semantics and returns the points spent. fatal reports a tool failure the
// same way the previous sequential loop did (turn ends).
func (m *Manager) executeScheduledBatch(ctx context.Context, sessionID string, toolCalls []ContentBlock) (spent int, fatal bool) {
	reg := m.schedRegistry()
	calls := make([]harness.ToolCall, 0, len(toolCalls))
	byID := make(map[string]ContentBlock, len(toolCalls))
	for _, tc := range toolCalls {
		input := map[string]any{}
		if tc.Arguments != nil {
			for k, v := range tc.Arguments {
				input[k] = v
			}
		}
		id := tc.ToolCallID
		calls = append(calls, harness.ToolCall{ID: id, Name: tc.Name, Input: input})
		byID[id] = tc
	}

	sched, err := harness.ScheduleTools(reg, calls, maxParallelTools)
	if err != nil || sched == nil {
		// Fall back to the sequential path on scheduling failure.
		return m.executeBatchSequential(ctx, sessionID, toolCalls)
	}

	st := &batchState{dedup: make(map[string]dedupEntry)}
	for _, group := range sched.ParallelGroups {
		if len(group) == 1 {
			tc := byID[calls[group[0]].ID]
			m.executeOneBatched(ctx, sessionID, tc, st)
			if st.fatal {
				return st.spent, true
			}
			continue
		}
		// Parallel read-only group: dedup identical calls first (run once,
		// mirror the result to every caller id) like the old sequential path.
		var uniq []ContentBlock
		seen := make(map[string][]ContentBlock)
		for _, idx := range group {
			tc := byID[calls[idx].ID]
			if isReadOnlyTool(tc.Name) {
				key := tc.Name + "\x00" + canonicalArgs(tc.Arguments)
				st.mu.Lock()
				_, cached := st.dedup[key]
				st.mu.Unlock()
				if cached {
					continue
				}
				if _, dup := seen[key]; !dup {
					seen[key] = append(seen[key], tc)
					uniq = append(uniq, tc)
				} else {
					seen[key] = append(seen[key], tc)
				}
				continue
			}
			uniq = append(uniq, tc)
		}
		var wg sync.WaitGroup
		for _, tc := range uniq {
			wg.Add(1)
			go func(tc ContentBlock) {
				defer wg.Done()
				m.executeOneBatched(ctx, sessionID, tc, st)
			}(tc)
		}
		wg.Wait()
		if st.fatal {
			return st.spent, true
		}
		// Mirror dedup'd results to duplicate caller ids in this group.
		for key, callers := range seen {
			if len(callers) <= 1 {
				continue
			}
			st.mu.Lock()
			ent := st.dedup[key]
			st.mu.Unlock()
			for _, tc := range callers[1:] {
				m.emitToolEnd(sessionID, tc, ent.content, ent.isErr)
				_ = m.appendToolResult(sessionID, tc, ent.content, ent.isErr)
			}
		}
	}
	return st.spent, st.fatal
}

// executeOneBatched executes one call with read-only dedup + bookkeeping.
func (m *Manager) executeOneBatched(ctx context.Context, sessionID string, tc ContentBlock, st *batchState) {
	key := tc.Name + "\x00" + canonicalArgs(tc.Arguments)
	if isReadOnlyTool(tc.Name) {
		st.mu.Lock()
		ent, ok := st.dedup[key]
		st.mu.Unlock()
		if ok {
			m.emitToolEnd(sessionID, tc, ent.content, ent.isErr)
			_ = m.appendToolResult(sessionID, tc, ent.content, ent.isErr)
			st.mu.Lock()
			st.spent += m.toolCost(tc.Name)
			st.mu.Unlock()
			return
		}
	}
	content, execErr := m.executeToolCall(ctx, sessionID, tc)
	if isReadOnlyTool(tc.Name) {
		st.mu.Lock()
		st.dedup[key] = dedupEntry{content: content, isErr: execErr != nil}
		st.mu.Unlock()
	}
	st.mu.Lock()
	st.spent += m.toolCost(tc.Name)
	if execErr != nil {
		st.fatal = true
	}
	st.mu.Unlock()
}

// executeBatchSequential is the pre-harness sequential path, kept as the
// fallback and for parity testing.
func (m *Manager) executeBatchSequential(ctx context.Context, sessionID string, toolCalls []ContentBlock) (spent int, fatal bool) {
	st := &batchState{dedup: make(map[string]dedupEntry)}
	for _, tc := range toolCalls {
		m.executeOneBatched(ctx, sessionID, tc, st)
		if st.fatal {
			return st.spent, true
		}
	}
	return st.spent, st.fatal
}
