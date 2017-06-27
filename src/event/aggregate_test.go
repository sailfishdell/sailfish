package eventsourcing

import (
	"github.com/stretchr/testify/assert"
    "fmt"
	"testing"
)

var _ = assert.Equal

type myInterface interface {
	GetThing() *myObj
}

type myEvent struct {
	withGUID
	withSequence
}

type myObj struct {
	baseAggregate
	balance int
}

func (m *myObj) GetThing() *myObj {
	return m
}

func newObj() *myObj {
	return &myObj{balance: 100, baseAggregate: baseAggregate{Version: 42}}
}

type testEvent1 struct {
	withGUID
	withSequence
	val int
}

func (e *testEvent1) Apply(a interface{}) {
	if o, ok := a.(*myObj); ok {
		o.balance = e.val
	}
}

// @see Aggregate.applyEvents
// exact copy of aggregate with different receiver type. Too bad there's not a better way to do this
func (a *myObj) applyEvent(event Event) {
	switch event := event.(type) {
	case Applier:
		event.Apply(a)
	default:
		panic(fmt.Sprintf("Unknown event %#v", event))
	}
}

// @see Aggregate.processCommand
// exact copy of aggregate with different receiver type. Too bad there's not a better way to do this
func (a myObj) processCommand(command Command) []Event {
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

func TestAggregate(t *testing.T) {
	o := newObj()
	assert.Equal(t, 100, o.balance)
	for _, e := range []Event{
        &testEvent1{val: 1},
        &testEvent1{val: 2},
        &testEvent1{val: 3},
        &testEvent1{val: 4},
        &testEvent1{val: 5},
        &testEvent1{val: 6},
        &testEvent1{val: 7},
        } {
		o.applyEvent(e)
	}
	assert.Equal(t, 7, o.balance)
}
