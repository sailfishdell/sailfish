package event

import (
	"context"
	"strings"

	"github.com/superchalupa/go-redfish/src/eventwaiter"
	"github.com/superchalupa/go-redfish/src/log"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	eh "github.com/looplab/eventhorizon"
)

type Options func(*privateStateStructure) error

type listener interface {
	Wait(context.Context) (eh.Event, error)
	Close()
}

type waiter interface {
	Listen(context.Context, func(eh.Event) bool) (*eventwaiter.EventListener, error)
}

type privateStateStructure struct {
	ctx      context.Context
	filterFn func(eh.Event) bool
	listener listener
}

func NewEventStreamProcessor(ctx context.Context, ew waiter, options ...Options) (d *privateStateStructure, err error) {
	d = &privateStateStructure{
		ctx: ctx,
		// default filter is to process all events
		filterFn: func(eh.Event) bool { return true },
	}
	err = nil

	for _, o := range options {
		err := o(d)
		if err != nil {
			return nil, err
		}
	}

	// set up listener that will fire when it sees /redfish/v1 created
	d.listener, err = ew.Listen(ctx, d.filterFn)
	if err != nil {
		return
	}

	return
}

func (d *privateStateStructure) Close() {
	if d.listener != nil {
		d.listener.Close()
		d.listener = nil
	}
}

func (d *privateStateStructure) RunOnce(fn func(eh.Event)) {
	go func() {
		defer d.Close()

		event, err := d.listener.Wait(d.ctx)
		if err != nil {
			log.MustLogger("eventstream").Info("Shutting down listener", "err", err)
			return
		}

		fn(event)
	}()
}

func (d *privateStateStructure) RunForever(fn func(eh.Event)) {
	go func() {
		defer d.Close()

		for {
			event, err := d.listener.Wait(d.ctx)
			if err != nil {
				log.MustLogger("eventstream").Info("Shutting down listener", "err", err)
				return
			}
			fn(event)
		}
	}()
}

func CustomFilter(fn func(eh.Event) bool) func(p *privateStateStructure) error {
	return func(p *privateStateStructure) error {
		p.filterFn = fn
		return nil
	}
}

func AND(fnA, fnB func(eh.Event) bool) func(p *privateStateStructure) error {
	return func(p *privateStateStructure) error {
		p.filterFn = func(e eh.Event) bool { return fnA(e) && fnB(e) }
		return nil
	}
}

func OR(fnA, fnB func(eh.Event) bool) func(p *privateStateStructure) error {
	return func(p *privateStateStructure) error {
		p.filterFn = func(e eh.Event) bool { return fnA(e) || fnB(e) }
		return nil
	}
}

func SelectEventResourceCreatedByURI(uri string) func(p *privateStateStructure) error {
	return func(p *privateStateStructure) error {
		p.filterFn = func(event eh.Event) bool {
			if event.EventType() != domain.RedfishResourceCreated {
				return false
			}
			if data, ok := event.Data().(domain.RedfishResourceCreatedData); ok {
				if data.ResourceURI == uri {
					return true
				}
			}
			return false
		}
		return nil
	}
}

func SelectEventResourceCreatedByURIPrefix(uri string) func(p *privateStateStructure) error {
	return func(p *privateStateStructure) error {
		p.filterFn = func(event eh.Event) bool {
			if event.EventType() != domain.RedfishResourceCreated {
				return false
			}
			if data, ok := event.Data().(domain.RedfishResourceCreatedData); ok {
				if strings.HasPrefix(data.ResourceURI, uri) {
					// Only return true for immediate sub uris, not grandchildren
					rest := strings.TrimPrefix(data.ResourceURI, uri)
					if strings.Contains(rest, "/") {
						return false
					}
					return true
				}
			}
			return false
		}
		return nil
	}
}

func SelectEventResourceRemovedByURI(uri string) func(p *privateStateStructure) error {
	return func(p *privateStateStructure) error {
		p.filterFn = func(event eh.Event) bool {
			if event.EventType() != domain.RedfishResourceRemoved {
				return false
			}
			if data, ok := event.Data().(domain.RedfishResourceRemovedData); ok {
				if data.ResourceURI == uri {
					return true
				}
			}
			return false
		}
		return nil
	}
}

func SelectEventResourceRemovedByURIPrefix(uri string) func(p *privateStateStructure) error {
	return func(p *privateStateStructure) error {
		p.filterFn = func(event eh.Event) bool {
			if event.EventType() != domain.RedfishResourceRemoved {
				return false
			}
			if data, ok := event.Data().(domain.RedfishResourceRemovedData); ok {
				if strings.HasPrefix(data.ResourceURI, uri) {
					// Only return true for immediate sub uris, not grandchildren
					rest := strings.TrimPrefix(data.ResourceURI, uri)
					if strings.Contains(rest, "/") {
						return false
					}
					return true
				}
			}
			return false
		}
		return nil
	}
}

func OnURICreated(ctx context.Context, ew waiter, uri string, f func()) {
	sp, err := NewEventStreamProcessor(ctx, ew, SelectEventResourceCreatedByURI(uri))
	if err != nil {
		log.MustLogger("eventstream").Error("Failed to create event stream processor", "err", err)
		return
	}
	sp.RunOnce(func(event eh.Event) {
		f()
	})
}
