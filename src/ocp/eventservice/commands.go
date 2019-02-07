package eventservice

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	eh "github.com/looplab/eventhorizon"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

const (
	POSTCommand = eh.CommandType("EventService:POST")
)

type Subscription struct {
	Destination string
	Protocol    string
	EventTypes  []string
	Context     string
}

// HTTP POST Command
type POST struct {
	es   *EventService
	d    *domain.DomainObjects
	auth *domain.RedfishAuthorizationProperty

	ID      eh.UUID           `json:"id"`
	CmdID   eh.UUID           `json:"cmdid"`
	Headers map[string]string `eh:"optional"`
	Sub     Subscription      `eh:"optional"`
}

// Static type checking for commands to prevent runtime errors due to typos
var _ = eh.Command(&POST{})

func (c *POST) AggregateType() eh.AggregateType { return domain.AggregateType }
func (c *POST) AggregateID() eh.UUID            { return c.ID }
func (c *POST) CommandType() eh.CommandType     { return POSTCommand }
func (c *POST) SetAggID(id eh.UUID)             { c.ID = id }
func (c *POST) SetCmdID(id eh.UUID)             { c.CmdID = id }
func (c *POST) SetUserDetails(a *domain.RedfishAuthorizationProperty) string {
	c.auth = a
	return "checkMaster"
}
func (c *POST) ParseHTTPRequest(r *http.Request) error {
	json.NewDecoder(r.Body).Decode(&c.Sub)
	return nil
}
func (c *POST) Handle(ctx context.Context, a *domain.RedfishResourceAggregate) error {
	view := c.es.CreateSubscription(ctx, domain.ContextLogger(ctx, "eventservice"), c.Sub, func() {})

	data := &domain.HTTPCmdProcessedData{
		CommandID:  c.CmdID,
		Results:    map[string]interface{}{"msg": "Error creating subscription"},
		StatusCode: 500,
		Headers:    map[string]string{}}

	agg, err := c.d.AggregateStore.Load(ctx, domain.AggregateType, view.GetUUID())
	if err != nil {
		a.PublishEvent(eh.NewEvent(domain.HTTPCmdProcessed, data, time.Now()))
		return errors.New("Could not load subscription aggregate")
	}
	redfishResource, ok := agg.(*domain.RedfishResourceAggregate)
	if !ok {
		a.PublishEvent(eh.NewEvent(domain.HTTPCmdProcessed, data, time.Now()))
		return errors.New("Wrong aggregate type returned")
	}

	redfishResource.ResultsCacheMu.Lock()
	defer redfishResource.ResultsCacheMu.Unlock()
	domain.NewGet(ctx, &redfishResource.Properties, c.auth)
	data.Results = domain.Flatten(redfishResource.Properties.Value)

	for k, v := range a.Headers {
		data.Headers[k] = v
	}
	data.Headers["Location"] = view.GetURI()
	data.StatusCode = 200
	a.PublishEvent(eh.NewEvent(domain.HTTPCmdProcessed, data, time.Now()))

	return nil
}
