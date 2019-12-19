package main

import (
	"context"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/viper"

	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/am3"

	"github.com/superchalupa/sailfish/cmd/metric-engine/telemetry-db"
	"github.com/superchalupa/sailfish/cmd/metric-engine/udb"
)

func setup(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, d *BusComponents) {
	// 2 instances of AM3 service. This means we can run concurrent message processing loops in 2 different goroutines
	// Each goroutine has exclusive access to its database

	// 		-- cgo events
	cgoStartup(logger.New("module", "cgo"), d)

	// Processing loop 2:
	//  	-- "New" DB access
	am3Svc_n2, _ := am3.StartService(ctx, logger.New("module", "AM3_DB"), "database", d)
	telemetry.RegisterAM3(logger.New("module", "sql_am3_functions"), cfgMgr, am3Svc_n2, d)

	// Processing loop 3:
	//  	-- UDB access
	am3Svc_n3, _ := am3.StartService(ctx, logger.New("module", "AM3_UDB"), "udb database", d)
	udb.RegisterAM3(logger.New("module", "udb_am3_functions"), cfgMgr, am3Svc_n3, d)

	return
}

func shutdown() {
	cgoShutdown()
}
