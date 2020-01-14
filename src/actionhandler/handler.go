package actionhandler

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func Setup(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus) {
	eh.RegisterCommand(func() eh.Command { return &POST{eventBus: eb, command: POSTCommand} })
	eh.RegisterCommand(func() eh.Command { return &POST{eventBus: eb, command: PATCHCommand} })
	eh.RegisterCommand(func() eh.Command { return &POST{eventBus: eb, command: DELETECommand} })
	eh.RegisterEventData(GenericActionEvent, func() eh.EventData { return &GenericActionEventData{} })
}

const (
	GenericActionEvent = eh.EventType("GenericActionEvent")
	POSTCommand        = eh.CommandType("GenericActionHandler:POST")
	PATCHCommand       = eh.CommandType("GenericActionHandler:PATCH")
	DELETECommand      = eh.CommandType("GenericActionHandler:DELETE")
)

type GenericActionEventData struct {
	ID          eh.UUID // id of aggregate
	CmdID       eh.UUID
	ResourceURI string
	Method      string

	ActionData interface{}
}

// HTTP POST Command
type POST struct {
	eventBus eh.EventBus
	command  eh.CommandType

	ID      eh.UUID           `json:"id"`
	CmdID   eh.UUID           `json:"cmdid"`
	Headers map[string]string `eh:"optional"`

	// make sure to make everything else optional or this will fail
	PostBody interface{} `eh:"optional"`

	Method string
}

// Static type checking for commands to prevent runtime errors due to typos
var _ = eh.Command(&POST{})

func (c *POST) AggregateType() eh.AggregateType { return domain.AggregateType }
func (c *POST) AggregateID() eh.UUID            { return c.ID }
func (c *POST) CommandType() eh.CommandType     { return c.command }
func (c *POST) SetAggID(id eh.UUID)             { c.ID = id }
func (c *POST) SetCmdID(id eh.UUID)             { c.CmdID = id }
func (c *POST) ParseHTTPRequest(r *http.Request) error {
	c.Method = r.Method
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
		Method:      c.Method,
	}, time.Now()))
	return nil
}

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

type BusObjs interface {
	GetBus() eh.EventBus
	GetWaiter() *eventwaiter.EventWaiter
	GetCommandHandler() eh.CommandHandler
}

func StartService(ctx context.Context, logger log.Logger, d BusObjs) *Service {
	ret := &Service{
		ch:      d.GetCommandHandler(),
		eb:      d.GetBus(),
		actions: map[string]*registration{},
	}

	// stream processor for action events
	listener := eventwaiter.NewListener(ctx, logger, d.GetWaiter(), func(ev eh.Event) bool {
		return ev.EventType() == GenericActionEvent
	})

	go listener.ProcessEvents(ctx, func(event eh.Event) {
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
			defer ret.RUnlock()
			reg, ok := ret.actions[data.ResourceURI]
			if !ok {
				// didn't find upload for this URL
				logger.Crit("COULD NOT FIND URI", "URI", data.ResourceURI)
				return
			}
			handler = reg.view.GetAction(reg.actionName)
			logger.Crit("URI", "uri", data.ResourceURI)

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
			go ret.eb.PublishEvent(ctx, responseEvent)
		}
	})

	return ret
}

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

		// only create receiver aggregate for the POST/PATCH for cases where it is a different
		// UIR from the view (this lets us handle PATCH on the actual view)
		if v.GetURIUnlocked() != uri {
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
		}

		return nil
	}
}
