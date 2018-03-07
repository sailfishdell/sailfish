package thermal

import (
	"context"

	plugins "github.com/superchalupa/go-redfish/src/ocp"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

const (
	ThermalPlugin = domain.PluginType("thermal")
)

type odataInt interface {
	GetOdataID() string
	GetUUID() eh.UUID
}

type service struct {
	*plugins.Service
	chas odataInt
}

func New(options ...interface{}) (*service, error) {
	p := &service{
		Service: plugins.NewService(plugins.PluginType(ThermalPlugin)),
	}
	p.ApplyOption(plugins.UUID())
	p.ApplyOption(options...)
	return p, nil
}

func InChassis(b odataInt) Option {
	return func(p *service) error {
		p.chas = b
		p.PropertyOnceUnlocked("uri", p.chas.GetOdataID()+"/Thermal")
		return nil
	}
}

func (s *service) AddResource(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	ch.HandleCommand(
		context.Background(),
		&domain.CreateRedfishResource{
			ID:          s.GetUUID(),
			Collection:  false,
			ResourceURI: s.GetOdataID(),
			Type:        "#Thermal.v1_1_0.Thermal",
			Context:     "/redfish/v1/$metadata#Thermal.Thermal",
			Privileges: map[string]interface{}{
				"GET":    []string{"Login"},
				"POST":   []string{}, // cannot create sub objects
				"PUT":    []string{},
				"PATCH":  []string{},
				"DELETE": []string{}, // can't be deleted
			},
			Properties: map[string]interface{}{
				"Id":   "Thermal",
				"Name": "Thermal",
			}})

	ch.HandleCommand(ctx,
		&domain.UpdateRedfishResourceProperties{
			ID: s.chas.GetUUID(),
			Properties: map[string]interface{}{
				"Thermal": map[string]interface{}{"@odata.id": s.GetOdataID()},
			},
		})
}
