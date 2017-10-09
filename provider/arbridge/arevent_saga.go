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
	privilegeSaga := saga.NewEventHandler(NewARBridgeSaga(ddd), ddd.GetCommandBus())
	ddd.GetEventBus().AddHandler(privilegeSaga, "AREvent")
}

type ARBridgeSaga struct {
	redfishStartURI string
	domain.DDDFunctions
}

func NewARBridgeSaga(ddd domain.DDDFunctions) *ARBridgeSaga {
	return &ARBridgeSaga{DDDFunctions: ddd}
}

func (s *ARBridgeSaga) SagaType() saga.Type { return ARBridgeSagaType }

func (s *ARBridgeSaga) RunSaga(ctx context.Context, event eh.Event) []eh.Command {
	switch event.EventType() {

	case "AREvent":
		// look up and set "Unauthenticated" for /redfish/ and /redfish/v1/ and session login link.
		// FOR NOW, set Login for all others. This needs to be fleshed out more
		if data, ok := event.Data().(*AREventData); ok {
			fmt.Println("GOT AN EVENT: ", data)

			// walk the redfish tree to see if anything matches this plugin.
			// this is the slowest possible way to do this! But it's quick to
			// implement, so we'll do it for now. Better would be to
			// pre-process the tree so we could have a lookup table.
			tree, err := domain.GetTree(ctx, s.GetReadRepo(), s.GetTreeID())
			if err != nil {
				fmt.Println("ERROR GETTING TREE: ", err)
				return nil
			}

			var commands []eh.Command

			for uri, uuid := range tree.Tree {
				fmt.Printf("Send cmd to URI(%s)  UUID(%s)\n", uri, uuid)
				commands = append(commands,
					&ProcessAREvent{RedfishResourceAggregateBaseCommand: domain.RedfishResourceAggregateBaseCommand{UUID: uuid}, Name: data.Name, Value: data.Value},
				)
			}

			return commands
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
