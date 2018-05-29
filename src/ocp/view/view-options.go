package view

import (
	"github.com/superchalupa/go-redfish/src/ocp/model"
)

func WithModel(m *model.Model) Option {
	return func(s *View) error {
		s.model = m
		return nil
	}
}
