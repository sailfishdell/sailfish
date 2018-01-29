package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
	domain "github.com/superchalupa/go-redfish/redfishresource"
)

func init() {
	domain.RegisterInitFN(InitService)
}

func InitService(ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	eh.RegisterCommand(func() eh.Command { return &POST{eventBus: eb, eventWaiter: ew} })
}

const (
	POSTCommand = eh.CommandType("TestAction:POST")
)

// HTTP POST Command
type POST struct {
	eventBus       eh.EventBus
    eventWaiter   *utils.EventWaiter

	ID      eh.UUID           `json:"id"`
	CmdID   eh.UUID           `json:"cmdid"`
	Headers map[string]string `eh:"optional"`

	// make sure to make everything else optional or this will fail
	TA TestAction `eh:"optional"`
}

type TestAction struct {
	ActionType string
}

// Static type checking for commands to prevent runtime errors due to typos
var _ = eh.Command(&POST{})

func (c *POST) AggregateType() eh.AggregateType { return domain.AggregateType }
func (c *POST) AggregateID() eh.UUID            { return c.ID }
func (c *POST) CommandType() eh.CommandType     { return POSTCommand }
func (c *POST) SetAggID(id eh.UUID)             { c.ID = id }
func (c *POST) SetCmdID(id eh.UUID)             { c.CmdID = id }
func (c *POST) ParseHTTPRequest(r *http.Request) error {
	json.NewDecoder(r.Body).Decode(&c.TA)
	return nil
}
func (c *POST) Handle(ctx context.Context, a *domain.RedfishResourceAggregate) error {
	fmt.Printf("Action handler!!\n")

	c.eventBus.HandleEvent(ctx, eh.NewEvent(domain.HTTPCmdProcessed, domain.HTTPCmdProcessedData{
		CommandID:  c.CmdID,
		Results:    map[string]interface{}{"happy": "joy"},
		StatusCode: 200,
		Headers:    map[string]string{},
	}, time.Now()))
	return nil
}
