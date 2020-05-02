package eemi

import (
	"context"

	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"
	"golang.org/x/xerrors"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/event"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
	"github.com/superchalupa/sailfish/src/ocp/am3"
)

const (
	// nobody else will ever create these, so not any compelling need to register registering
	GetMessageRegistry       eh.EventType = "GetMessageRegistry"
	AvailableMessageRegistry eh.EventType = "AvailableMessageRegistry"
)

type busComponents interface {
	GetBus() eh.EventBus
	GetWaiter() *eventwaiter.EventWaiter
}

func Startup(logger log.Logger, cfg *viper.Viper, am3Svc am3.Service, d busComponents) error {
	// Important: don't leak 'cfg' outside the scope of this function!
	MessageRegistry, err := NewMessageRegistry(cfg.GetString("eemi.msgregpath"))
	if err != nil {
		return xerrors.Errorf("couldn't construct message registry: %w", err)
	}
	err = am3Svc.AddEventHandler("getmsgreg", GetMessageRegistry, func(evt eh.Event) {
		event.Publish(context.Background(), d.GetBus(), AvailableMessageRegistry, MessageRegistry)
	})
	if err != nil {
		return xerrors.Errorf("could not add redfish am3 event handlers: %w", err)
	}

	event.Publish(context.Background(), d.GetBus(), AvailableMessageRegistry, MessageRegistry)

	return nil
}

type RegistryPromise func() *Registry

func DeferredGetMsgreg(logger log.Logger, d busComponents) RegistryPromise {
	var deferredRet *Registry
	return func() *Registry {
		if deferredRet != nil {
			return deferredRet
		}

		ctx := context.Background()

		// set up listener before publishing to avoid races
		l := eventwaiter.NewListener(ctx, logger, d.GetWaiter(), func(evt eh.Event) bool {
			return evt.EventType() == AvailableMessageRegistry
		})
		l.Name = "deferred msgreg listener"
		defer l.Close()

		// publish request to send us available registries
		event.Publish(ctx, d.GetBus(), GetMessageRegistry, "")

		err := l.ProcessOneEvent(context.Background(), func(evt eh.Event) {
			deferredRet, _ = evt.Data().(*Registry)
		}) // wait until we get response
		if err != nil {
			logger.Info("Wait ERROR", "err", err)
			return nil
		}
		return deferredRet
	}
}
