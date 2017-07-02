package domain

import (
	eh "github.com/superchalupa/eventhorizon"
)

const (
	// InviteCreatedEvent is when an invite is created.
	OdataCreatedEvent         eh.EventType = "OdataCreated"
	OdataPropertyAddedEvent   eh.EventType = "OdataPropertyAdded"
	OdataPropertyUpdatedEvent eh.EventType = "OdataPropertyUpdated"
	OdataPropertyRemovedEvent eh.EventType = "OdataPropertyRemoved"
	OdataRemovedEvent         eh.EventType = "OdataRemoved"
)

func init() {
	// Only the event for creating an invite has custom data.
	eh.RegisterEventData(OdataCreatedEvent, func() eh.EventData { return &OdataCreatedData{} })
	eh.RegisterEventData(OdataPropertyAddedEvent, func() eh.EventData { return &OdataPropertyAddedData{} })
	eh.RegisterEventData(OdataPropertyUpdatedEvent, func() eh.EventData { return &OdataPropertyUpdatedData{} })
	eh.RegisterEventData(OdataPropertyRemovedEvent, func() eh.EventData { return &OdataPropertyRemovedData{} })
	// no event data for OdataRemovedEvent
}

type OdataCreatedData struct {
	OdataURI   string
	UUID       eh.UUID
	Properties map[string]interface{}
}

type OdataPropertyAddedData struct {
	PropertyName  string
	PropertyValue interface{}
}

type OdataPropertyUpdatedData struct {
	PropertyName  string
	PropertyValue interface{}
}

type OdataPropertyRemovedData struct {
	PropertyName string
}
