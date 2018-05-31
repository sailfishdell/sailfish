package view

import (
	"context"
	"sync"

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/go-redfish/src/ocp/model"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

type Option func(*View) error

type controller interface {
	UpdateRequest(ctx context.Context, property string, value interface{}) (interface{}, error)
}

type getint func(
	v *View,
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	meta map[string]interface{},
) error

type patchint func(
	v *View,
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	meta map[string]interface{},
	body interface{},
	present bool,
)

type View struct {
	sync.RWMutex
	model        *model.Model
	viewInstance domain.PluginType
	uuid         eh.UUID
	controllers  map[string]controller
	get          getint
	patch        patchint
}

func NewView(options ...Option) *View {
	s := &View{
		controllers: map[string]controller{},
	}

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

func (s *View) GetModel(name string) *model.Model {
	s.RLock()
	defer s.RUnlock()

	return s.model
}

func (s *View) GetController(name string) controller {
	s.RLock()
	defer s.RUnlock()

	return s.controllers[name]
}
