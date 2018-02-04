package plugins

import (
    "context"
    "fmt"

        domain "github.com/superchalupa/go-redfish/redfishresource"

        eh "github.com/looplab/eventhorizon"
            "github.com/looplab/eventhorizon/utils"
)

type Options func(*privateStateStructure) error

type privateStateStructure struct {
    ctx context.Context
    filterFn func(eh.Event) bool
    listener *utils.EventListener
}

func NewEventStreamProcessor(ctx context.Context, ew *utils.EventWaiter, options... Options) (d *privateStateStructure, err error) {
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
	// wait for the root object to be created, then enhance it. Oneshot for now.
	go func() {
		defer d.Close()

		event, err := d.listener.Wait(d.ctx)
		if err != nil {
			fmt.Printf("Error waiting for event: %s\n", err.Error())
			return
		}

		fn(event)
	}()
}

func (d *privateStateStructure) RunForever(fn func(eh.Event)) {
	// wait for the root object to be created, then enhance it. Oneshot for now.
	go func() {
		defer d.Close()

        for {
            event, err := d.listener.Wait(d.ctx)
            if err != nil {
                fmt.Printf("Error waiting for event: %s\n", err.Error())
                return
            }
            fn(event)
        }
	}()
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
