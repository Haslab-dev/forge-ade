package events

import "testing"

func TestSubscribeUnsubscribeRemovesHandler(t *testing.T) {
	bus := NewBus()
	calls := 0
	unsub := bus.Subscribe("test:evt", func(e Event) { calls++ })
	unsub()
	bus.Publish(Event{Type: "test:evt"})
	if calls != 0 {
		t.Fatalf("handler ran %d times after unsubscribe; want 0", calls)
	}
}

func TestUnsubscribeOnlyOwnHandler(t *testing.T) {
	bus := NewBus()
	aCount, bCount := 0, 0
	unsubA := bus.Subscribe("test:evt2", func(Event) { aCount++ })
	bus.Subscribe("test:evt2", func(Event) { bCount++ })
	unsubA()
	bus.Publish(Event{Type: "test:evt2"})
	if aCount != 0 || bCount != 1 {
		t.Fatalf("a=%d b=%d; want a=0 b=1", aCount, bCount)
	}
}
