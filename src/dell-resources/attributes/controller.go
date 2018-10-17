package attributes

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/event"
	"github.com/superchalupa/sailfish/src/ocp/model"

	eh "github.com/looplab/eventhorizon"
)

type ARDump struct {
	fqdds []string
	eb    eh.EventBus
}

func NewController(ctx context.Context, m *model.Model, fqdds []string, ch eh.CommandHandler, eb eh.EventBus) (*ARDump, error) {
	c := &ARDump{
		fqdds: fqdds,
		eb:    eb,
	}

	// stream processor for action events
	sp, err := event.NewESP(ctx, event.CustomFilter(selectAttributeUpdate(fqdds)), event.SetListenerName("attributes"))
	if err != nil {
		log.MustLogger("ARDump_Controller").Error("Failed to create event stream processor", "err", err)
		return nil, errors.New("Failed to create stream processor")
	}
	go sp.RunForever(func(event eh.Event) {
		log.MustLogger("ARDump_Controller").Info("Updating model attribute", "event", event)
		if data, ok := event.Data().(*AttributeUpdatedData); ok {
			m.ApplyOption(WithAttribute(data.Group, data.Index, data.Name, data.Value))
		} else {
			log.MustLogger("ARDump_Controller").Warn("Should never happen: got an invalid event in the event handler")
		}
	})

	return c, nil
}

func (d *ARDump) Close() {
	// TODO: Need to fixup this controller so that it can be cleanly shut down without leaking
	panic("ARDump controller currently does not support shutting down and will leak memory. This function should never be called.")
}

func (d *ARDump) UpdateRequest(ctx context.Context, property string, value interface{}) (interface{}, error) {
	log.MustLogger("ARDump_Controller").Debug("UpdateRequest", "property", property, "value", value)

	for k, v := range value.(map[string]interface{}) {
		stuff := strings.Split(k, ".")
		reqUUID := eh.NewUUID()

		// TODO: validate
		//  - validate that the requested member is in this list
		//  - validate that it is writable
		//  - validate that user has perms
		//
		data := &AttributeUpdateRequestData{
			ReqID: reqUUID,
			FQDD:  d.fqdds[0],
			Group: stuff[0],
			Index: stuff[1],
			Name:  stuff[2],
			Value: v,
		}
		d.eb.PublishEvent(ctx, eh.NewEvent(AttributeUpdateRequest, data, time.Now()))
	}
	return nil, nil
}

func selectAttributeUpdate(fqdd []string) func(eh.Event) bool {
	return func(event eh.Event) bool {
		if event.EventType() == AttributeUpdated {
			if data, ok := event.Data().(*AttributeUpdatedData); ok {
				for _, testFQDD := range fqdd {
					if data.FQDD == testFQDD {
						return true
					}
				}
				return false
			}
		}
		return false
	}
}
