package eventservice

import (
	"context"
	"errors"
	"fmt"
	"time"

	eh "github.com/looplab/eventhorizon"
	"github.com/mitchellh/mapstructure"

	ah "github.com/superchalupa/sailfish/src/actionhandler"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func MakeSubmitTestEvent(eb eh.EventBus) func(context.Context, eh.Event, *domain.HTTPCmdProcessedData) error {
	return func(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
		domain.ContextLogger(ctx, "submit_event").Debug("got test event", "event_data", event.Data())

		data, ok := event.Data().(*ah.GenericActionEventData)
		if !ok {
			domain.ContextLogger(ctx, "submit_event").Crit("type assert failed", "event_data", event.Data(), "Type", fmt.Sprintf("%T", event.Data()))
			return errors.New("Didnt get the right kind of event")
		}

		redfishEvent := &RedfishEventData{}
		mapstructure.Decode(data.ActionData, redfishEvent)

		// need to publish here.
		responseEvent := eh.NewEvent(RedfishEvent, redfishEvent, time.Now())
		eb.PublishEvent(ctx, responseEvent)

		retData.Results = redfishEvent
		retData.StatusCode = 200
		return nil
	}
}
