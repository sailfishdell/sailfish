package view

import (
	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/go-redfish/src/ocp/model"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

func WithUniqueName(name string) Option {
	return func(s *View) error {
		s.viewInstance = domain.PluginType(name)
		return nil
	}
}

func WithModel(m *model.Model) Option {
	return func(s *View) error {
		s.model = m
		return nil
	}
}

func MakeUUID() Option {
	return func(s *View) error {
		s.uuid = eh.NewUUID()
		return nil
	}
}

func WithGET(g getint) Option {
	return func(s *View) error {
		s.get = g
		return nil
	}
}

func WithPATCH(g patchint) Option {
	return func(s *View) error {
		s.patch = g
		return nil
	}
}

func WithNamedController(name string, c controller) Option {
	return func(s *View) error {
		s.controllers[name] = c
		return nil
	}
}
