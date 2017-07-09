package domain

import (
	"context"
	"errors"
	"fmt"
	eh "github.com/superchalupa/eventhorizon"
)

var _ = fmt.Println

func init() {
	// odata
	eh.RegisterCommand(func() eh.Command { return &CreateRedfishResource{} })
	eh.RegisterCommand(func() eh.Command { return &AddRedfishResourceProperty{} })
	eh.RegisterCommand(func() eh.Command { return &UpdateRedfishResourceProperty{} })
	eh.RegisterCommand(func() eh.Command { return &RemoveRedfishResourceProperty{} })
	eh.RegisterCommand(func() eh.Command { return &RemoveRedfishResource{} })

	// collections
	eh.RegisterCommand(func() eh.Command { return &CreateRedfishResourceCollection{} })
	eh.RegisterCommand(func() eh.Command { return &AddRedfishResourceCollectionMember{} })
	eh.RegisterCommand(func() eh.Command { return &RemoveRedfishResourceCollectionMember{} })
}

const (
	CreateRedfishResourceCommand         eh.CommandType = "CreateRedfishResource"
	AddRedfishResourcePropertyCommand    eh.CommandType = "AddRedfishResourceProperty"
	UpdateRedfishResourcePropertyCommand eh.CommandType = "UpdateRedfishResourceProperty"
	RemoveRedfishResourcePropertyCommand eh.CommandType = "RemoveRedfishResourceProperty"
	RemoveRedfishResourceCommand         eh.CommandType = "RemoveRedfishResource"

	CreateRedfishResourceCollectionCommand       eh.CommandType = "CreateRedfishResourceCollection"
	AddRedfishResourceCollectionMemberCommand    eh.CommandType = "AddRedfishResourceCollectionMember"
	RemoveRedfishResourceCollectionMemberCommand eh.CommandType = "RemoveRedfishResourceCollectionMember"

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
	Properties  map[string]interface{}
}

func (c CreateRedfishResource) AggregateID() eh.UUID            { return c.UUID }
func (c CreateRedfishResource) AggregateType() eh.AggregateType { return RedfishResourceAggregateType }
func (c CreateRedfishResource) CommandType() eh.CommandType     { return CreateRedfishResourceCommand }
func (c CreateRedfishResource) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	fmt.Printf("\tStoring EVENTS to create resource\n")
	a.StoreEvent(RedfishResourceCreatedEvent,
		&RedfishResourceCreatedData{
			ResourceURI: c.ResourceURI,
		},
	)

	for k, v := range c.Properties {
		a.StoreEvent(RedfishResourcePropertyAddedEvent,
			&RedfishResourcePropertyAddedData{
				PropertyName:  k,
				PropertyValue: v,
			},
		)
	}

	a.StoreEvent(RedfishResourcePropertyAddedEvent,
		&RedfishResourcePropertyAddedData{
			PropertyName:  "@odata.id",
			PropertyValue: c.ResourceURI,
		},
	)

	a.StoreEvent(RedfishResourcePropertyAddedEvent,
		&RedfishResourcePropertyAddedData{
			PropertyName:  "@odata.type",
			PropertyValue: c.Type,
		},
	)

	a.StoreEvent(RedfishResourcePropertyAddedEvent,
		&RedfishResourcePropertyAddedData{
			PropertyName:  "@odata.context",
			PropertyValue: c.Context,
		},
	)

	return nil
}

type AddRedfishResourceProperty struct {
	UUID          eh.UUID
	PropertyName  string
	PropertyValue interface{}
}

func (c AddRedfishResourceProperty) AggregateID() eh.UUID { return c.UUID }
func (c AddRedfishResourceProperty) AggregateType() eh.AggregateType {
	return RedfishResourceAggregateType
}
func (c AddRedfishResourceProperty) CommandType() eh.CommandType {
	return AddRedfishResourcePropertyCommand
}
func (c AddRedfishResourceProperty) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	if _, ok := a.Properties[c.PropertyName]; !ok {

		a.StoreEvent(RedfishResourcePropertyAddedEvent,
			&RedfishResourcePropertyAddedData{
				PropertyName:  c.PropertyName,
				PropertyValue: c.PropertyValue,
			},
		)

		return nil
	}
	// TODO: Exception!
	return errors.New("Property already exists")
}

