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
}

const (
	CreateRedfishResourceCommand           = eh.CommandType("RedfishResource:Create")
	RemoveRedfishResourceCommand           = eh.CommandType("RedfishResource:Remove")
	CreateRedfishResourcePropertiesCommand = eh.CommandType("RedfishResourceProperties:Create")
	UpdateRedfishResourcePropertiesCommand = eh.CommandType("RedfishResourceProperties:Update")
	RemoveRedfishResourcePropertiesCommand = eh.CommandType("RedfishResourceProperties:Remove")
)

// Static type checking for commands to prevent runtime errors due to typos
var _ = eh.Command(&CreateRedfishResource{})
var _ = eh.Command(&RemoveRedfishResource{})
var _ = eh.Command(&CreateRedfishResourceProperties{})
var _ = eh.Command(&UpdateRedfishResourceProperties{})
var _ = eh.Command(&RemoveRedfishResourceProperties{})

// CreateRedfishResource Command
type CreateRedfishResource struct {
	ID          eh.UUID `json:"id"`
	ResourceURI string
	Properties  map[string]interface{} `eh:"optional"`
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
	a.Plugin = "DEFAULT"
	a.Properties = map[string]interface{}{}
	a.PrivilegeMap = map[string]interface{}{}
	a.Permissions = map[string]interface{}{}
	a.Headers = map[string]string{}
	a.Private = map[string]interface{}{}

	for k, v := range c.Properties {
		a.Properties[k] = v
	}

	a.eventBus.HandleEvent(ctx, eh.NewEvent(RedfishResourceCreated, &RedfishResourceCreatedData{
		ID:          c.ID,
		ResourceURI: c.ResourceURI,
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
