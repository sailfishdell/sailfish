package actionhandler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
	domain "github.com/superchalupa/go-redfish/redfishresource"
)

func init() {
	domain.RegisterInitFN(InitService)
}

func InitService(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	eh.RegisterCommand(func() eh.Command { return &POST{eventBus: eb, eventWaiter: ew} })
	eh.RegisterEventData(GenericActionEvent, func() eh.EventData { return &GenericActionEventData{} })
}

const (
	GenericActionEvent = eh.EventType("GenericActionEvent")
	POSTCommand        = eh.CommandType("GenericActionHandler:POST")
)

type GenericActionEventData struct {
	ID          eh.UUID // id of aggregate
	CmdID       eh.UUID
	ResourceURI string

	ActionData interface{}
}

// HTTP POST Command
type POST struct {
	eventBus    eh.EventBus
	eventWaiter *utils.EventWaiter

	ID      eh.UUID           `json:"id"`
	CmdID   eh.UUID           `json:"cmdid"`
	Headers map[string]string `eh:"optional"`

	// make sure to make everything else optional or this will fail
	PostBody interface{} `eh:"optional"`
}

// Static type checking for commands to prevent runtime errors due to typos
var _ = eh.Command(&POST{})

func (c *POST) AggregateType() eh.AggregateType { return domain.AggregateType }
func (c *POST) AggregateID() eh.UUID            { return c.ID }
func (c *POST) CommandType() eh.CommandType     { return POSTCommand }
func (c *POST) SetAggID(id eh.UUID)             { c.ID = id }
func (c *POST) SetCmdID(id eh.UUID)             { c.CmdID = id }
func (c *POST) ParseHTTPRequest(r *http.Request) error {
	json.NewDecoder(r.Body).Decode(&c.PostBody)
	return nil
}
func (c *POST) Handle(ctx context.Context, a *domain.RedfishResourceAggregate) error {
	// Action handler needs to send HTTP response
	c.eventBus.HandleEvent(ctx, eh.NewEvent(GenericActionEvent, GenericActionEventData{
		ID:          c.ID,
		CmdID:       c.CmdID,
		ResourceURI: a.ResourceURI,
		ActionData:  c.PostBody,
	}, time.Now()))
	return nil
}

func MakeListener(uri string) func(eh.Event) bool {
	return func(event eh.Event) bool {
		if event.EventType() != GenericActionEvent {
			return false
		}
		if data, ok := event.Data().(GenericActionEventData); ok {
			if data.ResourceURI == uri {
				return true
			}
		}
		return false
	}
}
