package view

import (
	"context"
	//	"fmt"
	"strings"
	"sync"

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/sailfish/src/ocp/model"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

type closer interface {
	Close()
}

type Option func(*View) error

type controller interface {
	UpdateRequest(ctx context.Context, property string, value interface{}, auth *domain.RedfishAuthorizationProperty) (interface{}, error)
	Close()
}

type formatter func(
	ctx context.Context,
	v *View,
	m *model.Model,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	auth *domain.RedfishAuthorizationProperty,
	meta map[string]interface{},
) error

type Action func(context.Context, eh.Event, *domain.HTTPCmdProcessedData) error
type Upload func(context.Context, eh.Event, *domain.HTTPCmdProcessedData) error

type View struct {
	sync.RWMutex
	pluginType       domain.PluginType
	viewURI          string
	uuid             eh.UUID
	controllers      map[string]controller
	models           map[string]*model.Model
	outputFormatters map[string]formatter
	actions          map[string]Action
	uploads          map[string]Upload
	actionURI        map[string]string
	uploadURI        map[string]string
	registerplugin   bool
	closefn          []func()
}

func New(options ...Option) *View {
	s := &View{
		uuid:             eh.NewUUID(),
		controllers:      map[string]controller{},
		models:           map[string]*model.Model{},
		outputFormatters: map[string]formatter{},
		actions:          map[string]Action{},
		uploads:          map[string]Upload{},
		actionURI:        map[string]string{},
		uploadURI:        map[string]string{},
		registerplugin:   true,
		closefn:          []func(){},
	}

	s.ApplyOption(options...)
	if s.registerplugin {
		// close any previous registrations
		p, err := domain.InstantiatePlugin(s.PluginType())
		if err == nil && p != nil {
			if c, ok := p.(closer); ok {
				c.Close()
			}
		}

		// caller responsible for registering if this isn't set
		s.closefn = append(s.closefn, func() { domain.UnregisterPlugin(s.PluginType()) })
		domain.RegisterPlugin(func() domain.Plugin { return s })
	}
	return s
}

func (s *View) Close() {
	for _, fn := range s.closefn {
		fn()
	}
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

func (s *View) GetUUIDUnlocked() eh.UUID {
	return s.uuid
}

func (s *View) GetUUID() eh.UUID {
	s.RLock()
	defer s.RUnlock()
	return s.GetUUIDUnlocked()
}

func (s *View) GetURIUnlocked() string {
	return s.viewURI
}

func (s *View) GetURI() string {
	s.RLock()
	defer s.RUnlock()
	return s.GetURIUnlocked()
}

func (s *View) GetModel(name string) *model.Model {
	s.RLock()
	defer s.RUnlock()
	return s.models[name]
}

// will return models that has the passed in substring
func (s *View) GetModels(sub_name string) map[string]*model.Model {
	modelMatch := map[string]*model.Model{}
	s.RLock()
	defer s.RUnlock()
	for n, m := range s.models {
		if strings.Contains(n, sub_name) {
			modelMatch[n] = m
		}
	}

	return modelMatch 
}

func (s *View) GetController(name string) controller {
	s.RLock()
	defer s.RUnlock()
	return s.controllers[name]
}

func (s *View) GetAction(name string) Action {
	s.RLock()
	defer s.RUnlock()
	return s.actions[name]
}

func (s *View) GetActionURI(name string) string {
	s.RLock()
	defer s.RUnlock()
	return s.actionURI[name]
}

func (v *View) SetActionURIUnlocked(name string, URI string) {
	v.actionURI[name] = URI
}

func (v *View) SetActionUnlocked(name string, a Action) {
	v.actions[name] = a
}

func (s *View) GetUpload(name string) Upload {
	s.RLock()
	defer s.RUnlock()
	return s.uploads[name]
}

func (s *View) GetUploadURI(name string) string {
	s.RLock()
	defer s.RUnlock()
	return s.uploadURI[name]
}

func (v *View) SetUploadURIUnlocked(name string, URI string) {
	v.uploadURI[name] = URI
}

func (v *View) SetUploadUnlocked(name string, u Upload) {
	v.uploads[name] = u
}
