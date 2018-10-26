package faultlist

import (
	"context"
	"fmt"

	eh "github.com/looplab/eventhorizon"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"

	"github.com/superchalupa/sailfish/src/eventwaiter"
	"github.com/superchalupa/sailfish/src/log"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

type viewer interface {
	GetUUID() eh.UUID
	GetURI() string
}

type FaultListService struct {
	ch eh.CommandHandler
	eb eh.EventBus
	ew *eventwaiter.EventWaiter
}

func New(ch eh.CommandHandler, eb eh.EventBus) *FaultListService {
	EventPublisher := eventpublisher.NewEventPublisher()
	eb.AddHandler(eh.MatchAnyEventOf(FaultEntryAdd), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter(eventwaiter.SetName("FaultList Service"), eventwaiter.NoAutoRun)
	EventPublisher.AddObserver(EventWaiter)
	go EventWaiter.Run()

	return &FaultListService{
		ch: ch,
		eb: eb,
		ew: EventWaiter,
	}
}

// StartService will create a model, view, and controller for the eventservice, then start a goroutine to publish events
func (f *FaultListService) StartService(ctx context.Context, logger log.Logger, rootView viewer) {
	faultListUri := rootView.GetURI() + "/Logs/FaultList"

	faultLogger := logger.New("module", "LCL")

	// Start up goroutine that listens for log-specific events and creates log aggregates
	f.manageLcLogs(ctx, faultLogger, faultListUri)
}

// manageLcLogs starts a background process to create new log entreis
func (f *FaultListService) manageLcLogs(ctx context.Context, logger log.Logger, logUri string) {

	// set up listener for the delete event
	// INFO: this listener will only ever get
	listener, err := f.ew.Listen(ctx,
		func(event eh.Event) bool {
			t := event.EventType()
			if t == FaultEntryAdd {
				if event.Data().(*FaultEntryAddData).MessageID == "" {
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
				uuid := eh.NewUUID()
				uri := fmt.Sprintf("%s/%s", logUri, uuid)
				logger.Info("Got internal redfish event", "event", event)
				switch typ := event.EventType(); typ {
				case FaultEntryAdd:
					faultEntry := event.Data().(*FaultEntryAddData)
					f.ch.HandleCommand(
						ctx,
						&domain.CreateRedfishResource{
							ID:          uuid,
							ResourceURI: uri,
							Type:        "#LogEntryCollection.LogEntryCollection",
							Context:     "/redfish/v1/$metadata#LogEntryCollection.LogEntryCollection",
							Privileges: map[string]interface{}{
								"GET":    []string{"ConfigureManager"},
								"POST":   []string{},
								"PUT":    []string{"ConfigureManager"},
								"PATCH":  []string{"ConfigureManager"},
								"DELETE": []string{"ConfigureManager"},
							},
							Properties: map[string]interface{}{
								"Description": faultEntry.Description,
								"Name":        faultEntry.Name,
								"EntryType":   faultEntry.EntryType,
								"Id":          faultEntry.Id,
								"MessageArgs": faultEntry.MessageArgs,
								"Message":     faultEntry.Message,
								"MessageID":   faultEntry.MessageID,
								"Category":    faultEntry.Category,
								"Severity":    faultEntry.Severity,
								"Action":      faultEntry.Action,
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
