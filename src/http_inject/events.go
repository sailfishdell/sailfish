package http_inject

import (
	eh "github.com/looplab/eventhorizon"
)

const (
	WatchdogEvent = eh.EventType("Watchdog")
	DroppedEvent  = eh.EventType("DroppedEvent")
)

func init() {
	eh.RegisterEventData(WatchdogEvent, func() eh.EventData { return &WatchdogEventData{} })
	eh.RegisterEventData(DroppedEvent, func() eh.EventData { return &DroppedEventData{} })
}

type WatchdogEventData struct {
	Seq int
}

type DroppedEventData struct {
	Name     eh.EventType
	EventSeq int64
}
