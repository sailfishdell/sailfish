package event

import (
	"sync"
	"time"

	eh "github.com/looplab/eventhorizon"
)

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
