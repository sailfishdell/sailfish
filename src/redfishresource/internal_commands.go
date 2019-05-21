package domain

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	eh "github.com/looplab/eventhorizon"
	"github.com/mitchellh/mapstructure"
	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/event"
)

type syncEvent interface {
	Add(int)
	Wait()
}

func init() {
	eh.RegisterCommand(func() eh.Command { return &CreateRedfishResource{} })
	eh.RegisterCommand(func() eh.Command { return &RemoveRedfishResource{} })
	eh.RegisterCommand(func() eh.Command { return &UpdateRedfishResourceProperties{} })
	eh.RegisterCommand(func() eh.Command { return &RemoveRedfishResourceProperty{} })
	eh.RegisterCommand(func() eh.Command { return &InjectEvent{} })
}

const (
	CreateRedfishResourceCommand           = eh.CommandType("internal:RedfishResource:Create")
	RemoveRedfishResourceCommand           = eh.CommandType("internal:RedfishResource:Remove")
	UpdateRedfishResourcePropertiesCommand = eh.CommandType("internal:RedfishResourceProperties:Update")
	RemoveRedfishResourcePropertyCommand   = eh.CommandType("internal:RedfishResourceProperties:Remove")
	InjectEventCommand                     = eh.CommandType("internal:Event:Inject")
)

// Static type checking for commands to prevent runtime errors due to typos
var _ = eh.Command(&CreateRedfishResource{})
var _ = eh.Command(&RemoveRedfishResource{})
var _ = eh.Command(&UpdateRedfishResourceProperties{})
var _ = eh.Command(&RemoveRedfishResourceProperty{})
var _ = eh.Command(&InjectEvent{})

var immutableProperties = []string{"@odata.id", "@odata.type", "@odata.context"}

// CreateRedfishResource Command
type CreateRedfishResource struct {
	ID          eh.UUID `json:"id"`
	ResourceURI string
	Type        string
	Context     string
	Privileges  map[string]interface{}

	// optional stuff
	Headers    map[string]string      `eh:"optional"`
	Plugin     string                 `eh:"optional"`
	Properties map[string]interface{} `eh:"optional"`
	Meta       map[string]interface{} `eh:"optional"`
	Private    map[string]interface{} `eh:"optional"`
}

// AggregateType satisfies base Aggregate interface
func (c *CreateRedfishResource) AggregateType() eh.AggregateType { return AggregateType }

// AggregateID satisfies base Aggregate interface
func (c *CreateRedfishResource) AggregateID() eh.UUID { return c.ID }

// CommandType satisfies base Command interface
func (c *CreateRedfishResource) CommandType() eh.CommandType { return CreateRedfishResourceCommand }

