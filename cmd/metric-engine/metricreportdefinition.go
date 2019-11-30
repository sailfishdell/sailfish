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
	statement, err := factory.database.Prepare(`delete from MetricReportDefinition where name=?`)
	if err != nil {
		factory.logger.Crit("Error Preparing statement for MetricReportDefinition table delete", "err", err)
		return
	}
	_, err = statement.Exec(mrdEvData.Name)
	if err != nil {
		factory.logger.Crit("ERROR deleting MetricReportDefinition", "err", err)
		return
	}
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

	fmt.Printf("CREATE/UPDATE metric report definition: %V\n", MRD)

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
	fmt.Printf("Updated/Inserted Metric Report Definition with ID=%d\n", MRD.ID)

	//
	// Update the list of metrics for this report
	//

	// First, just delete all the existing metric associations (not the actual MetricMeta)
	fmt.Printf("\tDelete from ReportDefinitionToMetricMeta where ReportDefID=%d\n", MRD.ID)
	_, err = tx.NamedExec(`delete from ReportDefinitionToMetricMeta where ReportDefID=:id`, map[string]interface{}{"id": MRD.ID})
	if err != nil {
		factory.logger.Crit("Error executing statement deleting metric meta associations for report definition", "err", err, "ID", MRD.ID)
		return
	}

	// Then we will create each association one at a time
	fmt.Printf("\tPreparing to update metrics for report %d...\n", MRD.ID)
	for i, metric := range MRD.Metrics {
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
		fmt.Printf("\t\tUpdating metric %d: %s\n", i, tempMetric)

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
			if err != sql.ErrNoRows {
				return
			}
			// Insert new MetricMeta if it doesn't already exist per above
			res, err = tx.NamedExec(
				`INSERT INTO MetricMeta
			( Name, SuppressDups, PropertyPattern, Wildcards, CollectionFunction, CollectionDuration)
			VALUES (:Name, :SuppressDups, '',  '', :CollectionFunction, :CollectionDuration)
			`, tempMetric)
			if err != nil {
				factory.logger.Crit("ERROR inserting MetricMeta", "MetricReportDefinition", MRD, "metric", metric, "err", err)
				return
			}

			metaID, _ = res.LastInsertId()
		}

		fmt.Printf("\tGOT MetricMeta ID=%d and Report ID=%d\n", metaID, MRD.ID)

		// Next cross link MetricMeta to ReportDefinition
		res, err = tx.Exec(`INSERT INTO ReportDefinitionToMetricMeta (ReportDefID, MetricMetaID) VALUES (?, ?)`, MRD.ID, metaID)
		if err != nil {
			factory.logger.Crit("ERROR inserting metricmeta association", "MetricReportDefinition", MRD, "metric", metric, "err", err)
			return
		}
	}

	// finally, now delete any "orphan" MetricMeta records
	_, err = tx.Exec(`
		DELETE FROM MetricMeta WHERE id IN
		(
			select mm.ID from MetricMeta as mm
			  LEFT JOIN ReportDefinitionToMetricMeta as rd2mm on mm.ID = rd2mm.MetricMetaID where rd2mm.MetricMetaID is null
		)`)
	if err != nil {
		factory.logger.Crit("Error deleting orphan MetricMeta records", "err", err)
	}

	fmt.Printf("COMMIT REPORT DEF ID %d\n", MRD.ID)

	tx.Commit()

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
			fmt.Printf("ERROR scanning MM: %s\n", err)
		}
		//fmt.Printf("WOULD BE DOING THIS: %s\n", mm)

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
				fmt.Printf("Collection period done for '%s' -> %d\n", mm.CollectionFunction, mm.InstanceID)
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
				fmt.Printf("ERROR converting metric value to numeric")
				continue
			} else {
				mm.CollectionScratch.Numvalues++
				mm.CollectionScratch.Sum += val
				mm.CollectionScratch.Maximum = math.Max(val, mm.CollectionScratch.Maximum)
				mm.CollectionScratch.Minimum = math.Min(val, mm.CollectionScratch.Minimum)
			}
		}

		if mm.SuppressDups && mm.LastValue == mm.ValueToWrite {
			saveValue = false
		}

		// TODO on ingress
		// Duration parsing: -?P(\d+D)?(T(\d+H)?(\d+M)?(\d+(.\d+)?S)?)?

		if saveValue {
			if mm.SuppressDups {
				mm.LastValue = mm.ValueToWrite
				mm.LastTS = mm.Timestamp
				saveInstance = true
			}

			_, err = tx.NamedExec(`
					INSERT INTO MetricValue
						( InstanceID, Timestamp, Value )
						VALUES (:InstanceID, :Timestamp, :Value )
					`, mm)
			if err != nil {
				factory.logger.Crit("ERROR inserting MetricValue", "MetaID", mm.MetaID, "InstanceID", mm.InstanceID, "err", err)
				return
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

	tx.Commit()
	return nil
}
