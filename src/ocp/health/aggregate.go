package health

import (
	"context"

	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func GetHealthFragment(v *view.View, modelName string, input map[string]interface{}) map[string]interface{} {
	status := map[string]interface{}{}
	healthmodel := v.GetModel(modelName)
	if healthmodel != nil {
		if _, ok := healthmodel.GetPropertyOk("health_rollup"); ok {
			status["HealthRollup@meta"] = v.Meta(view.GETProperty("health_rollup"), view.GETModel(modelName))
		}
		if _, ok := healthmodel.GetPropertyOk("state"); ok {
			status["State@meta"] = v.Meta(view.GETProperty("state"), view.GETModel(modelName))
		}
		if _, ok := healthmodel.GetPropertyOk("health"); ok {
			status["Health@meta"] = v.Meta(view.GETProperty("health"), view.GETModel(modelName))
		}
	}
	input["Status"] = status
	return input
}

func EnhanceAggregate(ctx context.Context, v *view.View, ch eh.CommandHandler) {
	properties := GetHealthFragment(v, "health", map[string]interface{}{})
	ch.HandleCommand(ctx,
		&domain.UpdateRedfishResourceProperties{
			ID:         v.GetUUID(),
			Properties: properties,
		})
}
