package udb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	eh "github.com/looplab/eventhorizon"
	"golang.org/x/xerrors"

	. "github.com/superchalupa/sailfish/cmd/metric-engine/metric"
	log "github.com/superchalupa/sailfish/src/log"
)

type UDBMeta struct {
	queryStr string
	query    *sqlx.Stmt
	lastTS   int64
	importFn func(string, ...string) error
}

type UDBFactory struct {
	logger   log.Logger
	database *sqlx.DB
	udb      map[string]UDBMeta
	bus      eh.EventBus
}

func NewUDBFactory(logger log.Logger, database *sqlx.DB, d BusComponents) (*UDBFactory, error) {
	ret := &UDBFactory{}
	ret.logger = logger
	ret.database = database
	ret.bus = d.GetBus()
	ret.udb = map[string]UDBMeta{
		"Sensor": UDBMeta{
			queryStr: `SELECT UnitModifier,SensorType,ElementName as FriendlyFQDD,DeviceID as FQDD,CurrentReading, cast(((julianday('now') - 2440587.5)*86400 * 100000000) as integer) as __Last_Update_TS FROM CIMVIEW_DCIM_NumericSensor where FQDD = ? ;`,
			importFn: func(n string, args ...string) error { return ret.ImportSensorFQDD(n, args...) },
		},
	}
	return ret, nil
}

func (l *UDBFactory) Import(udbImportName string, args ...string) (err error) {
	meta, ok := l.udb[udbImportName]
	if !ok {
		return xerrors.Errorf("UDB table %s not set up in meta struct", udbImportName)
	}

	if meta.importFn == nil {
		return xerrors.Errorf("UDB table %s present in meta, but has no import function specified.", udbImportName)
	}

	return l.udb[udbImportName].importFn(udbImportName, args...)
}

func (l *UDBFactory) ImportERROR(udbImportName string, args ...string) (err error) {
	fmt.Printf("DONT YET KNOW HOW TO IMPORT: %s\n", udbImportName)
	return xerrors.New("UNKNOWN IMPORT")
}

func (l *UDBFactory) IterUDBTables(fn func(string) error) error {
	for udbImportName, _ := range l.udb {
		err := fn(udbImportName)
		if err != nil {
			return xerrors.Errorf("Stopped iteration due to iteration function returning error: %w", err)
		}
	}
	return nil
}

func (l *UDBFactory) PrepareAll() error {
	err := l.IterUDBTables(func(udbImportName string) error {
		var err error
		udbMeta := l.udb[udbImportName]
		if udbMeta.query != nil {
			// already prepared, that's ok
			return nil
		}

		if udbMeta.queryStr == "" {
			// no query string, that's not ok
			return errors.New("Prepare failed for (%s)... missing query string")
		}

		udbMeta.query, err = l.database.Preparex(udbMeta.queryStr)
		if err != nil {
			fmt.Printf("Prepare failed for %s: %s\n", udbImportName, err)
			return xerrors.Errorf("Prepare failed for %s: %w", udbImportName, err)
		}
		udbMeta.queryStr = ""

		l.udb[udbImportName] = udbMeta
		return nil
	})

	return err
}

const maximport = 50

