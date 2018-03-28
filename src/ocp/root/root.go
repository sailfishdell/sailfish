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

type service struct {
	*plugins.Service
}

func New(options ...interface{}) (*service, error) {
	s := &service{
		Service: plugins.NewService(plugins.PluginType(RootPlugin)),
	}

	s.ApplyOption(plugins.UUID())
	s.ApplyOption(options...)
	return s, nil
}

func (s *service) AddResource(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:         s.GetUUID(),
			Collection: false,

			ResourceURI: "/redfish/v1",
			Type:        "#ServiceRoot.v1_0_2.ServiceRoot",
			Context:     "/redfish/v1/$metadata#ServiceRoot.ServiceRoot",

			Privileges: map[string]interface{}{
				"GET":   []string{"Unauthenticated"},
				"PATCH": []string{"Unauthenticated", "ConfigureManager"}, // for testing
			},
			Properties: map[string]interface{}{
				"Id":             "RootService",
				"Name":           "Root Service",
				"RedfishVersion": "1.0.2",
				"UUID":           s.GetUUID(),
				"TEST@meta":      s.Meta(plugins.PropGET("test"), plugins.PropPATCH("test")),
			}})
}
