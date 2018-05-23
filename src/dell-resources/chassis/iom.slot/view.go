package iom_chassis

import (
	"context"

	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/model"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
	ah "github.com/superchalupa/go-redfish/src/actionhandler"
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
				"Id": s.GetProperty("unique_name").(string),

				"AssetTag@meta":     s.Meta(model.PropGET("asset_tag")),
				"Description@meta":  s.Meta(model.PropGET("description")),
				"PowerState@meta":   s.Meta(model.PropGET("power_state")),
				"SerialNumber@meta": s.Meta(model.PropGET("serial")),
				"PartNumber@meta":   s.Meta(model.PropGET("part_number")),
				"ChassisType@meta":  s.Meta(model.PropGET("chassis_type")),
				"Model@meta":        s.Meta(model.PropGET("model")),
				"Name@meta":         s.Meta(model.PropGET("name")),
				"Manufacturer@meta": s.Meta(model.PropGET("manufacturer")),

				// TODO: "ManagedBy@odata.count": 1  (need some infrastructure for this)
				"ManagedBy@meta": s.Meta(model.PropGET("managed_by")),

				"SKU":          "",
				"IndicatorLED": "Lit",
				"Status": map[string]interface{}{
					"HealthRollup": "OK",
					"State":        "Enabled",
					"Health":       "OK",
				},
				"Oem": map[string]interface{}{
					"Dell": map[string]interface{}{
						"ServiceTag@meta":      s.Meta(model.PropGET("service_tag")),
						"InstPowerConsumption": 24,
						"OemChassis": map[string]interface{}{
							"@odata.id": model.GetOdataID(s) + "/Attributes",
						},
						"OemIOMConfiguration": map[string]interface{}{
							"@odata.id": model.GetOdataID(s) + "/IOMConfiguration",
						},
					},
				},

				"Actions": map[string]interface{}{
					"#Chassis.Reset": map[string]interface{}{
						"ResetType@Redfish.AllowableValues": []string{
							"On",
							"GracefulShutdown",
							"GracefulRestart",
						},
						"target": model.GetOdataID(s) + "/Actions/Chassis.Reset",
					},
					"Oem": map[string]interface{}{
						"DellChassis.v1_0_0#DellChassis.ResetPeakPowerConsumption": map[string]interface{}{
							"target": model.GetOdataID(s) + "/Actions/Oem/DellChassis.ResetPeakPowerConsumption",
						},
						"#DellChassis.v1_0_0.VirtualReseat": map[string]interface{}{
							"target": model.GetOdataID(s) + "/Actions/Oem/DellChassis.VirtualReseat",
						},
					},
				},
			}})

	ah.CreateAction(ctx, ch, eb, ew,
		logger,
		model.GetOdataID(s)+"/Actions/Chassis.Reset",
		"chassis.reset",
		s)

	ah.CreateAction(ctx, ch, eb, ew,
		logger,
		model.GetOdataID(s)+"/Actions/Oem/DellChassis.ResetPeakPowerConsumption",
		"chassis.reset_peak_power_consumption",
		s)

	ah.CreateAction(ctx, ch, eb, ew,
		logger,
		model.GetOdataID(s)+"/Actions/Oem/DellChassis.VirtualReseat",
		"chassis.virtual_reseat",
		s)
}
