package powersupply

import (
	"context"

	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/model"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

// So... this class is set up in a somewhat interesting way to support having
// PSU.Slot.N objects both as PowerSupplies/PSU.Slot.N as well as in the main
// Power object.

func AddView(ctx context.Context, logger log.Logger, s *model.Service, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) map[string]interface{} {
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

func GetViewFragment(s *model.Service) map[string]interface{} {
	return map[string]interface{}{
		"@odata.id":           model.GetOdataID(s),
		"@odata.type":         "#DellPower.v1_0_0.DellPowerTrend",
		"@odata.context":      "/redfish/v1/$metadata#Power.PowerSystem.Chassis.1/Power/$entity",
		"Name@meta":           s.Meta(model.PropGET("name")),
		"MemberId":            "PowerHistogram",
		"HistoryAverageWatts": s.Meta(model.PropGET("history_average_watts")),
		"HistoryMinWatts":     s.Meta(model.PropGET("history_min_watts")),
		"HistoryMinWattsTime": s.Meta(model.PropGET("history_min_watts_time")),
		"HistoryMaxWatts":     s.Meta(model.PropGET("history_max_watts")),
		"HistoryMaxWattsTime": s.Meta(model.PropGET("history_max_watts_time")),
	}
}
