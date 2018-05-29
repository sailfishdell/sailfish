package view

import (
	"github.com/superchalupa/go-redfish/src/ocp/model"
)

type MetaInt map[string]interface{}
type MetaOption func(*View, MetaInt) error

// TODO: this doesnt' need to be a method on *service. It could just take as parameter.
func (s *View) Meta(options ...MetaOption) (ret map[string]interface{}) {
	s.RLock()
	defer s.RUnlock()
	r := MetaInt{}
	r.ApplyOption(s, options...)
	return map[string]interface{}(r)
}

// ApplyOptions will run all of the provided options, you can give options that
// are for this specific service, or you can give base helper options. If you
// give an unknown option, you will get a runtime panic.
func (m MetaInt) ApplyOption(s *View, options ...MetaOption) error {
	for _, o := range options {
		err := o(s, m)
		if err != nil {
			return err
		}
	}
	return nil
}

func PropGET(name string) MetaOption {
	return func(s *View, m MetaInt) error {
		model.MustPropertyUnlocked(s.model, name)
		m["GET"] = map[string]interface{}{"plugin": string(s.PluginType()), "property": name}
		return nil
	}
}

func PropPATCH(name string) MetaOption {
	return func(s *View, m MetaInt) error {
		model.MustPropertyUnlocked(s.model, name)
		m["PATCH"] = map[string]interface{}{"plugin": string(s.PluginType()), "property": name}
		return nil
	}
}
