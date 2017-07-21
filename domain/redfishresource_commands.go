package domain

import (
	"context"
	"errors"
	"fmt"
	eh "github.com/looplab/eventhorizon"
)

var _ = fmt.Printf

func SetupCommands() {
	// odata
	eh.RegisterCommand(func() eh.Command { return &CreateRedfishResource{} })
	eh.RegisterCommand(func() eh.Command { return &UpdateRedfishResourceProperties{} })
	eh.RegisterCommand(func() eh.Command { return &RemoveRedfishResourceProperty{} })
	eh.RegisterCommand(func() eh.Command { return &RemoveRedfishResource{} })
}

const (
	CreateRedfishResourceCommand           eh.CommandType = "CreateRedfishResource"
	UpdateRedfishResourcePropertiesCommand eh.CommandType = "UpdateRedfishResourceProperties"
	RemoveRedfishResourcePropertyCommand   eh.CommandType = "RemoveRedfishResourceProperty"
	RemoveRedfishResourceCommand           eh.CommandType = "RemoveRedfishResource"

	// TODO
	UpdateRedfishResourcePrivilegesCommand  eh.CommandType = "UpdateRedfishResourcePrivileges"
	UpdateRedfishResourcePermissionsCommand eh.CommandType = "UpdateRedfishResourcePermissions"
	AddRedfishResourceHeaderCommand         eh.CommandType = "AddRedfishResourceHeader"
	UpdateRedfishResourceHeaderCommand      eh.CommandType = "UpdateRedfishResourceHeader"
	RemoveRedfishResourceHeaderCommand      eh.CommandType = "RemoveRedfishResourceHeader"
)

type RedfishResourceAggregateBaseCommand struct {
	UUID eh.UUID  // the uuid of the actual aggregate
}

func (c RedfishResourceAggregateBaseCommand) AggregateID() eh.UUID { return c.UUID }
func (c RedfishResourceAggregateBaseCommand) AggregateType() eh.AggregateType {
	return RedfishResourceAggregateType
}

type CreateRedfishResource struct {
	RedfishResourceAggregateBaseCommand
    TreeID      eh.UUID  // the uuid of the tree we'll be in
	ResourceURI string
	Type        string
	Context     string
	Properties  map[string]interface{} `eh:"optional"`
	Private     map[string]interface{} `eh:"optional"`
}

func (c CreateRedfishResource) CommandType() eh.CommandType { return CreateRedfishResourceCommand }
func (c CreateRedfishResource) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	disallowed := []string{"@odata.id", "@odata.type", "@odata.context"}
	np := map[string]interface{}{}

AddProp:
	for k, v := range c.Properties {
		// filter out so that the @odata.{id,type,context} cannot be changed
		// after creation
		for _, d := range disallowed {
			if k == d {
				continue AddProp
			}
		}
        np[k] = v
	}
	np["@odata.id"] = c.ResourceURI
	np["@odata.type"] = c.Type
	np["@odata.context"] = c.Context

	a.StoreEvent(RedfishResourceCreatedEvent,
		&RedfishResourceCreatedData{
            TreeID:      c.TreeID,
			ResourceURI: c.ResourceURI,
			Private:     c.Private,
			Properties:  np,
		},
	)

	return nil
}

// UNTESTED
type UpdateRedfishResourceProperties struct {
	RedfishResourceAggregateBaseCommand
	Properties map[string]interface{} `eh:"optional"`
	Private    map[string]interface{} `eh:"optional"`
}

func (c UpdateRedfishResourceProperties) CommandType() eh.CommandType {
	return UpdateRedfishResourcePropertiesCommand
}
func (c UpdateRedfishResourceProperties) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	disallowed := []string{"@odata.id", "@odata.type", "@odata.context"}
	np := map[string]interface{}{}

AddProp:
	for k, v := range c.Properties {
		// filter out so that the @odata.{id,type,context} cannot be changed
		// after creation
		for _, d := range disallowed {
			if k == d {
				continue AddProp
			}
		}
        // dont need to update if it's already that value
        oldv, ok := a.Properties[k]
        if !ok || v!=oldv {
		    np[k] = v
        }
	}

	// shallow copy Private
	npriv := map[string]interface{}{}
	for k, v := range c.Private {
		chk, ok := c.Private[k]
		if ok || chk != v {
			npriv[k] = v
		}
	}

	a.StoreEvent(RedfishResourcePropertiesUpdatedEvent,
		&RedfishResourcePropertiesUpdatedData{
			Properties: np,
			Private:    npriv,
		},
	)

	return nil
}

type RemoveRedfishResourceProperty struct {
	RedfishResourceAggregateBaseCommand
	PropertyName string
}

func (c RemoveRedfishResourceProperty) CommandType() eh.CommandType {
	return RemoveRedfishResourcePropertyCommand
}
func (c RemoveRedfishResourceProperty) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	if _, ok := a.Properties[c.PropertyName]; ok {
		a.StoreEvent(RedfishResourcePropertyRemovedEvent,
			&RedfishResourcePropertyRemovedData{
				PropertyName: c.PropertyName,
			},
		)

		return nil
	}
	return errors.New("Property doesnt exist")
}

type RemoveRedfishResource struct {
	RedfishResourceAggregateBaseCommand
}

func (c RemoveRedfishResource) CommandType() eh.CommandType { return RemoveRedfishResourceCommand }
func (c RemoveRedfishResource) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	a.StoreEvent(RedfishResourceRemovedEvent, nil)
	return nil
}


type UpdateRedfishResourcePrivileges struct {
	RedfishResourceAggregateBaseCommand
	Privileges map[string]interface{}
}

func (c UpdateRedfishResourcePrivileges) CommandType() eh.CommandType {
	return UpdateRedfishResourcePrivilegesCommand
}
func (c UpdateRedfishResourcePrivileges) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	a.StoreEvent(RedfishResourcePrivilegesUpdatedEvent,
		&RedfishResourcePrivilegesUpdatedData{
			Privileges: c.Privileges,
		},
	)

	// TODO
	return nil
}
