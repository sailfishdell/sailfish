package main

import (
	"database/sql"
	"fmt"
	"time"

	log "github.com/superchalupa/sailfish/src/log"
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
	ReportID int64
	Insert   func(*MetricValueEventData)
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

type MrdFactory struct {
	database                                    *sql.DB
	selectMetaRecordID, insertMeta, insertValue *sql.Stmt
}

func NewMRDFactory(database *sql.DB, selectMetaRecordID, insertMeta, insertValue *sql.Stmt) *MrdFactory {
	return &MrdFactory{database, selectMetaRecordID, insertMeta, insertValue}
}

func (f *MrdFactory) New(logger log.Logger, mrdEvData *MetricReportDefinitionData) (MRD *MetricReportDefinition, err error) {
	MRD = &MetricReportDefinition{
		MetricReportDefinitionData: mrdEvData,
	}

	// ===================================
	// Insert for new report definition
	// ===================================
	statement, err := f.database.Prepare(`INSERT INTO MetricReportDefinition (name) VALUES (?)`)
	if err != nil {
		logger.Crit("Error Preparing statement for MetricReportDefinition table insert", "err", err)
		return
	}
	res, err := statement.Exec(MRD.Name)
	if err != nil {
		fmt.Printf("ERROR inserting MetricReportDefinition: %s  ", err)
		return
	}
	MRD.ReportID, err = res.LastInsertId()
	if err != nil {
		fmt.Printf("ERROR getting last insert value for MetricReportDefinition: %s  ", err)
		return
	}
	statement.Close()

	// ==================================================================
	// insertFn is used to insert individual metric values into storage
	// ==================================================================
	MRD.Insert = func(mv *MetricValueEventData) {
		// TODO: implement un-suppress here
		success := false
		defer func() {
			if success {
				logit := logger.Info
				message := "Inserted Metric"
				if !success {
					logit = logger.Warn
					message = "FAILED inserting metric"
				}
				logit(message, "Report", MRD.Name, "metric id", mv.MetricID)
			}
		}()
		var recordID int64
		var stop bool
		row := f.selectMetaRecordID.QueryRow(MRD.ReportID, mv.MetricID, mv.URI, mv.Property, mv.Context)
		err := row.Scan(&recordID, &stop)
		if err != nil {
			if err == sql.ErrNoRows {
				res, err := f.insertMeta.Exec(MRD.ReportID, mv.MetricID, mv.URI, mv.Property, mv.Context, mv.Label, mv.Stop, mv.Stop)
				if err != nil {
					fmt.Printf("ERROR inserting meta: %s  ", err)
					return
				}
				recordID, err = res.LastInsertId()
				if err != nil {
					fmt.Printf("ERROR getting last insert value: %s  ", err)
					return
				}
			} else {
				logger.Crit("Error scanning for metric record id", "err", err, "Report", MRD.Name, "metric id", mv.MetricID)
			}
		}

		_, err = f.insertValue.Exec(recordID, mv.Timestamp, mv.MetricValue)
		if err != nil {
			logger.Crit("ERROR inserting value", "err", err)
			return
		}
		success = true
	}

	return
}
