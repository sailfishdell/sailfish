// +build idrac

package main

import (
	"fmt"

	eh "github.com/looplab/eventhorizon"
	_ "github.com/mattn/go-sqlite3"

	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/am3"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

const (
	UpdateMetricReportDefinition eh.EventType = "UpdateMetricReportDefinitionEvent"
	DeleteMetricReportDefinition eh.EventType = "DeleteMetricReportDefinitionEvent"
)

func addAM3DatabaseFunctions(logger log.Logger, dbpath string, am3Svc *am3.Service, d *domain.DomainObjects) {
	eh.RegisterEventData(UpdateMetricReportDefinition, func() eh.EventData { return &MetricReportDefinitionData{} })

	fmt.Printf("Creating database: %s\n", dbpath)
	database, err := createDatabase(logger, dbpath)
	if err != nil {
		logger.Crit("Could not create database", "err", err)
		return
	}

	MRDFactory, err := NewMRDFactory(logger, database)
	if err != nil {
		logger.Crit("Error creating factory", "err", err)
	}

	am3Svc.AddEventHandler("update_metric_report_definition", UpdateMetricReportDefinition, func(event eh.Event) {
		reportDef, ok := event.Data().(*MetricReportDefinitionData)
		if !ok {
			logger.Crit("Internal program error. Expected a *MetricReportDefinitionData but didn't get one.", "Actual", fmt.Sprintf("%T", event.Data()))
			return
		}

		// create one or update
		_, err = MRDFactory.Update(reportDef)
		if err != nil {
			logger.Crit("Failed to create or update the Report Definition", "Name", reportDef.Name, "err", err)
			return
		}
	})

	am3Svc.AddEventHandler("delete_metric_report_definition", DeleteMetricReportDefinition, func(event eh.Event) {
		reportDef, ok := event.Data().(*MetricReportDefinitionData)
		if !ok {
			logger.Crit("Internal program error. Expected a *MetricReportDefinitionData but didn't get one.", "Actual", fmt.Sprintf("%T", event.Data()))
			return
		}

		err := MRDFactory.Delete(reportDef)
		if err != nil {
			logger.Crit("Error deleting Metric Report Definition", "Name", reportDef.Name, "err", err)
		}
	})

	am3Svc.AddEventHandler("metric_storage", MetricValueEvent, func(event eh.Event) {
		metricValue, ok := event.Data().(*MetricValueEventData)
		if !ok {
			logger.Crit("Internal program error. Expected a *MetricValueEventData but didn't get one.", "Actual", fmt.Sprintf("%T", event.Data()))
			return
		}

		type valueInserter interface {
			InsertMetricValue(*MetricValueEventData)
		}
		MRDFactory.IterReportDefsForMetric(metricValue, func(mrd *MetricReportDefinition) {
			mrd.InsertMetricValue(metricValue)
		})
	})
}
