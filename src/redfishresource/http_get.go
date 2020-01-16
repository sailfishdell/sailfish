package domain

import (
	"context"
	//"errors"
	//"fmt"
	"time"

	eh "github.com/looplab/eventhorizon"
)

const (
	// All of these shortened from "http:RedfishResource:HTTP" to "R:HTTP" to save memory in the aggregate since this is the most common type
	GETCommand       = eh.CommandType("R:GET")
	DefaultCacheTime = 5
)

// Static type checking for commands to prevent runtime errors due to typos
var _ = eh.Command(&GET{})

// HTTP GET Command
type GET struct {
	ID           eh.UUID `json:"id"`
	CmdID        eh.UUID `json:"cmdid"`
	HTTPEventBus eh.EventBus
	auth         *RedfishAuthorizationProperty
}

func (c *GET) ShouldSave() bool                { return false }
func (c *GET) AggregateType() eh.AggregateType { return AggregateType }
func (c *GET) AggregateID() eh.UUID            { return c.ID }
func (c *GET) CommandType() eh.CommandType     { return GETCommand }
func (c *GET) SetAggID(id eh.UUID)             { c.ID = id }
func (c *GET) SetCmdID(id eh.UUID)             { c.CmdID = id }

func (c *GET) SetUserDetails(a *RedfishAuthorizationProperty) string {
	c.auth = a
	return "checkMaster"
}
func (c *GET) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	// set up the base response data
	data := &HTTPCmdProcessedData{
		CommandID:  c.CmdID,
		StatusCode: 200,
		Headers:    map[string]string{},
	}
	// TODO: Should be able to discern supported methods from the meta and return those

	for k, v := range a.Headers {
		data.Headers[k] = v
	}

	NewGet(ctx, a, &a.Properties, c.auth)
	data.Results = Flatten(&a.Properties, false)
	c.HTTPEventBus.PublishEvent(ctx, eh.NewEvent(HTTPCmdProcessed, data, time.Now()))

	return nil
}
