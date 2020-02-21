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
						"GET":   []string{"Login"},
						"PATCH": []string{"ConfigureManager"},
					},
					Properties: map[string]interface{}{
						"Id":                            "Thermal",
						"Name":                          "Thermal",
						"Description":                   "Represents the properties for Temperature and Cooling",
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
										"HealthRollup": nil,
										"Health":       nil,
									},
								},
								"TemperaturesSummary": map[string]interface{}{
									"Status": map[string]interface{}{
										"HealthRollup": nil,
										"Health":       nil,
									},
								},
							},
						},
					}},
			}, nil
		})
}

// small helper to avoid setting temperatures that should be nil
func updateTemperature(properties map[string]interface{}, key string, value int) {
	if value != -128 {
		properties[key] = value
	}
}

func health_map(health int) interface{} {

	switch health {
	case 0, 1: //other, unknown
		return nil
	case 2: //ok
		return "OK"
	case 3: //non-critical
		return "Warning"
	case 4, 5: //critical, non-recoverable
		return "Critical"
	default:
		return nil
	}
}

