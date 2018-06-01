package root

import (
	"context"

	"github.com/superchalupa/go-redfish/src/ocp/view"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

const (
	RootPlugin = domain.PluginType("obmc_root")
)

func AddView(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) (v *view.View) {
    // so simple we don't need a model at all here

	v = view.NewView(
		view.WithURI("/redfish/v1"),
	)

	domain.RegisterPlugin(func() domain.Plugin { return v })

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
			}})

    return
}
