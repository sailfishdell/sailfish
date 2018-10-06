package powertrends

import (
	"context"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func AddTrendsAggregate(ctx context.Context, logger log.Logger, v *view.View, ch eh.CommandHandler) {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          v.GetUUID(),
			ResourceURI: v.GetURI(),
			Type:        "#DellPower.v1_0_0.DellPowerTrends",
			Context:     "/redfish/v1/$metadata#Power.PowerSystem.Chassis.1/Power/$entity",
			Collection:  false,
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{},
				"PATCH":  []string{},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Name":                        "System Power",
				"MemberId":                    "PowerHistogram",
				"histograms@meta":             v.Meta(view.GETProperty("trend_histogram_uris"), view.GETFormatter("expand"), view.GETModel("default")),
				"histograms@odata.count@meta": v.Meta(view.GETProperty("trend_histogram_uris"), view.GETFormatter("count"), view.GETModel("default")),
			},
		})
}

func AddHistogramAggregate(ctx context.Context, logger log.Logger, v *view.View, ch eh.CommandHandler) {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          v.GetUUID(),
			ResourceURI: v.GetURI(),
			Type:        "#DellPower.v1_0_0.DellPowerTrend",
			Context:     "/redfish/v1/$metadata#Power.PowerSystem.Chassis.1/Power/$entity",
			Collection:  false,
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{},
				"PATCH":  []string{},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Name":                     "System Power History",
				"MemberId":                 "PowerHistogram",
				"HistoryMaxWattsTime@meta": v.Meta(view.GETProperty("max_watts_time"), view.GETModel("default")),
				"HistoryMaxWatts@meta":     v.Meta(view.GETProperty("max_watts"), view.GETModel("default")),
				"HistoryMinWattsTime@meta": v.Meta(view.GETProperty("min_watts_time"), view.GETModel("default")),
				"HistoryMinWatts@meta":     v.Meta(view.GETProperty("min_watts"), view.GETModel("default")),
				"HistoryAverageWatts@meta": v.Meta(view.GETProperty("average_watts"), view.GETModel("default")),
			},
		})
}
