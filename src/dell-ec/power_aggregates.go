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
					Context:     params["rooturi"].(string) + "/$metadata#Power.Power",
					Privileges: map[string]interface{}{
						"GET": []string{"Login"},
					},
					Properties: map[string]interface{}{
						"Id":                             "Power",
						"Description":                    "Power",
						"Name":                           "Power",
						"@odata.etag":                    `W/"abc123"`,
						"PowerSupplies@meta":             vw.Meta(view.GETProperty("power_supply_uris"), view.GETFormatter("expand"), view.GETModel("default")),
						"PowerSupplies@odata.count@meta": vw.Meta(view.GETProperty("power_supply_uris"), view.GETFormatter("count"), view.GETModel("default")),
						"PowerControl@meta":              vw.Meta(view.GETProperty("power_control_uris"), view.GETFormatter("expand"), view.GETModel("default")),
						"PowerControl@odata.count@meta":  vw.Meta(view.GETProperty("power_control_uris"), view.GETFormatter("count"), view.GETModel("default")),
						"Oem": map[string]interface{}{
							"Dell": map[string]interface{}{
								"PowerSuppliesSummary": map[string]interface{}{
									"Status": map[string]interface{}{
										"HealthRollup": nil,
									},
								},
								"PowerTrends@meta":             vw.Meta(view.GETProperty("power_trends_uri"), view.GETFormatter("expand"), view.GETModel("default")),
								"PowerTrends@odata.count@meta": vw.Meta(view.GETProperty("power_trends_uri"), view.GETFormatter("count"), view.GETModel("default")),
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
					Context:     "/redfish/v1/$metadata#Power.Power",
					Privileges: map[string]interface{}{
						"GET": []string{"Login"},
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
					Context:     "/redfish/v1/$metadata#Power.Power",
					Privileges: map[string]interface{}{
						"GET": []string{"Login"},
					},
					Properties: map[string]interface{}{
						"Name":                "System Power History",
						"MemberId":            "PowerHistogram",
						"HistoryMaxWattsTime": nil,
						"HistoryMaxWatts":     0,
						"HistoryMinWattsTime": nil,
						"HistoryMinWatts":     0,
						"HistoryAverageWatts": 0,
					}},
			}, nil
		})

	s.RegisterAggregateFunction("power_control",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#Power.v1_0_0.PowerControl",
					Context:     "/redfish/v1/$metadata#Power.v1_0_0.PowerControl",
					Privileges: map[string]interface{}{
						"GET":   []string{"Login"},
						"PATCH": []string{"ConfigureManager"},
					},
					Properties: map[string]interface{}{
						"Name":                    "System Power Control",
						"MemberId":                "PowerControl",
						"PowerAvailableWatts":     0,
						"PowerCapacityWatts@meta": vw.Meta(view.PropGET("capacity_watts")), //System.Chassis.1#ChassisPower.1#SystemInputMaxPowerCapacity
						"PowerConsumedWatts@meta": vw.Meta(view.PropGET("consumed_watts")),

						"Oem": map[string]interface{}{
							"EnergyConsumptionStartTime@meta": 0,
							"EnergyConsumptionkWh":            0,
							"HeadroomWatts":                   0,
							"MaxPeakWatts":                    0,
							"MaxPeakWattsTime":                0,
							"MinPeakWatts":                    0,
							"MinPeakWattsTime":                0,
							"PeakHeadroomWatts":               0,
						},

						"PowerLimit": map[string]interface{}{
							"LimitInWatts": 0,
						},
						"PowerMetrics": map[string]interface{}{
							"AverageConsumedWatts": 0,
							"IntervalInMin":        0,
							"MaxConsumedWatts":     0,
							"MinConsumedWatts":     0,
						},
						"RelatedItem": []interface{}{
							map[string]interface{}{
								"@odata.id": "/redfish/v1/Chassis/System.Chassis.1",
							},
						},
						"RelatedItem@odata.count": 1,
					},
				}}, nil
		})

	s.RegisterAggregateFunction("psu_slot",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#Power.v1_0_0.PowerSupply",
					Context:     "/redfish/v1/$metadata#Power.v1_0_0.PowerSupply",
					Privileges: map[string]interface{}{
						"GET":   []string{"Login"},
						"PATCH": []string{"ConfigureManager"},
					},
					Properties: map[string]interface{}{
						"Name@meta":               vw.Meta(view.PropGET("name")),
						"MemberId@meta":           vw.Meta(view.PropGET("unique_name")),
						"PowerCapacityWatts@meta": vw.Meta(view.PropGET("capacity_watts")),
						"LineInputVoltage":        0,
						"FirmwareVersion@meta":    vw.Meta(view.PropGET("firmware_version")),

						"Status": map[string]interface{}{
							"HealthRollup": "OK",
							"State@meta":   vw.Meta(view.PropGET("state")),
							"Health":       "OK",
						},

						"Oem": map[string]interface{}{
							"Dell": map[string]interface{}{
								"@odata.type":      "#DellPower.v1_0_0.DellPowerSupply",
								"ComponentID@meta": vw.Meta(view.PropGET("component_id")),
								"InputCurrent":     0,
								"Attributes@meta":  vw.Meta(view.GETProperty("attributes"), view.GETFormatter("attributeFormatter"), view.GETModel("default"), view.PropPATCH("attributes", "ar_dump")),
							},
						},
						// this should be a link using getformatter
						"Redundancy":             []interface{}{},
						"Redundancy@odata.count": 0,
					},
				}}, nil
		})

}
