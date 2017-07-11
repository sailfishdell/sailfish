package stdredfish

import (
	"context"
	eh "github.com/superchalupa/eventhorizon"
	"github.com/superchalupa/go-rfs/domain"

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

func (c HandleAccountServicesPOST) CommandType() eh.CommandType {
	return HandleAccountServicesPOSTCommand
}
func (c HandleAccountServicesPOST) Handle(ctx context.Context, a *domain.RedfishResourceAggregate) error {
	// Store HTTPCmdProcessedEvent in order to signal to the command is done
	// processing and to return the results that should be given back to the
	// user.

	a.StoreEvent(domain.HTTPCmdProcessedEvent,
		&domain.HTTPCmdProcessedData{
			CommandID: c.CommandID,
			Results:   map[string]interface{}{"MSG": "HELLO WORLD SPECIAL"},
		},
	)
	return nil
}
