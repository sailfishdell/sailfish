package domain

import (
    "context"
    "errors"
	eh "github.com/looplab/eventhorizon"
)

func init() {
	eh.RegisterCommand(func() eh.Command { return &CreateRedfishResource{} })
	eh.RegisterCommand(func() eh.Command { return &RemoveRedfishResource{} })
}

const (
	CreateRedfishResourceCommand = eh.CommandType("RedfishResource:Create")
	RemoveRedfishResourceCommand = eh.CommandType("RedfishResource:Remove")
)

// Static type checking for commands to prevent runtime errors due to typos
var _ = eh.Command(&CreateRedfishResource{})
var _ = eh.Command(&RemoveRedfishResource{})

// CreateRedfishResource Command
type CreateRedfishResource struct {
	ID eh.UUID `json:"id"`
    ResourceURI string
}

func (c *CreateRedfishResource) AggregateType() eh.AggregateType { return AggregateType }
func (c *CreateRedfishResource) AggregateID() eh.UUID            { return c.ID }
func (c *CreateRedfishResource) CommandType() eh.CommandType     { return CreateRedfishResourceCommand }

func (c *CreateRedfishResource) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
    if a.ID != eh.UUID("") {
        return errors.New("Already created!")
    }
    a.ID = c.ID
    a.ResourceURI = c.ResourceURI
    return nil
}

// RemoveRedfishResource Command
type RemoveRedfishResource struct {
	ID eh.UUID `json:"id"`
}

func (c *RemoveRedfishResource) AggregateType() eh.AggregateType { return AggregateType }
func (c *RemoveRedfishResource) AggregateID() eh.UUID            { return c.ID }
func (c *RemoveRedfishResource) CommandType() eh.CommandType     { return RemoveRedfishResourceCommand }

func (c *RemoveRedfishResource) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
    return nil
}
