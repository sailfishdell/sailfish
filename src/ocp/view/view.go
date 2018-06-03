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

type formatter func(
	ctx context.Context,
	v *View,
	m *model.Model,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	meta map[string]interface{},
) error

type action func()

type View struct {
	sync.RWMutex
	pluginType       domain.PluginType
	viewURI          string
	uuid             eh.UUID
	controllers      map[string]controller
	models           map[string]*model.Model
	outputFormatters map[string]formatter
	actions          map[string]action
}

func New(options ...Option) *View {
	s := &View{
		uuid:             eh.NewUUID(),
		controllers:      map[string]controller{},
		models:           map[string]*model.Model{},
		outputFormatters: map[string]formatter{},
		actions:          map[string]action{},
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
	return s.pluginType
}

func (s *View) GetUUID() eh.UUID {
	s.RLock()
	defer s.RUnlock()
	return s.uuid
}

func (s *View) GetURI() string {
	s.RLock()
	defer s.RUnlock()
	return s.viewURI
}

func (s *View) GetModel(name string) *model.Model {
	s.RLock()
	defer s.RUnlock()
	return s.models[name]
}

func (s *View) GetController(name string) controller {
	s.RLock()
	defer s.RUnlock()
	return s.controllers[name]
}
