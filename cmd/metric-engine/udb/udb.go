package udb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"
	"golang.org/x/xerrors"

	"github.com/superchalupa/sailfish/cmd/metric-engine/metric"
	log "github.com/superchalupa/sailfish/src/log"
)

// probably should be a configuration item
const maximport = 200

type constErr string

func (e constErr) Error() string { return string(e) }

const disabled = constErr("importer disabled")
const stopIter = constErr("stop iteration")

type dataSource struct {
	query        *sqlx.NamedStmt
	importFn     func(string) error
	HWM          int64 `db:"HWM"`
	lastImport   time.Time
	waitInterval int64
	scanInterval int64
	dbChange     map[string]map[string]struct{}
}

type dataImporter struct {
	logger   log.Logger
	database *sqlx.DB
	udb      map[string]dataSource
	bus      eh.EventBus
}

func newImportManager(logger log.Logger, database *sqlx.DB, d busComponents, cfg *viper.Viper) (*dataImporter, error) {
	ret := &dataImporter{}
	ret.logger = logger
	ret.database = database
	ret.bus = d.GetBus()
	ret.udb = map[string]dataSource{}

	impFns := map[string]func(string) error{
		"DISABLE-DirectMetric": func(n string) error { return disabled },
		"DirectMetric":         ret.importByMetricValue,
		"MetricColumns":        ret.importByColumn,
		"Error":                ret.importERROR,
	}

	// Parse the YAML file to set up database imports
	for k, v := range cfg.GetStringMap("UDB-Metric-Import") {
		fmt.Printf("Loading config for %s\n", k)

		meta := dataSource{}

		settings, ok := v.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("config file malformed. Section(%s) key(%s) value(%s)", "UDB-Metric-Import", k, v)
		}

		Query, ok := settings["query"].(string)
		if !ok {
			return nil, fmt.Errorf("missing or malformed query from config file. Section(%s) key(%s) value(%s)", "UDB-Metric-Import", k, v)
		}

		var err error
		meta.query, err = database.PrepareNamed(Query)
		if err != nil {
			return nil, xerrors.Errorf("prepare() failed for query Section(%s), key(%s), sql query(%s): %w", "UDB-Metric-Import", k, Query, err)
		}

		Type, ok := settings["type"].(string)
		if !ok {
			Type = "MetricColumns" // default
		}

		_, ok = impFns[Type]
		if !ok {
			return nil, fmt.Errorf(
				"requested IMPL func doesn't exist. Section(%s), key(%s), Requested func(%s). Available funcs: %+v", "UDB-Metric-Import",
				k, Type, impFns)
		}

		scanInterval, ok := settings["scaninterval"].(int)
		if !ok {
			scanInterval = 0
		}
		meta.scanInterval = int64(scanInterval)

		waitInterval, ok := settings["waitinterval"].(int)
		if !ok {
			waitInterval = 5
		}
		meta.waitInterval = int64(waitInterval)

		err = cfg.UnmarshalKey("UDB-Metric-Import."+k+".DBChange", &meta.dbChange)
		if err != nil {
			return nil, xerrors.Errorf("parse error trying to read the DBChange key(%s): %w", "UDB-Metric-Import."+k+".DBChange", err)
		}

		meta.importFn = impFns[Type]
		ret.udb[k] = meta
	}

	return ret, nil
}

func (l *dataImporter) conditionalImport(udbImportName string, meta dataSource, periodic bool) (err error) {
	now := time.Now()
	if periodic && meta.scanInterval == 0 {
		return
	}
	if periodic && now.Before(meta.lastImport.Add(time.Duration(meta.scanInterval)*time.Second)) {
		return
	}
	if now.Before(meta.lastImport.Add(time.Duration(meta.waitInterval) * time.Second)) {
		return
	}

	meta.lastImport = now
	l.udb[udbImportName] = meta
	return l.udb[udbImportName].importFn(udbImportName)
}

func (l *dataImporter) dbChanged(database, table string) (err error) {
	return l.iterUDBTables(func(udbImportName string, meta dataSource) error {
		tbls, ok := meta.dbChange[database]
		if !ok {
			return nil
		}
		_, ok = tbls[table]
		if !ok {
			return nil
		}
		return l.conditionalImport(udbImportName, meta, false)
	})
}

func (l *dataImporter) importERROR(udbImportName string) (err error) {
	fmt.Printf("DONT YET KNOW HOW TO IMPORT: %s\n", udbImportName)
	return xerrors.New("UNKNOWN IMPORT")
}

func (l *dataImporter) iterUDBTables(fn func(string, dataSource) error) error {
	for udbImportName, meta := range l.udb {
		err := fn(udbImportName, meta)
		if err != nil && err != disabled && err != stopIter {
			return xerrors.Errorf("Stopped iteration due to iteration function returning error: %w", err)
		}
	}
	return nil
}

func (l *dataImporter) condSetHWM(udbImportName string, ts int64) {
	dataSource := l.udb[udbImportName]
	if ts > dataSource.HWM {
		dataSource.HWM = ts
		l.udb[udbImportName] = dataSource
	}
}

func (l *dataImporter) send(events *[]eh.EventData) {
	if len(*events) == 0 {
		return
	}
	err := l.bus.PublishEvent(context.Background(), eh.NewEvent(metric.MetricValueEvent, *events, time.Now()))
	if err != nil {
		l.logger.Crit("Error publishing event to internal event bus. Should never happen!", "err", err)
	}
	*events = []eh.EventData{}
}

