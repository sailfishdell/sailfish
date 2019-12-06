package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	eh "github.com/looplab/eventhorizon"

	log "github.com/superchalupa/sailfish/src/log"
)

const ()

type HWM struct {
	query  *sqlx.Stmt
	lastTS int64
}

// Factory manages getting/putting into db
type LegacyFactory struct {
	logger    log.Logger
	database  *sqlx.DB
	legacyHWM map[string]HWM
	bus       eh.EventBus
}

func NewLegacyFactory(logger log.Logger, database *sqlx.DB, d *BusComponents) (ret *LegacyFactory, err error) {

	ret = &LegacyFactory{
		logger: logger, database: database,
		bus: d.GetBus(),
		legacyHWM: map[string]HWM{
			"AggregationMetrics":   HWM{lastTS: 0},
			"CPUMemMetrics":        HWM{lastTS: 0},
			"CPURegisters":         HWM{lastTS: 0},
			"CPUSensor":            HWM{lastTS: 0},
			"CUPS":                 HWM{lastTS: 0},
			"FCSensor":             HWM{lastTS: 0},
			"FPGASensor":           HWM{lastTS: 0},
			"FanSensor":            HWM{lastTS: 0},
			"GPUMetrics":           HWM{lastTS: 0},
			"GPUStatistics":        HWM{lastTS: 0},
			"MemorySensor":         HWM{lastTS: 0},
			"NICSensor":            HWM{lastTS: 0},
			"NICStatistics":        HWM{lastTS: 0},
			"NVMeSMARTData":        HWM{lastTS: 0},
			"PSUMetrics":           HWM{lastTS: 0},
			"PowerMetrics":         HWM{lastTS: 0},
			"PowerStatistics":      HWM{lastTS: 0},
			"Sensor":               HWM{lastTS: 0},
			"StorageDiskSMARTData": HWM{lastTS: 0},
			"StorageSensor":        HWM{lastTS: 0},
			"ThermalMetrics":       HWM{lastTS: 0},
			"ThermalSensor":        HWM{lastTS: 0},
		},
	}
	err = nil
	return
}

func (l *LegacyFactory) Import() error {
	for legacyTableName, hwm := range l.legacyHWM {
		fmt.Printf("IMPORTING... %s\n", legacyTableName)

		if hwm.query == nil {
			querytext := `select * from ` + legacyTableName + ` where __Last_Update_TS > ?;`
			q, err := l.database.Preparex(querytext)
			if err != nil {
				l.logger.Crit("Prepare failed", "err", err, "legacyTableName", legacyTableName)
				continue
			}
			hwm.query = q
		}

		rows, err := hwm.query.Queryx(hwm.lastTS)
		if err != nil {
			l.logger.Crit("Error querying for metrics", "err", err, "legacyTableName", legacyTableName)
			continue
		}

		events := []eh.EventData{}
		fqddmaps := []eh.EventData{}
		for rows.Next() {
			mm := map[string]interface{}{}
			err = rows.MapScan(mm)
			if err != nil {
				l.logger.Crit("Error mapscan", "err", err, "legacyTableName", legacyTableName)
				continue
			}

			// FriendlyFQDD
			// UNIQUEID
			// __Last_Update_TS
			// __ISO_8601_TS
			// __State_Marker
			LastTS, ok := mm["__Last_Update_TS"].(int64)
			if !ok {
				l.logger.Crit("last ts not int64", "legacyTableName", legacyTableName, "mm", mm)
				continue
			}
			if LastTS > hwm.lastTS {
				hwm.lastTS = LastTS
			}
			delete(mm, "__Last_Update_TS")

			FriendlyFQDDBytes, ok := mm["FriendlyFQDD"].([]byte)
			if !ok {
				l.logger.Crit("friendly fqdd not string", "legacyTableName", legacyTableName, "mm", mm)
				continue
			}
			delete(mm, "FriendlyFQDD")
			FriendlyFQDD := string(FriendlyFQDDBytes)

			// dont care about this one
			delete(mm, "__ISO_8601_TS")
			delete(mm, "__State_Marker")

			FQDDBytes, ok := mm["UNIQUEID"].([]byte)
			if !ok {
				l.logger.Crit("friendly fqdd not string", "legacyTableName", legacyTableName, "mm", mm)
				continue
			}
			delete(mm, "UNIQUEID")
			FQDD := string(FQDDBytes)

			// parse out std stuff:
			fmt.Printf("GOT a ROw!\n\tFriendlyFQDD: %s\n\tLastTS: %d\n\tFQDD: %s\n\t--> %v\n", FriendlyFQDD, LastTS, FQDD, mm)
			l.legacyHWM[legacyTableName] = hwm

			for key, value := range mm {
				abyteVal, ok := value.([]byte)
				if !ok {
					fmt.Printf("The value(%s) for key(%s) isnt a []byte, skipping...\n", value, key)
					continue
				}
				events = append(events, &MetricValueEventData{
					Timestamp: SqlTimeInt{time.Unix(LastTS, 0)},
					Name:      key,
					Value:     string(abyteVal),
					Context:   FQDD,
					Property:  "LEGACY:" + legacyTableName,
				})
			}

			fqddmaps = append(fqddmaps, &FQDDMappingData{FQDD: FQDD, FriendlyName: FriendlyFQDD})

			// NOTE: publish can accept an array of EventData to reduce callbacks (recommended)
		}
		l.bus.PublishEvent(context.Background(), eh.NewEvent(FriendlyFQDDMapping, fqddmaps, time.Now()))
		l.bus.PublishEvent(context.Background(), eh.NewEvent(MetricValueEvent, events, time.Now()))
	}

	return nil
}
