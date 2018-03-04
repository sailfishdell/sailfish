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
	sync.RWMutex
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
func (s *Service) GetUUID() eh.UUID { s.RLock(); defer s.RUnlock(); return s.properties["id"].(eh.UUID) }
func (s *Service) GetOdataID() string {
	s.RLock()
	defer s.RUnlock()
	return s.properties["uri"].(string)
}
func (s *Service) PluginType() domain.PluginType {
	s.RLock()
	defer s.RUnlock()
	return s.properties["plugin_type"].(domain.PluginType)
}

func (s *Service) PluginTypeUnlocked() domain.PluginType {
	return s.properties["plugin_type"].(domain.PluginType)
}

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

func (s *Service) GetProperty(p string) interface{} {
	s.RLock()
	defer s.RUnlock()
	return s.properties[p]
}
func (s *Service) GetPropertyOk(p string) (ret interface{}, ok bool) {
	s.RLock()
	defer s.RUnlock()
	ret, ok = s.properties[p]
	return
}
func (s *Service) UpdateProperty(p string, i interface{}) {
	s.Lock()
	defer s.Unlock()
	s.properties[p] = i
}

func (s *Service) GetPropertyUnlocked(p string) interface{} { return s.properties[p] }
func (s *Service) GetPropertyOkUnlocked(p string) (ret interface{}, ok bool) {
	ret, ok = s.properties[p]
	return
}
func (s *Service) UpdatePropertyUnlocked(p string, i interface{}) { s.properties[p] = i }

// MustProperty is equivalent to GetProperty with the exception that it will
// panic if the property has not already been set. Use for mandatory properties.
// This is the unlocked version of this function.
func (s *Service) MustPropertyUnlocked(name string) (ret interface{}) {
	ret, ok := s.properties[name]
	if ok {
		return
	}
	panic("Required property is not set: " + name)
}

// MustProperty is equivalent to GetProperty with the exception that it will
// panic if the property has not already been set. Use for mandatory properties.
func (s *Service) MustProperty(name string) interface{} {
	s.RLock()
	defer s.RUnlock()
	return s.MustPropertyUnlocked(name)
}

func (s *Service) PropertyOnce(p string, v interface{}) {
	s.Lock()
	defer s.Unlock()
	if _, ok := s.properties[p]; ok {
		panic("Property " + p + " can only be set once")
	}
	s.properties[p] = v
}

func (s *Service) PropertyOnceUnlocked(p string, v interface{}) {
	if _, ok := s.properties[p]; ok {
		panic("Property " + p + " can only be set once")
	}
	s.properties[p] = v
}

//
//  Options: these are construction-time functional options that can be passed
//  to the constructor, or after construction, you can pass them with
//  ApplyOptions
//

// UpdateProperty is a functional optioin to set  and option at construction time or update the value after using ApplyOption.
// Service is locked for Options in ApplyOption
func UpdateProperty(p string, v interface{}) Option {
	return func(s *Service) error {
		s.properties[p] = v
		return nil
	}
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
	s.MustPropertyUnlocked(name)
	return map[string]interface{}{"GET": map[string]interface{}{"plugin": string(s.PluginTypeUnlocked()), "property": name}}
}
