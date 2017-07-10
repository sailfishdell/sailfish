package domain

import (
    "errors"
	"context"
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
func LookupCommand(resource *RedfishResource, r *http.Request, cmdid eh.UUID) (eh.Command, error) {
    id := resource.Properties["@odata.id"].(string)
    typ := resource.Properties["@odata.type"].(string)
    context := resource.Properties["@odata.context"].(string)
    method := r.Method

    search := []string{
        method + ":" + id,
        method + ":" + typ,
        method + ":" + context,
        "HandleHTTP",
    }

    for _, s := range(search) {
        fmt.Printf("Looking up command %s\n", s)
        cmd, err := eh.CreateCommand(eh.CommandType(s))
        if err != nil {
            fmt.Printf("\terr = %s\n", err.Error())
        } else {
            fmt.Printf("\tERR = NIL\n")
            }
        if cmd != nil {
            fmt.Printf("\tcmd = %#v\n", cmd)
        } else {
            fmt.Printf("\tCMD = NIL\n")
        }
        if err == nil {
            cmdInit, ok := cmd.(Initializer)
            fmt.Printf("OK: %s\n", ok)
            if ok {
                cmdInit.Initialize(eh.UUID(id), cmdid, r)
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
	UUID      eh.UUID
	CommandID eh.UUID
	Request   *http.Request `eh:"optional"`
}

type Initializer interface{
    Initialize(id eh.UUID, cmdid eh.UUID, r *http.Request)
}

func (c *HandleHTTP) Initialize(id eh.UUID, cmdid eh.UUID, r *http.Request) {
    c.UUID = id
    c.CommandID = cmdid
    c.Request = r
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
