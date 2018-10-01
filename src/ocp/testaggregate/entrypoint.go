package testaggregate

import (
	"context"

	eh "github.com/looplab/eventhorizon"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"
	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/src/eventwaiter"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/view"
)

// goal: add /redfish/v1/testview#ABC entries on event

type viewer interface {
	GetUUID() eh.UUID
	GetURI() string
}

type TestService struct {
	ch eh.CommandHandler
	eb eh.EventBus
	ew *eventwaiter.EventWaiter
}

// This service will listen for test events and either publish or remove test items
// Once started, there is currently no provision to stop this service
func StartService(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, rootview *view.View, ch eh.CommandHandler, eb eh.EventBus) *TestService {
	tsLogger := logger.New("module", "test_service")

	EventPublisher := eventpublisher.NewEventPublisher()
	eb.AddHandler(eh.MatchAnyEventOf(TestEvent, TestDeletedEvent), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter(eventwaiter.SetName("Slot Event Service"))
	EventPublisher.AddObserver(EventWaiter)

	ret := &TestService{
		ch: ch,
		eb: eb,
		ew: EventWaiter,
	}

	go ret.manageTestObjs(ctx, tsLogger, cfgMgr, rootview)

	return ret
}

// starts a background process to create new log entries
func (l *TestService) manageTestObjs(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, rootview *view.View) {

	// set up listener for the delete event
	// INFO: this listener will only ever get
	listener, err := l.ew.Listen(ctx,
		func(event eh.Event) bool {
			t := event.EventType()
			if t == TestEvent {
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
				logger.Info("Got internal redfish TEST event", "event", event)
				switch typ := event.EventType(); typ {
				case TestEvent:
					d, ok := event.Data().(*TestEventData)
					if !ok {
						logger.Warn("Test Event without proper *TestEventData", "event", event)
					}

					InstantiateFromCfg(ctx, logger, cfgMgr, "testview_sub", map[string]interface{}{"unique": d.Unique, "rooturi": rootview.GetURI()})
				}

			case <-ctx.Done():
				logger.Info("context is done")
				return
			}
		}
	}()

	return
}
