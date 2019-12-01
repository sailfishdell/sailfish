package main

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"database/sql"
	"database/sql/driver"
	"github.com/jmoiron/sqlx"
	"golang.org/x/xerrors"

	log "github.com/superchalupa/sailfish/src/log"
)

type StringArray []string

func (m StringArray) Value() (driver.Value, error) {
	b, err := json.Marshal(m)
	return b, err
}

func (m *StringArray) Scan(src interface{}) error {
	return json.Unmarshal(src.([]byte), m)
}

// Validation: It's assumed that Duration is parsed on ingress. The ingress format is (Redfish Duration): -?P(\d+D)?(T(\d+H)?(\d+M)?(\d+(.\d+)?S)?)?
// When it gets to this struct, it needs to be expressed in Seconds.
type MRDMetric struct {
	Name               string        `db:"Name" json:"MetricID"`
	CollectionDuration time.Duration `db:"CollectionDuration"`
	CollectionFunction string        `db:"CollectionFunction"`
	// TODO: properties and wildcards
}

type MetricReportDefinitionData struct {
	Name         string      `db:"Name"`
	Enabled      bool        `db:"Enabled"`
	Type         string      `db:"Type"` // 'Periodic', 'OnChange', 'OnRequest'
	SuppressDups bool        `db:"SuppressDups"`
	Actions      StringArray `db:"Actions"` // 	'LogToMetricReportsCollection', 'RedfishEvent'
	Updates      string      `db:"Updates"` // 'AppendStopsWhenFull', 'AppendWrapsWhenFull', 'NewReport', 'Overwrite'
	Metrics      []MRDMetric `db:"Metrics" json:"Metrics"`
}

type MetricReportDefinition struct {
	*MetricReportDefinitionData
	*MRDFactory
	AppendLimit int   `db:"AppendLimit"`
	ID          int64 `db:"ID"`
	logger      log.Logger
	loaded      bool
}

// Factory manages getting/putting into db
type MRDFactory struct {
	logger   log.Logger
	database *sqlx.DB
}

func NewMRDFactory(logger log.Logger, database *sqlx.DB) (ret *MRDFactory, err error) {
	ret = &MRDFactory{logger: logger, database: database}
	err = nil
	return
}

func (factory *MRDFactory) Delete(mrdEvData *MetricReportDefinitionData) (err error) {
	res, err := factory.database.Exec(`delete from MetricReportDefinition where name=?`, mrdEvData.Name)
	if err != nil {
		factory.logger.Crit("ERROR deleting MetricReportDefinition", "err", err, "Name", mrdEvData.Name)
		return
	}
	numrows, err := res.RowsAffected()
	factory.logger.Debug("DELETED rows from MetricReportDefinition", "numrows", numrows, "err", err)
	return
}

