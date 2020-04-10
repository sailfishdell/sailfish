package rawjsonstream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
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
	ctx        context.Context
	sendTime   time.Time
	ingestTime time.Time

	PumpSendTime   int64             `json:"PumpSendTime"`
	EventSeq       int64             `json:"event_seq"`
	EventData      json.RawMessage   `json:"data"`
	EventArray     []json.RawMessage `json:"event_array"`
	ID             eh.UUID           `json:"id"`
	Name           eh.EventType      `json:"name"`
	Encoding       string            `json:"encoding"`
	sync.WaitGroup                   // this in the middle of the struct due to linter warning... this produces smallest struct size
	Barrier        bool              `json:"barrier"`     // EventBarrier is set if this event should block subsequent events until it is processed
	Synchronous    bool              `json:"Synchronous"` // Synchronous set if POST should not return until the message is processed
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
			logger.Warn("error decoding json", "err", err, "length", len(event), "event", event)
			return errors.New("failed to decode json")
		}

		cmd.SetPumpSendTime()
		cmd.Barrier = true
		cmd.Synchronous = true
		if cmd.Name == "" {
			logger.Warn("no name specified, dropping event", "cmd", cmd)
			return errors.New("no name specified. dropping")
		}

		cmd.Add(1)
		cmd.EventSeq = seq

		cmd.sendToChn(injectChan)

		cmd.Wait()
		seq++
		return nil
	}

	pipeIterator(logger, file, injectCmd)
	fstat, err := file.Stat()
	if err != nil {
		fmt.Printf("Error while reading sailfish.pipe file: %s\n", err)
	} else {
		fmt.Printf("Sailfish pipe size: %d\n", fstat.Size())
	}

	out, err := exec.Command("fuser", pipePath).Output()
	if err != nil {
		fmt.Printf("Error while running 'fuser': %s\n", err)
	} else {
		fmt.Printf("Fuser Output: %s\n", string(out))
		ppids := strings.Split(string(out), " ")
		for _, ppid := range ppids {
			out, err := exec.Command("ps", "-p", ppid).Output()
			if err != nil {
				fmt.Printf("ps did not recognize process id %s\n", ppid)
			} else {
				pInfo := strings.Split(string(out), "\n")
				fmt.Printf("PPID: %s\nProcess: %s\n", ppid, pInfo[len(pInfo)-2])
			}
		}
	}

	ret, err := nullWriter.Seek(0, 0)
	if err != nil {
		fmt.Printf("NULLWRITER OFFSET ERROR: %s", err)
	} else {
		fmt.Printf("NULLWRITER OFFSET SUCCEEDED")
		fmt.Printf("NEW OFFSET VALUE: %d", ret)
	}

	logger.Crit("Sending interrupt signal to restart sailfish")
	syscall.Kill(syscall.Getpid(), syscall.SIGINT)
}

// iterate elements in fifo by '\n'
// run each element as line in func
func pipeIterator(logger log.Logger, f io.Reader, fn func([]byte) error) {
	line := []byte{}
	for {
		buffer := make([]byte, 4096)
		len, err := f.Read(buffer)
		if err == io.EOF {
			logger.Crit("SAILFISH.PIPE EOF REACHED, STOPPING PIPE ITERATOR")
			break
		} else if err != nil {
			logger.Crit("pipe read error", "error", err)
		}
		if len == 0 {
			continue
		}
		if bytes.Contains(buffer, []byte{'\x00'}) {
			buffer = bytes.Trim(buffer, "\x00")
		}

		line = append(line, buffer...)
		idx := bytes.Index(line, []byte{'\n'})
		for ; idx != -1; idx = bytes.Index(line, []byte{'\n'}) {
			evt := line[:idx]
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

func (cmd *InjectCommand) sendToChn(injectChan chan *eventBundle) error {
	//requestLogger := log.ContextLogger(cmd.ctx, "internal_commands").New("module", "inject_event")
	//requestLogger.Crit("InjectService: preparing event", "Sequence", cmd.EventSeq, "Name", cmd.Name)

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
		cmd.Done()
	}()

	totalTrains := 0
	doneTrains := 0
	waitForEvent := func(evt event.SyncEvent) func() {
		return func() {
			doneTrains++
			if cmd.Synchronous {
				evt.Wait()
				// UNCOMMENT THE LINES HERE TO GET COMPREHENSIVE METRICS FOR TIMINGS FOR PROCESSING EACH EVENT
				// We should do Prometheus metrics RIGHT HERE
				//fmt.Printf("\tevent %s %d#%d/%d DONE:  ingest: %s  total: %s\n",
				//   cmd.Name, cmd.EventSeq, totalTrains, doneTrains, time.Now().Sub(cmd.ingestTime), time.Now().Sub(cmd.sendTime))
				//} else {
				// spawn a goroutine to wait for processing to complete since caller declines to wait.
				//go func(t, d int) {
				//	evt.Wait()
				// AND We should do Prometheus metrics RIGHT HERE
				//	fmt.Printf("\tevent %s %d#%d/%d DONE:  ingest: %s  total: %s\n",
				//     cmd.Name, cmd.EventSeq, totalTrains, doneTrains, time.Now().Sub(cmd.ingestTime), time.Now().Sub(cmd.sendTime))
				//}(totalTrains, doneTrains)
			}
		}
	}

	trainload := make([]eh.EventData, 0, maxConsolidatedEvents)
	sendTrain := func([]eh.EventData) {
		if len(trainload) == 0 {
			return
		}

		evt := event.NewSyncEvent(cmd.Name, trainload, time.Now())
		evt.Add(1)
		select {
		case injectChan <- &eventBundle{&evt, cmd.Barrier}:
			// make sure we don't add the .Wait() until after we know it's being
			// processed by the other side. Otherwise the context cancel (below, the
			// case <-cmd.ctx.Done()) will keep the message from being sent from our
			// side, and then we'll .Wait() for something that can never be .Done()
			totalTrains++
			waits = append(waits, waitForEvent(evt))
		case <-cmd.ctx.Done():
			//requestLogger.Info("CONTEXT CANCELLED! Discarding trainload", "err", cmd.ctx.Err(), "trainload", trainload, "EventName", cmd.Name)
		}
	}

	// accumulate decode events in trainload slice, then send as it gets full
	cmd.appendDecode(&trainload, cmd.Name, cmd.EventData)
	for _, d := range cmd.EventArray {
		cmd.appendDecode(&trainload, cmd.Name, d)
		if len(trainload) >= maxConsolidatedEvents {
			sendTrain(trainload)
			trainload = make([]eh.EventData, 0, maxConsolidatedEvents)
		}
	}
	// finally, send the final (partial) load
	sendTrain(trainload)

	return nil
}

func (cmd *InjectCommand) appendDecode(trainload *[]eh.EventData, eventType eh.EventType, m json.RawMessage) {
	requestLogger := log.ContextLogger(cmd.ctx, "internal_commands")
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
			requestLogger.Warn("Custom Decode error, send data as map[string]interface{}", "err", err, "EventName", cmd.Name)
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
