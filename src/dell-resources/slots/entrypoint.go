package slot

import (
	"context"
	"fmt"

	eh "github.com/looplab/eventhorizon"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"

	"github.com/superchalupa/go-redfish/src/eventwaiter"
	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/view"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

type viewer interface {
	GetUUID() eh.UUID
	GetURI() string
}

type SlotService struct {
	ch    eh.CommandHandler
	eb    eh.EventBus
	ew    *eventwaiter.EventWaiter
	slots map[string]interface{}
}

func New(ch eh.CommandHandler, eb eh.EventBus) *SlotService {
	EventPublisher := eventpublisher.NewEventPublisher()
	eb.AddHandler(eh.MatchAnyEventOf(SlotEvent), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter(eventwaiter.SetName("Slot Event Service"))
	EventPublisher.AddObserver(EventWaiter)
	ss := make(map[string]interface{})

	return &SlotService{
		ch:    ch,
		eb:    eb,
		ew:    EventWaiter,
		slots: ss,
	}
}

// StartService will create a model, view, and controller for the eventservice, then start a goroutine to publish events
func (l *SlotService) StartService(ctx context.Context, logger log.Logger, rootView viewer) *view.View {
	slotUri := rootView.GetURI() + "/Slots"

	slotLogger := logger.New("module", "slot")

	slotView := view.New(
		view.WithURI(slotUri),
		//ah.WithAction(ctx, slotLogger, "clear.logs", "/Actions/..fixme...", MakeClearLog(eb), ch, eb),
	)

	AddAggregate(ctx, slotLogger, slotView, rootView.GetUUID(), l.ch, l.eb)

	// Start up goroutine that listens for log-specific events and creates log aggregates
	l.manageSlots(ctx, slotLogger, slotUri)

	return slotView
}

// starts a background process to create new log entries
func (l *SlotService) manageSlots(ctx context.Context, logger log.Logger, logUri string) {

	// set up listener for the delete event
	// INFO: this listener will only ever get
	listener, err := l.ew.Listen(ctx,
		func(event eh.Event) bool {
			t := event.EventType()
			if t == SlotEvent {
				if event.Data().(*SlotEventData).Id == "" {
					return false
				}
				return true
			}
			return false
		},
	)
	if err != nil {
		return
	}

	go func() {
		defer listener.Close()

		inbox := listener.Inbox()
		for {
			select {
			case event := <-inbox:
				logger.Info("Got internal redfish slot event", "event", event)
				switch typ := event.EventType(); typ {
				case SlotEvent:
					SlotEntry := event.Data().(*SlotEventData)
					uuid := eh.NewUUID()
					uri := fmt.Sprintf("%s/%s", logUri, SlotEntry.Id)
					oldUuid, ok := l.slots[uri].(eh.UUID)
					if ok {
						// remove any old slot info at the same URI
						l.ch.HandleCommand(ctx, &domain.RemoveRedfishResource{ID: oldUuid, ResourceURI: uri})
					}

					// update the UUID for this slot
					l.slots[uri] = uuid

					l.ch.HandleCommand(
						ctx,
						&domain.CreateRedfishResource{
							ID:          uuid,
							ResourceURI: uri,
							Type:        "#DellSlot.v1_0_0.DellSlot",
							Context:     "/redfish/v1/$metadata#DellSlot.DellSlot",
							Privileges: map[string]interface{}{
								"GET":    []string{"ConfigureManager"},
								"POST":   []string{},
								"PUT":    []string{"ConfigureManager"},
								"PATCH":  []string{"ConfigureManager"},
								"DELETE": []string{"ConfigureManager"},
							},
							Properties: map[string]interface{}{
								"Config":   SlotEntry.Config,
								"Contains": SlotEntry.Contains,
								"Id":       SlotEntry.Id,
								"Name":     SlotEntry.Name,
								"Occupied": SlotEntry.Occupied,
								"SlotName": SlotEntry.SlotName,
							}})
				}

			case <-ctx.Done():
				logger.Info("context is done")
				return
			}
		}
	}()

	return
}
