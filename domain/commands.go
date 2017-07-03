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
	eh.RegisterCommand(func() eh.Command { return &CreateOdata{} })
	eh.RegisterCommand(func() eh.Command { return &AddOdataProperty{} })
	eh.RegisterCommand(func() eh.Command { return &UpdateOdataProperty{} })
	eh.RegisterCommand(func() eh.Command { return &RemoveOdataProperty{} })
	eh.RegisterCommand(func() eh.Command { return &RemoveOdata{} })

	// collections
	eh.RegisterCommand(func() eh.Command { return &CreateOdataCollection{} })
	eh.RegisterCommand(func() eh.Command { return &AddOdataCollectionMember{} })
	eh.RegisterCommand(func() eh.Command { return &RemoveOdataCollectionMember{} })
}

const (
	CreateOdataCommand         eh.CommandType = "CreateOdata"
	AddOdataPropertyCommand    eh.CommandType = "AddOdataProperty"
	UpdateOdataPropertyCommand eh.CommandType = "UpdateOdataProperty"
	RemoveOdataPropertyCommand eh.CommandType = "RemoveOdataProperty"
	RemoveOdataCommand         eh.CommandType = "RemoveOdata"

	CreateOdataCollectionCommand       eh.CommandType = "CreateOdataCollection"
	AddOdataCollectionMemberCommand    eh.CommandType = "AddOdataCollectionMember"
	RemoveOdataCollectionMemberCommand eh.CommandType = "RemoveOdataCollectionMember"
)

type CreateOdata struct {
	UUID       eh.UUID
	OdataURI   string
	Properties map[string]interface{}
}

func (c CreateOdata) AggregateID() eh.UUID            { return c.UUID }
func (c CreateOdata) AggregateType() eh.AggregateType { return OdataAggregateType }
func (c CreateOdata) CommandType() eh.CommandType     { return CreateOdataCommand }
func (c CreateOdata) Handle(ctx context.Context, a *OdataAggregate) error {
	np := map[string]interface{}{}
	for k, v := range c.Properties {
		np[k] = v
	}

	a.StoreEvent(OdataCreatedEvent,
		&OdataCreatedData{
			OdataURI:   c.OdataURI,
			Properties: np,
			UUID:       c.UUID,
		},
	)

	return nil
}

type AddOdataProperty struct {
	UUID          eh.UUID
	PropertyName  string
	PropertyValue interface{}
}

func (c AddOdataProperty) AggregateID() eh.UUID            { return c.UUID }
func (c AddOdataProperty) AggregateType() eh.AggregateType { return OdataAggregateType }
func (c AddOdataProperty) CommandType() eh.CommandType     { return AddOdataPropertyCommand }
func (c AddOdataProperty) Handle(ctx context.Context, a *OdataAggregate) error {
	if _, ok := a.Properties[c.PropertyName]; !ok {

		a.StoreEvent(OdataPropertyAddedEvent,
			&OdataPropertyAddedData{
				PropertyName:  c.PropertyName,
				PropertyValue: c.PropertyValue,
			},
		)

		return nil
	}
	// TODO: Exception!
	return errors.New("Property already exists")
}

type UpdateOdataProperty struct {
	UUID          eh.UUID
	PropertyName  string
	PropertyValue interface{}
}

func (c UpdateOdataProperty) AggregateID() eh.UUID            { return c.UUID }
func (c UpdateOdataProperty) AggregateType() eh.AggregateType { return OdataAggregateType }
func (c UpdateOdataProperty) CommandType() eh.CommandType     { return UpdateOdataPropertyCommand }
func (c UpdateOdataProperty) Handle(ctx context.Context, a *OdataAggregate) error {
	if _, ok := a.Properties[c.PropertyName]; ok {

		a.StoreEvent(OdataPropertyUpdatedEvent,
			&OdataPropertyUpdatedData{
				PropertyName:  c.PropertyName,
				PropertyValue: c.PropertyValue,
			},
		)

		return nil
	}
	// TODO: Exception!
	return errors.New("Property doesnt exist")
}

