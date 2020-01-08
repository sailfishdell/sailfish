package telemetry

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"

	. "github.com/superchalupa/sailfish/cmd/metric-engine/metric"
	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/am3"
)

const (
	AddMetricReportDefinition    eh.EventType = "AddMetricReportDefinitionEvent"
	UpdateMetricReportDefinition eh.EventType = "UpdateMetricReportDefinitionEvent"
	DeleteMetricReportDefinition eh.EventType = "DeleteMetricReportDefinitionEvent"
	DatabaseMaintenance          eh.EventType = "DatabaseMaintenanceEvent"
	GenerateMetricReport         eh.EventType = "GenerateMetricReport"
)

type BusComponents interface {
	GetBus() eh.EventBus
}

func RegisterAM3(logger log.Logger, cfg *viper.Viper, am3Svc *am3.Service, d BusComponents) {
	eh.RegisterEventData(AddMetricReportDefinition, func() eh.EventData { return &MetricReportDefinitionData{} })
	eh.RegisterEventData(UpdateMetricReportDefinition, func() eh.EventData { return &MetricReportDefinitionData{} })
	eh.RegisterEventData(DeleteMetricReportDefinition, func() eh.EventData { return &MetricReportDefinitionData{} })
	eh.RegisterEventData(GenerateMetricReport, func() eh.EventData { return &MetricReportDefinitionData{} })

	database, err := sqlx.Open("sqlite3", cfg.GetString("main.databasepath"))
	if err != nil {
		logger.Crit("Could not open database", "err", err)
		return
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
			logger.Crit("Error executing setup SQL", "err", err, "sql", sqltext)
			panic("Cannot set up telemetry timeseries DB! ABORTING: " + err.Error())
		}
	}

	MRDFactory, err := NewMRDFactory(logger, database)
	if err != nil {
		logger.Crit("Error creating report definition factory", "err", err)
	}

	// periodically optimize and vacuum database
	// slightly more than once every 1s. Idea here is to give messages a chance to drive the clock
	const clockPeriod = 1069 * time.Millisecond
	clockTicker := time.NewTicker(clockPeriod)

	go func() {
		// run once after startup. give a few seconds so we dont slow boot up
		time.Sleep(20 * time.Second)
		d.GetBus().PublishEvent(context.Background(), eh.NewEvent(DatabaseMaintenance, "vacuum", time.Now()))
		d.GetBus().PublishEvent(context.Background(), eh.NewEvent(DatabaseMaintenance, "optimize", time.Now()))

		// NOTE: the numbers below are selected as PRIME numbers so that they run concurrently as infrequently as possible
		// With the default 73/3607/10831, they will run concurrently every ~90 years.
		cleanValuesTicker := time.NewTicker(73 * time.Second) // once a minute
		vacuumTicker := time.NewTicker(3607 * time.Second)    // once an hour (-ish)
		optimizeTicker := time.NewTicker(10831 * time.Second) // once every three hours (-ish)

		defer cleanValuesTicker.Stop()
		defer vacuumTicker.Stop()
		defer optimizeTicker.Stop()
		defer clockTicker.Stop()
		for {
			select {
			case <-cleanValuesTicker.C:
				d.GetBus().PublishEvent(context.Background(), eh.NewEvent(DatabaseMaintenance, "clean values", time.Now()))
			case <-vacuumTicker.C:
				d.GetBus().PublishEvent(context.Background(), eh.NewEvent(DatabaseMaintenance, "vacuum", time.Now()))
			case <-optimizeTicker.C:
				d.GetBus().PublishEvent(context.Background(), eh.NewEvent(DatabaseMaintenance, "optimize", time.Now()))
			case <-clockTicker.C:
				d.GetBus().PublishEvent(context.Background(), eh.NewEvent(DatabaseMaintenance, "publish clock", time.Now()))
			}
		}
	}()

	// Create a new Metric Report Definition, or update an existing one
	updateDef := func(event eh.Event) {
		reportDef, ok := event.Data().(*MetricReportDefinitionData)
		if !ok {
			return
		}

		// Can't write to event sent in, so make a local copy
		localReportDefCopy := *reportDef
		err = MRDFactory.UpdateMRD(&localReportDefCopy)
		if err != nil {
			logger.Crit("Failed to create or update the Report Definition", "Name", reportDef.Name, "err", err)
			return
		}
		err := MRDFactory.GenerateMetricReport(&localReportDefCopy)
		if err != nil {
			logger.Crit("Error generating metric report", "err", err, "ReportDefintion", localReportDefCopy)
		}

		// After we've done the adjustments to ReportDefinitionToMetricMeta, there
		// might be orphan rows.  Schedule the database maintenace task to take
		// care of them. This will run *after* we've updated the report and return
		// from this function.
		d.GetBus().PublishEvent(context.Background(), eh.NewEvent(DatabaseMaintenance, "delete orphans", time.Now()))
	}
	am3Svc.AddEventHandler("Create Metric Report Definition", AddMetricReportDefinition, updateDef)
	am3Svc.AddEventHandler("Update Metric Report Definition", UpdateMetricReportDefinition, updateDef)

	am3Svc.AddEventHandler("Delete Metric Report Definition", DeleteMetricReportDefinition, func(event eh.Event) {
		reportDef, ok := event.Data().(*MetricReportDefinitionData)
		if !ok {
			return
		}

		// After we've done the adjustments to ReportDefinitionToMetricMeta, there
		// might be orphan rows.  Schedule the database maintenace task to take
		// care of them. This will run *after* we've updated the report and return
		// from this function.
		d.GetBus().PublishEvent(context.Background(), eh.NewEvent(DatabaseMaintenance, "delete orphans", time.Now()))

		err := MRDFactory.Delete(reportDef)
		if err != nil {
			logger.Crit("Error deleting Metric Report Definition", "Name", reportDef.Name, "err", err)
		}
	})

	lastHWM := time.Time{}
	am3Svc.AddEventHandler("Store Metric Value", MetricValueEvent, func(event eh.Event) {
		metricValue, ok := event.Data().(*MetricValueEventData)
		if !ok {
			return
		}

		err := MRDFactory.InsertMetricValue(metricValue)
		if err != nil {
			logger.Crit("Error Inserting Metric Value", "Metric", metricValue, "err", err)
			return
		}

		delta := MRDFactory.MetricTSHWM.Time.Sub(metricValue.Timestamp.Time)
		//fmt.Printf("Value Event TS: %s  HWM: %s  delta: %s\n", metricValue.Timestamp.Time, MRDFactory.MetricTSHWM.Time, delta)

		if !MRDFactory.MetricTSHWM.IsZero() && (delta > (1*time.Hour) || delta < -(1*time.Hour)) {
			fmt.Printf("Value Event TIME ERROR - We should never get events this far off (delta: %d) current HWM. Metric: %+v\n", delta, metricValue)
		}

		if MRDFactory.MetricTSHWM.Before(metricValue.Timestamp.Time) {
			//fmt.Printf("Set HWM from Value Event TS: %s  HWM: %s  delta: %s\n", metricValue.Timestamp.Time, MRDFactory.MetricTSHWM.Time, delta)
			MRDFactory.MetricTSHWM = metricValue.Timestamp
		}
	})

	am3Svc.AddEventHandler("Generate Metric Report", GenerateMetricReport, func(event eh.Event) {
		reportDef, ok := event.Data().(*MetricReportDefinitionData)
		if !ok {
			return
		}

		// TODO: implement un-suppress

		// Can't write to event sent in, so make a local copy
		localReportDefCopy := *reportDef
		err := MRDFactory.GenerateMetricReport(&localReportDefCopy)
		if err != nil {
			logger.Crit("Error generating metric report", "err", err, "ReportDefintion", localReportDefCopy)
		}
	})

	am3Svc.AddEventHandler("Database Maintenance", DatabaseMaintenance, func(event eh.Event) {
		command, ok := event.Data().(string)
		if !ok {
			return
		}

		switch command {
		case "publish clock":
			// if no events come in during time between clock publishes, we'll artificially bump HWM forward.
			if MRDFactory.MetricTSHWM.Equal(lastHWM) {
				//fmt.Printf("SET HWM FROM TICK: hwm(%s) - last(%s) = %d period(%d)\n", MRDFactory.MetricTSHWM, lastHWM, lastHWM.Sub(MRDFactory.MetricTSHWM.Time), clockPeriod)
				MRDFactory.MetricTSHWM = SqlTimeInt{Time: MRDFactory.MetricTSHWM.Add(clockPeriod)}
			}
			lastHWM = MRDFactory.MetricTSHWM.Time
			MRDFactory.FastCheckForNeededMRUpdates()

		case "optimize":
			MRDFactory.Optimize()

		case "vacuum":
			MRDFactory.Vacuum()

		case "clean values": // keep us under database size limits
			MRDFactory.DeleteOldestValues()

		case "delete orphans": // see factory comment for details.
			MRDFactory.DeleteOrphans()

		case "prune unused metric values":
			// TODO: Delete any MetricValues that are not part of a MetricReport or that wont be part of a report soon (periodic)
			//
			// select
			//   MV.rowid, MV.InstanceID, MV.Timestamp
			// from metricvalue as MV
			//   inner join metricinstance as MI on MV.InstanceID = MI.ID
			//   inner join metricmeta as MM on MI.MetricMetaID = MM.ID
			//   inner join ReportDefinitionToMetricMeta as rd2mm on MM.ID = rd2mm.MetricMetaID
			//   inner join MetricReportDefinition as MRD on rd2mm.ReportDefinitionID = MRD.ID
			//   inner join MetricReport as MR on MRD.ID = MR.ReportDefinitionID
			// WHERE
			//   MV.Timestamp < (insert query to get oldest metric report begin timestamp)
			//
			// TODO: MRDFactory.DeleteOrphans()

		default:
			logger.Warn("Unknown database maintenance command string received", "command", command)
		}
	})
}
