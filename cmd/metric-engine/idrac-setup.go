package main

import (
	"context"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/viper"

	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/am3"
)

func setup(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, d *BusComponents) {

	dbpath := cfgMgr.GetString("main.databasepath")
	if len(dbpath) == 0 {
		// appropriate to panic here, can't reasonably continue
		panic("main.databasepath not set in config, cannot continue.")
	}

	am3Svc, _ := am3.StartService(ctx, logger.New("module", "AM3"), d)

	// Generic "Events" -> "Metric Value Event"
	// This mapper will listen for any generic event on the bus, examine it and output Metrics
	addAM3Functions(logger.New("module", "metric_am3_functions"), am3Svc, d)

	// Store Metric Value Events in the database
	addAM3DatabaseFunctions(logger.New("module", "sql_am3_functions"), cfgMgr.GetString("main.databasepath"), am3Svc, d)

	// Import metrics from the legacy telemetryservice database
	addAM3LegacyDatabaseFunctions(logger.New("module", "legacy_sql_am3_functions"), cfgMgr.GetString("main.legacydatabasepath"), am3Svc, d)

	// Import metrics from the legacy telemetryservice database
	addAM3cgo(logger.New("module", "cgo"), am3Svc, d)

	return
}
