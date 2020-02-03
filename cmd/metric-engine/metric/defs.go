package metric

import (
	"database/sql/driver"
	"time"

	eh "github.com/looplab/eventhorizon"
)

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
	Name string
}

// ReportGeneratedData is the event data structure emitted after reports are generated
type ReportGeneratedData struct {
	Name string
}

// RegisterEvent should be called only once during initialization to register event types with event horizon
func RegisterEvent() {
	eh.RegisterEventData(MetricValueEvent, func() eh.EventData { return &MetricValueEventData{} })
	eh.RegisterEventData(FriendlyFQDDMapping, func() eh.EventData { return &FQDDMappingData{} })
	eh.RegisterEventData(RequestReport, func() eh.EventData { return &RequestReportData{} })
	eh.RegisterEventData(ReportGenerated, func() eh.EventData { return &ReportGeneratedData{} })
}
