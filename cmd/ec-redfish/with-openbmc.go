// +build !skip_openbmc

package main

import (
	"context"
	"sync"

	eh "github.com/looplab/eventhorizon"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"
	"github.com/spf13/viper"

	"github.com/superchalupa/go-redfish/src/openbmc"

	"github.com/superchalupa/go-redfish/src/eventwaiter"
	log "github.com/superchalupa/go-redfish/src/log"
)

func init() {
	implementations["openbmc"] = func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, viperMu *sync.Mutex, ch eh.CommandHandler, eb eh.EventBus) Implementation {

		EventPublisher := eventpublisher.NewEventPublisher()
		eb.AddHandler(eh.MatchAny(), EventPublisher)
		EventWaiter := eventwaiter.NewEventWaiter()
		EventPublisher.AddObserver(EventWaiter)

		return openbmc.New(ctx, logger, cfgMgr, viperMu, ch, eb, EventWaiter)
	}
}
