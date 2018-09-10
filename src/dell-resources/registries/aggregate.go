package registries

import (
	"context"

	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/view"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func AddAggregate(ctx context.Context, logger log.Logger, v *view.View, rootID eh.UUID, ch eh.CommandHandler, eb eh.EventBus) {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          v.GetUUID(),
			Collection:  true,
			ResourceURI: v.GetURI(),
			Type:        "#MessageRegistryFileCollection.MessageRegistryFileCollection",
			Context:     "/redfish/v1/$metadata#MessageRegistryFileCollection.MessageRegistryFileCollection",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{},
				"PATCH":  []string{},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Description": "Registry Repository",
				"Name":        "Registry File Collection",
			}})

	ch.HandleCommand(ctx,
		&domain.UpdateRedfishResourceProperties{
			ID: rootID,
			Properties: map[string]interface{}{
				"Registries": map[string]interface{}{"@odata.id": "/redfish/v1/Registries"},
			},
		})

}
