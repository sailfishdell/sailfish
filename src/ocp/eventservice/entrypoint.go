package eventservice

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	eh "github.com/looplab/eventhorizon"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"
	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
	"github.com/superchalupa/sailfish/src/ocp/model"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

type viewer interface {
	GetUUID() eh.UUID
	GetURI() string
}

type actionService interface {
	WithAction(context.Context, string, string, view.Action) view.Option
}

type EventService struct {
	d         *domain.DomainObjects
	ew        *eventwaiter.EventWaiter
	cfg       *viper.Viper
	cfgMu     *sync.RWMutex
	jc        chan Job
	wrap      func(string, map[string]interface{}) (log.Logger, *view.View, error)
	addparam  func(map[string]interface{}) map[string]interface{}
	actionSvc actionService
}

const WorkQueueLen = 10

var GlobalEventService *EventService

func New(ctx context.Context, cfg *viper.Viper, cfgMu *sync.RWMutex, d *domain.DomainObjects, instantiateSvc *testaggregate.Service, actionSvc actionService) *EventService {
	EventPublisher := eventpublisher.NewEventPublisher()
	d.EventBus.AddHandler(eh.MatchAnyEventOf(ExternalRedfishEvent, domain.RedfishResourceRemoved), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter(eventwaiter.SetName("Event Service"), eventwaiter.NoAutoRun)
	EventPublisher.AddObserver(EventWaiter)
	go EventWaiter.Run()

	ret := &EventService{
		d:         d,
		ew:        EventWaiter,
		cfg:       cfg,
		cfgMu:     cfgMu,
		jc:        CreateWorkers(100, 6),
		actionSvc: actionSvc,
		wrap: func(name string, params map[string]interface{}) (log.Logger, *view.View, error) {
			return instantiateSvc.InstantiateFromCfg(ctx, cfg, cfgMu, name, params)
		},
	}

	GlobalEventService = ret
	return ret
}

// StartEventService will create a model, view, and controller for the eventservice, then start a goroutine to publish events
//      If you want to save settings, hook up a mapper to the "default" view returned
func (es *EventService) StartEventService(ctx context.Context, logger log.Logger, instantiateSvc *testaggregate.Service, params map[string]interface{}) *view.View {
	es.addparam = func(input map[string]interface{}) (output map[string]interface{}) {
		output = map[string]interface{}{}
		for k, v := range params {
			output[k] = v
		}
		for k, v := range input {
			output[k] = v
		}
		return
	}

	_, esView, _ := instantiateSvc.InstantiateFromCfg(ctx, es.cfg, es.cfgMu, "eventservice", es.addparam(map[string]interface{}{
		"submittestevent": view.Action(MakeSubmitTestEvent(es.d.EventBus)),
	}))
	params["eventsvc_id"] = esView.GetUUID()
	params["eventsvc_uri"] = esView.GetURI()
	instantiateSvc.InstantiateFromCfg(ctx, es.cfg, es.cfgMu, "subscriptioncollection", es.addparam(map[string]interface{}{
		"collection_uri": "/redfish/v1/EventService/Subscriptions",
	}))

	// The Plugin: "EventService" property on the Subscriptions endpoint is how we know to run this command
	eh.RegisterCommand(func() eh.Command {
		return &POST{es: es, d: es.d}
	})
	PublishRedfishEvents(ctx, esView.GetModel("default"), es.d.EventBus)

	return esView
}

// CreateSubscription will create a model, view, and controller for the subscription
//      If you want to save settings, hook up a mapper to the "default" view returned
func (es *EventService) CreateSubscription(ctx context.Context, logger log.Logger, sub Subscription, cancel func()) *view.View {
	subLogger, subView, _ := es.wrap("subscription", es.addparam(map[string]interface{}{
		"destination": sub.Destination,
		"protocol":    sub.Protocol,
		"context":     sub.Context,
		"eventTypes":  sub.EventTypes,
	}))

	// set up listener for the delete event
	// INFO: this listener will only ever get domain.RedfishResourceRemoved or ExternalRedfishEvent
	listener, err := es.ew.Listen(ctx,
		func(event eh.Event) bool {
			t := event.EventType()
			// TODO: will need to add metric reports here
			// TODO: also need to add the whole event coalescing here as well
			if t == ExternalRedfishEvent {
				return true
			}
			if t != domain.RedfishResourceRemoved {
				return false
			}
			if data, ok := event.Data().(*domain.RedfishResourceRemovedData); ok {
				if data.ResourceURI == subView.GetURI() {
					return true
				}
			}
			return false
		},
	)
	if err != nil {
		return nil
	}

	go func() {
		// close the view when we exit this goroutine
		defer subView.Close()
		// delete the aggregate
		defer es.d.CommandHandler.HandleCommand(context.Background(), &domain.RemoveRedfishResource{ID: subView.GetUUID(), ResourceURI: subView.GetURI()})
		defer listener.Close()

		for {
			select {
			case event := <-listener.Inbox():
				if e, ok := event.(syncEvent); ok {
					e.Done()
				}

				subLogger.Debug("Got internal redfish event", "event", event)
				switch typ := event.EventType(); typ {
				case domain.RedfishResourceRemoved:
					subLogger.Info("Cancelling subscription", "uri", subView.GetURI())
					cancel()
				case ExternalRedfishEvent:
					subLogger.Info(" redfish event processing")
					// NOTE: we don't actually check to ensure that this is an actual ExternalRedfishEventData specifically because Metric Reports don't currently go through like this.
					esModel := subView.GetModel("default")
					if esModel.GetProperty("protocol") != "Redfish" {
						subLogger.Info("Not Redfish Protocol")
						continue
					}
					context := esModel.GetProperty("context")
					if dest, ok := esModel.GetProperty("destination").(string); ok {
						subLogger.Info("Send to destination", "dest", dest)
						select {
						case es.jc <- makePOST(dest, event, context):
						default: // drop the POST if the queue is full
						}
					}
				}

			case <-ctx.Done():
				subLogger.Debug("context is done: exiting event service publisher")
				return
			}
		}
	}()

	return subView
}

func makePOST(dest string, event eh.Event, context interface{}) func() {
	return func() {
		log.MustLogger("event_service").Info("POST!", "dest", dest, "event", event)

		evt := event.Data()
		var d []byte
		var err error
		if _, ok := evt.(*ExternalRedfishEventData); ok {
			d, err = json.Marshal(
				&struct {
					*ExternalRedfishEventData
					Context interface{} `json:",omitempty"`
				}{
					ExternalRedfishEventData: evt.(*ExternalRedfishEventData),
					Context:                  context,
				},
			)
		} else {
			d, err = json.Marshal(evt)
		}

		// TODO: should be able to configure timeout
		// TODO: Shore up security for POST
		client := &http.Client{
			Timeout: time.Second * 3,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
		req, err := http.NewRequest("POST", dest, bytes.NewBuffer(d))
		req.Header.Add("OData-Version", "4.0")
		resp, err := client.Do(req)
		if err != nil {
			log.MustLogger("event_service").Warn("ERROR POSTING", "err", err)
			return
		}
		resp.Body.Close()
	}
}

func (es *EventService) PublishResourceUpdatedEventsForModel(ctx context.Context, modelName string) view.Option {
	return view.WatchModel(modelName, func(v *view.View, m *model.Model, updates []model.Update) {
		eventData := &RedfishEventData{
			EventType:         "ResourceUpdated",
			//TODO MSM BUG: OriginOfCondition for events has to be a string or will be rejected
			OriginOfCondition: v.GetURI(),
		}
		go es.d.EventBus.PublishEvent(ctx, eh.NewEvent(RedfishEvent, eventData, time.Now()))
	})
}
