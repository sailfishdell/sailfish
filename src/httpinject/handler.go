package httpinject

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	eh "github.com/looplab/eventhorizon"
	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/event"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
)

// some constants... many of these should be read from config
const (
	watchdogsPerInterval  = 3
	maxOOOMessages        = 25
	outOfOrderTimeout     = 6 * time.Second
	maxConsolidatedEvents = 5
	maxUint               = ^int32(0)
	maxQueuedInjectEvents = 50
)

type busObjs interface {
	GetBus() eh.EventBus
	GetWaiter() *eventwaiter.EventWaiter
	GetPublisher() eh.EventPublisher
}

type InjectCommand struct {
	sync.WaitGroup
	ctx        context.Context
	sendTime   time.Time
	ingestTime time.Time

	EventSeq     int64             `json:"event_seq"`
	EventData    json.RawMessage   `json:"data"`
	EventArray   []json.RawMessage `json:"event_array"`
	ID           eh.UUID           `json:"id"`
	Name         eh.EventType      `json:"name"`
	Encoding     string            `json:"encoding"`
	Barrier      bool              `json:"barrier"`     // EventBarrier is set if this event should block subsequent events until it is processed
	Synchronous  bool              `json:"Synchronous"` // Synchronous set if POST should not return until the message is processed
	PumpSendTime int64             `json:"PumpSendTime"`
}

type eventBundle struct {
	event   *event.SyncEvent
	barrier bool
}

type service struct {
	logger         log.Logger
	sd             sdNotifier
	eb             eh.EventBus
	ew             *eventwaiter.EventWaiter
	injectCmdQueue chan *InjectCommand
	injectChan     chan *eventBundle
}

func (s *service) GetCommandCh() chan *InjectCommand {
	return s.injectCmdQueue
}

func NewInjectCommand() *InjectCommand {
	return &InjectCommand{
		ctx:        context.Background(),
		ingestTime: time.Now(),
	}
}


func (cmd *InjectCommand) SetPumpSendTime() {
	if cmd.PumpSendTime < int64(maxUint) {
		cmd.sendTime = time.Unix(cmd.PumpSendTime, 0)
	} else {
		cmd.sendTime = time.Unix(0, cmd.PumpSendTime)
	}
}

var reglock = sync.Once{}


func New(logger log.Logger, d busObjs) (svc *service) {
	reglock.Do(func() {
		eh.RegisterEventData(WatchdogEvent, func() eh.EventData { return &WatchdogEventData{} })
		eh.RegisterEventData(DroppedEvent, func() eh.EventData { return &DroppedEventData{} })
	})

	svc = &service{
		logger: logger.New("module", "injectservice"),
		eb:     d.GetBus(),
		ew:     d.GetWaiter(),
		// if things wedge here, making this queue longer wont do anything useful, so by default make it fully synchronous.
		injectCmdQueue: make(chan *InjectCommand),
		// everything here is sorted, it's ok to have this be a little longer, as it slows things down if this ever empties
		// not too big, though, or our max latency takes a big hit
		injectChan: make(chan *eventBundle, maxQueuedInjectEvents),
	}

	var err error
	svc.sd, err = NewSdnotify()
	if err != nil {
		logger.Warn("Running using simulation SD_NOTIFY", "err", err)
		svc.sd = SimulateSdnotify()
	}

	return
}

func (s *service) Ready() {
	s.sd.SDNotify("READY=1")
}

func (s *service) watchdog() {
	defer s.sd.Close()
	interval := s.sd.GetIntervalUsec()
	if interval == 0 {
		interval = 30000000
	}

	// send watchdogs 3x per interval
	interval /= watchdogsPerInterval
	seq := 0

	s.logger.Info("Setting up watchdog.", "interval-in-milliseconds", interval)

	// set up listener for the watchdog events
	listener, err := s.ew.Listen(context.Background(), func(event eh.Event) bool {
		return event.EventType() == WatchdogEvent
	})

	if err != nil {
		panic("Could not start listener")
	}

	// endless loop generating and responding to watchdog events
	watchdogTicker := time.NewTicker(time.Duration(interval) * time.Microsecond)
	defer watchdogTicker.Stop()
	for {
		select {
		// pet watchdog when we get an event
		case ev := <-listener.Inbox():
			if evtS, ok := ev.(event.SyncEvent); ok {
				evtS.Done()
			}
			s.sd.SDNotify("WATCHDOG=1")

		// periodically send event on bus to force watchdog
		case <-watchdogTicker.C:
			evt := event.NewSyncEvent(WatchdogEvent, &WatchdogEventData{Seq: seq}, time.Now())
			evt.Add(1)
			// use watchdogs with barrier set to periodically clean the queues out
			s.injectChan <- &eventBundle{&evt, true}
			seq++
		}
	}
}

