package health_mapper

import (
	"context"

	eh "github.com/looplab/eventhorizon"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"

	"github.com/superchalupa/go-redfish/src/eventwaiter"
	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/event"
	"github.com/superchalupa/go-redfish/src/ocp/model"

	"github.com/superchalupa/go-redfish/src/dell-resources/dm_event"
)

type waiter interface {
	Listen(context.Context, func(eh.Event) bool) (*eventwaiter.EventListener, error)
}

var New func (logger log.Logger, m *model.Model, property string, fqdd string) (error)

func Setup(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus) {
	EventPublisher := eventpublisher.NewEventPublisher()
	eb.AddHandler(eh.MatchAny(), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter()
	EventPublisher.AddObserver(EventWaiter)

    New = func(logger log.Logger, m *model.Model, property string, fqdd string) error {
        return new(ctx, logger, m, property, fqdd, ch, eb, EventWaiter)
    }
}

func new(ctx context.Context, logger log.Logger, m *model.Model, property string, fqdd string, ch eh.CommandHandler, eb eh.EventBus, ew waiter) (error) {

    if _, ok := m.GetPropertyOkUnlocked(property); !ok {
        logger.Info("Model property does not exist, creating", "property", property, "FQDD", fqdd)
        m.UpdateProperty(property, "absent")
    }


	// stream processor for action events
	sp, err := event.NewEventStreamProcessor(ctx, ew, event.CustomFilter(SelectHealthEvent()))
	if err != nil {
		logger.Error("Failed to create event stream processor", "err", err)
		return err
	}
	sp.RunForever(func(event eh.Event) {
		if data, ok := event.Data().(*dm_event.HealthEventData); ok {
			logger.Debug("Process Event", "data", data)
            if data.FQDD == fqdd {
                logger.Info("Updating Model", "property", property, "data", data)
                m.UpdateProperty(property, data.Health)
            }
		} else {
			logger.Warn("Should never happen: got an invalid event in the event handler")
		}
	})

	return nil
}


func SelectHealthEvent() func(eh.Event) bool {
	return func(event eh.Event) bool {
		if event.EventType() == dm_event.HealthEvent {
			return true
		}
		return false
	}
}
