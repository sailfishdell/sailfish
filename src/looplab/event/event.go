package event

import (
	"context"
	"sync"
	"time"

	eh "github.com/looplab/eventhorizon"
)

// drop-in replacement for eh.NewEvent
func NewEvent(t eh.EventType, d eh.EventData, s time.Time) eh.Event {
	return eh.NewEvent(t, d, s)
}

type SyncEvent struct {
	eh.Event
	*sync.WaitGroup
}

func NewSyncEvent(t eh.EventType, d eh.EventData, s time.Time) SyncEvent {
	return SyncEvent{
		Event:     eh.NewEvent(t, d, s),
		WaitGroup: &sync.WaitGroup{},
	}
}

func PrepSyncEvent(t eh.EventType, d eh.EventData, s time.Time) SyncEvent {
	evt := NewSyncEvent(t, d, s)
	evt.Add(1)
	return evt
}

type syncEvent interface {
	Add(int)
	Done()
}

func PinSyncEvent(evt eh.Event) {
	if e, ok := evt.(syncEvent); ok {
		e.Add(1)
	}
}

func ReleaseSyncEvent(evt eh.Event) {
	if e, ok := evt.(syncEvent); ok {
		e.Done()
	}
}

func PublishAndWait(ctx context.Context, bus eh.EventBus, t eh.EventType, d eh.EventData) {
	evt := PrepSyncEvent(t, d, time.Now())
	err := bus.PublishEvent(ctx, evt)
	if err != nil {
		return
	}
	evt.Wait()
}
