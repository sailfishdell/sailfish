package faultlist

import (
	eh "github.com/looplab/eventhorizon"
)

const (
	FaultEntryAdd eh.EventType = "FaultEntryAdd"
)

func init() {
	eh.RegisterEventData(FaultEntryAdd, func() eh.EventData { return &FaultEntryAddData{} })
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
