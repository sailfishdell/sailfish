package plugins

import (
	"context"
	"sync"

	eh "github.com/looplab/eventhorizon"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
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
func (s *Service) GetUUID() eh.UUID {
	s.RLock()
	defer s.RUnlock()
	return s.properties["id"].(eh.UUID)
}

func (s *Service) GetOdataIDUnlocked() string { return s.properties["uri"].(string) }
func (s *Service) GetOdataID() string {
	s.RLock()
	defer s.RUnlock()
	return s.properties["uri"].(string)
}
func (s *Service) PluginTypeUnlocked() domain.PluginType {
	return s.properties["plugin_type"].(domain.PluginType)
}
func (s *Service) PluginType() domain.PluginType {
	s.RLock()
	defer s.RUnlock()
	return s.properties["plugin_type"].(domain.PluginType)
}

// already locked at aggregate level when we get here
func (s *Service) PropertyGet(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	method string,
	meta map[string]interface{},
) {
	property, ok := meta["property"].(string)
	if ok {
		if p, ok := s.properties[property]; ok {
			rrp.Value = p
			return
		}
	}
}

// already locked at aggregate level when we get here
func (s *Service) PropertyPatch(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	method string,
	meta map[string]interface{},
	body interface{},
	present bool,
) {
	property, ok := meta["property"].(string)
	if present && ok {
		// validator function can coerce type, act as a notification callback, or enforce constraints
		validator, ok := s.properties[property+"@meta.validator"]
		if ok {
			if vFN, ok := validator.(func(*domain.RedfishResourceProperty, interface{})); ok {
				vFN(rrp, body)
				return
			}
		}

		s.properties[property] = body
		rrp.Value = body
	}
}

//
//  PropertyGet vs GetProperty is confusing. Ooops. Should fix this naming snafu soon.
//

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

// UpdateProperty is a functional option to set an option at construction time or update the value after using ApplyOption.
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

type MetaInt map[string]interface{}
type MetaOption func(*Service, MetaInt) error

func (s *Service) Meta(options ...MetaOption) (ret map[string]interface{}) {
	s.RLock()
	defer s.RUnlock()
	r := MetaInt{}
	r.ApplyOption(s, options...)
	return map[string]interface{}(r)
}

// ApplyOptions will run all of the provided options, you can give options that
// are for this specific service, or you can give base helper options. If you
// give an unknown option, you will get a runtime panic.
func (m MetaInt) ApplyOption(s *Service, options ...MetaOption) error {
	for _, o := range options {
		err := o(s, m)
		if err != nil {
			return err
		}
	}
	return nil
}

func PropGET(name string) MetaOption {
	return func(s *Service, m MetaInt) error {
		s.MustPropertyUnlocked(name)
		m["GET"] = map[string]interface{}{"plugin": string(s.PluginTypeUnlocked()), "property": name}
		return nil
	}
}

func PropPATCH(name string) MetaOption {
	return func(s *Service, m MetaInt) error {
		s.MustPropertyUnlocked(name)
		m["PATCH"] = map[string]interface{}{"plugin": string(s.PluginTypeUnlocked()), "property": name}
		return nil
	}
}
