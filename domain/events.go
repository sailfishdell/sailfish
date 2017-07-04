package domain

import (
	eh "github.com/superchalupa/eventhorizon"
)

const (
	// InviteCreatedEvent is when an invite is created.
	OdataResourceCreatedEvent         eh.EventType = "OdataResourceCreated"
	OdataResourcePropertyAddedEvent   eh.EventType = "OdataResourcePropertyAdded"
	OdataResourcePropertyUpdatedEvent eh.EventType = "OdataResourcePropertyUpdated"
	OdataResourcePropertyRemovedEvent eh.EventType = "OdataResourcePropertyRemoved"
	OdataResourceRemovedEvent         eh.EventType = "OdataResourceRemoved"
)

func init() {
	// Only the event for creating an invite has custom data.
	eh.RegisterEventData(OdataResourceCreatedEvent, func() eh.EventData { return &OdataResourceCreatedData{} })
	eh.RegisterEventData(OdataResourcePropertyAddedEvent, func() eh.EventData { return &OdataResourcePropertyAddedData{} })
	eh.RegisterEventData(OdataResourcePropertyUpdatedEvent, func() eh.EventData { return &OdataResourcePropertyUpdatedData{} })
	eh.RegisterEventData(OdataResourcePropertyRemovedEvent, func() eh.EventData { return &OdataResourcePropertyRemovedData{} })
	// no event data for OdataResourceRemovedEvent
}

type OdataResourceCreatedData struct {
	UUID        eh.UUID
	ResourceURI string
	Properties  map[string]interface{}
}

type OdataResourcePropertyAddedData struct {
	PropertyName  string
	PropertyValue interface{}
}

type OdataResourcePropertyUpdatedData struct {
	PropertyName  string
	PropertyValue interface{}
}

type OdataResourcePropertyRemovedData struct {
	PropertyName string
}
