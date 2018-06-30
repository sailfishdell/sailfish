package main

import (
	"context"
	"sync"
    "fmt"

	"github.com/spf13/viper"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"
	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/go-redfish/src/dell-ec"

	log "github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/eventwaiter"
)

func init() {
    fmt.Printf("Initializing dell-ec implementation\n")
    if implementations == nil {
        fmt.Printf("Implementations map is nil, initializing...\n")
        implementations = make(map[string]func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, viperMu *sync.Mutex, ch eh.CommandHandler, eb eh.EventBus) Implementation)
    }

    implementations["dell_ec"] = func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, viperMu *sync.Mutex, ch eh.CommandHandler, eb eh.EventBus) Implementation {

        EventPublisher := eventpublisher.NewEventPublisher()
        eb.AddHandler(eh.MatchAny(), EventPublisher)
        EventWaiter := eventwaiter.NewEventWaiter()
        EventPublisher.AddObserver(EventWaiter)

		return dell_ec.New(ctx, logger, cfgMgr, viperMu, ch, eb, EventWaiter)
    }
}
