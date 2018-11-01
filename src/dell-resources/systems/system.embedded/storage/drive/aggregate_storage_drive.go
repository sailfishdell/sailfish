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
				"BlockSizeBytes@meta":    v.Meta(view.PropGET("block_size_bytes")),
				"CapableSpeedGbs@meta":   v.Meta(view.PropGET("capable_speed")),
				"CapacityBytes@meta":     v.Meta(view.PropGET("capacity")),
				"Description@meta":       v.Meta(view.PropGET("description")),
				"EncryptionAbility@meta": v.Meta(view.PropGET("encryption_ability")),
				"EncryptionStatus@meta":  v.Meta(view.PropGET("encryption_status")),
				"FailurePredicted@meta":  v.Meta(view.PropGET("failure_predicted")),
				"HotspareType@meta":      v.Meta(view.PropGET("hotspare_type")),
				"Id@meta@meta":           v.Meta(view.PropGET("unique_name")),
				"Links": map[string]interface{}{
					"Enclosures": []map[string]interface{}{
						//Need to add Enclosures array
					},
					"Enclosures@odata.count@meta": v.Meta(view.PropGET("count")),
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
							"Connector@meta":              v.Meta(view.PropGET("connector")),
							"DriveFormFactor@meta":        v.Meta(view.PropGET("drive_formfactor")),
							"FreeSizeInBytes@meta":        v.Meta(view.PropGET("free_size")),
							"ManufacturingDay@meta":       v.Meta(view.PropGET("manufacturing_day")),
							"ManufacturingWeek@meta":      v.Meta(view.PropGET("manufacturing_week")),
							"ManufacturingYear@meta":      v.Meta(view.PropGET("manufacturing_year")),
							"PPID@meta":                   v.Meta(view.PropGET("ppid")),
							"PredictiveFailureState@meta": v.Meta(view.PropGET("predictive_failure_state")),
							"RaidStatus@meta":             v.Meta(view.PropGET("raid_status")),
							"SASAddress@meta":             v.Meta(view.PropGET("sas_address")),
							"Slot@meta":                   v.Meta(view.PropGET("slot")),
							"UsedSizeInBytes@meta":        v.Meta(view.PropGET("used_size")),
						},
					},
				},

				"PartNumber@meta":                    v.Meta(view.PropGET("part_number")),
				"PredictedMediaLifeLeftPercent@meta": v.Meta(view.PropGET("predicted_media_life_left_percent")),
				"Protocol@meta":                      v.Meta(view.PropGET("protocol")),
				"Revision@meta":                      v.Meta(view.PropGET("revision")),
				"RotationSpeedRPM@meta":              v.Meta(view.PropGET("rotation_speed")),
				"SerialNumber@meta":                  v.Meta(view.PropGET("serial_number")),
				"Status": map[string]interface{}{
					"HealthRollup@meta": v.Meta(view.PropGET("obj_status")),
					"State@meta":        v.Meta(view.PropGET("state")),
					"Health@meta":       v.Meta(view.PropGET("obj_status")),
				},
			}})
}
