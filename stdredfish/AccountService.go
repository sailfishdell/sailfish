package stdredfish

import (
	"context"
	eh "github.com/superchalupa/eventhorizon"
	"github.com/superchalupa/go-rfs/domain"
    "encoding/json"

	"fmt"
)

var _ = fmt.Println

func init() {
	// COMMAND registration
	eh.RegisterCommand(func() eh.Command { return &HandleAccountServicesPOST{} })
	domain.RegisterDynamicCommand(HandleAccountServicesPOSTCommand)
}

const (
	HandleAccountServicesPOSTCommand eh.CommandType = "POST:#AccountService.v1_0_2.AccountService"
)

type HandleAccountServicesPOST struct {
	domain.HandleHTTP
}

type LoginRequest struct {
    UserName    string
    Password    string
}

func (c HandleAccountServicesPOST) CommandType() eh.CommandType {
	return HandleAccountServicesPOSTCommand
}
func (c HandleAccountServicesPOST) Handle(ctx context.Context, a *domain.RedfishResourceAggregate) error {
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
