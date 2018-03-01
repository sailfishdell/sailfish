package plugins

import (
	"context"
	"errors"
	"reflect"
	"sync"

	domain "github.com/superchalupa/go-redfish/redfishresource"
	eh "github.com/looplab/eventhorizon"
)

type Option func(*Service) error

type Service struct {
	sync.Mutex
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

// runtime panic if upper layers dont set properties for id/uri
func (s *Service) GetUUID() eh.UUID   { return s.properties["id"].(eh.UUID) }
func (s *Service) GetOdataID() string { return s.properties["uri"].(string) }
func (s *Service) PluginType() domain.PluginType { return s.properties["plugin_type"].(domain.PluginType) }

// GetProperty will runtime panic if property doesn't exist
func (s *Service) GetProperty(p string) interface{} { return s.properties[p] }
// TODO: HasProperty() if needed (?)

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

    s.RefreshProperty_unlocked(ctx, agg, rrp, method, meta, body)
}

func (s *Service) RefreshProperty_unlocked(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	method string,
	meta map[string]interface{},
	body interface{},
) {
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

func UpdateProperty(p string, v interface{}) Option {
	return func(s *Service) error {
		s.properties[p] = v
		return nil
	}
}

func PropertyOnce(p string, v interface{}) Option {
	return func(s *Service) error {
		if _, ok := s.properties[p]; ok {
			panic("Property " + p + " can only be set once")
		}
		s.properties[p] = v
        return nil
	}
}

func URI(uri string) Option {
    return UpdateProperty("uri", uri)
}

func UUID() Option {
    return UpdateProperty("id", eh.NewUUID())
}

func PluginType(pt domain.PluginType) Option {
    return UpdateProperty("plugin_type", pt)
}
