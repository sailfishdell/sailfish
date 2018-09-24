package dell_ec

import (
	"context"
	"fmt"
	"time"

	eh "github.com/looplab/eventhorizon"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"

	ah "github.com/superchalupa/sailfish/src/actionhandler"
	"github.com/superchalupa/sailfish/src/eventwaiter"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

// TODO: need a logger
//    -- > request logger? <-- probably. get an example here

func makePumpHandledAction(name string, maxtimeout int, eb eh.EventBus) func(context.Context, eh.Event, *domain.HTTPCmdProcessedData) error {
	EventPublisher := eventpublisher.NewEventPublisher()

	// TODO: fix MatchAny
	eb.AddHandler(eh.MatchEvent(domain.HTTPCmdProcessed), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter(eventwaiter.SetName("Action Timeout Publisher"))
	EventPublisher.AddObserver(EventWaiter)

	return func(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
		// The actionhandler will discard the message if we set statuscode to 0. Client should never see it, and pump can send its own return
		retData.StatusCode = 0
		ourCmdID := retData.CommandID
		fmt.Printf("\n\nGot an action that we expect PUMP to handle. We'll set up a timeout to make sure that happens: %s.\n\n", ourCmdID)

		listener, err := EventWaiter.Listen(ctx, func(event eh.Event) bool {
			if event.EventType() != domain.HTTPCmdProcessed {
				return false
			}
			data, ok := event.Data().(*domain.HTTPCmdProcessedData)
			if ok && data.CommandID == ourCmdID {
				return true
			}
			return false
		})
		if err != nil {
			return err
		}
		listener.Name = "Pump handled action listener: " + name

		go func() {
			defer listener.Close()
			inbox := listener.Inbox()
			timer := time.NewTimer(time.Duration(maxtimeout) * time.Second)
			for {
				select {
				case <-inbox:
					// got an event from the pump with our exact cmdid, we are done
					return

				case <-timer.C:
					eventData := &domain.HTTPCmdProcessedData{
						CommandID:  event.Data().(*ah.GenericActionEventData).CmdID,
						Results:    map[string]interface{}{"msg": "Timed Out!"},
						StatusCode: 500,
						Headers:    map[string]string{},
					}
					responseEvent := eh.NewEvent(domain.HTTPCmdProcessed, eventData, time.Now())
					eb.PublishEvent(ctx, responseEvent)

				// user cancelled curl request before we could get a response
				case <-ctx.Done():
					return
				}
			}
		}()

		return nil
	}
}

func exportSystemConfiguration(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
	fmt.Printf("\n\nBMC Export System Configuration\n\n")
	retData.Results = map[string]interface{}{"msg": "BMC Export System Configuration!"}
	retData.StatusCode = 200
	return nil
}
func importSystemConfiguration(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
	fmt.Printf("\n\nBMC Import System Configuration\n\n")
	retData.Results = map[string]interface{}{"msg": "BMC Import System Configuration!"}
	retData.StatusCode = 200
	return nil
}
func importSystemConfigurationPreview(ctx context.Context, event eh.Event, retData *domain.HTTPCmdProcessedData) error {
	fmt.Printf("\n\nBMC Import System Configuration Preview\n\n")
	retData.Results = map[string]interface{}{"msg": "BMC Import System Configuration Preview!"}
	retData.StatusCode = 200
	return nil
}
