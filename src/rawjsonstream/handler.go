package rawjsonstream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	eh "github.com/looplab/eventhorizon"
	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/event"
)

const (
	maxConsolidatedEvents = 20
	maxQueuedInjectEvents = 10
	maxUint               = ^int32(0)
)

type Decoder interface {
	Decode(d map[string]interface{}) error
}

type eventBundle struct {
	event   *event.SyncEvent
	barrier bool
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

func StartPipeHandler(logger log.Logger, pipePath string, eb eh.EventBus) {
	injectChan := make(chan *eventBundle, maxQueuedInjectEvents)
	go sendEventsToInternalBus(injectChan, eb)

	// clear file of previous buffered data
	err := makeFifo(pipePath, 0660)
	if err != nil && !os.IsExist(err) {
		logger.Warn("Error creating UDB pipe", "err", err)
	}

	file, err := os.OpenFile(pipePath, os.O_CREATE, os.ModeNamedPipe)
	if err != nil {
		logger.Crit("Error opening UDB pipe", "err", err)
	}

	defer file.Close()

	nullWriter, err := os.OpenFile(pipePath, os.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		logger.Crit("Error opening UDB pipe for (placeholder) write", "err", err)
	}

	// defer .Close() to keep linters happy. Inside we know we never exit...
	defer nullWriter.Close()

	seq := int64(0)
	injectCmd := func(event []byte) error {
		cmd := NewInjectCommand()
		decoder := json.NewDecoder(bytes.NewReader(event))
		decoder.DisallowUnknownFields()
		err = decoder.Decode(cmd)

		if err != nil {

			fmt.Printf("error decoding stream json: %s %d\n", err, len(event))
			fmt.Printf("failed event: %s\n", event)
			return errors.New("failed to decode json")
		}

		cmd.SetPumpSendTime()
		cmd.Barrier = true
		cmd.Synchronous = true
		if cmd.Name == "" {
			fmt.Printf("No name specified. dropping: %+v\n", cmd)
			return errors.New("No name specified. dropping")
		}

		//Enable commented out lines for debugging
		cmd.Add(1)
		cmd.EventSeq = seq
		//fmt.Printf("Send to ch(%d): %+v\n", cmd.EventSeq, cmd)

		cmd.sendToChn(injectChan)

		//fmt.Printf("Waiting for: %d - %t\n", cmd.EventSeq)
		cmd.Wait()
		seq++
		//fmt.Printf("DONE       : %d \n", cmd.EventSeq )
		return nil
	}

	pipeIterator(logger, file, injectCmd)
}

// iterate elements in fifo by '\n'
// run each element as line in func
func pipeIterator(logger log.Logger, f *os.File, fn func([]byte) error) {
	line := []byte{}
	for {
		buffer := make([]byte, 4096)
		len, err := f.Read(buffer)
		if err == io.EOF {
			logger.Crit("sailfish.pipe EOF reached")
			break
		} else if err != nil {
			logger.Crit("pipe read error", "error", err)
		}
		if len == 0 {
			continue
		}
		nil_idx := bytes.Index(buffer, []byte{'\x00'})
		if nil_idx != -1 {
			//fmt.Printf("before trim %+q\n", buffer);
			buffer = bytes.Trim(buffer, "\x00")
		}

		line = append(line, buffer...)
		idx := bytes.Index(line, []byte{'\n'})
		for ; idx != -1; idx = bytes.Index(line, []byte{'\n'}) {
			evt := line[:idx]
			//fmt.Printf("EVT %+q \n", evt )
			fn(evt)
			line = line[idx+1:]
		}
	}
}

func sendEventsToInternalBus(injectChan chan *eventBundle, eb eh.EventBus) {
	for evb := range injectChan {
		eb.PublishEvent(context.Background(), *evb.event)
		// barrier is set if this event should block events after it
		if evb.barrier {
			evb.event.Wait()
		}
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
	requestLogger := log.ContextLogger(c.ctx, "internal_commands")
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
