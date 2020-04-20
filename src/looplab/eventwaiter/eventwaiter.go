// Copyright (c) 2017 - The Event Horizon authors.
// modifications Copyright (c) 2018 - Dell EMC
//  - don't drop events
//  - rework the api between waiter and listener so they aren't so incestuous
//  - rework api to be less circular
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
	"github.com/superchalupa/sailfish/src/log"
)

// TODO: accept override or read from config?
const (
	defaultWaiterQueueLen             = 200
	queueDefaultLoggingStartThreshold = 25
)

type Listener interface {
	GetID() eh.UUID
	ConsumeEventFromWaiter(event eh.Event)
	CloseInbox()
}

// EventWaiter waits for certain events to match a criteria.
type EventWaiter struct {
	name       string
	done       chan struct{}
	inbox      chan eh.Event
	register   chan Listener
	unregister chan Listener
	autorun    bool
	logger     log.Logger
}

type Option func(e *EventWaiter) error

// NewEventWaiter returns a new EventWaiter.
func NewEventWaiter(o ...Option) *EventWaiter {
	w := EventWaiter{
		done:       make(chan struct{}),
		inbox:      make(chan eh.Event, defaultWaiterQueueLen),
		register:   make(chan Listener),
		unregister: make(chan Listener),
		autorun:    true,
	}

	err := w.ApplyOption(o...)
	if err != nil && w.logger != nil {
		w.logger.Info("failed to apply option", "err", err)
	}

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
	listeners := map[eh.UUID]Listener{}
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
				l.CloseInbox()
			}
		case event := <-w.inbox:
			if len(w.inbox) > queueDefaultLoggingStartThreshold {
				startPrinting = true
			}
			if startPrinting && w.logger != nil {
				w.logger.Debug("Event Waiter congestion", "len", len(w.inbox), "cap", cap(w.inbox), "name", w.name)
			}
			if len(w.inbox) == 0 {
				startPrinting = false
			}
			for _, l := range listeners {
				l.ConsumeEventFromWaiter(event)
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
		e.Add(1)
	}

	w.inbox <- event
}

// Listen creates a new listener that will consume events from the waiter and call back for interesting ones
func (w *EventWaiter) Listen(ctx context.Context, match func(eh.Event) bool) (*EventListener, error) {
	return NewListener(ctx, w.logger, w, match), nil
}

func (w *EventWaiter) RegisterListener(l Listener) {
	w.register <- l
}

func (w *EventWaiter) UnRegisterListener(l Listener) {
	w.unregister <- l
}
