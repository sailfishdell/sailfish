package domain

import (
	"context"
	"fmt"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

func SetupSessionService(ctx context.Context, rootID eh.UUID, ew *utils.EventWaiter, ch eh.CommandHandler) {
	fmt.Printf("SetupSessionService\n")
	l, err := ew.Listen(ctx, func(event eh.Event) bool {
		fmt.Printf("SetupSessionService MATCH event called: %s\n", event)
		if event.EventType() != RedfishResourceCreated {
			fmt.Printf("\tNOT IT\n")
			return false
		}
		if data, ok := event.Data().(*RedfishResourceCreatedData); ok {
			if data.ResourceURI == "/redfish/v1/" {
				fmt.Printf("\tGOOOT IT\n")
				return true
			} else {
				fmt.Printf("\tnot IT: %s\n", data.ResourceURI)
			}
		}
		fmt.Printf("\tNOT IT -\n")
		return false
	})
	if err != nil {
		return
	}

	// wait for the root object to be created, then enhance it. Oneshot for now.
	go func() {
		defer l.Close()

		_, err := l.Wait(ctx)
		fmt.Printf("GOT EVENT!\n")
		if err != nil {
			fmt.Printf("Error waiting for event: %s\n", err.Error())
			return
		}

		ch.HandleCommand(ctx,
			&UpdateRedfishResourceProperties{
				ID: rootID,
				Properties: map[string]interface{}{
					"SessionService": map[string]interface{}{"@odata.id": "/redfish/v1/SessionService"},
					"Links":          map[string]interface{}{"Sessions": map[string]interface{}{"@odata.id": "/redfish/v1/SessionService/Sessions"}},
				},
			})
	}()
}
