package telemetry

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
	"github.com/superchalupa/sailfish/src/ocp/am3"
)

// constants to refer to event types
const (
	AddMetricReportDefinition    eh.EventType = "AddMetricReportDefinitionEvent"
	UpdateMetricReportDefinition eh.EventType = "UpdateMetricReportDefinitionEvent"
	DeleteMetricReportDefinition eh.EventType = "DeleteMetricReportDefinitionEvent"
	DatabaseMaintenance          eh.EventType = "DatabaseMaintenanceEvent"
)

// "configuration" -- TODO: need to move to config file
const (
	clockPeriod             = 1000 * time.Millisecond
	startupWaitTime         = 1 * time.Second
	cleanMVTime             = 73 * time.Second
	vacuumTime              = 3607 * time.Second
	optimizeTime            = 10831 * time.Second
	maxMetricTimestampDelta = 1 * time.Hour
)

type busComponents interface {
	GetBus() eh.EventBus
}

// StartupTelemetryBase registers event handlers with the awesome mapper and
// starts up timers and maintenance goroutines
func StartupTelemetryBase(logger log.Logger, cfg *viper.Viper, am3Svc *am3.Service, d busComponents) error {
	eh.RegisterEventData(AddMetricReportDefinition, func() eh.EventData { return &MetricReportDefinitionData{} })
	eh.RegisterEventData(UpdateMetricReportDefinition, func() eh.EventData { return &MetricReportDefinitionData{} })
	eh.RegisterEventData(DeleteMetricReportDefinition, func() eh.EventData { return &MetricReportDefinitionData{} })
	eh.RegisterEventData(DatabaseMaintenance, func() eh.EventData { return "" })
	metric.RegisterEvent()

	database, err := sqlx.Open("sqlite3", cfg.GetString("main.databasepath"))
	if err != nil {
		return xerrors.Errorf("could not open database(%s): %w", cfg.GetString("main.databasepath"))
	}

	// run sqlite with only one connection to avoid locking issues
	// If we run in WAL mode, you can only do one connection. Seems like a base
	// library limitation that's reflected up into the golang implementation.
	// SO: we will ensure that we have ONLY ONE GOROUTINE that does transactions
	// This isn't a terrible limitation as it is sort of what we want to do
	// anyways.
	database.SetMaxOpenConns(1)

	// Create tables and views from sql stored in our YAML
	for _, sqltext := range cfg.GetStringSlice("createdb") {
		_, err = database.Exec(sqltext)
		if err != nil {
			// ignore drop errors if we are migrating from old telemetry db
			if strings.HasPrefix(err.Error(), "use DROP VIEW") {
				logger.Info("Ignoring SQL error dropping view", "err", err, "sql", sqltext)
				continue
			}
			if strings.HasPrefix(err.Error(), "use DROP TABLE") {
				logger.Info("Ignoring SQL error dropping table", "err", err, "sql", sqltext)
				continue
			}
			logger.Crit("Error executing setup SQL", "err", err, "sql", sqltext)
			panic("Cannot set up telemetry timeseries DB! ABORTING: " + err.Error())
		}
	}

	telemetryMgr, err := newTelemetryManager(logger, database, cfg)
	if err != nil {
		return xerrors.Errorf("telemetry manager initialization failed: %w", err)
	}

	go func() {
		// run once after startup. give a few seconds so we dont slow boot up
		time.Sleep(startupWaitTime)
		err := d.GetBus().PublishEvent(context.Background(), eh.NewEvent(DatabaseMaintenance, "vacuum", time.Now()))
		if err != nil {
			logger.Crit("Error publishing event", "err", err)
		}
		err = d.GetBus().PublishEvent(context.Background(), eh.NewEvent(DatabaseMaintenance, "optimize", time.Now()))
		if err != nil {
			logger.Crit("Error publishing event", "err", err)
		}

		// NOTE: the numbers below are selected as PRIME numbers so that they run concurrently as infrequently as possible
		// With the default 73/3607/10831, they will run concurrently every ~90 years.
		clockTicker := time.NewTicker(clockPeriod)       // unfortunately every second, otherwise report generation drifts.
		cleanValuesTicker := time.NewTicker(cleanMVTime) // once a minute
		vacuumTicker := time.NewTicker(vacuumTime)       // once an hour (-ish)
		optimizeTicker := time.NewTicker(optimizeTime)   // once every three hours (-ish)

		defer cleanValuesTicker.Stop()
		defer vacuumTicker.Stop()
		defer optimizeTicker.Stop()
		defer clockTicker.Stop()
		for {
			var err error
			select {
			case <-cleanValuesTicker.C:
				err = d.GetBus().PublishEvent(context.Background(), eh.NewEvent(DatabaseMaintenance, "clean values", time.Now()))
			case <-vacuumTicker.C:
				err = d.GetBus().PublishEvent(context.Background(), eh.NewEvent(DatabaseMaintenance, "vacuum", time.Now()))
			case <-optimizeTicker.C:
				err = d.GetBus().PublishEvent(context.Background(), eh.NewEvent(DatabaseMaintenance, "optimize", time.Now()))
				if err != nil {
					logger.Crit("Error publishing event", "err", err)
				}
				// should never happen, but run occasionally just in case
				err = d.GetBus().PublishEvent(context.Background(), eh.NewEvent(DatabaseMaintenance, "delete orphans", time.Now()))
			case <-clockTicker.C:
				err = d.GetBus().PublishEvent(context.Background(), eh.NewEvent(DatabaseMaintenance, "publish clock", time.Now()))
			}
			if err != nil {
				logger.Crit("Error publishing event", "err", err)
			}
		}
	}()

	am3Svc.AddEventHandler("Create Metric Report Definition", AddMetricReportDefinition, func(event eh.Event) {
		reportDef, ok := event.Data().(*MetricReportDefinitionData)
		if !ok {
			return
		}

		// Can't write to event sent in, so make a local copy
		localReportDefCopy := *reportDef
		err = telemetryMgr.addMRD(&localReportDefCopy)
		if err != nil {
			logger.Crit("Failed to create or update the Report Definition", "Name", reportDef.Name, "err", err)
			return
		}

		// After we've done the adjustments to ReportDefinitionToMetricMeta, there
		// might be orphan rows.
		err = telemetryMgr.DeleteOrphans()
		if err != nil {
			logger.Crit("Orphan delete failed", "err", err)
		}
	})

	am3Svc.AddEventHandler("Update Metric Report Definition", UpdateMetricReportDefinition, func(event eh.Event) {
		reportDef, ok := event.Data().(*MetricReportDefinitionData)
		if !ok {
			return
		}

		// Can't write to event sent in, so make a local copy
		localReportDefCopy := *reportDef
		err = telemetryMgr.updateMRD(&localReportDefCopy)
		if err != nil {
			logger.Crit("Failed to create or update the Report Definition", "Name", reportDef.Name, "err", err)
			return
		}

		// After we've done the adjustments to ReportDefinitionToMetricMeta, there
		// might be orphan rows.
		err = telemetryMgr.DeleteOrphans()
		if err != nil {
			logger.Crit("Orphan delete failed", "err", err)
		}
	})

	am3Svc.AddEventHandler("Delete Metric Report Definition", DeleteMetricReportDefinition, func(event eh.Event) {
		reportDef, ok := event.Data().(*MetricReportDefinitionData)
		if !ok {
			return
		}

		err := telemetryMgr.deleteMRD(reportDef)
		if err != nil {
			logger.Crit("Error deleting Metric Report Definition", "Name", reportDef.Name, "err", err)
		}
		err = telemetryMgr.DeleteOrphans()
		if err != nil {
			logger.Crit("Orphan delete failed", "err", err)
		}
	})

	lastHWM := time.Time{}
	am3Svc.AddMultiHandler("Store Metric Value(s)", metric.MetricValueEvent, func(event eh.Event) {
		// This is a MULTI Handler! This function is called with an ARRAY of event
		// data, not the normal single event data.  This means we can wrap the
		// insert in a transaction and insert everything in the array in a single
		// transaction for a good performance boost.
		instancesUpdated := map[int64]struct{}{}
		err = telemetryMgr.wrapWithTX(func(tx *sqlx.Tx) error {
			dataArray, ok := event.Data().([]eh.EventData)
			if !ok {
				return nil
			}
			for _, eventData := range dataArray {
				metricValue, ok := eventData.(*metric.MetricValueEventData)
				if !ok {
					continue
				}

				err := telemetryMgr.InsertMetricValue(tx, metricValue, func(instanceid int64) { instancesUpdated[instanceid] = struct{}{} })
				if err != nil {
					logger.Crit("Error Inserting Metric Value", "Metric", metricValue, "err", err)
					continue
				}

				delta := telemetryMgr.MetricTSHWM.Sub(metricValue.Timestamp.Time)

				if (!telemetryMgr.MetricTSHWM.IsZero()) && (delta > maxMetricTimestampDelta || delta < -maxMetricTimestampDelta) {
					// if you see this warning consistently, check the import to ensure it's using UTC and not localtime
					fmt.Printf("Warning: Metric Value Event TIME OFF >1hr - (delta: %s)  Metric: %+v\n", delta, metricValue)
				}

				if telemetryMgr.MetricTSHWM.Before(metricValue.Timestamp.Time) {
					telemetryMgr.MetricTSHWM = metricValue.Timestamp.Time
				}
			}
			return nil
		})
		if err != nil {
			logger.Crit("critical error storing metric value", "err", err)
		}

		// this will set telemetryMgr.NextMRTS = telemetryMgr.LastMRTS+5s for any reports that have changes
		err := telemetryMgr.CheckOnChangeReports(nil, instancesUpdated)
		if err != nil {
			logger.Crit("Error Finding OnChange Reports for metrics", "instancesUpdated", instancesUpdated, "err", err)
		}
	})

	am3Svc.AddEventHandler("Request Generation of a Metric Report", metric.RequestReport, func(event eh.Event) {
		report, ok := event.Data().(*metric.RequestReportData)
		if !ok {
			return
		}

		// input event is a pointer to shared data struct, dont directly use, make a copy
		name := report.Name
		err := telemetryMgr.GenerateMetricReport(nil, name)
		if err != nil {
			logger.Crit("Error generating metric report", "err", err, "ReportDefintion", name)
		}
		err = d.GetBus().PublishEvent(context.Background(), eh.NewEvent(metric.ReportGenerated, metric.ReportGeneratedData{Name: name}, time.Now()))
		if err != nil {
			logger.Crit("Error publishing event", "err", err)
		}
	})

	am3Svc.AddEventHandler("Database Maintenance", DatabaseMaintenance, func(event eh.Event) {
		command, ok := event.Data().(string)
		if !ok {
			return
		}

		switch command {
		case "resync to db":
			// First, check existing expiry... ensure we dont drop any OnChange (NextMRTS==-1) reports that might have been marked
			reportList, _ := telemetryMgr.FastCheckForNeededMRUpdates()
			for _, report := range reportList {
				err := d.GetBus().PublishEvent(context.Background(), eh.NewEvent(metric.ReportGenerated, metric.ReportGeneratedData{Name: report}, time.Now()))
				if err != nil {
					logger.Crit("Error publishing event", "err", err)
				}
			}

			// next, go through database and delete any NextMRTS that arent present and reload
			reportList, _ = telemetryMgr.syncNextMRTSWithDB()
			for _, report := range reportList {
				err := d.GetBus().PublishEvent(context.Background(), eh.NewEvent(metric.ReportGenerated, metric.ReportGeneratedData{Name: report}, time.Now()))
				if err != nil {
					logger.Crit("Error publishing event", "err", err)
				}
			}

		case "publish clock":
			// if no events have kickstarted the clock, bail
			if telemetryMgr.MetricTSHWM.IsZero() {
				break
			}

			// if no events come in during time between clock publishes, we'll artificially bump HWM forward.
			// if time is uninitialized, wait for an event to come in to set it
			if telemetryMgr.MetricTSHWM.Equal(lastHWM) {
				telemetryMgr.MetricTSHWM = telemetryMgr.MetricTSHWM.Add(clockPeriod)
			}
			lastHWM = telemetryMgr.MetricTSHWM

			// Generate any metric reports that need it
			reportList, _ := telemetryMgr.FastCheckForNeededMRUpdates()
			for _, report := range reportList {
				err := d.GetBus().PublishEvent(context.Background(), eh.NewEvent(metric.ReportGenerated, metric.ReportGeneratedData{Name: report}, time.Now()))
				if err != nil {
					logger.Crit("Error publishing event", "err", err)
				}
			}

		case "optimize":
			fmt.Printf("Running scheduled database optimization\n")
			err = telemetryMgr.Optimize()
			if err != nil {
				logger.Crit("Optimize failed", "err", err)
			}

		case "vacuum":
			fmt.Printf("Running scheduled database storage recovery\n")
			err = telemetryMgr.Vacuum()
			if err != nil {
				logger.Crit("Vacuum failed", "err", err)
			}

		case "clean values": // keep us under database size limits
			fmt.Printf("Running scheduled cleanup of the stored Metric Values\n")
			err = telemetryMgr.DeleteOldestValues()
			if err != nil {
				logger.Crit("DeleteOldestValues failed.", "err", err)
			}

		case "delete orphans": // see factory comment for details.
			fmt.Printf("Running scheduled database consistency cleanup\n")
			err = telemetryMgr.DeleteOrphans()
			if err != nil {
				logger.Crit("Orphan delete failed", "err", err)
			}

		case "prune unused metric values":
			fmt.Printf("Running scheduled cleanup of the stored Metric Values\n")
			err = telemetryMgr.DeleteOldestValues()
			if err != nil {
				logger.Crit("DeleteOldestValues failed.", "err", err)
			}
			fmt.Printf("Running scheduled database consistency cleanup\n")
			err = telemetryMgr.DeleteOrphans()
			if err != nil {
				logger.Crit("Orphan delete failed", "err", err)
			}

		default:
			logger.Warn("Unknown database maintenance command string received", "command", command)
		}
	})

	return nil
}
