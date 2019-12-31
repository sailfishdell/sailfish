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

	. "github.com/superchalupa/sailfish/cmd/metric-engine/metric"
	log "github.com/superchalupa/sailfish/src/log"
)

type UDBMeta struct {
	query        *sqlx.NamedStmt
	importFn     func(string) error
	HWM          int64 `db:"HWM"`
	lastImport   time.Time
	waitInterval int64
	scanInterval int64
	dbChange     map[string]map[string]struct{}
}

type UDBFactory struct {
	logger   log.Logger
	database *sqlx.DB
	udb      map[string]UDBMeta
	bus      eh.EventBus
}

func NewUDBFactory(logger log.Logger, database *sqlx.DB, d BusComponents, cfg *viper.Viper) (*UDBFactory, error) {
	ret := &UDBFactory{}
	ret.logger = logger
	ret.database = database
	ret.bus = d.GetBus()
	ret.udb = map[string]UDBMeta{}

	impFns := map[string]func(string) error{
		"DirectMetric":  func(n string) error { return ret.ImportByMetricValue(n) },
		"MetricColumns": func(n string) error { return ret.ImportByColumn(n) },
		"Error":         func(n string) error { return ret.ImportERROR(n) },
	}

	// Parse the YAML file to set up database imports
	for k, v := range cfg.GetStringMap("UDB-Metric-Import") {
		fmt.Printf("Loading config for %s\n", k)

		meta := UDBMeta{}

		settings, ok := v.(map[string]interface{})
		if !ok {
			logger.Crit("SKIPPING: Config file section settings malformed.", "Section", "UDB-Metric-Import", "key", k, "malformed-value", v)
			continue
		}

		Query, ok := settings["query"].(string)
		if !ok {
			logger.Crit("SKIPPING: Config file section missing SQL QUERY - 'query' key missing.", "Section", "UDB-Metric-Import", "key", k, "malformed-value", v)
			continue
		}

		var err error
		meta.query, err = database.PrepareNamed(Query)
		if err != nil {
			logger.Crit("SKIPPING: SQL Query PREPARE failed", "Section", "UDB-Metric-Import", "key", k, "QUERY", Query, "err", err)
			continue
		}

		Type, ok := settings["type"].(string)
		if !ok {
			Type = "MetricColumns" // default
		}

		_, ok = impFns[Type]
		if !ok {
			logger.Crit("SKIPPING: No such Type", "Section", "UDB-Metric-Import", "key", k, "Type", Type)
			continue
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
			fmt.Printf("parsing error for dbchange: %s\n", err)
		}

		meta.importFn = impFns[Type]
		ret.udb[k] = meta
	}

	return ret, nil
}

func (l *UDBFactory) ConditionalImport(udbImportName string, meta UDBMeta, periodic bool) (err error) {
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

func (l *UDBFactory) DBChanged(database, table string) (err error) {
	return l.IterUDBTables(func(udbImportName string, meta UDBMeta) error {
		tbls, ok := meta.dbChange[database]
		if !ok {
			return nil
		}
		_, ok = tbls[table]
		if !ok {
			return nil
		}
		return l.ConditionalImport(udbImportName, meta, false)
	})
}

func (l *UDBFactory) ImportERROR(udbImportName string) (err error) {
	fmt.Printf("DONT YET KNOW HOW TO IMPORT: %s\n", udbImportName)
	return xerrors.New("UNKNOWN IMPORT")
}

func (l *UDBFactory) IterUDBTables(fn func(string, UDBMeta) error) error {
	for udbImportName, meta := range l.udb {
		err := fn(udbImportName, meta)
		if err != nil {
			return xerrors.Errorf("Stopped iteration due to iteration function returning error: %w", err)
		}
	}
	return nil
}

const maximport = 50

// ImportByColumn will import a database rows where each column is a different metric.
//   Each column that is a metric has to have its column name prefixed with "Metric-"
//   Timestamps are constructed based on the "Timestamp" column
//   Metric Context is constructed based on the "Context" column
//   Metric FQDD is constructed based on the "FQDD" column
//   Metric FriendlyFQDD is constructed based on the "FriendlyFQDD" column
//   Property paths are constructed by appending '#<metricname>' to the "Property" column
func (l *UDBFactory) ImportByColumn(udbImportName string) (err error) {
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
		err = xerrors.Errorf("SQL query wasn't prepared for %s!", udbImportName)
		return
	}

	rows, err := udbMeta.query.Queryx(udbMeta)
	if err != nil {
		err = xerrors.Errorf("query failed for %s: %w", udbImportName, err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		mm := map[string]interface{}{}
		err = rows.MapScan(mm)
		if err != nil {
			l.logger.Crit("Error scanning row into MetricEvent", "err", err, "udbImportName", udbImportName)
			continue
		}

		ts, ok := mm["Timestamp"].(int64)
		delete(mm, "Timestamp")

		if ts > udbMeta.HWM {
			udbMeta.HWM = ts
			l.udb[udbImportName] = udbMeta
		}

		var metricCtx string
		metricCtxB, ok := mm["Context"].([]byte)
		if ok {
			metricCtx = string(metricCtxB)
		}
		delete(mm, "Context")

		var property string
		propertyB, ok := mm["Property"].([]byte)
		if ok {
			property = string(propertyB)
		}
		delete(mm, "Property")

		var fqdd string
		fqddB, ok := mm["FQDD"].([]byte)
		if ok {
			fqdd = string(fqddB)
		}
		delete(mm, "FQDD")

		var friendlyfqdd string
		friendlyfqddB, ok := mm["FriendlyFQDD"].([]byte)
		if ok {
			friendlyfqdd = string(friendlyfqddB)
		}
		delete(mm, "FriendlyFQDD")

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
			event := &MetricValueEventData{
				Timestamp:    SqlTimeInt{time.Unix(0, ts)},
				Context:      metricCtx,
				FQDD:         fqdd,
				FriendlyFQDD: friendlyfqdd,
				Name:         metricName,
				Value:        fmt.Sprintf("%v", v),
				Property:     property + "#" + metricName,
			}

			if mts, ok := mm["Timestamp-"+metricName].(int64); ok {
				event.Timestamp = SqlTimeInt{time.Unix(0, mts)}
				if mts > udbMeta.HWM {
					udbMeta.HWM = mts
					l.udb[udbImportName] = udbMeta
				}

			}

			events = append(events, event)
			if len(events) > maximport {
				l.bus.PublishEvent(context.Background(), eh.NewEvent(MetricValueEvent, events, time.Now()))
				events = []eh.EventData{}
			}
		}
	}
	if len(events) > 0 {
		l.bus.PublishEvent(context.Background(), eh.NewEvent(MetricValueEvent, events, time.Now()))
	}

	return nil
}

func (l *UDBFactory) ImportByMetricValue(udbImportName string) (err error) {
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
		err = xerrors.Errorf("SQL query wasn't prepared for %s!", udbImportName)
		return
	}

	rows, err := udbMeta.query.Queryx(udbMeta)
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

		if ts := event.Timestamp.UnixNano(); ts > udbMeta.HWM {
			udbMeta.HWM = ts
			l.udb[udbImportName] = udbMeta
		}

		events = append(events, event)
		if len(events) > maximport {
			l.bus.PublishEvent(context.Background(), eh.NewEvent(MetricValueEvent, events, time.Now()))
			events = []eh.EventData{}
		}
	}
	if len(events) > 0 {
		l.bus.PublishEvent(context.Background(), eh.NewEvent(MetricValueEvent, events, time.Now()))
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
