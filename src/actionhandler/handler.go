package actionhandler

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/event"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func Setup(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus) {
	eh.RegisterCommand(func() eh.Command { return &POST{eventBus: eb} })
	eh.RegisterEventData(GenericActionEvent, func() eh.EventData { return &GenericActionEventData{} })
}

const (
	GenericActionEvent = eh.EventType("GenericActionEvent")
	POSTCommand        = eh.CommandType("GenericActionHandler:POST")
)

type GenericActionEventData struct {
	ID          eh.UUID // id of aggregate
	CmdID       eh.UUID
	ResourceURI string

	ActionData interface{}
}

// HTTP POST Command
type POST struct {
	eventBus eh.EventBus

	ID      eh.UUID           `json:"id"`
	CmdID   eh.UUID           `json:"cmdid"`
	Headers map[string]string `eh:"optional"`

	// make sure to make everything else optional or this will fail
	PostBody interface{} `eh:"optional"`
}

// Static type checking for commands to prevent runtime errors due to typos
var _ = eh.Command(&POST{})

func (c *POST) AggregateType() eh.AggregateType { return domain.AggregateType }
func (c *POST) AggregateID() eh.UUID            { return c.ID }
func (c *POST) CommandType() eh.CommandType     { return POSTCommand }
func (c *POST) SetAggID(id eh.UUID)             { c.ID = id }
func (c *POST) SetCmdID(id eh.UUID)             { c.CmdID = id }
func (c *POST) ParseHTTPRequest(r *http.Request) error {
	json.NewDecoder(r.Body).Decode(&c.PostBody)
	return nil
}
func (c *POST) Handle(ctx context.Context, a *domain.RedfishResourceAggregate) error {
	// Action handler needs to send HTTP response
	c.eventBus.PublishEvent(ctx, eh.NewEvent(GenericActionEvent, &GenericActionEventData{
		ID:          c.ID,
		CmdID:       c.CmdID,
		ResourceURI: a.ResourceURI,
		ActionData:  c.PostBody,
	}, time.Now()))
	return nil
}

func SelectAction(uri string) func(eh.Event) bool {
	return func(event eh.Event) bool {
		if event.EventType() != GenericActionEvent {
			return false
		}
		if data, ok := event.Data().(*GenericActionEventData); ok {
			if data.ResourceURI == uri {
				return true
			}
		}
		return false
	}
}

type prop interface {
	GetProperty(string) interface{}
}

type handler func(context.Context, eh.Event, *domain.HTTPCmdProcessedData) error

type actionrunner interface {
	GetAction(string) view.Action
}

type registration struct {
	actionName string
	view       actionrunner
}

type Service struct {
	sync.RWMutex
	ch      eh.CommandHandler
	eb      eh.EventBus
	actions map[string]*registration
}

func StartService(ctx context.Context, logger log.Logger, ch eh.CommandHandler, eb eh.EventBus) *Service {
	ret := &Service{
		ch:      ch,
		eb:      eb,
		actions: map[string]*registration{},
	}

	// stream processor for action events
	sp, err := event.NewESP(ctx, event.CustomFilter(func(ev eh.Event) bool {
		if ev.EventType() == GenericActionEvent {
			return true
		}
		return false
	}), event.SetListenerName("actionhandler"))
	if err != nil {
		logger.Error("Failed to create event stream processor", "err", err)
		return nil
	}
	go sp.RunForever(func(event eh.Event) {
		eventData := &domain.HTTPCmdProcessedData{
			CommandID:  event.Data().(*GenericActionEventData).CmdID,
			Results:    map[string]interface{}{"msg": "Not Implemented"},
			StatusCode: 500,
			Headers:    map[string]string{},
		}

		logger.Crit("Action running!")
		var handler view.Action
		if data, ok := event.Data().(*GenericActionEventData); ok {
			ret.RLock()
			reg := ret.actions[data.ResourceURI]
			handler = reg.view.GetAction(reg.actionName)
			logger.Crit("URI", "uri", data.ResourceURI)
			ret.RUnlock()
		}

		logger.Crit("handler", "handler", handler)

		// only send out our pre-canned response if no handler exists (above), or if handler sets the event status code to 0
		// for example, if data pump is going to directly send an httpcmdprocessed.
		if handler != nil {
			handler(ctx, event, eventData)
		} else {
			logger.Warn("UNHANDLED action event: no function handler set up for this event.", "event", event)
		}
		if eventData.StatusCode != 0 {
			responseEvent := eh.NewEvent(domain.HTTPCmdProcessed, eventData, time.Now())
			go eb.PublishEvent(ctx, responseEvent)
		}
	})

	return ret
}

