package root

import (
	"context"

	plugins "github.com/superchalupa/go-redfish/src/ocp"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

const (
	RootPlugin = domain.PluginType("obmc_root")
)

type Service struct {
	*plugins.Service
}

func New(options ...interface{}) (*Service, error) {
	s := &Service{
		Service: plugins.NewService(plugins.PluginType(RootPlugin)),
	}

	s.ApplyOption(plugins.UUID())
	s.ApplyOption(options...)
	return s, nil
}

func (s *Service) AddResource(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:         plugins.GetUUID(s),
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
				"UUID":           plugins.GetUUID(s),
			}})
}
