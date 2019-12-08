package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	eh "github.com/looplab/eventhorizon"
	"golang.org/x/xerrors"

	log "github.com/superchalupa/sailfish/src/log"
)

type LegacyMeta struct {
	query    *sqlx.Stmt
	lastTS   int64
	importFn func(string) error
}

type LegacyFactory struct {
	logger   log.Logger
	database *sqlx.DB
	legacy   map[string]LegacyMeta
	bus      eh.EventBus
}

func NewLegacyFactory(logger log.Logger, database *sqlx.DB, d *BusComponents) (*LegacyFactory, error) {
	ret := &LegacyFactory{}
	ret.logger = logger
	ret.database = database
	ret.bus = d.GetBus()
	ret.legacy = map[string]LegacyMeta{
		"AggregationMetrics":   LegacyMeta{importFn: func(n string) error { return ret.ImportByColumn(n) }},
		"CPUMemMetrics":        LegacyMeta{importFn: func(n string) error { return ret.ImportByColumn(n) }},
		"CPURegisters":         LegacyMeta{importFn: func(n string) error { return ret.ImportByColumn(n) }},
		"CUPS":                 LegacyMeta{importFn: func(n string) error { return ret.ImportByColumn(n) }},
		"FCSensor":             LegacyMeta{importFn: func(n string) error { return ret.ImportByColumn(n) }},
		"FPGASensor":           LegacyMeta{importFn: func(n string) error { return ret.ImportByColumn(n) }},
		"FanSensor":            LegacyMeta{importFn: func(n string) error { return ret.ImportByColumn(n) }},
		"GPUMetrics":           LegacyMeta{importFn: func(n string) error { return ret.ImportByColumn(n) }},
		"GPUStatistics":        LegacyMeta{importFn: func(n string) error { return ret.ImportByColumn(n) }},
		"MemorySensor":         LegacyMeta{importFn: func(n string) error { return ret.ImportByColumn(n) }},
		"NICSensor":            LegacyMeta{importFn: func(n string) error { return ret.ImportByColumn(n) }},
		"NICStatistics":        LegacyMeta{importFn: func(n string) error { return ret.ImportByColumn(n) }},
		"NVMeSMARTData":        LegacyMeta{importFn: func(n string) error { return ret.ImportByColumn(n) }},
		"PSUMetrics":           LegacyMeta{importFn: func(n string) error { return ret.ImportByColumn(n) }},
		"PowerMetrics":         LegacyMeta{importFn: func(n string) error { return ret.ImportByColumn(n) }},
		"PowerStatistics":      LegacyMeta{importFn: func(n string) error { return ret.ImportByColumn(n) }},
		"StorageDiskSMARTData": LegacyMeta{importFn: func(n string) error { return ret.ImportByColumn(n) }},
		"StorageSensor":        LegacyMeta{importFn: func(n string) error { return ret.ImportByColumn(n) }},
		"ThermalMetrics":       LegacyMeta{importFn: func(n string) error { return ret.ImportByColumn(n) }},

		// These tables require special handling because they formatted differently. :(
		// dont' have an importer here yet. Please write one
		"CPUSensor":     LegacyMeta{importFn: func(n string) error { return ret.ImportERROR(n) }},
		"Sensor":        LegacyMeta{importFn: func(n string) error { return ret.ImportERROR(n) }},
		"ThermalSensor": LegacyMeta{importFn: func(n string) error { return ret.ImportERROR(n) }},
	}
	return ret, nil
}

func (l *LegacyFactory) Import(legacyTableName string) error {
	meta, ok := l.legacy[legacyTableName]
	if !ok {
		return xerrors.Errorf("Legacy table %s not set up in meta struct", legacyTableName)
	}

	if meta.importFn == nil {
		return xerrors.Errorf("Legacy table %s present in meta, but has no import function specified.", legacyTableName)
	}

	return l.legacy[legacyTableName].importFn(legacyTableName)
}

func (l *LegacyFactory) ImportERROR(legacyTableName string) error {
	fmt.Printf("DONT YET KNOW HOW TO IMPORT: %s\n", legacyTableName)
	return xerrors.New("UNKNOWN IMPORT")
}

func (l *LegacyFactory) IterLegacyTables(fn func(string) error) error {
	for legacyTableName, _ := range l.legacy {
		err := fn(legacyTableName)
		if err != nil {
			return xerrors.Errorf("Stopped iteration due to iteration function returning error: %w", err)
		}
	}
	return nil
}

func (l *LegacyFactory) PrepareAll() error {
	return l.IterLegacyTables(func(legacyTableName string) error {
		var err error
		legacyMeta := l.legacy[legacyTableName]
		if legacyMeta.query != nil {
			// already prepared, that's ok
			return nil
		}

		querytext := `select * from ` + legacyTableName + ` where __Last_Update_TS > ?;`
		legacyMeta.query, err = l.database.Preparex(querytext)
		if err != nil {
			return xerrors.Errorf("Prepare failed for %s: %w", legacyTableName, err)
		}

		l.legacy[legacyTableName] = legacyMeta
		return nil
	})
}

func (l *LegacyFactory) ImportByColumn(legacyTableName string) (err error) {
	events := []eh.EventData{}
	defer func() {
		if err == nil {
			fmt.Printf("Emitted %d events for table %s\n", len(events), legacyTableName)
		} else {
			fmt.Printf("Emitted %d events for table %s (err: %s)\n", len(events), legacyTableName, err)
		}
	}()

	legacyMeta, ok := l.legacy[legacyTableName]
	if !ok {
		err = xerrors.Errorf("Somehow got called with a table name not in our supported tables list.")
		return
	}

	if legacyMeta.query == nil {
		err = xerrors.Errorf("SQL query wasn't prepared for %s!", legacyTableName)
		return
	}

	rows, err := legacyMeta.query.Queryx(legacyMeta.lastTS)
	if err != nil {
		err = xerrors.Errorf("query failed for %s: %w", legacyTableName, err)
		return
	}

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
		if LastTS > legacyMeta.lastTS {
			legacyMeta.lastTS = LastTS
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

		l.legacy[legacyTableName] = legacyMeta

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
	if len(fqddmaps) > 0 {
		_ = l.bus.PublishEvent(context.Background(), eh.NewEvent(FriendlyFQDDMapping, fqddmaps, time.Now()))
	}
	err = l.bus.PublishEvent(context.Background(), eh.NewEvent(MetricValueEvent, events, time.Now()))

	return nil
}
