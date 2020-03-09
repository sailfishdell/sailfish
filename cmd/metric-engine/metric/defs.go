package metric

import (
	"database/sql/driver"
	"fmt"
	"time"

	eh "github.com/looplab/eventhorizon"
)

// the purpose of this file is to contain all of the Event type definitions and
// structure definitions for the data for the events

// definitions for all of the event horizon event names
const (
	// this legitimately should be called a "Metric Value Event", so disable golint stutter wraning
	MetricValueEvent    eh.EventType = "MetricValueEvent" //nolint: golint
	FriendlyFQDDMapping eh.EventType = "FriendlyFQDDMapping"
	RequestReport       eh.EventType = "RequestReport"
	ReportGenerated     eh.EventType = "ReportGenerated"
)

// some report types/updates that we use in many places, make a const to save memory
const (
	// Types
	Periodic  = "Periodic"
	OnChange  = "OnChange"
	OnRequest = "OnRequest"

	// Updates
	Overwrite           = "Overwrite"
	NewReport           = "NewReport"
	AppendStopsWhenFull = "AppendStopsWhenFull"
	AppendWrapsWhenFull = "AppendWrapsWhenFull"
)

type Command struct {
	RequestID    eh.UUID
	ResponseType eh.EventType
}

func NewCommand(t eh.EventType) Command {
	return Command{RequestID: eh.NewUUID(), ResponseType: t}
}

func (cmd *Command) NewResponseEvent(err error) (eh.Event, error) {
	data, err := eh.CreateEventData(cmd.ResponseType)
	if err != nil {
		return nil, fmt.Errorf("could not create response: %w", err)
	}
	cr, ok := data.(*CommandResponse)
	if !ok {
		return nil, fmt.Errorf("internal programming error: response encoded in cmd wasn't a response type")
	}
	cr.err = err
	cr.RequestID = cmd.RequestID

	return eh.NewEvent(cmd.ResponseType, cr, time.Now()), nil
}

func (cmd *Command) ResponseWaitFn() func(eh.Event) bool {
	return func(evt eh.Event) bool {
		if evt.EventType() != cmd.ResponseType {
			return false
		}
		if data, ok := evt.Data().(*CommandResponse); ok && data.RequestID == cmd.RequestID {
			return true
		}
		return false
	}
}

type CommandResponse struct {
	RequestID eh.UUID
	err       error
}

// SQLTimeInt is a wrapper around golang time that serializes and deserializes 64-bit nanosecond time rather than the default 32-bit second
type SQLTimeInt struct {
	time.Time
}

// Value is the required interface to implement the sql marshalling
func (m SQLTimeInt) Value() (driver.Value, error) {
	return m.UnixNano(), nil
}

// Scan is the required interface to implement the sql unmarshalling
func (m *SQLTimeInt) Scan(src interface{}) error {
	m.Time = time.Unix(0, src.(int64))
	return nil
}

// MetricValueEventData is the data structure to hold everything needed to represent a metric value measurement on the event bus
// this legitimately should be called a "Metric Value Event", so disable golint stutter wraning
type MetricValueEventData struct { //nolint: golint
	Timestamp        SQLTimeInt    `db:"Timestamp"`
	Name             string        `db:"Name"`
	Value            string        `db:"Value"`
	Property         string        `db:"Property"`
	Context          string        `db:"Context"`
	FQDD             string        `db:"FQDD"`
	FriendlyFQDD     string        `db:"FriendlyFQDD"`
	Source           string        `db:"Source"`
	MVRequiresExpand bool          `db:"MVRequiresExpand"`
	MVSensorInterval time.Duration `db:"MVSensorInterval"`
	MVSensorSlack    time.Duration `db:"MVSensorSlack"`
}

// FQDDMappingData is the event data structure to transmit fqdd mappings on the event bus
type FQDDMappingData struct {
	FQDD         string
	FriendlyName string
}

// RequestReportData is the event data structure to tell which report names to generate
type RequestReportData struct {
	Command
	Name string
}

func NewRequestReportCommand(name string) (eh.Event, error) {
	data, err := eh.CreateEventData(RequestReport)
	if err != nil {
		return nil, fmt.Errorf("could not create request report command: %w", err)
	}
	cr, ok := data.(*RequestReportData)
	if !ok {
		return nil, fmt.Errorf("internal programming error: response encoded in cmd wasn't a response type")
	}
	cr.Name = name
	return eh.NewEvent(RequestReport, cr, time.Now()), nil
}

// ReportGeneratedData is the event data structure emitted after reports are generated
type ReportGeneratedData struct {
	CommandResponse
	Name string
}

// RegisterEvent should be called only once during initialization to register event types with event horizon
func RegisterEvent() {
	eh.RegisterEventData(MetricValueEvent, func() eh.EventData { return &MetricValueEventData{} })
	eh.RegisterEventData(FriendlyFQDDMapping, func() eh.EventData { return &FQDDMappingData{} })
	eh.RegisterEventData(RequestReport, func() eh.EventData { return &RequestReportData{Command: NewCommand(ReportGenerated)} })
	eh.RegisterEventData(ReportGenerated, func() eh.EventData { return &ReportGeneratedData{} })
}
