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
	eh.RegisterCommand(func() eh.Command { return &CreateOdataResource{} })
	eh.RegisterCommand(func() eh.Command { return &AddOdataResourceProperty{} })
	eh.RegisterCommand(func() eh.Command { return &UpdateOdataResourceProperty{} })
	eh.RegisterCommand(func() eh.Command { return &RemoveOdataResourceProperty{} })
	eh.RegisterCommand(func() eh.Command { return &RemoveOdataResource{} })

	// collections
	eh.RegisterCommand(func() eh.Command { return &CreateOdataResourceCollection{} })
	eh.RegisterCommand(func() eh.Command { return &AddOdataResourceCollectionMember{} })
	eh.RegisterCommand(func() eh.Command { return &RemoveOdataResourceCollectionMember{} })
}

const (
	CreateOdataResourceCommand         eh.CommandType = "CreateOdataResource"
	AddOdataResourcePropertyCommand    eh.CommandType = "AddOdataResourceProperty"
	UpdateOdataResourcePropertyCommand eh.CommandType = "UpdateOdataResourceProperty"
	RemoveOdataResourcePropertyCommand eh.CommandType = "RemoveOdataResourceProperty"
	RemoveOdataResourceCommand         eh.CommandType = "RemoveOdataResource"

	CreateOdataResourceCollectionCommand       eh.CommandType = "CreateOdataResourceCollection"
	AddOdataResourceCollectionMemberCommand    eh.CommandType = "AddOdataResourceCollectionMember"
	RemoveOdataResourceCollectionMemberCommand eh.CommandType = "RemoveOdataResourceCollectionMember"
)

type CreateOdataResource struct {
	UUID        eh.UUID
	ResourceURI string
	Type        string
	Context     string
	Properties  map[string]interface{}
}

func (c CreateOdataResource) AggregateID() eh.UUID            { return c.UUID }
func (c CreateOdataResource) AggregateType() eh.AggregateType { return OdataResourceAggregateType }
func (c CreateOdataResource) CommandType() eh.CommandType     { return CreateOdataResourceCommand }
func (c CreateOdataResource) Handle(ctx context.Context, a *OdataResourceAggregate) error {
	a.StoreEvent(OdataResourceCreatedEvent,
		&OdataResourceCreatedData{
			ResourceURI: c.ResourceURI,
			UUID:        c.UUID,
		},
	)

	for k, v := range c.Properties {
		a.StoreEvent(OdataResourcePropertyAddedEvent,
			&OdataResourcePropertyAddedData{
				PropertyName:  k,
				PropertyValue: v,
			},
		)
	}

	a.StoreEvent(OdataResourcePropertyAddedEvent,
		&OdataResourcePropertyAddedData{
			PropertyName:  "@odata.id",
			PropertyValue: c.ResourceURI,
		},
	)

	a.StoreEvent(OdataResourcePropertyAddedEvent,
		&OdataResourcePropertyAddedData{
			PropertyName:  "@odata.type",
			PropertyValue: c.Type,
		},
	)

	a.StoreEvent(OdataResourcePropertyAddedEvent,
		&OdataResourcePropertyAddedData{
			PropertyName:  "@odata.context",
			PropertyValue: c.Context,
		},
	)

	return nil
}

type AddOdataResourceProperty struct {
	UUID          eh.UUID
	PropertyName  string
	PropertyValue interface{}
}

func (c AddOdataResourceProperty) AggregateID() eh.UUID            { return c.UUID }
func (c AddOdataResourceProperty) AggregateType() eh.AggregateType { return OdataResourceAggregateType }
func (c AddOdataResourceProperty) CommandType() eh.CommandType     { return AddOdataResourcePropertyCommand }
func (c AddOdataResourceProperty) Handle(ctx context.Context, a *OdataResourceAggregate) error {
	if _, ok := a.Properties[c.PropertyName]; !ok {

		a.StoreEvent(OdataResourcePropertyAddedEvent,
			&OdataResourcePropertyAddedData{
				PropertyName:  c.PropertyName,
				PropertyValue: c.PropertyValue,
			},
		)

		return nil
	}
	// TODO: Exception!
	return errors.New("Property already exists")
}

type UpdateOdataResourceProperty struct {
	UUID          eh.UUID
	PropertyName  string
	PropertyValue interface{}
}

func (c UpdateOdataResourceProperty) AggregateID() eh.UUID { return c.UUID }
func (c UpdateOdataResourceProperty) AggregateType() eh.AggregateType {
	return OdataResourceAggregateType
}
func (c UpdateOdataResourceProperty) CommandType() eh.CommandType {
	return UpdateOdataResourcePropertyCommand
}
func (c UpdateOdataResourceProperty) Handle(ctx context.Context, a *OdataResourceAggregate) error {
	if _, ok := a.Properties[c.PropertyName]; ok {

		a.StoreEvent(OdataResourcePropertyUpdatedEvent,
			&OdataResourcePropertyUpdatedData{
				PropertyName:  c.PropertyName,
				PropertyValue: c.PropertyValue,
			},
		)

		return nil
	}
	// TODO: Exception!
	return errors.New("Property doesnt exist")
}

type RemoveOdataResourceProperty struct {
	UUID         eh.UUID
	PropertyName string
}

