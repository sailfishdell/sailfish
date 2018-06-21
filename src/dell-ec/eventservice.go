package dell_ec

import (
	"context"
    "errors"
    "fmt"
    "time"

	eh "github.com/looplab/eventhorizon"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
    ah "github.com/superchalupa/go-redfish/src/actionhandler"
)

const (
	RedfishEvent = eh.EventType("RedfishEvent")
)

func InitService(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew waiter) {
	eh.RegisterEventData(RedfishEvent, func() eh.EventData { return &RedfishEventData{} })
}

type RedfishEventData struct {
    EventType string
    EventId   string
    EventTimestamp  string
    Severity string
    Message  string
    MessageId string
    MessageArgs []string
    OriginOfCondition string
}

func MakeSubmitTestEvent(eb eh.EventBus) func(context.Context, eh.Event, *domain.HTTPCmdProcessedData) error {
    return func (ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
        domain.ContextLogger(ctx, "submit_event").Debug("got test event", "event_data", event.Data())

        data, ok := event.Data().(ah.GenericActionEventData)
        if ! ok {
            domain.ContextLogger(ctx, "submit_event").Crit("type assert failed", "event_data", event.Data(), "Type", fmt.Sprintf("%T", event.Data()) )
            return errors.New("Didnt get the right kind of event")
        }

        // need to publish here.
        responseEvent := eh.NewEvent(RedfishEvent, data.ActionData, time.Now())
        eb.PublishEvent(ctx, responseEvent)

        retData.Results = data.ActionData
        retData.StatusCode = 200
        return nil
    }
}
