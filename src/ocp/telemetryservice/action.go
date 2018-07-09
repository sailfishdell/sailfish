package telemetryservice

import (
	"context"
	"errors"
	"fmt"
	"time"

	eh "github.com/looplab/eventhorizon"

	ah "github.com/superchalupa/go-redfish/src/actionhandler"
	"github.com/superchalupa/go-redfish/src/ocp/eventservice"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

func MakeSubmitTestMetricReport(eb eh.EventBus) func(context.Context, eh.Event, *domain.HTTPCmdProcessedData) error {
	return func(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
		domain.ContextLogger(ctx, "submit_event").Debug("got test metric report event", "event_data", event.Data())

		data, ok := event.Data().(*ah.GenericActionEventData)
		if !ok {
			domain.ContextLogger(ctx, "submit_event").Crit("type assert failed", "event_data", event.Data(), "Type", fmt.Sprintf("%T", event.Data()))
			return errors.New("Didnt get the right kind of event")
		}

		// need to publish here.
		responseEvent := eh.NewEvent(eventservice.ExternalRedfishEvent, &data.ActionData, time.Now())
		eb.PublishEvent(ctx, responseEvent)

		retData.Results = data.ActionData
		retData.StatusCode = 200
		return nil
	}
}
