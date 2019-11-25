// +build idrac

package main

import (
	"context"
	"errors"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/godefs"
	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/am3"
	"github.com/superchalupa/sailfish/src/ocp/event"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func init() {
	implementations["idrac"] = func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, d *domain.DomainObjects) Implementation {
		// set up the event dispatcher
		event.Setup(d.CommandHandler, d.EventBus)
		domain.StartInjectService(logger, d)
		godefs.InitGoDef()

		dbpath := cfgMgr.GetString("main.databasepath")
		if len(dbpath) == 0 {
			logger.Crit("main.databasepath not set in config, cannot continue.")
			return errors.New("main.databasepath not set.")
		}

		am3Svc, _ := am3.StartService(ctx, logger.New("module", "AM3"), d.EventBus, d.CommandHandler, d)
		addAM3DatabaseFunctions(logger.New("module", "sql_am3_functions"), cfgMgr.GetString("main.databasepath"), am3Svc, d)
		addAM3Functions(logger.New("module", "metric_am3_functions"), am3Svc, d)

		return nil
	}
}
