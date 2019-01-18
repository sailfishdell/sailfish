package dell_ec

import (
	"context"
	"sync"

	"github.com/spf13/viper"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func RegisterThermalAggregate(s *testaggregate.Service) {
	s.RegisterAggregateFunction("thermal",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#Thermal.v1_0_2.Thermal",
					Context:     "/redfish/v1/$metadata#Thermal.Thermal",
					Privileges: map[string]interface{}{
						"GET":    []string{"Login"},
						"POST":   []string{}, // cannot create sub objects
						"PUT":    []string{},
						"PATCH":  []string{"ConfigureManager"},
						"DELETE": []string{}, // can't be deleted
					},
					Properties: map[string]interface{}{
						"Id":          "Thermal",
						"Name":        "Thermal",
						"Description": "Represents the properties for Temperature and Cooling",

						"Fans@meta":                     vw.Meta(view.GETProperty("fan_uris"), view.GETFormatter("expand"), view.GETModel("default")),
						"Fans@odata.count@meta":         vw.Meta(view.GETProperty("fan_uris"), view.GETFormatter("count"), view.GETModel("default")),
						"Temperatures@meta":             vw.Meta(view.GETProperty("temperature_uris"), view.GETFormatter("expand"), view.GETModel("default")),
						"Temperatures@odata.count@meta": vw.Meta(view.GETProperty("temperature_uris"), view.GETFormatter("count"), view.GETModel("default")),
						"Redundancy@meta":               vw.Meta(view.GETProperty("redundancy_uris"), view.GETFormatter("expand"), view.GETModel("default")),
						"Redundancy@odata.count@meta":   vw.Meta(view.GETProperty("redundancy_uris"), view.GETFormatter("count"), view.GETModel("default")),

						"Oem": map[string]interface{}{
							"EID_674": map[string]interface{}{
								"FansSummary": map[string]interface{}{
									"Status": map[string]interface{}{
										"HealthRollup@meta": vw.Meta(view.GETProperty("fan_rollup"), view.GETModel("global_health")),
										"Health@meta":       vw.Meta(view.GETProperty("fan_rollup"), view.GETModel("global_health")),
									},
								},
								"TemperaturesSummary": map[string]interface{}{
									"Status": map[string]interface{}{
										"HealthRollup@meta": vw.Meta(view.GETProperty("temperature_rollup"), view.GETModel("global_health")),
										"Health@meta":       vw.Meta(view.GETProperty("temperature_rollup"), view.GETModel("global_health")),
									},
								},
							},
						},
					}},
			}, nil
		})

	s.RegisterAggregateFunction("sensor",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#Thermal.v1_0_2.Temperature",
					Context:     "/redfish/v1/$metadata#Thermal.Thermal",
					Privileges: map[string]interface{}{
						"GET":    []string{"Login"},
						"POST":   []string{}, // cannot create sub objects
						"PUT":    []string{},
						"PATCH":  []string{},
						"DELETE": []string{}, // can't be deleted
					},
					Properties: map[string]interface{}{
						"Name":                           "Chassis Inlet Temperature",
						"Description":                    "Represents the properties for Temperature and Cooling",
						"LowerThresholdCritical@meta":    vw.Meta(view.GETProperty("LowerWarningThreshold"), view.GETModel("default")),
						"LowerThresholdNonCritical@meta": vw.Meta(view.GETProperty("LowerCriticalThreshold"), view.GETModel("default")),
						"MemberId":                       "System.Chassis.1",
						"ReadingCelsius@meta":            vw.Meta(view.GETProperty("sensorReading"), view.GETModel("default")),
						"Status": map[string]interface{}{
							"HealthRollup@meta": vw.Meta(view.GETProperty("sensorHealth"), view.GETModel("default")),
							"State":             "None", //hardcoded
							"Health@meta":       vw.Meta(view.GETProperty("sensorHealth"), view.GETModel("default")),
						},
						"UpperThresholdCritical@meta":    vw.Meta(view.GETProperty("UpperCriticalThreshold"), view.GETModel("default")),
						"UpperThresholdNonCritical@meta": vw.Meta(view.GETProperty("UpperWarningThreshold"), view.GETModel("default")),
					}},
			}, nil
		})
}
