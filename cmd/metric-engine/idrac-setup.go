package main

import (
	"context"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/viper"

	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/am3"

	"github.com/superchalupa/sailfish/cmd/metric-engine/metric"
	"github.com/superchalupa/sailfish/cmd/metric-engine/persistence"
	"github.com/superchalupa/sailfish/cmd/metric-engine/telemetry"
	"github.com/superchalupa/sailfish/cmd/metric-engine/triggers"
	"github.com/superchalupa/sailfish/cmd/metric-engine/udb"
	"github.com/superchalupa/sailfish/cmd/metric-engine/watchdog"
)

// nolint: gochecknoinits
// have to have init() function to runtime register the compile-time optional components, better suggestions welcome
func init() {
	initOptional()
	optionalComponents = append(optionalComponents, func(logger log.Logger, cfg *viper.Viper, d busIntf) func() {
		return setup(context.Background(), logger, cfg, d)
	})
}

func setDefaults(cfgMgr *viper.Viper) {
	cfgMgr.SetDefault("main.databasepath",
		"file:/run/telemetryservice/telemetry_timeseries_database.db?_foreign_keys=on&cache=shared&mode=rwc&_busy_timeout=1000&_journal_mode=WAL")
	cfgMgr.SetDefault("main.startup", "startup-events")
	cfgMgr.SetDefault("main.mddirectory", "/usr/share/telemetryservice/md/")
}

// setup will startup am3 services and database connections
//
// We are going to initialize 2 instances of AM3 service.  This means we can
// run concurrent message processing loops in 2 different goroutines Each
// goroutine has exclusive access to its database, so we'll be able to
// simultaneously do ops on each DB
func setup(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, d am3.BusObjs) func() {
	// register global metric events with event horizon
	metric.RegisterEvent()
	telemetry.RegisterEvents()

	// setup viper defaults
	setDefaults(cfgMgr)

	// Processing loop 1: telemetry database
	am3SvcN2, _ := am3.StartService(ctx, log.With(logger, "module", "AM3_DB"), "database", d)
	shutdownbase, err := telemetry.Startup(log.With(logger, "module", "telemetry"), cfgMgr, am3SvcN2, d)
	if err != nil {
		panic("Error initializing base telemetry subsystem: " + err.Error())
	}

	// Import all MD/MRD/Trigger
	err = persistence.Import(logger, cfgMgr, d.GetBus())
	if err != nil {
		panic("Error loading Metric Definitions: " + err.Error())
	}

	// Processing loop 2: UDB database
	am3SvcN3, _ := am3.StartService(ctx, log.With(logger, "module", "AM3_UDB"), "udb database", d)
	shutdownudb, err := udb.Startup(log.With(logger, "module", "UDB"), cfgMgr, am3SvcN3, d)
	if err != nil {
		panic("Error initializing UDB Import subsystem: " + err.Error())
	}

	// Trigger processing
	err = triggers.StartupTriggerProcessing(log.With(logger, "module", "trigger_am3_functions"), cfgMgr, am3SvcN2, d)
	if err != nil {
		panic("Error initializing trigger processing subsystem: " + err.Error())
	}

	// Watchdog
	err = watchdog.StartWatchdogHandling(logger, am3SvcN2, d)
	if err != nil {
		panic("Error initializing watchdog handling: " + err.Error())
	}

	return func() {
		shutdown(logger, am3SvcN3, am3SvcN2)
		shutdownudb()
		shutdownbase()
	}
}

type Shutdowner interface {
	Shutdown() error
}

func shutdown(logger log.Logger, shutdownlist ...Shutdowner) {
	for _, s := range shutdownlist {
		err := s.Shutdown()
		if err != nil {
			logger.Crit("Error shutting down AM3", "err", err)
		}
	}
}
