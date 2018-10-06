// Copyright (c) 2017 - The Event Horizon authors.
// modifications Copyright (c) 2018 - Dell EMC
//  - don't drop events
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
)

type listener interface {
	GetID() eh.UUID
	processEvent(event eh.Event)
	closeInbox()
}

// EventWaiter waits for certain events to match a criteria.
type EventWaiter struct {
	name       string
	inbox      chan eh.Event
	register   chan listener
	unregister chan listener
}

type Option func(e *EventWaiter) error

// NewEventWaiter returns a new EventWaiter.
func NewEventWaiter(o ...Option) *EventWaiter {
	w := EventWaiter{
		inbox:      make(chan eh.Event, 100),
		register:   make(chan listener),
		unregister: make(chan listener),
	}

	w.ApplyOption(o...)

	go w.run()
	return &w
}

func SetName(name string) Option {
	return func(w *EventWaiter) error {
		w.name = name
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

func (w *EventWaiter) run() {
	listeners := map[eh.UUID]listener{}
	for {
		select {
		case l := <-w.register:
			listeners[l.GetID()] = l
		case l := <-w.unregister:
			// Check for existence to avoid closing channel twice.
			if _, ok := listeners[l.GetID()]; ok {
				delete(listeners, l.GetID())
				l.closeInbox()
			}
		case event := <-w.inbox:
			for _, l := range listeners {
				l.processEvent(event)
			}
		}
	}
}

// Notify implements the eventhorizon.EventObserver.Notify method which forwards
// events to the waiters so that they can match the events.
func (w *EventWaiter) Notify(ctx context.Context, event eh.Event) {
	w.inbox <- event
}

// Listen waits unil the match function returns true for an event, or the context
// deadline expires. The match function can be used to filter or otherwise select
// interesting events by analysing the event data.
func (w *EventWaiter) Listen(ctx context.Context, match func(eh.Event) bool) (*EventListener, error) {
	l := &EventListener{
		Name:             "unnamed",
		id:               eh.NewUUID(),
		singleEventInbox: make(chan eh.Event, 100),
		match:            match,
		unregister:       w.unregister,
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
			oneEvent := eh.NewEvent(t, data, event.Timestamp())
			if l.match(oneEvent) {
				l.singleEventInbox <- oneEvent
			}
		}
	} else {
		if l.match(event) {
			l.singleEventInbox <- event
		}
	}
}

// Wait waits for the event to arrive.
func (l *EventListener) Wait(ctx context.Context) (eh.Event, error) {
	select {
	case event := <-l.singleEventInbox:
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
