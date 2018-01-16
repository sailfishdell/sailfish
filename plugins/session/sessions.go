package session

import (
	"context"
	"fmt"

	domain "github.com/superchalupa/go-redfish/internal/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

func SetupSessionService(ctx context.Context, rootID eh.UUID, ew *utils.EventWaiter, ch eh.CommandHandler, eb eh.EventBus) {
	fmt.Printf("SetupSessionService\n")

	// register our command
	eh.RegisterCommand(func() eh.Command { return &POST{eventBus: eb, commandHandler: ch} })

	l, err := ew.Listen(ctx, func(event eh.Event) bool {
		if event.EventType() != domain.RedfishResourceCreated {
			return false
		}
		if data, ok := event.Data().(*domain.RedfishResourceCreatedData); ok {
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

		// Create SessionService aggregate
		ch.HandleCommand(
			context.Background(),
			&domain.CreateRedfishResource{
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

		// Create Sessions Collection
		ch.HandleCommand(
			context.Background(),
			&domain.CreateRedfishResource{
				ID:          eh.NewUUID(),
				Plugin:      "SessionService",
				ResourceURI: "/redfish/v1/SessionService/Sessions",
				Collection:  true,
				Properties: map[string]interface{}{
					"@odata.type":         "#SessionCollection.SessionCollection",
					"Name":                "Session Collection",
					"Members@odata.count": 0,
					"Members":             []map[string]interface{}{},
					"@odata.context":      "/redfish/v1/$metadata#SessionService/Sessions/$entity",
					"@Redfish.Copyright":  "Copyright 2014-2016 Distributed Management Task Force, Inc. (DMTF). For the full DMTF copyright policy, see http://www.dmtf.org/about/policies/copyright.",
				}})

		// Create Sessions Collection
		ch.HandleCommand(
			context.Background(),
			&domain.CreateRedfishResource{
				ID:          eh.NewUUID(),
				ResourceURI: "/redfish/v1/SessionService/Sessions/1234567890ABCDEF",
				Properties: map[string]interface{}{
					"@odata.type":        "#Session.v1_0_2.Session",
					"Id":                 "1234567890ABCDEF",
					"Name":               "User Session",
					"Description":        "Manager User Session",
					"UserName":           "Administrator",
					"Oem":                map[string]interface{}{},
					"@odata.context":     "/redfish/v1/$metadata#Session.Session",
					"@odata.id":          "/redfish/v1/SessionService/Sessions/1234567890ABCDEF",
					"@Redfish.Copyright": "Copyright 2014-2016 Distributed Management Task Force, Inc. (DMTF). For the full DMTF copyright policy, see http://www.dmtf.org/about/policies/copyright.",
				}})

		ch.HandleCommand(ctx,
			&domain.UpdateRedfishResourceProperties{
				ID: rootID,
				Properties: map[string]interface{}{
					"SessionService": map[string]interface{}{"@odata.id": "/redfish/v1/SessionService"},
					"Links":          map[string]interface{}{"Sessions": map[string]interface{}{"@odata.id": "/redfish/v1/SessionService/Sessions"}},
				},
			})
	}()
}
