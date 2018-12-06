package logservices

import (
	"context"
	"sync"

	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func RegisterAggregate(s *testaggregate.Service) {
	s.RegisterAggregateFunction("logservices",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#LogServiceCollection.LogServiceCollection",
					Context:     params["rooturi"].(string) + "/$metadata#LogServiceCollection.LogServiceCollection",
					Privileges: map[string]interface{}{
						"GET": []string{"Login"},
					},
					Properties: map[string]interface{}{
						"Name":                     "Log Service Collection",
						"Description":              "Collection of Log Services for this Manager",
						"Members@meta":             vw.Meta(view.GETProperty("members"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
						"Members@odata.count@meta": vw.Meta(view.GETProperty("members"), view.GETFormatter("count"), view.GETModel("default")),
					}},
			}, nil
		})

	s.RegisterAggregateFunction("lclogservices",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#LogService.v1_0_2.LogService",
					Context:     params["rooturi"].(string) + "/$metadata#LogService.LogService",
					Privileges: map[string]interface{}{
						"GET": []string{"Login"},
					},
					Properties: map[string]interface{}{
						"Name":               "LifeCycle Controller Log Service",
						"Description":        "LifeCycle Controller Log Service",
						"OverWritePolicy":    "WrapsWhenFull",
						"MaxNumberOfRecords": 500000,
						"ServiceEnabled":     true,
						"Entries": map[string]interface{}{
							"@odata.id": "/redfish/v1/Managers/CMC.Integrated.1/Logs/Lclog",
						},
            "DateTime@meta":        map[string]interface{}{"GET": map[string]interface{}{"plugin": "datetime"}},
						"DateTimeLocalOffset": "+00:00",
						"Id": "LC",
						"Actions": map[string]interface{}{
							"#LogService.ClearLog": map[string]interface{}{
								"target": vw.GetActionURI("clearlog"),
							},
						},
					}},
			}, nil
		})

	s.RegisterAggregateFunction("lclogentrycollection",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#LogService.v1_0_2.LogService",
					Context:     params["rooturi"].(string) + "/$metadata#LogService.LogService",
					Privileges: map[string]interface{}{
						"GET": []string{"Login"},
					},
					Properties: map[string]interface{}{
						"Description":              "LC Logs for this manager",
						"Name":                     "Log Entry Collection",
						"Members@meta":             vw.Meta(view.GETProperty("members"), view.GETFormatter("expand"), view.GETModel("default")),
						"Members@odata.count@meta": vw.Meta(view.GETProperty("members"), view.GETFormatter("count"), view.GETModel("default")),
						"Members@odata.nextLink":   "/redfish/v1/Managers/CMC.Integrated.1/Logs/Lclog?$skip=50",
					}},
			}, nil
		})

	s.RegisterAggregateFunction("faultlistservices",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#LogService.v1_0_2.LogService",
					Context:     params["rooturi"].(string) + "/$metadata#LogService.LogService",
					Privileges: map[string]interface{}{
						"GET": []string{"Login"},
					},
					Properties: map[string]interface{}{
						"Name":        "FaultListEntries",
						"Description": "Collection of FaultList Entries",
						"Entries": map[string]interface{}{
							"@odata.id": "/redfish/v1/Managers/CMC.Integrated.1/Logs/FaultList",
						},
						"OverWritePolicy":     "WrapsWhenFull",
						"MaxNumberOfRecords":  500000,
						"ServiceEnabled":      true,
						"@odata.id":           "/redfish/v1/Managers/CMC.Integrated.1/LogServices/FaultList",
						"DateTimeLocalOffset": "+00:00",
            "DateTime@meta":        map[string]interface{}{"GET": map[string]interface{}{"plugin": "datetime"}},
						"Id":                  "FaultList",
					}},
			}, nil
		})

	s.RegisterAggregateFunction("faultlistentrycollection",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#LogEntryCollection.LogEntryCollection",
					Context:     params["rooturi"].(string) + "/$metadata#LogEntryCollection.LogEntryCollection",
					Privileges: map[string]interface{}{
						"GET": []string{"Login"},
					},
					Properties: map[string]interface{}{
						"Description":              "Providing additional health information for the devices which support rolled up health data",
						"Name":                     "FaultList Entries Collection",
						"Members@meta":             vw.Meta(view.GETProperty("members"), view.GETFormatter("expand"), view.GETModel("default")),
						"Members@odata.count@meta": vw.Meta(view.GETProperty("members"), view.GETFormatter("count"), view.GETModel("default")),
					}},
			}, nil
		})

	return
}
