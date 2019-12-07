package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	eh "github.com/looplab/eventhorizon"

	log "github.com/superchalupa/sailfish/src/log"
)

type HWM struct {
	query  *sqlx.Stmt
	lastTS int64
}

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
			"StorageDiskSMARTData": HWM{lastTS: 0},
			"StorageSensor":        HWM{lastTS: 0},
			"ThermalMetrics":       HWM{lastTS: 0},
		},

		// These tables require special handling because they formatted differently. :(
		//		legacyHWM: map[string]HWM{
		//  		"CPUSensor":            HWM{lastTS: 0},
		//			"Sensor":               HWM{lastTS: 0},
		// 			"ThermalSensor":        HWM{lastTS: 0},
		//		}
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
		var fqddmaps []*FQDDMappingData
		for rows.Next() {
			mm := map[string]interface{}{}
			err = rows.MapScan(mm)
			if err != nil {
				l.logger.Crit("Error mapscan", "err", err, "legacyTableName", legacyTableName)
				continue
			}

			// ================================================
			// fields we don't need for the new implementation
			delete(mm, "__ISO_8601_TS")
			delete(mm, "__State_Marker")

			// ================================================
			// Timestamp for metrics in this row
			LastTS, ok := mm["__Last_Update_TS"].(int64)
			if !ok {
				l.logger.Crit("last ts not int64", "legacyTableName", legacyTableName, "mm", mm)
				continue
			}
			if LastTS > hwm.lastTS {
				hwm.lastTS = LastTS
			}
			delete(mm, "__Last_Update_TS")

			// ================================================
			// Human-readable friendly FQDD for this row
			FriendlyFQDDBytes, ok := mm["FriendlyFQDD"].([]byte)
			if !ok {
				l.logger.Crit("friendly fqdd not string", "legacyTableName", legacyTableName, "mm", mm)
				continue
			}
			delete(mm, "FriendlyFQDD")
			FriendlyFQDD := string(FriendlyFQDDBytes)

			// ================================================
			// FQDD for this row
			FQDDBytes, ok := mm["UNIQUEID"].([]byte)
			if !ok {
				l.logger.Crit("friendly fqdd not string", "legacyTableName", legacyTableName, "mm", mm)
				continue
			}
			delete(mm, "UNIQUEID")
			FQDD := string(FQDDBytes)

			// ================================================
			// NOW: for each COLUMN, we emit a MetricValueEvent with the above TS/FQDD/etc
			for key, value := range mm {
				switch key {
				// if one of the tables has this info in it, it requires special handling and must be moved to a different handler
				// This shouldn't be hit after we finish going through this, so this code should be removed from the final handler
				case "DeviceId", "SensorName", "__UnitModifier", "MetricPrefix":
					l.logger.Crit("FOUND A BAD TABLE. Take it out of the legacy table list, it needs special handling.", "legacyTableName", legacyTableName, "key", key)
					continue
				}

				if value != nil {
					abyteVal, ok := value.([]byte)
					if !ok {
						fmt.Printf("The value(%s) for key(%s) isnt a []byte, skipping...\n", value, key)
						continue
					}
					mm[key] = string(abyteVal)
					events = append(events, &MetricValueEventData{
						Timestamp: SqlTimeInt{time.Unix(LastTS, 0)},
						Name:      key,
						Value:     string(abyteVal),
						Context:   FQDD,
						Property:  "LEGACY:" + legacyTableName,
					})
				}
			}

			l.legacyHWM[legacyTableName] = hwm

			// Add a new friendly fqdd mapping. Only add each mapping once
			if FQDD != FriendlyFQDD {
				found := false
				for _, f := range fqddmaps {
					if FQDD == f.FQDD && FriendlyFQDD == f.FriendlyName {
						found = true
					}
				}
				if !found {
					fqddmaps = append(fqddmaps, &FQDDMappingData{FQDD: FQDD, FriendlyName: FriendlyFQDD})
				}
			}
		}
		fmt.Printf("\tEmitted %d events for table %s\n", len(events), legacyTableName)
		if len(fqddmaps) > 0 {
			l.bus.PublishEvent(context.Background(), eh.NewEvent(FriendlyFQDDMapping, fqddmaps, time.Now()))
		}
		l.bus.PublishEvent(context.Background(), eh.NewEvent(MetricValueEvent, events, time.Now()))
	}

	return nil
}
