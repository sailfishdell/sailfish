package view

import (
	"sync"

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/go-redfish/src/ocp/model"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

type Option func(*View) error

type View struct {
	sync.RWMutex
	model        *model.Model
	viewInstance domain.PluginType
	uuid         eh.UUID
}

func NewView(options ...Option) *View {
	s := &View{}

	s.ApplyOption(options...)
	return s
}

func (s *View) ApplyOption(options ...Option) error {
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

func (s *View) PluginType() domain.PluginType {
	return s.viewInstance
}

func (s *View) GetUUID() eh.UUID {
	s.RLock()
	defer s.RUnlock()
	id := s.uuid
	return id
}
