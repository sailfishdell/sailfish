package cmc_chassis

import (
	"context"

	"github.com/superchalupa/go-redfish/src/dell-resources/ar_mapper"
	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/model"
	"github.com/superchalupa/go-redfish/src/ocp/view"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

func AddView(ctx context.Context, logger log.Logger, s *model.Model, c *ar_mapper.ARMappingController, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) *view.View {

	v := view.New(
		view.WithURI("/redfish/v1/Chassis/"+s.GetProperty("unique_name").(string)),
		view.WithModel("default", s),
		view.WithController("ar_mapper", c),
	)

	domain.RegisterPlugin(func() domain.Plugin { return v })

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
				"Id":                s.GetProperty("unique_name").(string),
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

	return v
}
