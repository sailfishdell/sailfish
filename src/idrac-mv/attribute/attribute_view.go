package attribute

import (
	"context"

	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

func (s *service) PropertyGet(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	meta map[string]interface{},
) {
	s.Lock()
	defer s.Unlock()

	res := map[string]interface{}{}
	for group, v := range s.attributes {
		for index, v2 := range v {
			for name, value := range v2 {
				res[group+"."+index+"."+name] = value
			}
		}
	}
	rrp.Value = res
}

func (s *service) AddView(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	ch.HandleCommand(ctx,
		&domain.UpdateRedfishResourceProperties{
			ID: s.baseResource.GetUUID(),
			Properties: map[string]interface{}{
				"Attributes@meta": map[string]interface{}{"GET": map[string]interface{}{"plugin": string(s.PluginType())}},
			},
		})
}
