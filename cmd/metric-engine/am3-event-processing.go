package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	eh "github.com/looplab/eventhorizon"
	_ "github.com/mattn/go-sqlite3"

	"github.com/superchalupa/sailfish/godefs"
	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/am3"
)

const (
	MetricValueEvent eh.EventType = "MetricValueEvent"
)

type MetricValueEventData struct {
	Timestamp SqlTimeInt `db:"Timestamp"`
	Name      string     `db:"Name"`
	Value     string
	Property  string `db:"Property"`
	Context   string `db:"Context"`
}

func addAM3ConversionFunctions(logger log.Logger, am3Svc *am3.Service, d *BusComponents) {
	eh.RegisterEventData(MetricValueEvent, func() eh.EventData { return &MetricValueEventData{} })
	godefs.InitGoDef()

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

		// NOTE: publish can accept an array of EventData to reduce callbacks (recommended)
		d.EventBus.PublishEvent(context.Background(),
			eh.NewEvent(MetricValueEvent,
				[]eh.EventData{
					&MetricValueEventData{
						Timestamp: SqlTimeInt{time.Now()},
						Name:      "RPMReading",
						Value:     fmt.Sprintf("%d", (fanobj.Rotor1rpm+fanobj.Rotor2rpm)/2),
						Property:  URI + "#Reading",
						Context:   FullFQDD,
					},
					&MetricValueEventData{
						Timestamp: SqlTimeInt{time.Now()},
						Name:      "RPMPct",
						Value:     fmt.Sprintf("%d", (fanobj.Int)),
						Property:  URI + "#OEM/Reading",
						Context:   FullFQDD,
					}},
				time.Now()))
	})
}
