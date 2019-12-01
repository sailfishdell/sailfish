package http_inject

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"sync"
	"time"

	eh "github.com/looplab/eventhorizon"
	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/event"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
)

const MAX_QUEUED = 10

type waiter interface {
	Listen(context.Context, func(eh.Event) bool) (*eventwaiter.EventListener, error)
	Notify(context.Context, eh.Event)
}

type busObjs interface {
	GetBus() eh.EventBus
	GetWaiter() *eventwaiter.EventWaiter
	GetPublisher() eh.EventPublisher
}

type InjectCommand struct {
	sync.WaitGroup
	ctx context.Context

	EventSeq    int64             `json:"event_seq"`
	EventData   json.RawMessage   `json:"data"`
	EventArray  []json.RawMessage `json:"event_array"`
	ID          eh.UUID           `json:"id"`
	Name        eh.EventType      `json:"name"`
	Encoding    string            `json:"encoding"`
	Barrier     bool              `json:"barrier"`     // EventBarrier is set if this event should block subsequent events until it is processed
	Synchronous bool              `json:"Synchronous"` // Synchronous set if POST should not return until the message is processed
}

type eventBundle struct {
	event   *event.SyncEvent
	barrier bool
}

var injectCmdQueue chan *InjectCommand
var injectChan chan *eventBundle

// inject event timeout
//var IETIMEOUT time.Duration = 250 * time.Millisecond
var IETIMEOUT time.Duration = 2 * time.Second

type service struct {
	logger log.Logger
	sd     sdNotifier
	eb     eh.EventBus
	ew     *eventwaiter.EventWaiter
}

// SSEHandler struct holds authentication/authorization data as well as the domain variables
type InjectHandler struct {
	UserName   string
	Privileges []string
	d          busObjs
	logger     log.Logger
}

// NewInjectHandler constructs a new InjectHandler with the given username and privileges.
func NewInjectHandler(dobjs busObjs, logger log.Logger, u string, p []string) *InjectHandler {
	return &InjectHandler{UserName: u, Privileges: p, d: dobjs, logger: logger}
}

func (rh *InjectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID := eh.NewUUID()
	ctx := WithRequestID(r.Context(), requestID)
	requestLogger := ContextLogger(ctx, "INJECT")

	// set headers first
	w.Header().Set("OData-Version", "4.0")
	w.Header().Set("Server", "sailfish")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Cache-Control", "no-Store,no-Cache")
	w.Header().Set("Pragma", "no-cache")

	// security headers
	w.Header().Add("Strict-Transport-Security", "max-age=63072000; includeSubDomains") // for A+ SSL Labs score
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	w.Header().Set("X-Content-Security-Policy", "default-src 'self'")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// compatibility headers
	w.Header().Set("X-UA-Compatible", "IE=11")

	// TODO: query option for extra debug print

	cmd := &InjectCommand{}

	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(cmd)
	if err != nil {
		requestLogger.Crit("JSON decode failure", "err", err)
		http.Error(w, "could not JSON decode command: "+err.Error(), http.StatusBadRequest)
		return
	}

	// ideally, this would all be handled internally by auto-backpressure on the internal queue, but we're not set up to examine the internal queues yet
	//requestLogger.Debug("PUSH injectCmdQueue LEN", "len", len(injectCmdQueue), "cap", cap(injectCmdQueue), "module", "inject", "cmd", cmd)
	sync := cmd.Synchronous
	if sync {
		// cancel the send of event through the stack if HTTP request is cancelled
		cmd.ctx = ctx
	} else {
		// otherwise just let it be background
		cmd.ctx = context.Background()
	}

	// manually override barrier settings given by sender in some cases
	cmd.markBarrier()

	// don't do anything with "cmd" *at all* in this go-routine, except .Wait() after this .Add(1)
	// That means do not access any structure members or anything except .Wait()
	cmd.Add(1)
	injectCmdQueue <- cmd

	// For cmd.Synchronous==false: this will either wait until event has made it
	// through inital command queue and is being processed "in order"(*)
	//
	// For cmd.Synchronous==true: this will either wait until event has made it
	// through inital command queue and all subsequent events that it spawns have
	// been fully processed.
	if sync {
		cmd.Wait()
	}

	w.WriteHeader(http.StatusOK)
}

