package rawjsonstream

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"time"

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/sailfish/src/fifoutil"
	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/event"
)

const (
	maxConsolidatedEvents = 20
	maxUint               = ^int32(0)
)

type Decoder interface {
	Decode(json.RawMessage) error
}

type InjectCommand struct {
	ctx        context.Context
	logger     log.Logger
	sendTime   time.Time
	ingestTime time.Time

	PumpSendTime int64             `json:"PumpSendTime"`
	EventSeq     int64             `json:"event_seq"`
	EventData    json.RawMessage   `json:"data"`
	EventArray   []json.RawMessage `json:"event_array"`
	ID           eh.UUID           `json:"id"`
	Name         eh.EventType      `json:"name"`
	Encoding     string            `json:"encoding"`
	Barrier      bool              `json:"barrier"`     // EventBarrier is set if this event should block subsequent events until it is processed
	Synchronous  bool              `json:"Synchronous"` // Synchronous set if POST should not return until the message is processed
}

func NewInjectCommand(logger log.Logger) *InjectCommand {
	return &InjectCommand{
		ctx:        context.Background(),
		ingestTime: time.Now(),
		Barrier:    true,
		logger:     logger,
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
	logger = log.With(logger, "module", "pipeinput")
	seq := int64(0)

	// hotel california
	for {
		if !fifoutil.IsFIFO(pipePath) {
			// remove if somehow it's not a pipe. does not happen in normal circumstances
			_ = os.Remove(pipePath) // dont care if it doesnt exist or whatever

			// re-create fifo
			err := fifoutil.MakeFifo(pipePath, 0o660) //golang octal prefix: 0o
			if err != nil {
				logger.Crit("Error creating UDB pipe", "err", err)
			}
		}

		processPipe(logger, pipePath, &seq, eb)
	}
}

// open and process FIFO data line by line
func processPipe(logger log.Logger, pipePath string, seq *int64, eb eh.EventBus) {
	// because we open O_RDONLY, this is a blocking call. That's ok.
	file, err := os.OpenFile(pipePath, os.O_RDONLY, 0o660)
	if err != nil {
		logger.Crit("Error opening input pipe", "err", err)
	}

	defer file.Close()

	logger.Debug("Opened FIFO", "path", pipePath)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		buffer := bytes.Trim(scanner.Bytes(), "\x00")
		cmd := NewInjectCommand(logger)
		decoder := json.NewDecoder(bytes.NewReader(buffer))
		decoder.DisallowUnknownFields()
		err := decoder.Decode(cmd)

		if err != nil {
			logger.Warn("JSON Decode error", "err", err)
			continue
		}

		if cmd.Name == "" {
			logger.Warn("pipe input command parse error. no name specified. dropping.")
			continue
		}

		cmd.SetPumpSendTime()
		cmd.EventSeq = *seq
		cmd.sendToChn(eb)
		*seq++
	}
	if err := scanner.Err(); err != nil {
		logger.Warn("Scanner error", "err", err)
	}
}

func (cmd *InjectCommand) sendToChn(eb eh.EventBus) {
	trainload := make([]eh.EventData, 0, maxConsolidatedEvents)

	// accumulate decode events in trainload slice, then send as it gets full
	cmd.appendDecode(&trainload, cmd.EventData)
	for _, d := range cmd.EventArray {
		cmd.appendDecode(&trainload, d)
		if len(trainload) >= maxConsolidatedEvents {
			cmd.sendTrain(trainload, eb)
			trainload = make([]eh.EventData, 0, maxConsolidatedEvents)
		}
	}
	// finally, send the final (partial) load
	if len(trainload) > 0 {
		cmd.sendTrain(trainload, eb)
	}
}

func (cmd *InjectCommand) sendTrain(trainload []eh.EventData, eb eh.EventBus) {
	evt := event.NewSyncEvent(cmd.Name, trainload, time.Now())
	evt.Add(1)
	_ = eb.PublishEvent(context.Background(), evt) // ignore errors, not much we can do
	if cmd.Barrier {
		evt.Wait()
	}
}

func (cmd *InjectCommand) appendDecode(trainload *[]eh.EventData, m json.RawMessage) {
	if m == nil {
		return
	}
	// create a new, empty event of the requested type. The data will be deserialized into it.
	data, err := eh.CreateEventData(cmd.Name)
	if err != nil {
		cmd.logger.Crit("Decode: event type does not exist, skipping", "EventType", cmd.Name, "err", err)
		return
	}

	switch ds := data.(type) {
	case Decoder:
		err = ds.Decode(m)
	default:
		err = json.Unmarshal(m, &data)
	}

	if err != nil {
		cmd.logger.Crit("Event decode failed", "err", err, "EventType", cmd.Name)
		return
	}
	// fast path, avoid logging unless debugging
	//cmd.logger.Debug("Decode: normal json decode added to trainload", "data", data)
	*trainload = append(*trainload, data)
}