type RemoveOdataProperty struct {
	UUID         eh.UUID
	PropertyName string
}

func (c RemoveOdataProperty) AggregateID() eh.UUID            { return c.UUID }
func (c RemoveOdataProperty) AggregateType() eh.AggregateType { return OdataAggregateType }
func (c RemoveOdataProperty) CommandType() eh.CommandType     { return RemoveOdataPropertyCommand }
func (c RemoveOdataProperty) Handle(ctx context.Context, a *OdataAggregate) error {
	if _, ok := a.Properties[c.PropertyName]; ok {

		a.StoreEvent(OdataPropertyRemovedEvent,
			&OdataPropertyRemovedData{
				PropertyName: c.PropertyName,
			},
		)

		return nil
	}
	// TODO: Exception!
	return errors.New("Property doesnt exist")
}

type RemoveOdata struct {
	UUID eh.UUID
}

func (c RemoveOdata) AggregateID() eh.UUID            { return c.UUID }
func (c RemoveOdata) AggregateType() eh.AggregateType { return OdataAggregateType }
func (c RemoveOdata) CommandType() eh.CommandType     { return RemoveOdataCommand }
func (c RemoveOdata) Handle(ctx context.Context, a *OdataAggregate) error {
	a.StoreEvent(OdataRemovedEvent, nil)
	return nil
}

type CreateOdataCollection struct {
	UUID       eh.UUID
	OdataURI   string
	Properties map[string]interface{}
	Members    map[string]string
}

func (c CreateOdataCollection) AggregateID() eh.UUID            { return c.UUID }
func (c CreateOdataCollection) AggregateType() eh.AggregateType { return OdataAggregateType }
func (c CreateOdataCollection) CommandType() eh.CommandType     { return CreateOdataCollectionCommand }
func (c CreateOdataCollection) Handle(ctx context.Context, a *OdataAggregate) error {
	np := map[string]interface{}{}
	for k, v := range c.Properties {
		np[k] = v
	}

	nm := map[string]string{}
	for k, v := range c.Members {
		nm[k] = v
	}

	a.StoreEvent(OdataCreatedEvent,
		&OdataCreatedData{
            UUID:   c.UUID,
			OdataURI:   c.OdataURI,
			Properties: np,
		},
	)

	a.StoreEvent(OdataPropertyAddedEvent,
		&OdataPropertyAddedData{
			PropertyName:  "Members@odata.count",
			PropertyValue: "0",
		},
	)

	a.StoreEvent(OdataPropertyAddedEvent,
		&OdataPropertyAddedData{
			PropertyName:  "Members",
			PropertyValue: []interface{}{},
		},
	)

	return nil
}

type AddOdataCollectionMember struct {
	UUID       eh.UUID
	MemberName string
	MemberURI  string
}

func (c AddOdataCollectionMember) AggregateID() eh.UUID            { return c.UUID }
func (c AddOdataCollectionMember) AggregateType() eh.AggregateType { return OdataAggregateType }
func (c AddOdataCollectionMember) CommandType() eh.CommandType     { return AddOdataCollectionMemberCommand }
func (c AddOdataCollectionMember) Handle(ctx context.Context, a *OdataAggregate) error {
	return nil
}

type RemoveOdataCollectionMember struct {
	UUID       eh.UUID
	MemberName string
}

func (c RemoveOdataCollectionMember) AggregateID() eh.UUID            { return c.UUID }
func (c RemoveOdataCollectionMember) AggregateType() eh.AggregateType { return OdataAggregateType }
func (c RemoveOdataCollectionMember) CommandType() eh.CommandType {
	return RemoveOdataCollectionMemberCommand
}
func (c RemoveOdataCollectionMember) Handle(ctx context.Context, a *OdataAggregate) error {
	return nil
}
