package am3

import (
	"context"

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/event"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
)

const (
	ConfigureAM3Event = eh.EventType("ConfigureAM3Event")
	ConfigureAM3Multi = eh.EventType("ConfigureAM3Multi")
	ShutdownAM3       = eh.EventType("AM3Shutdown")
)

type EventHandler func(eh.Event)
type Service interface {
	AddEventHandler(string, eh.EventType, EventHandler) error
	AddMultiHandler(string, eh.EventType, EventHandler) error
}

type ConfigureAM3EventData struct {
	serviceName string
	name        string
	et          eh.EventType
	fn          EventHandler
}

type ShutdownAM3Data struct {
	serviceName string
}

type AwesomeMapper3 struct {
	logger        log.Logger
	eb            eh.EventBus
	eventhandlers map[eh.EventType]map[string]EventHandler
	multihandlers map[eh.EventType]map[string]EventHandler
	handledEvents map[eh.EventType]struct{}
	serviceName   string
	listener      *eventwaiter.MultiEventListener
}

func (s *AwesomeMapper3) AddEventHandler(name string, et eh.EventType, fn EventHandler) error {
	return event.PublishAndWaitErr(
		context.Background(),
		s.eb,
		ConfigureAM3Event,
		&ConfigureAM3EventData{serviceName: s.serviceName, name: name, et: et, fn: fn},
	)
}

func (s *AwesomeMapper3) AddMultiHandler(name string, et eh.EventType, fn EventHandler) error {
	return event.PublishAndWaitErr(
		context.Background(),
		s.eb,
		ConfigureAM3Multi,
		&ConfigureAM3EventData{serviceName: s.serviceName, name: name, et: et, fn: fn},
	)
}

func (s *AwesomeMapper3) Shutdown() error {
	return event.PublishAndWaitErr(
		context.Background(),
		s.eb,
		ShutdownAM3,
		&ShutdownAM3Data{serviceName: s.serviceName},
	)
}

type BusObjs interface {
	GetBus() eh.EventBus
	GetWaiter() *eventwaiter.EventWaiter
}

func (s *AwesomeMapper3) inlineAddHandler(config *ConfigureAM3EventData) {
	if config == nil && config.serviceName != s.serviceName {
		return
	}
	h, ok := s.eventhandlers[config.et]
	if !ok {
		h = map[string]EventHandler{}
	}
	h[config.name] = config.fn
	s.eventhandlers[config.et] = h
}
func (s *AwesomeMapper3) inlineAddMultiHandler(config *ConfigureAM3EventData) {
	if config == nil && config.serviceName != s.serviceName {
		return
	}
	h, ok := s.multihandlers[config.et]
	if !ok {
		h = map[string]EventHandler{}
	}
	h[config.name] = config.fn
	s.multihandlers[config.et] = h
}

func (s *AwesomeMapper3) inlineProcessEvent(evt eh.Event) {
	t := evt.EventType()
	for _, fn := range s.eventhandlers[t] {
		for _, eventData := range evt.Data().([]eh.EventData) {
			fn(eh.NewEvent(t, eventData, evt.Timestamp()))
		}
	}

	for _, fn := range s.multihandlers[t] {
		fn(evt)
	}
}

func (s *AwesomeMapper3) inlineCheckEvent(evt eh.Event) (ret bool) {
	// normal case first: hash lookup to see if we process this event, should be the fastest way
	typ := evt.EventType()
	if _, ok := s.handledEvents[typ]; ok {
		// self configure... no locks! yay!
		if typ == ConfigureAM3Event || typ == ConfigureAM3Multi {
			dataArray, _ := evt.Data().([]eh.EventData)
			for _, data := range dataArray {
				d, ok := data.(*ConfigureAM3EventData)
				if !ok || d.serviceName != s.serviceName {
					return false
				}
				s.handledEvents[d.et] = struct{}{}
			}
		}
		return true
	}
	return false
}

func StartService(ctx context.Context, logger log.Logger, name string, d BusObjs) (*AwesomeMapper3, error) {
	var ret *AwesomeMapper3
	ret = &AwesomeMapper3{
		serviceName:   name,
		logger:        log.With(logger, "module", "am3"),
		eb:            d.GetBus(),
		handledEvents: map[eh.EventType]struct{}{ConfigureAM3Event: {}, ConfigureAM3Multi: {}},
		multihandlers: map[eh.EventType]map[string]EventHandler{},
		eventhandlers: map[eh.EventType]map[string]EventHandler{
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
	ret.listener.Name = "am3-" + name

	go func() {
		defer ret.listener.Close()
		err := ret.listener.ProcessEvents(ctx, ret.inlineProcessEvent)
		if err != nil {
			logger.Crit("ProcessEvents for AM3 returned an error", "name", ret.serviceName, "err", err)
		}
	}()

	return ret, nil
}
