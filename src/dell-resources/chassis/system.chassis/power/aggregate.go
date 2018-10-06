package power

import (
	"context"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

// TODO: current odatalite stack has this as part of output, but that seems completely wrong:
//       "PowerTrends@odata.count": 7,
// because powertrends is not an array. (7 = # of keys in powertrends, probably not intentional)

func AddAggregate(ctx context.Context, logger log.Logger, v *view.View, ch eh.CommandHandler) {
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
			Properties: map[string]interface{}{
				"Id":          "Power",
				"Description": "Power",
				"Name":        "Power",

				"PowerSupplies@meta":             v.Meta(view.GETProperty("power_supply_uris"), view.GETFormatter("expand"), view.GETModel("default")),
				"PowerSupplies@odata.count@meta": v.Meta(view.GETProperty("power_supply_uris"), view.GETFormatter("count"), view.GETModel("default")),
				"PowerControl@meta":              v.Meta(view.GETProperty("power_control_uris"), view.GETFormatter("expand"), view.GETModel("default")),
				"PowerControl@odata.count@meta":  v.Meta(view.GETProperty("power_control_uris"), view.GETFormatter("count"), view.GETModel("default")),
				"Oem": map[string]interface{}{
					"OemPower": map[string]interface{}{
						"PowerTrends@meta":        v.Meta(view.GETProperty("power_trends_uri"), view.GETFormatter("expandone"), view.GETModel("default")),
						"PowerTrends@odata.count": 7, // TODO: Fix this, it's wrong... this shoulndt even be here
					},
					"EID_674": map[string]interface{}{
						"PowerSuppliesSummary": map[string]interface{}{
							"Status": map[string]interface{}{
								"HealthRollup@meta": v.Meta(view.GETProperty("psu_rollup"), view.GETModel("global_health")),
							},
						},
					},
				},
			},
		})
}
