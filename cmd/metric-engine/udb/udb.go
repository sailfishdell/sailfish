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
			// it's basically a VIEW...
			queryStr: `
			SELECT
			  cast( (julianday('now') - 2440587.5)*86400.0 * 1000000000 as integer) as Timestamp,
			  case
				  when SensorType = 0 then "InvalidMetric"
					when SensorType = 1 then
					case
						when DeviceID = "iDRAC.Embedded.1#SystemBoardCPUUsage" then "CPUUsage"
						when DeviceID = "iDRAC.Embedded.1#SystemBoardSYSUsage" then "SystemUsage"
						when DeviceID = "iDRAC.Embedded.1#SystemBoardMEMUsage" then "MemoryUsage"
						when DeviceID = "iDRAC.Embedded.1#SystemBoardIOUsage"  then "IOUsage"
					end
					when SensorType = 2 then "TemperatureReading"
					when SensorType = 3 then "VoltageReading"
					when SensorType = 5 then "RPMReading"
				end as Name,
				CurrentReading as Value,
				'/redfish/v1/Dell/Systems/System.Embedded.1/DellNumericSensorCollection/' || DeviceID as Property,  -- needs to be URI encoded, but close enough for now
				ElementName as Context   -- FriendlyFQDD
				-- COMMENT OUT UNTIL WE HAVE A DEST TO RECEIVE THIS: DeviceID as FQDD,
			FROM CIMVIEW_DCIM_NumericSensor;`,
			//where DeviceID = ? ;`,
			importFn: func(n string, args ...string) error { return ret.ImportByMetricValue(n, args...) },
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

func (l *UDBFactory) ImportByMetricValue(udbImportName string, args ...string) (err error) {
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

	rows, err := udbMeta.query.Queryx()
	if err != nil {
		err = xerrors.Errorf("query failed for %s: %w", udbImportName, err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		event := &MetricValueEventData{}
		err = rows.StructScan(event)
		if err != nil {
			l.logger.Crit("Error scanning row into MetricEvent", "err", err, "udbImportName", udbImportName)
			continue
		}

		events = append(events, event)
		if len(events) > maximport {
			l.bus.PublishEvent(context.Background(), eh.NewEvent(MetricValueEvent, events, time.Now()))
			events = []eh.EventData{}
		}
	}
	l.bus.PublishEvent(context.Background(), eh.NewEvent(MetricValueEvent, events, time.Now()))

	return nil
}

func getString(mm map[string]interface{}, name string) (ret string) {
	byt, ok := mm[name].([]byte)
	if ok {
		ret = string(byt)
	}
	return
}
