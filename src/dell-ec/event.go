package dell_ec

import (
	eh "github.com/looplab/eventhorizon"
)

const (
	LogEvent      eh.EventType = "LogEvent"
	FaultEntryAdd eh.EventType = "FaultEntryAdd"
)

func init() {
	eh.RegisterEventData(LogEvent, func() eh.EventData { return &LogEventData{} })
	eh.RegisterEventData(FaultEntryAdd, func() eh.EventData { return &FaultEntryAddData{} })
}

type LogEventData struct {
	Description string
	Name        string
	EntryType   string
	Id          int
	MessageArgs []string
	Message     string
	MessageID   string
	Category    string
	Severity    string
	Action      string
}

type FaultEntryAddData struct {
	Description string
	Name        string
	EntryType   string
	Id          int
	MessageArgs []string
	Message     string
	MessageID   string
	Category    string
	Severity    string
	Action      string
}
