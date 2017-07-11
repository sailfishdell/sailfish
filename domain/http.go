package domain

import (
	"context"
	"errors"
	eh "github.com/superchalupa/eventhorizon"
	"net/http"

	"fmt"
)

const (
	// EVENTS
	HTTPCmdProcessedEvent eh.EventType = "HTTPCmdProcessed"
)

type HTTPCmdProcessedData struct {
	CommandID eh.UUID
	Results   map[string]interface{}
}

func SetupHTTP() {
	// COMMAND registration
	eh.RegisterCommand(func() eh.Command { return &HandleHTTP{} })

	// EVENT registration
	eh.RegisterEventData(HTTPCmdProcessedEvent, func() eh.EventData { return &HTTPCmdProcessedData{} })
}

// LookupCommand will try to find a command for an http operation
// Search path:
//      ${METHOD}@odata.id
//      ${METHOD}@odata.type
//      ${METHOD}@odata.context
func LookupCommand(repo eh.ReadRepo, treeID, cmdID eh.UUID, resource *RedfishResource, r *http.Request) (eh.Command, error) {
	aggregateID := resource.Properties["@odata.id"].(string)
	typ := resource.Properties["@odata.type"].(string)
	context := resource.Properties["@odata.context"].(string)
	method := r.Method

	search := []string{
		method + ":" + aggregateID,
		method + ":" + typ,
		method + ":" + context,
		"HandleHTTP",
	}

	for _, s := range search {
		fmt.Printf("Looking up command %s\n", s)
		cmd, err := eh.CreateCommand(eh.CommandType(s))
		fmt.Printf("\tcmd = %#v\n", cmd)
		if err == nil {
			cmdInit, ok := cmd.(Initializer)
			fmt.Printf("OK: %s\n", ok)
			if ok {
				cmdInit.Initialize(repo, treeID, eh.UUID(aggregateID), cmdID, r)
				fmt.Printf("\tINIT cmd = %#v\n", cmd)
				return cmd, nil
			}
		}
	}
	return nil, errors.New("Command not found")
}

const (
	HandleHTTPCommand eh.CommandType = "HandleHTTP"
)

type HandleHTTP struct {
	UUID        eh.UUID
	CommandID   eh.UUID
	HTTPRequest *http.Request `eh:"optional"`

	// below is everything needed for command side to query the read side, if necessary.
	// This should be done in only very limited circumstances
	// also keep in mind that read side is only ***eventually*** consistent
	ReadSide eh.ReadRepo
	TreeID   eh.UUID
}

type Initializer interface {
	Initialize(eh.ReadRepo, eh.UUID, eh.UUID, eh.UUID, *http.Request)
}

func (c *HandleHTTP) Initialize(repo eh.ReadRepo, treeID, aggregateID, cmdid eh.UUID, r *http.Request) {
	c.UUID = aggregateID
	c.CommandID = cmdid
	c.HTTPRequest = r
	c.ReadSide = repo
	c.TreeID = treeID
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
