// +build !skip_ec

package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	eh "github.com/looplab/eventhorizon"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/godefs"
	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/am3"
	"github.com/superchalupa/sailfish/src/ocp/event"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

const (
	ComponentEvent   eh.EventType = "ComponentEvent"
	MetricValueEvent eh.EventType = "MetricValueEvent"
)

type ComponentEventData struct {
	FQDD       string
	ParentFQDD string
	TESTING    string
}

type MetricValueEventData struct {
	Timestamp   time.Time
	MetricID    string
	MetricValue string
	URI         string
	Property    string
	Context     string
	Label       string
}

func init() {
	implementations["idrac"] = func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, viperMu *sync.RWMutex, ch eh.CommandHandler, eb eh.EventBus, d *domain.DomainObjects) Implementation {
		eh.RegisterEventData(ComponentEvent, func() eh.EventData { return &ComponentEventData{TESTING: "foobar"} })
		eh.RegisterEventData(MetricValueEvent, func() eh.EventData { return &MetricValueEventData{} })

		// set up the event dispatcher
		event.Setup(ch, eb)
		domain.StartInjectService(logger, d)
		godefs.InitGoDef()

		am3Svc, _ := am3.StartService(ctx, logger.New("module", "AM3"), eb, ch, d)
		addAM3Functions(logger.New("module", "idrac_am3_functions"), am3Svc, d)

		database, _ := sql.Open("sqlite3", "./metricvalues.db")
		statement, _ := database.Prepare("CREATE TABLE IF NOT EXISTS metricvalues (ts datetime, metricid varchar(64), metricvalue varchar(64))")
		statement.Exec()

		metricInserts := map[string]*sql.Stmt{}

		statement, _ = database.Prepare("INSERT INTO metricvalues (ts, metricid, metricvalue) VALUES (?, ?, ?)")
		metricInserts["RPMReading"] = statement

		am3Svc.AddEventHandler("metric_storage", MetricValueEvent, func(event eh.Event) {
			fmt.Printf("HELLO WORLD\n")
			metricValue, ok := event.Data().(*MetricValueEventData)
			if !ok {
				fmt.Println("Should never happen: got a metric value event without metricvalueeventdata")
				return
			}
			fmt.Println("DEBUG:", metricValue.Timestamp, metricValue.MetricID, metricValue.MetricValue)
			statement.Exec(metricValue.Timestamp, metricValue.MetricID, metricValue.MetricValue)

		})

		return nil
	}
}

func addAM3Functions(logger log.Logger, am3Svc *am3.Service, d *domain.DomainObjects) {
	am3Svc.AddEventHandler("modular_update_fan_data", eh.EventType("thp_fan_data_object"), func(event eh.Event) {
		dmobj, ok := event.Data().(*godefs.DMObject)
		fanobj, ok := dmobj.Data.(*godefs.DM_thp_fan_data_object)
		if !ok {
			logger.Error("updateFanData did not have fan event", "type", event.EventType, "data", event.Data())
			return
		}

		FullFQDD, err := dmobj.GetStringFromOffset(int(fanobj.OffsetKey))
		if err != nil {
			logger.Error("Got an thp_fan_data_object that somehow didn't have a string for FQDD.", "err", err)
			return
		}

		FQDDParts := strings.SplitN(FullFQDD, "#", 2)
		if len(FQDDParts) < 2 {
			logger.Error("Got an thp_fan_data_object with an FQDD that had no hash. Shouldnt happen", "FQDD", FullFQDD)
			return
		}

		URI := "/redfish/v1/Chassis/" + FQDDParts[0] + "/Sensors/Fans/" + FQDDParts[1]
		d.EventBus.PublishEvent(context.Background(), eh.NewEvent(MetricValueEvent, &MetricValueEventData{
			Timestamp:   time.Now(),
			MetricID:    "RPMReading",
			MetricValue: fmt.Sprintf("%d", (fanobj.Rotor1rpm+fanobj.Rotor2rpm)/2),
			URI:         URI,
			Property:    "Reading",
			Context:     FullFQDD,
			Label:       "fixme label",
		}, time.Now()))

		d.EventBus.PublishEvent(context.Background(), eh.NewEvent(MetricValueEvent, &MetricValueEventData{
			Timestamp:   time.Now(),
			MetricID:    "RPMPct",
			MetricValue: fmt.Sprintf("%d", (fanobj.Int)),
			URI:         URI,
			Property:    "OEM/Reading",
			Context:     FullFQDD,
			Label:       "fixme label",
		}, time.Now()))

	})
}