// ImportByColumn will import a database rows where each column is a different metric.
//   Each column that is a metric has to have its column name prefixed with "Metric-"
//   Timestamps are constructed based on the "Timestamp" column
//   Metric Context is constructed based on the "Context" column
//   Metric FQDD is constructed based on the "FQDD" column
//   Metric FriendlyFQDD is constructed based on the "FriendlyFQDD" column
//   Property paths are constructed by appending '#<metricname>' to the "Property" column
func (l *dataImporter) importByColumn(udbImportName string) (err error) {
	err = nil
	events := []eh.EventData{}
	totalEvents := 0
	totalRows := 0
	defer func(totalEvents *int, totalRows *int, udbImportName string) {
		if err == nil {
			if *totalEvents > 0 {
				fmt.Printf("Processed %d rows. Emitted %d events for table(%s).\n", *totalRows, *totalEvents, udbImportName)
			}
		} else {
			fmt.Printf("Processed %d rows. Emitted %d events for table(%s). ERROR(%s).\n", *totalRows, *totalEvents, udbImportName, err)
		}
	}(&totalEvents, &totalRows, udbImportName)

	dataSource, ok := l.udb[udbImportName]
	if !ok {
		return xerrors.Errorf("Somehow got called with a table name not in our supported tables list.")
	}

	// if query isn't prepared, this will panic. That'll need to get fixed, as it's a config file error.
	rows, err := dataSource.query.Queryx(dataSource)
	if err != nil {
		return xerrors.Errorf("query failed for %s: %w", udbImportName, err)
	}
	defer rows.Close()

	for rows.Next() {
		mm := map[string]interface{}{}
		err = rows.MapScan(mm)
		if err != nil {
			l.logger.Crit("Error scanning row into MetricEvent", "err", err, "udbImportName", udbImportName)
			continue
		}
		totalRows++

		ts := getInt64(mm, "Timestamp")
		l.condSetHWM(udbImportName, ts)

		baseEvent := metric.MetricValueEventData{
			Timestamp:        metric.SQLTimeInt{Time: time.Unix(0, ts)},
			Context:          getString(mm, "Context"),
			FQDD:             getString(mm, "FQDD"),
			FriendlyFQDD:     getString(mm, "FriendlyFQDD"),
			Source:           udbImportName,
			MVRequiresExpand: getInt64(mm, "RequiresExpand") == 1,
			MVSensorSlack:    time.Duration(getInt64(mm, "SensorSlack")),
			MVSensorInterval: time.Duration(getInt64(mm, "SensorInterval")),
		}

		property := getString(mm, "Property")

		for k, v := range mm {
			if v == nil {
				// we dont add NULL metrics
				continue
			}

			if !strings.HasPrefix(k, "Metric-") {
				// not a metric
				continue
			}

			metricName := k[7:]
			eventToSend := baseEvent
			eventToSend.Property = property + "#" + metricName
			eventToSend.Value = fmt.Sprintf("%v", v)

			if mts, ok := mm["Timestamp-"+metricName].(int64); ok {
				l.condSetHWM(udbImportName, mts)
				eventToSend.Timestamp = metric.SQLTimeInt{Time: time.Unix(0, mts)}
			}

			totalEvents++
			events = append(events, &eventToSend)
			if len(events) > maximport {
				l.send(&events)
			}
		}
	}
	l.send(&events)

	return nil
}

func (l *dataImporter) importByMetricValue(udbImportName string) (err error) {
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

	dataSource, ok := l.udb[udbImportName]
	if !ok {
		err = xerrors.Errorf("Somehow got called with a table name not in our supported tables list.")
		return
	}

	// we'll panic here if query isn't prepared or if there was a syntax error in config file
	rows, err := dataSource.query.Queryx(dataSource)
	if err != nil {
		err = xerrors.Errorf("query failed for %s: %w", udbImportName, err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		event := &metric.MetricValueEventData{
			Source: udbImportName,
		}
		err = rows.StructScan(event)
		if err != nil {
			l.logger.Crit("Error scanning row into MetricEvent", "err", err, "udbImportName", udbImportName)
			continue
		}

		if ts := event.Timestamp.UnixNano(); ts > dataSource.HWM {
			dataSource.HWM = ts
			l.udb[udbImportName] = dataSource
		}

		events = append(events, event)
		if len(events) > maximport {
			err := l.bus.PublishEvent(context.Background(), eh.NewEvent(metric.MetricValueEvent, events, time.Now()))
			if err != nil {
				l.logger.Crit("Error publishing event to internal event bus. Should never happen!", "err", err)
			}
			events = []eh.EventData{}
		}
	}
	if len(events) > 0 {
		err := l.bus.PublishEvent(context.Background(), eh.NewEvent(metric.MetricValueEvent, events, time.Now()))
		if err != nil {
			l.logger.Crit("Error publishing event to internal event bus. Should never happen!", "err", err)
		}
	}

	return nil
}

func getString(mm map[string]interface{}, name string) (ret string) {
	byt, ok := mm[name].([]byte)
	if ok {
		ret = string(byt)
	}
	return
}

func getInt64(mm map[string]interface{}, name string) (ret int64) {
	byt, ok := mm[name]
	if ok {
		ret = byt.(int64)
	}
	return
}
