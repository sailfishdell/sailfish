package domain

import (
	"context"
	"fmt"
	eh "github.com/superchalupa/eventhorizon"
	"net/http"
)

func init() {
	eh.RegisterCommand(func() eh.Command { return &HandleHTTP{} })
}

const (
	HandleHTTPCommand eh.CommandType = "HandleHTTP"
)

type HandleHTTP struct {
	UUID      eh.UUID
	CommandID eh.UUID
	Request   *http.Request
}

func (c HandleHTTP) AggregateID() eh.UUID            { return c.UUID }
func (c HandleHTTP) AggregateType() eh.AggregateType { return RedfishResourceAggregateType }
func (c HandleHTTP) CommandType() eh.CommandType     { return HandleHTTPCommand }
func (c HandleHTTP) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	fmt.Printf("\tHandling HTTP Command\n")

	/*
		a.StoreEvent(RedfishResourceCreatedEvent,
			&RedfishResourceCreatedData{
				ResourceURI: c.ResourceURI,
			},
		)
	*/

	return nil
}
