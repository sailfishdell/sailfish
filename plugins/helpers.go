package plugins

import (
	"context"
	"errors"
	"reflect"
	"sync"

	eh "github.com/looplab/eventhorizon"
	domain "github.com/superchalupa/go-redfish/redfishresource"
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

func (s *Service) ApplyOption(options ...Option) error {
    s.Lock()
    defer s.Unlock()
	for _, o := range options {
		err := o(s)
		if err != nil {
			return err
		}
	}
	return nil
}

// runtime panic if upper layers dont set properties for id/uri
func (s *Service) GetUUID() eh.UUID   { return s.properties["id"].(eh.UUID) }
func (s *Service) GetOdataID() string { return s.properties["uri"].(string) }
func (s *Service) PluginType() domain.PluginType {
	return s.properties["plugin_type"].(domain.PluginType)
}

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

// Service is locked for Options in ApplyOption
func UpdateProperty(p string, v interface{}) Option {
	return func(s *Service) error {
		s.properties[p] = v
		return nil
	}
}

func (s *Service) MustProperty_unlocked(name string) (ret interface{}) {
	ret, ok := s.properties[name]
    if ok { return }
	panic("Required property is not set: " + name)
}


func (s *Service) MustProperty(name string) interface{} {
    s.Lock()
    defer s.Unlock()
    return s.MustProperty_unlocked(name)
}

// Service is locked for Options in ApplyOption
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

func (s *Service) MetaReadOnlyProperty(name string) map[string]interface{} {
    s.Lock()
    defer s.Unlock()
	// should panic if property not there
	s.MustProperty_unlocked(name)
	return map[string]interface{}{"GET": map[string]interface{}{"plugin": string(s.PluginType()), "property": name}}
}
