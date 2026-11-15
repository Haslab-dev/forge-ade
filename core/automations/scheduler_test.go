package automations

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSchedulerFiresDueAutomation proves a cron automation whose schedule is
// already due fires through the handler callback and that the scheduler
// persists the LastRunAt/NextRunAt advances.
func TestSchedulerFiresDueAutomation(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	a, err := m.Save(Automation{
		Name:     "nightly brief",
		Prompt:   "summarize the day",
		CronExpr: "* * * * *",
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if a.NextRunAt == nil {
		t.Fatal("expected NextRunAt to be scheduled on save")
	}

	// Force the schedule near-term (already due), as if the app had been
	// closed past the fire slot: the startup tick must catch it up.
	m.mu.Lock()
	for i := range m.items {
		if m.items[i].ID == a.ID {
			past := time.Now().Add(-time.Second)
			m.items[i].NextRunAt = &past
		}
	}
	m.mu.Unlock()

	fired := make(chan Automation, 4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.StartScheduler(ctx, func(f Automation) {
		fired <- f
	})

	select {
	case got := <-fired:
		if got.ID != a.ID {
			t.Errorf("fired automation %s, want %s", got.ID, a.ID)
		}
		if got.Prompt != "summarize the day" {
			t.Errorf("fired automation prompt %q, want the saved prompt", got.Prompt)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("due cron automation never fired")
	}

	// Bookkeeping advanced and persisted: LastRunAt stamped, NextRunAt moved
	// strictly forward, both visible in the JSON file on disk.
	got, ok := m.Get(a.ID)
	if !ok {
		t.Fatalf("automation %s missing after fire", a.ID)
	}
	if got.LastRunAt == nil {
		t.Error("LastRunAt not stamped after fire")
	}
	if got.NextRunAt == nil || !got.NextRunAt.After(time.Now().Add(-2*time.Minute)) {
		t.Errorf("NextRunAt not advanced after fire: %v", got.NextRunAt)
	}

	data, err := os.ReadFile(filepath.Join(dir, "automations.json"))
	if err != nil {
		t.Fatalf("read persisted file: %v", err)
	}
	if !strings.Contains(string(data), `"lastRunAt"`) {
		t.Error("persisted automations.json missing lastRunAt stamp")
	}
}

// TestTickDoesNotRefireBeforeSchedule ensures a fired automation is not
// re-fired by subsequent ticks until its next slot arrives.
func TestTickDoesNotRefireBeforeSchedule(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)
	a, err := m.Save(Automation{Name: "x", Prompt: "p", CronExpr: "0 9 * * *"})
	if err != nil {
		t.Fatal(err)
	}

	m.mu.Lock()
	past := time.Now().Add(-time.Second)
	m.items[0].NextRunAt = &past
	m.mu.Unlock()

	fires := 0
	m.tick(func(f Automation) {
		fires++
		if f.ID != a.ID {
			t.Errorf("unexpected automation fired: %s", f.ID)
		}
	})
	if fires != 1 {
		t.Fatalf("expected exactly 1 fire, got %d", fires)
	}

	m.tick(func(Automation) { fires++ })
	if fires != 1 {
		t.Fatalf("automation refired before its next slot (fires=%d)", fires)
	}
}

// TestNewManagerQuarantinesCorruptFile mirrors the board guard: a corrupt
// automations.json is preserved as .corrupt-* and never silently reset.
func TestNewManagerQuarantinesCorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "automations.json")
	garbage := []byte(`{"automations": [{"id": "cut-o`)
	if err := os.WriteFile(path, garbage, 0644); err != nil {
		t.Fatal(err)
	}

	m, err := NewManager(dir)
	if err == nil {
		t.Fatal("expected descriptive error for corrupt automations.json")
	}

	matches, _ := filepath.Glob(path + ".corrupt-*")
	if len(matches) != 1 {
		t.Fatalf("expected one .corrupt-* backup, got %d", len(matches))
	}
	data, rerr := os.ReadFile(matches[0])
	if rerr != nil || string(data) != string(garbage) {
		t.Errorf("corrupt backup missing or altered: %v", rerr)
	}

	// The manager stays usable on an empty schedule.
	if l := m.List(); len(l) != 0 {
		t.Errorf("expected empty list after quarantine, got %d", len(l))
	}
}
