package engine

import (
	"strings"
	"testing"
	"time"

	"github.com/Haslab-dev/RakitKode/events"
	"github.com/hasdev/forge-ade/core/automations"
)

func TestValidateBranchName(t *testing.T) {
	valid := []string{"main", "feature/x", "release-1.2", " hotfix ", "v2|pipe"}
	for _, b := range valid {
		if err := validateBranchName(b); err != nil {
			t.Errorf("validateBranchName(%q) unexpected error: %v", b, err)
		}
	}
	invalid := []string{"", "   ", "-C", "--force", "-oProxyCommand=x"}
	for _, b := range invalid {
		if err := validateBranchName(b); err == nil {
			t.Errorf("validateBranchName(%q) must reject", b)
		}
	}
}

func TestGitCheckoutRejectsDashBranch(t *testing.T) {
	e := &Engine{}
	if err := e.GitCheckout(t.TempDir(), "--exec=evil"); err == nil {
		t.Fatal("GitCheckout must reject flag-like branch names")
	}
	if err := e.GitCreateBranch(t.TempDir(), "-q"); err == nil {
		t.Fatal("GitCreateBranch must reject flag-like branch names")
	}
}

// TestAutomationFireDispatch verifies the scheduler's fire delivery: the
// registered handler receives the automation payload and the bus publishes
// the automations:fire event.
func TestAutomationFireDispatch(t *testing.T) {
	bus := events.NewBus()
	e := &Engine{Bus: bus}

	got := make(chan automations.Automation, 1)
	e.SetAutomationFireHandler(func(a automations.Automation) { got <- a })

	evCh := make(chan events.Event, 1)
	bus.Subscribe(automationFireEvent, func(ev events.Event) { evCh <- ev })

	e.dispatchAutomationFire(automations.Automation{
		ID:        "a-1",
		Name:      "nightly",
		Prompt:    "run checks",
		Workspace: "/ws",
		CronExpr:  "0 9 * * *",
	})

	select {
	case a := <-got:
		if a.ID != "a-1" || a.Prompt != "run checks" {
			t.Fatalf("handler got %+v, want the fired automation payload", a)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("fire handler never invoked")
	}

	select {
	case ev := <-evCh:
		if ev.Type != automationFireEvent {
			t.Fatalf("bus event type %q, want %q", ev.Type, automationFireEvent)
		}
		if ev.Data["id"] != "a-1" || ev.Data["prompt"] != "run checks" {
			t.Fatalf("bus event data %+v missing payload", ev.Data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("automations:fire event never published")
	}
}

// TestSetAutomationFireHandlerNilClears pins the nil-clears contract.
func TestSetAutomationFireHandlerNilClears(t *testing.T) {
	e := &Engine{}
	e.SetAutomationFireHandler(func(automations.Automation) {})
	e.SetAutomationFireHandler(nil)

	e.autoMu.Lock()
	fn := e.autoHandler
	e.autoMu.Unlock()
	if fn != nil {
		t.Fatal("nil must clear the fire handler")
	}
	if !strings.Contains(string(automationFireEvent), "automations") {
		t.Fatalf("unexpected event name %q", automationFireEvent)
	}
}
