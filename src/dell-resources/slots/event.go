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
	Config   string
	Contains string
	Id       string
	Name     string
	Occupied string
	SlotName string
}
