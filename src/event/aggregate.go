package eventsourcing

import (
	"fmt"
)

type Command interface {
	GUIDer
}

type Applier interface {
	Apply(interface{})
}

type Processor interface {
	Process(interface{}) []Event
}

type Aggregate interface {
	GUIDer
	Sequencer
	applyEvent(Event)
	processCommand(Command) []Event
}

type baseAggregate struct {
	withGUID
	withSequence
	Version int
}

func RestoreAggregate(g guid, a Aggregate, es EventStore) {
	a.SetGUID(g)
	for ev := range es.GetEventsForGUID(g) {
		a.applyEvent(ev)
	}
}

// @see Aggregate.applyEvents
func (a *baseAggregate) applyEvent(event Event) {
	switch event := event.(type) {
	case Applier:
		event.Apply(a)
	default:
		panic(fmt.Sprintf("Unknown event %#v", event))
	}
}

// @see Aggregate.processCommand
func (a baseAggregate) processCommand(command Command) []Event {
	var events []Event = []Event{}
	switch c := command.(type) {
	case Processor:
		events = append(events, c.Process(a)...)
	default:
		panic(fmt.Sprintf("Unknown command %#v", c))
	}
	for _, event := range events {
		event.SetGUID(command.GetGUID())
	}
	return events
}
