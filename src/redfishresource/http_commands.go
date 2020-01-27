package domain

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	eh "github.com/looplab/eventhorizon"
)

const (
	// All of these shortened from "http:RedfishResource:HTTP" to "R:HTTP" to save memory in the aggregate since this is the most common type
	DELETECommand = eh.CommandType("R:DELETE")

	PATCHCommand = eh.CommandType("R:PATCH")
)

// Static type checking for commands to prevent runtime errors due to typos
var _ = eh.Command(&DELETE{})

var _ = eh.Command(&PATCH{})

// HTTP DELETE Command
type DELETE struct {
	ID    eh.UUID `json:"id"`
	CmdID eh.UUID `json:"cmdid"`
}

func (c *DELETE) ShouldSave() bool                { return false }
func (c *DELETE) AggregateType() eh.AggregateType { return AggregateType }
func (c *DELETE) AggregateID() eh.UUID            { return c.ID }
func (c *DELETE) CommandType() eh.CommandType     { return DELETECommand }
func (c *DELETE) SetAggID(id eh.UUID)             { c.ID = id }
func (c *DELETE) SetCmdID(id eh.UUID)             { c.CmdID = id }
func (c *DELETE) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	// TODO: "Services may return a representation of the just deleted resource in the response body."
	// - can create a new CMD for GET with an identical CMD ID. Is that cheating?
	// TODO: return http 405 status for undeletable objects. right now we use privileges

	//data.Results, _ = ProcessDELETE(ctx, a.Properties, c.Body)

	// send event to trigger delete
	a.PublishEvent(eh.NewEvent(RedfishResourceRemoved, &RedfishResourceRemovedData{
		ID:          c.ID,
		ResourceURI: a.ResourceURI,
	}, time.Now()))

	// send http response
	a.PublishEvent(eh.NewEvent(HTTPCmdProcessed, &HTTPCmdProcessedData{
		CommandID:  c.CmdID,
		Results:    map[string]interface{}{},
		StatusCode: 200,
		Headers:    map[string]string{},
	}, time.Now()))
	return nil
}

// HTTP PATCH Command
type PATCH struct {
	ID    eh.UUID `json:"id"`
	CmdID eh.UUID `json:"cmdid"`

	Body         map[string]interface{} `eh:"optional"`
	auth         *RedfishAuthorizationProperty
	HTTPEventBus eh.EventBus
}

func (c *PATCH) ShouldSave() bool                { return false }
func (c *PATCH) AggregateType() eh.AggregateType { return AggregateType }
func (c *PATCH) AggregateID() eh.UUID            { return c.ID }
func (c *PATCH) CommandType() eh.CommandType     { return PATCHCommand }
func (c *PATCH) SetAggID(id eh.UUID)             { c.ID = id }
func (c *PATCH) SetCmdID(id eh.UUID)             { c.CmdID = id }
func (c *PATCH) SetUserDetails(a *RedfishAuthorizationProperty) string {
	c.auth = a
	return "checkMaster"
}
func (c *PATCH) ParseHTTPRequest(r *http.Request) error {
	err := json.NewDecoder(r.Body).Decode(&c.Body)
	if len(c.Body) == 0 || err != nil {
		err_body := map[string]interface{}{"Attributes": map[string]interface{}{"ERROR": "BADJSON"}}
		c.Body = err_body
	}
	return nil
}

func (c *PATCH) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	// set up the base response data
	data := &HTTPCmdProcessedData{
		CommandID: c.CmdID,
		Headers:   map[string]string{},
	}
	for k, v := range a.Headers {
		data.Headers[k] = v
	}
	tmpResponse := map[string]interface{}{}
	NewPatch(ctx, tmpResponse, a, &a.Properties, c.auth, c.Body)
	data.Results = Flatten(&a.Properties, false)

	r, ok := data.Results.(map[string]interface{})
	if ok {
		for k, v := range tmpResponse {
			r[k] = v
		}
	}

	data.StatusCode = a.StatusCode

	c.HTTPEventBus.PublishEvent(ctx, eh.NewEvent(HTTPCmdProcessed, data, time.Now()))

	return nil
}
