package domain

import (
	"context"
	"errors"
	"fmt"
	eh "github.com/looplab/eventhorizon"
	"time"
)

func init() {
	eh.RegisterCommand(func() eh.Command { return &CreateRedfishResource{} })
	eh.RegisterCommand(func() eh.Command { return &RemoveRedfishResource{} })
	eh.RegisterCommand(func() eh.Command { return &CreateRedfishResourceProperties{} })
	eh.RegisterCommand(func() eh.Command { return &UpdateRedfishResourceProperties{} })
	eh.RegisterCommand(func() eh.Command { return &RemoveRedfishResourceProperties{} })
	eh.RegisterCommand(func() eh.Command { return &AddResourceToRedfishResourceCollection{} })
	eh.RegisterCommand(func() eh.Command { return &RemoveResourceFromRedfishResourceCollection{} })
}

const (
	CreateRedfishResourceCommand                       = eh.CommandType("RedfishResource:Create")
	RemoveRedfishResourceCommand                       = eh.CommandType("RedfishResource:Remove")
	CreateRedfishResourcePropertiesCommand             = eh.CommandType("RedfishResourceProperties:Create")
	UpdateRedfishResourcePropertiesCommand             = eh.CommandType("RedfishResourceProperties:Update")
	RemoveRedfishResourcePropertiesCommand             = eh.CommandType("RedfishResourceProperties:Remove")
	AddResourceToRedfishResourceCollectionCommand      = eh.CommandType("RedfishResourceCollection:Add")
	RemoveResourceFromRedfishResourceCollectionCommand = eh.CommandType("RedfishResourceCollection:Remove")
)

// Static type checking for commands to prevent runtime errors due to typos
var _ = eh.Command(&CreateRedfishResource{})
var _ = eh.Command(&RemoveRedfishResource{})
var _ = eh.Command(&CreateRedfishResourceProperties{})
var _ = eh.Command(&UpdateRedfishResourceProperties{})
var _ = eh.Command(&RemoveRedfishResourceProperties{})
var _ = eh.Command(&AddResourceToRedfishResourceCollection{})
var _ = eh.Command(&RemoveResourceFromRedfishResourceCollection{})

// CreateRedfishResource Command
type CreateRedfishResource struct {
	ID          eh.UUID                `json:"id"`
	Plugin      string                 `eh:"optional"`
	ResourceURI string
    Type        string
    Context     string
	Properties  map[string]interface{} `eh:"optional"`
	Private     map[string]interface{} `eh:"optional"`
	Collection  bool                   `eh:"optional"`
}

func (c *CreateRedfishResource) AggregateType() eh.AggregateType { return AggregateType }
func (c *CreateRedfishResource) AggregateID() eh.UUID            { return c.ID }
func (c *CreateRedfishResource) CommandType() eh.CommandType     { return CreateRedfishResourceCommand }

func (c *CreateRedfishResource) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	fmt.Printf("CreateRedfishResource (COMMAND)\n")
	if a.ID != eh.UUID("") {
		fmt.Printf("Aggregate already exists!\n")
		return errors.New("Already created!")
	}
	a.ID = c.ID
	a.ResourceURI = c.ResourceURI
	a.Plugin = c.Plugin
	if a.Plugin == "" {
		a.Plugin = "DEFAULT"
	}
	a.Properties = map[string]interface{}{}
	a.PrivilegeMap = map[string]interface{}{}
	a.Permissions = map[string]interface{}{}
	a.Headers = map[string]string{}
	a.Private = map[string]interface{}{}

	for k, v := range c.Properties {
		a.Properties[k] = v
	}

    a.Properties["@odata.id"] = c.ResourceURI
    a.Properties["@odata.type"] = c.Type
    a.Properties["@odata.context"] = c.Context

	a.eventBus.HandleEvent(ctx, eh.NewEvent(RedfishResourceCreated, &RedfishResourceCreatedData{
		ID:          c.ID,
		ResourceURI: c.ResourceURI,
		Collection:  c.Collection,
	}, time.Now()))
	return nil
}

