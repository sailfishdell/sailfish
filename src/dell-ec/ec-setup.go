package dell_ec

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"

	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/event"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

func getStartupFileContents(logger log.Logger, d *domain.DomainObjects) *viper.Viper {
	cfgMgr := viper.New()
	// Environment variables
	cfgMgr.SetEnvPrefix("startup")
	cfgMgr.AutomaticEnv()

	// Configuration file
	cfgMgr.SetConfigName("redfish-startup")
	cfgMgr.AddConfigPath(".")
	cfgMgr.AddConfigPath("/etc/")
	if err := cfgMgr.ReadInConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Could not read config file: %s\n", err)
		panic(fmt.Sprintf("Could not read config file: %s", err))
	}

	return cfgMgr
}

func setup(logger log.Logger, d *domain.DomainObjects) {
	// get start up events
	setupCfg := getStartupFileContents(logger, d)

	injectStartupEvents := func(section string) {
		fmt.Printf("Processing Startup Events from %s\n", section)
		startup := setupCfg.Get(section)
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

			//fmt.Printf("\tPublishing Startup Event (%s)\n", name)
			evt := event.NewSyncEvent(eventType, eventData, time.Now())
			evt.Add(1)
			d.EventBus.PublishEvent(context.Background(), evt)
			evt.Wait()
		}
		fmt.Println("Published all Startup Events")
	}

	// After we have our event loops set up,
	// Read the config file and process any startup events that are listed
	startup := setupCfg.GetStringSlice("main.startup")
	for _, section := range startup {
		injectStartupEvents(section)
	}
}
