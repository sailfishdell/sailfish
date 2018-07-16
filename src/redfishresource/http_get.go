package domain

import (
	"context"
	"time"

	eh "github.com/looplab/eventhorizon"
)

const (
	GETCommand = eh.CommandType("http:RedfishResource:GET")
)

// Static type checking for commands to prevent runtime errors due to typos
var _ = eh.Command(&GET{})

// HTTP GET Command
type GET struct {
	ID    eh.UUID `json:"id"`
	CmdID eh.UUID `json:"cmdid"`
    HTTPEventBus eh.EventBus
}

func (c *GET) AggregateType() eh.AggregateType { return AggregateType }
func (c *GET) AggregateID() eh.UUID            { return c.ID }
func (c *GET) CommandType() eh.CommandType     { return GETCommand }
func (c *GET) SetAggID(id eh.UUID)             { c.ID = id }
func (c *GET) SetCmdID(id eh.UUID)             { c.CmdID = id }
func (c *GET) SetUserDetails(u string, p []string) string {
	return "checkMaster"
}
func (c *GET) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	// set up the base response data
	data := &HTTPCmdProcessedData{
		CommandID:  c.CmdID,
		StatusCode: 200,
	}
	// TODO: Should be able to discern supported methods from the meta and return those

    a.propertiesMu.RLock()
	data.Results, _ = ProcessGET(ctx, a.properties)
    a.propertiesMu.RUnlock()

	// TODO: set error status code based on err from ProcessGET
	data.Headers = a.Headers

	c.HTTPEventBus.PublishEvent(ctx, eh.NewEvent(HTTPCmdProcessed, data, time.Now()))
	return nil
}
