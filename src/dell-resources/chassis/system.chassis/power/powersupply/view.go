package powersupply

import (
	"context"

	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/model"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

func AddView(ctx context.Context, logger log.Logger, s *model.Service, attr *model.Service, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) map[string]interface{} {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          model.GetUUID(s),
			Collection:  false,
			ResourceURI: model.GetOdataID(s),
			Type:        "#Power.v1_0_2.Power",
			Context:     "/redfish/v1/$metadata#Power.PowerSystem.Chassis.1/Power/$entity",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: GetViewFragment(s, attr),
		})

	return GetViewFragment(s, attr)
}

func GetViewFragment(s *model.Service, attr *model.Service) map[string]interface{} {
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
