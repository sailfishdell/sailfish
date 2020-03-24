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
	AddMetricReportDefinition            eh.EventType = "AddMetricReportDefinitionEvent"
	AddMetricReportDefinitionResponse    eh.EventType = "AddMetricReportDefinitionEventResponse"
	UpdateMetricReportDefinition         eh.EventType = "UpdateMetricReportDefinitionEvent"
	UpdateMetricReportDefinitionResponse eh.EventType = "UpdateMetricReportDefinitionEventResponse"
	DeleteMetricReportDefinition         eh.EventType = "DeleteMetricReportDefinitionEvent"
	DeleteMetricReportDefinitionResponse eh.EventType = "DeleteMetricReportDefinitionEventResponse"
	DeleteMetricReport                   eh.EventType = "DeleteMetricReportEvent"
	DeleteMetricReportResponse           eh.EventType = "DeleteMetricReportEventResponse"
	DatabaseMaintenance                  eh.EventType = "DatabaseMaintenanceEvent"
	PublishClock                         eh.EventType = "PublishClockEvent"
)

// MetricReportDefinitionData is the eh event data for adding a new report definition
type MetricReportDefinitionData struct {
	Name    string      `db:"Name" json:"Id"`
	Type    string      `db:"Type" json:"MetricReportDefinitionType"` // 'Periodic', 'OnChange', 'OnRequest'
	Actions StringArray `db:"Actions" json:"ReportActions"`           // 	'LogToMetricReportsCollection', 'RedfishEvent'
	Updates string      `db:"Updates" json:"ReportUpdates"`           // 'AppendStopsWhenFull', 'AppendWrapsWhenFull', 'NewReport', 'Overwrite'

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

	// stuff we still need to implement
	// ShortDesc
	// LongDesc
	// Heartbeat
	//
	// This is in the output, but isn't really an input, so can leave it out
	// Status
}

// UnmarshalJSON special decoder for MetricReportDefinitionData to unmarshal the "period" specially
func (mrd *MetricReportDefinitionData) UnmarshalJSON(data []byte) error {
	type Alias MetricReportDefinitionData
	target := struct {
		*Alias
		Schedule struct{ RecurrenceInterval RedfishDuration }
	}{
		Alias: (*Alias)(mrd),
	}

	if err := json.Unmarshal(data, &target); err != nil {
		return err
	}
	mrd.Period = target.Schedule.RecurrenceInterval
	return nil
}

func (mrd MetricReportDefinitionData) GetTimeSpan() time.Duration {
	return time.Duration(mrd.TimeSpan)
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

type AddMetricReportDefinitionData struct {
	metric.Command
	MetricReportDefinitionData
}

func (a *AddMetricReportDefinitionData) DecodeFromReader(ctx context.Context, logger log.Logger, r io.Reader, vars map[string]string) error {
	decoder := json.NewDecoder(r)
	return decoder.Decode(a)
}

type AddMetricReportDefinitionResponseData struct {
	metric.CommandResponse
}

type UpdateMetricReportDefinitionData struct {
	metric.Command
	ReportDefinitionName string
	Patch                json.RawMessage
}

type UpdateMetricReportDefinitionResponseData struct {
	metric.CommandResponse
}

func (u *UpdateMetricReportDefinitionData) DecodeFromReader(ctx context.Context, logger log.Logger, r io.Reader, vars map[string]string) error {
	u.ReportDefinitionName = vars["ID"]
	decoder := json.NewDecoder(r)
	return decoder.Decode(&u.Patch)
}

type DeleteMetricReportDefinitionData struct {
	metric.Command
	Name string
}

func (delMRD *DeleteMetricReportDefinitionData) DecodeFromReader(ctx context.Context, logger log.Logger, r io.Reader, vars map[string]string) error {
	delMRD.Name = vars["ID"]
	return nil
}

type DeleteMetricReportDefinitionResponseData struct {
	metric.CommandResponse
}

type DeleteMetricReportData struct {
	metric.Command
	Name string
}

func (delMR *DeleteMetricReportData) DecodeFromReader(ctx context.Context, logger log.Logger, r io.Reader, vars map[string]string) error {
	delMR.Name = vars["ID"]
	return nil
}

type DeleteMetricReportResponseData struct {
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
	// =========== ADD MRD - AddMetricReportDefinition ==========================
	eh.RegisterEventData(AddMetricReportDefinition, func() eh.EventData {
		return &AddMetricReportDefinitionData{Command: metric.NewCommand(AddMetricReportDefinitionResponse)}
	})
	eh.RegisterEventData(AddMetricReportDefinitionResponse, func() eh.EventData { return &AddMetricReportDefinitionResponseData{} })

	// =========== UPDATE MRD - UpdateMetricReportDefinition ====================
	eh.RegisterEventData(UpdateMetricReportDefinition, func() eh.EventData {
		return &UpdateMetricReportDefinitionData{Command: metric.NewCommand(UpdateMetricReportDefinitionResponse)}
	})
	eh.RegisterEventData(UpdateMetricReportDefinitionResponse, func() eh.EventData { return &UpdateMetricReportDefinitionResponseData{} })

	// =========== DEL MRD - DeleteMetricReportDefinition =======================
	eh.RegisterEventData(DeleteMetricReportDefinition, func() eh.EventData {
		return &DeleteMetricReportDefinitionData{Command: metric.NewCommand(DeleteMetricReportDefinitionResponse)}
	})
	eh.RegisterEventData(DeleteMetricReportDefinitionResponse, func() eh.EventData { return &DeleteMetricReportDefinitionResponseData{} })

	// =========== DEL MR - DeleteMetricReport ==================================
	eh.RegisterEventData(DeleteMetricReport, func() eh.EventData {
		return &DeleteMetricReportData{Command: metric.NewCommand(DeleteMetricReportResponse)}
	})
	eh.RegisterEventData(DeleteMetricReportResponse, func() eh.EventData { return &DeleteMetricReportResponseData{} })

	// These aren't planned to ever be commands
	//   - no need for these to be callable from redfish or other interfaces
	eh.RegisterEventData(DatabaseMaintenance, func() eh.EventData { return "" })
}
