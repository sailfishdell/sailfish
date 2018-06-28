package eventservice

import (
	"context"
	"reflect"
	"time"

	eh "github.com/looplab/eventhorizon"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"

	"github.com/superchalupa/go-redfish/src/eventwaiter"
	"github.com/superchalupa/go-redfish/src/log"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

const MaxEventsQueued = 10
const QueueTime = 100 * time.Millisecond

type waiter interface {
	Listen(context.Context, func(eh.Event) bool) (*eventwaiter.EventListener, error)
}

// PublishRedfishEvents starts a background goroutine to collage internal
// redfish events for external consumption
func PublishRedfishEvents(ctx context.Context, eb eh.EventBus) error {

	EventPublisher := eventpublisher.NewEventPublisher()
	eb.AddHandler(eh.MatchAny(), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter()
	EventPublisher.AddObserver(EventWaiter)

	listener, err := EventWaiter.Listen(ctx, selectRedfishEvent)
	if err != nil {
		return err
	}

	// background task to collate internal redfish events and publish
	go func() {
		defer listener.Close()
		inbox := listener.Inbox()
		eventQ := []*RedfishEventData{}
		timer := time.NewTimer(10 * time.Second)
		timer.Stop()
		id := 0
		for {
			select {
			case event := <-inbox:
				log.MustLogger("event_service").Info("Got event", "event", event)
				switch data := event.Data().(type) {
				case RedfishEventData:
					// mitigate duplicate messages
					found := false
					for _, evt := range eventQ {
						if reflect.DeepEqual(*evt, data) {
							found = true
						}
					}

					if !found {
						eventQ = append(eventQ, &data)
					}

					if len(eventQ) > MaxEventsQueued {
						log.MustLogger("event_service").Warn("Full queue: sending now.", "id", id)
						// if queue has max number of events, send them now
						sendEvents(ctx, id, eventQ, eb)
						if !timer.Stop() {
							//drain timer
							<-timer.C
						}
						id = id + 1
						eventQ = []*RedfishEventData{}

					} else {
						// otherwise, start up timer to send the events in a bit
						timer.Reset(QueueTime)
					}

				case domain.RedfishResourceCreatedData:
					eventData := RedfishEventData{
						EventType:         "ResourceCreated",
						OriginOfCondition: map[string]interface{}{"@odata.id": data.ResourceURI},
					}

					eb.PublishEvent(ctx, eh.NewEvent(RedfishEvent, eventData, time.Now()))

				case domain.RedfishResourceRemovedData:
					eventData := RedfishEventData{
						EventType:         "ResourceRemoved",
						OriginOfCondition: map[string]interface{}{"@odata.id": data.ResourceURI},
					}

					eb.PublishEvent(ctx, eh.NewEvent(RedfishEvent, eventData, time.Now()))

				default:
					log.MustLogger("event_service").Warn("Should never happen: got an invalid event in the event handler")
				}

			case <-timer.C:
				log.MustLogger("event_service").Warn("times up: sending now.", "id", id)
				sendEvents(ctx, id, eventQ, eb)
				eventQ = []*RedfishEventData{}
				id = id + 1
			case <-ctx.Done():
				return
			}

		}
	}()

	return nil
}

func sendEvents(ctx context.Context, id int, events []*RedfishEventData, eb eh.EventBus) {
	data := ExternalRedfishEventData{
		Id:      id,
		Context: "/redfish/v1/$metadata#Event.Event",
		Name:    "Event Array",
		Type:    "#Event.v1_1_0.Event",
		Events:  events,
	}

	eb.PublishEvent(ctx, eh.NewEvent(ExternalRedfishEvent, data, time.Now()))
}

func selectRedfishEvent(event eh.Event) bool {
	if event.EventType() != RedfishEvent &&
		event.EventType() != domain.RedfishResourceCreated &&
		event.EventType() != domain.RedfishResourceRemoved {
		return false
	}
	return true
}
