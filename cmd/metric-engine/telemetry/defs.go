package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/sailfish/cmd/metric-engine/metric"
	"github.com/superchalupa/sailfish/src/log"
)

// constants to refer to event types
const (
	// GET - get most redfish URIs.
	GenericGETCommandEvent  eh.EventType = "GenericGETCommandEvent"
	GenericGETResponseEvent eh.EventType = "GenericGETResponseEvent"

	// MRD - Metric Report Definition
	AddMRDCommandEvent     eh.EventType = "AddMetricReportDefinitionEvent"
	AddMRDResponseEvent    eh.EventType = "AddMetricReportDefinitionEventResponse"
	UpdateMRDCommandEvent  eh.EventType = "UpdateMetricReportDefinitionEvent"
	UpdateMRDResponseEvent eh.EventType = "UpdateMetricReportDefinitionEventResponse"
	DeleteMRDCommandEvent  eh.EventType = "DeleteMetricReportDefinitionEvent"
	DeleteMRDResponseEvent eh.EventType = "DeleteMetricReportDefinitionEventResponse"

	// MR - Metric Report
	DeleteMRCommandEvent  eh.EventType = "DeleteMetricReportEvent"
	DeleteMRResponseEvent eh.EventType = "DeleteMetricReportEventResponse"

	// MD - Metric Definition
	AddMDCommandEvent  eh.EventType = "AddMetricDefinitionEvent"
	AddMDResponseEvent eh.EventType = "AddMetricDefinitionEventResponse"

	// generic events
	DatabaseMaintenance eh.EventType = "DatabaseMaintenanceEvent"
	PublishClock        eh.EventType = "PublishClockEvent"
)

type GenericGETCommandData struct {
	metric.Command
	URI string
}

type GenericGETResponseData struct {
	metric.CommandResponse
	dataChan   chan []byte
	statusChan chan int
}

func (u *GenericGETCommandData) UseVars(ctx context.Context, logger log.Logger, vars map[string]string) error {
	u.URI = vars["uri"]
	return nil
}

// panic if called multiple times, only call once!
func (cr *GenericGETResponseData) SetStatus(s int) {
	defer close(cr.statusChan)
	cr.statusChan <- s
}

func (cr *GenericGETResponseData) Status(setStatus func(int)) {
	// this will block until we get status, or until requester cancels request
	select {
	case s := <-cr.statusChan:
		setStatus(s)
	case <-cr.Ctx.Done():
		// no-op, just drop out here and close it all down
	}
}

func (cr *GenericGETResponseData) StreamResponse(w io.Writer) (err error) {
	// this will block until dataChan is populated
	for data := range cr.dataChan {
		_, err = w.Write(data)
		if err != nil {
			return err
		}
	}
	return nil
}

// MetricReportDefinitionData is the eh event data for adding a new report definition
type MetricReportDefinitionData struct {
	Name      string      `db:"Name" json:"Id"`
	ShortDesc string      `db:"ShortDesc" json:"Name"`
	LongDesc  string      `db:"LongDesc" json:"Description"`
	Type      string      `db:"Type" json:"MetricReportDefinitionType"` // 'Periodic', 'OnChange', 'OnRequest'
	Actions   StringArray `db:"Actions" json:"ReportActions"`           // 	'LogToMetricReportsCollection', 'RedfishEvent'
	Updates   string      `db:"Updates" json:"ReportUpdates"`           // 'AppendStopsWhenFull', 'AppendWrapsWhenFull', 'NewReport', 'Overwrite'

	// Validation: It's assumed that TimeSpan is parsed on ingress. MRD Schema
	// specifies TimeSpan as a duration.
	// Represents number of seconds worth of metrics in a report. Metrics will be
	// reported from the Report generation as the "End" and metrics must have
	// timestamp > max(End-timespan, report start)
	TimeSpan RedfishDuration `db:"TimeSpan" json:"ReportTimespan"`

	Enabled      bool `db:"Enabled" json:"MetricReportDefinitionEnabled"`
	SuppressDups bool `db:"SuppressDups" json:"SuppressRepeatedMetricValue"`

	// Validation: It's assumed that Period is parsed on ingress. Redfish
	// "Schedule" object is flexible, but we'll allow only period in seconds for
	// now When it gets to this struct, it needs to be expressed in Seconds.
	Period    RedfishDuration `db:"Period" json:"-"` // when type=periodic, it's a Redfish Duration
	Heartbeat RedfishDuration `db:"HeartbeatInterval" json:"MetricReportHeartbeatInterval"`
	Metrics   []RawMetricMeta `db:"Metrics"`

	Hidden bool `db:"Hidden" json:"-"`
}

