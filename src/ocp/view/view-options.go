package view

import (
	"github.com/superchalupa/go-redfish/src/ocp/model"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

func WithURI(name string) Option {
	return func(s *View) error {
		s.pluginType = domain.PluginType(name)
		s.viewURI   = name
		return nil
	}
}

func WithModel(name string, m *model.Model) Option {
	return func(s *View) error {
		s.models[name] = m
		return nil
	}
}

func WithFormatter(name string, g formatter) Option {
	return func(s *View) error {
		s.outputFormatters[name] = g
		return nil
	}
}

func WithController(name string, c controller) Option {
	return func(s *View) error {
		s.controllers[name] = c
		return nil
	}
}
