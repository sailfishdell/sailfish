package domain

import (
	"context"
	eh "github.com/superchalupa/eventhorizon"
	"net/http"
)

const (
	// EVENTS
	HTTPCmdProcessedEvent eh.EventType = "HTTPCmdProcessed"
)

func init() {
	// COMMAND registration
	eh.RegisterCommand(func() eh.Command { return &HandleHTTP{} })

	// EVENT registration
	eh.RegisterEventData(HTTPCmdProcessedEvent, func() eh.EventData { return &HTTPCmdProcessedData{} })
}

const (
	HandleHTTPCommand eh.CommandType = "HandleHTTP"
)

type HTTPCmdProcessedData struct {
	CommandID eh.UUID
	Results   map[string]interface{}
}

type HandleHTTP struct {
	UUID      eh.UUID
	CommandID eh.UUID
	Request   *http.Request `eh:"optional"`
}

func (c HandleHTTP) AggregateID() eh.UUID            { return c.UUID }
func (c HandleHTTP) AggregateType() eh.AggregateType { return RedfishResourceAggregateType }
func (c HandleHTTP) CommandType() eh.CommandType     { return HandleHTTPCommand }
func (c HandleHTTP) Handle(ctx context.Context, a *RedfishResourceAggregate) error {

    // Store HTTPCmdProcessedEvent in order to signal to the command is done
    // processing and to return the results that should be given back to the
    // user.
	a.StoreEvent(HTTPCmdProcessedEvent,
		&HTTPCmdProcessedData{
			CommandID: c.CommandID,
			Results:   map[string]interface{}{"MSG": "HELLO WORLD"},
		},
	)

	return nil
}
