package am3

import (
	"context"
	"errors"
	"time"

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
)

const (
	ConfigureAM3Event = eh.EventType("ConfigureAM3Event")
)

type ConfigureAM3EventData struct {
	name string
	et   eh.EventType
	fn   func(eh.Event)
}

type Service struct {
	logger        log.Logger
	eb            eh.EventBus
	eventhandlers map[eh.EventType]map[string]func(eh.Event)
	handledEvents map[eh.EventType]struct{}
}

func (s *Service) AddEventHandler(name string, et eh.EventType, fn func(eh.Event)) {
	s.eb.PublishEvent(context.Background(), eh.NewEvent(ConfigureAM3Event, &ConfigureAM3EventData{name: name, et: et, fn: fn}, time.Now()))
}

type waiter interface {
	Listen(context.Context, func(eh.Event) bool) (*eventwaiter.EventListener, error)
}

type syncEvent interface {
	Done()
}

type BusObjs interface {
	GetBus() eh.EventBus
	GetWaiter() *eventwaiter.EventWaiter
	GetPublisher() eh.EventPublisher
}

func StartService(ctx context.Context, logger log.Logger, d BusObjs) (*Service, error) {
	eh.RegisterEventData(ConfigureAM3Event, func() eh.EventData { return &ConfigureAM3EventData{} })
	var ret *Service
	ret = &Service{
		logger:        logger.New("module", "am2"),
		eb:            d.GetBus(),
		handledEvents: map[eh.EventType]struct{}{ConfigureAM3Event: struct{}{}},
		eventhandlers: map[eh.EventType]map[string]func(eh.Event){
			ConfigureAM3Event: map[string]func(eh.Event){
				// This function is run from inside the event loop to configure things.
				// No need for locks as everything is guaranteed to be single-threaded
				// and not concurrently running
				"setup": func(ev eh.Event) {
					config := ev.Data().(*ConfigureAM3EventData)
					if config != nil {
						h, ok := ret.eventhandlers[config.et]
						if !ok {
							h = map[string]func(eh.Event){}
						}
						h[config.name] = config.fn
						ret.eventhandlers[config.et] = h
					}
				},
			},
		},
	}

	// stream processor for action events
	filter := func(ev eh.Event) bool {
		// normal case first: hash lookup to see if we process this event, should be the fastest way
		typ := ev.EventType()
		if _, ok := ret.handledEvents[typ]; ok {
			// self configure... no locks! yay!
			if typ == ConfigureAM3Event {
				data, ok := ev.Data().(*ConfigureAM3EventData)
				if ok {
					ret.handledEvents[data.et] = struct{}{}
				}
			}

			return true
		}

		return false
	}

	listener, err := d.GetWaiter().Listen(ctx, filter)
	if err != nil {
		return nil, errors.New("Couldnt listen")
	}
	listener.Name = "am3"

	go func() {
		defer listener.Close()
		keepRunning := true
		for keepRunning {
			func() {
				event, err := listener.UnSyncWait(ctx)
				if e, ok := event.(syncEvent); ok {
					defer e.Done()
				}
				if err != nil {
					log.MustLogger("eventstream").Info("Shutting down listener", "err", err)
					keepRunning = false
					return
				}

				for name, fn := range ret.eventhandlers[event.EventType()] {
					ret.logger.Info("Running handler", "name", name)
					fn(event)
				}

			}()
		}
	}()

	return ret, nil
}
