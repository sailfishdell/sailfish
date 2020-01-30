// +build !skip_mockup

package main

import (
	"context"
	"sync"

	"github.com/spf13/viper"

	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/mockup"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func init() {
	implementations["mockup"] = func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, viperMu *sync.RWMutex, d *domain.DomainObjects) interface{} {
		mockup.New(ctx, logger, cfgMgr, viperMu, d.CommandHandler, d.GetBus(), d)
		return nil
	}
}
