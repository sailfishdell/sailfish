package attribute_property

import (
	"fmt"

	"github.com/superchalupa/go-redfish/src/ocp/model"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

type odataInt interface {
	GetProperty(string) interface{}
}

type service struct {
	*model.Service
	baseResource odataInt
	fqdd         []string
	//         group      index      attribute  value
	attributes map[string]map[string]map[string]interface{}
}

func New(options ...interface{}) (*service, error) {
	s := &service{
		Service:    model.NewService(),
		attributes: map[string]map[string]map[string]interface{}{},
		fqdd:       []string{},
	}
	s.ApplyOption(model.UUID())
	s.ApplyOption(options...)
	pluginType := domain.PluginType("attribute property: " + fmt.Sprintf("%v", s.GetProperty("id")))
	s.ApplyOption(model.PluginType(pluginType))
	return s, nil
}

func BaseResource(b odataInt) Option {
	return func(p *service) error {
		p.baseResource = b
		return nil
	}
}

func WithFQDD(fqdd string) Option {
	return func(s *service) error {
		s.fqdd = append(s.fqdd, fqdd)
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
