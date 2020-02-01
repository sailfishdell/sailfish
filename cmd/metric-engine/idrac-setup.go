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
	"github.com/superchalupa/sailfish/cmd/metric-engine/telemetry-db"
	"github.com/superchalupa/sailfish/cmd/metric-engine/udb"
)

func setup(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, d *busComponents) {
	// register global metric events with event horizon
	metric.RegisterEvent()

	// cgo will start up its own thread and event processing loop
	cgoStartup(logger.New("module", "cgo"), d)

	// We are going to initialize 2 instances of AM3 service.  This means we can
	// run concurrent message processing loops in 2 different goroutines Each
	// goroutine has exclusive access to its database, so we'll be able to
	// simultaneously do ops on each DB

	// Processing loop 1:
	//  	-- "New" DB access
	am3SvcN2, _ := am3.StartService(ctx, logger.New("module", "AM3_DB"), "database", d)
	err := telemetry.StartupTelemetryBase(logger.New("module", "sql_am3_functions"), cfgMgr, am3SvcN2, d)
	if err != nil {
		panic("Error initializing: " + err.Error())
	}

	// Processing loop 2:
	//  	-- UDB access
	am3SvcN3, _ := am3.StartService(ctx, logger.New("module", "AM3_UDB"), "udb database", d)
	udb.StartupUDBImport(logger.New("module", "udb_am3_functions"), cfgMgr, am3SvcN3, d)

	injectStartupEvents := func(section string) {
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
				// well if it doesn't unmarshall, try to just send it as a string (Used for sending DatabaseMaintenance events.
				eventData = dataString
			}

			fmt.Printf("\tPublishing Startup Event (%s)\n", name)
			evt := event.NewSyncEvent(eventType, eventData, time.Now())
			evt.Add(1)
			err = d.GetBus().PublishEvent(context.Background(), evt)
			if err != nil {
				logger.Crit("Error publishing event to internal event bus, should never happen!", "err", err)
			}
		}
	}

	// After we have our event loops set up,
	// Read the config file and process any startup events that are listed
	startup := cfgMgr.GetStringSlice("main.startup")
	for _, section := range startup {
		injectStartupEvents(section)
	}
}

func shutdown() {
	cgoShutdown()
}
