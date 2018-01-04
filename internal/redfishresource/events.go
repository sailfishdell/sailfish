package domain

import (
	eh "github.com/looplab/eventhorizon"
)

const (
	// Created is the event after a todo list is created.
	RedfishResourceCreated = eh.EventType("RedfishResource:created")
)

func init() {
	eh.RegisterEventData(RedfishResourceCreated, func() eh.EventData {
		return &RedfishResourceCreatedData{}
	})
}

// RedfishResourceCreatedData is the event data for the RedfishResourceCreated event.
type RedfishResourceCreatedData struct {
	ID int `json:"id"     bson:"id"`
}
