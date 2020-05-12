package eventwaiter

import (
	"context"

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/event"
)

// MultiEventListener receives events from an EventWaiter. It does not break
// out when there is an []eh.EventData, it passes all events to the waiters as
// []eh.EventData, even if the event originally arrived as eh.EventData.
type MultiEventListener struct {
	*BaseEventListener
}

func NewMultiListener(ctx context.Context, log log.Logger, ew EW, match func(eh.Event) bool) *MultiEventListener {
	l := &MultiEventListener{
		BaseEventListener: NewBaseListener(ctx, log, ew, match),
	}

	ew.RegisterListener(l)
	return l
}

// ConsumeEventFromWaiter for MultiEventListener overrides the base. The base
// specifically always breaks up any []eh.EventData and always sends events
// with one EventData. This one is the opposite. It will always send an array
// (even if it's an array of one).
func (l *MultiEventListener) ConsumeEventFromWaiter(evt eh.Event) {
	_, isArray := evt.Data().([]eh.EventData)
	_, isSync := evt.(event.SyncEvent)

	// want to just always return sync event, but unfortunately in the middle of the stack downstream expects
	if !isArray && isSync {
		// manually create to avoid auto-rewrap
		evt = event.SyncEvent{
			Event:     eh.NewEvent(evt.EventType(), []eh.EventData{evt.Data()}, evt.Timestamp()),
			WaitGroup: evt.(event.SyncEvent).WaitGroup,
		}
	} else if !isArray {
		evt = eh.NewEvent(evt.EventType(), []eh.EventData{evt.Data()}, evt.Timestamp())
	}

	if !l.match(evt) {
		return
	}
	event.PinSyncEvent(evt)
	// fast path, commented out but put back in case of emergency debugging
	//l.logger.Debug("eventlistener queue",
	// 	"sync", isSync,
	// 	"len", len(l.listenerInbox),
	// 	"cap", cap(l.listenerInbox),
	// 	"name", l.Name,
	// 	"module", "ELC",
	// 	"module", "ELC-"+l.Name,
	// 	"event", evt.EventType())
	l.listenerInbox <- evt
}
