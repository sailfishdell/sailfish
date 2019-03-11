package eventservice

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"path"
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

type uploadService interface {
	WithUpload(context.Context, string, string, view.Upload) view.Option
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
	uploadSvc uploadService
}

var GlobalEventService *EventService

func New(ctx context.Context, cfg *viper.Viper, cfgMu *sync.RWMutex, d *domain.DomainObjects, instantiateSvc *testaggregate.Service, actionSvc actionService, uploadSvc uploadService) *EventService {
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
	uri := subView.GetURI()
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
		defer es.d.CommandHandler.HandleCommand(context.Background(), &domain.RemoveRedfishResource{ID: subView.GetUUID(), ResourceURI: subView.GetURI()})
		defer listener.Close()
		firstEvents := true
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
					return
				case ExternalRedfishEvent:
					subLogger.Info(" redfish event processing")
					// NOTE: we don't actually check to ensure that this is an actual ExternalRedfishEventData specifically because Metric Reports don't currently go through like this.
					esModel := subView.GetModel("default")
					if esModel.GetProperty("protocol") != "Redfish" {
						subLogger.Info("Not Redfish Protocol")
						continue
					} else if firstEvents {
						//MSM work around, replay mCHARS faults into events
						firstEvents = false
						evt := event.Data()
						if eventPtr, ok := evt.(*ExternalRedfishEventData); ok {
							eventlist := makeMCHARSevents(es, ctx)
							for idx := range eventlist {
								eventPtr.Events = append(eventPtr.Events, &eventlist[idx])
							}
						}
					}
					context := esModel.GetProperty("context")
					eventtypes := esModel.GetProperty("eventTypes")
					memberid := subView.GetUUID()
					if dest, ok := esModel.GetProperty("destination").(string); ok {
						subLogger.Info("Send to destination", "dest", dest)
						makePOST(es, dest, event, context, memberid, eventtypes)
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

func makePOST(es *EventService, dest string, event eh.Event, context interface{}, id eh.UUID, et interface{}) {
	log.MustLogger("event_service").Info("POST!", "dest", dest, "event", event)

	evt := event.Data()
	eventPtr, ok := evt.(*ExternalRedfishEventData)
	if !ok {
		return
	}
	eventlist := eventPtr.Events
	var outputEvents []*RedfishEventData
	validEvents, ok := et.([]string)
	if !ok {
		//TODO no subscription types are getting to here but right now all clients want Alert only
		validEvents = []string{"Alert"}
	}
	//Keep only the events in the Event Array which match the subscription
	for _, subevent := range eventlist {
		if subevent != nil {
			for _, subvalid := range validEvents {
				if subevent.EventType == subvalid {
					outputEvents = append(outputEvents, subevent)
					break
				}
			}
		}
	}
	if len(outputEvents) == 0 {
		return
	}
	//TODO put back when MSM is Redfish Event compliant
	select {
	case es.jc <- func() {
		for _, eachEvent := range outputEvents {
			d, err := json.Marshal(
				&struct {
					Context   interface{} `json:",omitempty"`
					MemberId  eh.UUID     `json:"MemberId"`
					ArgsCount int         `json:"MessageArgs@odata.count"`
					*RedfishEventData
				}{
					Context:          context,
					MemberId:         id,
					ArgsCount:        len(eachEvent.MessageArgs),
					RedfishEventData: eachEvent,
				},
			)

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
			req.Header.Set("Content-Type", "application/json")
			resp, err := client.Do(req)
			if err != nil {
				log.MustLogger("event_service").Warn("ERROR POSTING", "err", err)
			} else {
				resp.Body.Close()
			}
		}
	}:
	default: // drop the POST if the queue is full
		log.MustLogger("event_service").Crit("External Event Queue Full, dropping")
	}
}

func (es *EventService) PublishResourceUpdatedEventsForModel(ctx context.Context, modelName string) view.Option {
	return view.WatchModel(modelName, func(v *view.View, m *model.Model, updates []model.Update) {
		eventData := &RedfishEventData{
			EventType: "ResourceUpdated",
			//TODO MSM BUG: OriginOfCondition for events has to be a string or will be rejected
			OriginOfCondition: v.GetURI(),
		}
		go es.d.EventBus.PublishEvent(ctx, eh.NewEvent(RedfishEvent, eventData, time.Now()))
	})
}

func makeMCHARSevents(es *EventService, ctx context.Context) []RedfishEventData {
	mCharsUri := "/redfish/v1/Managers/CMC.Integrated.1/Logs/FaultList"
	uriList := es.d.FindMatchingURIs(func(uri string) bool { return path.Dir(uri) == mCharsUri })
	returnList := []RedfishEventData{}
	for _, uri := range uriList {
		faultAgg, err := es.d.ExpandURI(ctx, uri)
		if err != nil {
			continue
		}
		mcharsMap, ok := domain.Flatten(faultAgg, false).(map[string]interface{})
		if !ok {
			continue
		}

		oem, ok := mcharsMap["Oem"].(map[string]interface{})
		var fqddString string
		if ok {
			dell, ok := oem["Dell"].(map[string]interface{})
			if !ok {
				continue
			}
			fqddString = dell["FQDD"].(string)
		}

		messArgs, ok := mcharsMap["MessageArgs"].([]interface{})
		messageArgsStringArray := []string{}
		if ok {
			for _, arg := range messArgs {
				messageArgsStringArray = append(messageArgsStringArray, arg.(string))
			}
		}

		mainEvent := RedfishEventData{
			EventType:         "Alert",
			EventId:           mcharsMap["Id"].(string),
			EventTimestamp:    mcharsMap["Created"].(string),
			Severity:          mcharsMap["Severity"].(string),
			Message:           mcharsMap["Message"].(string),
			MessageId:         mcharsMap["MessageId"].(string),
			MessageArgs:       messageArgsStringArray,
			OriginOfCondition: fqddString,
		}
		//Create an event out of the mCHARS
		returnList = append(returnList, mainEvent)
	}
	return returnList
}
