// +build idrac

package main

import (
	"database/sql"
	"fmt"
	"time"

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

type MRDMetric struct {
	CollectionDuration  time.Duration
	CollectionFunction  string
	CollectionTimeScope string
	MetricID            string
	// future: MetricProperties []struct{}
}

type MetricReportDefinitionData struct {
	Name                          string
	MetricReportDefinitionEnabled bool
	MetricReportDefinitionType    string
	MetricReportHeartbeatInterval time.Duration
	ReportActions                 []string
	ReportTimespan                time.Duration
	ReportUpdates                 []string
	Schedule                      string // TODO
	Metrics                       []MRDMetric
}

type MetricReportDefinition struct {
	*MetricReportDefinitionData
	Insert func(*MetricValueEventData)
	Drop   func()
}

func (mrd *MetricReportDefinition) FilterMetricValue(mv *MetricValueEventData) bool {
	// TODO: construct a reverse map to optimize this
	for _, m := range mrd.Metrics {
		if m.MetricID == mv.MetricID {
			return true
		}
	}
	return false
}

type MetricMap map[string][]*MetricReportDefinition

func (mrd *MetricReportDefinition) UpdateMetricMap(mm MetricMap) {
	// TODO: when we support more expressive filtering (ie. propery-based), add wildcard entry if needed: mm['*'] = [mrd, ...]
	for _, m := range mrd.Metrics {
		ary, ok := mm[m.MetricID]
		if !ok {
			ary = []*MetricReportDefinition{}
		}
		ary = append(ary, mrd)
		mm[m.MetricID] = ary
	}
}

func (mrd *MetricReportDefinition) Enabled() bool {
	return mrd.MetricReportDefinitionEnabled
}

func addAM3DatabaseFunctions(logger log.Logger, am3Svc *am3.Service, d *domain.DomainObjects) {
	eh.RegisterEventData(UpdateMetricReportDefinition, func() eh.EventData { return &MetricReportDefinitionData{} })

	database, _ := sql.Open("sqlite3", "./metricvalues.db")

	metricReports := map[string]*MetricReportDefinition{}
	mm := MetricMap{}

	am3Svc.AddEventHandler("update_metric_report_definition", UpdateMetricReportDefinition, func(event eh.Event) {
		reportDef, ok := event.Data().(*MetricReportDefinitionData)
		if !ok {
			fmt.Println("should never happen")
			return
		}

		statement, _ := database.Prepare(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s_Values (ts datetime, metricid varchar(64), metricvalue varchar(64))", reportDef.Name))
		statement.Exec()

		insertStatement, _ := database.Prepare(fmt.Sprintf("INSERT INTO %s_Values (ts, metricid, metricvalue) VALUES (?, ?, ?)", reportDef.Name))
		insertFn := func(mv *MetricValueEventData) {
			// TODO: implement un-suppress here
			insertStatement.Exec(mv.Timestamp, mv.MetricID, mv.MetricValue)
		}
		dropStatement, _ := database.Prepare(fmt.Sprintf("drop table %s", reportDef.Name))

		metricReports[reportDef.Name] = &MetricReportDefinition{
			MetricReportDefinitionData: reportDef,
			Insert:                     insertFn,
			Drop:                       func() { dropStatement.Exec() },
		}

		mm = MetricMap{}
		for _, mrd := range metricReports {
			mrd.UpdateMetricMap(mm)
		}
	})

	am3Svc.AddEventHandler("delete_metric_report_definition", DeleteMetricReportDefinition, func(event eh.Event) {
		reportDef, ok := event.Data().(*MetricReportDefinitionData)
		if !ok {
			fmt.Println("should never happen")
			return
		}

		metricReports[reportDef.Name].Drop()
		delete(metricReports, reportDef.Name)
	})

	am3Svc.AddEventHandler("metric_storage", MetricValueEvent, func(event eh.Event) {
		fmt.Printf("HELLO WORLD\n")
		metricValue, ok := event.Data().(*MetricValueEventData)
		if !ok {
			fmt.Println("Should never happen: got a metric value event without metricvalueeventdata:", event.Data())
			return
		}
		fmt.Println("DEBUG:", metricValue.Timestamp, metricValue.MetricID, metricValue.MetricValue)
		for _, mrd := range mm[metricValue.MetricID] {
			mrd.Insert(metricValue)
		}
	})
}