// UnmarshalJSON special decoder for MetricReportDefinitionData to unmarshal the "period" specially
func (mrd *MetricReportDefinitionData) UnmarshalJSON(data []byte) error {
	type Alias MetricReportDefinitionData
	target := struct {
		*Alias
		Schedule *struct{ RecurrenceInterval RedfishDuration } `json:omitifempty`
	}{
		Alias: (*Alias)(mrd),
	}

	if err := json.Unmarshal(data, &target); err != nil {
		return err
	}
	if target.Schedule != nil {
		mrd.Period = target.Schedule.RecurrenceInterval
	}
	return nil
}

func (mrd MetricReportDefinitionData) GetTimeSpan() time.Duration {
	return time.Duration(mrd.TimeSpan)
}

// MetricDefifinitionData - is the eh event data for adding a new new definition (future)
type MetricDefinitionData struct {
	MetricID        string      `db:"MetricID"          json:"Id"`
	Name            string      `db:"Name"              json:"Name"`
	Description     string      `db:"Description"       json:"Description"`
	MetricType      string      `db:"MetricType"        json:"MetricType"`
	MetricDataType  string      `db:"MetricDataType"    json:"MetricDataType"`
	Units           string      `db:"Units"             json:"Units"`
	Accuracy        float32     `db:"Accuracy"          json:"Accuracy"`
	SensingInterval string      `db:"SensingInterval"   json:"SensingInterval"`
	DiscreteValues  StringArray `db:"DiscreteValues"   json:"DiscreteValues"`
}

// RawMetricMeta is a sub structure to help serialize stuff to db. it containst
// the stuff we are putting in or taking out of DB for Meta.
// Validation: It's assumed that Duration is parsed on ingress. The ingress
// format is (Redfish Duration): -?P(\d+D)?(T(\d+H)?(\d+M)?(\d+(.\d+)?S)?)?
// When it gets to this struct, it needs to be expressed in Seconds.
type RawMetricMeta struct {
	// Meta fields
	MetaID             int64           `db:"MetaID"`
	NamePattern        string          `db:"NamePattern" json:"MetricID"`
	CollectionFunction string          `db:"CollectionFunction" json:"CollectionFunction"`
	CollectionDuration RedfishDuration `db:"CollectionDuration" json:"CollectionDuration"`

	FQDDPattern     string      `db:"FQDDPattern" json:"-"`
	SourcePattern   string      `db:"SourcePattern" json:"-"`
	PropertyPattern string      `db:"PropertyPattern" json:"-"`
	Wildcards       StringArray `db:"Wildcards"  json:"-"`
}

// UnmarshalJSON special decoder for MetricReportDefinitionData to unmarshal the "period" specially
func (m *RawMetricMeta) UnmarshalJSON(data []byte) error {
	type Alias RawMetricMeta
	target := struct {
		*Alias
		Oem struct {
			Dell struct {
				FQDDPattern   string `json:"FQDD"`
				SourcePattern string `json:"Source"`
			} `json:"Dell"`
		} `json:"OEM"`
	}{
		Alias: (*Alias)(m),
	}

	if err := json.Unmarshal(data, &target); err != nil {
		return err
	}
	m.FQDDPattern = target.Oem.Dell.FQDDPattern
	m.SourcePattern = target.Oem.Dell.SourcePattern

	return nil
}

// MetricReportDefinition represents a DB record for a metric report
// definition. Basically adds ID and a static AppendLimit (for now, until we
// can figure out how to make this dynamic).
type MetricReportDefinition struct {
	*MetricReportDefinitionData
	AppendLimit int   `db:"AppendLimit"`
	ID          int64 `db:"ID"`
}

type AddMRDCommandData struct {
	metric.Command
	MetricReportDefinitionData
}

func (a *AddMRDCommandData) UseInput(ctx context.Context, logger log.Logger, r io.Reader) error {
	decoder := json.NewDecoder(r)
	return decoder.Decode(a)
}

type AddMRDResponseData struct {
	metric.CommandResponse
	metric.URIChanged
}

type UpdateMRDCommandData struct {
	metric.Command
	ReportDefinitionName string
	Patch                json.RawMessage
}

type UpdateMRDResponseData struct {
	metric.CommandResponse
	metric.URIChanged
}

