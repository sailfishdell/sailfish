package attribute_property

import (
	"fmt"

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
	fqdd         []string
	//         group      index      attribute  value
	attributes map[string]map[string]map[string]interface{}
}

func New(options ...interface{}) (*service, error) {
	s := &service{
		Service:    plugins.NewService(),
		attributes: map[string]map[string]map[string]interface{}{},
		fqdd:       []string{},
	}
	s.ApplyOption(plugins.UUID())
	s.ApplyOption(options...)
	s.ApplyOption(plugins.PluginType(domain.PluginType("attribute property: " + fmt.Sprintf("%v", s.GetProperty("id")))))
	return s, nil
}

func BaseResource(b odataInt) Option {
	return func(p *service) error {
		p.baseResource = b
		return nil
	}
}

func WithFQDD(fqdd string) Option {
	return func(p *service) error {
		p.fqdd = append(p.fqdd, fqdd)
		return nil
	}
}

//
// Use this to add an attribute or to update an attribute
//
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
