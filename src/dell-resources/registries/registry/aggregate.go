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
				"Id@meta":                    v.Meta(view.GETProperty("id"), view.GETModel("default")),
				"Name@meta":                  v.Meta(view.GETProperty("name"), view.GETModel("default")),
				"Description@meta":           v.Meta(view.GETProperty("description"), view.GETModel("default")),
				"Registry@meta":              v.Meta(view.GETProperty("type"), view.GETModel("default")),
				"Languages@meta":             v.Meta(view.GETProperty("languages"), view.GETModel("default")),
				"Languages@odata.count@meta": v.Meta(view.GETProperty("languages"), view.GETFormatter("count"), view.GETModel("default")),
				"Location@meta":              v.Meta(view.GETProperty("location"), view.GETModel("default")),
				"Location@odata.count@meta":  v.Meta(view.GETProperty("location"), view.GETFormatter("count"), view.GETModel("default")),
			}})
}
