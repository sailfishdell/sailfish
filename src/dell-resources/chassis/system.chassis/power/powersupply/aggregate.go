package powersupply

import (
	"context"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

// So... this class is set up in a somewhat interesting way to support having
// PSU.Slot.N objects both as PowerSupplies/PSU.Slot.N as well as in the main
// Power object.

func AddAggregate(ctx context.Context, logger log.Logger, v *view.View, ch eh.CommandHandler) {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          v.GetUUID(),
			ResourceURI: v.GetURI(),
			Type:        "#Power.v1_0_2.PowerSupply",
			Context:     "/redfish/v1/$metadata#Power.PowerSystem.Chassis.1/Power/$entity",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Name@meta":               v.Meta(view.PropGET("name")),
				"MemberId@meta":           v.Meta(view.PropGET("unique_name")),
				"PowerCapacityWatts@meta": v.Meta(view.PropGET("capacity_watts")),
				"LineInputVoltage@meta":   v.Meta(view.PropGET("line_input_voltage")),
				"FirmwareVersion@meta":    v.Meta(view.PropGET("firmware_version")),

				"Status": map[string]interface{}{
					"HealthRollup@meta": v.Meta(view.PropGET("obj_status")),
					"State@meta":        v.Meta(view.PropGET("state")),
					"Health@meta":       v.Meta(view.PropGET("obj_status")),
				},

				"Oem": map[string]interface{}{
					"Dell": map[string]interface{}{
						"@odata.type":       "#DellPower.v1_0_0.DellPowerSupply",
						"ComponentID@meta":  v.Meta(view.PropGET("component_id")),
						"InputCurrent@meta": v.Meta(view.PropGET("input_current")),
						"Attributes@meta":   v.Meta(view.GETProperty("attributes"), view.GETFormatter("attributeFormatter"), view.GETModel("default"), view.PropPATCH("attributes", "ar_dump")),
					},
				},
				// this should be a link using getformatter
				"Redundancy":             []interface{}{},
				"Redundancy@odata.count": 0,
			},
		})
}
