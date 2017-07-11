package stdredfish

import (
	"context"
	eh "github.com/superchalupa/eventhorizon"
	"github.com/superchalupa/go-rfs/domain"
    "encoding/json"

	"fmt"
)

var _ = fmt.Println

// this needs to be a saga

func SetupSessionService() {
	// COMMAND registration
	eh.RegisterCommand(func() eh.Command { return &HandleSessionServicesPOST{} })
	domain.RegisterDynamicCommand(HandleSessionServicesPOSTCommand)
}

const (
	HandleSessionServicesPOSTCommand eh.CommandType = "POST:#SessionService.v1_0_2.SessionService"
)

type HandleSessionServicesPOST struct {
	domain.HandleHTTP
}

type LoginRequest struct {
    UserName    string
    Password    string
}

func (c HandleSessionServicesPOST) CommandType() eh.CommandType {
	return HandleSessionServicesPOSTCommand
}
func (c HandleSessionServicesPOST) Handle(ctx context.Context, a *domain.RedfishResourceAggregate) error {
    decoder := json.NewDecoder(c.HTTPRequest.Body)
    var lr LoginRequest
    err := decoder.Decode(&lr)

    if err == nil {
        fmt.Printf("HAPPY: user(%s) pass(%s)\n", lr.UserName, lr.Password)
    }

	a.StoreEvent(domain.HTTPCmdProcessedEvent,
		&domain.HTTPCmdProcessedData{
			CommandID: c.CommandID,
			Results:   map[string]interface{}{
                "MSG": "HELLO WORLD SPECIAL",
                "user": lr.UserName,
                "pass": lr.Password,
                },
			Headers:   map[string]string{
                "X-Token-Auth": "HELLO WORLD SPECIAL",
                "Location": "/redfish/v1/SessionService/Sessions/1",
                },
		},
	)
	return nil
}
