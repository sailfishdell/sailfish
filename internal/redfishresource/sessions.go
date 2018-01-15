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
		if event.EventType() != RedfishResourceCreated {
			return false
		}
		if data, ok := event.Data().(*RedfishResourceCreatedData); ok {
			if data.ResourceURI == "/redfish/v1/" {
				return true
			}
		}
		return false
	})
	if err != nil {
		return
	}

	// wait for the root object to be created, then enhance it. Oneshot for now.
	go func() {
		defer l.Close()

		_, err := l.Wait(ctx)
		if err != nil {
			fmt.Printf("Error waiting for event: %s\n", err.Error())
			return
		}

		ch.HandleCommand(
			context.Background(),
			&CreateRedfishResource{
				ID:          eh.NewUUID(),
				ResourceURI: "/redfish/v1/SessionService",
				Properties: map[string]interface{}{
					"@odata.type": "#SessionService.v1_0_2.SessionService",
					"Id":          "SessionService",
					"Name":        "Session Service",
					"Description": "Session Service",
					"Status": map[string]interface{}{
						"State":  "Enabled",
						"Health": "OK",
					},
					"ServiceEnabled": true,
					"SessionTimeout": 30,
					"Sessions": map[string]interface{}{
						"@odata.id": "/redfish/v1/SessionService/Sessions",
					},
					"@odata.context":     "/redfish/v1/$metadata#SessionService",
					"@odata.id":          "/redfish/v1/SessionService",
					"@Redfish.Copyright": "Copyright 2014-2016 Distributed Management Task Force, Inc. (DMTF). For the full DMTF copyright policy, see http://www.dmtf.org/about/policies/copyright.",
				}})

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
