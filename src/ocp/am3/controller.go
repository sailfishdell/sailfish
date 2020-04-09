package am3

import (
	"context"
	"time"

	"golang.org/x/xerrors"

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
)

const (
	ConfigureAM3Event = eh.EventType("ConfigureAM3Event")
	ConfigureAM3Multi = eh.EventType("ConfigureAM3Multi")
	ShutdownAM3       = eh.EventType("AM3Shutdown")
)

type ConfigureAM3EventData struct {
	serviceName string
	name        string
	et          eh.EventType
	fn          func(eh.Event)
}

type ShutdownAM3Data struct {
	serviceName string
}

type Service struct {
	logger        log.Logger
	eb            eh.EventBus
	eventhandlers map[eh.EventType]map[string]func(eh.Event)
	multihandlers map[eh.EventType]map[string]func(eh.Event)
	handledEvents map[eh.EventType]struct{}
	serviceName   string
	listener      *eventwaiter.MultiEventListener
}

func (s *Service) AddEventHandler(name string, et eh.EventType, fn func(eh.Event)) error {
	err := s.eb.PublishEvent(
		context.Background(),
		eh.NewEvent(ConfigureAM3Event, &ConfigureAM3EventData{serviceName: s.serviceName, name: name, et: et, fn: fn}, time.Now()))
	if err != nil {
		return xerrors.Errorf("Error publishing event to add handler(%s) to AM3(%s): %w", name, s.serviceName, err)
	}
	return nil
}

func (s *Service) AddMultiHandler(name string, et eh.EventType, fn func(eh.Event)) error {
	err := s.eb.PublishEvent(
		context.Background(),
		eh.NewEvent(ConfigureAM3Multi, &ConfigureAM3EventData{serviceName: s.serviceName, name: name, et: et, fn: fn}, time.Now()))
	if err != nil {
		return xerrors.Errorf("Error publishing event to add handler(%s) to AM3(%s): %w", name, s.serviceName, err)
	}
	return nil
}

func (s *Service) Shutdown() error {
	err := s.eb.PublishEvent(context.Background(), eh.NewEvent(ShutdownAM3, &ShutdownAM3Data{serviceName: s.serviceName}, time.Now()))
	if err != nil {
		return xerrors.Errorf("Error publishing event to shut down AM3(%s): %w", s.serviceName, err)
	}
	return nil
}

type BusObjs interface {
	GetBus() eh.EventBus
	GetWaiter() *eventwaiter.EventWaiter
}

func (s *Service) inlineAddHandler(config *ConfigureAM3EventData) {
	if config != nil && config.serviceName == s.serviceName {
		h, ok := s.eventhandlers[config.et]
		if !ok {
			h = map[string]func(eh.Event){}
		}
		h[config.name] = config.fn
		s.eventhandlers[config.et] = h
	}
}
func (s *Service) inlineAddMultiHandler(config *ConfigureAM3EventData) {
	if config != nil && config.serviceName == s.serviceName {
		h, ok := s.multihandlers[config.et]
		if !ok {
			h = map[string]func(eh.Event){}
		}
		h[config.name] = config.fn
		s.multihandlers[config.et] = h
	}
}

func (s *Service) inlineProcessEvent(event eh.Event) {
	t := event.EventType()
	for _, fn := range s.eventhandlers[t] {
		for _, eventData := range event.Data().([]eh.EventData) {
			fn(eh.NewEvent(t, eventData, event.Timestamp()))
		}
	}

	for _, fn := range s.multihandlers[t] {
		fn(event)
	}
}

func (s *Service) inlineCheckEvent(ev eh.Event) bool {
	// normal case first: hash lookup to see if we process this event, should be the fastest way
	typ := ev.EventType()
	if _, ok := s.handledEvents[typ]; ok {
		// self configure... no locks! yay!
		if typ == ConfigureAM3Event || typ == ConfigureAM3Multi {
			dataArray, _ := ev.Data().([]eh.EventData)
			for _, data := range dataArray {
				if d, ok := data.(*ConfigureAM3EventData); ok && d.serviceName == s.serviceName {
					s.handledEvents[d.et] = struct{}{}
				}
			}
		}
		return true
	}
	return false
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
					config, _ := ev.Data().(*ConfigureAM3EventData) // non-panic()-ing type assert, check nil later
					ret.inlineAddHandler(config)
				},
			},
			ConfigureAM3Multi: {
				"setup": func(ev eh.Event) {
					config, _ := ev.Data().(*ConfigureAM3EventData) // non-panic()-ing type assert, check nil later
					ret.inlineAddMultiHandler(config)
				},
			},
			ShutdownAM3: {
				"setup": func(ev eh.Event) {
					config, ok := ev.Data().(*ShutdownAM3Data)
					if ok && config != nil && config.serviceName == ret.serviceName {
						ret.listener.Close()
					}
				},
			},
		},
	}

	// stream processor for action events
	ret.listener = eventwaiter.NewMultiListener(ctx, logger, d.GetWaiter(), ret.inlineCheckEvent)
	ret.listener.Name = "am3"

	go func() {
		defer ret.listener.Close()
		err := ret.listener.ProcessEvents(ctx, ret.inlineProcessEvent)
		if err != nil {
			logger.Crit("ProcessEvents for AM3 returned an error", "name", ret.serviceName, "err", err)
		}
	}()

	return ret, nil
}