func (c RemoveOdataResourceProperty) AggregateID() eh.UUID { return c.UUID }
func (c RemoveOdataResourceProperty) AggregateType() eh.AggregateType {
	return OdataResourceAggregateType
}
func (c RemoveOdataResourceProperty) CommandType() eh.CommandType {
	return RemoveOdataResourcePropertyCommand
}
func (c RemoveOdataResourceProperty) Handle(ctx context.Context, a *OdataResourceAggregate) error {
	if _, ok := a.Properties[c.PropertyName]; ok {

		a.StoreEvent(OdataResourcePropertyRemovedEvent,
			&OdataResourcePropertyRemovedData{
				PropertyName: c.PropertyName,
			},
		)

		return nil
	}
	// TODO: Exception!
	return errors.New("Property doesnt exist")
}

type RemoveOdataResource struct {
	UUID eh.UUID
}

func (c RemoveOdataResource) AggregateID() eh.UUID            { return c.UUID }
func (c RemoveOdataResource) AggregateType() eh.AggregateType { return OdataResourceAggregateType }
func (c RemoveOdataResource) CommandType() eh.CommandType     { return RemoveOdataResourceCommand }
func (c RemoveOdataResource) Handle(ctx context.Context, a *OdataResourceAggregate) error {
	a.StoreEvent(OdataResourceRemovedEvent, nil)
	return nil
}

type CreateOdataResourceCollection struct {
	UUID        eh.UUID
	ResourceURI string
	Type        string
	Context     string
	Properties  map[string]interface{}
	Members     []string
}

func (c CreateOdataResourceCollection) AggregateID() eh.UUID { return c.UUID }
func (c CreateOdataResourceCollection) AggregateType() eh.AggregateType {
	return OdataResourceAggregateType
}
func (c CreateOdataResourceCollection) CommandType() eh.CommandType {
	return CreateOdataResourceCollectionCommand
}
func (c CreateOdataResourceCollection) Handle(ctx context.Context, a *OdataResourceAggregate) error {
	a.StoreEvent(OdataResourceCreatedEvent,
		&OdataResourceCreatedData{
			UUID:        c.UUID,
			ResourceURI: c.ResourceURI,
			Type:        c.Type,
			Context:     c.Context,
		},
	)

	for k, v := range c.Properties {
		a.StoreEvent(OdataResourcePropertyAddedEvent,
			&OdataResourcePropertyAddedData{
				PropertyName:  k,
				PropertyValue: v,
			},
		)
	}

	a.StoreEvent(OdataResourcePropertyAddedEvent,
		&OdataResourcePropertyAddedData{
			PropertyName:  "Members@odata.count",
			PropertyValue: len(c.Members),
		},
	)

	nm := []map[string]interface{}{}
	for _, v := range c.Members {
		nm = append(nm, map[string]interface{}{"@odata.id": v})
	}

	a.StoreEvent(OdataResourcePropertyAddedEvent,
		&OdataResourcePropertyAddedData{
			PropertyName:  "Members",
			PropertyValue: nm,
		},
	)

	a.StoreEvent(OdataResourcePropertyAddedEvent,
		&OdataResourcePropertyAddedData{
			PropertyName:  "@odata.id",
			PropertyValue: c.ResourceURI,
		},
	)

	a.StoreEvent(OdataResourcePropertyAddedEvent,
		&OdataResourcePropertyAddedData{
			PropertyName:  "@odata.type",
			PropertyValue: c.Type,
		},
	)

	a.StoreEvent(OdataResourcePropertyAddedEvent,
		&OdataResourcePropertyAddedData{
			PropertyName:  "@odata.context",
			PropertyValue: c.Context,
		},
	)

	return nil
}

type AddOdataResourceCollectionMember struct {
	UUID      eh.UUID
	MemberURI string
}

func (c AddOdataResourceCollectionMember) AggregateID() eh.UUID { return c.UUID }
func (c AddOdataResourceCollectionMember) AggregateType() eh.AggregateType {
	return OdataResourceAggregateType
}
func (c AddOdataResourceCollectionMember) CommandType() eh.CommandType {
	return AddOdataResourceCollectionMemberCommand
}
func (c AddOdataResourceCollectionMember) Handle(ctx context.Context, a *OdataResourceAggregate) error {

	nm := a.Properties["Members"].([]map[string]interface{})

	a.StoreEvent(OdataResourcePropertyAddedEvent,
		&OdataResourcePropertyAddedData{
			PropertyName:  "Members",
			PropertyValue: append(nm, map[string]interface{}{"@odata.id": c.MemberURI}),
		},
	)

	a.StoreEvent(OdataResourcePropertyAddedEvent,
		&OdataResourcePropertyAddedData{
			PropertyName:  "Members@odata.count",
			PropertyValue: len(nm) + 1,
		},
	)

	return nil
}

type RemoveOdataResourceCollectionMember struct {
	UUID      eh.UUID
	MemberURI string
}

func (c RemoveOdataResourceCollectionMember) AggregateID() eh.UUID { return c.UUID }
func (c RemoveOdataResourceCollectionMember) AggregateType() eh.AggregateType {
	return OdataResourceAggregateType
}
func (c RemoveOdataResourceCollectionMember) CommandType() eh.CommandType {
	return RemoveOdataResourceCollectionMemberCommand
}
func (c RemoveOdataResourceCollectionMember) Handle(ctx context.Context, a *OdataResourceAggregate) error {
	return nil
}
