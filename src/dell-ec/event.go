package dell_ec

import (
	eh "github.com/looplab/eventhorizon"
)

const (
	LogEvent eh.EventType = "LogEvent"
)

func init() {
	eh.RegisterEventData(LogEvent, func() eh.EventData { return &LogEventData{} })
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
