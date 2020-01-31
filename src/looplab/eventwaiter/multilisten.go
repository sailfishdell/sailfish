package eventwaiter

import (
	"context"

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/sailfish/src/log"
	myevent "github.com/superchalupa/sailfish/src/looplab/event"
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
func (l *MultiEventListener) ConsumeEventFromWaiter(event eh.Event) {
	_, isArray := event.Data().([]eh.EventData)
	syncE, isSync := event.(myevent.SyncEvent)
	var EventToSend eh.Event

	if isArray {
		// already an array ([]eh.EventData)
		EventToSend = event
	} else {
		// create an array ([]eh.EventData)
		t := event.EventType()
		if isSync {
			EventToSend = myevent.NewSyncEvent(t, []eh.EventData{event.Data()}, event.Timestamp())
			syncEvToSend := EventToSend.(myevent.SyncEvent)
			syncEvToSend.WaitGroup = syncE.WaitGroup
		} else {
			EventToSend = eh.NewEvent(t, []eh.EventData{event.Data()}, event.Timestamp())
		}
	}

	if !l.match(EventToSend) {
		return
	}
	if isSync {
		EventToSend.(myevent.SyncEvent).Add(1)
	}
	l.listenerInbox <- EventToSend
}
