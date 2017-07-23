package domain

import (
	"context"
	eh "github.com/looplab/eventhorizon"
)

const (
	// InviteCreatedEvent is when an invite is created.
	RedfishResourceCreatedEvent           eh.EventType = "RedfishResourceCreated"
	RedfishResourcePropertiesUpdatedEvent eh.EventType = "RedfishResourcePropertiesUpdated"
	RedfishResourcePropertyRemovedEvent   eh.EventType = "RedfishResourcePropertyRemoved"
	RedfishResourceRemovedEvent           eh.EventType = "RedfishResourceRemoved"

	// methods, privileges(from roles), and permissions(read/write). For now, only support wholesale update of object privileges
	RedfishResourcePrivilegesUpdatedEvent eh.EventType = "RedfishResourcePrivilegesUpdated"
)

func SetupEvents(DDDFunctions) {
	// Only the event for creating an invite has custom data.
	eh.RegisterEventData(RedfishResourceCreatedEvent, func() eh.EventData { return &RedfishResourceCreatedData{} })
	eh.RegisterEventData(RedfishResourcePropertiesUpdatedEvent, func() eh.EventData { return &RedfishResourcePropertiesUpdatedData{} })
	eh.RegisterEventData(RedfishResourcePropertyRemovedEvent, func() eh.EventData { return &RedfishResourcePropertyRemovedData{} })
	// no event data for RedfishResourceRemovedEvent

	eh.RegisterEventData(RedfishResourcePrivilegesUpdatedEvent, func() eh.EventData { return &RedfishResourcePrivilegesUpdatedData{} })
}

type RedfishResourceCreatedData struct {
	TreeID      eh.UUID
	ResourceURI string
	Type        string
	Context     string
	Properties  map[string]interface{}
	Private     map[string]interface{}
}

func (data RedfishResourceCreatedData) ApplyToAggregate(ctx context.Context, a *RedfishResourceAggregate, event eh.Event) error {
	a.ResourceURI = data.ResourceURI
	a.Properties = map[string]interface{}{}
	for k, v := range data.Properties {
		a.Properties[k] = v
	}
	a.Private = map[string]interface{}{}
	for k, v := range data.Private {
		a.Private[k] = v
	}
	a.PrivilegeMap = map[string]interface{}{}
	return nil
}

type RedfishResourcePropertiesUpdatedData struct {
	Properties map[string]interface{}
	Private    map[string]interface{}
}

func (data RedfishResourcePropertiesUpdatedData) ApplyToAggregate(ctx context.Context, a *RedfishResourceAggregate, event eh.Event) error {
	for k, v := range data.Properties {
		a.Properties[k] = v
	}
	return nil
}

type RedfishResourcePropertyRemovedData struct {
	PropertyName string
}

func (data RedfishResourcePropertyRemovedData) ApplyToAggregate(ctx context.Context, a *RedfishResourceAggregate, event eh.Event) error {
	delete(a.Properties, data.PropertyName)
	return nil
}

type RedfishResourcePrivilegesUpdatedData struct {
	Privileges map[string]interface{}
}

func (data RedfishResourcePrivilegesUpdatedData) ApplyToAggregate(ctx context.Context, a *RedfishResourceAggregate, event eh.Event) error {
	// no op for aggregate (only does anything on read side)
	for k, v := range data.Privileges {
		a.PrivilegeMap[k] = v
	}
	return nil
}
