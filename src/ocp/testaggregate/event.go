package testaggregate

import (
	eh "github.com/looplab/eventhorizon"
)

const (
	TestEvent        = eh.EventType("TestEvent")
	TestDeletedEvent = eh.EventType("TestDeletedEvent")
)

func init() {
	eh.RegisterEventData(TestEvent, func() eh.EventData { return &TestEventData{} })
	eh.RegisterEventData(TestDeletedEvent, func() eh.EventData { return &TestDeletedEventData{} })
}

type TestEventData struct {
	Unique string
}

type TestDeletedEventData struct {
	Unique string
}
