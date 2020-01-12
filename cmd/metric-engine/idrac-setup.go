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

	// After we have our event loops set up,
	// Read the config file and process any startup events that are listed
	const section = "Startup-Events"
	fmt.Printf("Processing Startup Events\n")
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

		fmt.Printf("\tStartup Event: %s\n", name)
		evt := event.NewSyncEvent(eventType, eventData, time.Now())
		evt.Add(1)
		d.GetBus().PublishEvent(context.Background(), evt)
		evt.Wait()
	}
}

func shutdown() {
	cgoShutdown()
}
