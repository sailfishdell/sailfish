package actionhandler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	eh "github.com/looplab/eventhorizon"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"

	"github.com/superchalupa/go-redfish/src/eventwaiter"
	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/event"
	"github.com/superchalupa/go-redfish/src/ocp/view"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

type waiter interface {
	Listen(context.Context, func(eh.Event) bool) (*eventwaiter.EventListener, error)
}

func InitService(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew waiter) {
	eh.RegisterCommand(func() eh.Command { return &POST{eventBus: eb, eventWaiter: ew} })
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
	eventBus    eh.EventBus
	eventWaiter waiter

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
	c.eventBus.PublishEvent(ctx, eh.NewEvent(GenericActionEvent, GenericActionEventData{
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
		if data, ok := event.Data().(GenericActionEventData); ok {
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

	EventPublisher := eventpublisher.NewEventPublisher()
	eb.AddHandler(eh.MatchAny(), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter()
	EventPublisher.AddObserver(EventWaiter)

	// stream processor for action events
	sp, err := event.NewEventStreamProcessor(ctx, EventWaiter, event.CustomFilter(SelectAction(actionURI)))
	if err != nil {
		logger.Error("Failed to create event stream processor", "err", err)
		return
	}
	sp.RunForever(func(event eh.Event) {
		eventData := domain.HTTPCmdProcessedData{
			CommandID:  event.Data().(GenericActionEventData).CmdID,
			Results:    map[string]interface{}{"msg": "Not Implemented"},
			StatusCode: 500,
			Headers:    map[string]string{},
		}

		logger.Crit("Action running!")
		handler := vw.GetAction(action)
		logger.Crit("handler", "handler", handler)
		if handler != nil {
			handler(ctx, event, &eventData)
		} else {
			logger.Warn("UNHANDLED action event: no function handler set up for this event.", "event", event)
		}

		responseEvent := eh.NewEvent(domain.HTTPCmdProcessed, eventData, time.Now())
		eb.PublishEvent(ctx, responseEvent)
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
