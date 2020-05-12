package httpinject

import (
	eh "github.com/looplab/eventhorizon"
)

const (
	WatchdogEvent = eh.EventType("Watchdog")
	DroppedEvent  = eh.EventType("DroppedEvent")
)

type WatchdogEventData struct {
	Seq int
}

type DroppedEventData struct {
	Name     eh.EventType
	EventSeq int64
}
