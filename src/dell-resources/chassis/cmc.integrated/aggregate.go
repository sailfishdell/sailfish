package cmc_integrated

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
			Type:        "#Chassis.v1_0_2.Chassis",
			Context:     "/redfish/v1/$metadata#ChassisCollection.ChassisCollection/Members/$entity",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"@odata.etag@meta":  v.Meta(view.GETProperty("etag"), view.GETModel("etag")),
				"Id@meta":           v.Meta(view.PropGET("unique_name")),
				"AssetTag":          nil,
				"SerialNumber@meta": v.Meta(view.PropGET("serial")),      //uses sys.chas.1 ar value
				"PartNumber@meta":   v.Meta(view.PropGET("part_number")), //uses sys.chas.1 ar value
				"ChassisType@meta":  v.Meta(view.PropGET("chassis_type")),
				"Model@meta":        v.Meta(view.PropGET("model")),
				"Manufacturer@meta": v.Meta(view.PropGET("manufacturer")),
				"Name@meta":         v.Meta(view.PropGET("name")),
				"SKU":               nil,
				"Description@meta":  v.Meta(view.PropGET("description")),
				"Links":             map[string]interface{}{},
				"Status": map[string]interface{}{
					"HealthRollup@meta": v.Meta(view.PropGET("health")),
					"State@meta":        v.Meta(view.PropGET("health_state")),
					"Health@meta":       v.Meta(view.PropGET("health")),
				},
				"IndicatorLED": "Blinking", // static.  MSM does a patch operation and reads from attributes
				"Oem": map[string]interface{}{
					"OemChassis": map[string]interface{}{
						"@odata.id": v.GetURI() + "/Attributes",
					},
				},
			}})
}
