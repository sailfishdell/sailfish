package eventservice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	eh "github.com/looplab/eventhorizon"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"

	ah "github.com/superchalupa/sailfish/src/actionhandler"
	"github.com/superchalupa/sailfish/src/eventwaiter"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/model"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

type viewer interface {
	GetUUID() eh.UUID
	GetURI() string
}

type EventService struct {
	ch eh.CommandHandler
	eb eh.EventBus
	ew *eventwaiter.EventWaiter
	jc chan Job
}

const WorkQueueLen = 10

var gEs *EventService

func New(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus) *EventService {
	EventPublisher := eventpublisher.NewEventPublisher()
	eb.AddHandler(eh.MatchAnyEventOf(ExternalRedfishEvent, domain.RedfishResourceRemoved), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter(eventwaiter.SetName("Event Service"))
	EventPublisher.AddObserver(EventWaiter)

	ret := &EventService{
		ch: ch,
		eb: eb,
		ew: EventWaiter,
		jc: CreateWorkers(100, 6),
	}

	gEs = ret
	return ret
}

// StartEventService will create a model, view, and controller for the eventservice, then start a goroutine to publish events
//      If you want to save settings, hook up a mapper to the "default" view returned
<<<<<<< HEAD
func startEventService(ctx context.Context, logger log.Logger, rootView viewer, ch eh.CommandHandler, eb eh.EventBus) *view.View {

=======
func (es *EventService) StartEventService(ctx context.Context, logger log.Logger, rootView viewer) *view.View {
>>>>>>> origin/dev/meb/master
	esLogger := logger.New("module", "EventService")

	esModel := model.New(
		model.UpdateProperty("max_milliseconds_to_queue", 500),
		model.UpdateProperty("max_events_to_queue", 20),
		model.UpdateProperty("delivery_retry_attempts", 3),
		model.UpdateProperty("delivery_retry_interval_seconds", 60),
	)

	esView := view.New(
		view.WithModel("default", esModel),
		view.WithModel("etag", esModel),
		view.WithURI(rootView.GetURI()+"/EventService"),
		ah.WithAction(ctx, esLogger, "submit.test.event", "/Actions/EventService.SubmitTestEvent", MakeSubmitTestEvent(es.eb), es.ch, es.eb),
		view.UpdateEtag("etag", []string{"max_milliseconds_to_queue", "max_events_to_queue", "delivery_retry_attempts", "delivery_retry_interval_seconds"}),
	)

	// The Plugin: "EventService" property on the Subscriptions endpoint is how we know to run this command
	eh.RegisterCommand(func() eh.Command { return &POST{model: esView.GetModel("default"), ch: es.ch, eb: es.eb} })
	AddAggregate(ctx, esLogger, esView, rootView.GetUUID(), es.ch, es.eb)
	PublishRedfishEvents(ctx, esModel, es.eb)

	return esView
}

// CreateSubscription forwards to a global instance of event service
// This func used by the POST Command handler in commands.go until that can be re-worked
func CreateSubscription(ctx context.Context, logger log.Logger, sub Subscription, cancel func()) *view.View {
	return gEs.CreateSubscription(ctx, logger, sub, cancel)
}

// CreateSubscription will create a model, view, and controller for the subscription
//      If you want to save settings, hook up a mapper to the "default" view returned
func (es *EventService) CreateSubscription(ctx context.Context, logger log.Logger, sub Subscription, cancel func()) *view.View {
	uuid := eh.NewUUID()
	uri := fmt.Sprintf("/redfish/v1/EventService/Subscriptions/%s", uuid)

	//esLogger := logger.New("module", "EventSubscription")
	esModel := model.New(
		model.UpdateProperty("destination", sub.Destination),
		model.UpdateProperty("protocol", sub.Protocol),
		model.UpdateProperty("context", sub.Context),
		model.UpdateProperty("event_types", sub.EventTypes),
	)
	subView := view.New(
		view.WithModel("default", esModel),
		view.WithURI(uri),
	)

	retprops := map[string]interface{}{
		"@odata.id":        uri,
		"@odata.type":      "#EventDestination.v1_2_0.EventDestination",
		"@odata.context":   "/redfish/v1/$metadata#EventDestination.EventDestination",
		"Id":               fmt.Sprintf("%s", uuid),
		"Protocol@meta":    subView.Meta(view.GETProperty("protocol"), view.GETModel("default")),
		"Name@meta":        subView.Meta(view.GETProperty("name"), view.GETModel("default")),
		"Destination@meta": subView.Meta(view.GETProperty("destination"), view.GETModel("default"), view.PropPATCH("session_timeout", "default")),
		"EventTypes@meta":  subView.Meta(view.GETProperty("event_types"), view.GETModel("default")),
		"Context@meta":     subView.Meta(view.GETProperty("context"), view.GETModel("default")),
	}

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
				if data.ResourceURI == uri {
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
		defer es.ch.HandleCommand(context.Background(), &domain.RemoveRedfishResource{ID: uuid, ResourceURI: uri})
		defer listener.Close()

		inbox := listener.Inbox()
		for {
			select {
			case event := <-inbox:
				log.MustLogger("event_service").Debug("Got internal redfish event", "event", event)
				switch typ := event.EventType(); typ {
				case domain.RedfishResourceRemoved:
					log.MustLogger("event_service").Info("Cancelling subscription", "uri", uri)
					cancel()
				case ExternalRedfishEvent:
					log.MustLogger("event_service").Info(" redfish event processing")
					// NOTE: we don't actually check to ensure that this is an actual ExternalRedfishEventData specifically because Metric Reports don't currently go through like this.
					if esModel.GetProperty("protocol") != "Redfish" {
						log.MustLogger("event_service").Info("Not Redfish Protocol")
						continue
					}
					context := esModel.GetProperty("context")
					if dest, ok := esModel.GetProperty("destination").(string); ok {
						log.MustLogger("event_service").Info("Send to destination", "dest", dest)
						select {
						case es.jc <- makePOST(dest, event, context):
						default: // drop the POST if the queue is full
						}
					}
				}

			case <-ctx.Done():
				log.MustLogger("event_service").Info("context is done")
				return
			}
		}
	}()

	// TODO: error handling
	es.ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          uuid,
			ResourceURI: retprops["@odata.id"].(string),
			Type:        retprops["@odata.type"].(string),
			Context:     retprops["@odata.context"].(string),
			Privileges: map[string]interface{}{
				"GET":    []string{"ConfigureManager"},
				"POST":   []string{},
				"PUT":    []string{"ConfigureManager"},
				"PATCH":  []string{"ConfigureManager"},
				"DELETE": []string{"ConfigureManager"},
			},
			Properties: retprops,
		})

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
		client := &http.Client{
			Timeout: time.Second * 1,
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
			OriginOfCondition: map[string]interface{}{"@odata.id": v.GetURI()},
		}
		go es.eb.PublishEvent(ctx, eh.NewEvent(RedfishEvent, eventData, time.Now()))
	})
}
