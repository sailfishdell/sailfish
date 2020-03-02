package udb

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"
	"golang.org/x/xerrors"

	"github.com/superchalupa/sailfish/cmd/metric-engine/metric"
	log "github.com/superchalupa/sailfish/src/log"
)

type importDirect struct {
	*dataSource
}

func newDirectMetric(logger log.Logger, db *sqlx.DB, d busComponents, cfg *viper.Viper, n string) (DataImporter, error) {
	source, err := commonMakeNewSource(logger, db, d, cfg, n)
	if err != nil {
		return nil, err
	}
	ret := &importDirect{
		dataSource: source,
	}
	ret.importFn = ret.doImport
	return ret, nil
}

func (meta *importDirect) doImport() (err error) {
	err = nil
	events := []eh.EventData{}
	totalRows := 0
	totalEvents := 0
	defer func() {
		observedInterval := time.Since(meta.lastImport)
		if err == nil {
			if totalRows > 0 {
				fmt.Printf("DirectMetric Processed %d rows. Emitted %d events from source='%s' observed interval=%s.\n",
					totalRows, totalEvents, meta.sourceName, observedInterval)
			}
		} else {
			fmt.Printf("DirectMetric Processed %d rows. Emitted %d events from source='%s' observed interval=%s. ERROR(%s).\n",
				totalRows, totalEvents, meta.sourceName, observedInterval, err)
		}
	}()

	// we'll panic here if query isn't prepared or if there was a syntax error in config file
	rows, err := meta.query.Queryx(meta)
	if err != nil {
		return xerrors.Errorf("query failed for %s: %w", meta.udbImportName, err)
	}
	defer rows.Close()

	for rows.Next() {
		totalRows++
		event := &metric.MetricValueEventData{
			Source: meta.sourceName,
		}
		err = rows.StructScan(event)
		if err != nil {
			meta.logger.Crit("Error scanning row into MetricEvent", "err", err, "udbImportName", meta.udbImportName)
			continue
		}

		if ts := event.Timestamp.UnixNano(); ts > meta.HWM {
			meta.HWM = ts
		}

		totalEvents++
		events = append(events, event)
		if len(events) > maximport {
			meta.sendFn(&events)
		}
	}
	meta.sendFn(&events)

	return nil
}
