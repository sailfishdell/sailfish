package powersupply

import (
	"context"

	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/model"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

func GetViewFragment(ctx context.Context, logger log.Logger, s *model.Service, attr *model.Service, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) map[string]interface{} {
	return map[string]interface{}{
		"@odata.type":             "#Power.v1_0_2.PowerSupply",
		"@odata.context":          "/redfish/v1/$metadata#Power.PowerSystem.Chassis.1/Power/$entity",
		"@odata.id":               model.GetOdataID(s),
		"Name@meta":               s.Meta(model.PropGET("name")),
		"MemberId@meta":           s.Meta(model.PropGET("unique_id")),
		"PowerCapacityWatts@meta": s.Meta(model.PropGET("capacity_watts")),
		"LineInputVoltage@meta":   s.Meta(model.PropGET("line_input_voltage")),
		"FirmwareVersion@meta":    s.Meta(model.PropGET("firmware_version")),

		"Status": map[string]interface{}{
			"HealthRollup": "OK",
			"State":        "Enabled",
			"Health":       "OK",
		},

		"Oem": map[string]interface{}{
			"Dell": map[string]interface{}{
				"@odata.type":       "#DellPower.v1_0_0.DellPowerSupply",
				"ComponentID@meta":  s.Meta(model.PropGET("component_id")),
				"InputCurrent@meta": s.Meta(model.PropGET("input_current")),
				"Attributes@meta":   map[string]interface{}{"GET": map[string]interface{}{"plugin": string(attr.PluginType())}},
			},
		},
	}
}
