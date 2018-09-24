// +build !skip_ec

package main

import (
	"context"
	"sync"

	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/src/dell-ec"

	log "github.com/superchalupa/sailfish/src/log"
)

func init() {
	implementations["dell_ec"] = func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, viperMu *sync.Mutex, ch eh.CommandHandler, eb eh.EventBus) Implementation {
		return dell_ec.New(ctx, logger, cfgMgr, viperMu, ch, eb)
	}
}
