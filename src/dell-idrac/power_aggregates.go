package dell_idrac

import (
	"context"
	"sync"
	//"errors"
	//"fmt"

	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	"github.com/superchalupa/sailfish/src/ocp/view"
	//"github.com/superchalupa/sailfish/src/ocp/awesome_mapper2"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func RegisterAggregate(s *testaggregate.Service) {
	s.RegisterAggregateFunction("power",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#Power.v1_5_0.Power",
					Context:     params["rooturi"].(string) + "/$metadata#Power.Power",
					Privileges: map[string]interface{}{
						"GET": []string{"Login"},
					},
					Properties: map[string]interface{}{
						"Id":          "Power",
						"Description": "Power",
						"Name":        "Power",

						"PowerSupplies@meta":             vw.Meta(view.GETProperty("power_supply_uris"), view.GETFormatter("expand"), view.GETModel("default")),
						"PowerSupplies@odata.count@meta": vw.Meta(view.GETProperty("power_supply_uris"), view.GETFormatter("count"), view.GETModel("default")),
						"PowerControl@meta":              vw.Meta(view.GETProperty("power_control_uris"), view.GETFormatter("expand"), view.GETModel("default")),
						"PowerControl@odata.count@meta":  vw.Meta(view.GETProperty("power_control_uris"), view.GETFormatter("count"), view.GETModel("default")),
						"Redundancy":              []interface{}{},
                                                "Redundancy@odata.count":  0,
						"Voltages@meta":             vw.Meta(view.GETProperty("voltage_sensor_uris"), view.GETFormatter("expand"), view.GETModel("default")),
						"Voltages@odata.count@meta": vw.Meta(view.GETProperty("voltage_sensor_uris"), view.GETFormatter("count"), view.GETModel("default")),

					}},
			}, nil
		})

	s.RegisterAggregateFunction("power_control",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#Power.v1_4_0.PowerControl",
					Context:     "/redfish/v1/$metadata#Power.Power",
					Privileges: map[string]interface{}{
						"GET":    []string{"Login"},
						"POST":   []string{}, // cannot create sub objects
						"PUT":    []string{},
						"PATCH":  []string{"ConfigureManager"},
						"DELETE": []string{}, // can't be deleted
					},
					Properties: map[string]interface{}{
						"Name":                     "System Power Control",
						"MemberId":                 "PowerControl",
						"PowerAllocatedWatts@meta": vw.Meta(view.PropGET("headroom_watts")),
						"PowerAvailableWatts@meta": vw.Meta(view.PropGET("headroom_watts")),
						"PowerCapacityWatts@meta":  vw.Meta(view.PropGET("capacity_watts")),
						"PowerConsumedWatts@meta":  vw.Meta(view.PropGET("consumed_watts")),
						"PowerLimit": map[string]interface{}{
							"CorrectionInMs":    0,
							"LimitException":    "HardPowerOff",
							"LimitInWatts@meta": vw.Meta(view.PropGET("limit_in_watts")),
						},
						"PowerMetrics": map[string]interface{}{
							"AverageConsumedWatts@meta": vw.Meta(view.PropGET("avgPwrLastMin")),
							"IntervalInMin":        1,
							"MaxConsumedWatts@meta":     vw.Meta(view.PropGET("maxPwrLastMin")),
							"MinConsumedWatts@meta":     vw.Meta(view.PropGET("minPwrLastMin")),
						},
						"PowerRequestedWatts@meta":     vw.Meta(view.PropGET("power_requested_watts")),
						"RelatedItem@meta":             vw.Meta(view.GETProperty("power_related_items"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
						"RelatedItem@odata.count@meta": vw.Meta(view.GETProperty("power_related_items"), view.GETFormatter("count"), view.GETModel("default")),
					},
				}}, nil
		})

	s.RegisterAggregateFunction("psu_slot",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#Power.v1_5_0.PowerSupply",
					Context:     "/redfish/v1/$metadata#Power.Power",
					Privileges: map[string]interface{}{
						"GET":    []string{"Login"},
						"POST":   []string{}, // cannot create sub objects
						"PUT":    []string{},
						"PATCH":  []string{"ConfigureManager"},
						"DELETE": []string{}, // can't be deleted
					},
					Properties: map[string]interface{}{
						"Name@meta":     vw.Meta(view.PropGET("name")),
						"MemberId@meta": vw.Meta(view.PropGET("member_id")),
						"Assembly": map[string]interface{}{
							"@odata.id": "/redfish/v1/Chassis/System.Embedded.1/Assembly",
						},
						"EfficiencyPercent@meta":    vw.Meta(view.PropGET("efficiency_percent")),
						"FirmwareVersion@meta":      vw.Meta(view.PropGET("fwVer")),
						"HotPluggable@meta":         vw.Meta(view.PropGET("hotpluggable")),
						"InputRanges":               []interface{}{
							map[string]interface{}{
							"InputType@meta":    vw.Meta(view.PropGET("input_pstype")),
							"MaximumFrequencyHz@meta": vw.Meta(view.PropGET("maximum_frequencyHz")),
							"MaximumVoltage@meta":	vw.Meta(view.PropGET("maximum_voltage")),
							"MinimumFrequencyHz@meta": vw.Meta(view.PropGET("minimum_frequencyHz")),
							"MinimumVoltage@meta": vw.Meta(view.PropGET("minimum_voltage")),
							"OutputWattage@meta": vw.Meta(view.PropGET("output_wattage")),
						}},
						"InputRanges@odata.count":   1,
						"LastPowerOutputWatts@meta": vw.Meta(view.PropGET("")),
						"LineInputVoltage@meta":     vw.Meta(view.PropGET("lineinputVoltage")),
						"LineInputVoltageType@meta": vw.Meta(view.PropGET("lineinputVoltagetype")),
						"Manufacturer@meta":         vw.Meta(view.PropGET("manufacturer")),
						"Model@meta":                vw.Meta(view.PropGET("model")),
						"PartNumber@meta":           vw.Meta(view.PropGET("boardpartnumber")),
						"PowerCapacityWatts@meta":   vw.Meta(view.PropGET("powercapacitywatts")),
						"PowerInputWatts@meta":      vw.Meta(view.PropGET("powerinputwatts")),
						"PowerOutputWatts@meta":     vw.Meta(view.PropGET("poweroutputwatts")),
						"PowerSupplyType@meta":      vw.Meta(view.PropGET("powersupplytype")),
						"SerialNumber@meta":         vw.Meta(view.PropGET("serialnumber")),
						"SparePartNumber@meta":      vw.Meta(view.PropGET("sparepartnumber")),

						"Status": map[string]interface{}{
							"State@meta":        vw.Meta(view.PropGET("obj_state")),
							"Health@meta":       vw.Meta(view.PropGET("obj_status")),
						},
						// this should be a link using getformatter
						"Redundancy":              []interface{}{},
						"Redundancy@odata.count":  0,
						"RelatedItem":             []interface{}{},
						"RelatedItem@odata.count": 0,
					},
				}}, nil
		})

	s.RegisterAggregateFunction("voltage_sensor",
                func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
                        return []eh.Command{
                                &domain.CreateRedfishResource{
                                        ResourceURI: vw.GetURI(),
                                        Type:        "#Power.v1_3_0.Voltage",
                                        Context:     "/redfish/v1/$metadata#Power.Power",
                                        Privileges: map[string]interface{}{
                                                "GET":    []string{"Login"},
                                                "POST":   []string{}, // cannot create sub objects
                                                "PUT":    []string{},
                                                "PATCH":  []string{"ConfigureManager"},
                                                "DELETE": []string{}, // can't be deleted
                                        },
                                        Properties: map[string]interface{}{
                                                "Name@meta":                     vw.Meta(view.PropGET("name")),
                                                "MemberId@meta":                 vw.Meta(view.PropGET("unique_name")),
                                                "LowerThresholdCritical@meta": vw.Meta(view.PropGET("lowerthresholdcritical")),
                                                "LowerThresholdFatal@meta": vw.Meta(view.PropGET("lowerthresholdfatal")),
                                                "LowerThresholdNonCritical@meta":  vw.Meta(view.PropGET("lowerthresholdnoncritical")),
                                                "MaxReadingRange@meta":  vw.Meta(view.PropGET("max_reading_range")),
						"MinReadingRange@meta":  vw.Meta(view.PropGET("min_reading_range")),
						"PhysicalContext@meta":  vw.Meta(view.PropGET("physical_context")),
						"ReadingVolts@meta":  vw.Meta(view.PropGET("reading_volts")),
						"SensorNumber@meta":  vw.Meta(view.PropGET("sensor_number")),
						"UpperThresholdCritical@meta":  vw.Meta(view.PropGET("upperthresholdcritical")),
						"UpperThresholdFatal@meta":  vw.Meta(view.PropGET("upperthresholdfatal")),
						"UpperThresholdNonCritical@meta":  vw.Meta(view.PropGET("upperthresholdnoncritical")),
                                                "Status": map[string]interface{}{
                                                        "State@meta":        vw.Meta(view.PropGET("obj_status")),
                                                        "Health@meta":       vw.Meta(view.PropGET("obj_status")),
                                                },
                                                "RelatedItem@meta":             vw.Meta(view.GETProperty("power_related_items"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
                                                "RelatedItem@odata.count@meta": vw.Meta(view.GETProperty("power_related_items"), view.GETFormatter("count"), view.GETModel("default")),
                                        },
                                }}, nil
                })


}
