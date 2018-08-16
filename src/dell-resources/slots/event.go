package slot

import (
	eh "github.com/looplab/eventhorizon"
)

const (
	SlotEvent eh.EventType = "SlotEvent"
)

func init() {
	eh.RegisterEventData(SlotEvent, func() eh.EventData { return &SlotEventData{} })
}

type SlotEventData struct {
	Id string
}
