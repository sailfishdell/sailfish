package domain

import (
	"context"
	"errors"
	"fmt"
	eh "github.com/superchalupa/eventhorizon"
)

var _ = fmt.Printf

func SetupCommands() {
	// odata
	eh.RegisterCommand(func() eh.Command { return &CreateRedfishResource{} })
	eh.RegisterCommand(func() eh.Command { return &UpdateRedfishResourceProperties{} })
	eh.RegisterCommand(func() eh.Command { return &RemoveRedfishResourceProperty{} })
	eh.RegisterCommand(func() eh.Command { return &RemoveRedfishResource{} })

	// collections
	eh.RegisterCommand(func() eh.Command { return &CreateRedfishResourceCollection{} })
	eh.RegisterCommand(func() eh.Command { return &AddRedfishResourceCollectionMember{} })
	eh.RegisterCommand(func() eh.Command { return &RemoveRedfishResourceCollectionMember{} })
}

const (
	CreateRedfishResourceCommand           eh.CommandType = "CreateRedfishResource"
	UpdateRedfishResourcePropertiesCommand eh.CommandType = "UpdateRedfishResourceProperties"
	RemoveRedfishResourcePropertyCommand   eh.CommandType = "RemoveRedfishResourceProperty"
	RemoveRedfishResourceCommand           eh.CommandType = "RemoveRedfishResource"

	CreateRedfishResourceCollectionCommand       eh.CommandType = "CreateRedfishResourceCollection"
	AddRedfishResourceCollectionMemberCommand    eh.CommandType = "AddRedfishResourceCollectionMember"
	RemoveRedfishResourceCollectionMemberCommand eh.CommandType = "RemoveRedfishResourceCollectionMember"

	// TODO
	UpdateRedfishResourcePrivilegesCommand  eh.CommandType = "UpdateRedfishResourcePrivileges"
	UpdateRedfishResourcePermissionsCommand eh.CommandType = "UpdateRedfishResourcePermissions"
	AddRedfishResourceHeaderCommand         eh.CommandType = "AddRedfishResourceHeader"
	UpdateRedfishResourceHeaderCommand      eh.CommandType = "UpdateRedfishResourceHeader"
	RemoveRedfishResourceHeaderCommand      eh.CommandType = "RemoveRedfishResourceHeader"
)

type CreateRedfishResource struct {
	UUID        eh.UUID
	ResourceURI string
	Type        string
	Context     string
	Properties  map[string]interface{} `eh:"optional"`
	Private     map[string]interface{} `eh:"optional"`
}

func (c CreateRedfishResource) AggregateID() eh.UUID            { return c.UUID }
func (c CreateRedfishResource) AggregateType() eh.AggregateType { return RedfishResourceAggregateType }
func (c CreateRedfishResource) CommandType() eh.CommandType     { return CreateRedfishResourceCommand }
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
			np[k] = v
		}
	}
	np["@odata.id"] = c.ResourceURI
	np["@odata.type"] = c.Type
	np["@odata.context"] = c.Context

	a.StoreEvent(RedfishResourceCreatedEvent,
		&RedfishResourceCreatedData{
			ResourceURI: c.ResourceURI,
			Private:     c.Private,
			Properties:  np,
		},
	)

	return nil
}

type UpdateRedfishResourceProperties struct {
	UUID       eh.UUID
	Properties map[string]interface{} `eh:"optional"`
	Private    map[string]interface{} `eh:"optional"`
}

func (c UpdateRedfishResourceProperties) AggregateID() eh.UUID { return c.UUID }
func (c UpdateRedfishResourceProperties) AggregateType() eh.AggregateType {
	return RedfishResourceAggregateType
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
	UUID         eh.UUID
	PropertyName string
}

func (c RemoveRedfishResourceProperty) AggregateID() eh.UUID { return c.UUID }
func (c RemoveRedfishResourceProperty) AggregateType() eh.AggregateType {
	return RedfishResourceAggregateType
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
	UUID eh.UUID
}

func (c RemoveRedfishResource) AggregateID() eh.UUID            { return c.UUID }
func (c RemoveRedfishResource) AggregateType() eh.AggregateType { return RedfishResourceAggregateType }
func (c RemoveRedfishResource) CommandType() eh.CommandType     { return RemoveRedfishResourceCommand }
func (c RemoveRedfishResource) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	a.StoreEvent(RedfishResourceRemovedEvent, nil)
	return nil
}

type CreateRedfishResourceCollection struct {
	UUID eh.UUID
	CreateRedfishResource
	Members []string
}

func (c CreateRedfishResourceCollection) AggregateID() eh.UUID { return c.UUID }
func (c CreateRedfishResourceCollection) AggregateType() eh.AggregateType {
	return RedfishResourceAggregateType
}
func (c CreateRedfishResourceCollection) CommandType() eh.CommandType {
	return CreateRedfishResourceCollectionCommand
}
func (c CreateRedfishResourceCollection) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	nm := []map[string]interface{}{}
	for _, v := range c.Members {
		nm = append(nm, map[string]interface{}{"@odata.id": v})
	}

	c.Properties["Members@odata.count"] = len(c.Members)
	c.Properties["Members"] = nm
	c.CreateRedfishResource.Handle(ctx, a)

	return nil
}

type AddRedfishResourceCollectionMember struct {
	UUID      eh.UUID
	MemberURI string
}

func (c AddRedfishResourceCollectionMember) AggregateID() eh.UUID { return c.UUID }
func (c AddRedfishResourceCollectionMember) AggregateType() eh.AggregateType {
	return RedfishResourceAggregateType
}
func (c AddRedfishResourceCollectionMember) CommandType() eh.CommandType {
	return AddRedfishResourceCollectionMemberCommand
}
func (c AddRedfishResourceCollectionMember) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	nm, ok := a.Properties["Members"]
	if !ok {
		return errors.New("Not a collection")
	}

	members := nm.([]map[string]interface{})

	a.StoreEvent(RedfishResourcePropertiesUpdatedEvent,
		&RedfishResourcePropertiesUpdatedData{
			Properties: map[string]interface{}{
				"Members":             append(members, map[string]interface{}{"@odata.id": c.MemberURI}),
				"Members@odata.count": len(members) + 1,
			},
		},
	)

	return nil
}

type RemoveRedfishResourceCollectionMember struct {
	UUID      eh.UUID
	MemberURI string
}

func (c RemoveRedfishResourceCollectionMember) AggregateID() eh.UUID { return c.UUID }
func (c RemoveRedfishResourceCollectionMember) AggregateType() eh.AggregateType {
	return RedfishResourceAggregateType
}
func (c RemoveRedfishResourceCollectionMember) CommandType() eh.CommandType {
	return RemoveRedfishResourceCollectionMemberCommand
}
func (c RemoveRedfishResourceCollectionMember) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	// TODO
	return nil
}

type UpdateRedfishResourcePrivileges struct {
	UUID       eh.UUID
	Privileges map[string]interface{}
}

func (c UpdateRedfishResourcePrivileges) AggregateID() eh.UUID { return c.UUID }
func (c UpdateRedfishResourcePrivileges) AggregateType() eh.AggregateType {
	return RedfishResourceAggregateType
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
