package storage_controller

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
			Type:        "#Storage.v1_4_0.StorageController",
			Context:     "/redfish/v1/$metadata#Storage.Storage",
			Privileges: map[string]interface{}{
				"GET": []string{"Login"},
			},
			Properties: map[string]interface{}{
				"Assembly": map[string]interface{}{
					"@odata.id": "/redfish/v1/Chassis/System.Embedded.1/Assembly",
				},
				"FirmwareVersion@meta": v.Meta(view.PropGET("firmware_version")),
				"Identifiers":          map[string]interface{}{
					//need to make this an array.
				},
				"Links":             map[string]interface{}{},
				"Manufacturer@meta": v.Meta(view.PropGET("manufacturer")),
				"MemberId@meta":     v.Meta(view.PropGET("member_id")),
				"Model@meta":        v.Meta(view.PropGET("model")),
				"Name@meta":         v.Meta(view.PropGET("name")),
				"SpeedGbps@meta":    v.Meta(view.PropGET("speed")),
				"Status": map[string]interface{}{
					"HealthRollup@meta": v.Meta(view.PropGET("obj_status")),
					"State@meta":        v.Meta(view.PropGET("state")),
					"Health@meta":       v.Meta(view.PropGET("obj_status")),
				},
				"SupportedControllerProtocols": map[string]interface{}{
					//need to make this an array
				},
				"SupportedDeviceProtocols": map[string]interface{}{
					//need to make this an array
				},
			}})
}
