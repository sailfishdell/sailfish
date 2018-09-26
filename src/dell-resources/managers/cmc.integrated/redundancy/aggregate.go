package redundancy

import (
	"context"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func AddAggregate(ctx context.Context, logger log.Logger, v *view.View, ch eh.CommandHandler) map[string]interface{} {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          v.GetUUID(),
			Collection:  false,
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
			Properties: getViewFragment(v),
		})

	return getViewFragment(v)
}

func getViewFragment(v *view.View) map[string]interface{} {
	properties := map[string]interface{}{
                "@odata.type": "#Redundancy.v1_0_2.Redundancy",
                "Name": "ManagerRedundancy",
                "@odata.id@meta":            v.Meta(view.PropGET("unique_id")), //"/redfish/v1/Managers/CMC.Integrated.1#Redundancy",
                "@odata.context":            "/redfish/v1/$metadata#Redundancy.Redundancy",
                "Mode@meta":                 v.Meta(view.PropGET("redundancy_mode")),
                "MinNumNeeded@meta":         v.Meta(view.PropGET("redundancy_min")),
                "MaxNumSupported@meta":      v.Meta(view.PropGET("redundancy_max")),
		"Status": map[string]interface{}{
			"Health@meta": v.Meta(view.PropGET("health")),
			"HeathRollup@meta": v.Meta(view.PropGET("health")),
			"State@meta": v.Meta(view.PropGET("health_state")),
		},
	}

	properties["RedundancySet@meta"] = v.Meta(view.PropGET("redundancy_set"))
	properties["RedundancySet@odata.count@meta"] = v.Meta(view.PropGET("redundancy_set_count"))

	return properties
}
