package power

import (
	"context"

	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/view"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

// TODO: current odatalite stack has this as part of output, but that seems completely wrong:
//       "PowerTrends@odata.count": 7,
// because powertrends is not an array. (7 = # of keys in powertrends, probably not intentional)

func AddAggregate(ctx context.Context, logger log.Logger, v *view.View, ch eh.CommandHandler) {

	properties := map[string]interface{}{
		"Id":          "Power",
		"Description": "Power",
		"Name":        "Power",
		"PowerSupplies@odata.count@meta": v.Meta(view.PropGET("power_supply_views_count")),
		"PowerSupplies@meta":             v.Meta(view.PropGET("power_supply_views")),
		"PowerControl@odata.count@meta":  v.Meta(view.PropGET("power_control_views_count")),
		"PowerControl@meta":              v.Meta(view.PropGET("power_control_views")),
		"Oem": map[string]interface{}{
			"OemPower": map[string]interface{}{
				"PowerTrends@meta": v.Meta(view.PropGET("power_trend_views")),
				"PowerTrends@odata.count@meta": v.Meta(view.PropGET("power_trend_count")), // gets # of elements in power trend array instead of # of keys in first power trend array element
			},
			"EID_674": map[string]interface{}{
				"PowerSuppliesSummary": map[string]interface{}{
					"Status": map[string]interface{}{
						"HealthRollup@meta": v.Meta(view.GETProperty("psu_rollup"), view.GETModel("global_health")),
					},
				},
			},
		},
	}
	//properties["Oem"].(map[string]interface{})["OemPower"].(map[string]interface{})["PowerTrends@odata.count"] = len(properties["Oem"].(map[string]interface{})["OemPower"].(map[string]interface{})["PowerTrends"].(map[string]interface{}))

	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          v.GetUUID(),
			Collection:  false,
			ResourceURI: v.GetURI(),
			Type:        "#Power.v1_0_2.Power",
			Context:     "/redfish/v1/$metadata#Power.PowerSystem.Chassis.1/Power/$entity",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: properties,
		})
}
