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
	Wait()
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

func WaitSyncEvent(evt eh.Event) {
	if e, ok := evt.(syncEvent); ok {
		e.Wait()
	}
}

func PublishEventAndWaitErr(ctx context.Context, bus eh.EventBus, evt eh.Event) error {
	err := bus.PublishEvent(ctx, evt)
	if err != nil {
		return err
	}
	WaitSyncEvent(evt)
	return nil
}

func PublishEventAndWait(ctx context.Context, bus eh.EventBus, evt eh.Event) {
	_ = PublishEventAndWaitErr(ctx, bus, evt)
}

func PublishAndWaitErr(ctx context.Context, bus eh.EventBus, t eh.EventType, d eh.EventData) error {
	evt := PrepSyncEvent(t, d, time.Now())
	return PublishEventAndWaitErr(ctx, bus, evt)
}

func PublishAndWait(ctx context.Context, bus eh.EventBus, t eh.EventType, d eh.EventData) {
	_ = PublishAndWaitErr(ctx, bus, t, d) // throw away err
}

func PublishErr(ctx context.Context, bus eh.EventBus, t eh.EventType, d eh.EventData) error {
	evt := NewEvent(t, d, time.Now())
	return PublishEventAndWaitErr(ctx, bus, evt) // type assert to sync event will fail, this is ok and wont actually wait
}

func Publish(ctx context.Context, bus eh.EventBus, t eh.EventType, d eh.EventData) {
	_ = PublishErr(ctx, bus, t, d) // throw away err
}
