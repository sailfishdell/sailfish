package eventwaiter

import (
	"context"

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/sailfish/src/log"
)

// EventListener receives events from an EventWaiter.
type EventListener struct {
	*BaseEventListener
}

func NewListener(ctx context.Context, log log.Logger, ew EW, match func(eh.Event) bool) *EventListener {
	l := &EventListener{
		BaseEventListener: NewBaseListener(ctx, log, ew, match),
	}

	ew.RegisterListener(l)

	return l
}

// Wait waits for the event to arrive.
// STRONGLY PREFER TO USE .ProcessEvents() instead of this
func (l *EventListener) Wait(ctx context.Context) (eh.Event, error) {
	select {
	case event := <-l.listenerInbox:
		// TODO: separation of concerns: this should be factored out into a middleware of some sort...
		// now that we are waiting on the listeners, we can .Done() the waitgroup for the eventwaiter itself
		if e, ok := event.(syncEvent); ok {
			e.Done()
			//defer fmt.Printf("Done in Wait()\n")
		}

		return event, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// UnSyncWait waits for the event to arrive, but does not .Done() the event. Caller is responsible!
// STRONGLY PREFER TO USE .ProcessEvents() instead of this
func (l *EventListener) UnSyncWait(ctx context.Context) (eh.Event, error) {
	select {
	case event := <-l.listenerInbox:
		return event, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Inbox returns the channel that events will be delivered on so that you can integrate into your own select() if needed.
// DOES NOT RUN .Done()! Caller is responsible!
// STRONGLY PREFER TO USE .ProcessEvents() instead of this
func (l *EventListener) Inbox() <-chan eh.Event {
	return l.listenerInbox
}
