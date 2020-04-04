package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	eh "github.com/looplab/eventhorizon"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/viper"
	"golang.org/x/xerrors"

	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/event"
	"github.com/superchalupa/sailfish/src/ocp/am3"

	"github.com/superchalupa/sailfish/cmd/metric-engine/metric"
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
		"file:/run/telemetryservice/telemetry_timeseries_database.db?_foreign_keys=on&cache=shared&mode=rwc&_busy_timeout=1000")
	cfgMgr.SetDefault("main.startup", "startup-events")
	cfgMgr.SetDefault("main.mddirectory", "/usr/share/factory/telemetryservice/md/")
	cfgMgr.SetDefault("main.mrddirectory", "/usr/share/factory/telemetryservice/mrd/")
}

func setup(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, d busIntf) func() {
	// register global metric events with event horizon
	metric.RegisterEvent()
	telemetry.RegisterEvents()

	// setup viper defaults
	setDefaults(cfgMgr)

	// We are going to initialize 2 instances of AM3 service.  This means we can
	// run concurrent message processing loops in 2 different goroutines Each
	// goroutine has exclusive access to its database, so we'll be able to
	// simultaneously do ops on each DB

	// Processing loop 1:
	//  	-- "New" DB access
	am3SvcN2, _ := am3.StartService(ctx, log.With(logger, "module", "AM3_DB"), "database", d)
	shutdownbase, err := telemetry.Startup(log.With(logger, "module", "sql_am3_functions"), cfgMgr, am3SvcN2, d)
	if err != nil {
		panic("Error initializing base telemetry subsystem: " + err.Error())
	}

	// After base event loop, process startup events. They all belong to the base loop.
	// config file main.startup specifies the section(s) that have the list of events
	startup := cfgMgr.GetStringSlice("main.startup")
	for _, section := range startup {
		err = injectStartupEvents(logger, cfgMgr, section, d.GetBus())
		if err != nil {
			panic("Error processing startup events from YAML, can't continue: " + err.Error())
		}
	}

	// Import all MDs at start
	err = importPersistentSavedRedfishData(logger, cfgMgr, d.GetBus())
	if err != nil {
		panic("Error loading Metric Definitions: " + err.Error())
	}

	// Processing loop 2:
	//  	-- UDB access
	am3SvcN3, _ := am3.StartService(ctx, log.With(logger, "module", "AM3_UDB"), "udb database", d)
	shutdownudb, err := udb.Startup(log.With(logger, "module", "udb_am3_functions"), cfgMgr, am3SvcN3, d)
	if err != nil {
		panic("Error initializing UDB Import subsystem: " + err.Error())
	}

	//-- Trigger processing
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
		fmt.Printf("\n\nSHUTTING DOWN\n\n")
		am3SvcN3.Shutdown()
		am3SvcN2.Shutdown()
		shutdownudb()
		shutdownbase()
	}
}

// Populate all MDs at start from filesystem
func importPersistentSavedRedfishData(_ log.Logger, cfg *viper.Viper, bus eh.EventBus) error {
	var persistentImportDirs = []struct {
		name      string
		subdir    string
		eventType eh.EventType
	}{
		{"MetricDefinition", cfg.GetString("main.mddirectory"), telemetry.AddMetricDefinition},
		//{"MetricReportDefinition", cfg.GetString("main.mrddirectory"), telemetry.AddMetricReportDefinition},
	}

	// strategy: this process *has* to succeed. If it does not, return error and we panic.
	// This ensures that we notice and take care of any issues with the saved files
	// TODO: need to think of a process to clear out errors and automatically. Theoretically should not happen, but would be fatal if it did.
	for _, importDir := range persistentImportDirs {
		files, err := ioutil.ReadDir(importDir.subdir)
		if err != nil {
			return xerrors.Errorf("Error reading import dir: %s to import %s: %w", importDir.subdir, importDir.name, err)
		}

		for _, file := range files {
			filename := importDir.subdir + file.Name()
			jsonstr, err := ioutil.ReadFile(filename)
			if err != nil {
				return xerrors.Errorf("Error reading %s import file(%s): %w", importDir.name, filename, err)
			}

			eventData, err := eh.CreateEventData(importDir.eventType)
			if err != nil {
				return xerrors.Errorf("Couldnt create %s event for file(%s) import. Should never happen: %w", importDir.name, filename, err)
			}
			err = json.Unmarshal(jsonstr, eventData)
			if err != nil {
				return xerrors.Errorf("Malformed %s JSON file(%s), error unmarshalling JSON: %w", importDir.name, filename, err)
			}

			evt := event.NewSyncEvent(importDir.eventType, eventData, time.Now())
			evt.Add(1)
			err = bus.PublishEvent(context.Background(), evt)
			if err != nil {
				return xerrors.Errorf("Error publishing %s event for file(%s) import: %w", importDir.name, filename, err)
			}
			evt.Wait()
		}
	}

	return nil
}

func injectStartupEvents(logger log.Logger, cfgMgr *viper.Viper, section string, bus eh.EventBus) error {
	startup := cfgMgr.Get(section)
	if startup == nil {
		logger.Info("SKIPPING STARTUP Event Injection: no startup events found")
		return nil
	}

	// strategy: this process *has* to succeed. If it does not, return error and we panic.
	// This ensures that we notice and take care of any issues with the YAML file
	events, ok := startup.([]interface{})
	if !ok {
		return xerrors.Errorf("Startup event section malformed. Section(%s): Need (%T), got (%T).", section, []interface{}{}, startup)
	}

	for i, v := range events {
		settings, ok := v.(map[interface{}]interface{})
		if !ok {
			return xerrors.Errorf("Startup Event: malformed event in Section(%s), index(%i): Need (%T), got (%T) - %v",
				section, i, map[interface{}]interface{}{}, v, v)
		}

		name, ok := settings["name"].(string)
		if !ok {
			return xerrors.Errorf("Startup Event: event in Section(%s), index(%i) missing 'name' key: %v", section, i, v)
		}
		eventType := eh.EventType(name + "Event")

		dataString, ok := settings["data"].(string)
		if !ok {
			return xerrors.Errorf("Startup Event: event in Section(%s), index(%i) missing 'data' key: %v", section, i, v)
		}

		eventData, err := eh.CreateEventData(eventType)
		if err != nil {
			return xerrors.Errorf("Startup Event: event in Section(%s), index(%i). Could not instantiate event(%+v): %w", section, i, v, err)
		}

		err = json.Unmarshal([]byte(dataString), &eventData)
		if err != nil {
			return xerrors.Errorf("Startup Event: event in Section(%s), index(%i). Failed to unmarshal JSON for (%+v): %w", section, i, v, err)
		}

		logger.Info("Publishing Startup Event", "section", section, "index", i, "eventType", eventType)
		evt := event.NewSyncEvent(eventType, eventData, time.Now())
		evt.Add(1)
		err = bus.PublishEvent(context.Background(), evt)
		if err != nil {
			return xerrors.Errorf("Startup Event: event in Section(%s), index(%i). Failed to publish event for (%+v): %w", section, i, v, err)
		}
		evt.Wait()
	}
	return nil
}
