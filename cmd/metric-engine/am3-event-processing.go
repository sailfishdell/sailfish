// +build idrac

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
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

const (
	MetricValueEvent eh.EventType = "MetricValueEvent"
)

type MetricValueEventData struct {
	Timestamp   time.Time
	MetricID    string
	MetricValue string
	URI         string
	Property    string
	Context     string
	Label       string

	StablePeriod            bool
	ReportPeriod            time.Duration
	RepeatPrevious          string
	RepeatPreviousSupported bool
	StopSupported           bool
	Stop                    bool
}

func addAM3Functions(logger log.Logger, am3Svc *am3.Service, d *domain.DomainObjects) {
	eh.RegisterEventData(MetricValueEvent, func() eh.EventData { return &MetricValueEventData{} })

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
