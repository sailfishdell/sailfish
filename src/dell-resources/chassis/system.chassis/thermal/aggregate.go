package thermal

import (
	"context"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func AddAggregate(ctx context.Context, logger log.Logger, v *view.View, ch eh.CommandHandler) {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          v.GetUUID(),
			Collection:  false,
			ResourceURI: v.GetURI(),
			Type:        "#Thermal.v1_0_2.Thermal",
			Context:     "/redfish/v1/$metadata#Thermal.Thermal",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Id":          "Thermal",
				"Name":        "Thermal",
				"Description": "Represents the properties for Temperature and Cooling",

				"Fans@meta":                     v.Meta(view.GETProperty("fan_uris"), view.GETFormatter("expand"), view.GETModel("default")),
				"Fans@odata.count@meta":         v.Meta(view.GETProperty("fan_uris"), view.GETFormatter("count"), view.GETModel("default")),
				"Temperatures@meta":             v.Meta(view.GETProperty("temperature_uris"), view.GETFormatter("expand"), view.GETModel("default")),
				"Temperatures@odata.count@meta": v.Meta(view.GETProperty("temperature_uris"), view.GETFormatter("count"), view.GETModel("default")),
				"Redundancy@meta":               v.Meta(view.GETProperty("redundancy_uris"), view.GETFormatter("expand"), view.GETModel("default")),
				"Redundancy@odata.count@meta":   v.Meta(view.GETProperty("redundancy_uris"), view.GETFormatter("count"), view.GETModel("default")),

				"Oem": map[string]interface{}{
					"EID_674": map[string]interface{}{
						"FansSummary": map[string]interface{}{
							"Status": map[string]interface{}{
								"HealthRollup@meta": v.Meta(view.GETProperty("fan_rollup"), view.GETModel("global_health")),
								"Health@meta":       v.Meta(view.GETProperty("fan_rollup"), view.GETModel("global_health")),
							},
						},
						"TemperaturesSummary": map[string]interface{}{
							"Status": map[string]interface{}{
								"HealthRollup@meta": v.Meta(view.GETProperty("temperature_rollup"), view.GETModel("global_health")),
								"Health@meta":       v.Meta(view.GETProperty("temperature_rollup"), view.GETModel("global_health")),
							},
						},
					},
				},
			}})
}
