package domain

import (
	"context"
	"errors"
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
	outChan      chan<- CompletionEvent
}

func (c *GET) AggregateType() eh.AggregateType { return AggregateType }
func (c *GET) AggregateID() eh.UUID            { return c.ID }
func (c *GET) CommandType() eh.CommandType     { return GETCommand }
func (c *GET) SetAggID(id eh.UUID)             { c.ID = id }
func (c *GET) SetCmdID(id eh.UUID)             { c.CmdID = id }

func (c *GET) UseEventChan(out chan<- CompletionEvent) {
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
		Headers:    map[string]string{},
	}
	// TODO: Should be able to discern supported methods from the meta and return those

	for k, v := range a.Headers {
		data.Headers[k] = v
	}

	var complete func()
	complete = func() { a.ResultsCacheMu.RUnlock() }

	// provide a construct to break out of
	for {
		// assume cache HIT
		a.ResultsCacheMu.RLock()
		if a.ResultsCache != nil {
			// TODO: we can compare auth/query and return flattend results
			// TODO: if auth/query doesn't match, we could use results cache to speed up return by re-flattening
			// TODO: if results cache already pre-queried, re-run query

			fmt.Printf(".")
			data.Results = a.ResultsCache
			if c.outChan != nil {
				c.outChan <- CompletionEvent{event: eh.NewEvent(HTTPCmdProcessed, data, time.Now()), complete: complete}
			}
			return nil
		}
		a.ResultsCacheMu.RUnlock()

		// fill in data for cache miss, and then go to the top of the loop
		a.ResultsCacheMu.Lock()
		if a.ResultsCache != nil {
			// redo the comparo from above because we may be here because
			// 1) some other thread already updated results cache for us
			// 2) we couldn't re-use the results cache because something doesnt match up
		}

		// if we got here, we need to refresh the data
		if a.ResultsCache == nil {
			fmt.Printf("X")
			NewGet(ctx, &a.Properties, c.auth)

			// TODO: flatten results
			a.ResultsCache = Flatten(a.Properties.Value)
			a.ResultsCacheAuth = c.auth

			// simplest possible solution for now
			go func(cacheTime int) {
				if cacheTime == 0 {
					cacheTime = DefaultCacheTime
				}
				select {
				case <-time.After(time.Duration(cacheTime) * time.Second):
					fmt.Printf("E")
					a.ResultsCacheMu.Lock()

					// TODO: release ephemerals too

					a.ResultsCache = nil
					a.ResultsCacheMu.Unlock()
				}
			}(a.CacheTimeSec)
		}
		a.ResultsCacheMu.Unlock()
	}

	return errors.New("Can't happen why did we get here?")
}
