package iom_chassis

import (
	"context"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func AddAggregate(ctx context.Context, logger log.Logger, v *view.View, ch eh.CommandHandler, eb eh.EventBus) {
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
				"Id@meta":           v.Meta(view.PropGET("unique_name")),
				"AssetTag@meta":     v.Meta(view.PropGET("asset_tag")),
				"Description@meta":  v.Meta(view.PropGET("description")),
				"PowerState@meta":   v.Meta(view.PropGET("power_state")),
				"SerialNumber@meta": v.Meta(view.PropGET("serial")),
				"PartNumber@meta":   v.Meta(view.PropGET("part_number")),
				"ChassisType@meta":  v.Meta(view.PropGET("chassis_type")),
				"Model@meta":        v.Meta(view.PropGET("model")),
				"Name@meta":         v.Meta(view.PropGET("name")),
				"Manufacturer@meta": v.Meta(view.PropGET("manufacturer")),

				"Links": map[string]interface{}{
					"ManagedBy@meta":             v.Meta(view.GETProperty("managed_by"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
					"ManagedBy@odata.count@meta": v.Meta(view.GETProperty("managed_by"), view.GETFormatter("count"), view.GETModel("default")),
				},

				"SKU@meta":          v.Meta(view.PropGET("service_tag")),
				"IndicatorLED@meta": v.Meta(view.PropGET("indicator_led")),
				"Status": map[string]interface{}{
					"HealthRollup@meta": v.Meta(view.PropGET("health")),
					"State":             "Enabled", //hard coded
					"Health@meta":       v.Meta(view.PropGET("health")),
				},
				"Oem": map[string]interface{}{
					"Dell": map[string]interface{}{
						"ServiceTag@meta":           v.Meta(view.PropGET("service_tag")),
						"InstPowerConsumption@meta": v.Meta(view.PropGET("Instantaneous_Power")),
						"OemChassis": map[string]interface{}{
							"@odata.id": v.GetURI() + "/Attributes",
						},
						"OemIOMConfiguration": map[string]interface{}{
							"@odata.id": v.GetURI() + "/IOMConfiguration",
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
						"target": v.GetActionURI("iom.chassis.reset"),
					},
					"Oem": map[string]interface{}{
						"#DellChassis.v1_0_0.ResetPeakPowerConsumption": map[string]interface{}{
							"target": v.GetActionURI("iom.resetpeakpowerconsumption"),
						},
						"#DellChassis.v1_0_0.VirtualReseat": map[string]interface{}{
							"target": v.GetActionURI("iom.virtualreseat"),
						},
						// TODO: Remove per JIT-66996
						"DellChassis.v1_0_0#DellChassis.ResetPeakPowerConsumption": map[string]interface{}{
							"target": v.GetActionURI("iom.resetpeakpowerconsumption"),
						},
						// TODO: Remove per JIT-66996
						"DellChassis.v1_0_0#DellChassis.VirtualReseat": map[string]interface{}{
							"target": v.GetActionURI("iom.virtualreseat"),
						},
					},
				},
			}})

}
