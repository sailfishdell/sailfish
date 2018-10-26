package subsystemhealth

import (
	"context"
	"strings"
	"sync"

	eh "github.com/looplab/eventhorizon"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"

	"github.com/superchalupa/sailfish/src/eventwaiter"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/view"

	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/src/dell-resources/dm_event"
	"github.com/superchalupa/sailfish/src/ocp/testaggregate"
)

type viewer interface {
	GetUUID() eh.UUID
	GetURI() string
}

type SubSystemHealthService struct {
	ch         eh.CommandHandler
	eb         eh.EventBus
	ew         *eventwaiter.EventWaiter
	subsystems map[string]interface{}
}

func New(ch eh.CommandHandler, eb eh.EventBus) *SubSystemHealthService {
	EventPublisher := eventpublisher.NewEventPublisher()
	eb.AddHandler(eh.MatchAnyEventOf(dm_event.HealthEvent), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter(eventwaiter.SetName("SubSystemHealth Service"))
	EventPublisher.AddObserver(EventWaiter)
	ss := make(map[string]interface{})

	return &SubSystemHealthService{
		ch:         ch,
		eb:         eb,
		ew:         EventWaiter,
		subsystems: ss,
	}
}

func (l *SubSystemHealthService) StartService(ctx context.Context, logger log.Logger, rootView viewer, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, instantiateSvc *testaggregate.Service) *view.View {
	subSysHealthLogger, subSysHealthView, _ := instantiateSvc.InstantiateFromCfg(ctx, cfgMgr, cfgMgrMu, "subsyshealth",
		map[string]interface{}{
			"rooturi": rootView.GetURI(),
		},
	)

	AddAggregate(ctx, subSysHealthLogger, subSysHealthView, l.ch)

	l.manageSubSystems(ctx, subSysHealthLogger, subSysHealthView)

	return subSysHealthView
}

func (l *SubSystemHealthService) manageSubSystems(ctx context.Context, logger log.Logger, vw *view.View) {
	subsystem_healths := map[string]interface{}{}

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

					s := strings.Split(HealthEntry.FQDD, "#")
					subsys := s[len(s)-1]
					health := HealthEntry.Health

					health_entry := map[string]interface{}{"Status": map[string]string{"HealthRollup": health}}

					if health == "Absent" {

						//if receive subsystem health is absent, delete subsystem entry if present
						if _, ok := subsystem_healths[subsys]; ok { //property exists, delete
							delete(subsystem_healths, subsys)
							UpdateAggregate(ctx, vw, l.ch, subsystem_healths)
						}
					} else {
						//if health is not absent, create or update subsystem entry
						subsystem_healths[subsys] = health_entry
						UpdateAggregate(ctx, vw, l.ch, subsystem_healths)
					}
					//fmt.Println(subsystem_healths)
				}

			case <-ctx.Done():
				logger.Info("context is done")
				return
			}
		}
	}()

	return
}
