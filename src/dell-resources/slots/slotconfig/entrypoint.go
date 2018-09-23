package slotconfig

import (
	"context"
	"fmt"
	"strings"

	eh "github.com/looplab/eventhorizon"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"

	"github.com/superchalupa/sailfish/src/eventwaiter"
	"github.com/superchalupa/sailfish/src/log"
	//"github.com/superchalupa/sailfish/src/ocp/awesome_mapper"
	"github.com/superchalupa/sailfish/src/dell-resources/ar_mapper"
	"github.com/superchalupa/sailfish/src/ocp/model"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	"github.com/spf13/viper"
	"github.com/superchalupa/sailfish/src/dell-resources/component"
)

type viewer interface {
	GetUUID() eh.UUID
	GetURI() string
}

type SlotConfigService struct {
	ch          eh.CommandHandler
	eb          eh.EventBus
	ew          *eventwaiter.EventWaiter
	slotconfigs map[string]interface{}
}

func New(ch eh.CommandHandler, eb eh.EventBus) *SlotConfigService {
	EventPublisher := eventpublisher.NewEventPublisher()
	eb.AddHandler(eh.MatchAnyEventOf(component.ComponentEvent), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter(eventwaiter.SetName("Slot Config Event Service"))
	EventPublisher.AddObserver(EventWaiter)
	ss := make(map[string]interface{})

	return &SlotConfigService{
		ch:          ch,
		eb:          eb,
		ew:          EventWaiter,
		slotconfigs: ss,
	}
}

// StartService will create a model, view, and controller for the eventservice, then start a goroutine to publish events
func (l *SlotConfigService) StartService(ctx context.Context, logger log.Logger, rootView viewer, cfgMgr *viper.Viper, updateFns []func(context.Context, *viper.Viper), ch eh.CommandHandler, eb eh.EventBus) *view.View {
	sCfgUri := rootView.GetURI() + "/SlotConfigs"
	sCfgLog := logger.New("module", "slot")

	sCfgView := view.New(
		view.WithURI(sCfgUri),
		//ah.WithAction(ctx, slotLogger, "clear.logs", "/Actions/..fixme...", MakeClearLog(eb), ch, eb),
	)

	AddAggregate(ctx, sCfgLog, sCfgView, rootView.GetUUID(), l.ch, l.eb)

	// Start up goroutine that listens for log-specific events and creates log aggregates
	l.manageSlots(ctx, sCfgLog, sCfgUri, cfgMgr, updateFns, ch, eb)

	return sCfgView
}

// starts a background process to create new log entries
func (l *SlotConfigService) manageSlots(ctx context.Context, logger log.Logger, cfgUri string, cfgMgr *viper.Viper, updateFns []func(context.Context, *viper.Viper), ch eh.CommandHandler, eb eh.EventBus) {

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
		for {
			select {
			case event := <-inbox:
				logger.Info("Got internal redfish component event", "event", event)
				switch typ := event.EventType(); typ {
				case component.ComponentEvent:
					SlotConfig := event.Data().(*component.ComponentEventData)
					if SlotConfig.Type != "SlotConfig" {
						// Type is not SlotConfig, so not a SlotConfig event
						break
					}

					uuid := eh.NewUUID()
					uri := fmt.Sprintf("%s/%s", cfgUri, SlotConfig.Id)
					s := strings.Split(SlotConfig.Id, ".")
					group, index := s[0], s[1]

					oldUuid, ok := l.slotconfigs[uri].(eh.UUID)
					if ok {
						// early out if the same slot config already exists (same URI)
						logger.Warn("slot config already created, early out", "uuid", oldUuid)
						break
					}

					sCfgModel := model.New()
					//awesome_mapper.New(ctx, logger, cfgMgr, sCfgModel, "slotconfig", map[string]interface{}{"group": group, "index": index})

					armapper, _ := ar_mapper.New(ctx, logger, sCfgModel, "Chassis/SlotConfigs", map[string]string{"Group": group, "Index": index, "FQDD": ""}, ch, eb)
					updateFns = append(updateFns, armapper.ConfigChangedFn)
					armapper.ConfigChangedFn(context.Background(), cfgMgr)

					slotView := view.New(
						view.WithModel("default", sCfgModel),
						view.WithURI(uri),
						view.WithController("ar_mapper", armapper),
					)

					// update the UUID for this slot
					l.slotconfigs[uri] = uuid

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
