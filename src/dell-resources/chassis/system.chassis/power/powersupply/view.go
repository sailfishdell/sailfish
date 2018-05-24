package powersupply

import (
	"context"

	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/model"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

func GetViewFragment(ctx context.Context, logger log.Logger, s *model.Service, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) map[string]interface{} {

	logger.Error("Dump debug info", "model", s)

	return map[string]interface{}{
		"@odata.type":             "#Power.v1_0_2.PowerSupply",
		"@odata.context":          "/redfish/v1/$metadata#Power.PowerSystem.Chassis.1/Power/$entity",
		"@odata.id":               model.GetOdataID(s),
		"Name@meta":               s.Meta(model.PropGET("name")),
		"MemberId@meta":           s.Meta(model.PropGET("unique_id")),
		"PowerCapacityWatts@meta": s.Meta(model.PropGET("capacity_watts")),
		"Status": map[string]interface{}{
			"HealthRollup": "OK",
			"State":        "Enabled",
			"Health":       "OK",
		},
	}
}
