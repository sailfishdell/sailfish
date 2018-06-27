package eventservice

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/go-redfish/src/ocp/model"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
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
	model *model.Model
	ch    eh.CommandHandler
	eb    eh.EventBus

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
func (c *POST) ParseHTTPRequest(r *http.Request) error {
	json.NewDecoder(r.Body).Decode(&c.Sub)
	return nil
}
func (c *POST) Handle(ctx context.Context, a *domain.RedfishResourceAggregate) error {
	view := CreateSubscription(ctx, domain.ContextLogger(ctx, "eventservice"), c.Sub, func() {}, c.ch, c.eb)

	a.PublishEvent(eh.NewEvent(domain.HTTPCmdProcessed, domain.HTTPCmdProcessedData{
		CommandID:  c.CmdID,
		Results:    map[string]interface{}{"msg": "subscription created successfully"},
		StatusCode: 200,
		Headers: map[string]string{
			"Location": view.GetURI(),
		},
	}, time.Now()))
	return nil
}