func (l *UDBFactory) ImportSensorFQDD(udbImportName string, args ...string) (err error) {
	err = nil
	events := []eh.EventData{}
	defer func() {
		if err == nil {
			if len(events) > 0 {
				fmt.Printf("Emitted %d events for table %s.\n", len(events), udbImportName)
			}
		} else {
			fmt.Printf("Emitted %d events for table %s. (err: %s)\n", len(events), udbImportName, err)
		}
	}()

	udbMeta, ok := l.udb[udbImportName]
	if !ok {
		err = xerrors.Errorf("Somehow got called with a table name not in our supported tables list.")
		return
	}

	if udbMeta.query == nil {
		err = xerrors.Errorf("SQL query wasn't prepared for %s! QUERY(%s)", udbImportName, udbMeta.queryStr)
		return
	}

	fqdd := "invalid"
	if len(args) > 0 {
		fqdd = args[0]
	}
	rows, err := udbMeta.query.Queryx(fqdd)
	if err != nil {
		err = xerrors.Errorf("query failed for %s: %w", udbImportName, err)
		return
	}
	defer rows.Close()

	var fqddmaps []*FQDDMappingData
	for rows.Next() {
		mm := map[string]interface{}{}
		err = rows.MapScan(mm)
		if err != nil {
			l.logger.Crit("Error mapscan", "err", err, "udbImportName", udbImportName)
			continue
		}

		// ================================================
		// Timestamp for metrics in this row
		LastTS, ok := mm["__Last_Update_TS"].(int64)
		if !ok {
			l.logger.Crit("last ts not int64", "udbImportName", udbImportName, "mm", mm)
			continue
		}
		delete(mm, "__Last_Update_TS")

		// ================================================
		// Human-readable friendly FQDD for this row
		FriendlyFQDDBytes, ok := mm["FriendlyFQDD"].([]byte)
		if !ok {
			l.logger.Crit("friendly fqdd not string", "udbImportName", udbImportName, "mm", mm)
			continue
		}
		delete(mm, "FriendlyFQDD")
		FriendlyFQDD := string(FriendlyFQDDBytes)

		// ================================================
		// FQDD for this row
		FQDDBytes, ok := mm["FQDD"].([]byte)
		if !ok {
			l.logger.Crit("friendly fqdd not string", "udbImportName", udbImportName, "mm", mm)
			continue
		}
		delete(mm, "FQDD")
		FQDD := string(FQDDBytes)

		abyteVal, ok := mm["CurrentReading"].(int64)
		event := &MetricValueEventData{
			Timestamp: SqlTimeInt{time.Unix(0, LastTS)},
			Name:      "invalid",
			Value:     fmt.Sprintf("%d", abyteVal),
			Context:   FQDD,
			Property:  "UDB:" + udbImportName + ":" + FQDD,
		}

		// Metric ID mappings
		sensorType, ok := mm["SensorType"].(int64)
		switch sensorType {
		case 0: // invalid?
		case 1: //
			// 0|1|System Board IO Usage|System Board IO Usage|iDRAC.Embedded.1#SystemBoardIOUsage|0|1575707108
			// 0|1|System Board MEM Usage|System Board MEM Usage|iDRAC.Embedded.1#SystemBoardMEMUsage|0|1575707108
			// 0|1|System Board SYS Usage|System Board SYS Usage|iDRAC.Embedded.1#SystemBoardSYSUsage|0|1575707108
			// 0|1|System Board CPU Usage|System Board CPU Usage|iDRAC.Embedded.1#SystemBoardCPUUsage|0|1575707108
			switch FQDD {
			case "iDRAC.Embedded.1#SystemBoardCPUUsage":
				event.Name = "CPUUsage"
			case "iDRAC.Embedded.1#SystemBoardSYSUsage":
				event.Name = "SystemUsage"
			case "iDRAC.Embedded.1#SystemBoardMEMUsage":
				event.Name = "MemoryUsage"
			case "iDRAC.Embedded.1#SystemBoardIOUsage":
				event.Name = "IOUsage"
			}
		case 2:
			event.Name = "TemperatureReading"
		case 3:
			event.Name = "VoltageReading"
		case 5:
			event.Name = "RPMReading"
		}

		events = append(events, event)

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

		// dont add too many events
		if len(events) > maximport {
			break
		}
	}
	if len(fqddmaps) > 0 {
		_ = l.bus.PublishEvent(context.Background(), eh.NewEvent(FriendlyFQDDMapping, fqddmaps, time.Now()))
	}
	err = l.bus.PublishEvent(context.Background(), eh.NewEvent(MetricValueEvent, events, time.Now()))

	return nil
}
