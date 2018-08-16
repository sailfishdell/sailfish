// Copyright (c) 2017 - The Event Horizon authors.
// modifications Copyright (c) 2018 - Dell EMC
//  - don't drop events
//  - major rewrite - get rid of two levels of channels because we weren't getting backpressure and things deadlock when listeners are slow
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
	"fmt"
	"sync"

	eh "github.com/looplab/eventhorizon"
)

// EventWaiter waits for certain events to match a criteria.
type EventWaiter struct {
	name        string
	listeners   map[eh.UUID]*EventListener
	listenersMu sync.RWMutex
}

type Option func(e *EventWaiter) error

// NewEventWaiter returns a new EventWaiter.
func NewEventWaiter(o ...Option) *EventWaiter {
	w := EventWaiter{
		listeners: make(map[eh.UUID]*EventListener),
	}

	w.ApplyOption(o...)

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

// Notify implements the eventhorizon.EventObserver.Notify method which forwards
// events to the waiters so that they can match the events.
func (w *EventWaiter) Notify(ctx context.Context, event eh.Event) {
	w.listenersMu.RLock()
	defer w.listenersMu.RUnlock()
	for _, l := range w.listeners {
		if l.match(event) {
			if len(l.inbox) > (cap(l.inbox) * 3 / 4) {
				fmt.Printf("LISTENER(%s) nearing capacity: %d of %d\n", l.Name, len(l.inbox), cap(l.inbox))
			}
			l.inbox <- event
		}
	}
}

// EventListener receives events from an EventWaiter.
type EventListener struct {
	Name  string
	id    eh.UUID
	inbox chan eh.Event
	match func(eh.Event) bool
	done  func()
}

// Listen waits unil the match function returns true for an event, or the context
// deadline expires. The match function can be used to filter or otherwise select
// interesting events by analysing the event data.
func (w *EventWaiter) Listen(ctx context.Context, match func(eh.Event) bool) (*EventListener, error) {
	id := eh.NewUUID()
	l := &EventListener{
		Name:  "unnamed",
		id:    id,
		inbox: make(chan eh.Event, 1000),
		match: match,
		done: func() {
			w.listenersMu.Lock()
			delete(w.listeners, id)
			w.listenersMu.Unlock()
		},
	}

	w.listenersMu.Lock()
	w.listeners[id] = l
	w.listenersMu.Unlock()

	return l, nil
}

// Wait waits for the event to arrive.
func (l *EventListener) Wait(ctx context.Context) (eh.Event, error) {
	select {
	case event := <-l.inbox:
		return event, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Inbox returns the channel that events will be delivered on so that you can integrate into your own select() if needed.
func (l *EventListener) Inbox() <-chan eh.Event {
	return l.inbox
}

// Close stops listening for more events.
func (l *EventListener) Close() {
	l.done()
	close(l.inbox)
}
