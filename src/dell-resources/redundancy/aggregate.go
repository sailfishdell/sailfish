package redundancy

import (
	"context"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func init() {
	// RegisterAggregate("redundancy", fn() {} )
}

func AddAggregate(ctx context.Context, logger log.Logger, v *view.View, ch eh.CommandHandler) {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          v.GetUUID(),
			ResourceURI: v.GetURI(),
			Type:        "#Redundancy.v1_0_2.Redundancy",
			Context:     "/redfish/v1/$metadata#Redundancy.Redundancy",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Name":                           "ManagerRedundancy",
				"Mode@meta":                      v.Meta(view.PropGET("redundancy_mode")),
				"MinNumNeeded@meta":              v.Meta(view.PropGET("redundancy_min")),
				"MaxNumSupported@meta":           v.Meta(view.PropGET("redundancy_max")),
				"RedundancySet@meta":             v.Meta(view.GETProperty("redundancy_set"), view.GETFormatter("formatOdataList"), view.GETModel("default")),
				"RedundancySet@odata.count@meta": v.Meta(view.GETProperty("redundancy_set"), view.GETFormatter("count"), view.GETModel("default")),
				"Status": map[string]interface{}{
					"Health@meta":       v.Meta(view.PropGET("health")),
					"HealthRollup@meta": v.Meta(view.PropGET("health")),
					"State@meta":        v.Meta(view.PropGET("health_state")),
				},
			},
		})
}
