package dell_ec 

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"

	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/event"
 	domain "github.com/superchalupa/sailfish/src/redfishresource"

)

func setup(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, d *domain.DomainObjects) {
	// 2 instances of AM3 service. This means we can run concurrent message processing loops in 2 different goroutines
	// Each goroutine has exclusive access to its database
	

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
			d.GetBus().PublishEvent(context.Background(), evt)
			evt.Wait()
		}
	}

	// After we have our event loops set up,
	// Read the config file and process any startup events that are listed
	startup := cfgMgr.GetStringSlice("main.startup")
	for _, section := range startup {
		injectStartupEvents(section)
	}
}
