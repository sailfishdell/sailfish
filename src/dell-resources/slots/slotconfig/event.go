package slotconfig

import (
	eh "github.com/looplab/eventhorizon"
)

const (
	SlotConfigAddEvent eh.EventType = "SlotConfigEvent"
)

func init() {
	eh.RegisterEventData(SlotConfigAddEvent, func() eh.EventData { return &SlotConfigAddEventData{} })
}

type SlotConfigAddEventData struct {
	Id string
}
