package eventsourcing

import (
	"errors"
	"fmt"
)

// Common interface for all events
type Event interface {
	GUIDer
	Sequencer
}

type EventStore interface {
	GetEventsForGUID(guid guid) chan Event
}

type MemEventStore struct {
	ops chan func(map[guid][]Event)
}

func (m *MemEventStore) loop() {
	events := make(map[guid][]Event)
	for op := range m.ops {
		op(events)
	}
	fmt.Println("SHUTDOWN LOOP")
}

func NewMemEventStore() *MemEventStore {
	m := &MemEventStore{ops: make(chan func(map[guid][]Event))}
	go m.loop()
	return m
}

func (m *MemEventStore) Shutdown() {
	close(m.ops)
}

func (m *MemEventStore) AddEvents(newEvents []Event) error {
	retErrorsChan := make(chan error)
	m.ops <- func(m map[guid][]Event) {
		for _, event := range newEvents {
			evList := m[event.GetGUID()]
			if evList == nil {
				evList = []Event{}
			}
			if len(evList) == event.GetSequence() {
				evList = append(evList, event)
				m[event.GetGUID()] = evList
			} else {
				retErrorsChan <- errors.New("tried to add out of sequence event.")
				break
			}
		}
		close(retErrorsChan)
	}
	return <-retErrorsChan
}

func (m *MemEventStore) GetEventsForGUID(g guid) chan Event {
	retEventsChan := make(chan Event)
	m.ops <- func(m map[guid][]Event) {
		if evs, ok := m[g]; ok {
			for _, e := range evs {
				retEventsChan <- e
			}
		}
		close(retEventsChan)
	}
	return retEventsChan
}
