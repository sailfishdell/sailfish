package slotconfig

import (
	"context"
	"fmt"
	"strings"

	eh "github.com/looplab/eventhorizon"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"

	"github.com/superchalupa/go-redfish/src/eventwaiter"
	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/awesome_mapper"
	"github.com/superchalupa/go-redfish/src/ocp/model"
	"github.com/superchalupa/go-redfish/src/ocp/view"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"

	"github.com/spf13/viper"
)

type viewer interface {
	GetUUID() eh.UUID
	GetURI() string
}

type SlotConfigService struct {
	ch    eh.CommandHandler
	eb    eh.EventBus
	ew    *eventwaiter.EventWaiter
	slots map[string]interface{}
}

func New(ch eh.CommandHandler, eb eh.EventBus) *SlotConfigService {
	EventPublisher := eventpublisher.NewEventPublisher()
	eb.AddHandler(eh.MatchAnyEventOf(SlotConfigAddEvent), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter(eventwaiter.SetName("Slot Event Service"))
	EventPublisher.AddObserver(EventWaiter)
	ss := make(map[string]interface{})

	return &SlotConfigService{
		ch:    ch,
		eb:    eb,
		ew:    EventWaiter,
		slots: ss,
	}
}

// StartService will create a model, view, and controller for the eventservice, then start a goroutine to publish events
func (l *SlotConfigService) StartService(ctx context.Context, logger log.Logger, rootView viewer, cfgMgr *viper.Viper) *view.View {
	sCfgUri := rootView.GetURI() + "/SlotConfigs"
	sCfgLog := logger.New("module", "slot")

	sCfgView := view.New(
		view.WithURI(sCfgUri),
		//ah.WithAction(ctx, slotLogger, "clear.logs", "/Actions/..fixme...", MakeClearLog(eb), ch, eb),
	)

	AddAggregate(ctx, sCfgLog, sCfgView, rootView.GetUUID(), l.ch, l.eb)

	// Start up goroutine that listens for log-specific events and creates log aggregates
	l.manageSlots(ctx, sCfgLog, sCfgUri, cfgMgr)

	return sCfgView
}

// starts a background process to create new log entries
func (l *SlotConfigService) manageSlots(ctx context.Context, logger log.Logger, cfgUri string, cfgMgr *viper.Viper) {


	// set up listener for the delete event
	// INFO: this listener will only ever get
	listener, err := l.ew.Listen(ctx,
		func(event eh.Event) bool {
			t := event.EventType()
			if t == SlotConfigAddEvent {
				if event.Data().(*SlotConfigAddEventData).Id == "" {
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
				logger.Info("Got internal redfish slot config event", "event", event)
				switch typ := event.EventType(); typ {
				case SlotConfigAddEvent:
					SlotConfig := event.Data().(*SlotConfigAddEventData)
					uuid := eh.NewUUID()
					uri := fmt.Sprintf("%s/%s", cfgUri, SlotConfig.Id)
					s := strings.Split(SlotConfig.Id, ".")
					group, index := s[0], s[1]

					oldUuid, ok := l.slots[uri].(eh.UUID)
					if ok {
						// early out if the same slot config already exists (same URI)
						logger.Warn("slot config already created, early out", "uuid", oldUuid)
						break
					}

					sCfgModel := model.New()
					awesome_mapper.New(ctx, logger, cfgMgr, sCfgModel, "slotconfig", map[string]interface{}{"group": group, "index": index})
					slotView := view.New(view.WithModel("default", sCfgModel), view.WithURI(uri))

					// update the UUID for this slot
					l.slots[uri] = uuid

					l.ch.HandleCommand(
						ctx,
						&domain.CreateRedfishResource{
							ID:          uuid,
							ResourceURI: uri,
							Type:        "#DellSlotConfig.v1_0_0.DellSlotConfig",
							Context:     "/redfish/v1/$metadata#DellSlotConfig.DellSlotConfig",
							Privileges: map[string]interface{}{
								"GET":    []string{"ConfigureManager"},
								"POST":   []string{},
								"PUT":    []string{"ConfigureManager"},
								"PATCH":  []string{"ConfigureManager"},
								"DELETE": []string{"ConfigureManager"},
							},
							Properties: map[string]interface{}{
								"Id":                SlotConfig.Id,
								"Columns@meta":      slotView.Meta(view.PropGET("columns")),
								"Location@meta":     slotView.Meta(view.PropGET("location")),
								"Name@meta":         slotView.Meta(view.PropGET("name")),
								"Order@meta":        slotView.Meta(view.PropGET("order")),
								"Orientation@meta":  slotView.Meta(view.PropGET("orientation")),
								"ParentConfig@meta": slotView.Meta(view.PropGET("parentConfig")),
								"Rows@meta":         slotView.Meta(view.PropGET("rows")),
								"Type@meta":         slotView.Meta(view.PropGET("type")),
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
