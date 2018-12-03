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
					Type:        "#Power.v1_0_2.Power",
					Context:     params["rooturi"].(string) + "/$metadata#Power.PowerSystem.Chassis.1/Power/$entity",
					Privileges: map[string]interface{}{
						"GET": []string{"Unauthenticated"},
					},
					Properties: map[string]interface{}{
						"Id":          "Power",
						"Description": "Power",
						"Name":        "Power",
						"@odata.etag": `W/"abc123"`,

						"PowerSupplies@meta":             vw.Meta(view.GETProperty("power_supply_uris"), view.GETFormatter("expand"),view.GETModel("default")),
						"PowerSupplies@odata.count@meta": vw.Meta(view.GETProperty("power_supply_uris"), view.GETFormatter("count"), view.GETModel("default")),
						"PowerControl@meta":              vw.Meta(view.GETProperty("power_control_uris"), view.GETFormatter("expand"), view.GETModel("default")),
						"PowerControl@odata.count@meta":  vw.Meta(view.GETProperty("power_control_uris"), view.GETFormatter("count"), view.GETModel("default")),
						}},
			}, nil
		})


	s.RegisterAggregateFunction("power_control",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#Power.v1_0_2.PowerControl",
					Context:     "/redfish/v1/$metadata#Power.PowerSystem.Chassis.1/Power/$entity",
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
						"PowerAllocatedWatts@meta":	vw.Meta(view.PropGET("headroom_watts")),
						"PowerAvailableWatts@meta": vw.Meta(view.PropGET("headroom_watts")),
						"PowerCapacityWatts@meta":  vw.Meta(view.PropGET("capacity_watts")), //System.Chassis.1#ChassisPower.1#SystemInputMaxPowerCapacity
						"PowerConsumedWatts@meta":  vw.Meta(view.PropGET("consumed_watts")),
						"PowerLimit": map[string]interface{}{
							"CorrectionInMs": 0,
							"LimitException": "HardPowerOff",
							"LimitInWatts@meta": vw.Meta(view.PropGET("limit_in_watts")),
						},
						"PowerMetrics": map[string]interface{}{
							"AverageConsumedWatts": 0,
							"IntervalInMin":        0,
							"MaxConsumedWatts":     0,
							"MinConsumedWatts":     0,
						},
						"PowerRequestedWatts@meta": vw.Meta(view.PropGET("peak_headroom_watts")),
						"RelatedItem@meta":   vw.Meta(view.GETProperty("power_related_items"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
						"RelatedItem@odata.count@meta": vw.Meta(view.GETProperty("power_related_items"), view.GETFormatter("count"), view.GETModel("default")),
					},
				}}, nil
		})

	s.RegisterAggregateFunction("psu_slot",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#Power.v1_0_2.PowerSupply",
					Context:     "/redfish/v1/$metadata#Power.PowerSystem.Chassis.1/Power/$entity",
					Privileges: map[string]interface{}{
						"GET":    []string{"Login"},
						"POST":   []string{}, // cannot create sub objects
						"PUT":    []string{},
						"PATCH":  []string{"ConfigureManager"},
						"DELETE": []string{}, // can't be deleted
					},
					Properties: map[string]interface{}{
						"Name@meta":               vw.Meta(view.PropGET("name")),
						"MemberId@meta":           vw.Meta(view.PropGET("unique_name")),
						"Assembly": map[string]interface{}{
							"@odata.id":"/redfish/v1/Chassis/System.Embedded.1/Assembly",
						},
						"EfficiencyPercent@meta": vw.Meta(view.PropGET("capacity_watts")),
						"FirmwareVersion@meta":    vw.Meta(view.PropGET("firmware_version")),
						"HotPluggable@meta": vw.Meta(view.PropGET("firmware_version")),
						"InputRanges": []interface{}{},
						"InputRanges@odata.count": 0,
						"LastPowerOutputWatts@meta": vw.Meta(view.PropGET("firmware_version")),
						"LineInputVoltage@meta":   vw.Meta(view.PropGET("line_input_voltage")),
						"LineInputVoltageType@meta": vw.Meta(view.PropGET("line_input_voltage")),
						"Manufacturer": "Dell",
						"Model@meta": vw.Meta(view.PropGET("capacity_watts")),
						"PartNumber@meta": vw.Meta(view.PropGET("capacity_watts")),
						"PowerCapacityWatts@meta": vw.Meta(view.PropGET("capacity_watts")),
						"PowerInputWatts@meta": vw.Meta(view.PropGET("capacity_watts")),
						"PowerOutputWatts@meta": vw.Meta(view.PropGET("capacity_watts")),
						"PowerSupplyType@meta": vw.Meta(view.PropGET("capacity_watts")),
						"SerialNumber@meta": vw.Meta(view.PropGET("capacity_watts")),
						"SparePartNumber@meta": vw.Meta(view.PropGET("capacity_watts")),

						"Status": map[string]interface{}{
							"HealthRollup@meta": vw.Meta(view.PropGET("obj_status")),
							"State@meta":        vw.Meta(view.PropGET("state")),
							"Health@meta":       vw.Meta(view.PropGET("obj_status")),
						},
						// this should be a link using getformatter
						"Redundancy":             []interface{}{},
						"Redundancy@odata.count": 0,
						"RelatedItem": []interface{}{},
						"RelatedItem@odata.count": 0,
					},
				}}, nil
		})

}
