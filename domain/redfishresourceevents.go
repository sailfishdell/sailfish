package domain

import (
	eh "github.com/looplab/eventhorizon"
)

const (
	// InviteCreatedEvent is when an invite is created.
	RedfishResourceCreatedEvent           eh.EventType = "RedfishResourceCreated"
	RedfishResourcePropertiesUpdatedEvent eh.EventType = "RedfishResourcePropertiesUpdated"
	RedfishResourcePropertyRemovedEvent   eh.EventType = "RedfishResourcePropertyRemoved"
	RedfishResourceRemovedEvent           eh.EventType = "RedfishResourceRemoved"

	// methods, privileges(from roles), and permissions(read/write). For now, only support wholesale update of object privileges
	RedfishResourcePrivilegesUpdatedEvent  eh.EventType = "RedfishResourcePrivilegesUpdated"
	RedfishResourcePermissionsUpdatedEvent eh.EventType = "RedfishResourcePermissionsUpdated"

	// granular header updates (Etags, probably)
	RedfishResourceHeadersUpdatedEvent eh.EventType = "RedfishResourceHeadersUpdated"
	RedfishResourceHeaderRemovedEvent  eh.EventType = "RedfishResourceHeaderRemoved"
)

func SetupEvents() {
	// Only the event for creating an invite has custom data.
	eh.RegisterEventData(RedfishResourceCreatedEvent, func() eh.EventData { return &RedfishResourceCreatedData{} })
	eh.RegisterEventData(RedfishResourcePropertiesUpdatedEvent, func() eh.EventData { return &RedfishResourcePropertiesUpdatedData{} })
	eh.RegisterEventData(RedfishResourcePropertyRemovedEvent, func() eh.EventData { return &RedfishResourcePropertyRemovedData{} })
	// no event data for RedfishResourceRemovedEvent

	eh.RegisterEventData(RedfishResourcePrivilegesUpdatedEvent, func() eh.EventData { return &RedfishResourcePrivilegesUpdatedData{} })
	eh.RegisterEventData(RedfishResourcePermissionsUpdatedEvent, func() eh.EventData { return &RedfishResourcePermissionsUpdatedData{} })

	eh.RegisterEventData(RedfishResourceHeadersUpdatedEvent, func() eh.EventData { return &RedfishResourceHeadersUpdatedData{} })
	eh.RegisterEventData(RedfishResourceHeaderRemovedEvent, func() eh.EventData { return &RedfishResourceHeaderRemovedData{} })
}

type RedfishResourceCreatedData struct {
	ResourceURI string
	Type        string
	Context     string
	Properties  map[string]interface{}
	Private     map[string]interface{}
}

type RedfishResourcePropertiesUpdatedData struct {
	Properties map[string]interface{}
	Private    map[string]interface{}
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

type RedfishResourceHeadersUpdatedData struct {
	Headers map[string]string
}
type RedfishResourceHeaderRemovedData struct {
	HeaderName string
}
