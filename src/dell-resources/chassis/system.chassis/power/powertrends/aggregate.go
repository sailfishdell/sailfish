package powertrends

import (
	"context"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func AddAggregate(ctx context.Context, logger log.Logger, v *view.View, member bool, ch eh.CommandHandler) map[string]interface{} {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          v.GetUUID(),
			Collection:  false,
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
			Properties: getViewFragment(v, member),
		})

	return getViewFragment(v, member)
}

// this view fragment can be attached elsewhere in the tree
func getViewFragment(v *view.View, member bool) map[string]interface{} {
	properties := map[string]interface{}{
		"@odata.context": "/redfish/v1/$metadata#Power.PowerSystem.Chassis.1/Power/$entity",
		"@odata.id":      v.GetURI(),
		"Name":           "System Power",
		"MemberId":       "PowerHistogram",
	}

	if member == true {
		properties["Name"] = "System Power History"
		properties["@odata.type"] = "#DellPower.v1_0_0.DellPowerTrend"
		properties["HistoryAverageWatts@meta"] = v.Meta(view.PropGET("history_average_watts"))  //TODO
		properties["HistoryMaxWatts@meta"] = v.Meta(view.PropGET("history_max_watts"))          //TODO
		properties["HistoryMaxWattsTime@meta"] = v.Meta(view.PropGET("history_max_watts_time")) //TODO
		properties["HistoryMinWatts@meta"] = v.Meta(view.PropGET("history_min_watts"))          //TODO
		properties["HistoryMinWattsTime@meta"] = v.Meta(view.PropGET("history_min_watts_time")) //TODO
	} else {
		properties["Name"] = "System Power"
		properties["@odata.type"] = "#DellPower.v1_0_0.DellPowerTrends"
		properties["histograms@meta"] = v.Meta(view.PropGET("histograms"))
		properties["histograms@odata.count@meta"] = v.Meta(view.PropGET("histograms_count"))
	}

	return properties
}
