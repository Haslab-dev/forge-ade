package git

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestGetStatusWaiterSurvivesInvalidationDuringWait reproduces the branch
// checkout crash ("fatal error: sync: unlock of unlocked mutex" at
// status.go:54): a GetStatus caller waits on an in-flight fetch, and the
// entry is invalidated out of the cache mid-wait (Invalidate/InvalidateAll
// fire on every file event, so a checkout invalidates constantly). The old
// implementation fell through to the TTL branch with the mutex already
// unlocked and executed a second Unlock. The fetcher also publishes a fresh
// cachedAt right before waking the waiter, which made the TTL branch taken.
func TestGetStatusWaiterSurvivesInvalidationDuringWait(t *testing.T) {
	e := NewEngine()
	repo := t.TempDir()

	// Simulate an in-flight fetch registered by another caller.
	inFlight := &statusEntry{done: make(chan struct{})}
	e.statusCache[repo] = inFlight

	returned := make(chan struct{})
	go func() {
		defer close(returned)
		res, err := e.GetStatus(context.Background(), repo)
		if err != nil {
			t.Errorf("GetStatus returned error: %v", err)
		}
		if res == nil {
			t.Error("GetStatus returned nil result")
		}
	}()

	// Give the waiter time to park on the in-flight done channel.
	time.Sleep(50 * time.Millisecond)

	// While it waits: the fetcher finishes and publishes a FRESH entry, but a
	// concurrent invalidation (file-event sweep) removed the entry from the
	// cache first — the exact interleaving that crashed the old code.
	e.statusMu.Lock()
	delete(e.statusCache, repo)
	inFlight.res = &GitStatusResult{Branch: "main"}
	inFlight.cachedAt = time.Now()
	close(inFlight.done)
	inFlight.done = nil // the engine's publish block nils done after closing
	e.statusMu.Unlock()

	select {
	case <-returned:
	case <-time.After(10 * time.Second):
		t.Fatal("GetStatus waiter did not return (deadlock or double-unlock crash)")
	}
}

// TestGetStatusWaiterFollowsReplacement covers the second latent bug: after
// waiting on one fetch, the caller may find a NEW in-flight fetch registered
// (invalidation + fresh caller). The old code returned its (nil, nil) result
// instead of waiting for the new fetch.
func TestGetStatusWaiterFollowsReplacement(t *testing.T) {
	e := NewEngine()
	repo := t.TempDir()

	first := &statusEntry{done: make(chan struct{})}
	e.statusCache[repo] = first

	returned := make(chan *GitStatusResult, 1)
	go func() {
		res, _ := e.GetStatus(context.Background(), repo)
		returned <- res
	}()

	time.Sleep(50 * time.Millisecond)

	// The first fetch is invalidated; a second caller registers a new
	// in-flight entry before the first fetch completes.
	e.statusMu.Lock()
	delete(e.statusCache, repo)
	second := &statusEntry{done: make(chan struct{})}
	e.statusCache[repo] = second
	e.statusMu.Unlock()

	// First fetch completes with a nil result (it was never published).
	close(first.done)
	first.done = nil

	// Give the waiter a moment to re-check and park on the second fetch.
	time.Sleep(50 * time.Millisecond)

	// The replacement fetch completes with a real result — the waiter must
	// observe it, not the first fetch's nil.
	e.statusMu.Lock()
	second.res = &GitStatusResult{Branch: "expected"}
	second.err = nil
	second.cachedAt = time.Now()
	close(second.done)
	second.done = nil
	e.statusMu.Unlock()

	select {
	case res := <-returned:
		if res == nil || res.Branch != "expected" {
			t.Fatalf("waiter returned stale/nil result instead of the replacement fetch's: %+v", res)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("GetStatus waiter did not return")
	}
}

// TestGetStatusInvalidationStorm hammers GetStatus concurrently with
// invalidations (the checkout/file-event pattern) to shake out any remaining
// lock misuse. Run with -race in CI.
func TestGetStatusInvalidationStorm(t *testing.T) {
	e := NewEngine()
	repo := t.TempDir()

	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				e.InvalidateAll()
				time.Sleep(time.Millisecond)
			}
		}
	}()

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			deadline := time.Now().Add(2 * time.Second)
			for time.Now().Before(deadline) {
				_, _ = e.GetStatus(context.Background(), repo)
			}
		}()
	}
	wg.Wait()
	close(stop)
}
