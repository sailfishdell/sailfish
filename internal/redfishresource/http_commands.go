package domain

import (
	eh "github.com/looplab/eventhorizon"
)

func init() {
	eh.RegisterCommand(func() eh.Command { return &GET{} })
	eh.RegisterCommand(func() eh.Command { return &PUT{} })
	eh.RegisterCommand(func() eh.Command { return &PATCH{} })
	eh.RegisterCommand(func() eh.Command { return &POST{} })
	eh.RegisterCommand(func() eh.Command { return &DELETE{} })
	eh.RegisterCommand(func() eh.Command { return &HEAD{} })
	eh.RegisterCommand(func() eh.Command { return &OPTIONS{} })
}

const (
	GETCommand     = eh.CommandType("RedfishResource:GET")
	PUTCommand     = eh.CommandType("RedfishResource:PUT")
	PATCHCommand   = eh.CommandType("RedfishResource:PATCH")
	POSTCommand    = eh.CommandType("RedfishResource:POST")
	DELETECommand  = eh.CommandType("RedfishResource:DELETE")
	HEADCommand    = eh.CommandType("RedfishResource:HEAD")
	OPTIONSCommand = eh.CommandType("RedfishResource:OPTIONS")
)

// Static type checking for commands to prevent runtime errors due to typos
var _ = eh.Command(&GET{})
var _ = eh.Command(&PUT{})
var _ = eh.Command(&PATCH{})
var _ = eh.Command(&POST{})
var _ = eh.Command(&DELETE{})
var _ = eh.Command(&HEAD{})
var _ = eh.Command(&OPTIONS{})

// HTTP GET Command
type GET struct {
	ID eh.UUID `json:"id"`
}

func (c *GET) AggregateType() eh.AggregateType { return AggregateType }
func (c *GET) AggregateID() eh.UUID            { return c.ID }
func (c *GET) CommandType() eh.CommandType     { return GETCommand }

// HTTP PUT Command
type PUT struct {
	ID   eh.UUID `json:"id"`
	Body map[string]interface{}
}

func (c *PUT) AggregateType() eh.AggregateType { return AggregateType }
func (c *PUT) AggregateID() eh.UUID            { return c.ID }
func (c *PUT) CommandType() eh.CommandType     { return PUTCommand }

// HTTP PATCH Command
type PATCH struct {
	ID   eh.UUID `json:"id"`
	Body map[string]interface{}
}

func (c *PATCH) AggregateType() eh.AggregateType { return AggregateType }
func (c *PATCH) AggregateID() eh.UUID            { return c.ID }
func (c *PATCH) CommandType() eh.CommandType     { return PATCHCommand }

// HTTP POST Command
type POST struct {
	ID   eh.UUID `json:"id"`
	Body map[string]interface{}
}

func (c *POST) AggregateType() eh.AggregateType { return AggregateType }
func (c *POST) AggregateID() eh.UUID            { return c.ID }
func (c *POST) CommandType() eh.CommandType     { return POSTCommand }

// HTTP DELETE Command
type DELETE struct {
	ID eh.UUID `json:"id"`
}

func (c *DELETE) AggregateType() eh.AggregateType { return AggregateType }
func (c *DELETE) AggregateID() eh.UUID            { return c.ID }
func (c *DELETE) CommandType() eh.CommandType     { return DELETECommand }

// HTTP HEAD Command
type HEAD struct {
	ID eh.UUID `json:"id"`
}

func (c *HEAD) AggregateType() eh.AggregateType { return AggregateType }
func (c *HEAD) AggregateID() eh.UUID            { return c.ID }
func (c *HEAD) CommandType() eh.CommandType     { return HEADCommand }

// HTTP OPTIONS Command
type OPTIONS struct {
	ID eh.UUID `json:"id"`
}

func (c *OPTIONS) AggregateType() eh.AggregateType { return AggregateType }
func (c *OPTIONS) AggregateID() eh.UUID            { return c.ID }
func (c *OPTIONS) CommandType() eh.CommandType     { return OPTIONSCommand }
