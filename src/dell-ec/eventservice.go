package dell_ec

import (
	"context"
    "errors"
    "fmt"
    "time"

    "github.com/mitchellh/mapstructure"
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

        redfishEvent := &RedfishEventData{}
        mapstructure.Decode( data.ActionData, redfishEvent )

        // Require EventType and EventID or else we bail
        if redfishEvent.EventType == "" || redfishEvent.EventId == "" {
            retData.Results = map[string]interface{}{"error": "Bad request"}
            retData.StatusCode = 400
            return errors.New("Did not get a valid redfish event to publish")
        }

        // need to publish here.
        responseEvent := eh.NewEvent(RedfishEvent, redfishEvent, time.Now())
        eb.PublishEvent(ctx, responseEvent)

        retData.Results = redfishEvent
        retData.StatusCode = 200
        return nil
    }
}
