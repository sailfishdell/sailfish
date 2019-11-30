package http_inject

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
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

type waiter interface {
	Listen(context.Context, func(eh.Event) bool) (*eventwaiter.EventListener, error)
	Notify(context.Context, eh.Event)
}

type busObjs interface {
	GetBus() eh.EventBus
	GetWaiter() *eventwaiter.EventWaiter
	GetPublisher() eh.EventPublisher
}

// NewInjectHandler constructs a new InjectHandler with the given username and privileges.
func NewInjectHandler(dobjs busObjs, logger log.Logger, u string, p []string) *InjectHandler {
	return &InjectHandler{UserName: u, Privileges: p, d: dobjs, logger: logger}
}

// SSEHandler struct holds authentication/authorization data as well as the domain variables
type InjectHandler struct {
	UserName   string
	Privileges []string
	d          busObjs
	logger     log.Logger
}

func (rh *InjectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID := eh.NewUUID()
	ctx := WithRequestID(r.Context(), requestID)
	requestLogger := ContextLogger(ctx, "INJECT")

	// TODO: query option for extra debug print

	b, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		requestLogger.Crit("Could not read event", "err", err)
		http.Error(w, "could not read command: "+err.Error(), http.StatusBadRequest)
		return
	}

	cmd := &InjectEvent{ctx: ctx}
	contentType := r.Header.Get("Content-type")
	if contentType == "application/xml" {
		if err := xml.Unmarshal(b, &cmd); err != nil {
			requestLogger.Crit("XML decode failure", "err", err, "body", b)
			http.Error(w, "could not XML decode command: "+err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		if err := json.Unmarshal(b, &cmd); err != nil {
			requestLogger.Crit("JSON decode failure", "err", err, "body", b)
			http.Error(w, "could not JSON decode command: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	cmd.Add(1)
	if 100*len(injectChanSlice)/cap(injectChanSlice) > 50 {
		requestLogger.Debug("PUSH injectChanSlice LEN", "len", len(injectChanSlice), "cap", cap(injectChanSlice), "module", "inject")
	}
	injectChanSlice <- cmd

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

	if cmd.Synchronous {
		cmd.Wait()
	}
}

type InjectEvent struct {
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
	event   event.SyncEvent
	barrier bool
}

var injectChanSlice chan *InjectEvent
var injectChan chan *eventBundle

// inject event timeout
var IETIMEOUT time.Duration = 250 * time.Millisecond

type service struct {
	logger log.Logger
	sd     sdNotifier
	eb     eh.EventBus
	ew     *eventwaiter.EventWaiter
}

func New(logger log.Logger, d busObjs) (svc *service) {
	injectChanSlice = make(chan *InjectEvent, 100)
	injectChan = make(chan *eventBundle, 10)

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
	go func() {
		defer s.sd.Close()
		interval := s.sd.GetIntervalUsec()
		if interval == 0 {
			s.logger.Crit("Watchdog interval is not set, so skipping watchdog setup. Set WATCHDOG_USEC to set.")
			return
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
		for {
			select {
			// pet watchdog when we get an event
			case ev := <-listener.Inbox():
				if evtS, ok := ev.(event.SyncEvent); ok {
					evtS.Done()
				}
				s.sd.SDNotify("WATCHDOG=1")

			// periodically send event on bus to force watchdog
			case <-time.After(time.Duration(interval) * time.Microsecond):
				ev := event.NewSyncEvent(WatchdogEvent, &WatchdogEventData{Seq: seq}, time.Now())
				ev.Add(1)
				// use watchdogs to clean out cruft. Maybe a good idea, not sure.
				injectChan <- &eventBundle{ev, true}
				seq++
			}
		}
	}()

	// goroutine to synchronously handle the event inject queue
	go func() {
		queued := []*InjectEvent{}
		internalSeq := int64(0)
		// The 'standard' way to create a stopped timer
		sequenceTimer := time.NewTimer(math.MaxInt64)
		if !sequenceTimer.Stop() {
			<-sequenceTimer.C
		}
		timerActive := false

		tryToPublish := func(tryHard bool) {
			// iterate through our queue until we find a message beyond our current sequence, then stop
			i := 0
			for i = 0; i < len(queued); i++ {
				injectCmd := queued[i]

				// force resync on event with '0' seq or less
				if injectCmd.EventSeq < 1 {
					internalSeq = injectCmd.EventSeq
				}

				if injectCmd.EventSeq < internalSeq {
					// event is older than last published event, drop
					ev := event.NewSyncEvent(DroppedEvent, &DroppedEventData{
						Name:     injectCmd.Name,
						EventSeq: injectCmd.EventSeq,
					}, time.Now())
					ev.Add(1)
					injectChan <- &eventBundle{ev, false}
					continue
				}

				// if the seq is correct, send it
				//  or if internal seq has been reset, send and take the identity of that seq
				//  or if we are in a "force" send all events, send it.
				if injectCmd.EventSeq == internalSeq || internalSeq == 0 || tryHard {
					injectCmd.sendToChn()
					// command with seq == -1 will "reset". The counter increments to 0 and the next event becomes the new starting sequence
					internalSeq = injectCmd.EventSeq
					internalSeq++
					continue
				}

				break //  injectCmd.EventSeq > internalSeq, no sense going through the rest
			}

			// trim off any processed commands
			queued = append([]*InjectEvent{}, queued[i:]...)
		}

		for {
			select {
			case event := <-injectChanSlice:
				if 100*len(injectChanSlice)/cap(injectChanSlice) > 50 {
					s.logger.Debug("POP  injectChanSlice LEN", "len", len(injectChanSlice), "cap", cap(injectChanSlice), "module", "inject")
				}
				queued = append(queued, event)
				if len(queued) > 1 {
					sort.SliceStable(queued, func(i, j int) bool {
						return queued[i].EventSeq < queued[j].EventSeq
					})
				}

				if len(queued) < 1 {
					break
				}

				// queue is sorted, so first event seq can be checked
				//   any events less than or equal to internalSeq can be dealt with
				//   either by dropping them or sending them
				if queued[0].EventSeq <= internalSeq {
					tryToPublish(false)
				}

				// oops, we have some left, start a new timer
				if len(queued) > 0 && !timerActive {
					sequenceTimer.Reset(IETIMEOUT)
					timerActive = true
				}

				// we got everything, stop any timers
				if len(queued) == 0 && timerActive {
					if !sequenceTimer.Stop() {
						<-sequenceTimer.C
					}
					timerActive = false
				}

			case <-sequenceTimer.C:
				s.logger.Crit("TIMEOUT waiting for missing sequence events. force send.")
				// we timed out waiting
				timerActive = false
				internalSeq = 0
				tryToPublish(true)

				// oops, we have some left, start a timer
				if len(queued) > 0 {
					sequenceTimer.Reset(IETIMEOUT)
				}
			}
		}
	}()

	go func() {
		for {
			select {
			case evb := <-injectChan:
				s.eb.PublishEvent(context.Background(), evb.event)
				// barrier is set if this event should block events after it
				if evb.barrier {
					evb.event.Wait()
				}
			case <-time.After(time.Duration(5) * time.Second):
				// debug if we start getting full channels
				if len(injectChan) > 0 {
					fmt.Printf("InjectChan queue: %d / %d\n", len(injectChan), cap(injectChan))
				}
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
func (c *InjectEvent) markBarrier() {
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

func (c *InjectEvent) sendToChn() error {
	requestLogger := ContextLogger(c.ctx, "internal_commands").New("module", "inject_event")
	//requestLogger.Crit("InjectService: preparing event", "Sequence", c.EventSeq, "Name", c.Name)

	waits := []func(){}
	defer func() {
		defer c.Done() // this is what supports "Synchronous" commands
		for _, fn := range waits {
			defer fn()
		}
	}()
	trainload := make([]eh.EventData, 0, MAX_CONSOLIDATED_EVENTS)
	sendTrain := func(force bool) {
		// limit number of consolidated events to prevent overflowing queues and deadlocking
		if (force && len(trainload) > 0) || len(trainload) >= MAX_CONSOLIDATED_EVENTS {
			// for now, specific check for events that should be barrier events
			c.markBarrier()

			e := &eventBundle{event: event.NewSyncEvent(c.Name, trainload, time.Now()), barrier: c.Barrier}
			e.event.Add(1) // for EVENT "barrier"
			if c.Synchronous {
				waits = append(waits, e.event.Wait)
			}
			select {
			case injectChan <- e:
			case <-c.ctx.Done():
			}

			trainload = make([]eh.EventData, 0, MAX_CONSOLIDATED_EVENTS)
		}
	}

	decode := func(eventType eh.EventType, m json.RawMessage) {
		if m == nil {
			return
		}
		var data interface{}
		data, err := eh.CreateEventData(eventType)
		if err != nil {
			requestLogger.Info("Decode(%s): fallback to map[string]interface{}", "eventType", eventType, "err", err)
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
				// failed decode, just send the raw binary data and see what happens
				requestLogger.Warn("Decode error", "err", err)
				trainload = append(trainload, eventData) //preallocated
				return
			}
			trainload = append(trainload, data) //preallocated
			return
		}

		err = json.Unmarshal(m, &data)
		if err != nil {
			requestLogger.Warn("Decode message: unmarshal rawmessage failed", "err", err, "RawMessage", string(m))
			return
		}
		trainload = append(trainload, data)
	}

	// create a new, empty event of the requested type. The data will be deserialized into it.
	decode(c.Name, c.EventData)
	for _, d := range c.EventArray {
		sendTrain(false)
		decode(c.Name, d)
	}

	// finally, force send the final load
	sendTrain(true)

	return nil
}
