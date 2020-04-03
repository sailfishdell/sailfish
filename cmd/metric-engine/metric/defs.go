package metric

import (
	"database/sql/driver"
	"fmt"
	"io"
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

func (cmd *Command) NewResponseEvent(responseErr error) (eh.Event, error) {
	data, err := eh.CreateEventData(cmd.ResponseType)
	if err != nil {
		return nil, fmt.Errorf("could not create response: %w", err)
	}
	cr, ok := data.(Responser)
	if !ok {
		return nil, fmt.Errorf("internal programming error: response encoded in cmd wasn't a response type: %T -> %+v", data, data)
	}
	cr.SetError(responseErr)
	cr.setRequestID(cmd.RequestID)

	return eh.NewEvent(cmd.ResponseType, cr, time.Now()), nil
}

func (cmd *Command) ResponseWaitFn() func(eh.Event) bool {
	return func(evt eh.Event) bool {
		if evt.EventType() != cmd.ResponseType {
			return false
		}
		if data, ok := evt.Data().(Responser); ok && data.GetRequestID() == cmd.RequestID {
			return true
		}
		return false
	}
}

func (cmd *Command) GetRequestID() eh.UUID {
	return cmd.RequestID
}

type Responser interface {
	GetRequestID() eh.UUID
	setRequestID(eh.UUID)
	SetError(error)
	StreamResponse(io.Writer)
}

type CommandResponse struct {
	RequestID eh.UUID
	err       error
}

func (cr *CommandResponse) GetRequestID() eh.UUID {
	return cr.RequestID
}

func (cr *CommandResponse) setRequestID(id eh.UUID) {
	cr.RequestID = id
}

func (cr *CommandResponse) GetError() error {
	return cr.err
}

func (cr *CommandResponse) Status(setStatus func(int)) {
	// expect that subclasses will override this
	if cr.GetError() != nil {
		setStatus(400) // same as http.StatusBadRequest (without explicitly importing http package)
	}
	setStatus(200) // OK status
}

func (cr *CommandResponse) Headers(setHeader func(string, string)) {
	// common headers
	setHeader("OData-Version", "4.0")
	setHeader("Server", "sailfish")
	setHeader("Content-Type", "application/json; charset=utf-8")
	setHeader("Connection", "keep-alive")
	setHeader("Cache-Control", "no-Store,no-Cache")
	setHeader("Pragma", "no-cache")

	// security headers
	setHeader("Strict-Transport-Security", "max-age=63072000; includeSubDomains") // for A+ SSL Labs score
	setHeader("Access-Control-Allow-Origin", "*")
	setHeader("X-Frame-Options", "DENY")
	setHeader("X-XSS-Protection", "1; mode=block")
	setHeader("X-Content-Security-Policy", "default-src 'self'")
	setHeader("X-Content-Type-Options", "nosniff")

	// compatibility headers
	setHeader("X-UA-Compatible", "IE=11")
}

func (cr *CommandResponse) SetError(err error) {
	cr.err = err
}

func (cr *CommandResponse) StreamResponse(w io.Writer) {
	fmt.Fprintf(w, "CMD RESPONSE: %+v\n", cr)
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
