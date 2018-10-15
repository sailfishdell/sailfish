package storage_collection

import (
	"context"

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func AddAggregate(ctx context.Context, logger log.Logger, v *view.View, ch eh.CommandHandler) {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          v.GetUUID(),
			Collection:  true,
			ResourceURI: v.GetURI(),
			Type:        "#StorageCollection.StorageCollection",
			Context:     "/redfish/v1/$metadata#StorageCollection.StorageCollection",
			Privileges: map[string]interface{}{
				"GET": []string{"Login"},
			},
			Properties: map[string]interface{}{
				"Description@meta": v.Meta(view.PropGET("description")), //Done
				"Name@meta":        v.Meta(view.PropGET("name")),        //Done
			}})
}
