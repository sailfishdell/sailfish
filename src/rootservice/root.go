package rootservice

import (
	"context"

	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

// wait in a listener for the root service to be created, then extend it
func InitService(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	// set up some basic stuff
	rootID := eh.NewUUID()
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          rootID,
			ResourceURI: "/redfish/v1",
			Type:        "#ServiceRoot.v1_0_2.ServiceRoot",
			Context:     "/redfish/v1/$metadata#ServiceRoot.ServiceRoot",
			// anybody can access
			Privileges: map[string]interface{}{"GET": []string{"Unauthenticated"}},
			Properties: map[string]interface{}{
				"Id":             "RootService",
				"Name":           "Root Service",
				"RedfishVersion": "1.0.2",
				"UUID":           rootID,
			},
		},
	)
}
