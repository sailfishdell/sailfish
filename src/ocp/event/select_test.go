package event

import (
	"testing"
	"time"

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/go-redfish/src/log"
)

type TestData struct {
	Foo  string
	Bar  int
	Bool bool
}

func TestFilter(t *testing.T) {
	tables := []struct {
		event  eh.Event
		filter string
		want   bool
	}{
		{eh.NewEvent(eh.EventType("foobar"), TestData{"happy", 10, true}, time.Now()), "data.Bool", true},
		{eh.NewEvent(eh.EventType("foobar"), TestData{"happy", 10, true}, time.Now()), "type == 'foobar'", true},
		{eh.NewEvent(eh.EventType("foobar"), TestData{"happy", 10, true}, time.Now()), "type == 'foxbar'", false},
		{eh.NewEvent(eh.EventType("foobar"), TestData{"happy", 10, true}, time.Now()), "string(event.EventType()) == 'foobar'", true},
		{eh.NewEvent(eh.EventType("foobar"), TestData{"happy", 10, true}, time.Now()), "string(event.EventType()) == 'foobar' && data.Bool == true", true},
		{eh.NewEvent(eh.EventType("foobar"), TestData{"happy", 10, true}, time.Now()), "string(event.EventType()) == 'foobar' && data.Bool == false", false},
		{eh.NewEvent(eh.EventType("foobar"), TestData{"happy", 10, true}, time.Now()), "type == 'foobar' && data.Bar > 9", true},
		{eh.NewEvent(eh.EventType("foobar"), TestData{"happy", 10, true}, time.Now()), "type == 'foobar' && data.Bar < 11", true},
		{eh.NewEvent(eh.EventType("foobar"), TestData{"happy", 10, true}, time.Now()), "type == 'foobar' && data.Bar == 11", false},
	}

	logger := log.MustLogger("TEST")

	for _, table := range tables {
		filter, err := Filter(logger, table.filter)
		if err != nil {
			t.Errorf("problem in (%s) parsing string: %s\n", table.filter, err)
			continue
		}
		got := filter(table.event)
		if got != table.want {
			t.Errorf("filter(%s) incorrect got: %v, want: %v.", table.filter, got, table.want)
			continue
		}
	}
}
