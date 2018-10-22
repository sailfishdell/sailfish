package slots

import (
	eh "github.com/looplab/eventhorizon"
)

const (
	ComponentEvent eh.EventType = "ComponentEvent"
)

func init() {
	eh.RegisterEventData(ComponentEvent, func() eh.EventData { return &ComponentEventData{} })
}

type ComponentEventData struct {
	Id   string
	Type string
}
