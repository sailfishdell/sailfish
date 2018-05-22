package root

import (
	"context"

	"github.com/superchalupa/go-redfish/src/ocp/model"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

const (
	RootPlugin = domain.PluginType("obmc_root")
)

type Service = model.Model

func New(options ...model.Option) (*model.Model, error) {
	s := model.NewModel(model.PluginType(RootPlugin))

	s.ApplyOption(model.UUID())
	s.ApplyOption(options...)
	return s, nil
}

func AddView(ctx context.Context, s *model.Model, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:         model.GetUUID(s),
			Collection: false,

			ResourceURI: "/redfish/v1",
			Type:        "#ServiceRoot.v1_0_2.ServiceRoot",
			Context:     "/redfish/v1/$metadata#ServiceRoot.ServiceRoot",

			Privileges: map[string]interface{}{
				"GET": []string{"Unauthenticated"},
			},
			Properties: map[string]interface{}{
				"Id":             "RootService",
				"Name":           "Root Service",
				"RedfishVersion": "1.0.2",
				"UUID":           model.GetUUID(s),
			}})
}