func (factory *MRDFactory) Update(mrdEvData *MetricReportDefinitionData) (MRD *MetricReportDefinition, err error) {
	// Random TODO: need a validation function
	MRD = &MetricReportDefinition{
		MetricReportDefinitionData: mrdEvData,
		MRDFactory:                 factory,
		logger:                     factory.logger,
		AppendLimit:                1000,
	}

	factory.logger.Info("CREATE/UPDATE metric report definition", "MRD", MRD)

	// ===================================
	// Setup Transaction
	// ===================================
	tx, err := factory.database.Beginx()
	if err != nil {
		factory.logger.Crit("Error creating transaction to update MRD", "err", err)
		return
	}

	// if we error out at all, roll back
	defer tx.Rollback()

	// Insert or update existing record
	_, err = tx.NamedExec(
		`INSERT INTO MetricReportDefinition
			( Name,  Enabled, AppendLimit, Type, SuppressDups, Actions, Updates)
			VALUES (:Name, :Enabled, :AppendLimit, :Type, :SuppressDups, :Actions, :Updates)
		 on conflict(Name) do update set
		 	Enabled=:Enabled,
			AppendLimit=:AppendLimit,
			Type=:Type,
			SuppressDups=:SuppressDups,
			Actions=:Actions,
			Updates=:Updates
			`, MRD)
	if err != nil {
		factory.logger.Crit("ERROR inserting MetricReportDefinition", "MetricReportDefinition", MRD, "err", err)
		return
	}

	// can't use LastInsertId() because it doesn't work on upserts in sqlite
	err = tx.Get(&MRD.ID, `SELECT ID FROM MetricReportDefinition where Name=?`, MRD.Name)
	if err != nil {
		factory.logger.Crit("Error getting MetricReportDefinition ID", "err", err)
		return
	}
	factory.logger.Info("Updated/Inserted Metric Report Definition", "Report Definition ID", MRD.ID, "MRD", MRD)

	//
	// Update the list of metrics for this report
	//

	// First, just delete all the existing metric associations (not the actual MetricMeta, then we'll re-create
	res, err := tx.Exec(`delete from ReportDefinitionToMetricMeta where ReportDefinitionID=:id`, MRD.ID)
	if err != nil {
		factory.logger.Crit("Error executing statement deleting metric meta associations for report definition", "err", err, "Report Definition ID", MRD.ID)
		return
	}
	numrows, err := res.RowsAffected()
	factory.logger.Debug("DELETED rows from ReportDefinitionToMetricMeta", "numrows", numrows, "err", err)

	// Then we will create each association one at a time
	for _, metric := range MRD.Metrics {
		var metaID int64
		var res sql.Result
		var statement *sqlx.NamedStmt
		tempMetric := struct {
			*MRDMetric
			SuppressDups bool `db:"SuppressDups"`
		}{
			MRDMetric:    &metric,
			SuppressDups: MRD.SuppressDups,
		}

		// First, Find the MetricMeta
		statement, err = tx.PrepareNamed(`
			select ID from MetricMeta where
				Name=:Name and
				SuppressDups=:SuppressDups and
				PropertyPattern='' and
				Wildcards='' and
				CollectionFunction=:CollectionFunction and
				CollectionDuration=:CollectionDuration
		`)
		if err != nil {
			factory.logger.Crit("Error getting MetricReportDefinition ID", "err", err)
			return
		}

		err = statement.Get(&metaID, tempMetric)
		if err != nil {
			if !xerrors.Is(err, sql.ErrNoRows) {
				factory.logger.Crit("Error getting MetricMeta ID", "err", err, "metric", tempMetric)
				return
			}
			// Insert new MetricMeta if it doesn't already exist per above
			res, err = tx.NamedExec(
				`INSERT INTO MetricMeta
			( Name, SuppressDups, PropertyPattern, Wildcards, CollectionFunction, CollectionDuration)
			VALUES (:Name, :SuppressDups, '',  '', :CollectionFunction, :CollectionDuration)
			`, tempMetric)
			if err != nil {
				factory.logger.Crit("ERROR inserting MetricMeta", "MetricReportDefinition", MRD, "metric", tempMetric, "err", err)
				return
			}

			metaID, err = res.LastInsertId()
			if err != nil {
				factory.logger.Crit("Error getting last inserted row ID for MetricMeta", "err", err, "metric", tempMetric)
				return
			}
			numrows, err := res.RowsAffected()
			factory.logger.Info("Added new MetricMeta", "MetaID", metaID, "metric", tempMetric, "numrows", numrows, "err", err)
		}

		// Next cross link MetricMeta to ReportDefinition
		res, err = tx.Exec(`INSERT INTO ReportDefinitionToMetricMeta (ReportDefinitionID, MetricMetaID) VALUES (?, ?)`, MRD.ID, metaID)
		if err != nil {
			factory.logger.Crit("ERROR inserting metricmeta association", "MetricReportDefinition", MRD, "metric", metric, "err", err)
			return
		}
		numrows, err := res.RowsAffected()
		factory.logger.Debug("Linked Report Def to MetricMeta", "Report Definition ID", MRD.ID, "Meta ID", metaID, "numrows", numrows, "err", err)
	}

	// finally, now delete any "orphan" MetricMeta records
	res, err = tx.Exec(`
		DELETE FROM MetricMeta WHERE id IN
		(
			select mm.ID from MetricMeta as mm
			  LEFT JOIN ReportDefinitionToMetricMeta as rd2mm on mm.ID = rd2mm.MetricMetaID where rd2mm.MetricMetaID is null
		)`)
	if err != nil {
		factory.logger.Crit("Error deleting orphan MetricMeta records", "err", err)
		return
	}
	numrows, err = res.RowsAffected()
	if numrows > 0 {
		factory.logger.Debug("DELETED ORPHANS from MetricMeta", "numrows", numrows, "err", err)
	} else {
		factory.logger.Debug("no orphans in MetricMeta", "numrows", numrows, "err", err)
	}

	// ReCreate the Metric Report
	// First, just delete all the existing metric associations (not the actual MetricMeta, then we'll re-create
	res, err = tx.Exec(`delete from MetricReport where ReportDefinitionID=?`, MRD.ID)
	if err != nil {
		factory.logger.Crit("Error deleting metric reports for report definition", "err", err, "Report Definition ID", MRD.ID)
		return
	}
	numrows, err = res.RowsAffected()
	if numrows > 0 {
		factory.logger.Debug("DELETED rows from MetricReport", "numrows", numrows, "err", err)
	} else {
		factory.logger.Debug("no rows to delete from MetricReport", "numrows", numrows, "err", err)
	}

	// Type: Periodic, OnChange, OnRequest
	// 		Updates: AppendStopsWhenFull | AppendWrapsWhenFull | Overwrite | NewReport
	//
	// Periodic:   (-) AppendStopsWhenFull  (-) AppendWrapsWhenFull   (-) Overwrite   (!) NewReport
	// OnChange:   (-) AppendStopsWhenFull  (-) AppendWrapsWhenFull   (-) Overwrite   (X) NewReport
	// OnRequest:  (-) AppendStopsWhenFull  (-) AppendWrapsWhenFull   (X) Overwrite   (X) NewReport
	//
	// key:
	// 		(-) makes sense and should be implemented
	// 		(X) invalid combination, dont accept
	// 	  (!) makes sense but sort of difficult to implement, so skip for now
	//    (o) partially implemented
	//    (*) Done
	//
	// behaviour:
	//    Periodic: (generate a report, then at time interval dump accumulated values)
	//      --> The Metric Value insert doesn't change reports. Best performance.
	// 			--> Sequence is updated on period
	// 		  --> Timestamp is updated on period
	// 			AppendStopsWhenFull: (Status=Full), StartTimestamp = fixed, EndTimestamp = fixed/updated until full, then fixed
	//          -- DELETE to clear?
	// 					-- newer entries fall on floor
	// 			AppendWrapsWhenFull: (Status=Full), StartTimestamp = NULL until full, then fixed/updated periodically, EndTimestamp = fixed/updated periodically
	// 					-- older entries fall on floor
	//      NewReport: insert new MetricReport, setup timestamps  (timestamps = fixed)
	// 					-- need a policy to delete older reports(?)
	//      Overwrite: starttimestamp=fixed, endtimestamp=fixed
	// 				  -- update same metric report
	//
	//    OnRequest:  things trickle in as they come
	//      --> Have to update the report on insert of Metric Value
	// 			--> Sequence is 0 unless "generated" (trigger)
	// 		  --> Timestamp is 0 unless "generated" (trigger)
	// 			AppendStopsWhenFull: continuously update EndTimestamp until full, then fixed
	//          -- DELETE to clear?
	// 					-- newer entries fall on floor
	// 			AppendWrapsWhenFull: EndTimestamp=NULL, continuously update start
	// 					-- older entries fall on floor
	//      NewReport: no
	//      Overwrite: no
	//
	// Dont see how this one is different from OnRequest from a behaviour standpoint. Maybe Actions?
	//    OnChange:  things trickle in as they come
	//      --> Have to update the report on insert of Metric Value
	// 			--> Sequence is 0 unless "generated" (trigger)
	// 		  --> Timestamp is 0 unless "generated" (trigger)
	// 			AppendStopsWhenFull: continuously update EndTimestamp until full, then fixed
	//          -- DELETE to clear?
	// 					-- newer entries fall on floor
	// 			AppendWrapsWhenFull: EndTimestamp=NULL, continuously update start
	// 					-- older entries fall on floor
	//      NewReport: no
	//      Overwrite: no

	res, err = tx.Exec(`INSERT INTO MetricReport (Name, ReportDefinitionID, Sequence) VALUES (?, ?, ?)`, MRD.Name, MRD.ID, 0)
	if err != nil {
		factory.logger.Crit("ERROR inserting MetricReport", "MetricReportDefinition", MRD, "err", err)
		return
	}
	numrows, err = res.RowsAffected()
	if numrows > 0 {
		factory.logger.Debug("Inserted MetricReport rows", "numrows", numrows, "err", err, "Name", MRD.Name, "ID", MRD.ID)
	} else {
		factory.logger.Debug("no rows to delete from MetricReport", "numrows", numrows, "err", err, "Name", MRD.Name, "ID", MRD.ID)
	}

	err = tx.Commit()
	if err != nil {
		factory.logger.Crit("FAILED Transaction Commit for updates to Report Definition", "Report Definition ID", MRD.ID, "err", err)
		return
	}
	factory.logger.Debug("Transaction Committed for updates to Report Definition", "Report Definition ID", MRD.ID)

	return
}