func (c *CreateRedfishResource) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	requestLogger := ContextLogger(ctx, "internal_commands")
	requestLogger.Info("CreateRedfishResource", "META", a.Properties.Meta)

	if a.ID != eh.UUID("") {
		requestLogger.Error("Aggregate already exists!", "command", "CreateRedfishResource", "UUID", a.ID, "URI", a.ResourceURI, "request_URI", c.ResourceURI)
		return errors.New("Already created!")
	}
	a.ID = c.ID
	a.ResourceURI = c.ResourceURI
	a.Plugin = c.Plugin
	if a.Plugin == "" {
		a.Plugin = "RedfishResource"
	}
	a.Headers = make(map[string]string, len(c.Headers))
	for k, v := range c.Headers {
		a.Headers[k] = v
	}

	a.PrivilegeMap = make(map[HTTPReqType]interface{}, len(c.Privileges))
	for k, v := range c.Privileges {
		a.PrivilegeMap[MapStringToHTTPReq(k)] = v
	}

	// ensure no collisions
	for _, p := range immutableProperties {
		delete(c.Properties, p)
	}

	d := &RedfishResourcePropertiesUpdatedData{
		ID:            c.ID,
		ResourceURI:   a.ResourceURI,
		PropertyNames: []string{},
	}
	e := &RedfishResourcePropertyMetaUpdatedData{
		ID:          c.ID,
		ResourceURI: a.ResourceURI,
		Meta:        map[string]interface{}{},
	}

	v := map[string]interface{}{}
	a.Properties.Value = v
	a.Properties.Parse(c.Properties)
	a.Properties.Meta = c.Meta

	var resourceURI []string
	// preserve slashes
	for _, x := range strings.Split(c.ResourceURI, "/") {
		resourceURI = append(resourceURI, url.PathEscape(x))
	}

	v["@odata.id"] = strings.Join(resourceURI, "/")
	v["@odata.type"] = c.Type
	v["@odata.context"] = c.Context

	// send out event that it's created first
	a.PublishEvent(eh.NewEvent(RedfishResourceCreated, &RedfishResourceCreatedData{
		ID:          c.ID,
		ResourceURI: c.ResourceURI,
	}, time.Now()))

	// then send out possible notifications about changes in the properties or meta
	if len(d.PropertyNames) > 0 {
		a.PublishEvent(eh.NewEvent(RedfishResourcePropertiesUpdated, d, time.Now()))
	}
	if len(e.Meta) > 0 {
		a.PublishEvent(eh.NewEvent(RedfishResourcePropertyMetaUpdated, e, time.Now()))
	}

	return nil
}

// RemoveRedfishResource Command
type RemoveRedfishResource struct {
	ID          eh.UUID `json:"id"`
	ResourceURI string  `eh:"optional"`
}

// AggregateType satisfies base Aggregate interface
func (c *RemoveRedfishResource) AggregateType() eh.AggregateType { return AggregateType }

// AggregateID satisfies base Aggregate interface
func (c *RemoveRedfishResource) AggregateID() eh.UUID { return c.ID }

// CommandType satisfies base Command interface
func (c *RemoveRedfishResource) CommandType() eh.CommandType { return RemoveRedfishResourceCommand }

func (c *RemoveRedfishResource) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	a.ResultsCacheMu.Lock()
	defer a.ResultsCacheMu.Unlock()
	a.PublishEvent(eh.NewEvent(RedfishResourceRemoved, &RedfishResourceRemovedData{
		ID:          c.ID,
		ResourceURI: a.ResourceURI,
	}, time.Now()))
	return nil
}

type RemoveRedfishResourceProperty struct {
	ID       eh.UUID `json:"id"`
	Property string  `eh:"optional"`
}

// AggregateType satisfies base Aggregate interface
func (c *RemoveRedfishResourceProperty) AggregateType() eh.AggregateType { return AggregateType }

// AggregateID satisfies base Aggregate interface
func (c *RemoveRedfishResourceProperty) AggregateID() eh.UUID { return c.ID }

// CommandType satisfies base Command interface
func (c *RemoveRedfishResourceProperty) CommandType() eh.CommandType {
	return RemoveRedfishResourcePropertyCommand
}
func (c *RemoveRedfishResourceProperty) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	a.ResultsCacheMu.Lock()
	defer a.ResultsCacheMu.Unlock()

	properties := a.Properties.Value.(map[string]interface{})
	for key, _ := range properties {
		if key == c.Property {
			delete(properties, key)
		}
	}
	return nil
}

type UpdateRedfishResourceProperties struct {
	ID         eh.UUID                `json:"id"`
	Properties map[string]interface{} `eh:"optional"`
}

// AggregateType satisfies base Aggregate interface
func (c *UpdateRedfishResourceProperties) AggregateType() eh.AggregateType { return AggregateType }

// AggregateID satisfies base Aggregate interface
func (c *UpdateRedfishResourceProperties) AggregateID() eh.UUID { return c.ID }

