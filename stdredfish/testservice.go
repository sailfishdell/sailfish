package stdredfish

import (
	"context"
	"encoding/json"
	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/go-redfish/domain"

	"fmt"
)

var _ = fmt.Println

func SetupTestService() {
	// COMMAND registration
	eh.RegisterCommand(func() eh.Command { return &HandleTestServicesPOST{} })
	domain.RegisterDynamicCommand(HandleTestServicesPOSTCommand)
}

const (
	HandleTestServicesPOSTCommand eh.CommandType = "POST:#TestService"
)

type HandleTestServicesPOST struct {
	domain.HandleHTTP
}

type TestRequest struct {
	UserName string
	Password string
}

func (c HandleTestServicesPOST) CommandType() eh.CommandType {
	return HandleTestServicesPOSTCommand
}
func (c HandleTestServicesPOST) Handle(ctx context.Context, a *domain.RedfishResourceAggregate) error {
	decoder := json.NewDecoder(c.HTTPRequest.Body)
	var lr TestRequest
	err := decoder.Decode(&lr)

	if err == nil {
		fmt.Printf("HAPPY: user(%s) pass(%s)\n", lr.UserName, lr.Password)
	}

	a.StoreEvent(domain.HTTPCmdProcessedEvent,
		&domain.HTTPCmdProcessedData{
			CommandID: c.CommandID,
			Results: map[string]interface{}{
				"MSG":  "HELLO WORLD SPECIAL",
				"user": lr.UserName,
				"pass": lr.Password,
			},
			Headers: map[string]string{
				"X-Token-Auth": "HELLO WORLD SPECIAL",
				"Location":     "/redfish/v1/SessionService/Sessions/1",
			},
		},
	)
	return nil
}
