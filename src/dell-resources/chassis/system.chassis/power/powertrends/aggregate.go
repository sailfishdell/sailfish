package powertrends

import (
	"context"
	"sync"

	"github.com/spf13/viper"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func RegisterAggregate(s *testaggregate.Service) {
	s.RegisterAggregateFunction("power_trends",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{

					ResourceURI: vw.GetURI(),
					Type:        "#DellPower.v1_0_0.DellPowerTrends",
					Context:     "/redfish/v1/$metadata#Power.PowerSystem.Chassis.1/Power/$entity",
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
						"histograms@meta":             vw.Meta(view.GETProperty("trend_histogram_uris"), view.GETFormatter("expand"), view.GETModel("default")),
						"histograms@odata.count@meta": vw.Meta(view.GETProperty("trend_histogram_uris"), view.GETFormatter("count"), view.GETModel("default")),
					}},
			}, nil
		})

	s.RegisterAggregateFunction("trend_histogram",
		func(ctx context.Context, subLogger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, extra interface{}, params map[string]interface{}) ([]eh.Command, error) {
			return []eh.Command{
				&domain.CreateRedfishResource{
					ResourceURI: vw.GetURI(),
					Type:        "#DellPower.v1_0_0.DellPowerTrend",
					Context:     "/redfish/v1/$metadata#Power.PowerSystem.Chassis.1/Power/$entity",
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
						"HistoryMaxWattsTime@meta": vw.Meta(view.GETProperty("max_watts_time"), view.GETModel("default")),
						"HistoryMaxWatts@meta":     vw.Meta(view.GETProperty("max_watts"), view.GETModel("default")),
						"HistoryMinWattsTime@meta": vw.Meta(view.GETProperty("min_watts_time"), view.GETModel("default")),
						"HistoryMinWatts@meta":     vw.Meta(view.GETProperty("min_watts"), view.GETModel("default")),
						"HistoryAverageWatts@meta": vw.Meta(view.GETProperty("average_watts"), view.GETModel("default")),
					}},
			}, nil
		})
}