func New(logger log.Logger, d busObjs) (svc *service) {
	injectCmdQueue = make(chan *InjectCommand, MAX_QUEUED+1)
	injectChan = make(chan *eventBundle)

	svc = &service{
		logger: logger.New("module", "injectservice"),
		sd:     SimulateSdnotify(),
		eb:     d.GetBus(),
		ew:     d.GetWaiter(),
	}

	s, err := NewSdnotify()
	if err != nil {
		logger.Warn("Error setting up SD_NOTIFY, using simulation instead", "err", err)
		return
	}
	svc.sd = s

	return
}

func (s *service) Ready() {
	s.sd.SDNotify("READY=1")
}

func (s *service) Start() {
	// This service starts three (3) goroutines
	//
	// The first is a watchdog goroutine that sends events and then receives its
	// own events to ping the systemd watchdog
	//
	// The second gets the raw inject commands from HTTP and tries to ensure that
	// they are in the correct order before sending them on the event bus
	//
	// The third takes the ordered inject events and publishes them on the
	// internal event bus. it also is responsible for ensuring that event
	// barriers are respected.

	go func() {
		defer s.sd.Close()
		interval := s.sd.GetIntervalUsec()
		if interval == 0 {
			interval = 30000000
		}

		// send watchdogs 3x per interval
		interval = interval / 3
		seq := 0

		s.logger.Info("Setting up watchdog.", "interval", time.Duration(interval)*time.Microsecond)

		// set up listener for the watchdog events
		listener, err := s.ew.Listen(context.Background(), func(event eh.Event) bool {
			if event.EventType() == WatchdogEvent {
				return true
			}
			return false
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
				injectChan <- &eventBundle{&evt, true}
				seq++
			}
		}
	}()

	// goroutine to synchronously handle the event inject queue
	go func() {
		queued := make([]*InjectCommand, 0, MAX_QUEUED+1)
		internalSeq := int64(0)
		// The 'standard' way to create a stopped timer
		sequenceTimer := time.NewTimer(math.MaxInt64)
		if !sequenceTimer.Stop() {
			<-sequenceTimer.C
		}
		timerActive := false

		tryToPublish := func(forceSend bool) {
			// iterate through our queue until we find a message beyond our current sequence, then stop
			i := 0
			for i = len(queued) - 1; i >= 0; i-- {
				injectCmd := queued[i]
				// fast path debug statement. comment out unless actively debugging
				//s.logger.Info("PROCESS QUEUE EVENT", "seq", internalSeq, "cmd", injectCmd.Name, "cmdseq", injectCmd.EventSeq, "index", i)

				// force resync on event with '0' seq or less
				if injectCmd.EventSeq < 1 {
					s.logger.Debug("Event sent which forced queue resync", "seq", internalSeq, "cmd", injectCmd.Name, "cmdseq", injectCmd.EventSeq, "index", i)
					internalSeq = injectCmd.EventSeq
				}

				if injectCmd.EventSeq < internalSeq {
					s.logger.Crit("Dropped out-of sequence message", "seq", internalSeq, "cmd", injectCmd.Name, "cmdseq", injectCmd.EventSeq, "index", i)
					// event is older than last published event, drop
					evt := event.NewSyncEvent(DroppedEvent, &DroppedEventData{
						Name:     injectCmd.Name,
						EventSeq: injectCmd.EventSeq,
					}, time.Now())
					evt.Add(1)
					injectChan <- &eventBundle{&evt, false}
					queued[i] = nil
					continue
				}

				// if the seq is correct, send it
				//  or if internal seq has been reset, send and take the identity of that seq
				doSend := false
				if injectCmd.EventSeq == internalSeq || internalSeq <= 0 {
					doSend = true
				} else if forceSend {
					// force up to one event to be sent out of order
					forceSend = false
					doSend = true
				}

				if doSend {
					// fast path debug statement. comment out unless actively debugging
					//	s.logger.Debug("Send", "seq", internalSeq, "cmd", injectCmd.Name, "cmdseq", injectCmd.EventSeq, "index", i)
					injectCmd.sendToChn()
					internalSeq = injectCmd.EventSeq
					internalSeq++
					queued[i] = nil
					continue
				}

				break //  injectCmd.EventSeq > internalSeq, no sense going through the rest
			}

			// trim off any processed commands
			queued = queued[:i+1]
		}

		for {
			select {
			case cmd := <-injectCmdQueue:
				// fast path debug statement. comment out unless actively debugging
				//s.logger.Debug("POP  injectCmdQueue LEN", "len", len(injectCmdQueue), "cap", cap(injectCmdQueue), "module", "inject", "cmdname", cmd.Name)
				queued = append(queued, cmd)
				if len(queued) > 1 {
					sort.SliceStable(queued, func(i, j int) bool {
						return queued[i].EventSeq > queued[j].EventSeq
					})
					// fast path debug statement. comment out unless actively debugging
					//s.logger.Info("SOME STUFF QUEUED!", "len", len(queued), "FIRST_SEQ", queued[len(queued)-1].EventSeq)
				}

				if len(queued) < 1 {
					panic("Somehow we added a command to the queue but now have nothing in the queue. This can't happen.")
					break
				}

				// queue is sorted, so first event seq can be checked
				//   any events less than or equal to internalSeq can be dealt with
				//   either by dropping them or sending them
				if queued[len(queued)-1].EventSeq <= internalSeq {
					// fast path debug statement. comment out unless actively debugging
					//s.logger.Debug("Stuff to publish!")
					tryToPublish(false)
				} else {
					s.logger.Debug("OUT OF ORDER", "internalseq", internalSeq, "msgseq", queued[len(queued)-1].EventSeq)
				}

				// Don't allow the out of order queue to get too big
				// Force out the first entry if it's over
				if len(queued) > MAX_QUEUED {
					// force publish
					s.logger.Info("Queue exceeds max len, force publish. Implied missing or out of order events present.")
					tryToPublish(true)
				}

				// oops, we have some left, start a new timer
				if len(queued) > 0 && !timerActive {
					// SEMI-fast path debug statement. comment out unless actively debugging
					//s.logger.Debug("OUT OF ORDER: Set timer to empty queue!")
					sequenceTimer.Reset(IETIMEOUT)
					timerActive = true
					break // no need to test subsequent if statements, they can't be true
				}

				// we got everything, stop any timers
				if len(queued) == 0 && timerActive {
					// SEMI-fast path debug statement. comment out unless actively debugging
					//s.logger.Debug("STOP TIMER. queue empty")
					if !sequenceTimer.Stop() {
						<-sequenceTimer.C
					}
					timerActive = false
				}

			case <-sequenceTimer.C:
				warnstr := fmt.Sprintf("TIMEOUT waiting for event sequence %d. QUEUE: ", internalSeq)
				for i, q := range queued {
					warnstr += fmt.Sprintf(" IDX(%d)/SEQ(%d)", i, q.EventSeq)
				}
				s.logger.Warn(warnstr)

				timerActive = false
				internalSeq = 0    // force sync to first message
				tryToPublish(true) // Force the first message out

				// we have some left, start a new timer
				if len(queued) > 0 && !timerActive {
					s.logger.Debug("OUT OF ORDER: Set timer to empty queue!")
					sequenceTimer.Reset(IETIMEOUT)
					timerActive = true
				}
			}
		}
	}()

	go func() {
		debugOutputTicker := time.NewTicker(5 * time.Second)
		for {
			select {
			case evb := <-injectChan:
				s.eb.PublishEvent(context.Background(), *evb.event)
				// barrier is set if this event should block events after it
				if evb.barrier {
					evb.event.Wait()
				}
			case <-debugOutputTicker.C:
				// debug if we start getting full channels
				s.logger.Debug("queue length monitor", "module", "queuemonitor", "len(injectChan)", len(injectChan), "len(injectChan)", cap(injectChan), "len(injectCmdQueue)", len(injectCmdQueue), "cap(injectCmdQueue)", cap(injectCmdQueue))
			}
		}
	}()

}

