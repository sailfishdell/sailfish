package attribute

import (
	plugins "github.com/superchalupa/go-redfish/src/ocp"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

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

