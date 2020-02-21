// +build !skip_msm

package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/src/dell-msm"

	log "github.com/superchalupa/sailfish/src/log"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	_ "github.com/superchalupa/sailfish/godefs"
)

func init() {
	implementations["dell_msm"] = func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, viperMu *sync.RWMutex, d *domain.DomainObjects) interface{} {
		fmt.Println("TEST")
		return dell_msm.New(ctx, logger, cfgMgr, viperMu, d)
	}
}
