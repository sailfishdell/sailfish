package slotconfig

import (
	"context"

	"github.com/superchalupa/sailfish/src/ocp/view"

	"github.com/superchalupa/sailfish/src/log"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

func AddAggregate(ctx context.Context, logger log.Logger, v *view.View, rootID eh.UUID, ch eh.CommandHandler, eb eh.EventBus) {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          v.GetUUID(),
			Collection:  true,
			ResourceURI: v.GetURI(),
			Type:        "#DellSlotConfigsCollection.DellSlotConfigsCollection",
			Context:     "/redfish/v1/$metadata#DellSlotConfigsCollection.DellSlotConfigsCollection",
			Privileges: map[string]interface{}{
				"GET":    []string{"ConfigureManager"},
				"POST":   []string{},
				"PUT":    []string{},
				"PATCH":  []string{},
				"DELETE": []string{},
			},
			Properties: map[string]interface{}{
				"Name": "DellSlotConfigsCollection",
			},
		})

	return
}
