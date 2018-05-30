package view

import (
	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/go-redfish/src/ocp/model"
)

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

func WithNamedController(name string, c controller) Option {
	return func(s *View) error {
		s.controllers[name] = c
		return nil
	}
}
