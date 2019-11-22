package am3

import (
	"context"
	"errors"
	"time"

	eh "github.com/looplab/eventhorizon"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
	"github.com/superchalupa/sailfish/src/ocp/event"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

const (
	ConfigureAM3Event = eh.EventType("ConfigureAM3Event")
)

func init() {
	eh.RegisterEventData(ConfigureAM3Event, func() eh.EventData { return &ConfigureAM3EventData{} })
}

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

func StartService(ctx context.Context, logger log.Logger, eb eh.EventBus, ch eh.CommandHandler, d *domain.DomainObjects) (*Service, error) {
	EventPublisher := eventpublisher.NewEventPublisher()

	// TODO: we should change MatchAny to filter here... benefit? Should measure that.
	eb.AddHandler(eh.MatchAny(), EventPublisher)

	EventWaiter := eventwaiter.NewEventWaiter(eventwaiter.SetName("Awesome Mapper2"), eventwaiter.NoAutoRun)
	EventPublisher.AddObserver(EventWaiter)
	go EventWaiter.Run()

	var ret *Service
	ret = &Service{
		logger:        logger.New("module", "am2"),
		eb:            eb,
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
	filter := event.CustomFilter(
		func(ev eh.Event) bool {
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
		})

	sp, err := event.NewESP(ctx, filter, event.SetListenerName("am3"))

	if err != nil {
		ret.logger.Error("Failed to create event stream processor", "err", err)
		return nil, errors.New("")
	}

	go sp.RunForever(func(event eh.Event) {
		for name, fn := range ret.eventhandlers[event.EventType()] {
			ret.logger.Info("Running handler", "name", name)
			fn(event)
		}
	})

	return ret, nil
}
