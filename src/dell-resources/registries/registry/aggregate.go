package registry

import (
	"context"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func AddAggregate(ctx context.Context, logger log.Logger, v *view.View, ch eh.CommandHandler, eb eh.EventBus) {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          v.GetUUID(),
			Collection:  false,
			ResourceURI: v.GetURI(),
			Type:        "#MessageRegistryFile.v1_0_2.MessageRegistryFile",
			Context:     "/redfish/v1/$metadata#MessageRegistryFile.MessageRegistryFile",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{},
				"PATCH":  []string{},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Description@meta":           v.Meta(view.GETProperty("registry_description"), view.GETModel("default")),
				"Id@meta":                    v.Meta(view.GETProperty("registry_id"), view.GETModel("default")),
				"Languages@meta":             v.Meta(view.GETProperty("languages"), view.GETModel("default")),
				"Languages@odata.count@meta": v.Meta(view.GETProperty("languages"), view.GETFormatter("count"), view.GETModel("default")),
				"Location@meta":              v.Meta(view.GETProperty("location"), view.GETModel("default")),
				"Location@odata.count@meta":  v.Meta(view.GETProperty("location"), view.GETFormatter("count"), view.GETModel("default")),
				"Name@meta":                  v.Meta(view.GETProperty("registry_name"), view.GETModel("default")),
				"Registry@meta":              v.Meta(view.GETProperty("registry_type"), view.GETModel("default")),
			}})
}
