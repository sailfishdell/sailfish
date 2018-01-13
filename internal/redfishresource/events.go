package domain

import (
	eh "github.com/looplab/eventhorizon"
)

const (
	// Created is the event after a todo list is created.
	RedfishResourceCreated              = eh.EventType("RedfishResource:created")
	HTTPCmdProcessed       eh.EventType = "HTTPCmdProcessed"
)

func init() {
	eh.RegisterEventData(RedfishResourceCreated, func() eh.EventData {
		return &RedfishResourceCreatedData{}
	})
	eh.RegisterEventData(HTTPCmdProcessed, func() eh.EventData { return &HTTPCmdProcessedData{} })
}

// RedfishResourceCreatedData is the event data for the RedfishResourceCreated event.
type RedfishResourceCreatedData struct {
	ID int `json:"id"     bson:"id"`
}

type HTTPCmdProcessedData struct {
	CommandID  eh.UUID
	Results    map[string]interface{}
	StatusCode int
	Headers    map[string]string
}
