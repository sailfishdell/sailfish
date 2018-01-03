package domain

import (
	eh "github.com/looplab/eventhorizon"
)

func init() {
	eh.RegisterCommand(func() eh.Command { return &CreateRedfishResource{} })
}

const (
	CreateRedfishResourceCommand = eh.CommandType("RedfishResource:Create")
)

// Static type checking for commands to prevent runtime errors due to typos
var _ = eh.Command(&CreateRedfishResource{})

// CreateRedfishResource Command
type CreateRedfishResource struct {
	ID eh.UUID `json:"id"`
}

func (c *CreateRedfishResource) AggregateType() eh.AggregateType { return AggregateType }
func (c *CreateRedfishResource) AggregateID() eh.UUID            { return c.ID }
func (c *CreateRedfishResource) CommandType() eh.CommandType     { return CreateRedfishResourceCommand }
