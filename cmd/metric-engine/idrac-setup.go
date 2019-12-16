package main

import (
	"context"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/viper"

	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/am3"

	"github.com/superchalupa/sailfish/cmd/metric-engine/events-to-metrics"
	"github.com/superchalupa/sailfish/cmd/metric-engine/telemetry-db"
	"github.com/superchalupa/sailfish/cmd/metric-engine/telemetry-legacy"
)

func setup(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, d *BusComponents) {

	dbpath := cfgMgr.GetString("main.databasepath")
	if len(dbpath) == 0 {
		// appropriate to panic here, can't reasonably continue
		panic("main.databasepath not set in config, cannot continue.")
	}

	// 3 instances of AM3 service. This means we can run 3 concurrent message processing loops in 3 different goroutines

	// Processing loop 1:
	// 		-- cgo events
	// 		-- generic event conversions into MetricValue. NO DATABASE ACCESS
	am3Svc_n1, _ := am3.StartService(ctx, logger.New("module", "AM3"), "conversion mapper", d)
	addAM3cgo(logger.New("module", "cgo"), am3Svc_n1, d)
	event_conversions.RegisterAM3(logger.New("module", "conversions"), am3Svc_n1, d)

	// Processing loop 2:
	//  	-- "New" DB access
	am3Svc_n2, _ := am3.StartService(ctx, logger.New("module", "AM3"), "database", d)
	telemetry.RegisterAM3(logger.New("module", "sql_am3_functions"), cfgMgr.GetString("main.databasepath"), am3Svc_n2, d)

	// Processing loop 3:
	//  	-- Legacy Telemetry DB access
	am3Svc_n3, _ := am3.StartService(ctx, logger.New("module", "AM3"), "legacy database", d)
	legacy_telemetry.RegisterAM3(logger.New("module", "legacy_sql_am3_functions"), cfgMgr.GetString("main.legacydatabasepath"), am3Svc_n3, d)

	// Processing loop 4:
	//  	-- FUTURE: UDB access
	//am3Svc_n4, _ := am3.StartService(ctx, logger.New("module", "AM3"), "udb database", d)

	return
}
