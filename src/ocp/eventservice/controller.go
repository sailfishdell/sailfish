package eventservice

import (
	"context"
	"fmt"
	"time"

	eh "github.com/looplab/eventhorizon"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

const defaultMaxEventsToQueue = 50
const defaultQueueTimeMs = 400 * time.Millisecond

type propertygetter interface {
	GetPropertyOk(string) (interface{}, bool)
}

type syncEvent interface {
	Add(int)
	Done()
}

// PublishRedfishEvents starts a background goroutine to collage internal
// redfish events for external consumption
func PublishRedfishEvents(ctx context.Context, logger log.Logger, m propertygetter, eb eh.EventBus) error {

	EventPublisher := eventpublisher.NewEventPublisher()
	eb.AddHandler(eh.MatchAnyEventOf(RedfishEvent, domain.RedfishResourceCreated, domain.RedfishResourceRemoved), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter(eventwaiter.SetName("Event Service Publisher"), eventwaiter.NoAutoRun)
	EventPublisher.AddObserver(EventWaiter)
	go EventWaiter.Run()

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
		eventQ := []*RedfishEventData{}
		timer := time.NewTimer(10 * time.Second)
		timer.Stop()
		id := 0
		var maxE int = defaultMaxEventsToQueue
		for {
			select {
			case event := <-listener.Inbox():
				// have to mark the event complete if we don't use Wait and take it off the bus ourselves
				if e, ok := event.(syncEvent); ok {
					e.Done()
				}
				logger.Info("Got event", "event", event)
				// is RedfishResourceUpdated handled under RedfishEventData?
				switch data := event.Data().(type) {
				case *MetricReportData:
					continue
				case *RedfishEventData:
					// mitigate duplicate messages
					found := false
					//TODO MSM BUG: OriginOfCondition for events has to be a string or will be rejected
					for _, evt := range eventQ {
						if data.EventType == "ResourceUpdated" &&
							evt.EventType == data.EventType &&
							evt.OriginOfCondition == data.OriginOfCondition {
							logger.Debug("duplicate")
							found = true
							break
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
						logger.Info("Full queue: sending now.", "id", id)
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
						EventType: "ResourceAdded",
						//TODO MSM BUG: OriginOfCondition for events has to be a string or will be rejected
						OriginOfCondition: data.ResourceURI,
						MessageId:         "TST100",
					}

					eb.PublishEvent(ctx, eh.NewEvent(RedfishEvent, eventData, time.Now()))

				case *domain.RedfishResourceRemovedData:
					eventData := &RedfishEventData{
						EventType: "ResourceRemoved",
						//TODO MSM BUG: OriginOfCondition for events has to be a string or will be rejected
						OriginOfCondition: data.ResourceURI,
						MessageId:         "TST100",
					}

					eb.PublishEvent(ctx, eh.NewEvent(RedfishEvent, eventData, time.Now()))

				default:
					logger.Warn("Should never happen: got an invalid event in the event handler", "data", data, "deets", fmt.Sprintf("%T", data))
				}

			case <-timer.C:
				logger.Info("times up: sending now.", "id", id)
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
