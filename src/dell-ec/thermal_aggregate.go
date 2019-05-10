package dell_ec

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/spf13/viper"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	"github.com/superchalupa/sailfish/src/dell-resources/dm_event"
	"github.com/superchalupa/sailfish/src/ocp/awesome_mapper2"

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

func initThermalSensor(logger log.Logger, instantiateSvc *testaggregate.Service, ch eh.CommandHandler, d *domain.DomainObjects) {

	awesome_mapper2.AddFunction("updateSensorEvent", func(args ...interface{}) (interface{}, error) {
		sensorUri, ok := args[0].(string)
		if !ok {
			logger.Crit("uri not passed as string", "args[0]", args[0])
			return nil, errors.New("uri not passed as string")
		}

		thermalSensorEvent, ok := args[1].(*dm_event.ThermalSensorEventData)
		if !ok {
			logger.Crit("ThermalSensorEventData passed", "args[1]", args[1], "TYPE", fmt.Sprintf("%T", args[1]))
			return nil, errors.New("ThermalSensorEventData not passed")
		}

		// crate the sensor properties, the temperatures are set to nil to start, values that are not
		// -128 are left nil.
		var sensorProperties = map[string]interface{}{
			"Name":                      thermalSensorEvent.OffsetDeviceName,
			"Description":               "Represents the properties for Temperature and Cooling",
			"LowerThresholdCritical":    nil,
			"LowerThresholdNonCritical": nil,
			"MemberId":                  thermalSensorEvent.OffsetDeviceFQDD,
			"ReadingCelsius":            nil,
			"Status": map[string]interface{}{
				"HealthRollup": health_map(thermalSensorEvent.SensorHealth),
				"State":        nil, //hardcoded
				"Health":       health_map(thermalSensorEvent.SensorHealth),
			},
			"UpperThresholdCritical":    nil,
			"UpperThresholdNonCritical": nil,
		}

		// update temperatures.
		updateTemperature(sensorProperties, "ReadingCelsius", thermalSensorEvent.SensorReading)
		updateTemperature(sensorProperties, "LowerThresholdCritical", thermalSensorEvent.LowerCriticalThreshold)
		updateTemperature(sensorProperties, "LowerThresholdNonCritical", thermalSensorEvent.LowerWarningThreshold)
		updateTemperature(sensorProperties, "UpperThresholdCritical", thermalSensorEvent.UpperCriticalThreshold)
		updateTemperature(sensorProperties, "UpperThresholdNonCritical", thermalSensorEvent.UpperWarningThreshold)

		sensorUri += "/" + thermalSensorEvent.ObjectHeader.FQDD

		// remove any existing one
		id, ok := d.GetAggregateIDOK(sensorUri)
		if ok && !((thermalSensorEvent.SensorStateMask & 1) == 1) {
			// exists and needs to be removed
			logger.Debug("remove sensor", "id", id, "ok", ok, "URI", sensorUri)
			ch.HandleCommand(context.Background(), &domain.RemoveRedfishResource{ID: id})
		} else if !ok && ((thermalSensorEvent.SensorStateMask & 1) == 1) {
			// doesn't exist but neeeds to be added
			uuid := eh.NewUUID()
			logger.Debug("Need to add a sensor", "id", id, "ok", ok, "uuid", uuid, "URI", sensorUri)
			ch.HandleCommand(
				context.Background(),
				&domain.CreateRedfishResource{
					ID:          uuid,
					ResourceURI: sensorUri,
					Type:        "#Thermal.v1_0_0.Temperature",
					Context:     "/redfish/v1/$metadata#Thermal.Thermal",
					Privileges: map[string]interface{}{
						"GET": []string{"Login"},
					},
					Properties: sensorProperties,
				},
			)
		} else if ok && ((thermalSensorEvent.SensorStateMask & 1) == 1) {
			// exists and needs to be updated
			logger.Debug("update sensor", "id", id, "URI", sensorUri)

			// only update the values from the sensor event, the rest can stay (they won't change)
			ch.HandleCommand(
				context.Background(),
				&domain.UpdateRedfishResourceProperties{
					ID:         id,
					Properties: sensorProperties,
				},
			)
		}
		return true, nil
	})
}
