package lcl

import (
	"context"

	eh "github.com/looplab/eventhorizon"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"

	//ah "github.com/superchalupa/go-redfish/src/actionhandler"
	"github.com/superchalupa/go-redfish/src/eventwaiter"
	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/view"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
    mgrCMCIntegrated "github.com/superchalupa/go-redfish/src/dell-resources/managers/cmc.integrated"
)

type viewer interface {
	GetUUID() eh.UUID
	GetURI() string
}

type LCLService struct {
	ch eh.CommandHandler
	eb eh.EventBus
	ew *eventwaiter.EventWaiter
}

func New(ch eh.CommandHandler, eb eh.EventBus) *LCLService {
	EventPublisher := eventpublisher.NewEventPublisher()
	eb.AddHandler(eh.MatchAnyEventOf(LogEvent), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter(eventwaiter.SetName("Event Service"))
	EventPublisher.AddObserver(EventWaiter)

	return &LCLService{
		ch: ch,
		eb: eb,
		ew: EventWaiter,
	}
}

// StartService will create a model, view, and controller for the eventservice, then start a goroutine to publish events
func (l *LCLService) StartService(ctx context.Context, logger log.Logger, rootView viewer) *view.View {
	// COLLECTION AGGREGATE to hold Lclog and Faultlist: /redfish/v1/Managers/CMC.Integrated.1/LogServices
	// AGGREGATE: /redfish/v1/Managers/CMC.Integrated.1/LogServices/Lclog
	// COLLECTION AGGREGATE: /redfish/v1/Managers/CMC.Integrated.1/Logs/Lclog.json  <-- lets save this for last
	//		^--- need a new feature: autoexpand - put this into the aggregate. Test the feature in redfish_handler and auto expand
	// COLLECTION MEMBER AGGREGATE: /redfish/v1/Managers/CMC.Integrated.1/Logs/Lclog/66.json
	//
	// SKIP FOR NOW (implement after LCL done) __redfish__v1__Managers__CMC.Integrated.1__Logs__FaultList.json

	lclLogger := logger.New("module", "LCL")

    lclView := view.New(
		view.WithURI(rootView.GetURI() + "/LogServices"),
		//ah.WithAction(ctx, lclLogger, "clear.logs", "/Actions/..fixme...", MakeClearLog(eb), ch, eb),
	)

    mgrCMCIntegrated.AddAggregate(ctx, lclLogger, lclView, l.ch)

	// AddAggregate( LogServices ...)
	// AddAggregate( LogServices/Lclog ...)
	// AddAggregate( Logs ...)

	// Start up goroutine that listens for log-specific events and creates log aggregates
	l.manageLcLogs(ctx, lclLogger)

	return lclView
}

// manageLcLogs starts a background process to create new log entreis
func (l *LCLService) manageLcLogs(ctx context.Context, logger log.Logger) {

	// individual log entry to add
	retprops := map[string]interface{}{
		"@odata.id":      "/LogServices",
		"@odata.type":    "#EventDestination.v1_2_0.EventDestination",
		"@odata.context": "/redfish/v1/$metadata#EventDestination.EventDestination",
		"Context@meta":   nil,
		"Id":             nil,
        "Description":    nil,
	}

	// set up listener for the delete event
	// INFO: this listener will only ever get
	listener, err := l.ew.Listen(ctx,
		func(event eh.Event) bool {
			t := event.EventType()
			if t == LogEvent {
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
				logger.Debug("Got internal redfish event", "event", event)
				switch typ := event.EventType(); typ {
				case LogEvent:
					l.ch.HandleCommand(
						ctx,
						&domain.CreateRedfishResource{
							ID:          "", //event.Data().(*LogEventData).Id, // fixme uuid
							ResourceURI: retprops["@odata.id"].(string),
							Type:        retprops["@odata.type"].(string),
							Context:     retprops["@odata.context"].(string),
							Privileges: map[string]interface{}{
								"GET":    []string{"ConfigureManager"},
								"POST":   []string{},
								"PUT":    []string{"ConfigureManager"},
								"PATCH":  []string{"ConfigureManager"},
								"DELETE": []string{"ConfigureManager"},
							},
							Properties: retprops,
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
