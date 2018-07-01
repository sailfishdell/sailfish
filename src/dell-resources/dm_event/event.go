package dm_event

import (
	eh "github.com/looplab/eventhorizon"
)

const (
	HealthEvent = eh.EventType("HealthEvent")
)

func init() {
	eh.RegisterEventData(HealthEvent, func() eh.EventData { return &HealthEventData{} })
}

type HealthEventData struct {
    FQDD string
    Health string
}
