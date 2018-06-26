package iom_chassis

import (
	"context"

	"github.com/superchalupa/go-redfish/src/eventwaiter"
	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/view"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

type waiter interface {
	Listen(context.Context, func(eh.Event) bool) (*eventwaiter.EventListener, error)
}

func AddAggregate(ctx context.Context, logger log.Logger, v *view.View, ch eh.CommandHandler, eb eh.EventBus, ew waiter) {
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
					// TODO: "ManagedBy@odata.count": 1  (need some infrastructure for this)
					"ManagedBy@meta": v.Meta(view.PropGET("managed_by")),
				},

				"SKU":          "",
				"IndicatorLED": "Lit",
				"Status": map[string]interface{}{
					"HealthRollup": "OK",
					"State":        "Enabled",
					"Health":       "OK",
				},
				"Oem": map[string]interface{}{
					"Dell": map[string]interface{}{
						"ServiceTag@meta":      v.Meta(view.PropGET("service_tag")),
						"InstPowerConsumption": "TEST_VALUE",
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
						"target": v.GetURI() + "/Actions/Chassis.Reset",
					},
					"Oem": map[string]interface{}{
						"#DellChassis.v1_0_0.ResetPeakPowerConsumption": map[string]interface{}{
							"target": v.GetActionURI("iom.resetpeakpowerconsumption"),
						},
						"#DellChassis.v1_0_0.VirtualReseat": map[string]interface{}{
							"target": v.GetActionURI("iom.virtualreseat"),
						},
						"DellChassis.v1_0_0#DellChassis.ResetPeakPowerConsumption": map[string]interface{}{
							"target": v.GetActionURI("iom.resetpeakpowerconsumption"),
						},
						"DellChassis.v1_0_0#DellChassis.VirtualReseat": map[string]interface{}{
							"target": v.GetActionURI("iom.virtualreseat"),
						},
					},
				},
			}})

}
