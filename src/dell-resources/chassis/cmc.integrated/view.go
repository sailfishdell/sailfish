package cmc_chassis

import (
	"context"

	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/model"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

func AddView(ctx context.Context, logger log.Logger, s *model.Service, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          model.GetUUID(s),
			Collection:  false,
			ResourceURI: model.GetOdataID(s),
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
				"Id":                s.GetProperty("unique_name").(string),
				"AssetTag@meta":     s.Meta(model.PropGET("asset_tag")),
				"SerialNumber@meta": s.Meta(model.PropGET("serial")),
				"PartNumber@meta":   s.Meta(model.PropGET("part_number")),
				"ChassisType@meta":  s.Meta(model.PropGET("chassis_type")),
				"Model@meta":        s.Meta(model.PropGET("model")),
				"Manufacturer@meta": s.Meta(model.PropGET("manufacturer")),
				"Name@meta":         s.Meta(model.PropGET("name")),
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
						"@odata.id": model.GetOdataID(s) + "/Attributes",
					},
				},
			}})
}