const MAX_CONSOLIDATED_EVENTS = 5

type Decoder interface {
	Decode(d map[string]interface{}) error
}

// markBarrier will mark specific events as barrier events, ie. that they
// prevent any events from being added behind it in the queue until it has been
// fully processed
//
// This is somewhat arbitrary and is domain-specific knowledge
//
func (c *InjectCommand) markBarrier() {
	switch c.Name {
	// can create objects that are needed by subsequent events
	case "ComponentEvent",
		"LogEvent",
		"FaultEntryAdd":
		c.Barrier = true

	// these can overwhelm, but want to process quickly
	case "AttributeUpdated":
		// just a swag: barrier every 5th one
		c.Barrier = false
		if c.EventSeq%5 == 0 {
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

func (c *InjectCommand) sendToChn() error {
	requestLogger := ContextLogger(c.ctx, "internal_commands").New("module", "inject_event")
	//requestLogger.Crit("InjectService: preparing event", "Sequence", c.EventSeq, "Name", c.Name)

	waits := []func(){}
	defer func() {
		// run the Command .Done() after we've sent all the commands from the "command" queue to the "event" queue (but not yet published).
		// After the HTTP POST has returned, caller knows that this event is being processed "in order", but might not yet be finished.
		defer c.Done()
		for _, fn := range waits {
			// These are a queue of .Wait() for individual internal Published events.
			// If the command is Synchronous=true, then these are added. These will
			// cause the .Done() for the command that queued these events (above) to
			// not be marked complete until the events are fully processed.
			//
			// If the command is Synchronous, that means that after the HTTP POST has
			// returned, caller knows that the event has been fully processed by all
			// goroutines that are listening for it.
			defer fn()
		}
	}()

	trainload := make([]eh.EventData, 0, MAX_CONSOLIDATED_EVENTS)
	sendTrain := func([]eh.EventData) {
		if len(trainload) == 0 {
			return
		}

		evt := event.NewSyncEvent(c.Name, trainload, time.Now())
		evt.Add(1)
		if c.Synchronous {
			waits = append(waits, evt.Wait)
		}
		select {
		case injectChan <- &eventBundle{&evt, c.Barrier}:
		case <-c.ctx.Done():
			requestLogger.Info("CONTEXT CANCELLED! Discarding trainload", "err", c.ctx.Err(), "trainload", trainload, "EventName", c.Name)
		}
	}

	// accumulate decode events in trainload slice, then send as it gets full
	c.appendDecode(requestLogger, &trainload, c.Name, c.EventData)
	for _, d := range c.EventArray {
		c.appendDecode(requestLogger, &trainload, c.Name, d)
		if len(trainload) >= MAX_CONSOLIDATED_EVENTS {
			sendTrain(trainload)
			trainload = make([]eh.EventData, 0, MAX_CONSOLIDATED_EVENTS)
		}
	}
	// finally, send the final (partial) load
	sendTrain(trainload)

	return nil
}

func (c *InjectCommand) appendDecode(requestLogger log.Logger, trainload *[]eh.EventData, eventType eh.EventType, m json.RawMessage) {
	if m == nil {
		// not worth logging unless debugging something wierd
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
	return
}
