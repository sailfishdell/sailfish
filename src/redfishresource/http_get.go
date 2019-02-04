package domain

import (
	"context"
	"fmt"
	"time"

	eh "github.com/looplab/eventhorizon"
)

const (
	GETCommand       = eh.CommandType("http:RedfishResource:GET")
	DefaultCacheTime = 20
)

// Static type checking for commands to prevent runtime errors due to typos
var _ = eh.Command(&GET{})

// HTTP GET Command
type GET struct {
	ID           eh.UUID `json:"id"`
	CmdID        eh.UUID `json:"cmdid"`
	HTTPEventBus eh.EventBus
	auth         *RedfishAuthorizationProperty
	outChan      chan<- eh.Event
}

func (c *GET) AggregateType() eh.AggregateType { return AggregateType }
func (c *GET) AggregateID() eh.UUID            { return c.ID }
func (c *GET) CommandType() eh.CommandType     { return GETCommand }
func (c *GET) SetAggID(id eh.UUID)             { c.ID = id }
func (c *GET) SetCmdID(id eh.UUID)             { c.CmdID = id }

func (c *GET) UseEventChan(out chan<- eh.Event) {
	c.outChan = out
}

func (c *GET) SetUserDetails(a *RedfishAuthorizationProperty) string {
	c.auth = a
	return "checkMaster"
}
func (c *GET) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	// set up the base response data
	data := &HTTPCmdProcessedData{
		CommandID:  c.CmdID,
		StatusCode: 200,
	}
	// TODO: Should be able to discern supported methods from the meta and return those
	// TODO: set error status code based on err from ProcessGET
	data.Headers = a.Headers

	a.ResultsCacheMu.RLock()
	if a.ResultsCache != nil {
		fmt.Printf(".")
		data.Results = a.ResultsCache
		a.ResultsCacheMu.RUnlock()
		if c.outChan != nil {
			c.outChan <- eh.NewEvent(HTTPCmdProcessed, data, time.Now())
		} else {
			c.HTTPEventBus.PublishEvent(ctx, eh.NewEvent(HTTPCmdProcessed, data, time.Now()))
		}
		return nil
	}
	a.ResultsCacheMu.RUnlock()

	a.ResultsCacheMu.Lock()
	if a.ResultsCache == nil {
		fmt.Printf("X")
		data.Results, _ = ProcessGET(ctx, &a.Properties, c.auth)
		a.ResultsCache = data.Results

		// TODO: cache until an invalidation message comes in
		// TODO: release cache if not used for a while
		// TODO: heartbeat cache to indicate use

		// simplest possible solution for now
		go func(cacheTime int) {
			if cacheTime == 0 {
				cacheTime = DefaultCacheTime
			}
			select {
			case <-time.After(time.Duration(cacheTime) * time.Second):
				fmt.Printf("E")
				a.ResultsCacheMu.Lock()
				a.ResultsCache = nil
				a.ResultsCacheMu.Unlock()
			}
		}(a.CacheTimeSec)
	} else {
		fmt.Printf("O")
		data.Results = a.ResultsCache
	}
	a.ResultsCacheMu.Unlock()

	if c.outChan != nil {
		c.outChan <- eh.NewEvent(HTTPCmdProcessed, data, time.Now())
	} else {
		c.HTTPEventBus.PublishEvent(ctx, eh.NewEvent(HTTPCmdProcessed, data, time.Now()))
	}
	return nil
}
