// +build !skip_ec

package main

import (
	"context"
	"sync"

	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/src/dell-ec"

	log "github.com/superchalupa/sailfish/src/log"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	_ "github.com/superchalupa/sailfish/godefs"
)

func init() {
	implementations["dell_ec"] = func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, viperMu *sync.RWMutex, d *domain.DomainObjects) interface{} {
		return dell_ec.New(ctx, logger, cfgMgr, viperMu, d)
	}
}
