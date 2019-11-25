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
	database, selectMetaRecordID, insertMeta, insertValue, err := createDatabase(logger, dbpath)
	if err != nil {
		logger.Crit("Could not create database", "err", err)
		return
	}

	MRDFactory := NewMRDFactory(database, selectMetaRecordID, insertMeta, insertValue)

	metricReports := map[string]*MetricReportDefinition{}
	mm := MetricMap{}

	am3Svc.AddEventHandler("update_metric_report_definition", UpdateMetricReportDefinition, func(event eh.Event) {
		reportDef, ok := event.Data().(*MetricReportDefinitionData)
		if !ok {
			fmt.Println("should never happen")
			return
		}

		mrd, ok := metricReports[reportDef.Name]
		if !ok {
			mrd, err = MRDFactory.New(logger, reportDef)
			if err != nil {
				fmt.Println("error creating new report definition")
				return
			}
			metricReports[reportDef.Name] = mrd
		}

		// TODO: update it, if needed

		mm = MetricMap{}
		for _, mrd := range metricReports {
			mrd.UpdateMetricMap(mm)
		}

		fmt.Printf("Metric Reports: %V\n\n", metricReports)
		fmt.Printf("Metric Map: %V\n\n", mm)
	})

	am3Svc.AddEventHandler("delete_metric_report_definition", DeleteMetricReportDefinition, func(event eh.Event) {
		reportDef, ok := event.Data().(*MetricReportDefinitionData)
		if !ok {
			fmt.Println("should never happen")
			return
		}

		// TODO: close prepared statement handles
		// TODO: delete database records
		delete(metricReports, reportDef.Name)
	})

	am3Svc.AddEventHandler("metric_storage", MetricValueEvent, func(event eh.Event) {
		metricValue, ok := event.Data().(*MetricValueEventData)
		if !ok {
			fmt.Println("Should never happen: got a metric value event without metricvalueeventdata:", event.Data())
			return
		}
		for _, mrd := range mm[metricValue.MetricID] {
			mrd.Insert(metricValue)
		}
	})
}
