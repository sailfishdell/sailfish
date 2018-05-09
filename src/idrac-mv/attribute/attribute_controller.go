package attribute

import (
	"context"
	"fmt"

	"github.com/superchalupa/go-redfish/src/log"
	plugins "github.com/superchalupa/go-redfish/src/ocp"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

const (
	AttributeUpdated eh.EventType = "AttributeUpdated"
)

func init() {
	eh.RegisterEventData(AttributeUpdated, func() eh.EventData { return &AttributeUpdatedData{} })
}

type AttributeUpdatedData struct {
	FQDD  string
	Group string
	Index string
	Name  string
	Value interface{}
}

func (s *service) AddController(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	// stream processor for action events
	sp, err := plugins.NewEventStreamProcessor(ctx, ew, plugins.CustomFilter(SelectAttributeUpdate(s.fqdd)))
	if err != nil {
		log.MustLogger("idrac_mv").Error("Failed to create event stream processor", "err", err)
		return
	}
	sp.RunForever(func(event eh.Event) {
		log.MustLogger("idrac_mv").Info("Got action event", "event", event)
		if data, ok := event.Data().(*AttributeUpdatedData); ok {
			s.ApplyOption(WithAttribute(data.Group, data.Index, data.Name, data.Value))
		} else {
			log.MustLogger("idrac_mv").Warn("Should never happen: got an invalid event in the event handler")
		}
	})
}

func SelectAttributeUpdate(fqdd string) func(eh.Event) bool {
	return func(event eh.Event) bool {
		log.MustLogger("idrac_mv").Debug("Checking event", "event", event)
		if event.EventType() != AttributeUpdated {
			log.MustLogger("idrac_mv").Debug("no match: type")
			return false
		}
		if data, ok := event.Data().(*AttributeUpdatedData); ok {
			if data.FQDD == fqdd {
				log.MustLogger("idrac_mv").Debug("FQDD MATCH")
				return true
			}
			log.MustLogger("idrac_mv").Debug("FQDD FAIL")
			return false
		}
		log.MustLogger("idrac_mv").Debug("TYPE ASSERT FAIL!", "data", fmt.Sprintf("%#v", event.Data()))
		return false
	}
}
