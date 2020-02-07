package telemetryservice

import (
	eh "github.com/looplab/eventhorizon"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

const (
	MetricValueEvent eh.EventType = "MetricValueEvent"
	AddMDEvent 	eh.EventType = "AddMetricDefinitionEvent"
	AddedMRDEvent 	eh.EventType = "AddMetricReportDefinitionEvent"
)

func init() {
	eh.RegisterEventData(MetricValueEvent, func() eh.EventData { return &MetricValueEventData{} })
	eh.RegisterEventData(AddMDEvent, func() eh.EventData { return &AddMDData{} })
	eh.RegisterEventData(AddedMRDEvent, func() eh.EventData { return &MRDData{} })
}

// Properties should be in the format pathtoprop : prop_value
type MetricValueEventData struct {
	MetricId         string
	MetricValue      interface{} 
	Timestamp        string
	MetricProperty   string
}


type MRDData struct {
	UUID			      eh.UUID
	Id                            string
	Description                   string
	Name                          string
	Metrics			      []Metric
	MetricReportDefinitionType    string
	MetricReportDefinitionEnabled bool
	MetricReportHeartbeatInterval string
	SuppressRepeatedMetricValue   bool
	Wildcards                     []domain.WC

}
type Metric struct {
	MetricProperties 	[]string
	MetricID		string
	CollectionDuration	string
	CollectionFunction	string
	CollectionTimeScope	string
}

type AddMDData struct{
	Accuracy int
	Calibration int
	Id 		string
	Implementation string
	MaxReadingRange int
	MetricDataType string
	MetricProperties []string
	MetricType	string
	MinReadingRange int
	Name	string
	PhysicalContext string
	Precision int
	SensingInterval string
	TimestampAccuracy string
	Units	string
	Wildcards []domain.WC
}

