package storage_volume_collection

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
			ID: v.GetUUID(),
			// Collection:  true, // FIXME: collection code removed, move this to awesome mapper collection instead
			ResourceURI: v.GetURI(),
			Type:        "#VolumeCollection.VolumeCollection",
			Context:     "/redfish/v1/$metadata#VolumeCollection.VolumeCollection",
			Privileges: map[string]interface{}{
				"GET":  []string{"Login"},
				"POST": []string{"ConfigureManager"},
			},
			Properties: map[string]interface{}{
				"Description@meta": v.Meta(view.PropGET("description")), //Done
				"Name@meta":        v.Meta(view.PropGET("name")),        //Done
			}})
}
