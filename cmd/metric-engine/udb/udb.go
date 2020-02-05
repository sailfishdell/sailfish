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
	HWM          int64             `db:"HWM"`
	lastImport   metric.SQLTimeInt `db:"LastImportTime"`
	waitInterval time.Duration
	scanInterval time.Duration
	dbChange     map[string]map[string]struct{}
}

// dataSourceConfig mirrors exactly what we expect in the config file yaml.
// we'll unmarshal to this, then validate/parse into a dataSource which has
// more semantic meaning.
type dataSourceConfig struct {
	Type         string
	Query        string
	WaitInterval int64
	ScanInterval int64
	DBChange     map[string]map[string]struct{}
}

type dataImporter struct {
	logger   log.Logger
	database *sqlx.DB
	udb      map[string]dataSource
	bus      eh.EventBus
}

func newImportManager(logger log.Logger, database *sqlx.DB, d busComponents, cfg *viper.Viper) (*dataImporter, error) {
	ret := &dataImporter{
		logger:   logger,
		database: database,
		bus:      d.GetBus(),
		udb:      map[string]dataSource{},
	}

	impFns := map[string]func(string) error{
		"DISABLE-DirectMetric": func(n string) error { return disabled },
		"DirectMetric":         ret.importByMetricValue,
		"MetricColumns":        ret.importByColumn,
		"Error":                ret.importERROR,
	}

	// Parse the YAML file to set up database imports
	subcfg := cfg.Sub("UDB-Metric-Import")
	if subcfg == nil {
		return nil, xerrors.Errorf("config file parse error. missing secion 'UDB-Metric-Import'")
	}

	for k := range subcfg.AllSettings() {
		fmt.Printf("Loading config for %s\n", k)
		settings, err := unmarshalSourceCfg(subcfg.Sub(k))
		if err != nil {
			return nil, xerrors.Errorf("failed to parse config section(UDB-Metric-Import.%s): %s", k, err)
		}

		meta := dataSource{
			scanInterval: time.Duration(settings.ScanInterval) * time.Second,
			waitInterval: time.Duration(settings.WaitInterval) * time.Second,
			dbChange:     settings.DBChange,
		}

		//fmt.Printf("DEBUG: dbchange: %+v\n", meta.dbChange)

		meta.query, err = database.PrepareNamed(settings.Query)
		if err != nil {
			return nil, xerrors.Errorf("prepare() failed for query Section(%s), key(%s), sql query(%s): %w", "UDB-Metric-Import", k, settings.Query, err)
		}

		var ok bool
		if meta.importFn, ok = impFns[settings.Type]; !ok {
			return nil, fmt.Errorf(
				"config error, nonexistent func. Section(%s), key(%s), func(%s). Available: %+v", "UDB-Metric-Import", k, settings.Type, impFns)
		}

		ret.udb[k] = meta
	}

	return ret, nil
}

func unmarshalSourceCfg(cfg *viper.Viper) (*dataSourceConfig, error) {
	cfg.SetDefault("Type", "MetricColumns")
	cfg.SetDefault("WaitInterval", "5")
	cfg.SetDefault("ScanInterval", "0")
	settings := dataSourceConfig{}
	err := cfg.Unmarshal(&settings)
	if err != nil {
		return nil, err
	}

	// have to specifically unmarshal dbchange for some wierd reason
	err = cfg.UnmarshalKey("dbchange", &settings.DBChange)
	if err != nil {
		return nil, err
	}
	return &settings, err
}

func (l *dataImporter) conditionalImport(udbImportName string, meta dataSource, periodic bool) (err error) {
	now := time.Now()
	if periodic && meta.scanInterval == 0 {
		return
	}
	if periodic && now.Before(meta.lastImport.Add(meta.scanInterval)) {
		return
	}
	if now.Before(meta.lastImport.Add(meta.waitInterval)) {
		return
	}

	meta.lastImport.Time = now
	l.udb[udbImportName] = meta
	return l.udb[udbImportName].importFn(udbImportName)
}

func (l *dataImporter) runImport(periodic bool) error {
	// TODO: get smarter about this. We ought to calculate time until next report and set a timer for that
	return l.iterUDBSources(func(name string, src dataSource) error {
		err := l.conditionalImport(name, src, periodic)
		if err != nil && !xerrors.Is(err, disabled) {
			return xerrors.Errorf("error from import of report(%s): %w", name, err)
		}
		return nil
	})
}

func (l *dataImporter) runImportForUDBChange(database, table string) (err error) {
	return l.iterUDBSources(func(udbImportName string, meta dataSource) error {
		//fmt.Printf("UDB CHANGE: %s/%s", udbImportName, database)
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
	//fmt.Printf("DONT YET KNOW HOW TO IMPORT: %s\n", udbImportName)
	return xerrors.New("UNKNOWN IMPORT")
}

func (l *dataImporter) iterUDBSources(fn func(string, dataSource) error) error {
	for udbImportName, meta := range l.udb {
		err := fn(udbImportName, meta)
		if err != nil && err != disabled && err != stopIter {
			return xerrors.Errorf("Stopped iteration due to iteration function returning error: %w", err)
		}
	}
	return nil
}

func (l *dataImporter) condSetHWMForSource(udbImportName string, ts int64) {
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
	totalEvents := 0
	totalRows := 0
	err = l.wrappedImportByColumn(udbImportName, &totalEvents, &totalRows)
	if err == nil {
		if totalEvents > 0 {
			fmt.Printf("Processed %d rows. Emitted %d events from source='%s'.\n", totalRows, totalEvents, udbImportName)
		}
	} else {
		fmt.Printf("Processed %d rows. Emitted %d events from source='%s'. ERROR(%s).\n", totalRows, totalEvents, udbImportName, err)
	}

	return err
}

func (l *dataImporter) wrappedImportByColumn(udbImportName string, totalEvents *int, totalRows *int) (err error) {
	events := []eh.EventData{}
	dataSource := l.udb[udbImportName] // will panic if we get called with invalid name, that would be a programming error

	rows, err := dataSource.query.Queryx(dataSource) // panic if query not prepared, can't happen
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
		*totalRows++

		property := getString(mm, "Property")
		ts := getInt64(mm, "Timestamp")
		l.condSetHWMForSource(udbImportName, ts)

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

		for k, v := range mm {
			if v == nil || !strings.HasPrefix(k, "Metric-") {
				continue // we dont add NULL metrics or things without Metric- prefix
			}

			metricName := k[len("Metric-"):]
			eventToSend := baseEvent
			eventToSend.Property = property + "#" + metricName
			eventToSend.Value = fmt.Sprintf("%v", v)

			if mts, ok := mm["Timestamp-"+metricName].(int64); ok {
				l.condSetHWMForSource(udbImportName, mts)
				eventToSend.Timestamp = metric.SQLTimeInt{Time: time.Unix(0, mts)}
			}

			*totalEvents++
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
