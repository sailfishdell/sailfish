package plugins

import (
	"context"
	"errors"
	"reflect"
	"sync"

	domain "github.com/superchalupa/go-redfish/redfishresource"
)

type Option func(interface{}) error

type Service struct {
	sync.Mutex
	pluginType domain.PluginType
	properties map[string]interface{}
}

func NewService(options ...Option) *Service {
	s := &Service{
		properties: map[string]interface{}{},
	}
	s.ApplyOption(options...)
	return s
}

func (c *Service) ApplyOption(options ...Option) error {
	for _, o := range options {
		err := o(c)
		if err != nil {
			return err
		}
	}
	return nil
}

func PluginType(pt domain.PluginType) Option {
	return func(s interface{}) error {
		s.(*Service).pluginType = pt
		return nil
	}
}

func UpdateProperty(p string, v interface{}) Option {
	return func(s interface{}) error {
		s.(*Service).properties[p] = v
		return nil
	}
}

func (s *Service) PluginType() domain.PluginType { return s.pluginType }

func (s *Service) RefreshProperty(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	method string,
	meta map[string]interface{},
	body interface{},
) {
	s.Lock()
	defer s.Unlock()

	property, ok := meta["property"].(string)
	if ok {
		if p, ok := s.properties[property]; ok {
			rrp.Value = p
			return
		}
	}
}

func RefreshProperty(
	ctx context.Context,
	s interface{},
	rrp *domain.RedfishResourceProperty,
	meta map[string]interface{},
) error {
	property, ok := meta["property"].(string)
	if ok {
		v := reflect.ValueOf(s)
		for i := 0; i < v.NumField(); i++ {
			// Get the field, returns https://golang.org/pkg/reflect/#StructField
			tag := v.Type().Field(i).Tag.Get("property")
			if tag == property {
				rrp.Value = v.Field(i).Interface()
				return nil
			}
		}
	}
	return errors.New("Couldn't find property.")
}
