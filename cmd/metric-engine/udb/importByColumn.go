package udb

import (
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

type importByColumn struct {
	*dataSource
}

func newImportByColumn(logger log.Logger, db *sqlx.DB, d busComponents, cfg *viper.Viper, n string) (DataImporter, error) {
	source, err := commonMakeNewSource(logger, db, d, cfg, n)
	if err != nil {
		return nil, err
	}
	ret := &importByColumn{
		dataSource: source,
	}
	ret.importFn = ret.doImport
	return ret, nil
}

// ImportByColumn will import a database rows where each column is a different metric.
//   Each column that is a metric has to have its column name prefixed with "Metric-"
//   Timestamps are constructed based on the "Timestamp" column
//   Metric Context is constructed based on the "Context" column
//   Metric FQDD is constructed based on the "FQDD" column
//   Metric FriendlyFQDD is constructed based on the "FriendlyFQDD" column
//   Property paths are constructed by appending '#<metricname>' to the "Property" column
func (meta *importByColumn) doImport() (err error) {
	totalEvents := 0
	totalRows := 0
	err = meta.wrappedImportByColumn(&totalEvents, &totalRows)
	observedInterval := time.Since(meta.lastImport)
	if err == nil {
		if totalEvents > 0 {
			fmt.Printf("ByColumn Processed %d rows. Emitted %d events from source='%s' observed interval=%s.\n",
				totalRows, totalEvents, meta.sourceName, observedInterval)
		}
	} else {
		fmt.Printf("ByColumn Processed %d rows. Emitted %d events from source='%s' observed interval=%s. ERROR(%s).\n",
			totalRows, totalEvents, meta.sourceName, observedInterval, err)
	}

	return err
}

func (meta *importByColumn) wrappedImportByColumn(totalEvents *int, totalRows *int) (err error) {
	events := []eh.EventData{}

	rows, err := meta.query.Queryx(meta)
	if err != nil {
		return xerrors.Errorf("query failed for %s: %w", meta.udbImportName, err)
	}
	defer rows.Close()

	for rows.Next() {
		mm := map[string]interface{}{}
		err = rows.MapScan(mm)
		if err != nil {
			meta.logger.Crit("Error scanning row into MetricEvent", "err", err, "udbImportName", meta.udbImportName)
			continue
		}
		*totalRows++

		property := getString(mm, "Property")
		ts := getInt64(mm, "Timestamp")
		condSetHWMForSource(meta.dataSource, ts)

		baseEvent := metric.MetricValueEventData{
			Timestamp:        metric.SQLTimeInt{Time: time.Unix(0, ts)},
			Context:          getString(mm, "Context"),
			FQDD:             getString(mm, "FQDD"),
			FriendlyFQDD:     getString(mm, "FriendlyFQDD"),
			Source:           meta.sourceName,
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
			eventToSend.Name = metricName

			if mts, ok := mm["Timestamp-"+metricName].(int64); ok {
				condSetHWMForSource(meta.dataSource, mts)
				eventToSend.Timestamp = metric.SQLTimeInt{Time: time.Unix(0, mts)}
			}

			*totalEvents++
			events = append(events, &eventToSend)
			if len(events) > maximport {
				meta.sendFn(&events)
			}
		}
	}
	meta.sendFn(&events)

	return nil
}
