package storage_enclosure

import (
	"context"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func AddAggregate(ctx context.Context, logger log.Logger, v *view.View, ch eh.CommandHandler) {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          v.GetUUID(),
			ResourceURI: v.GetURI(),
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
				"AssetTag@meta":    v.Meta(view.PropGET("asset_tag")), //Done
				"ChassisType@meta": v.Meta(view.PropGET("chassis_type")),
				"Description@meta": v.Meta(view.PropGET("description")),
				"Id@meta":          v.Meta(view.PropGET("unique_name")),
				"Links":            map[string]interface{}{
					//Need to add links detail.
				},
				"Links@meta":             v.Meta(view.GETProperty("links_uris"), view.GETFormatter("expand"), view.GETModel("default")),
				"Links@odata.count@meta": v.Meta(view.GETProperty("links_uris"), view.GETFormatter("count"), view.GETModel("default")),
				"Manufacturer@meta":      v.Meta(view.PropGET("manufacturer")), //Done
				"Model@meta":             v.Meta(view.PropGET("model")),        //Done
				"Name@meta":              v.Meta(view.PropGET("name")),         //Done
				"Oem": map[string]interface{}{ //Done
					"Dell": map[string]interface{}{
						"DellEnclosure@meta": v.Meta(view.GETProperty("enclosure_uri"), view.GETFormatter("expandone"), view.GETModel("default")),

						/*
							"DellEnclosure": map[string]interface{}{
								"@odata.context": "/redfish/v1/$metadata#DellEnclosure.DellEnclosure",
								"@odata.id":      "/redfish/v1/Dell/Chassis/System.Embedded.1/DellEnclosure/$Entity",
								"@odata.type":    "#DellEnclosure.v1_0_0.DellEnclosure",
								"Connector":      v.Meta(view.PropGET("connector")),
								"ServiceTag":     v.Meta(view.PropGET("service_tag")),
								"SlotCount":      v.Meta(view.PropGET("slot_count")),
								"Version":        v.Meta(view.PropGET("version")),
								"WiredOrder":     v.Meta(view.PropGET("wired_order")),
							},
						*/

					},
				},
				"PCIeDevices":                  map[string]interface{}{},
				"PCIeDevices@odata.count@meta": v.Meta(view.PropGET("pcie_devices_count")),
				"PartNumber@meta":              v.Meta(view.PropGET("part_number")),
				"PowerState@meta":              v.Meta(view.PropGET("power_state")),
				"SKU@meta":                     v.Meta(view.PropGET("sku")),
				"SerialNumber@meta":            v.Meta(view.PropGET("serial")),
				"Status": map[string]interface{}{
					"HealthRollup@meta": v.Meta(view.PropGET("health")),
					"State@meta":        v.Meta(view.PropGET("health_state")),
					"Health@meta":       v.Meta(view.PropGET("health")),
				},
			}})
}
