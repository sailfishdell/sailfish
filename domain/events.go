package domain

import (
	eh "github.com/superchalupa/eventhorizon"
)

const (
	// InviteCreatedEvent is when an invite is created.
	RedfishResourceCreatedEvent         eh.EventType = "RedfishResourceCreated"
	RedfishResourcePropertyAddedEvent   eh.EventType = "RedfishResourcePropertyAdded"
	RedfishResourcePropertyUpdatedEvent eh.EventType = "RedfishResourcePropertyUpdated"
	RedfishResourcePropertyRemovedEvent eh.EventType = "RedfishResourcePropertyRemoved"
	RedfishResourceRemovedEvent         eh.EventType = "RedfishResourceRemoved"

	// methods, privileges(from roles), and permissions(read/write). For now, only support wholesale update of object privileges
	RedfishResourcePrivilegesUpdatedEvent  eh.EventType = "RedfishResourcePrivilegesUpdated"
	RedfishResourcePermissionsUpdatedEvent eh.EventType = "RedfishResourcePermissionsUpdated"

	// granular header updates (Etags, probably)
	RedfishResourceHeaderAddedEvent   eh.EventType = "RedfishResourceHeaderAdded"
	RedfishResourceHeaderUpdatedEvent eh.EventType = "RedfishResourceHeaderUpdated"
	RedfishResourceHeaderRemovedEvent eh.EventType = "RedfishResourceHeaderRemoved"
)

func SetupEvents() {
	// Only the event for creating an invite has custom data.
	eh.RegisterEventData(RedfishResourceCreatedEvent, func() eh.EventData { return &RedfishResourceCreatedData{} })
	eh.RegisterEventData(RedfishResourcePropertyAddedEvent, func() eh.EventData { return &RedfishResourcePropertyAddedData{} })
	eh.RegisterEventData(RedfishResourcePropertyUpdatedEvent, func() eh.EventData { return &RedfishResourcePropertyUpdatedData{} })
	eh.RegisterEventData(RedfishResourcePropertyRemovedEvent, func() eh.EventData { return &RedfishResourcePropertyRemovedData{} })
	// no event data for RedfishResourceRemovedEvent

	eh.RegisterEventData(RedfishResourcePrivilegesUpdatedEvent, func() eh.EventData { return &RedfishResourcePrivilegesUpdatedData{} })
	eh.RegisterEventData(RedfishResourcePermissionsUpdatedEvent, func() eh.EventData { return &RedfishResourcePermissionsUpdatedData{} })

	eh.RegisterEventData(RedfishResourceHeaderAddedEvent, func() eh.EventData { return &RedfishResourceHeaderAddedData{} })
	eh.RegisterEventData(RedfishResourceHeaderUpdatedEvent, func() eh.EventData { return &RedfishResourceHeaderUpdatedData{} })
	eh.RegisterEventData(RedfishResourceHeaderRemovedEvent, func() eh.EventData { return &RedfishResourceHeaderRemovedData{} })
}

type RedfishResourceCreatedData struct {
	ResourceURI string
	Type        string
	Context     string
	Properties  map[string]interface{}
}

type RedfishResourcePropertyAddedData struct {
	PropertyName  string
	PropertyValue interface{}
}

type RedfishResourcePropertyUpdatedData struct {
	PropertyName  string
	PropertyValue interface{}
}

type RedfishResourcePropertyRemovedData struct {
	PropertyName string
}

type RedfishResourcePrivilegesUpdatedData struct {
	Privileges map[string]interface{}
}
type RedfishResourcePermissionsUpdatedData struct {
	Permissions map[string]interface{}
}

type RedfishResourceHeaderAddedData struct {
	HeaderName  string
	HeaderValue string
}
type RedfishResourceHeaderUpdatedData struct {
	HeaderName  string
	HeaderValue string
}
type RedfishResourceHeaderRemovedData struct {
	HeaderName string
}
