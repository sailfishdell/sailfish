package plugins

import (
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
	"sync"
)

type Option func(*Model) error

// backwards compatible name
type Service = Model

func NewService(options ...Option) *Model {
	return NewModel(options...)
}

type Model struct {
	sync.RWMutex
	properties map[string]interface{}
}

func NewModel(options ...Option) *Model {
	s := &Model{
		properties: map[string]interface{}{},
	}

	s.ApplyOption(options...)
	return s
}

func (s *Model) ApplyOption(options ...Option) error {
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

func (s *Model) GetProperty(p string) interface{} {
	s.RLock()
	defer s.RUnlock()
	return s.properties[p]
}
func (s *Model) GetPropertyOk(p string) (ret interface{}, ok bool) {
	s.RLock()
	defer s.RUnlock()
	ret, ok = s.properties[p]
	return
}
func (s *Model) UpdateProperty(p string, i interface{}) {
	s.Lock()
	defer s.Unlock()
	s.properties[p] = i
}

func (s *Model) GetPropertyUnlocked(p string) interface{} { return s.properties[p] }
func (s *Model) GetPropertyOkUnlocked(p string) (ret interface{}, ok bool) {
	ret, ok = s.properties[p]
	return
}
func (s *Model) UpdatePropertyUnlocked(p string, i interface{}) { s.properties[p] = i }

func (s *Model) PluginType() domain.PluginType {
	return s.GetProperty("plugin_type").(domain.PluginType)
}

func (s *Model) PluginTypeUnlocked() domain.PluginType {
	return s.GetPropertyUnlocked("plugin_type").(domain.PluginType)
}
