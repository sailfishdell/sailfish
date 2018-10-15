package storage_instance

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
			Collection:  false,
			ResourceURI: v.GetURI(),
			Type:        "#Storage.v1_4_0.Storage",
			Context:     "/redfish/v1/$metadata#Storage.Storage",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{"ConfigureManager"},
				"PUT":    []string{},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{"ConfigureManager"},
			},
			Properties: map[string]interface{}{
				"Description@meta":        v.Meta(view.PropGET("description")), //Done
				"Drives@meta":             v.Meta(view.PropGET("drives")),      //Should we add redundancy uris here?
				"Drives@odata.count@meta": v.Meta(view.PropGET("drives_count")),
				"Id@meta":                 v.Meta(view.PropGET("unique_name")),
				"Links": map[string]interface{}{
					"Enclosures": []map[string]interface{}{
						//Need to add Enclosures array
					},
					"Enclosures@odata.count": v.Meta(view.PropGET("count")),
				},
				"Name@meta": v.Meta(view.PropGET("name")), //Done
				"Oem": map[string]interface{}{ //Done
					"Dell": map[string]interface{}{
						"DellController": map[string]interface{}{
							"@odata.context":            "/redfish/v1/$metadata#DellController.DellController",
							"@odata.id":                 "/redfish/v1/Dell/Systems/System.Emdedded.1/Storage/DellController/$entity",
							"@odata.type":               "#DellController.v1_0_0.DellController",
							"CacheSizeInMB":             v.Meta(view.PropGET("cache_size")),
							"CachecadeCapability":       v.Meta(view.PropGET("cache_capability")),
							"ControllerFirmwareVersion": v.Meta(view.PropGET("controller_firmware_version")),
							"DeviceCardSlotType":        v.Meta(view.PropGET("device_card_slot_type")),
							"DriverVersion":             v.Meta(view.PropGET("driver_version")),
							"EncryptionCapability":      v.Meta(view.PropGET("encryption_capability")),
							"EncryptionMode":            v.Meta(view.PropGET("encryption_mode")),
							"PCISlot":                   v.Meta(view.PropGET("pci_slot")),
							"PatrolReadState":           v.Meta(view.PropGET("patrol_read_state")),
							"RollupStatus":              v.Meta(view.PropGET("rollup_status")),
							"SecurityStatus":            v.Meta(view.PropGET("security_status")),
							"SlicedVDCapability":        v.Meta(view.PropGET("sliced_vd_capabiltiy")),
						},
					},
				},
				"Status": map[string]interface{}{
					"HealthRollup@meta": v.Meta(view.PropGET("obj_status")),
					"State@meta":        v.Meta(view.PropGET("state")),
					"Health@meta":       v.Meta(view.PropGET("obj_status")),
				},
				"StorageControllers": map[string]interface{}{ //Done
					"@odata.type":          "#Storage.v1_4_0.StorageController",
					"@odata.context":       "/redfish/v1/Systems/System.Embedded.1/StorageControllers/$entity",
					"@odata.id":            v.GetURI(),
					"FirmwareVersion@meta": v.Meta(view.PropGET("firmware_version")),
					"Identifiers":          map[string]interface{}{
						//need make this an array.
					},
					"Links":             map[string]interface{}{},
					"Manufacturer@meta": v.Meta(view.PropGET("manufacturer")),
					"MemberId@meta":     v.Meta(view.PropGET("member_id")),
					"Model@meta":        v.Meta(view.PropGET("model")),
					"Name@meta":         v.Meta(view.PropGET("name")),
					"SpeedGbps@meta":    v.Meta(view.PropGET("speed")),
					"Status": map[string]interface{}{
						"HealthRollup@meta": v.Meta(view.GETProperty(""), view.GETModel("")),
						"State@meta":        v.Meta(view.PropGET("health_state")),
						"Health@meta":       v.Meta(view.PropGET("health")),
					},
					"SupportedControllerProtocols": map[string]interface{}{
						//need to make this an array
					},
					"SupportedDeviceProtocols": map[string]interface{}{
						//need to make this an array
					},
				},
				"StorageControllers@odata.count@meta": v.Meta(view.PropGET("storage_controller_count")),
				"Volumes@meta":                        v.Meta(view.PropGET("volumes")),
			}})
}
