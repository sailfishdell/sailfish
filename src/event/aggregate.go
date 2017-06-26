package eventsourcing

type Command interface {
	GUIDer
}

type Aggregate interface {
	GUIDer
	Sequencer
	applyEvent(Event)
	processCommands(Command) []Event
}

type baseAggregate struct {
	withGUID
	Version int
}

func RestoreAggregate(g guid, a Aggregate, es EventStore) {
	a.SetGUID(g)
	for ev := range es.GetEventsForGUID(g) {
		a.applyEvent(ev)
	}
}
