package fan_controller

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

var New func(logger log.Logger, m *model.Model, fqdd string) error

// this gets called once and the eventwaiter is re-used for all instances. (ie. no leaks)
// use this pattern to make it so that the New() function doesn't need to have a bunch of params
func Setup(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus) {
	EventPublisher := eventpublisher.NewEventPublisher()
	eb.AddHandler(eh.MatchAny(), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter()
	EventPublisher.AddObserver(EventWaiter)

	New = func(logger log.Logger, m *model.Model, fqdd string) error {
		return new(ctx, logger, m, fqdd, ch, eb, EventWaiter)
	}
}

func new(ctx context.Context, logger log.Logger, m *model.Model, fqdd string, ch eh.CommandHandler, eb eh.EventBus, ew waiter) error {

	// stream processor for action events
	sp, err := event.NewEventStreamProcessor(ctx, ew, event.CustomFilter(selectFanEvent()))
	if err != nil {
		logger.Error("Failed to create event stream processor", "err", err)
		return err
	}
	sp.RunForever(func(event eh.Event) {
		if data, ok := event.Data().(*dm_event.FanEventData); ok {
			logger.Debug("Process Event", "data", data)
			if data.ObjectHeader.FQDD == fqdd {
				m.UpdateProperty("Fanwpm", data.Fanpwm)
				m.UpdateProperty("Key", data.Key)
				m.UpdateProperty("FanName", data.FanName)
				m.UpdateProperty("Fanpwm_int", data.Fanpwm_int)
				m.UpdateProperty("VendorName", data.VendorName)
				m.UpdateProperty("WarningThreshold", data.WarningThreshold)
				m.UpdateProperty("DeviceName", data.DeviceName)
				m.UpdateProperty("TachName", data.TachName)
				m.UpdateProperty("CriticalThreshold", data.CriticalThreshold)
				m.UpdateProperty("Fanhealth", data.Fanhealth)
				m.UpdateProperty("Numrotors", data.Numrotors)
				m.UpdateProperty("Rotor2rpm", data.Rotor2rpm)
				m.UpdateProperty("Rotor1rpm", data.Rotor1rpm)
				m.UpdateProperty("FanStateMask", data.FanStateMask)
			}
		} else {
			logger.Warn("Should never happen: got an invalid event in the event handler")
		}
	})

	return nil
}

func selectFanEvent() func(eh.Event) bool {
	return func(event eh.Event) bool {
		if event.EventType() == dm_event.FanEvent {
			return true
		}
		return false
	}
}
