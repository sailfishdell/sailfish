package enclosure

import (
	"context"
"sync"

"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/view"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func RegisterAggregate(s *testaggregate.Service) {
	s.RegisterAggregateFunction("idrac_storage_enclosure",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{

					ResourceURI: vw.GetURI(),
					Type:        "#Chassis.v1_6_0.Chassis",
					Context:     "/redfish/v1/$metadata#Chassis.Chassis",
					Privileges: map[string]interface{}{
						"GET": []string{"Login"},
					},

					Properties: map[string]interface{}{
						"@Redfish.Settings": map[string]interface{}{ //Done
							"@odata.context": "/redfish/v1/$metadata#Settings.Settings",
							"@odata.id":      "/redfish/v1/Chassis/$Entity/Settings",
							"@odata.type":    "#Settings.v1_1_0.Settings",
							"SupportedApplyTimes": []string{
								"Immediate",
								"OnReset",
								"AtMaintenanceWindowStart",
								"InMaintenanceWindowOnReset",
							},
						},
						"Actions":          map[string]interface{}{},
						"AssetTag@meta":    vw.Meta(view.PropGET("asset_tag")), //Done
						"ChassisType@meta": vw.Meta(view.PropGET("chassis_type")),
						"Description@meta": vw.Meta(view.PropGET("description")),
						"Id@meta":          vw.Meta(view.PropGET("unique_name")),
						"Links":            map[string]interface{}{
							//Need to add links detail.
						},
						"Links@meta":             vw.Meta(view.GETProperty("links_uris"), view.GETFormatter("expand"), view.GETModel("default")),
						"Links@odata.count@meta": vw.Meta(view.GETProperty("links_uris"), view.GETFormatter("count"), view.GETModel("default")),
						"Manufacturer@meta":      vw.Meta(view.PropGET("manufacturer")), //Done
						"Model@meta":             vw.Meta(view.PropGET("model")),        //Done
						"Name@meta":              vw.Meta(view.PropGET("name")),         //Done
						"Oem": map[string]interface{}{ //Done
							"Dell": map[string]interface{}{
								"DellEnclosure@meta": vw.Meta(view.GETProperty("enclosure_uris"), view.GETFormatter("expandone"), view.GETModel("default")),
							},
						},
						"PCIeDevices":                  map[string]interface{}{},
						"PCIeDevices@odata.count@meta": vw.Meta(view.PropGET("pcie_devices_count")),
						"PartNumber@meta":              vw.Meta(view.PropGET("part_number")),
						"PowerState@meta":              vw.Meta(view.PropGET("power_state")),
						"SKU@meta":                     vw.Meta(view.PropGET("sku")),
						"SerialNumber@meta":            vw.Meta(view.PropGET("serial_number")),
						"Status": map[string]interface{}{
							"HealthRollup@meta": vw.Meta(view.PropGET("health")),
							"State@meta":        vw.Meta(view.PropGET("health_state")),
							"Health@meta":       vw.Meta(view.PropGET("health")),
						},
					},
				},
			}, nil
		})

}
