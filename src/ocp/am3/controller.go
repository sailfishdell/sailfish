package am3

import (
	"context"
	"time"

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
)

const (
	ConfigureAM3Event = eh.EventType("ConfigureAM3Event")
	ConfigureAM3Multi = eh.EventType("ConfigureAM3Multi")
)

type ConfigureAM3EventData struct {
	serviceName string
	name        string
	et          eh.EventType
	fn          func(eh.Event)
}

type Service struct {
	logger        log.Logger
	eb            eh.EventBus
	eventhandlers map[eh.EventType]map[string]func(eh.Event)
	multihandlers map[eh.EventType]map[string]func(eh.Event)
	handledEvents map[eh.EventType]struct{}
	serviceName   string
}

func (s *Service) AddEventHandler(name string, et eh.EventType, fn func(eh.Event)) {
	s.eb.PublishEvent(context.Background(), eh.NewEvent(ConfigureAM3Event, &ConfigureAM3EventData{serviceName: s.serviceName, name: name, et: et, fn: fn}, time.Now()))
}

func (s *Service) AddMultiHandler(name string, et eh.EventType, fn func(eh.Event)) {
	s.eb.PublishEvent(context.Background(), eh.NewEvent(ConfigureAM3Multi, &ConfigureAM3EventData{serviceName: s.serviceName, name: name, et: et, fn: fn}, time.Now()))
}

type BusObjs interface {
	GetBus() eh.EventBus
	GetWaiter() *eventwaiter.EventWaiter
}

func StartService(ctx context.Context, logger log.Logger, name string, d BusObjs) (*Service, error) {
	var ret *Service
	ret = &Service{
		serviceName:   name,
		logger:        log.With(logger, "module", "am3"),
		eb:            d.GetBus(),
		handledEvents: map[eh.EventType]struct{}{ConfigureAM3Event: {}, ConfigureAM3Multi: {}},
		multihandlers: map[eh.EventType]map[string]func(eh.Event){},
		eventhandlers: map[eh.EventType]map[string]func(eh.Event){
			ConfigureAM3Event: {
				// These functions are run from inside the event loop to configure
				// things.  No need for locks as everything is guaranteed to be
				// single-threaded and not concurrently running
				"setup": func(ev eh.Event) {
					config := ev.Data().(*ConfigureAM3EventData)
					if config != nil && config.serviceName == ret.serviceName {
						h, ok := ret.eventhandlers[config.et]
						if !ok {
							h = map[string]func(eh.Event){}
						}
						h[config.name] = config.fn
						ret.eventhandlers[config.et] = h
					}
				},
			},
			ConfigureAM3Multi: {
				"setup": func(ev eh.Event) {
					config := ev.Data().(*ConfigureAM3EventData)
					if config != nil && config.serviceName == ret.serviceName {
						h, ok := ret.multihandlers[config.et]
						if !ok {
							h = map[string]func(eh.Event){}
						}
						h[config.name] = config.fn
						ret.multihandlers[config.et] = h
					}
				},
			},
		},
	}

	// stream processor for action events
	listener := eventwaiter.NewMultiListener(ctx, logger, d.GetWaiter(), func(ev eh.Event) bool {
		// normal case first: hash lookup to see if we process this event, should be the fastest way
		typ := ev.EventType()
		if _, ok := ret.handledEvents[typ]; ok {
			// self configure... no locks! yay!
			if typ == ConfigureAM3Event || typ == ConfigureAM3Multi {
				dataArray, _ := ev.Data().([]eh.EventData)
				for _, data := range dataArray {
					if d, ok := data.(*ConfigureAM3EventData); ok {
						ret.handledEvents[d.et] = struct{}{}
					}
				}
			}
			return true
		}
		return false
	})

	listener.Name = "am3"

	go func() {
		defer listener.Close()
		// ProcessEvents handles sync events .Done() for us. We don't need to care
		listener.ProcessEvents(ctx, func(event eh.Event) {
			t := event.EventType()
			for _, fn := range ret.eventhandlers[t] {
				for _, eventData := range event.Data().([]eh.EventData) {
					fn(eh.NewEvent(t, eventData, event.Timestamp()))
				}
			}

			for _, fn := range ret.multihandlers[t] {
				fn(event)
			}
		})
	}()

	return ret, nil
}
