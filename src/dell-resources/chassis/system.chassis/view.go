package system_chassis

import (
	"context"

	"github.com/superchalupa/go-redfish/src/dell-resources/ar_mapper"
	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/model"
	"github.com/superchalupa/go-redfish/src/ocp/view"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
	ah "github.com/superchalupa/go-redfish/src/actionhandler"
)

func AddView(ctx context.Context, logger log.Logger, s *model.Model, c *ar_mapper.ARMappingController, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) *view.View {

	uri := "/redfish/v1/Chassis/" + s.GetProperty("unique_name").(string)

	v := view.NewView(
		view.WithURI(uri),
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
				"SerialNumber@meta": v.Meta(view.PropGET("serial")),
				"ChassisType@meta":  v.Meta(view.PropGET("chassis_type")),
				"Model@meta":        v.Meta(view.PropGET("model")),
				"Manufacturer@meta": v.Meta(view.PropGET("manufacturer")),
				"PartNumber@meta":   v.Meta(view.PropGET("part_number")),
				"Name@meta":         v.Meta(view.PropGET("name")),
				"AssetTag@meta":     v.Meta(view.PropGET("asset_tag")),
				"Description@meta":  v.Meta(view.PropGET("description")),
				"PowerState@meta":   v.Meta(view.PropGET("power_state")),

				"IndicatorLED": "Lit",
				"SKU":          "PT00033",

				"Links": map[string]interface{}{
					"ManagedBy@meta": v.Meta(view.PropGET("managed_by")),
				},

				"Status": map[string]interface{}{
					"HealthRollup": "OK",
					"State":        "Enabled",
					"Health":       "OK",
				},

				"Power":   map[string]interface{}{"@odata.id": v.GetURI() + "/Power"},
				"Thermal": map[string]interface{}{"@odata.id": v.GetURI() + "/Thermal"},
				"Oem": map[string]interface{}{
					"Dell": map[string]interface{}{
						"SubSystemHealth": map[string]interface{}{
							"@odata.id": v.GetURI() + "/SubSystemHealth",
						},
						"Slots": map[string]interface{}{
							"@odata.id": v.GetURI() + "/Slots",
						},
						"SlotConfigs": map[string]interface{}{
							"@odata.id": v.GetURI() + "/SlotConfigs",
						},
						"OemChassis": map[string]interface{}{
							"@odata.id": v.GetURI() + "/Attributes",
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
						"target": v.GetURI() + "/Actions/Chassis.Reset",
					},
					"Oem": map[string]interface{}{
						"#MSMConfigBackupURI": map[string]interface{}{
							"target": v.GetURI() + "/Actions/Oem/MSMConfigBackup",
						},
						"#DellChassis.v1_0_0.MSMConfigBackup": map[string]interface{}{
							"target": v.GetURI() + "/Actions/Oem/DellChassis.MSMConfigBackup",
						},
					},
				},
			}})

	ah.CreateAction(ctx, ch, eb, ew,
		logger,
		v.GetURI()+"/Actions/Chassis.Reset",
		"chassis.reset",
		s)

	ah.CreateAction(ctx, ch, eb, ew,
		logger,
		v.GetURI()+"/Actions/Oem/MSMConfigBackup",
		"msm_config_backup",
		s)

	ah.CreateAction(ctx, ch, eb, ew,
		logger,
		v.GetURI()+"/Actions/Oem/DellChassis.MSMConfigBackup",
		"chassis_msm_config_backup",
		s)

	return v
}
