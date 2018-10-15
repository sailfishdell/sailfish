// +build !skip_mockup

package main

import (
	"context"
	"sync"

	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"

	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/mockup"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func init() {
	implementations["mockup"] = func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, viperMu *sync.Mutex, ch eh.CommandHandler, eb eh.EventBus, d *domain.DomainObjects) Implementation {
		mockup.New(ctx, logger, cfgMgr, viperMu, ch, eb, d)
		return nil
	}
}
