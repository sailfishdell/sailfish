package eventservice

import (
	"context"
	"fmt"

	eh "github.com/looplab/eventhorizon"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"

	ah "github.com/superchalupa/go-redfish/src/actionhandler"
	"github.com/superchalupa/go-redfish/src/eventwaiter"
	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/model"
	"github.com/superchalupa/go-redfish/src/ocp/view"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

type viewer interface {
	GetUUID() eh.UUID
	GetURI() string
}

var StartEventService func(context.Context, log.Logger, viewer) *view.View
var CreateSubscription func(context.Context, log.Logger, Subscription, func()) *view.View

func InitService(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus) {
	EventPublisher := eventpublisher.NewEventPublisher()
	eb.AddHandler(eh.MatchAny(), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter()
	EventPublisher.AddObserver(EventWaiter)

	StartEventService = func(ctx context.Context, logger log.Logger, rootView viewer) *view.View {
		return startEventService(ctx, logger, rootView, ch, eb)
	}
	CreateSubscription = func(ctx context.Context, logger log.Logger, sub Subscription, cancel func()) *view.View {
		return createSubscription(ctx, logger, sub, cancel, ch, EventWaiter)
	}
}

// StartEventService will create a model, view, and controller for the eventservice, then start a goroutine to publish events
//      If you want to save settings, hook up a mapper to the "default" view returned
func startEventService(ctx context.Context, logger log.Logger, rootView viewer, ch eh.CommandHandler, eb eh.EventBus) *view.View {
	esLogger := logger.New("module", "EventService")

	esModel := model.New(
		model.UpdateProperty("delivery_retry_attempts", 3),
		model.UpdateProperty("delivery_retry_interval_seconds", 60),
	)

	esView := view.New(
		view.WithModel("default", esModel),
		view.WithURI(rootView.GetURI()+"/EventService"),
		ah.WithAction(ctx, esLogger, "submit.test.event", "/Actions/EventService.SubmitTestEvent", MakeSubmitTestEvent(eb), ch, eb),
	)

	// The Plugin: "EventService" property on the Subscriptions endpoint is how we know to run this command
	eh.RegisterCommand(func() eh.Command { return &POST{model: esView.GetModel("default"), ch: ch, eb: eb} })
	AddAggregate(ctx, esLogger, esView, rootView.GetUUID(), ch, eb)
	PublishRedfishEvents(ctx, eb)

	return esView
}

// CreateSubscription will create a model, view, and controller for the subscription
//      If you want to save settings, hook up a mapper to the "default" view returned
func createSubscription(ctx context.Context, logger log.Logger, sub Subscription, cancel func(), ch eh.CommandHandler, EventWaiter waiter) *view.View {
	uuid := eh.NewUUID()
	uri := fmt.Sprintf("/redfish/v1/EventService/Subscriptions/%s", uuid)

	//esLogger := logger.New("module", "EventSubscription")
	esModel := model.New(
		model.UpdateProperty("destination", sub.Destination),
		model.UpdateProperty("protocol", sub.Protocol),
		model.UpdateProperty("context", sub.Context),
		model.UpdateProperty("event_types", sub.EventTypes),
	)
	subView := view.New(
		view.WithModel("default", esModel),
		view.WithURI(uri),
	)

	retprops := map[string]interface{}{
		"@odata.id":        uri,
		"@odata.type":      "#EventDestination.v1_2_0.EventDestination",
		"@odata.context":   "/redfish/v1/$metadata#EventDestination.EventDestination",
		"Id":               fmt.Sprintf("%s", uuid),
		"Protocol@meta":    subView.Meta(view.GETProperty("protocol"), view.GETModel("default")),
		"Name@meta":        subView.Meta(view.GETProperty("name"), view.GETModel("default")),
		"Destination@meta": subView.Meta(view.GETProperty("destination"), view.GETModel("default")),
		"EventTypes@meta":  subView.Meta(view.GETProperty("event_types"), view.GETModel("default")),
		"Context@meta":     subView.Meta(view.GETProperty("context"), view.GETModel("default")),
	}

	// set up listener for the delete event
	listener, err := EventWaiter.Listen(ctx,
		func(event eh.Event) bool {
			if event.EventType() != domain.RedfishResourceRemoved {
				return false
			}
			if data, ok := event.Data().(domain.RedfishResourceRemovedData); ok {
				if data.ResourceURI == uri {
					return true
				}
			}
			return false
		},
	)
	if err != nil {
		return nil
	}

	go func() {
		// close the view when we exit this goroutine
		defer subView.Close()
		// delete the aggregate
		defer ch.HandleCommand(context.Background(), &domain.RemoveRedfishResource{ID: uuid, ResourceURI: uri})
		defer listener.Close()

		inbox := listener.Inbox()
		for {
			select {
			case event := <-inbox:
				log.MustLogger("event_service").Info("Got internal redfish event", "event", event)
				cancel()
				return

			case <-ctx.Done():
				return
			}
		}
	}()

	// TODO: error handling
	ch.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          uuid,
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

	return subView
}