// CommandType satisfies base Command interface
func (c *UpdateRedfishResourceProperties) CommandType() eh.CommandType {
	return UpdateRedfishResourcePropertiesCommand
}
func (c *UpdateRedfishResourceProperties) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	a.ResultsCacheMu.Lock()
	defer a.ResultsCacheMu.Unlock()

	// ensure no collisions with immutable properties
	for _, p := range immutableProperties {
		delete(c.Properties, p)
	}

	d := &RedfishResourcePropertiesUpdatedData{
		ID:            c.ID,
		ResourceURI:   a.ResourceURI,
		PropertyNames: []string{},
	}
	e := &RedfishResourcePropertyMetaUpdatedData{
		ID:          c.ID,
		ResourceURI: a.ResourceURI,
		Meta:        map[string]interface{}{},
	}

	a.Properties.Parse(c.Properties)

	if len(d.PropertyNames) > 0 {
		a.PublishEvent(eh.NewEvent(RedfishResourcePropertiesUpdated, d, time.Now()))
	}
	if len(e.Meta) > 0 {
		a.PublishEvent(eh.NewEvent(RedfishResourcePropertyMetaUpdated, e, time.Now()))
	}

	return nil
}

type InjectEvent struct {
	ID          eh.UUID                  `json:"id" eh:"optional"`
	Name        eh.EventType             `json:"name"`
	Synchronous bool                     `eh:"optional"`
	EventData   map[string]interface{}   `json:"data" eh:"optional"`
	EventArray  []map[string]interface{} `json:"event_array" eh:"optional"`
}

// AggregateType satisfies base Aggregate interface
func (c *InjectEvent) AggregateType() eh.AggregateType { return AggregateType }

// AggregateID satisfies base Aggregate interface
func (c *InjectEvent) AggregateID() eh.UUID { return c.ID }

// CommandType satisfies base Command interface
func (c *InjectEvent) CommandType() eh.CommandType {
	return InjectEventCommand
}

var injectChan chan eh.Event

func StartInjectService(logger log.Logger, d *DomainObjects) {
	injectChan = make(chan eh.Event, 100)
	logger = logger.New("module", "injectservice")
	eb := d.EventBus
	ew := d.EventWaiter

	var s closeNotifier
	s, err := NewSdnotify()
	if err != nil {
		fmt.Printf("Error setting up SD_NOTIFY, using simulation instead: %s\n", err)
		s = SimulateSdnotify()
	}

	if interval := s.GetIntervalUsec(); interval == 0 {
		fmt.Printf("Watchdog interval is not set, so skipping watchdog setup. Set WATCHDOG_USEC to set.\n")
	} else {
		fmt.Printf("Setting up watchdog\n")

		// send watchdogs 3x per interval
		interval = interval / 3

		// set up listener for the watchdog events
		listener, err := ew.Listen(context.Background(), func(event eh.Event) bool {
			if event.EventType() == WatchdogEvent {
				return true
			}
			return false
		})

		if err != nil {
			panic("Could not start listener")
		}

		// goroutine to run sd_notify whenever we see a watchdog event
		go func() {
			defer s.Close()
			for {
				_, err := listener.Wait(context.Background())
				if err != nil {
					fmt.Printf("Watchdog wait exited\n")
					break
				}

				s.Notify("WATCHDOG=1")
				d.CheckTree()
			}
		}()

		// goroutine to inject watchdog events
		go func() {
			// inject a watchdog event every 10s. It will be processed by a listener elsewhere.
			for {
				time.Sleep(time.Duration(interval) * time.Microsecond)
				data, err := eh.CreateEventData("WatchdogEvent")
				if err != nil {
					injectChan <- eh.NewEvent(WatchdogEvent, data, time.Now())
				}
			}
		}()
	}

	// goroutine to synchronously handle the event inject queue
	go func() {
		startPrinting := false
		for {
			event := <-injectChan
			// if (len(injectChan)*10)/(cap(injectChan)*10) > 5 {
			if len(injectChan) > 20 {
				startPrinting = true
			}
			if startPrinting {
				logger.Debug("Inject chan congestion", "cap", cap(injectChan), "len", len(injectChan))
			}
			if len(injectChan) == 0 {
				startPrinting = false
			}

			eb.PublishEvent(context.Background(), event) // in a goroutine (comment for grep purposes)

			// if we get a sync event, we have to pause processing new events until it's completed processing
			// it may be processing in the background
			if ev, ok := event.(syncEvent); ok {
				ev.Wait()
			}
		}
	}()
}

