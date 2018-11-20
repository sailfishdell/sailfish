package attributes

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/event"
	"github.com/superchalupa/sailfish/src/ocp/model"
)

type Service struct {
	sync.RWMutex
	forwardMapping map[*model.Model][]string
	cache          map[string][]*model.Model
	logger         log.Logger
	eb             eh.EventBus
}

func StartService(ctx context.Context, logger log.Logger, eb eh.EventBus) (*Service, error) {
	ret := &Service{
		forwardMapping: map[*model.Model][]string{},
		cache:          map[string][]*model.Model{},
		logger:         logger,
		eb:             eb,
	}

	// stream processor for action events
	sp, err := event.NewESP(ctx, event.CustomFilter(ret.selectCachedAttributes()), event.SetListenerName("NEW_attributes"))
	if err != nil {
		logger.Error("Failed to create event stream processor", "err", err)
		return nil, errors.New("Failed to create stream processor")
	}
	go sp.RunForever(func(event eh.Event) {
		data, ok := event.Data().(*AttributeUpdatedData)
		if !ok {
			return
		}

		ret.RLock()
		defer ret.RUnlock()

		modelArray, ok := ret.cache[data.FQDD]
		if !ok {
			return
		}

		for _, m := range modelArray {
			m.ApplyOption(WithAttribute(data.Group, data.Index, data.Name, data.Value))
		}
	})

	return ret, nil
}

type breadcrumb struct {
	s *Service
	m *model.Model
}

func (s *Service) NewMapping(ctx context.Context, m *model.Model, fqdds []string) *breadcrumb {
	s.Lock()
	defer s.Unlock()

	s.forwardMapping[m] = fqdds
	s.updateCache()

	return &breadcrumb{m: m, s: s}
}

func (s *Service) updateCache() {
	s.cache = map[string][]*model.Model{}
	for mdl, fqdds := range s.forwardMapping {
		for _, fqdd := range fqdds {
			mdls, ok := s.cache[fqdd]
			if !ok {
				mdls = []*model.Model{}
			}
			mdls = append(mdls, mdl)
			s.cache[fqdd] = mdls
		}
	}
}

func (s *Service) selectCachedAttributes() func(eh.Event) bool {
	return func(event eh.Event) bool {
		s.RLock()
		defer s.RUnlock()
		if event.EventType() == AttributeUpdated {
			if data, ok := event.Data().(*AttributeUpdatedData); ok {
				if _, ok := s.cache[data.FQDD]; ok {
					return true
				}
			}
		}
		return false
	}
}

func (b *breadcrumb) UpdateRequest(ctx context.Context, property string, value interface{}) (interface{}, error) {
	b.s.RLock()
	defer b.s.RUnlock()

	b.s.logger.Debug("UpdateRequest", "property", property, "value", value)

	for k, v := range value.(map[string]interface{}) {
		stuff := strings.Split(k, ".")
		reqUUID := eh.NewUUID()

		// TODO: validate
		//  - validate that the requested member is in this list
		//  - validate that it is writable
		//  - validate that user has perms
		//
		data := &AttributeUpdateRequestData{
			ReqID: reqUUID,
			FQDD:  b.s.forwardMapping[b.m][0], // take the first fqdd
			Group: stuff[0],
			Index: stuff[1],
			Name:  stuff[2],
			Value: v,
		}
		b.s.eb.PublishEvent(ctx, eh.NewEvent(AttributeUpdateRequest, data, time.Now()))
	}
	return nil, nil
}

func (b *breadcrumb) Close() {
	b.s.Lock()
	defer b.s.Unlock()
	delete(b.s.forwardMapping, b.m)
}
