package arbridge

import (
	"context"
	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/eventhandler/saga"
	"github.com/superchalupa/go-redfish/domain"

	"fmt"
)

const ARBridgeSagaType saga.Type = "ARBridgeSaga"

func SetupARBridgeSaga(ddd domain.DDDFunctions) {
	privilegeSaga := saga.NewEventHandler(NewARBridgeSaga(ddd.GetReadRepo()), ddd.GetCommandBus())
	ddd.GetEventBus().AddHandler(privilegeSaga, "AREvent")
}

type ARBridgeSaga struct {
	repo eh.ReadRepo
	// TODO: fix hardcoded /redfish references here
	redfishStartURI string
}

func NewARBridgeSaga(redfishRepo eh.ReadRepo) *ARBridgeSaga {
	return &ARBridgeSaga{repo: redfishRepo}
}

func (s *ARBridgeSaga) SagaType() saga.Type { return ARBridgeSagaType }

func (s *ARBridgeSaga) RunSaga(ctx context.Context, event eh.Event) []eh.Command {
	switch event.EventType() {

	case "AREvent":
		// look up and set "Unauthenticated" for /redfish/ and /redfish/v1/ and session login link.
		// FOR NOW, set Login for all others. This needs to be fleshed out more
		if data, ok := event.Data().(*AREventData); ok {
			fmt.Println("GOT AN EVENT: ", data)
			return nil
			/*            return []eh.Command{
			              &UpdateRedfishResourcePrivileges{
			                  RedfishResourceAggregateBaseCommand: RedfishResourceAggregateBaseCommand{UUID: event.AggregateID()},
			                  Privileges:                          map[string]interface{}{"GET": []string{"Unauthenticated"}},
			              },
			          } */
		}
	}

	return nil
}
