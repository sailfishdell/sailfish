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
	query    *sqlx.Stmt
	importFn func(string, ...string) error
	interval int64
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

	impFns := map[string]func(string, ...string) error{
		"DirectMetric":  func(n string, args ...string) error { return ret.ImportByMetricValue(n, args...) },
		"MetricColumns": func(n string, args ...string) error { return ret.ImportByColumn(n, args...) },
		"Error":         func(n string, args ...string) error { return ret.ImportERROR(n, args...) },
	}

	for k, v := range cfg.GetStringMap("UDB-Metric-Import") {
		fmt.Printf("Loading config for %s\n", k)
		settings, ok := v.(map[string]interface{})
		if !ok {
			fmt.Printf("\tcouldnt type assert\n")
			continue
		}
		Type, ok := settings["type"].(string)
		if !ok {
			fmt.Printf("\tNo type!\n")
			continue
		}

		Query, ok := settings["query"].(string)
		if !ok {
			fmt.Printf("\tNo Query!\n")
			continue
		}

		Interval, ok := settings["scaninterval"].(int)
		if !ok {
			fmt.Printf("\tNo scaninterval!\n")
			continue
		}

		query, err := database.Preparex(Query)
		if err != nil {
			fmt.Printf("Prepare failed for %s: SQL(%s) %s\n", k, Query, err)
			continue
		}

		_, ok = impFns[Type]
		if !ok {
			Type = "Error"
		}
		ret.udb[k] = UDBMeta{importFn: impFns[Type], query: query, interval: int64(Interval)}
	}

	return ret, nil
}

func (l *UDBFactory) ConditionalImport(udbImportName string) (err error) {
	meta, ok := l.udb[udbImportName]
	if !ok {
		return xerrors.Errorf("UDB table %s not set up in meta struct", udbImportName)
	}

	if meta.interval == 0 || time.Now().Unix()%meta.interval != 0 {
		return
	}

	return l.udb[udbImportName].importFn(udbImportName, []string{}...)
}

func (l *UDBFactory) Import(udbImportName string, args ...string) (err error) {
	meta, ok := l.udb[udbImportName]
	if !ok {
		return xerrors.Errorf("UDB table %s not set up in meta struct", udbImportName)
	}

	return meta.importFn(udbImportName, args...)
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

const maximport = 50

// ImportByColumn will import a database rows where each column is a different metric.
//   Each column that is a metric has to have its column name prefixed with "Metric-"
//   Timestamps are constructed based on the "Timestamp" column
//   Metric Context is constructed based on the "Context" column
//   Property paths are constructed by appending '#<metricname>' to the "Property" column
func (l *UDBFactory) ImportByColumn(udbImportName string, args ...string) (err error) {
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

	rows, err := udbMeta.query.Queryx()
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

		ts := mm["Timestamp"].(int64)
		delete(mm, "Timestamp")
		metricCtx := string(mm["Context"].([]byte))
		delete(mm, "Context")
		property := string(mm["Property"].([]byte))
		delete(mm, "Property")

		for k, v := range mm {
			event := &MetricValueEventData{Timestamp: SqlTimeInt{time.Unix(0, ts)}, Context: metricCtx}
			if !strings.HasPrefix(k, "Metric-") {
				fmt.Printf("Skipping %s\n", k)
				continue
			}
			if v == nil {
				//fmt.Printf("Skipping nil Metric: %s\n", k)
				continue
			}
			event.Name = k[7:]
			event.Value = fmt.Sprintf("%v", v)
			event.Property = property + "#" + k[7:]

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
		err = xerrors.Errorf("SQL query wasn't prepared for %s!", udbImportName)
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
