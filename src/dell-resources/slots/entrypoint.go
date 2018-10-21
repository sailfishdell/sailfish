package slots

import (
	"context"
	"strings"

	eh "github.com/looplab/eventhorizon"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"

	"github.com/superchalupa/sailfish/src/eventwaiter"
	"github.com/superchalupa/sailfish/src/log"

	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/src/dell-resources/component"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
)

type viewer interface {
	GetUUID() eh.UUID
	GetURI() string
}

type SlotService struct {
	ch        eh.CommandHandler
	eb        eh.EventBus
	ew        *eventwaiter.EventWaiter
	modParams func(...map[string]interface{}) map[string]interface{}
}

func New(ch eh.CommandHandler, eb eh.EventBus) *SlotService {
	EventPublisher := eventpublisher.NewEventPublisher()
	eb.AddHandler(eh.MatchAnyEventOf(component.ComponentEvent), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter(eventwaiter.SetName("Slot Event Service"))
	EventPublisher.AddObserver(EventWaiter)

	return &SlotService{
		ch: ch,
		eb: eb,
		ew: EventWaiter,
	}
}

// StartService will create a model, view, and controller for the eventservice, then start a goroutine to publish events
func (l *SlotService) StartService(ctx context.Context, logger log.Logger, baseView viewer, cfgMgr *viper.Viper, instantiateSvc *testaggregate.Service, modParams func(map[string]interface{}) map[string]interface{}, ch eh.CommandHandler, eb eh.EventBus) {

	l.modParams = func(in ...map[string]interface{}) map[string]interface{} {
		mod := map[string]interface{}{}
		mod["collection_uri"] = baseView.GetURI() + "/Slots"
		for _, i := range in {
			for k, v := range i {
				mod[k] = v
			}
		}
		return modParams(mod)
	}
	slotLogger, _, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "slotcollection", l.modParams())

	slotLogger.Info("Created slot collection", "uri", baseView.GetURI()+"/Slots")

	// Start up goroutine that listens for log-specific events and creates log aggregates
	l.manageSlots(ctx, slotLogger, cfgMgr, instantiateSvc, ch, eb)
}

// starts a background process to create new log entries
func (l *SlotService) manageSlots(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, instantiateSvc *testaggregate.Service, ch eh.CommandHandler, eb eh.EventBus) {

	// set up listener for the delete event
	// INFO: this listener will only ever get
	listener, err := l.ew.Listen(ctx,
		func(event eh.Event) bool {
			t := event.EventType()
			if t == component.ComponentEvent {
				if event.Data().(*component.ComponentEventData).Id == "" {
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
		slots := map[string]struct{}{}
		for {
			select {
			case event := <-inbox:
				logger.Info("Got internal redfish component event", "event", event)
				switch typ := event.EventType(); typ {
				case component.ComponentEvent:
					SlotEntry := event.Data().(*component.ComponentEventData)
					if SlotEntry.Type != "Slot" {
						break
					}

					s := strings.Split(SlotEntry.Id, ".")
					group, index := s[0], s[1]

					_, ok := slots[SlotEntry.Id]
					if ok {
						logger.Info("slot already created, skip", "baseSlotURI", l.modParams()["collection_uri"], "SlotEntry.Id", SlotEntry.Id)
						break
					}
					// track that this slot is instantiated
					slots[SlotEntry.Id] = struct{}{}

					slotLogger, _, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, "slot",
						l.modParams(map[string]interface{}{
							"FQDD":  SlotEntry.Id,
							"Group": group, // for ar mapper
							"Index": index, // for ar mapper
						}),
					)
					slotLogger.Info("Created Slot", "SlotEntry.Id", SlotEntry.Id)
				}

			case <-ctx.Done():
				logger.Info("context is done")
				return
			}
		}
	}()

	return
}