func (s *service) handleInjectQueue() {
	for {
		select {
		case cmd := <-s.injectCmdQueue:
			cmd.MarkBarrier()
			cmd.sendToChn(s.injectChan)
		}
	}
}

func (s *service) sentEventsToInternalBus() {
	for evb := range s.injectChan {
		s.eb.PublishEvent(context.Background(), *evb.event)
		// barrier is set if this event should block events after it
		if evb.barrier {
			evb.event.Wait()
		}
	}
}

func (s *service) Start() {
	// This service starts three (3) goroutines
	//
	// The first is a watchdog goroutine that sends events and then receives its
	// own events to ping the systemd watchdog
	go s.watchdog()

	// The second gets the raw inject commands from HTTP and tries to ensure that
	// they are in the correct order before sending them on the event bus
	go s.handleInjectQueue()

	// The third takes the ordered inject events and publishes them on the
	// internal event bus. it also is responsible for ensuring that event
	// barriers are respected.
	go s.sentEventsToInternalBus()
}

type Decoder interface {
	Decode(d map[string]interface{}) error
}

// MarkBarrier will mark specific events as barrier events, ie. that they
// prevent any events from being added behind it in the queue until it has been
// fully processed
//
// This is somewhat arbitrary and is domain-specific knowledge
//
func (c *InjectCommand) MarkBarrier() {
	switch c.Name {
	// can create objects that are needed by subsequent events
	case "ComponentEvent",
		"LogEvent",
		"FaultEntryAdd":
		c.Barrier = true

		// force caller synchronous because these can take significant time
		c.Synchronous = true

	case "AttributeUpdated":
		// these can overwhelm, but want to process quickly
		c.Barrier = false
		if c.EventSeq%2 == 0 {
			c.Barrier = true
		}

	case "AvgPowerConsumptionStatDataObjEvent",
		"FileReadEvent",
		"FanEvent",
		"PowerConsumptionDataObjEvent",
		"PowerSupplyObjEvent",
		"TemperatureHistoryEvent",
		"ThermalSensorEvent",
		"thp_fan_data_object":
		c.Barrier = false

	// rare events, or events that can't arrive quickly
	case "HealthEvent", "IomCapability":
		c.Barrier = false

	default:
		c.Barrier = true
	}
}