type UpdateRedfishResourceProperty struct {
	UUID          eh.UUID
	PropertyName  string
	PropertyValue interface{}
}

func (c UpdateRedfishResourceProperty) AggregateID() eh.UUID { return c.UUID }
func (c UpdateRedfishResourceProperty) AggregateType() eh.AggregateType {
	return RedfishResourceAggregateType
}
func (c UpdateRedfishResourceProperty) CommandType() eh.CommandType {
	return UpdateRedfishResourcePropertyCommand
}
func (c UpdateRedfishResourceProperty) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	if _, ok := a.Properties[c.PropertyName]; ok {

		a.StoreEvent(RedfishResourcePropertyUpdatedEvent,
			&RedfishResourcePropertyUpdatedData{
				PropertyName:  c.PropertyName,
				PropertyValue: c.PropertyValue,
			},
		)

		return nil
	}
	// TODO: Exception!
	return errors.New("Property doesnt exist")
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
	// TODO: Exception!
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
	UUID        eh.UUID
	ResourceURI string
	Type        string
	Context     string
	Properties  map[string]interface{}
	Members     []string
}

func (c CreateRedfishResourceCollection) AggregateID() eh.UUID { return c.UUID }
func (c CreateRedfishResourceCollection) AggregateType() eh.AggregateType {
	return RedfishResourceAggregateType
}
func (c CreateRedfishResourceCollection) CommandType() eh.CommandType {
	return CreateRedfishResourceCollectionCommand
}
func (c CreateRedfishResourceCollection) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	a.StoreEvent(RedfishResourceCreatedEvent,
		&RedfishResourceCreatedData{
			ResourceURI: c.ResourceURI,
			Type:        c.Type,
			Context:     c.Context,
		},
	)

	for k, v := range c.Properties {
		a.StoreEvent(RedfishResourcePropertyAddedEvent,
			&RedfishResourcePropertyAddedData{
				PropertyName:  k,
				PropertyValue: v,
			},
		)
	}

	a.StoreEvent(RedfishResourcePropertyAddedEvent,
		&RedfishResourcePropertyAddedData{
			PropertyName:  "Members@odata.count",
			PropertyValue: len(c.Members),
		},
	)

	nm := []map[string]interface{}{}
	for _, v := range c.Members {
		nm = append(nm, map[string]interface{}{"@odata.id": v})
	}

	a.StoreEvent(RedfishResourcePropertyAddedEvent,
		&RedfishResourcePropertyAddedData{
			PropertyName:  "Members",
			PropertyValue: nm,
		},
	)

	a.StoreEvent(RedfishResourcePropertyAddedEvent,
		&RedfishResourcePropertyAddedData{
			PropertyName:  "@odata.id",
			PropertyValue: c.ResourceURI,
		},
	)

	a.StoreEvent(RedfishResourcePropertyAddedEvent,
		&RedfishResourcePropertyAddedData{
			PropertyName:  "@odata.type",
			PropertyValue: c.Type,
		},
	)

	a.StoreEvent(RedfishResourcePropertyAddedEvent,
		&RedfishResourcePropertyAddedData{
			PropertyName:  "@odata.context",
			PropertyValue: c.Context,
		},
	)

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

	nm := a.Properties["Members"].([]map[string]interface{})

	a.StoreEvent(RedfishResourcePropertyAddedEvent,
		&RedfishResourcePropertyAddedData{
			PropertyName:  "Members",
			PropertyValue: append(nm, map[string]interface{}{"@odata.id": c.MemberURI}),
		},
	)

	a.StoreEvent(RedfishResourcePropertyAddedEvent,
		&RedfishResourcePropertyAddedData{
			PropertyName:  "Members@odata.count",
			PropertyValue: len(nm) + 1,
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
	fmt.Printf("HANDLE UpdateRedfishResourcePrivileges\n")
	a.StoreEvent(RedfishResourcePrivilegesUpdatedEvent,
		&RedfishResourcePrivilegesUpdatedData{
			Privileges: c.Privileges,
		},
	)

	// TODO
	return nil
}
