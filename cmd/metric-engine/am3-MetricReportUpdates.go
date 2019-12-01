// +build idrac

package main

import (
	"context"
	"fmt"
	"time"

	eh "github.com/looplab/eventhorizon"

	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/am3"
)

const (
	UpdateMetricReportDefinition eh.EventType = "UpdateMetricReportDefinitionEvent"
	DeleteMetricReportDefinition eh.EventType = "DeleteMetricReportDefinitionEvent"
	DatabaseMaintenance          eh.EventType = "DatabaseMaintenanceEvent"
)

func addAM3DatabaseFunctions(logger log.Logger, dbpath string, am3Svc *am3.Service, d *BusComponents) {
	eh.RegisterEventData(UpdateMetricReportDefinition, func() eh.EventData { return &MetricReportDefinitionData{} })
	eh.RegisterEventData(DeleteMetricReportDefinition, func() eh.EventData { return &MetricReportDefinitionData{} })

	database, err := createDatabase(logger, dbpath)
	if err != nil {
		logger.Crit("FATAL: Could not create database", "err", err)
		return
	}

	MRDFactory, err := NewMRDFactory(logger, database)
	if err != nil {
		logger.Crit("Error creating report definition factory", "err", err)
	}

	// periodically optimize and vacuum database
	go func() {
		time.Sleep(20 * time.Second)
		d.GetBus().PublishEvent(context.Background(), eh.NewEvent(DatabaseMaintenance, "vacuum", time.Now()))
		d.GetBus().PublishEvent(context.Background(), eh.NewEvent(DatabaseMaintenance, "optimize", time.Now()))

		// NOTE: the numbers below are selected as PRIME numbers so that they run at the same time as infrequently as possible
		// With the default 181/73, they will run concurrently every ~9 days
		vacuumTicker := time.NewTicker(time.Duration(73) * time.Minute)
		optimizeTicker := time.NewTicker(time.Duration(181) * time.Minute)
		defer vacuumTicker.Stop()
		defer optimizeTicker.Stop()
		for {
			select {
			case <-vacuumTicker.C:
				d.GetBus().PublishEvent(context.Background(), eh.NewEvent(DatabaseMaintenance, "vacuum", time.Now()))
			case <-optimizeTicker.C:
				d.GetBus().PublishEvent(context.Background(), eh.NewEvent(DatabaseMaintenance, "optimize", time.Now()))
			}
		}
	}()

	am3Svc.AddEventHandler("Create/Update Metric Report Definition", UpdateMetricReportDefinition, func(event eh.Event) {
		reportDef, ok := event.Data().(*MetricReportDefinitionData)
		if !ok {
			logger.Crit("Expected a *MetricReportDefinitionData but didn't get one.", "Actual Type", fmt.Sprintf("%T", event.Data()), "Actual Data", event.Data())
			return
		}

		// create one or update
		_, err = MRDFactory.Update(reportDef)
		if err != nil {
			logger.Crit("Failed to create or update the Report Definition", "Name", reportDef.Name, "err", err)
			return
		}
	})

	am3Svc.AddEventHandler("Delete Metric Report Definition", DeleteMetricReportDefinition, func(event eh.Event) {
		reportDef, ok := event.Data().(*MetricReportDefinitionData)
		if !ok {
			logger.Crit("Expected a *MetricReportDefinitionData but didn't get one.", "Actual Type", fmt.Sprintf("%T", event.Data()), "Actual Data", event.Data())
			return
		}

		err := MRDFactory.Delete(reportDef)
		if err != nil {
			logger.Crit("Error deleting Metric Report Definition", "Name", reportDef.Name, "err", err)
		}
	})

	am3Svc.AddEventHandler("Store Metric Value", MetricValueEvent, func(event eh.Event) {
		metricValue, ok := event.Data().(*MetricValueEventData)
		if !ok {
			logger.Warn("Expected a *MetricValueEventData but didn't get one.", "Actual Type", fmt.Sprintf("%T", event.Data()), "Actual Data", event.Data())
			return
		}

		err := MRDFactory.InsertMetricValue(metricValue)
		if err != nil {
			logger.Crit("Error inserting Metric Value", "err", err, "metric", metricValue)
		}
	})

	am3Svc.AddEventHandler("Database Maintenance", DatabaseMaintenance, func(event eh.Event) {
		command, ok := event.Data().(string)
		if !ok {
			logger.Warn("Expected a command string.", "Actual Type", fmt.Sprintf("%T", event.Data()), "Actual Data", event.Data())
			return
		}

		switch command {
		case "optimize":
			MRDFactory.Optimize()
		case "vacuum":
			MRDFactory.Vacuum()
		default:
			logger.Warn("Unknown database maintenance command string received", "command", command)
		}
	})
}
