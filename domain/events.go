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

	// methods, privileges(from roles), and permissions(read/write). For now, only support wholesale update of object privileges
	OdataResourcePrivilegesUpdatedEvent  eh.EventType = "OdataResourcePrivilegesUpdated"
	OdataResourcePermissionsUpdatedEvent eh.EventType = "OdataResourcePermissionsUpdated"
	OdataResourceMethodsUpdatedEvent     eh.EventType = "OdataResourceMethodsUpdated"

	// granular header updates (Etags, probably)
	OdataResourceHeaderAddedEvent   eh.EventType = "OdataResourceHeaderAdded"
	OdataResourceHeaderUpdatedEvent eh.EventType = "OdataResourceHeaderUpdated"
	OdataResourceHeaderRemovedEvent eh.EventType = "OdataResourceHeaderRemoved"
)

func init() {
	// Only the event for creating an invite has custom data.
	eh.RegisterEventData(OdataResourceCreatedEvent, func() eh.EventData { return &OdataResourceCreatedData{} })
	eh.RegisterEventData(OdataResourcePropertyAddedEvent, func() eh.EventData { return &OdataResourcePropertyAddedData{} })
	eh.RegisterEventData(OdataResourcePropertyUpdatedEvent, func() eh.EventData { return &OdataResourcePropertyUpdatedData{} })
	eh.RegisterEventData(OdataResourcePropertyRemovedEvent, func() eh.EventData { return &OdataResourcePropertyRemovedData{} })
	// no event data for OdataResourceRemovedEvent

	eh.RegisterEventData(OdataResourcePrivilegesUpdatedEvent, func() eh.EventData { return &OdataResourcePrivilegesUpdatedData{} })
	eh.RegisterEventData(OdataResourcePermissionsUpdatedEvent, func() eh.EventData { return &OdataResourcePermissionsUpdatedData{} })
	eh.RegisterEventData(OdataResourceMethodsUpdatedEvent, func() eh.EventData { return &OdataResourceMethodsUpdatedData{} })

	eh.RegisterEventData(OdataResourceHeaderAddedEvent, func() eh.EventData { return &OdataResourceHeaderAddedData{} })
	eh.RegisterEventData(OdataResourceHeaderUpdatedEvent, func() eh.EventData { return &OdataResourceHeaderUpdatedData{} })
	eh.RegisterEventData(OdataResourceHeaderRemovedEvent, func() eh.EventData { return &OdataResourceHeaderRemovedData{} })
}

type OdataResourceCreatedData struct {
	ResourceURI string
	Type        string
	Context     string
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

type OdataResourcePrivilegesUpdatedData struct {
	Privileges map[string]interface{}
}
type OdataResourcePermissionsUpdatedData struct {
	Permissions map[string]interface{}
}
type OdataResourceMethodsUpdatedData struct {
	Methods map[string]interface{}
}

type OdataResourceHeaderAddedData struct {
	HeaderName  string
	HeaderValue string
}
type OdataResourceHeaderUpdatedData struct {
	HeaderName  string
	HeaderValue string
}
type OdataResourceHeaderRemovedData struct {
	HeaderName string
}