func (u *UpdateMRDCommandData) UseInput(ctx context.Context, logger log.Logger, r io.Reader) error {
	decoder := json.NewDecoder(r)
	return decoder.Decode(&u.Patch)
}

func (u *UpdateMRDCommandData) UseVars(ctx context.Context, logger log.Logger, vars map[string]string) error {
	u.ReportDefinitionName = vars["ID"]
	return nil
}

type DeleteMRDCommandData struct {
	metric.Command
	Name string
}

func (delMRD *DeleteMRDCommandData) UseVars(ctx context.Context, logger log.Logger, vars map[string]string) error {
	delMRD.Name = vars["ID"]
	return nil
}

type DeleteMRDResponseData struct {
	metric.CommandResponse
	metric.URIChanged
}

type DeleteMRCommandData struct {
	metric.Command
	Name string
}

func (delMR *DeleteMRCommandData) UseVars(ctx context.Context, logger log.Logger, vars map[string]string) error {
	delMR.Name = vars["ID"]
	return nil
}

type DeleteMRResponseData struct {
	metric.CommandResponse
}

// MD defs
type MetricDefinition struct {
	*MetricDefinitionData
}

type AddMDCommandData struct {
	metric.Command
	MetricDefinitionData
}

func (a *AddMDCommandData) UseInput(ctx context.Context, logger log.Logger, r io.Reader) error {
	decoder := json.NewDecoder(r)
	return decoder.Decode(a)
}

type AddMDResponseData struct {
	metric.CommandResponse
}

func Factory(et eh.EventType) func() (eh.Event, error) {
	return func() (eh.Event, error) {
		data, err := eh.CreateEventData(et)
		if err != nil {
			return nil, fmt.Errorf("could not create request report command: %w", err)
		}
		return eh.NewEvent(et, data, time.Now()), nil
	}
}

func RegisterEvents() {
	// Full commands (request/response)
	// =========== GET - generically get any specific URI =======================
	eh.RegisterEventData(GenericGETCommandEvent, func() eh.EventData {
		return &GenericGETCommandData{Command: metric.NewCommand(GenericGETResponseEvent)}
	})
	eh.RegisterEventData(GenericGETResponseEvent, func() eh.EventData {
		ev := &GenericGETResponseData{
			dataChan:   make(chan []byte),
			statusChan: make(chan int),
		}
		return ev
	})

	// =========== ADD MRD - AddMRDCommand ==========================
	eh.RegisterEventData(AddMRDCommandEvent, func() eh.EventData {
		return &AddMRDCommandData{Command: metric.NewCommand(AddMRDResponseEvent)}
	})
	eh.RegisterEventData(AddMRDResponseEvent, func() eh.EventData { return &AddMRDResponseData{} })

	// =========== UPDATE MRD - UpdateMRDCommandEvent ====================
	eh.RegisterEventData(UpdateMRDCommandEvent, func() eh.EventData {
		return &UpdateMRDCommandData{Command: metric.NewCommand(UpdateMRDResponseEvent)}
	})
	eh.RegisterEventData(UpdateMRDResponseEvent, func() eh.EventData { return &UpdateMRDResponseData{} })

	// =========== DEL MRD - DeleteMRDCommandEvent =======================
	eh.RegisterEventData(DeleteMRDCommandEvent, func() eh.EventData {
		return &DeleteMRDCommandData{Command: metric.NewCommand(DeleteMRDResponseEvent)}
	})
	eh.RegisterEventData(DeleteMRDResponseEvent, func() eh.EventData { return &DeleteMRDResponseData{} })

	// =========== DEL MR - DeleteMRCommandEvent ==================================
	eh.RegisterEventData(DeleteMRCommandEvent, func() eh.EventData {
		return &DeleteMRCommandData{Command: metric.NewCommand(DeleteMRResponseEvent)}
	})
	eh.RegisterEventData(DeleteMRResponseEvent, func() eh.EventData { return &DeleteMRResponseData{} })

	// =========== ADD MD - AddMDCommandEvent ==========================
	eh.RegisterEventData(AddMDCommandEvent, func() eh.EventData {
		return &AddMDCommandData{Command: metric.NewCommand(AddMDResponseEvent)}
	})
	eh.RegisterEventData(AddMDResponseEvent, func() eh.EventData { return &AddMDResponseData{} })

	// These aren't planned to ever be commands
	//   - no need for these to be callable from redfish or other interfaces
	eh.RegisterEventData(DatabaseMaintenance, func() eh.EventData { return "" })
}