// RemoveRedfishResource Command
type RemoveRedfishResource struct {
	ID          eh.UUID `json:"id"`
	ResourceURI string
}

func (c *RemoveRedfishResource) AggregateType() eh.AggregateType { return AggregateType }
func (c *RemoveRedfishResource) AggregateID() eh.UUID            { return c.ID }
func (c *RemoveRedfishResource) CommandType() eh.CommandType     { return RemoveRedfishResourceCommand }

func (c *RemoveRedfishResource) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	a.eventBus.HandleEvent(ctx, eh.NewEvent(RedfishResourceRemoved, &RedfishResourceRemovedData{
		ID:          c.ID,
		ResourceURI: c.ResourceURI,
	}, time.Now()))
	return nil
}

type CreateRedfishResourceProperties struct {
	ID         eh.UUID                `json:"id"`
	Properties map[string]interface{} `eh:"optional"`
}

func (c *CreateRedfishResourceProperties) AggregateType() eh.AggregateType { return AggregateType }
func (c *CreateRedfishResourceProperties) AggregateID() eh.UUID            { return c.ID }
func (c *CreateRedfishResourceProperties) CommandType() eh.CommandType {
	return CreateRedfishResourcePropertiesCommand
}
func (c *CreateRedfishResourceProperties) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	return nil
}

type UpdateRedfishResourceProperties struct {
	ID         eh.UUID                `json:"id"`
	Properties map[string]interface{} `eh:"optional"`
}

func (c *UpdateRedfishResourceProperties) AggregateType() eh.AggregateType { return AggregateType }
func (c *UpdateRedfishResourceProperties) AggregateID() eh.UUID            { return c.ID }
func (c *UpdateRedfishResourceProperties) CommandType() eh.CommandType {
	return UpdateRedfishResourcePropertiesCommand
}
func (c *UpdateRedfishResourceProperties) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	for k, v := range c.Properties {
		a.Properties[k] = v
	}
	return nil
}

type RemoveRedfishResourceProperties struct {
	ID         eh.UUID                `json:"id"`
	Properties map[string]interface{} `eh:"optional"`
}

func (c *RemoveRedfishResourceProperties) AggregateType() eh.AggregateType { return AggregateType }
func (c *RemoveRedfishResourceProperties) AggregateID() eh.UUID            { return c.ID }
func (c *RemoveRedfishResourceProperties) CommandType() eh.CommandType {
	return RemoveRedfishResourcePropertiesCommand
}
func (c *RemoveRedfishResourceProperties) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	return nil
}

type AddResourceToRedfishResourceCollection struct {
	ID          eh.UUID `json:"id"`
	ResourceURI string  // resource to add to the collection
}

func (c *AddResourceToRedfishResourceCollection) AggregateType() eh.AggregateType {
	return AggregateType
}
func (c *AddResourceToRedfishResourceCollection) AggregateID() eh.UUID { return c.ID }
func (c *AddResourceToRedfishResourceCollection) CommandType() eh.CommandType {
	return AddResourceToRedfishResourceCollectionCommand
}
func (c *AddResourceToRedfishResourceCollection) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	// TODO: send property updated event
	if collection, ok := a.Properties["Members"]; ok {
		if co, ok := collection.([]map[string]interface{}); ok {
			a.Properties["Members"] = append(co, map[string]interface{}{"@odata.id": c.ResourceURI})
			a.Properties["Members@odata.count"] = len(a.Properties["Members"].([]map[string]interface{}))
		}
	}
	return nil
}

type RemoveResourceFromRedfishResourceCollection struct {
	ID          eh.UUID `json:"id"`
	ResourceURI string
}

func (c *RemoveResourceFromRedfishResourceCollection) AggregateType() eh.AggregateType {
	return AggregateType
}
func (c *RemoveResourceFromRedfishResourceCollection) AggregateID() eh.UUID { return c.ID }
func (c *RemoveResourceFromRedfishResourceCollection) CommandType() eh.CommandType {
	return RemoveResourceFromRedfishResourceCollectionCommand
}
func (c *RemoveResourceFromRedfishResourceCollection) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	return nil
}