type NanoTime struct {
	time.Time
}

func (m NanoTime) Value() (driver.Value, error) {
	return m.UnixNano(), nil
}

func (m *NanoTime) Scan(src interface{}) error {
	m.Time = time.Unix(0, src.(int64))
	return nil
}

type Scratch struct {
	Start     NanoTime
	Numvalues int
	Sum       float64
	Maximum   float64
	Minimum   float64
}

func (m Scratch) Value() (driver.Value, error) {
	b, err := json.Marshal(m)
	return b, err
}

func (m *Scratch) Scan(src interface{}) error {
	json.Unmarshal(src.([]byte), m)
	return nil
}

// Fusion structure: Meta + Instance + MetricValueEvent
type MetricMeta struct {
	*MetricValueEventData
	ValueToWrite string `db:"Value"`

	// Meta fields
	Label              string        `db:"Label"`
	MetaID             int64         `db:"MetaID"`
	PropertyPattern    string        `db:"PropertyPattern"`
	Wildcards          string        `db:"Wildcards"`
	CollectionFunction string        `db:"CollectionFunction"`
	CollectionDuration time.Duration `db:"CollectionDuration"`

	// Instance fields
	ID                int64    `db:"ID"`
	InstanceID        int64    `db:"InstanceID"`
	CollectionScratch Scratch  `db:"CollectionScratch"`
	SuppressDups      bool     `db:"SuppressDups"`
	LastTS            NanoTime `db:"LastTS"`
	LastValue         string   `db:"LastValue"`
}

