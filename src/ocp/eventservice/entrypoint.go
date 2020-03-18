package eventservice

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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

type eventBinary struct {
	id   string
	data []byte
}

type actionService interface {
	WithAction(context.Context, string, string, view.Action) view.Option
}

type uploadService interface {
	WithUpload(context.Context, string, string, view.Upload) view.Option
}

type SubscriptionCtx struct {
	firstEvent  bool
	Destination string
	Protocol    string
	EventTypes  []string
	Context     string
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

var GlobalEventService *EventService

func New(ctx context.Context, cfg *viper.Viper, cfgMu *sync.RWMutex, d *domain.DomainObjects, instantiateSvc *testaggregate.Service, actionSvc actionService, uploadSvc uploadService) *EventService {
	EventPublisher := eventpublisher.NewEventPublisher()
	d.EventBus.AddHandler(eh.MatchAnyEventOf(ExternalRedfishEvent, domain.RedfishResourceRemoved, ExternalMetricEvent), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter(eventwaiter.SetName("Event Service"), eventwaiter.NoAutoRun)
	EventPublisher.AddObserver(EventWaiter)
	go EventWaiter.Run()

	ret := &EventService{
		d:     d,
		ew:    EventWaiter,
		cfg:   cfg,
		cfgMu: cfgMu,
		jc:    CreateWorkers(100, 6),
		wrap: func(name string, params map[string]interface{}) (log.Logger, *view.View, error) {
			return instantiateSvc.Instantiate(name, params)
		},
		actionSvc: actionSvc,
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

	_, esView, _ := instantiateSvc.Instantiate("eventservice", es.addparam(map[string]interface{}{
		"submittestevent": view.Action(MakeSubmitTestEvent(es.d.EventBus)),
	}))
	params["eventsvc_id"] = esView.GetUUID()
	params["eventsvc_uri"] = esView.GetURI()
	instantiateSvc.InstantiateNoRet("subscriptioncollection", es.addparam(map[string]interface{}{
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
	// INFO: this listener will only ever get domain.RedfishResourceRemoved ExternalMetricEvent or ExternalRedfishEvent
	uri := subView.GetURI()
	listener, err := es.ew.Listen(ctx,
		func(event eh.Event) bool {
			t := event.EventType()
			// TODO: will need to add metric reports here
			// TODO: also need to add the whole event coalescing here as well
			if t == ExternalRedfishEvent ||
				t == ExternalMetricEvent {
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

	// get model once
	esModel := subView.GetModel("default")

	dest := esModel.GetProperty("destination").(string)
	prot := esModel.GetProperty("protocol").(string)
	ctex := esModel.GetProperty("context").(string)

	subCtx := SubscriptionCtx{
		true,
		dest,
		prot,
		[]string{},
		ctex,
	}
	subCtx.EventTypes = append(subCtx.EventTypes, sub.EventTypes...)

	uuid := subView.GetUUID()

	logS := fmt.Sprintf("%s -- New Subscription created for uri=%s, prot=%s,eventT=%v?\n",
		time.Now().UTC().Format(time.UnixDate),
		dest,
		prot,
		subCtx.EventTypes)
	logToEventFile(logS)

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
				es.evaluateEvent(subLogger, subCtx, event, cancel, subView.GetURI(), uuid, ctx)
				if subCtx.firstEvent {
					subCtx.firstEvent = false
				}

			case <-ctx.Done():
				subLogger.Debug("context is done: exiting event service publisher")
				logToEventFile(fmt.Sprintf("%s -- Publisher Exited for uri=%s\n", time.Now().UTC().Format(time.UnixDate),
					dest))
				return
			}
		}
	}()

	return subView
}

func (es *EventService) evaluateEvent(log log.Logger, subCtx SubscriptionCtx, event eh.Event, cancel func(), URI string, uuid eh.UUID, ctx context.Context) {
	log.Debug("Got internal redfish event", "event", event)

	eventlist := []eventBinary{}

	if subCtx.Protocol != "Redfish" {
		log.Info("Not Redfish Protocol")
		return
	}
	if subCtx.Destination == "" {
		log.Info("Destination is empty, not sending event")
		return
	}

	switch typ := event.EventType(); typ {
	case domain.RedfishResourceRemoved:
		log.Info("Cancelling subscription", "uri", URI)
		logToEventFile(fmt.Sprintf("%s -- Subscription removed for uri=%s\n", time.Now().UTC().Format(time.UnixDate),
			subCtx.Destination))
		cancel()
		return
	case ExternalRedfishEvent:
		log.Info(" redfish event processing")
		// NOTE: we don't actually check to ensure that this is an actual ExternalRedfishEventData specifically because Metric Reports don't currently go through like this.

		tmpevt := event.Data()

		// eventPtr is shared btwn go routines(ssehandler), writes should be avoided
		eventPtr, ok := tmpevt.(*ExternalRedfishEventData)
		if !ok {
			log.Info("ExternalRedfishEvent does not have ExternalRedfishEventData")
			return
		}

		totalEvents := []*RedfishEventData{}
		if subCtx.firstEvent {
			//MSM work around, replay mCHARS faults into events
			redfishevents := makeMCHARSevents(es, ctx)

			for idx := range redfishevents {
				totalEvents = append(totalEvents, &redfishevents[idx])
			}
		}
		totalEvents = append(totalEvents, eventPtr.Events...)

		eventlist = makeExternalRedfishEvent(subCtx, totalEvents, uuid)
		if len(eventlist) == 0 {
			return
		}
		es.postExternalEvent(subCtx, event, eventlist)

	case ExternalMetricEvent:
		evt := event.Data()
		evtPtr, ok := evt.(MetricReportData)
		if !ok {

			log.Info("ExternalMetricEvent does not have ExternalMetricEventData")
			return
		}
		var id string
		idtmp, ok := evtPtr.Data["MetricName"]
		if !ok {
			id = "unknown"
		} else {
			id, ok = idtmp.(string)
			if !ok {
				id = "unknown"
			}
		}

		jsonBody, err := json.Marshal(evtPtr.Data)
		if err == nil {
			eb := eventBinary{
				id,
				jsonBody}

			eventlist = append(eventlist, eb)
			es.postExternalEvent(subCtx, event, eventlist)
		}

	}
}

// Externally POST ExternalRedfishEvent and ExternalMetricReportEvent
func (es *EventService) postExternalEvent(subCtx SubscriptionCtx, event eh.Event, eventlist []eventBinary) {
	//TODO put back when MSM is Redfish Event compliant
	select {
	case es.jc <- func() {
		for _, eachEvent := range eventlist {

			client := &http.Client{
				Timeout: time.Second * 5,
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				},
			}
			//Try up to 5 times to send event
			logToEventFile(fmt.Sprintf("%s -- STARTING to send MessageId=%s to uri=%s\n", time.Now().UTC().Format(time.UnixDate), eachEvent.id, subCtx.Destination))
			logSent := false
			for i := 0; i < 5 && !logSent; i++ {
				//Increasing wait between retries, first time don't wait i==0
				time.Sleep(time.Duration(i) * 2 * time.Second)
				req, err := http.NewRequest("POST", subCtx.Destination, bytes.NewBuffer(eachEvent.data))
				if err != nil {
					log.MustLogger("event_service").Crit("ERROR CREATING REQUEST", "destination", subCtx.Destination, "Buffer bytes", eachEvent.data)
					break
				}
				req.Header.Add("OData-Version", "4.0")
				req.Header.Set("Content-Type", "application/json")
				resp, err := client.Do(req)
				if err != nil {
					log.MustLogger("event_service").Crit("ERROR POSTING", "Id", eachEvent.id, "err", err)
					logToEventFile(fmt.Sprintf("%s -- ERROR POSTING Id=%s to uri=%s attempt=%d err=%s\n", time.Now().UTC().Format(time.UnixDate), eachEvent.id, subCtx.Destination, i+1, err))
				} else if resp.StatusCode == http.StatusOK ||
					resp.StatusCode == http.StatusCreated ||
					resp.StatusCode == http.StatusAccepted ||
					resp.StatusCode == http.StatusNoContent {
					//Got a good response end loop
					logToEventFile(fmt.Sprintf("%s -- Success sent Id=%s to uri=%s attempt=%d HTTP Status=%d\n", time.Now().UTC().Format(time.UnixDate), eachEvent.id, subCtx.Destination, i+1, resp.StatusCode))
					logSent = true
				} else {
					//Error code return
					log.MustLogger("event_service").Crit("ERROR POSTING", "Id", eachEvent.id, "StatusCode", resp.StatusCode, "uri", subCtx.Destination)
					logToEventFile(fmt.Sprintf("%s -- ERROR POSTING Id=%s to uri=%s attempt=%d HTTP Status=%d\n", time.Now().UTC().Format(time.UnixDate), eachEvent.id, subCtx.Destination, i+1, resp.StatusCode))
				}
				if resp != nil {
					resp.Body.Close()
				}
			}
			if !logSent {
				logToEventFile(fmt.Sprintf("%s -- FAILURE to send Id=%s to uri=%s\n", time.Now().UTC().Format(time.UnixDate), eachEvent.id, subCtx.Destination))
				log.MustLogger("event_service").Crit("ERROR POSTING, DROPPED", "Id", eachEvent.id, "uri", subCtx.Destination)
			}
		}
	}:
	default: // drop the POST if the queue is full
		log.MustLogger("event_service").Crit("External Event Queue Full, dropping")
		logToEventFile(fmt.Sprintf("%s -- DROP Messages to uri=%s\n", time.Now().UTC().Format(time.UnixDate), subCtx.Destination))
	}
}

func makeExternalRedfishEvent(subCtx SubscriptionCtx, events []*RedfishEventData, uuid eh.UUID) []eventBinary {
	log.MustLogger("event_service").Info("POST!", "dest", subCtx.Destination, "redfish event data", events)
	eventlist := []eventBinary{}

	if len(subCtx.EventTypes) == 0 {
		subCtx.EventTypes = append(subCtx.EventTypes, "Alert")
	}

	//Keep only the events in the Event Array which match the subscription
	for _, tmpEvent := range events {
		for _, subvalid := range subCtx.EventTypes {
			if tmpEvent.EventType == subvalid {
				jsonBody, err := json.Marshal(&struct {
					Context   interface{} `json:",omitempty"`
					MemberId  eh.UUID     `json:"MemberId"`
					ArgsCount int         `json:"MessageArgs@odata.count"`
					*RedfishEventData
				}{
					Context:          subCtx.Context,
					MemberId:         uuid,
					ArgsCount:        len(tmpEvent.MessageArgs),
					RedfishEventData: tmpEvent,
				},
				)
				if err == nil {
					id := tmpEvent.MessageId
					eb := eventBinary{
						id,
						jsonBody}

					eventlist = append(eventlist, eb)
				}
			}
		}
	}
	return eventlist
}

func (es *EventService) PublishResourceUpdatedEventsForModel(ctx context.Context, modelName string) view.Option {
	return view.WatchModel(modelName, func(v *view.View, m *model.Model, updates []model.Update) {
		go func() {
			eventData := &RedfishEventData{
				EventType: "ResourceUpdated",
				//TODO MSM BUG: OriginOfCondition for events has to be a string or will be rejected
				OriginOfCondition: v.GetURI(),
				MessageId:         "TST100",
			}
			es.d.EventBus.PublishEvent(ctx, eh.NewEvent(RedfishEvent, eventData, time.Now()))
		}()
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

func logToEventFile(msg string) {
	eventLogFileName := "/var/log/go/sailfish_events.log"
	logfile, _ := os.OpenFile(eventLogFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if logfile != nil {
		logfile.WriteString(msg)
		logfile.Close()
	} else {
		log.MustLogger("event_service").Crit(msg)
	}
}
