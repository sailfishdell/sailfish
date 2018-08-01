package cmc_integrated

import (
	"context"

	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/view"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func AddAggregate(ctx context.Context, logger log.Logger, v *view.View, ch eh.CommandHandler) {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          v.GetUUID(),
			Collection:  false,
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
				"AssetTag@meta":     v.Meta(view.PropGET("asset_tag"), view.PropPATCH("asset_tag", "ar_mapper")), //hardcoded null in odatalite
				"SerialNumber@meta": v.Meta(view.PropGET("serial")),                                              //uses sys.chas.1 ar value
				"PartNumber@meta":   v.Meta(view.PropGET("part_number")),                                         //uses sys.chas.1 ar value
				"ChassisType@meta":  v.Meta(view.PropGET("chassis_type")),
				"Model@meta":        v.Meta(view.PropGET("model")),
				"Manufacturer@meta": v.Meta(view.PropGET("manufacturer")),
				"Name@meta":         v.Meta(view.PropGET("name")),
				"SKU@meta":          v.Meta(view.PropGET("sku"), view.PropPATCH("sku", "ar_mapper")), //hardcoded null in odatalite
				"Description@meta":  v.Meta(view.PropGET("description")),
				"Links":             map[string]interface{}{},
				"Status": map[string]interface{}{
					"HealthRollup@meta": v.Meta(view.PropGET("health_rollup")), //smil call?
					"State@meta":        v.Meta(view.PropGET("health_state")),
					"Health@meta":       v.Meta(view.PropGET("health")), //smil call?
				},
				"IndicatorLED@meta": v.Meta(view.PropGET("indicator_led")), //uses sys.chas.1 ar value
				"Oem": map[string]interface{}{
					"OemChassis": map[string]interface{}{
						"@odata.id": v.GetURI() + "/Attributes",
					},
				},
			}})
}
