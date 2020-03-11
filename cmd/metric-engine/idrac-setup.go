package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	eh "github.com/looplab/eventhorizon"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/viper"

	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/event"
	"github.com/superchalupa/sailfish/src/ocp/am3"

	"github.com/superchalupa/sailfish/cmd/metric-engine/metric"
	"github.com/superchalupa/sailfish/cmd/metric-engine/telemetry"
	"github.com/superchalupa/sailfish/cmd/metric-engine/triggers"
	"github.com/superchalupa/sailfish/cmd/metric-engine/udb"
	"github.com/superchalupa/sailfish/cmd/metric-engine/watchdog"
)

func init() {
	initOptional()
	optionalComponents = append(optionalComponents, func(logger log.Logger, cfg *viper.Viper, d *busComponents) func() {
		setup(context.Background(), logger, cfg, d)
		return nil
	})
}

func setup(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, d *busComponents) {
	// register global metric events with event horizon
	metric.RegisterEvent()

	// We are going to initialize 2 instances of AM3 service.  This means we can
	// run concurrent message processing loops in 2 different goroutines Each
	// goroutine has exclusive access to its database, so we'll be able to
	// simultaneously do ops on each DB

	// Processing loop 1:
	//  	-- "New" DB access
	am3SvcN2, _ := am3.StartService(ctx, logger.New("module", "AM3_DB"), "database", d)
	err := telemetry.StartupTelemetryBase(logger.New("module", "sql_am3_functions"), cfgMgr, am3SvcN2, d)
	if err != nil {
		panic("Error initializing base telemetry subsystem: " + err.Error())
	}

	// After base event loop, process startup events. They all belong to the base loop.
	// config file main.startup specifies the section(s) that have the list of events
	cfgMgr.SetDefault("main.startup", "startup-events")
	startup := cfgMgr.GetStringSlice("main.startup")
	for _, section := range startup {
		injectStartupEvents(logger, cfgMgr, section, d.GetBus())
	}

	// Processing loop 2:
	//  	-- UDB access
	am3SvcN3, _ := am3.StartService(ctx, logger.New("module", "AM3_UDB"), "udb database", d)
	err = udb.StartupUDBImport(logger.New("module", "udb_am3_functions"), cfgMgr, am3SvcN3, d)
	if err != nil {
		panic("Error initializing UDB Import subsystem: " + err.Error())
	}

	//-- Trigger processing
	err = triggers.StartupTriggerProcessing(logger.New("module", "trigger_am3_functions"), cfgMgr, am3SvcN2, d)
	if err != nil {
		panic("Error initializing trigger processing subsystem: " + err.Error())
	}

	// Watchdog
	err = watchdog.StartWatchdogHandling(logger, am3SvcN2, d)
	if err != nil {
		panic("Error initializing trigger processing subsystem: " + err.Error())
	}

	// if UDB event loop needs any events (none currently), then we could have a separate list and process that list here.
}

func injectStartupEvents(logger log.Logger, cfgMgr *viper.Viper, section string, bus eh.EventBus) {
	fmt.Printf("Processing Startup Events from %s\n", section)
	startup := cfgMgr.Get(section)
	if startup == nil {
		logger.Warn("SKIPPING: no startup events found")
		return
	}
	events, ok := startup.([]interface{})
	if !ok {
		logger.Crit("SKIPPING: Startup Events skipped - malformed.", "Section", section, "malformed-value", startup)
		return
	}
	for i, v := range events {
		settings, ok := v.(map[interface{}]interface{})
		if !ok {
			logger.Crit("SKIPPING: malformed event. Expected map", "Section", section, "index", i, "malformed-value", v, "TYPE", fmt.Sprintf("%T", v))
			continue
		}

		name, ok := settings["name"].(string)
		if !ok {
			logger.Crit("SKIPPING: Config file section missing event name- 'name' key missing.", "Section", section, "index", i, "malformed-value", v)
			continue
		}
		eventType := eh.EventType(name + "Event")

		dataString, ok := settings["data"].(string)
		if !ok {
			logger.Crit("SKIPPING: Config file section missing event name- 'data' key missing.", "Section", section, "index", i, "malformed-value", v)
			continue
		}

		eventData, err := eh.CreateEventData(eventType)
		if err != nil {
			logger.Crit("SKIPPING: couldnt instantiate event", "Section", section, "index", i, "malformed-value", v, "event", name, "err", err)
			continue
		}

		err = json.Unmarshal([]byte(dataString), &eventData)
		if err != nil {
			logger.Crit("Unmarshal error", "err", err, "json_string", dataString)
			continue
		}

		fmt.Printf("\tPublishing Startup Event (%s)\n", name)
		evt := event.NewSyncEvent(eventType, eventData, time.Now())
		evt.Add(1)
		err = bus.PublishEvent(context.Background(), evt)
		if err != nil {
			logger.Crit("Error publishing event to internal event bus, should never happen!", "err", err)
		}
		evt.Wait()
	}
}
