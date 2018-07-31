package sled_chassis

import (
	"context"

	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/view"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func AddAggregate(ctx context.Context, logger log.Logger, v *view.View, ch eh.CommandHandler, eb eh.EventBus) {
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
				"SKU@meta":          v.Meta(view.PropGET("service_tag")),
				"PowerState@meta":   v.Meta(view.PropGET("power_state")),
				"ChassisType@meta":  v.Meta(view.PropGET("chassis_type")),
				"Model@meta":        v.Meta(view.PropGET("model")),
				"Manufacturer@meta": v.Meta(view.PropGET("manufacturer")),
				"SerialNumber@meta": v.Meta(view.PropGET("serial")),

				"Links": map[string]interface{}{
					"ManagedBy@meta":             v.Meta(view.PropGET("managed_by")),
					"ManagedBy@odata.count@meta": v.Meta(view.PropGET("managed_by_count")),
				},

				"Description": v.Meta(view.PropGET("description")),
				"Status": map[string]interface{}{
					"HealthRollup@meta": v.Meta(view.PropGET("health_rollup")), //smil call?
					"State":             "Enabled",                             //hardcoded
					"Health@meta":       v.Meta(view.PropGET("health")),        //smil call?
				},
				"PartNumber@meta": v.Meta(view.PropGET("part_number")),
				"Name@meta":       v.Meta(view.PropGET("name")),
				"Oem": map[string]interface{}{
					"OemChassis": map[string]interface{}{
						"@odata.id": v.GetURI() + "/Attributes",
					},
				},
				"Actions": map[string]interface{}{
					"Oem": map[string]interface{}{
						// TODO: Remove per JIT-66996
						"#DellChassis.v1_0_0#DellChassis.PeripheralMapping": map[string]interface{}{
							"MappingType@Redfish.AllowableValues": []string{
								"Accept",
								"Clear",
							},
							"target": v.GetActionURI("chassis.peripheralmapping"),
						},
						"#Chassis.VirtualReseat": map[string]interface{}{
							"target": v.GetActionURI("sledvirtualreseat"),
						},
						"#DellChassis.v1_0_0.PeripheralMapping": map[string]interface{}{
							"MappingType@Redfish.AllowableValues": []string{
								"Accept",
								"Clear",
							},
							"target": v.GetActionURI("chassis.peripiheralmapping"),
						},
						"#DellChassis.v1_0_0.VirtualReseat": map[string]interface{}{
							"target": v.GetActionURI("chassis.sledvirtualreseat"),
						},
					},
				},
			}})
}
