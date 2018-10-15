package storage_drive

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
			Type:        "#Drive.v1_3_0.Drive",
			Context:     "/redfish/v1/$metadata#Drive.Drive",
			Privileges: map[string]interface{}{
				"GET":  []string{"Login"},
				"POST": []string{"ConfigureManager"},
			},
			Properties: map[string]interface{}{
				"Assembly": map[string]interface{}{
					"@odata.id": "/redfish/v1/Chassis/System.Embedded.1/Assembly",
				},
				//Acion needs to be added.
				"BlockSizeBytes":    v.Meta(view.PropGET("block_size_bytes")),
				"CapableSpeedGbs":   v.Meta(view.PropGET("cache_capability")),
				"CapacityBytes":     v.Meta(view.PropGET("controller_firmware_version")),
				"Description":       v.Meta(view.PropGET("device_card_slot_type")),
				"EncryptionAbility": v.Meta(view.PropGET("driver_version")),
				"EncryptionStatus":  v.Meta(view.PropGET("encryption_capability")),
				"FailurePredicted":  v.Meta(view.PropGET("encryption_mode")),
				"HotspareType":      v.Meta(view.PropGET("pci_slot")),
				"Id@meta":           v.Meta(view.PropGET("$entity")),
				"Links": map[string]interface{}{
					"Enclosures": []map[string]interface{}{
						//Need to add Enclosures array
					},
					"Enclosures@odata.count": v.Meta(view.PropGET("count")),
				},
				"Location":                map[string]interface{}{},
				"Manufacturer@meta":       v.Meta(view.PropGET("manufacturer")),     //Done
				"MediaType@meta":          v.Meta(view.PropGET("media_type")),       //Done
				"Model@meta":              v.Meta(view.PropGET("model")),            //Done
				"Name@meta":               v.Meta(view.PropGET("name")),             //Done
				"NegotiatedSpeedGbs@meta": v.Meta(view.PropGET("negotiated_speed")), //Done
				"Oem": map[string]interface{}{ //Done
					"Dell": map[string]interface{}{
						"DellPhysicalDisk": map[string]interface{}{
							"@odata.context":         "/redfish/v1/$metadata#DellPhysicalDisk.DellPhysicalDisk",
							"@odata.id":              "/redfish/v1/Dell/Systems/System.Embedded.1/Storage/Drives/DellPhysicalDisk/$entity",
							"@odata.type":            "#DellPhysicalDisk.v1_0_0.DellPhysicalDisk",
							"Connector":              v.Meta(view.PropGET("connector")),
							"DriveFormFactor":        v.Meta(view.PropGET("drive_formfactor")),
							"FreeSizeInBytes":        v.Meta(view.PropGET("free_size")),
							"ManufacturingDay":       v.Meta(view.PropGET("manufacturing_day")),
							"ManufacturingWeek":      v.Meta(view.PropGET("manufacturing_week")),
							"ManufacturingYear":      v.Meta(view.PropGET("manufacturing_year")),
							"PPID":                   v.Meta(view.PropGET("ppid")),
							"PredictiveFailureState": v.Meta(view.PropGET("predictive_failure_state")),
							"RaidStatus":             v.Meta(view.PropGET("raid_status")),
							"SASAddress":             v.Meta(view.PropGET("sas_address")),
							"Slot":                   v.Meta(view.PropGET("slot")),
							"UsedSizeInBytes":        v.Meta(view.PropGET("used_size")),
						},
					},
				},

				"PartNumber":                    v.Meta(view.PropGET("part_number")),
				"PredictedMediaLifeLeftPercent": v.Meta(view.PropGET("predicted_media_life_left_percent")),
				"Protocol":                      v.Meta(view.PropGET("protocol")),
				"Revision":                      v.Meta(view.PropGET("revision")),
				"RotationSpeedRPM":              v.Meta(view.PropGET("rotation_speed")),
				"SerialNumber":                  v.Meta(view.PropGET("serial_number")),
				"Status": map[string]interface{}{
					"HealthRollup@meta": v.Meta(view.PropGET("obj_status")),
					"State@meta":        v.Meta(view.PropGET("state")),
					"Health@meta":       v.Meta(view.PropGET("obj_status")),
				},
			}})
}
