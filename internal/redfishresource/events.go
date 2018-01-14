package domain

import (
	eh "github.com/looplab/eventhorizon"
)

const (
	RedfishResourceCreated              = eh.EventType("RedfishResource:created")
	RedfishResourceRemoved              = eh.EventType("RedfishResource:removed")
	HTTPCmdProcessed       eh.EventType = "HTTPCmdProcessed"
)

func init() {
	eh.RegisterEventData(RedfishResourceCreated, func() eh.EventData {
		return &RedfishResourceCreatedData{}
	})
	eh.RegisterEventData(RedfishResourceRemoved, func() eh.EventData {
		return &RedfishResourceRemovedData{}
	})
	eh.RegisterEventData(HTTPCmdProcessed, func() eh.EventData { return &HTTPCmdProcessedData{} })
}

// RedfishResourceCreatedData is the event data for the RedfishResourceCreated event.
type RedfishResourceCreatedData struct {
	ID          eh.UUID `json:"id"     bson:"id"`
	ResourceURI string
}

// RedfishResourceRemovedData is the event data for the RedfishResourceRemoved event.
type RedfishResourceRemovedData struct {
	ID          eh.UUID `json:"id"     bson:"id"`
	ResourceURI string
}

type HTTPCmdProcessedData struct {
	CommandID  eh.UUID
	Results    map[string]interface{}
	StatusCode int
	Headers    map[string]string
}
