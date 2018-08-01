package root

import (
	"context"

	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

type view interface {
	GetUUID() eh.UUID
	GetURI() string
}

func AddAggregate(ctx context.Context, v view, ch eh.CommandHandler, eb eh.EventBus) {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:         v.GetUUID(),
			Collection: false,

			ResourceURI: v.GetURI(),
			Type:        "#ServiceRoot.v1_0_2.ServiceRoot",
			Context:     "/redfish/v1/$metadata#ServiceRoot.ServiceRoot",

			Privileges: map[string]interface{}{
				"GET": []string{"Unauthenticated"},
			},
			Properties: map[string]interface{}{
				"Id":             "RootService",
				"Name":           "Root Service",
				"RedfishVersion": "1.0.2",
                "@odata.etag":    "abc123",
			}})

	return
}
