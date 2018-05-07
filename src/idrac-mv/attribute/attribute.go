package attribute

import (
	"context"
	"fmt"

	"github.com/superchalupa/go-redfish/src/log"
	plugins "github.com/superchalupa/go-redfish/src/ocp"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

const (
	AttributeUpdated eh.EventType = "AttributeUpdated"
)

func init() {
	eh.RegisterEventData(AttributeUpdated, func() eh.EventData { return &AttributeUpdatedData{} })
}

type AttributeUpdatedData struct {
	FQDD  string
	Group string
	Index string
	Name  string
	Value interface{}
}

type odataInt interface {
	GetOdataID() string
	GetUUID() eh.UUID
}

type service struct {
	*plugins.Service
	baseResource odataInt
	fqdd         string
	//          group      index      attribute   value
	attributes map[string]map[string]map[string]interface{}
}

func New(options ...interface{}) (*service, error) {
	p := &service{
		// TODO: fix
		Service:    plugins.NewService(plugins.PluginType(domain.PluginType("TODO:FIXME:unique-per-instance-thingy"))),
		attributes: map[string]map[string]map[string]interface{}{},
	}
	p.ApplyOption(options...)
	return p, nil
}

func InResource(b odataInt, fqdd string) Option {
	return func(p *service) error {
		p.baseResource = b
		p.fqdd = fqdd
		return nil
	}
}

func WithAttribute(group, gindex, name string, value interface{}) Option {
	return func(s *service) error {
		groupMap, ok := s.attributes[group]
		if !ok {
			groupMap = map[string]map[string]interface{}{}
			s.attributes[group] = groupMap
		}

		index, ok := groupMap[gindex]
		if !ok {
			index = map[string]interface{}{}
			groupMap[gindex] = index
		}

		index[name] = value

		return nil
	}
}

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

func (s *service) AddResource(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	ch.HandleCommand(ctx,
		&domain.UpdateRedfishResourceProperties{
			ID: s.baseResource.GetUUID(),
			Properties: map[string]interface{}{
				"Attributes@meta": map[string]interface{}{"GET": map[string]interface{}{"plugin": string(s.PluginType())}},
			},
		})

	// stream processor for action events
	sp, err := plugins.NewEventStreamProcessor(ctx, ew, plugins.CustomFilter(SelectAttributeUpdate(s.fqdd)))
	if err != nil {
		log.MustLogger("idrac_mv").Error("Failed to create event stream processor", "err", err)
		return
	}
	sp.RunForever(func(event eh.Event) {
		log.MustLogger("idrac_mv").Info("Got action event", "event", event)
		if data, ok := event.Data().(*AttributeUpdatedData); ok {
			s.ApplyOption(WithAttribute(data.Group, data.Index, data.Name, data.Value))
		} else {
			log.MustLogger("idrac_mv").Warn("Should never happen: got an invalid event in the event handler")
		}
	})
}

func SelectAttributeUpdate(fqdd string) func(eh.Event) bool {
	return func(event eh.Event) bool {
		log.MustLogger("idrac_mv").Debug("Checking event", "event", event)
		if event.EventType() != AttributeUpdated {
			log.MustLogger("idrac_mv").Debug("no match: type")
			return false
		}
		if data, ok := event.Data().(*AttributeUpdatedData); ok {
			if data.FQDD == fqdd {
				log.MustLogger("idrac_mv").Debug("FQDD MATCH")
				return true
			}
			log.MustLogger("idrac_mv").Debug("FQDD FAIL")
			return false
		}
		log.MustLogger("idrac_mv").Debug("TYPE ASSERT FAIL!", "data", fmt.Sprintf("%#v", event.Data()))
		return false
	}
}
