package subsystemhealth

import (
	"context"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func AddAggregate(ctx context.Context, logger log.Logger, v *view.View, ch eh.CommandHandler, eb eh.EventBus, healths map[string]string) {
  properties := map[string]interface{}{}
  //properties["SubSystems@meta"] = v.Meta(view.PropGET("subsystems"))
  /*for subsystem, health := range healths {
    properties[subsystem] = map[string]interface{}{
      "Status": map[string]interface{}{
        "HealthRollup@meta": v.Meta(view.GETProperty("health"), view.GETModel("health")), //TODO: fix me
      },
    }
  }*/

  ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          v.GetUUID(),
			ResourceURI: v.GetURI(),
			Type:        "DellSubSystemHealth.v1_0_0.DellSubSystemHealth",
			Context:     "/redfish/v1/$metadata#ChassisCollection.ChassisCollection/Members/$entity",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: properties,
		},
	)
}