const MAX_CONSOLIDATED_EVENTS = 10
const injectUUID = eh.UUID("49467bb4-5c1f-473b-0000-00000000000f")

func (c *InjectEvent) Handle(ctx context.Context, a *RedfishResourceAggregate) error {
	requestLogger := ContextLogger(ctx, "internal_commands").New("module", "inject_event")

	a.ID = injectUUID

	eventList := make([]map[string]interface{}, 0, len(c.EventArray)+1)
	if len(c.EventData) > 0 {
		// comment out debug prints in the hot path, uncomment for debugging
		//requestLogger.Debug("InjectEvent - ONE", "events", c.EventData)
		eventList = append(eventList, c.EventData) // preallocated
	}
	if len(c.EventArray) > 0 {
		// comment out debug prints in the hot path, uncomment for debugging
		//requestLogger.Debug("InjectEvent - ARRAY", "events", c.EventArray)
		eventList = append(eventList, c.EventArray...) // preallocated
	}

	// comment out debug prints in the hot path, uncomment for debugging
	//requestLogger.Debug("InjectEvent - NEW ARRAY INJECT", "events", eventList)

	//debugTrain := false
	//if len(eventList) >= MAX_CONSOLIDATED_EVENTS {
	//fmt.Printf("Event list (%s) len (%d) greater than max number of events (%d). Going to break into #(%d) chunks\n", c.Name, len(eventList), MAX_CONSOLIDATED_EVENTS, len(eventList)/MAX_CONSOLIDATED_EVENTS)
	//debugTrain = true
	//}

	trainload := make([]eh.EventData, 0, MAX_CONSOLIDATED_EVENTS)
	for _, eventData := range eventList {
		data, err := eh.CreateEventData(c.Name)
		if err != nil {
			// this debug statement probably not hit too often, leave enabled for now
			requestLogger.Info("InjectEvent - event type not registered: injecting raw event.", "event name", c.Name, "error", err)
			trainload = append(trainload, eventData) //preallocated
		} else {
			err = mapstructure.Decode(eventData, &data)
			if err != nil {
				requestLogger.Warn("InjectEvent - could not decode event data, skipping event", "error", err, "raw-eventdata", eventData, "dest-event", data)
				trainload = append(trainload, eventData) //preallocated
			} else {
				trainload = append(trainload, data) //preallocated
			}
		}
		// comment out debug prints in the hot path, uncomment for debugging
		//requestLogger.Debug("InjectEvent - publishing", "event name", c.Name, "event_data", data)

		// limit number of consolidated events to 30 to prevent overflowing queues and deadlocking
		if len(trainload) >= MAX_CONSOLIDATED_EVENTS {
			//fmt.Printf("Train (%s) leaving early: %d\n", c.Name, len(trainload))
			e := event.NewSyncEvent(c.Name, trainload, time.Now())
			e.Add(1)
			if c.Synchronous {
				defer e.Wait()
			}
			trainload = make([]eh.EventData, 0, MAX_CONSOLIDATED_EVENTS)
			injectChan <- e
		}
	}

	if len(trainload) > 0 {
		//if debugTrain {
		//fmt.Printf("Straggler (%s) roundup: #%d events\n", c.Name, len(trainload))
		//}
		e := event.NewSyncEvent(c.Name, trainload, time.Now())
		e.Add(1)
		if c.Synchronous {
			defer e.Wait()
		}
		injectChan <- e
	}

	return nil
}
