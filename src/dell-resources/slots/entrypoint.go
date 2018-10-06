package slots

import (
	"context"
	"fmt"
	"strings"

	eh "github.com/looplab/eventhorizon"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"

	"github.com/superchalupa/sailfish/src/eventwaiter"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	"github.com/superchalupa/sailfish/src/ocp/model"
	//"github.com/superchalupa/sailfish/src/ocp/awesome_mapper"
	"github.com/spf13/viper"
	"github.com/superchalupa/sailfish/src/dell-resources/ar_mapper2"

	"github.com/superchalupa/sailfish/src/dell-resources/component"
)

type viewer interface {
	GetUUID() eh.UUID
	GetURI() string
}

type SlotService struct {
	ch        eh.CommandHandler
	eb        eh.EventBus
	ew        *eventwaiter.EventWaiter
	slots     map[string]interface{}
	arService *ar_mapper2.ARService
}

func New(arservice *ar_mapper2.ARService, ch eh.CommandHandler, eb eh.EventBus) *SlotService {
	EventPublisher := eventpublisher.NewEventPublisher()
	eb.AddHandler(eh.MatchAnyEventOf(component.ComponentEvent), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter(eventwaiter.SetName("Slot Event Service"))
	EventPublisher.AddObserver(EventWaiter)
	ss := make(map[string]interface{})

	return &SlotService{
		ch:        ch,
		eb:        eb,
		ew:        EventWaiter,
		slots:     ss,
		arService: arservice,
	}
}

// StartService will create a model, view, and controller for the eventservice, then start a goroutine to publish events
func (l *SlotService) StartService(ctx context.Context, logger log.Logger, rootView viewer, cfgMgr *viper.Viper, updateFns []func(context.Context, *viper.Viper), ch eh.CommandHandler, eb eh.EventBus) *view.View {

	slotUri := rootView.GetURI() + "/Slots"
	slotLogger := logger.New("module", "slot")

	slotView := view.New(
		view.WithURI(slotUri),
		//ah.WithAction(ctx, slotLogger, "clear.logs", "/Actions/..fixme...", MakeClearLog(eb), ch, eb),
	)

	AddAggregate(ctx, slotLogger, slotView, rootView.GetUUID(), l.ch, l.eb)

	// Start up goroutine that listens for log-specific events and creates log aggregates
	l.manageSlots(ctx, slotLogger, slotUri, cfgMgr, updateFns, ch, eb)

	return slotView
}

// starts a background process to create new log entries
func (l *SlotService) manageSlots(ctx context.Context, logger log.Logger, logUri string, cfgMgr *viper.Viper, updateFns []func(context.Context, *viper.Viper), ch eh.CommandHandler, eb eh.EventBus) {

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
					SlotEntry := event.Data().(*component.ComponentEventData)
					if SlotEntry.Type != "Slot" {
						break
					}

					uuid := eh.NewUUID()
					uri := fmt.Sprintf("%s/%s", logUri, SlotEntry.Id)
					s := strings.Split(SlotEntry.Id, ".")
					group, index := s[0], s[1]

					oldUuid, ok := l.slots[uri].(eh.UUID)
					if ok {
						// early out if the same slot already exists (same URI)
						logger.Info("slot already created, early out", "uuid", oldUuid)
						break
					}

					slotModel := model.New()
					//awesome_mapper.New(ctx, logger, cfgMgr, slotModel, "slots", map[string]interface{}{"group": group, "index": index})

					armapper := l.arService.NewMapping(logger, uri, "Chassis/Slots", slotModel, map[string]string{"Group": group, "Index": index, "FQDD": ""})

					slotView := view.New(
						view.WithURI(uri),
						view.WithModel("default", slotModel),
						view.WithController("ar_mapper", armapper),
					)

					// update the UUID for this slot
					l.slots[uri] = uuid

					properties := map[string]interface{}{
						"Config@meta":      slotView.Meta(view.PropGET("slot_config")),
						"Contains@meta":    slotView.Meta(view.PropGET("slot_contains")),
						"Id":               SlotEntry.Id,
						"Name@meta":        slotView.Meta(view.PropGET("slot_name")),
						"Occupied@meta":    slotView.Meta(view.PropGET("slot_occupied")),
						"SlotName@meta":    slotView.Meta(view.PropGET("slot_slotname")),
						"SledProfile@meta": slotView.Meta(view.PropGET("slot_profile")),
					}

					// TODO: add to awesome mapper probably
					/*if strings.Contains(SlotEntry.Id, "SledSlot") {
											if properties["Contains@meta"] != nil {
											    sled_key := properties["Contains@meta"].(string)
											    //properties["SledProfile"] = (sled_key)#Info.1#SledProfile <- How to do this?
											    sledmapper, _ := ar_mapper.New(ctx, logger, slotModel, "Chassis/System.Modular", map[string]string{"Group":"", "Index":"", "FQDD":sled_key}, ch, eb)
					                                                    slotView.ApplyOption(view.WithController("sled_mapper", sledmapper))
											    properties["SledProfile"] = slotView.Meta(view.PropGET("sled_profile"))
											} else {
											    properties["SledProfile"] = nil
											}
										}*/

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
							Properties: properties,
						})
				}

			case <-ctx.Done():
				logger.Info("context is done")
				return
			}
		}
	}()

	return
}
