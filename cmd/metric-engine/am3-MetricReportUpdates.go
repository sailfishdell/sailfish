// +build idrac

package main

import (
	"fmt"
	eh "github.com/looplab/eventhorizon"

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

	am3Svc.AddEventHandler("update_metric_report_definition", UpdateMetricReportDefinition, func(event eh.Event) {
		reportDef, ok := event.Data().(*MetricReportDefinitionData)
		if !ok {
			logger.Crit("Expected a *MetricReportDefinitionData but didn't get one.", "Actual Type", fmt.Sprintf("%T", event.Data()), "Acutal Data", event.Data())
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
			logger.Crit("Expected a *MetricReportDefinitionData but didn't get one.", "Actual Type", fmt.Sprintf("%T", event.Data()), "Acutal Data", event.Data())
			return
		}

		err := MRDFactory.Delete(reportDef)
		if err != nil {
			logger.Crit("Error deleting Metric Report Definition", "Name", reportDef.Name, "err", err)
		}
	})

	am3Svc.AddEventHandler("store_metric_value", MetricValueEvent, func(event eh.Event) {
		metricValue, ok := event.Data().(*MetricValueEventData)
		if !ok {
			logger.Crit("Expected a *MetricValueEventData but didn't get one.", "Actual Type", fmt.Sprintf("%T", event.Data()), "Acutal Data", event.Data())
			return
		}

		err := MRDFactory.InsertMetricValue(metricValue)
		if err != nil {
			logger.Crit("Error inserting Metric Value", "err", err, "metric", metricValue)
		}
	})
}
