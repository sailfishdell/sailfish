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
				"Id@meta":           v.Meta(view.PropGET("unique_name")),
				"AssetTag@meta":     v.Meta(view.PropGET("asset_tag"), view.PropPATCH("asset_tag", "ar_mapper")),
				"SerialNumber@meta": v.Meta(view.PropGET("serial")),
				"PartNumber@meta":   v.Meta(view.PropGET("part_number")),
				"ChassisType@meta":  v.Meta(view.PropGET("chassis_type")),
				"Model@meta":        v.Meta(view.PropGET("model")),
				"Manufacturer@meta": v.Meta(view.PropGET("manufacturer")),
				"Name@meta":         v.Meta(view.PropGET("name")),
				"SKU":               "",
				"Description":       "An instance of Chassis Management Controller",
				"Links":             map[string]interface{}{},
				"Status": map[string]interface{}{
					"HealthRollup": "OK",
					"State":        "StandbySpare",
					"Health":       "OK",
				},
				"IndicatorLED": "Lit",
				"Oem": map[string]interface{}{
					"OemChassis": map[string]interface{}{
						"@odata.id": v.GetURI() + "/Attributes",
					},
				},
			}})
}
