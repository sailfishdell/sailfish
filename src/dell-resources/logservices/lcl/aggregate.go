package lcl

import (
	"context"

	"github.com/superchalupa/go-redfish/src/ocp/view"

	"github.com/superchalupa/go-redfish/src/log"
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
			Type:        "#LogServiceCollection.LogServiceCollection",
			Context:     "/redfish/v1/$metadata#LogServiceCollection.LogServiceCollection",
			Privileges: map[string]interface{}{
				"GET":    []string{"ConfigureManager"},
				"POST":   []string{},
				"PUT":    []string{"ConfigureManager"},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{"ConfigureManager"},
			},
			Properties: map[string]interface{}{
				"Description": "LC Logs for this manager",
				"Name":        "Log Entry Collection",
			},
		})

	return
}
