package eventservice

import (
	"context"
	"fmt"
	"time"

	eh "github.com/looplab/eventhorizon"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"

	"github.com/superchalupa/go-redfish/src/eventwaiter"
	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/model"
	"github.com/superchalupa/go-redfish/src/ocp/view"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

const defaultMaxEventsToQueue = 50
const defaultQueueTimeMs = 400 * time.Millisecond

type waiter interface {
	Listen(context.Context, func(eh.Event) bool) (*eventwaiter.EventListener, error)
}

func PublishResourceUpdatedEventsForModel(ctx context.Context, modelName string, eb eh.EventBus) view.Option {
	return view.WatchModel(modelName, func(v *view.View, m *model.Model, updates []model.Update) {
		//eventData := &RedfishEventData{
			//EventType:         "ResourceUpdated",
			//OriginOfCondition: map[string]interface{}{"@odata.id": v.GetURI()},
		//}
		//go eb.PublishEvent(ctx, eh.NewEvent(RedfishEvent, eventData, time.Now()))
	})
}

type propertygetter interface {
	GetPropertyOk(string) (interface{}, bool)
}

// PublishRedfishEvents starts a background goroutine to collage internal
// redfish events for external consumption
func PublishRedfishEvents(ctx context.Context, m propertygetter, eb eh.EventBus) error {

	EventPublisher := eventpublisher.NewEventPublisher()
	eb.AddHandler(eh.MatchAnyEventOf(RedfishEvent, domain.RedfishResourceCreated, domain.RedfishResourceRemoved), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter(eventwaiter.SetName("Event Service Publisher"))
	EventPublisher.AddObserver(EventWaiter)

	// INFO: the publisher only sends RedfishEvent, domain.RedfishResourceCreated, domain.RedfishResourceRemoved)
	//  because of MatchAnyEventOf
	listener, err := EventWaiter.Listen(ctx, selectRedfishEvent)
	if err != nil {
		return err
	}

	listener.Name = "event service listener"

	// background task to collate internal redfish events and publish
	go func() {
		defer listener.Close()
		inbox := listener.Inbox()
		eventQ := []*RedfishEventData{}
		timer := time.NewTimer(10 * time.Second)
		timer.Stop()
		id := 0
		var maxE int = defaultMaxEventsToQueue
		for {
			select {
			case event := <-inbox:
				log.MustLogger("event_service").Info("Got event", "event", event)
				switch data := event.Data().(type) {
				case *RedfishEventData:
					// mitigate duplicate messages
					found := false
					for _, evt := range eventQ {
						if data.EventType == "ResourceUpdated" &&
							evt.EventType == data.EventType &&
							evt.OriginOfCondition["@odata.id"] == data.OriginOfCondition["@odata.id"] {
							log.MustLogger("event_service").Debug("duplicate")
							found = true
						}
					}

					if found {
						continue
					} else {
						eventQ = append(eventQ, data)
					}

					var QueueTime time.Duration = -1 * time.Millisecond

					if maxEventsToQueue, ok := m.GetPropertyOk("max_events_to_queue"); ok {
						if maxE, ok = maxEventsToQueue.(int); !ok {
							maxE = defaultMaxEventsToQueue
						}
					}

					if ms, ok := m.GetPropertyOk("max_milliseconds_to_queue"); ok {
						var msInt int
						if msInt, ok = ms.(int); !ok {
							msInt = -1
						}
						QueueTime = time.Duration(msInt) * time.Millisecond
					}

					if QueueTime < 0 {
						QueueTime = defaultQueueTimeMs
					}

					if len(eventQ) > maxE {
						log.MustLogger("event_service").Info("Full queue: sending now.", "id", id)
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

				case *domain.RedfishResourceCreatedData:
					eventData := &RedfishEventData{
						EventType:         "ResourceCreated",
						OriginOfCondition: map[string]interface{}{"@odata.id": data.ResourceURI},
					}

					go eb.PublishEvent(ctx, eh.NewEvent(RedfishEvent, eventData, time.Now()))

				case *domain.RedfishResourceRemovedData:
					eventData := &RedfishEventData{
						EventType:         "ResourceRemoved",
						OriginOfCondition: map[string]interface{}{"@odata.id": data.ResourceURI},
					}

					go eb.PublishEvent(ctx, eh.NewEvent(RedfishEvent, eventData, time.Now()))

				default:
					log.MustLogger("event_service").Warn("Should never happen: got an invalid event in the event handler", "data", data, "deets", fmt.Sprintf("%T", data))
				}

			case <-timer.C:
				log.MustLogger("event_service").Info("times up: sending now.", "id", id)
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
	data := &ExternalRedfishEventData{
		Id:      id,
		Context: "/redfish/v1/$metadata#Event.Event",
		Name:    "Event Array",
		Type:    "#Event.v1_1_0.Event",
		Events:  events,
	}

	go eb.PublishEvent(ctx, eh.NewEvent(ExternalRedfishEvent, data, time.Now()))
}

func selectRedfishEvent(event eh.Event) bool {
	if event.EventType() != RedfishEvent &&
		event.EventType() != domain.RedfishResourceCreated &&
		event.EventType() != domain.RedfishResourceRemoved {
		return false
	}
	return true
}
