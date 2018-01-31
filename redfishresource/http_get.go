package domain

import (
	"context"
	"fmt"
	"time"

	eh "github.com/looplab/eventhorizon"
)

func init() {
	eh.RegisterCommand(func() eh.Command { return &GET{} })
}

const (
	GETCommand = eh.CommandType("http:RedfishResource:GET")
)

// Static type checking for commands to prevent runtime errors due to typos
var _ = eh.Command(&GET{})

// HTTP GET Command
type GET struct {
	ID    eh.UUID `json:"id"`
	CmdID eh.UUID `json:"cmdid"`
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
	data := HTTPCmdProcessedData{
		CommandID:  c.CmdID,
		Results:    map[string]interface{}{},
		StatusCode: 200,
		Headers:    a.Headers,
	}
	// TODO: mutual exclusion
	// need to think about this: there are threading concerns here if multiple
	// threads call at the same time, same with the a.Properties access below.
	// Probably need to have locking in the aggregate and some access functions
	// to wrap it. Either that, *or* we need to set up a goroutine per
	// aggregate to process data get/set.

	// tell the aggregate to process any metadata that it has. This can include
	// sending commands to update itself.
	fmt.Printf("PROCESS META START: %s\n", a)
	a.ProcessMeta(ctx, "GET")

	fmt.Printf("PROCESS META DONE: %s\n", a)
	for k, v := range a.newProperties {
		fmt.Printf("Set property(%s) == %s\n", k, v)
		data.Results[k] = v
	}
	a.eventBus.HandleEvent(ctx, eh.NewEvent(HTTPCmdProcessed, data, time.Now()))
	return nil
}
