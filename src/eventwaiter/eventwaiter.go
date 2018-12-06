// Copyright (c) 2017 - The Event Horizon authors.
// modifications Copyright (c) 2018 - Dell EMC
//  - don't drop events
//  - rework the api between waiter and listener so they aren't so incestuous
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package eventwaiter

import (
	"context"

	eh "github.com/looplab/eventhorizon"
	myevent "github.com/superchalupa/sailfish/src/event"
	"github.com/superchalupa/sailfish/src/log"
)

type listener interface {
	GetID() eh.UUID
	processEvent(event eh.Event)
	closeInbox()
}

// EventWaiter waits for certain events to match a criteria.
type EventWaiter struct {
	name       string
	done       chan struct{}
	inbox      chan eh.Event
	register   chan listener
	unregister chan listener
	autorun    bool
	logger     log.Logger
}

type Option func(e *EventWaiter) error

// NewEventWaiter returns a new EventWaiter.
func NewEventWaiter(o ...Option) *EventWaiter {
	w := EventWaiter{
		done:       make(chan struct{}),
		inbox:      make(chan eh.Event, 50),
		register:   make(chan listener),
		unregister: make(chan listener),
		autorun:    true,
	}

	w.ApplyOption(o...)

	if w.autorun {
		go w.Run()
	}
	return &w
}

func (w *EventWaiter) Close() {
	close(w.done)
}

func NoAutoRun(w *EventWaiter) error {
	w.autorun = false
	return nil
}

func SetName(name string) Option {
	return func(w *EventWaiter) error {
		w.name = name
		return nil
	}
}

func WithLogger(l log.Logger) Option {
	return func(w *EventWaiter) error {
		w.logger = l
		return nil
	}
}

func (w *EventWaiter) ApplyOption(options ...Option) error {
	for _, o := range options {
		err := o(w)
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *EventWaiter) Run() {
	listeners := map[eh.UUID]listener{}
	startPrinting := false
	for {
		select {
		case <-w.done:
			return
		case l := <-w.register:
			listeners[l.GetID()] = l
		case l := <-w.unregister:
			// Check for existence to avoid closing channel twice.
			if _, ok := listeners[l.GetID()]; ok {
				delete(listeners, l.GetID())
				l.closeInbox()
			}
		case event := <-w.inbox:
			if len(w.inbox) > 25 {
				startPrinting = true
			}
			if startPrinting && w.logger != nil {
				w.logger.Debug("Event Waiter congestion", "len", len(w.inbox), "cap", cap(w.inbox), "name", w.name)
			}
			if len(w.inbox) == 0 {
				startPrinting = false
			}
			for _, l := range listeners {

				l.processEvent(event)
			}

			// TODO: separation of concerns: this should be factored out into a middleware of some sort...
			// now that we are waiting on the listeners, we can .Done() the waitgroup for the eventwaiter itself
			if e, ok := event.(syncEvent); ok {
				//fmt.Printf("Done in listener\n")
				e.Done()
			}

		}
	}
}

type syncEvent interface {
	Add(int)
	Done()
}

// Notify implements the eventhorizon.EventObserver.Notify method which forwards
// events to the waiters so that they can match the events.
func (w *EventWaiter) Notify(ctx context.Context, event eh.Event) {

	// TODO: separation of concerns: this should be factored out into a middleware of some sort...
	if e, ok := event.(syncEvent); ok {
		//fmt.Printf("ADD(1) in eventwaiter Notify\n")
		e.Add(1)
	}

	w.inbox <- event
}

// Listen waits unil the match function returns true for an event, or the context
// deadline expires. The match function can be used to filter or otherwise select
// interesting events by analysing the event data.
func (w *EventWaiter) Listen(ctx context.Context, match func(eh.Event) bool) (*EventListener, error) {
	l := &EventListener{
		Name:             "unnamed",
		id:               eh.NewUUID(),
		singleEventInbox: make(chan eh.Event, 5),
		match:            match,
		unregister:       w.unregister,
		logger:           w.logger,
	}

	w.RegisterListener(l)

	return l, nil
}

func (w *EventWaiter) RegisterListener(l listener) {
	w.register <- l
}

// EventListener receives events from an EventWaiter.
type EventListener struct {
	Name             string
	id               eh.UUID
	singleEventInbox chan eh.Event
	match            func(eh.Event) bool
	unregister       chan listener
	eventType        *eh.EventType
	startPrinting    bool
	logger           log.Logger
}

func (l *EventListener) SetSingleEventType(t eh.EventType) {
	l.eventType = &t
}

func (l *EventListener) GetID() eh.UUID { return l.id }

func (l *EventListener) processEvent(event eh.Event) {
	t := event.EventType()
	if l.eventType != nil && *l.eventType != t {
		// early return
		return
	}

	eventDataArray, ok := event.Data().([]eh.EventData)
	if ok {
		for _, data := range eventDataArray {

			var oneEvent eh.Event
			if _, ok := event.(myevent.SyncEvent); ok {
				newEv := myevent.NewSyncEvent(t, data, event.Timestamp())
				newEv.WaitGroup = event.(myevent.SyncEvent).WaitGroup
				oneEvent = newEv
			} else {
				oneEvent = eh.NewEvent(t, data, event.Timestamp())
			}

			if l.match(oneEvent) {
				// TODO: separation of concerns: this should be factored out into a middleware of some sort...
				// now that we are waiting on the listeners, we can .Done() the waitgroup for the eventwaiter itself
				if e, ok := oneEvent.(syncEvent); ok {
					//fmt.Printf("ADD(1) in listener processEvent\n")
					e.Add(1)
				}
				l.singleEventInbox <- oneEvent
			}
		}
	} else {
		if l.match(event) {
			// TODO: separation of concerns: this should be factored out into a middleware of some sort...
			// now that we are waiting on the listeners, we can .Done() the waitgroup for the eventwaiter itself
			if e, ok := event.(syncEvent); ok {
				//fmt.Printf("ADD(1) in listener processEvent\n")
				e.Add(1)
			}
			l.singleEventInbox <- event
		}
	}
}

// Wait waits for the event to arrive.
func (l *EventListener) Wait(ctx context.Context) (eh.Event, error) {
	select {
	case event := <-l.singleEventInbox:
		if len(l.singleEventInbox) > 25 {
			l.startPrinting = true
		}
		if l.startPrinting && l.logger != nil {
			l.logger.Debug("Event Listener congestion", "len", len(l.singleEventInbox), "cap", cap(l.singleEventInbox), "name", l.Name)
		}
		if len(l.singleEventInbox) == 0 {
			l.startPrinting = false
		}

		// TODO: separation of concerns: this should be factored out into a middleware of some sort...
		// now that we are waiting on the listeners, we can .Done() the waitgroup for the eventwaiter itself
		if e, ok := event.(syncEvent); ok {
			defer e.Done()
			//defer fmt.Printf("Done in Wait()\n")
		}

		return event, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Inbox returns the channel that events will be delivered on so that you can integrate into your own select() if needed.
func (l *EventListener) Inbox() <-chan eh.Event {
	return l.singleEventInbox
}

// Close stops listening for more events.
func (l *EventListener) Close() {
	l.unregister <- l
}

// close the inbox
func (l *EventListener) closeInbox() {
	close(l.singleEventInbox)
}
