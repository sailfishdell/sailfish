package eventsourcing

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

var _ = assert.Equal

type MyEvent struct {
	withGUID
	withSequence
}

func TestEventStoreAdd(t *testing.T) {
	testEvent := &MyEvent{withGUID: withGUID{GUID: guid("happy")}}
	m := NewMemEventStore()
	defer m.Shutdown()
	m.AddEvents([]Event{testEvent})
	count := 0
	for e := range m.GetEventsForGUID("happy") {
		count = count + 1
		assert.Equal(t, testEvent, e)
	}
	assert.Equal(t, 1, count)

	// negative case
	count = 0
	for range m.GetEventsForGUID("joy") {
		count = count + 1
	}
	assert.Equal(t, 0, count)
}

func TestEventStoreAddMultiple(t *testing.T) {
	testEvent1 := &MyEvent{withGUID: withGUID{guid("happy")}}
	testEvent2 := &MyEvent{withGUID: withGUID{guid("joy")}}
	// test adding two events... negati
	testEvent3 := &MyEvent{withGUID: withGUID{guid("dup")}}
	testEvent4 := &MyEvent{
		withGUID:     withGUID{guid("dup")},
		withSequence: withSequence{1},
	}
	m := NewMemEventStore()
	defer m.Shutdown()
	m.AddEvents([]Event{testEvent1, testEvent2, testEvent3, testEvent4})
	count := 0
	for e := range m.GetEventsForGUID("happy") {
		count = count + 1
		assert.Equal(t, testEvent1, e)    // positive
		assert.NotEqual(t, testEvent2, e) // negative
	}
	assert.Equal(t, 1, count)

	count = 0
	for e := range m.GetEventsForGUID("joy") {
		count = count + 1
		assert.Equal(t, testEvent2, e)    //positive
		assert.NotEqual(t, testEvent1, e) // negative
	}
	assert.Equal(t, 1, count)

	count = 0
	for range m.GetEventsForGUID("dup") {
		count = count + 1
	}
	assert.Equal(t, 2, count)
}

func TestEventStoreAddMultipleWrongSeq(t *testing.T) {
	// test adding two events with incorrect sequence
	testEvent1 := &MyEvent{withGUID: withGUID{GUID: guid("dup")}}
	testEvent2 := &MyEvent{withGUID: withGUID{GUID: guid("dup")}}
	m := NewMemEventStore()
	defer m.Shutdown()
	m.AddEvents([]Event{testEvent1, testEvent2})
	count := 0
	for range m.GetEventsForGUID("dup") {
		count = count + 1
	}
	assert.Equal(t, 1, count)
}