//
// NEW HOTNESS
//

func (s *Service) WithAction(ctx context.Context, name string, uriSuffix string, a view.Action) view.Option {
	return func(v *view.View) error {
		uri := v.GetURIUnlocked() + uriSuffix
		v.SetActionUnlocked(name, a)
		v.SetActionURIUnlocked(name, uri)

		s.Lock()
		defer s.Unlock()
		s.actions[uri] = &registration{
			actionName: name,
			view:       v,
		}

		// The following redfish resource is created only for the purpose of being
		// a 'receiver' for the action command specified above.
		s.ch.HandleCommand(
			ctx,
			&domain.CreateRedfishResource{
				ID:          eh.NewUUID(),
				ResourceURI: uri,
				Type:        "Action",
				Context:     "Action",
				Plugin:      "GenericActionHandler",
				Privileges: map[string]interface{}{
					"POST": []string{"ConfigureManager"},
				},
				Properties: map[string]interface{}{},
			},
		)

		return nil
	}
}

//
// OLD BUSTED
//

func CreateViewAction(
	ctx context.Context,
	logger log.Logger,
	action string,
	actionURI string,
	vw actionrunner,
	ch eh.CommandHandler,
	eb eh.EventBus,
) {

	logger.Info("CREATING ACTION", "action", action, "actionURI", actionURI)

	// The following redfish resource is created only for the purpose of being
	// a 'receiver' for the action command specified above.
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          eh.NewUUID(),
			ResourceURI: actionURI,
			Type:        "Action",
			Context:     "Action",
			Plugin:      "GenericActionHandler",
			Privileges: map[string]interface{}{
				"POST": []string{"ConfigureManager"},
			},
			Properties: map[string]interface{}{},
		},
	)

	// stream processor for action events
	sp, err := event.NewESP(ctx, event.CustomFilter(SelectAction(actionURI)), event.SetListenerName("actionhandler"))
	if err != nil {
		logger.Error("Failed to create event stream processor", "err", err)
		return
	}
	go sp.RunForever(func(event eh.Event) {
		eventData := &domain.HTTPCmdProcessedData{
			CommandID:  event.Data().(*GenericActionEventData).CmdID,
			Results:    map[string]interface{}{"msg": "Not Implemented"},
			StatusCode: 500,
			Headers:    map[string]string{},
		}

		logger.Crit("Action running!")
		handler := vw.GetAction(action)
		logger.Crit("handler", "handler", handler)

		// only send out our pre-canned response if no handler exists (above), or if handler sets the event status code to 0
		// for example, if data pump is going to directly send an httpcmdprocessed.
		if handler != nil {
			handler(ctx, event, eventData)
		} else {
			logger.Warn("UNHANDLED action event: no function handler set up for this event.", "event", event)
		}
		if eventData.StatusCode != 0 {
			responseEvent := eh.NewEvent(domain.HTTPCmdProcessed, eventData, time.Now())
			go eb.PublishEvent(ctx, responseEvent)
		}
	})
}

func WithAction(ctx context.Context, logger log.Logger, name string, uriSuffix string, a view.Action, ch eh.CommandHandler, eb eh.EventBus) view.Option {
	return func(s *view.View) error {
		uri := s.GetURIUnlocked() + uriSuffix
		s.SetActionUnlocked(name, a)
		s.SetActionURIUnlocked(name, uri)
		CreateViewAction(ctx, logger, name, uri, s, ch, eb)
		return nil
	}
}
