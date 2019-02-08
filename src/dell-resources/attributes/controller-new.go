package attributes

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
  "fmt"
	eh "github.com/looplab/eventhorizon"


	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/event"
	"github.com/superchalupa/sailfish/src/ocp/model"
  "github.com/superchalupa/sailfish/src/looplab/eventwaiter"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

type Service struct {
	sync.RWMutex
	forwardMapping map[*model.Model][]string
	cache          map[string][]*model.Model
	logger         log.Logger
	eb             eh.EventBus
  ew             waiter
}

type syncEvent interface {
  Add(int)
  Done()
}

type waiter interface {
  Listen(context.Context, func(eh.Event) bool) (*eventwaiter.EventListener, error)
}

type listener interface {
  Inbox() <-chan eh.Event
  Close()
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
  ret.ew = &sp.EW

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
			value := AttributeData{
				Privileges: data.Privileges,
				Value:      data.Value,
			}
			m.ApplyOption(WithAttribute(data.Group, data.Index, data.Name, value))
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

func getAttrValue(m *model.Model, group, gindex, name string) (ret interface{}, ok bool) {
	attributes, ok := m.GetPropertyOkUnlocked("attributes")
	if !ok {
		return nil, ok
	}
	attrMap := attributes.(map[string]map[string]map[string]interface{})

	groupMap, ok := attrMap[group]
	if !ok {
		return nil, ok
	}

	index, ok := groupMap[gindex]
	if !ok {
		return nil, ok
	}

	value, ok := index[name]
	if !ok {
		return nil, ok
	}

	return value, ok
}

type HTTP_code struct {
  status_code int
  err_message []string
}

func (e HTTP_code) StatusCode() int {
  return e.status_code
}

func (e HTTP_code) ErrMessage() []string {
  return e.err_message
}

func (e HTTP_code) Error() string {
  return fmt.Sprintf("Request Error Message: %s, Return Code: %d", e.err_message, e.status_code)
}

func (b *breadcrumb) UpdateRequest(ctx context.Context, property string, value interface{}, auth *domain.RedfishAuthorizationProperty) (interface{}, error) {
	b.s.RLock()
	defer b.s.RUnlock()

	b.s.logger.Debug("UpdateRequest", "property", property, "value", value)

  reqIDs := []eh.UUID{}
  responses := []AttributeUpdatedData{}
  status_code := 400
  errs := []string{}
  patch_timeout := 3

  l, err := b.s.ew.Listen(ctx, func(event eh.Event) bool {
    if event.EventType() != AttributeUpdated {
      return false
    }
    _, ok := event.Data().(*AttributeUpdatedData)
    if !ok {
      return false
    }
    return true
  })
  if err != nil {
    b.s.logger.Error("Could not create listener", "err", err)
    return nil, errors.New("Failed to make attribute updated event listener")
  }
  l.Name = "patch listener"
  var listener listener
  listener = l

  defer listener.Close()

	for k, v := range value.(map[string]interface{}) {
		stuff := strings.Split(k, ".")
		reqUUID := eh.NewUUID()

		//  - validate that the requested member is in this list
		//  - validate that it is writable
		//  - validate that user has perms
		attrVal, ok := getAttrValue(b.m, stuff[0], stuff[1], stuff[2])
		if !ok {
			b.s.logger.Error("not found", "Attribute", k)
			continue
		}

		var ad AttributeData
		if !ad.WriteAllowed(attrVal, auth) {
			b.s.logger.Error("Unable to set", "Attribute", k)
			continue
		}

		data := &AttributeUpdateRequestData{
			ReqID:         reqUUID,
			FQDD:          b.s.forwardMapping[b.m][0], // take the first fqdd
			Group:         stuff[0],
			Index:         stuff[1],
			Name:          stuff[2],
			Value:         v,
			Authorization: *auth,
		}
		b.s.eb.PublishEvent(ctx, eh.NewEvent(AttributeUpdateRequest, data, time.Now()))
    reqIDs = append(reqIDs, reqUUID)
	}

  // create a timer based on number of attributes to be patched
  timer := time.NewTimer(time.Duration(patch_timeout*len(reqIDs)) * time.Second)

  for {
    select {
    case event := <- listener.Inbox():
      data, _ := event.Data().(*AttributeUpdatedData)
      for i, reqID := range reqIDs {
        if reqID == data.ReqID {
          //remove found reqid from list
          reqIDs[i] = reqIDs[len(reqIDs)-1]
          reqIDs = reqIDs[:len(reqIDs)-1]
          responses = append(responses, *data)
          if (data.Error == "") {
            status_code = 200
          } else {
            errs = append(errs, data.Error)
          }
          break
        }
      }

      if e, ok := event.(syncEvent); ok {
        e.Done()
      }

      if (len(reqIDs) == 0) {
        //all reqIDs found
        http_response := HTTP_code{status_code: status_code, err_message: errs}
        return nil, http_response
      }

    case <- timer.C:
      //time out for any attr updated events that we are still waiting for
      //return 400, nil
      return nil, HTTP_code{status_code: 400, err_message: []string{"Timed out!"}}

    case <- ctx.Done():
      return nil, HTTP_code{status_code: 200, err_message: nil}
    }
  }
}

func (b *breadcrumb) Close() {
	b.s.Lock()
	defer b.s.Unlock()
	delete(b.s.forwardMapping, b.m)
}
