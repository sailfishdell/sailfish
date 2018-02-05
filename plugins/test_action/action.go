package test_action

import (
	"context"
	"time"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
	domain "github.com/superchalupa/go-redfish/redfishresource"

	"github.com/superchalupa/go-redfish/plugins"
	ah "github.com/superchalupa/go-redfish/plugins/actionhandler"
)

func init() {
	domain.RegisterInitFN(CreateTestActionEndpoint)
}

// Example of creating a minimal tree object to recieve action requests. Doesn't need much more
func CreateTestActionEndpoint(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	// Step 1: Add entry point for test action
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          eh.NewUUID(),
			ResourceURI: "/redfish/v1/Actions/Test",
			Type:        "Action",
			Context:     "Action",
			Plugin:      "GenericActionHandler",
			Privileges: map[string]interface{}{
				"POST": []string{"ConfigureManager"},
			},
			Properties: map[string]interface{}{},
		},
	)

	// step 2: Create event stream processor to handle action request
	sp, err := plugins.NewEventStreamProcessor(ctx, ew, plugins.CustomFilter(ah.SelectAction("/redfish/v1/Actions/Test")))
	if err == nil {
		sp.RunForever(func(event eh.Event) {
			eb.HandleEvent(ctx, eh.NewEvent(domain.HTTPCmdProcessed, domain.HTTPCmdProcessedData{
				CommandID:  event.Data().(ah.GenericActionEventData).CmdID,
				Results:    map[string]interface{}{"TEST RESULT": "SUCCESS"},
				StatusCode: 200,
				Headers:    map[string]string{},
			}, time.Now()))
		})
	}
}
