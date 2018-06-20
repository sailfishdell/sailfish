package eventservice

import (
	"context"

	"github.com/superchalupa/go-redfish/src/ocp/view"

	"github.com/superchalupa/go-redfish/src/log"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func AddAggregate(ctx context.Context, logger log.Logger, v *view.View, rootID eh.UUID, ch eh.CommandHandler, eb eh.EventBus) {

	// Create SessionService aggregate
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          v.GetUUID(),
			ResourceURI: v.GetURI(),
			Type:        "#EventService.v1_0_4.EventService",
			Context:     "/redfish/v1/$metadata#EventService.EventService",
			Privileges: map[string]interface{}{
				"GET":    []string{"ConfigureManager"},
				"POST":   []string{},
				"PUT":    []string{},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{},
			},
			Properties: map[string]interface{}{
				"Id":   "EventService",
				"Name": "Event Service",
				"Status": map[string]interface{}{
					"State":  "Enabled",
					"Health": "OK",
				},
				"ServiceEnabled":                    true,
				"DeliveryRetryAttempts@meta":        v.Meta(view.PropGET("delivery_retry_attempts")),
				"DeliveryRetryIntervalSeconds@meta": v.Meta(view.PropGET("delivery_retry_interval_seconds")),
				"EventTypesForSubscription": []string{
					"StatusChange",
					"ResourceUpdated",
					"ResourceAdded",
					"ResourceRemoved",
					"Alert",
				},
				"Subscriptions": map[string]interface{}{
					"@odata.id": "/redfish/v1/EventService/Subscriptions",
				},
				"Actions": map[string]interface{}{
					"#EventService.SubmitTestEvent": map[string]interface{}{
						"target": v.GetActionURI("submit.test.event"),
						"EventType@Redfish.AllowableValues": []string{
							"StatusChange",
							"ResourceUpdated",
							"ResourceAdded",
							"ResourceRemoved",
							"Alert",
						},
					},
					"Oem": map[string]interface{}{},
				},
				"Oem": map[string]interface{}{},
			}})

	// Create Sessions Collection
	ch.HandleCommand(
		context.Background(),
		&domain.CreateRedfishResource{
			ID:          eh.NewUUID(),
			Collection:  true,
			ResourceURI: v.GetURI() + "/Subscriptions",
			Type:        "#EventDestinationCollection.EventDestinationCollection",
			Context:     "/redfish/v1/$metadata#EventDestinationCollection.EventDestinationCollection",
			Privileges: map[string]interface{}{
				"GET":    []string{"ConfigureManager"},
				"POST":   []string{"ConfigureManager"},
				"PUT":    []string{"ConfigureManager"},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{"ConfigureManager"},
			},
			Properties: map[string]interface{}{
				"Name":                "Event Subscriptions Collection",
				"Members@odata.count": 0,
				"Members":             []map[string]interface{}{},
			}})

	ch.HandleCommand(ctx,
		&domain.UpdateRedfishResourceProperties{
			ID: rootID,
			Properties: map[string]interface{}{
				"EventService": map[string]interface{}{"@odata.id": "/redfish/v1/EventService"},
			},
		})

	return
}
