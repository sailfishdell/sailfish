package eventservice

import (
	"context"
	"fmt"
	"sync"

	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func RegisterAggregate(s *testaggregate.Service) {
	s.RegisterAggregateFunction("eventservice",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#EventService.v1_1_0.EventService",
					Context:     "/redfish/v1/$metadata#EventService.EventService",
					Privileges: map[string]interface{}{
						"GET":   []string{"Login"},
						"PATCH": []string{"ConfigureManager"},
					},
					Properties: map[string]interface{}{
						"@odata.etag@meta":   vw.Meta(view.GETProperty("etag"), view.GETModel("etag")),
						"Id":                 "EventService",
						"Name":               "Event Service",
						"Description":        "Event Service represents the properties for the service",
						"ServerSentEventUri": "/redfish_events",
						"Status": map[string]interface{}{
							"HealthRollup": "OK", //hardcoded
							"Health":       "OK", //hardcoded
						},
						"ServiceEnabled":                    true,
						"DeliveryRetryAttempts@meta":        vw.Meta(view.PropGET("delivery_retry_attempts")),
						"DeliveryRetryIntervalSeconds@meta": vw.Meta(view.PropGET("delivery_retry_interval_seconds")),
						"EventTypesForSubscription": []string{
							"StatusChange",
							"ResourceUpdated",
							"ResourceAdded",
							"ResourceRemoved",
							"Alert",
						},
						"EventTypesForSubscription@odata.count": 5,
						"Actions": map[string]interface{}{
							"#EventService.SubmitTestEvent": map[string]interface{}{
								"target": vw.GetActionURI("submit.test.event"),
								"EventType@Redfish.AllowableValues": []string{
									"StatusChange",
									"ResourceUpdated",
									"ResourceAdded",
									"ResourceRemoved",
									"Alert",
								},
							},
						},
						"Oem": map[string]interface{}{ //??
							"sailfish": map[string]interface{}{
								"max_milliseconds_to_queue@meta": vw.Meta(view.PropGET("max_milliseconds_to_queue")),
								"max_events_to_queue@meta":       vw.Meta(view.PropGET("max_events_to_queue")),
							},
						},
					}},

				&domain.UpdateRedfishResourceProperties{
					ID: params["rootid"].(eh.UUID),
					Properties: map[string]interface{}{
						"EventService": map[string]interface{}{"@odata.id": vw.GetURI()},
					}},
			}, nil
		})

	s.RegisterAggregateFunction("subscriptioncollection",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#EventDestinationCollection.EventDestinationCollection",
					Context:     params["rooturi"].(string) + "/$metadata#EventDestinationCollection.EventDestinationCollection",
					// Plugin is how we find the POST command handler
					Plugin: "EventService",
					Privileges: map[string]interface{}{
						"GET":   []string{"Login"},
						"POST":  []string{"ConfigureManager"},
						"PUT":   []string{"ConfigureManager"},
						"PATCH": []string{"ConfigureManager"},
					},
					Properties: map[string]interface{}{
						"Name":                     "Event Subscriptions Collection",
						"Description":              "List of Event subscriptions",
						"Members@meta":             vw.Meta(view.GETProperty("members"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
						"Members@odata.count@meta": vw.Meta(view.GETProperty("members"), view.GETFormatter("count"), view.GETModel("default")),
					}},

				&domain.UpdateRedfishResourceProperties{
					ID: params["eventsvc_id"].(eh.UUID),
					Properties: map[string]interface{}{
						"Subscriptions": map[string]interface{}{"@odata.id": vw.GetURI()},
					}},
			}, nil
		})

	s.RegisterAggregateFunction("subscription",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),

					Type:    "#EventDestination.v1_2_0.EventDestination",
					Context: params["rooturi"].(string) + "/$metadata#EventDestination.EventDestination",
					// Plugin is how we find the POST command handler
					Plugin: "EventService",
					Privileges: map[string]interface{}{
						"GET":    []string{"Login"},
						"PUT":    []string{"ConfigureManager"},
						"PATCH":  []string{"ConfigureManager"},
						"DELETE": []string{"ConfigureManager"},
					},
					Properties: map[string]interface{}{
						"Id":                          vw.GetUUID(),
						"Protocol@meta":               vw.Meta(view.GETProperty("protocol"), view.GETModel("default")),
						"Name":                        fmt.Sprintf("EventSubscription %s", vw.GetUUID()),
						"Destination@meta":            vw.Meta(view.GETProperty("destination"), view.GETModel("default"), view.PropPATCH("session_timeout", "default")),
						"EventTypes@meta":             vw.Meta(view.GETProperty("event_types"), view.GETModel("default")),
						"EventTypes@odata.count@meta": vw.Meta(view.GETProperty("event_types"), view.GETFormatter("count"), view.GETModel("default")),
						"Context@meta":                vw.Meta(view.GETProperty("context"), view.GETModel("default")),
					}},
			}, nil
		})

	return
}
