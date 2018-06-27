package dm_event

import (
	eh "github.com/looplab/eventhorizon"
)

const (
	DMEvent = eh.EventType("DataManagerEvent")
)

func disabled_init() {
	eh.RegisterEventData(DMEvent, func() eh.EventData {
		return &DMEventData{}
	})
}

type Generic map[string]interface{}

type DMEventData struct {
	FQDD string
	*Generic
}
