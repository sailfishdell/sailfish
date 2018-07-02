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
	sp, err := event.NewEventStreamProcessor(ctx, ew, event.CustomFilter(selectDataManagerEvent()))
	if err != nil {
		logger.Error("Failed to create event stream processor", "err", err)
		return err
	}
	sp.RunForever(func(event eh.Event) {
		if data, ok := event.Data().(dm_event.DataManagerEventData); ok {
			datamap, ok := data.(map[string]interface{})
			if !ok {
				logger.Warn("Data wasn't a map, odd..", "data", data)
				return
			}

			eventFqdd, ok := datamap["FQDD"]
			if !ok {
				// logger.Warn("Data map didn't have an FQDD, that's odd...", "data", data)
				return
			}

			if eventFqdd != fqdd {
				logger.Debug("This is not the event we are looking for.", "data", data)
				return
			}

			logger.Warn("Fan controller: Process Event", "data", data)

			if fanpwm, ok := datamap["fanpwm"]; ok {
				m.UpdateProperty("fanpwm", fanpwm)
			}
			if numrotors, ok := datamap["numrotors"]; ok {
				m.UpdateProperty("numrotors", numrotors)
			}
			if rotor1rpm, ok := datamap["rotor1rpm"]; ok {
				m.UpdateProperty("rotor1rpm", rotor1rpm)
			}
			if rotor2rpm, ok := datamap["rotor2rpm"]; ok {
				m.UpdateProperty("rotor2rpm", rotor2rpm)
			}
			if warningThreshold, ok := datamap["warningThreshold"]; ok {
				m.UpdateProperty("warningThreshold", warningThreshold)
			}
			if criticalThreshold, ok := datamap["criticalThreshold"]; ok {
				m.UpdateProperty("criticalThreshold", criticalThreshold)
			}

			/*
			   "map[
			       FQDD:System.Chassis.1#Fan.Slot.1
			       objFlags:8
			       objSize:138
			       objStatus:2
			       objType:3330
			       refreshInterval:0
			       thp_fan_data_object:map[
			           warningThreshold:1.936941416e+09
			           fanhealth:7
			           objExtFlags:108
			           rotor1rpm:1
			           rotor2rpm:1.953724755e+09
			           numrotors:0
			           criticalThreshold:1.127116133e+09
			           fanStateMask:8.25127785e+08
			           fanpwm:2.8992865226880465e-42
			           fanpwm_int:0]]
			*/

		} else {
			logger.Warn("Should never happen: got an invalid event in the event handler")
		}
	})

	return nil
}

func selectDataManagerEvent() func(eh.Event) bool {
	return func(event eh.Event) bool {
		if event.EventType() == dm_event.DataManagerEvent {
			return true
		}
		return false
	}
}
