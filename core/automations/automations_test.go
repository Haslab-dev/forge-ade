package automations

import (
	"context"
	"testing"
	"time"
)

// ── cron parser ─────────────────────────────────────────────────────────────

func mustParse(t *testing.T, expr string) *cronExpr {
	t.Helper()
	c, err := parseCron(expr)
	if err != nil {
		t.Fatalf("parseCron(%q) unexpected error: %v", expr, err)
	}
	return c
}

func TestParseCronValid(t *testing.T) {
	cases := []string{
		"* * * * *",
		"0 9 * * *",
		"*/15 * * * *",
		"30 9 * * 1-5",
		"0 9,18 * * *",
		"0 9 * * 0",
		"0 9 * * 7",
		"5 8 1 * *",
		"0-30/10 12 * * *",
		"0 9 1,15 * *",
	}
	for _, expr := range cases {
		mustParse(t, expr)
	}
}

func TestParseCronInvalid(t *testing.T) {
	cases := []string{
		"", "9:00", "* * * *", "* * * * * *", "60 * * * *", "* 24 * * *",
		"* * 32 * *", "* * * 13 *", "* * * * 8x", "a * * * *", "*/0 * * * *",
	}
	for _, expr := range cases {
		if _, err := parseCron(expr); err == nil {
			t.Errorf("parseCron(%q) expected error, got nil", expr)
		}
	}
}

func TestCronNext(t *testing.T) {
	// Fixed reference: Wednesday 2026-10-07 10:30 local.
	base := time.Date(2026, 10, 7, 10, 30, 0, 0, time.Local)

	cases := []struct {
		name string
		expr string
		want time.Time
	}{
		{"next minute wildcard", "* * * * *", base.Add(time.Minute)},
		{"daily at 09:00 → today is past, tomorrow", "0 9 * * *", time.Date(2026, 10, 8, 9, 0, 0, 0, time.Local)},
		{"daily at 10:30 exactly → +1 day (strictly after)", "30 10 * * *", time.Date(2026, 10, 8, 10, 30, 0, 0, time.Local)},
		{"hourly at :15 → 11:15 same day", "15 * * * *", time.Date(2026, 10, 7, 11, 15, 0, 0, time.Local)},
		{"every 15 minutes", "*/15 * * * *", time.Date(2026, 10, 7, 10, 45, 0, 0, time.Local)},
		{"weekdays: Thu after Wed", "0 9 * * 1-5", time.Date(2026, 10, 8, 9, 0, 0, 0, time.Local)},
		{"weekly Monday: skips to Mon 10-12", "0 9 * * 1", time.Date(2026, 10, 12, 9, 0, 0, 0, time.Local)},
		{"dow 7 = sunday: Sun 10-11", "0 9 * * 7", time.Date(2026, 10, 11, 9, 0, 0, 0, time.Local)},
		{"monthly on the 1st: Nov 1", "0 9 1 * *", time.Date(2026, 11, 1, 9, 0, 0, 0, time.Local)},
		{"dom and dow both restricted ORs", "0 9 7 * 1", time.Date(2026, 10, 12, 9, 0, 0, 0, time.Local)}, // Wed 7th matches dom today? strictly-after → next: Thu? no — dom=7, dow=1 → Mon 12th (next match after base; Wed 7 already passed 9:00)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := mustParse(t, tc.expr)
			got := c.next(base)
			if got.IsZero() {
				t.Fatalf("next() returned zero time")
			}
			if !got.Equal(tc.want) {
				t.Errorf("next(%q) from %s = %s, want %s", tc.expr, base, got, tc.want)
			}
		})
	}
}

func TestCronNextNeverReturnsAfterBoundary(t *testing.T) {
	c := mustParse(t, "0 9 30 2 *") // Feb 30 — impossible date
	got := c.next(time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local))
	if !got.IsZero() {
		t.Errorf("impossible schedule should return zero time, got %s", got)
	}
}

// ── scheduler ───────────────────────────────────────────────────────────────

func TestSchedulerFiresDueAndRecomputes(t *testing.T) {
	m, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// Every minute — Save recomputes NextRunAt into the future, so force it
	// into the past afterwards to make the first tick fire immediately.
	past := time.Now().Add(-2 * time.Minute)
	saved, err := m.Save(Automation{
		Name: "minutely", Prompt: "do the thing", CronExpr: "* * * * *",
		Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	m.items[0].NextRunAt = &past
	m.persistLocked()
	m.mu.Unlock()
	_ = saved

	fired := make(chan Automation, 4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.StartScheduler(ctx, func(a Automation) {
		fired <- a
		_, _ = m.RecordRun(a.ID, Run{SessionID: "sess-1"})
	})

	select {
	case a := <-fired:
		_ = a
		// The stored automation must have its next run recomputed into the
		// future (the fired snapshot is taken before the recompute).
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			updated, ok := m.Get(a.ID)
			if ok {
				// A "* * * * *" cron only guarantees the next minute
				// boundary — asserting now+30s here flakes when the tick
				// lands in the back half of a minute.
				if updated.NextRunAt == nil || !updated.NextRunAt.After(time.Now()) {
					t.Fatalf("next run not recomputed into the future: %+v", updated.NextRunAt)
				}
				// Wait briefly for the RecordRun from the fire callback.
				for time.Now().Before(deadline) {
					updated, _ = m.Get(a.ID)
					if len(updated.Runs) == 1 {
						return // success
					}
					time.Sleep(20 * time.Millisecond)
				}
				t.Errorf("run history was not recorded after fire")
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatal("automation disappeared from the store")
	case <-time.After(5 * time.Second):
		t.Fatal("scheduler never fired the due automation")
	}
}

func TestSchedulerSkipsPausedAndManual(t *testing.T) {
	m, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-2 * time.Minute)
	if _, err := m.Save(Automation{Name: "paused", Prompt: "p", CronExpr: "* * * * *", Enabled: false, NextRunAt: &past}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Save(Automation{Name: "manual", Prompt: "m"}); err != nil {
		t.Fatal(err)
	}

	fired := make(chan Automation, 4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.StartScheduler(ctx, func(a Automation) { fired <- a })

	// One tick runs immediately on start; wait past two ticks.
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		select {
		case a := <-fired:
			t.Errorf("paused/manual automation fired: %+v", a)
		default:
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestSetEnabledRecomputesNextRun(t *testing.T) {
	m, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	saved, err := m.Save(Automation{Name: "daily", Prompt: "p", CronExpr: "0 9 * * *"})
	if err != nil {
		t.Fatal(err)
	}
	// Created enabled with a next run.
	if saved.Enabled != true || saved.NextRunAt == nil {
		t.Fatalf("expected enabled automation with next run, got %+v", saved)
	}
	// Pause drops it.
	paused, _ := m.SetEnabled(saved.ID, false)
	if paused.NextRunAt != nil {
		t.Errorf("paused automation should have no next run")
	}
	// Resume recomputes it.
	resumed, _ := m.SetEnabled(saved.ID, true)
	if resumed.NextRunAt == nil || resumed.NextRunAt.Before(time.Now()) {
		t.Errorf("resumed automation should have a future next run")
	}
}
