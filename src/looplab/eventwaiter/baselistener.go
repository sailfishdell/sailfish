package eventwaiter

import (
	"context"

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/event"
)

// TODO: read from config or accept as override?
const (
	defaultInboxLen = 20
)

// BaseEventListener receives events from an EventWaiter.
type BaseEventListener struct {
	listenerInbox chan eh.Event
	Name          string
	id            eh.UUID
	match         func(eh.Event) bool
	logger        log.Logger
	ew            EW
	cancel        func()
	ctx           context.Context
}

type EW interface {
	RegisterListener(Listener)
	UnRegisterListener(Listener)
}

func NewBaseListener(ctx context.Context, log log.Logger, ew EW, match func(eh.Event) bool) *BaseEventListener {
	listenerCtx, cancel := context.WithCancel(ctx)
	return &BaseEventListener{
		listenerInbox: make(chan eh.Event, defaultInboxLen),
		Name:          "unnamed",
		id:            eh.NewUUID(),
		match:         match,
		logger:        log,
		ew:            ew,
		cancel:        cancel,
		ctx:           listenerCtx,
	}
}

func (l *BaseEventListener) GetID() eh.UUID      { return l.id }
func (l *BaseEventListener) GetName() string     { return l.Name }
func (l *BaseEventListener) SetName(name string) { l.Name = name }

// Close stops listening for more events.
func (l *BaseEventListener) Close() {
	// CloseInbox() is called back by the waiter so we avoid race conditions
	l.ew.UnRegisterListener(l)
}

// close the inbox
func (l *BaseEventListener) CloseInbox() {
	l.cancel()
	close(l.listenerInbox)

	// closing inbox that may have some inbound events. go ahead and mark them all done
	for evt := range l.listenerInbox {
		event.ReleaseSyncEvent(evt)
	}
}

// ConsumeEventFromWaiter is called by the EventWaiter to deliver events from
// its queue to the individual listener queue.  The base class always delivers
// one eh.EventData at a time in the event, automatically detecting if an array
// of []eh.EventData was given in the original event and splitting them out.
// Consumers can rely on always getting only single events.
func (l *BaseEventListener) ConsumeEventFromWaiter(evt eh.Event) {
	eventDataArray, isArray := evt.Data().([]eh.EventData)
	if !isArray { // optimize single data case to reduce allocations
		if !l.match(evt) {
			return
		}
		event.PinSyncEvent(evt)
		l.listenerInbox <- evt
		return
	}

	_, isSyncEvent := evt.(event.SyncEvent)
	for _, data := range eventDataArray {
		var oneEvent eh.Event
		if isSyncEvent {
			// manually create or we auto-rewrap it back into an array for big lols
			oneEvent = event.SyncEvent{
				Event:     eh.NewEvent(evt.EventType(), data, evt.Timestamp()),
				WaitGroup: evt.(event.SyncEvent).WaitGroup,
			}
		} else {
			oneEvent = eh.NewEvent(evt.EventType(), data, evt.Timestamp())
		}

		if !l.match(oneEvent) {
			continue
		}
		event.PinSyncEvent(oneEvent)
		// fast path, commented out but put back in case of emergency debugging
		//l.logger.Debug("eventlistener queue",
		// 	"sync", isSync,
		//  "len", len(l.listenerInbox),
		// 	"cap", cap(l.listenerInbox),
		// 	"name", l.Name,
		//	"module", "ELC",
		// 	"module", "ELC-"+l.Name,
		//	"event", evt.EventType())
		l.listenerInbox <- oneEvent
	}
}

// ProcessOneEvent calls the given function for exactly the first matching event
func (l *BaseEventListener) ProcessOneEvent(ctx context.Context, fn func(events eh.Event)) error {
	select {
	case evt := <-l.listenerInbox:
		// closure and defer to ensure that we can cleanly recover from panic without hanging the system
		return func() error {
			defer event.ReleaseSyncEvent(evt)
			fn(evt)
			return nil
		}()

	case <-ctx.Done():
		return ctx.Err()
	case <-l.ctx.Done():
		// shut down listener if the listener main context is closed
		l.Close()
		return l.ctx.Err()
	}
}

// ProcessEvents repeatedly calls the given function with matching events until the context cancels
func (l *BaseEventListener) ProcessEvents(ctx context.Context, fn func(events eh.Event)) error {
	for {
		select {
		case evt := <-l.listenerInbox:
			// closure and defer to ensure that we can cleanly recover from panic without hanging the system
			func() {
				defer event.ReleaseSyncEvent(evt)
				fn(evt)
			}()

		case <-ctx.Done():
			return ctx.Err()
		case <-l.ctx.Done():
			// shut down listener if the listener main context is closed
			l.Close()
			return l.ctx.Err()
		}
	}
}
