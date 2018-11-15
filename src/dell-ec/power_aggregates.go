package dell_ec

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
	s.RegisterAggregateFunction("power",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#Power.v1_0_2.Power",
					Context:     params["rooturi"].(string) + "/redfish/v1/$metadata#Power.PowerSystem.Chassis.1/Power/$entity",
					Privileges: map[string]interface{}{
						"GET": []string{"Unauthenticated"},
					},
					Properties: map[string]interface{}{
						"Id":          "Power",
						"Description": "Power",
						"Name":        "Power",
						"@odata.etag": `W/"abc123"`,

						"PowerSupplies@meta":             vw.Meta(view.GETProperty("power_supply_uris"), view.GETFormatter("expand"), view.GETModel("default")),
						"PowerSupplies@odata.count@meta": vw.Meta(view.GETProperty("power_supply_uris"), view.GETFormatter("count"), view.GETModel("default")),
						"PowerControl@meta":              vw.Meta(view.GETProperty("power_control_uris"), view.GETFormatter("expand"), view.GETModel("default")),
						"PowerControl@odata.count@meta":  vw.Meta(view.GETProperty("power_control_uris"), view.GETFormatter("count"), view.GETModel("default")),
						"Oem": map[string]interface{}{
							"OemPower": map[string]interface{}{
								"PowerTrends@meta":        vw.Meta(view.GETProperty("power_trends_uri"), view.GETFormatter("expandone"), view.GETModel("default")),
								"PowerTrends@odata.count": 7, // TODO: Fix this, it's wrong... this shoulndt even be here
							},
							"EID_674": map[string]interface{}{
								"PowerSuppliesSummary": map[string]interface{}{
									"Status": map[string]interface{}{
										"HealthRollup@meta": vw.Meta(view.GETProperty("psu_rollup"), view.GETModel("global_health")),
									},
								},
							},
						}}},
			}, nil
		})

	s.RegisterAggregateFunction("power_trends",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{

					ResourceURI: vw.GetURI(),
					Type:        "#DellPower.v1_0_0.DellPowerTrends",
					Context:     "/redfish/v1/$metadata#Power.PowerSystem.Chassis.1/Power/$entity",
					Privileges: map[string]interface{}{
						"GET":    []string{"Login"},
						"POST":   []string{}, // cannot create sub objects
						"PUT":    []string{},
						"PATCH":  []string{},
						"DELETE": []string{}, // can't be deleted
					},
					Properties: map[string]interface{}{
						"Name":                        "System Power",
						"MemberId":                    "PowerHistogram",
						"histograms@meta":             vw.Meta(view.GETProperty("trend_histogram_uris"), view.GETFormatter("expand"), view.GETModel("default")),
						"histograms@odata.count@meta": vw.Meta(view.GETProperty("trend_histogram_uris"), view.GETFormatter("count"), view.GETModel("default")),
					}},
			}, nil
		})

	s.RegisterAggregateFunction("trend_histogram",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#DellPower.v1_0_0.DellPowerTrend",
					Context:     "/redfish/v1/$metadata#Power.PowerSystem.Chassis.1/Power/$entity",
					Privileges: map[string]interface{}{
						"GET":    []string{"Login"},
						"POST":   []string{}, // cannot create sub objects
						"PUT":    []string{},
						"PATCH":  []string{},
						"DELETE": []string{}, // can't be deleted
					},
					Properties: map[string]interface{}{
						"Name":                     "System Power History",
						"MemberId":                 "PowerHistogram",
						"HistoryMaxWattsTime@meta": vw.Meta(view.GETProperty("max_watts_time"), view.GETModel("default")),
						"HistoryMaxWatts@meta":     vw.Meta(view.GETProperty("max_watts"), view.GETModel("default")),
						"HistoryMinWattsTime@meta": vw.Meta(view.GETProperty("min_watts_time"), view.GETModel("default")),
						"HistoryMinWatts@meta":     vw.Meta(view.GETProperty("min_watts"), view.GETModel("default")),
						"HistoryAverageWatts@meta": vw.Meta(view.GETProperty("average_watts"), view.GETModel("default")),
					}},
			}, nil
		})

}
