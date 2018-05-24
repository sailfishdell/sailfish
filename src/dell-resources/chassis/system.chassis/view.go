package system_chassis

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
				"Id":                s.GetProperty("unique_name").(string),
				"SerialNumber@meta": s.Meta(model.PropGET("serial")),
				"ChassisType@meta":  s.Meta(model.PropGET("chassis_type")),
				"Model@meta":        s.Meta(model.PropGET("model")),
				"Manufacturer@meta": s.Meta(model.PropGET("manufacturer")),
				"PartNumber@meta":   s.Meta(model.PropGET("part_number")),
				"Name@meta":         s.Meta(model.PropGET("name")),
				"AssetTag@meta":     s.Meta(model.PropGET("asset_tag")),
				"Description@meta":  s.Meta(model.PropGET("description")),
				"PowerState@meta":   s.Meta(model.PropGET("power_state")),

				"IndicatorLED": "Lit",
				"SKU":          "PT00033",

				"Links": map[string]interface{}{
					"ManagedBy@meta": s.Meta(model.PropGET("managed_by")),
				},

				"Status": map[string]interface{}{
					"HealthRollup": "OK",
					"State":        "Enabled",
					"Health":       "OK",
				},

				"Power":   map[string]interface{}{"@odata.id": model.GetOdataID(s) + "/Power"},
				"Thermal": map[string]interface{}{"@odata.id": model.GetOdataID(s) + "/Thermal"},
				"Oem": map[string]interface{}{
					"Dell": map[string]interface{}{
						"SubSystemHealth": map[string]interface{}{
							"@odata.id": model.GetOdataID(s) + "/SubSystemHealth",
						},
						"Slots": map[string]interface{}{
							"@odata.id": model.GetOdataID(s) + "/Slots",
						},
						"SlotConfigs": map[string]interface{}{
							"@odata.id": model.GetOdataID(s) + "/SlotConfigs",
						},
						"OemChassis": map[string]interface{}{
							"@odata.id": model.GetOdataID(s) + "/Attributes",
						},
					},
				},

				"Actions": map[string]interface{}{
					"#Chassis.Reset": map[string]interface{}{
						"ResetType@Redfish.AllowableValues": []string{
							"On",
							"ForceOff",
							"GracefulShutdown",
							"GracefulRestart",
						},
						"target": model.GetOdataID(s) + "/Actions/Chassis.Reset",
					},
					"Oem": map[string]interface{}{
						"#MSMConfigBackupURI": map[string]interface{}{
							"target": model.GetOdataID(s) + "/Actions/Oem/MSMConfigBackup",
						},
						"#DellChassis.v1_0_0.MSMConfigBackup": map[string]interface{}{
							"target": model.GetOdataID(s) + "/Actions/Oem/DellChassis.MSMConfigBackup",
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
		model.GetOdataID(s)+"/Actions/Oem/MSMConfigBackup",
		"msm_config_backup",
		s)

	ah.CreateAction(ctx, ch, eb, ew,
		logger,
		model.GetOdataID(s)+"/Actions/Oem/DellChassis.MSMConfigBackup",
		"chassis_msm_config_backup",
		s)
}
