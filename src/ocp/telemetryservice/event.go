package telemetryservice

import (
	eh "github.com/looplab/eventhorizon"
)

const (
	MetricValueEvent eh.EventType = "MetricValueEvent"
)

func init() {
	eh.RegisterEventData(MetricValueEvent, func() eh.EventData { return &MetricValueEventData{} })
}

// Properties should be in the format pathtoprop : prop_value
type MetricValueEventData struct {
	UUID             eh.UUID
	Properties       map[string]interface{}
	MetricId         string
	MetricValue      string
	Timestamp        string
	MetricProperty   string
	reportUpdateType string
}