func (factory *MRDFactory) Optimize() {
	factory.logger.Debug("Optimizing database - start")
	defer factory.logger.Debug("Optimizing database - done")
	_, err := factory.database.Exec("PRAGMA optimize")
	if err != nil {
		factory.logger.Crit("Problem optimizing database", "err", err)
	}
}

func (factory *MRDFactory) Vacuum() {
	factory.logger.Debug("Vacuuming database - start")
	defer factory.logger.Debug("Vacuuming database - done")
	_, err := factory.database.Exec("vacuum")
	if err != nil {
		factory.logger.Crit("Problem vacuuming database", "err", err)
	}
}

func (factory *MRDFactory) InsertMetricValue(ev *MetricValueEventData) (err error) {
	// ===================================
	// Setup transaction
	// ===================================
	tx, err := factory.database.Beginx()
	if err != nil {
		factory.logger.Crit("Error creating transaction to update MRD", "err", err)
		return
	}
	// if we error out at all, roll back
	defer tx.Rollback()

	// TODO: cache the MetricMeta(?)
	// it may speed things up if we cache the MetricMeta in-process rather than going to DB every time.
	// This should be straightforward because we do all updates in one goroutine, so could add the cache as a factory member

	// First, Find the MetricMeta
	rows, err := tx.Queryx(`select ID as MetaID, PropertyPattern, Wildcards, CollectionFunction, CollectionDuration from MetricMeta where Name=?`, ev.Name)
	if err != nil {
		factory.logger.Crit("Error querying for MetricMeta", "err", err)
		return
	}

	for rows.Next() {
		mm := &MetricMeta{MetricValueEventData: ev}
		err = rows.StructScan(mm)
		if err != nil {
			factory.logger.Crit("Error scanning metric meta for event", "err", err, "metric", ev)
			continue
		}

		// TODO: check PropertyPattern/Wildcards

		// Construct label and Scratch space
		if mm.CollectionFunction != "" {
			mm.Label = fmt.Sprintf("%s - %s - %s", mm.Context, mm.Name, mm.CollectionFunction)
			mm.CollectionScratch.Start = mm.Timestamp
			mm.CollectionScratch.Sum = 0
			mm.CollectionScratch.Numvalues = 0
			mm.CollectionScratch.Maximum = -math.MaxFloat64
			mm.CollectionScratch.Minimum = math.MaxFloat64
		} else {
			mm.Label = fmt.Sprintf("%s - %s", mm.Context, mm.Name)
		}

		// create instances for each metric meta corresponding to this metric value
		_, err = tx.NamedExec(`
			INSERT INTO MetricInstance
				(MetaID, Property, Context, Label, CollectionScratch, LastValue, LastTS)
				VALUES (:MetaID, :Property, :Context, :Label, :CollectionScratch, '', 0)
		 	on conflict(MetaID, Property, Context, Label) do nothing
			`, mm)
		if err != nil {
			factory.logger.Crit("Error inserting new MetricInstance", "err", err, "mm", mm)
			return
		}
	}

	// And now, foreach MetricInstance, insert MetricValue
	mm := &MetricMeta{MetricValueEventData: ev}
	rows, err = tx.NamedQuery(`
		select
		  MI.ID as InstanceID,
			MI.MetaID,
			MI.CollectionScratch,
			MI.LastTS,
			MI.LastValue,
			MM.SuppressDups,
			MM.CollectionFunction,
			MM.CollectionDuration
		from MetricInstance as MI
			inner join MetricMeta as MM on MI.MetaID = MM.ID
		where
			MM.Name=:Name and
			MI.Property=:Property and
			MI.Context=:Context
		`, mm)
	if err != nil {
		factory.logger.Crit("Error querying for MetricMeta", "err", err)
		return
	}

	for rows.Next() {
		// TODO: implement un-suppress
		// TODO: honor AppendLimit
		// TODO: honor AppendStopsWhenFull | AppendWrapsWhenFull | Overwrite | NewReport
		// TODO: honor Type=OnChange (send an event?)

		saveValue := true
		saveInstance := false

		mm := &MetricMeta{MetricValueEventData: ev}
		err = rows.StructScan(mm)
		if err != nil {
			factory.logger.Crit("ERROR loading data from database into MetricMeta struct", "err", err, "MetricMeta", mm)
			continue
		}

		if mm.CollectionFunction == "" {
			mm.ValueToWrite = mm.Value
		} else {
			saveValue = false
			saveInstance = true

			// has the period expired?
			// TODO: if the next measurement wont come in before the end of collection duration, drop in the measurement (? - can we do this?)
			if mm.Timestamp.After(mm.CollectionScratch.Start.Add(mm.CollectionDuration * time.Second)) {
				// Calculate what we should be dropping in the output
				saveValue = true
				factory.logger.Info("Collection period done Metric Instance", "Instance ID", mm.InstanceID, "CollectionFunction", mm.CollectionFunction)
				switch mm.CollectionFunction {
				case "Average":
					mm.ValueToWrite = strconv.FormatFloat(mm.CollectionScratch.Sum/float64(mm.CollectionScratch.Numvalues), 'G', -1, 64)
				case "Maximum":
					mm.ValueToWrite = strconv.FormatFloat(mm.CollectionScratch.Maximum, 'G', -1, 64)
				case "Minimum":
					mm.ValueToWrite = strconv.FormatFloat(mm.CollectionScratch.Minimum, 'G', -1, 64)
				case "Summation":
					mm.ValueToWrite = strconv.FormatFloat(mm.CollectionScratch.Sum, 'G', -1, 64)
				default:
					mm.ValueToWrite = "Invalid or Unsupported CollectionFunction"
				}

				// now, reset everything
				mm.CollectionScratch.Start = mm.Timestamp
				mm.CollectionScratch.Sum = 0
				mm.CollectionScratch.Numvalues = 0
				mm.CollectionScratch.Maximum = -math.MaxFloat64
				mm.CollectionScratch.Minimum = math.MaxFloat64
			}

			val, err := strconv.ParseFloat(mm.Value, 64)
			if err != nil {
				factory.logger.Warn("Collection failed on metric because Value couldn't be converted to float. Discarding this metric value from the result.",
					"Instance ID", mm.InstanceID, "CollectionFunction", mm.CollectionFunction, "Name", mm.Name, "Value", mm.Value, "err", err)
				continue
			}
			mm.CollectionScratch.Numvalues++
			mm.CollectionScratch.Sum += val
			mm.CollectionScratch.Maximum = math.Max(val, mm.CollectionScratch.Maximum)
			mm.CollectionScratch.Minimum = math.Min(val, mm.CollectionScratch.Minimum)
		}

		if mm.SuppressDups && mm.LastValue == mm.ValueToWrite {
			saveValue = false
		}

		if saveValue {
			if mm.SuppressDups {
				mm.LastValue = mm.ValueToWrite
				mm.LastTS = mm.Timestamp
				saveInstance = true
			}

			var res sql.Result
			var numrows int64
			res, err = tx.NamedExec(`
					INSERT INTO MetricValue
						( InstanceID, Timestamp, Value )
						VALUES (:InstanceID, :Timestamp, :Value )
					`, mm)
			if err != nil {
				factory.logger.Crit("ERROR inserting MetricValue", "MetaID", mm.MetaID, "InstanceID", mm.InstanceID, "err", err)
				return
			}
			numrows, err = res.RowsAffected()
			if numrows > 0 {
				factory.logger.Debug("Inserted MetricValue rows", "numrows", numrows, "err", err, "MetaID", mm.MetaID, "InstanceID", mm.InstanceID)
			} else {
				factory.logger.Warn("no rows to insert MetricValue", "numrows", numrows, "err", err, "MetaID", mm.MetaID, "InstanceID", mm.InstanceID)
			}
		}

		if saveInstance {
			_, err = tx.NamedExec(`
				UPDATE MetricInstance SET LastTS=:LastTS, LastValue=:LastValue, CollectionScratch=:CollectionScratch
					WHERE ID=:InstanceID
					`, mm)
			if err != nil {
				factory.logger.Crit("ERROR updating MetricInstance", "MetaID", mm.MetaID, "InstanceID", mm.InstanceID, "err", err)
				return
			}
		}
	}

	err = tx.Commit()
	if err != nil {
		factory.logger.Crit("Transaction Committed FAILED for Metric Value insertion", "err", err)
		return
	}
	factory.logger.Info("Transaction Committed for Metric Value insertion")

	return nil
}
