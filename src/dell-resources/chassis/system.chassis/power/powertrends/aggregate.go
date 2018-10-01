package powertrends

import (
	"context"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func AddAggregate(ctx context.Context, logger log.Logger, v *view.View, member string, ch eh.CommandHandler) map[string]interface{} {
	s := ""
	if member == "" {
	    s = "s"
	}
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          v.GetUUID(),
			Collection:  false,
			ResourceURI: v.GetURI(),
			Type:        "#DellPower.v1_0_0.DellPowerTrend"+s,
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
func getViewFragment(v *view.View, member string) map[string]interface{} {
	properties := map[string]interface{}{
		"@odata.id":      v.GetURI(),
		"Name":           "System Power",
		"MemberId":       "PowerHistogram",
	}

	if member != "" {
		properties["Name"] = "System Power History"
		properties["HistoryAverageWatts@meta"] = v.Meta(view.PropGET("avg_watts_"+member))
		properties["HistoryMaxWatts@meta"] = v.Meta(view.PropGET("max_watts_"+member))
		properties["HistoryMaxWattsTime@meta"] = v.Meta(view.PropGET("max_watts_time_"+member))
		properties["HistoryMinWatts@meta"] = v.Meta(view.PropGET("min_watts_"+member))
		properties["HistoryMinWattsTime@meta"] = v.Meta(view.PropGET("min_watts_time_"+member))
	} else {
		properties["@odata.context"] = "/redfish/v1/$metadata#Power.PowerSystem.Chassis.1/Power/$entity"
		properties["@odata.type"] = "#DellPower.v1_0_0.DellPowerTrends"
		properties["histograms@meta"] = v.Meta(view.PropGET("histograms"))
		properties["histograms@odata.count@meta"] = v.Meta(view.PropGET("histograms_count"))
	}
	return properties
}
