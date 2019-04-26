package domain

import (
	"context"
	//"errors"
	//"fmt"
	"time"

	eh "github.com/looplab/eventhorizon"
)

const (
	GETCommand       = eh.CommandType("http:RedfishResource:GET")
	DefaultCacheTime = 5
)

// Static type checking for commands to prevent runtime errors due to typos
var _ = eh.Command(&GET{})

type CompletionEvent struct {
	event    eh.Event
	complete func()
}

// HTTP GET Command
type GET struct {
	ID           eh.UUID `json:"id"`
	CmdID        eh.UUID `json:"cmdid"`
	HTTPEventBus eh.EventBus
	auth         *RedfishAuthorizationProperty
}

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

	// fill in data for cache miss, and then go to the top of the loop
	a.ResultsCacheMu.Lock()
	defer a.ResultsCacheMu.Unlock()

	NewGet(ctx, a, &a.Properties, c.auth)
	data.Results = Flatten(&a.Properties, false)
	data.StatusCode = a.StatusCode
	c.HTTPEventBus.PublishEvent(ctx, eh.NewEvent(HTTPCmdProcessed, data, time.Now()))

	return nil
}
