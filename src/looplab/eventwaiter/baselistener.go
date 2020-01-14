package eventwaiter

import (
	"context"

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/sailfish/src/log"
	myevent "github.com/superchalupa/sailfish/src/looplab/event"
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
	listenerCtx, cancel := context.WithCancel(context.Background())
	return &BaseEventListener{
		listenerInbox: make(chan eh.Event, 20),
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
	for event := range l.listenerInbox {
		if e, ok := event.(syncEvent); ok {
			e.Done()
		}
	}
}

func (l *BaseEventListener) ConsumeEventFromWaiter(event eh.Event) {
	t := event.EventType()

	eventDataArray, ok := event.Data().([]eh.EventData)
	if !ok {
		eventDataArray = []eh.EventData{event.Data()}
	}

	_, isSyncEvent := event.(syncEvent)

	for _, data := range eventDataArray {
		var oneEvent eh.Event
		if isSyncEvent {
			newEv := myevent.NewSyncEvent(t, data, event.Timestamp())
			newEv.WaitGroup = event.(myevent.SyncEvent).WaitGroup
			oneEvent = newEv
		} else {
			oneEvent = eh.NewEvent(t, data, event.Timestamp())
		}

		if l.match(oneEvent) {
			if e, ok := oneEvent.(syncEvent); ok {
				e.Add(1)
			}
			l.listenerInbox <- oneEvent
		}
	}
}

// ProcessEvents repeatedly calls the given function with matching events until the context cancels
func (l *BaseEventListener) ProcessEvents(ctx context.Context, fn func(events eh.Event)) error {
	for {
		select {
		case event := <-l.listenerInbox:
			// closure and defer to ensure that we can cleanly recover from panic without hanging the system
			func() {
				if e, ok := event.(syncEvent); ok {
					defer e.Done()
				}

				fn(event)
			}()

		case <-ctx.Done():
			return ctx.Err()
		case <-l.ctx.Done():
			return l.ctx.Err()
		}
	}
}
