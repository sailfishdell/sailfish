package dell_idrac

import (
	eh "github.com/looplab/eventhorizon"
)

const (
	IDRACComponentEvent eh.EventType = "IDRACComponentEvent"
)

func init() {
	eh.RegisterEventData(IDRACComponentEvent, func() eh.EventData { return &IDRACComponentEventData{} })
}

type IDRACComponentEventData struct {
	Type          string
	FQDD          string
	ParentFQDD    string
	AssociateFQDD string
}