func (c *InjectCommand) sendToChn(injectChan chan *eventBundle) error {
	//requestLogger := log.ContextLogger(c.ctx, "internal_commands").New("module", "inject_event")
	//requestLogger.Crit("InjectService: preparing event", "Sequence", c.EventSeq, "Name", c.Name)

	waits := []func(){}
	defer func() {
		for _, fn := range waits {
			// These are a queue of .Wait() for individual internal Published events.
			// If the command is Synchronous=true, then these are added. These will
			// cause the .Done() for the command that queued these events (above) to
			// not be marked complete until the events are fully processed.
			//
			// If the command is Synchronous, that means that after the HTTP POST has
			// returned, caller knows that the event has been fully processed by all
			// goroutines that are listening for it.
			fn()
		}

		// run the Command .Done() after we've sent all the commands from the "command" queue to the "event" queue (but not yet published).
		// After the HTTP POST has returned, caller knows that this event is being processed "in order", but might not yet be finished.
		c.Done()
	}()

	totalTrains := 0
	doneTrains := 0
	waitForEvent := func(evt event.SyncEvent) func() {
		return func() {
			doneTrains++
			if c.Synchronous {
				evt.Wait()
				// UNCOMMENT THE LINES HERE TO GET COMPREHENSIVE METRICS FOR TIMINGS FOR PROCESSING EACH EVENT
				// We should do Prometheus metrics RIGHT HERE
				//fmt.Printf("\tevent %s %d#%d/%d DONE:  ingest: %s  total: %s\n", c.Name, c.EventSeq, totalTrains, doneTrains, time.Now().Sub(c.ingestTime), time.Now().Sub(c.sendTime))
				//} else {
				// spawn a goroutine to wait for processing to complete since caller declines to wait.
				//go func(t, d int) {
				//	evt.Wait()
				// AND We should do Prometheus metrics RIGHT HERE
				//	fmt.Printf("\tevent %s %d#%d/%d DONE:  ingest: %s  total: %s\n", c.Name, c.EventSeq, totalTrains, doneTrains, time.Now().Sub(c.ingestTime), time.Now().Sub(c.sendTime))
				//}(totalTrains, doneTrains)
			}
		}
	}

	trainload := make([]eh.EventData, 0, maxConsolidatedEvents)
	sendTrain := func([]eh.EventData) {
		if len(trainload) == 0 {
			return
		}

		evt := event.NewSyncEvent(c.Name, trainload, time.Now())
		evt.Add(1)
		select {
		case injectChan <- &eventBundle{&evt, c.Barrier}:
			// make sure we don't add the .Wait() until after we know it's being
			// processed by the other side. Otherwise the context cancel (below, the
			// case <-c.ctx.Done()) will keep the message from being sent from our
			// side, and then we'll .Wait() for something that can never be .Done()
			totalTrains++
			waits = append(waits, waitForEvent(evt))
		case <-c.ctx.Done():
			//requestLogger.Info("CONTEXT CANCELLED! Discarding trainload", "err", c.ctx.Err(), "trainload", trainload, "EventName", c.Name)
		}
	}

	// accumulate decode events in trainload slice, then send as it gets full
	c.appendDecode(&trainload, c.Name, c.EventData)
	for _, d := range c.EventArray {
		c.appendDecode(&trainload, c.Name, d)
		if len(trainload) >= maxConsolidatedEvents {
			sendTrain(trainload)
			trainload = make([]eh.EventData, 0, maxConsolidatedEvents)
		}
	}
	// finally, send the final (partial) load
	sendTrain(trainload)

	return nil
}

func (c *InjectCommand) appendDecode(trainload *[]eh.EventData, eventType eh.EventType, m json.RawMessage) {
	requestLogger := log.ContextLogger(c.ctx, "internal_commands").New("module", "inject_event")
	if m == nil {
		// not worth logging unless debugging something weird
		// requestLogger.Info("Decode: nil message", "eventType", eventType)
		return
	}
	// create a new, empty event of the requested type. The data will be deserialized into it.
	data, err := eh.CreateEventData(eventType)
	if err != nil {
		requestLogger.Info("Decode: fallback to map[string]interface{}", "eventType", eventType, "err", err)
		data = map[string]interface{}{}
	}

	// check if event wants to deserialize itself with a custom decoder
	// this will handle DM objects
	if ds, ok := data.(Decoder); ok {
		eventData := map[string]interface{}{}
		err := json.Unmarshal(m, &eventData)
		if err != nil {
			requestLogger.Warn("Decode: unmarshal rawmessage failed", "err", err)
			return
		}

		err = ds.Decode(eventData)
		if err != nil {
			// failed decode, just send the raw map[string]interface data
			requestLogger.Warn("Custom Decode error, send data as map[string]interface{}", "err", err, "EventName", c.Name)
			*trainload = append(*trainload, eventData) //preallocated
			return
		}
		*trainload = append(*trainload, data) //preallocated
		// fast path, avoid logging unless debugging
		//requestLogger.Debug("Decode: added to trainload", "data", data)
		return
	}

	err = json.Unmarshal(m, &data)
	if err != nil {
		requestLogger.Warn("Decode message: unmarshal rawmessage failed", "err", err, "RawMessage", string(m))
		return
	}
	// fast path, avoid logging unless debugging
	//requestLogger.Debug("Decode: normal json decode added to trainload", "data", data)
	*trainload = append(*trainload, data)
}
