package subsystemhealth

import (
	"context"
	"fmt"

	eh "github.com/looplab/eventhorizon"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"

	"github.com/superchalupa/sailfish/src/eventwaiter"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/view"
	//domain "github.com/superchalupa/sailfish/src/redfishresource"

	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/src/dell-resources/dm_event"
  	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
)

type viewer interface {
	GetUUID() eh.UUID
	GetURI() string
}

type SubSystemHealthService struct {
	ch    		eh.CommandHandler
	eb    		eh.EventBus
	ew    		*eventwaiter.EventWaiter
	subsystems 	map[string]interface{}
}

func New(ch eh.CommandHandler, eb eh.EventBus) *SubSystemHealthService {
	EventPublisher := eventpublisher.NewEventPublisher()
	eb.AddHandler(eh.MatchAnyEventOf(dm_event.HealthEvent), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter(eventwaiter.SetName("SubSystemHealth Service"))
	EventPublisher.AddObserver(EventWaiter)
	ss := make(map[string]interface{})

	return &SubSystemHealthService{
		ch:    ch,
		eb:    eb,
		ew:    EventWaiter,
		subsystems: ss,
	}
}

// StartService will create a model, view, and controller for the eventservice, then start a goroutine to publish events
func (l *SubSystemHealthService) StartService(ctx context.Context, logger log.Logger, rootView viewer, cfgMgr *viper.Viper, instantiateSvc *testaggregate.Service,  ch eh.CommandHandler, eb eh.EventBus) *view.View {

	subSysHealthUri := rootView.GetURI() + "/SubSystemHealth"
	subSysHealthLogger := logger.New("module", "subsyshealth")

	subSysHealthView := view.New(
		view.WithURI(subSysHealthUri),
	)

	AddAggregate(ctx, subSysHealthLogger, subSysHealthView, l.ch, l.eb)

	l.manageSubSystems(ctx, subSysHealthLogger, subSysHealthView, cfgMgr, ch, eb)

	return subSysHealthView
}

func (l *SubSystemHealthService) manageSubSystems(ctx context.Context, logger log.Logger, vw *view.View, cfgMgr *viper.Viper, ch eh.CommandHandler, eb eh.EventBus) {
	listener, err := l.ew.Listen(ctx,
		func(event eh.Event) bool {
			t := event.EventType()
			if t == dm_event.HealthEvent {
				if event.Data().(*dm_event.HealthEventData).FQDD == "" {
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
				logger.Info("Got internal redfish event", "event", event)
				switch typ := event.EventType(); typ {
				case dm_event.HealthEvent:
					HealthEntry := event.Data().(*dm_event.HealthEventData)
					
					subsys := HealthEntry.FQDD
					health := HealthEntry.Health
					fmt.Println("subsys: ", subsys)
					fmt.Println("health: ", health)
					//view.GetModel()
			}

			case <-ctx.Done():
				logger.Info("context is done")
				return
			}
		}
	}()

	return
}
