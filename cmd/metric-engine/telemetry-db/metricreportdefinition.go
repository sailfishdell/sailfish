package telemetry

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

	. "github.com/superchalupa/sailfish/cmd/metric-engine/metric"
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

// Validation: It's assumed that Duration is parsed on ingress. The ingress
// format is (Redfish Duration): -?P(\d+D)?(T(\d+H)?(\d+M)?(\d+(.\d+)?S)?)?
// When it gets to this struct, it needs to be expressed in Seconds.
type MRDMetric struct {
	Name               string        `db:"Name" json:"MetricID"`
	CollectionDuration time.Duration `db:"CollectionDuration"`
	CollectionFunction string        `db:"CollectionFunction"`
	PropertyPattern    string        `db:"PropertyPattern"`
	Wildcards          StringArray   `db:"Wildcards"`
}

type MetricReportDefinitionData struct {
	Name         string      `db:"Name"`
	Enabled      bool        `db:"Enabled"`
	Type         string      `db:"Type"` // 'Periodic', 'OnChange', 'OnRequest'
	SuppressDups bool        `db:"SuppressDups"`
	Actions      StringArray `db:"Actions"` // 	'LogToMetricReportsCollection', 'RedfishEvent'
	Updates      string      `db:"Updates"` // 'AppendStopsWhenFull', 'AppendWrapsWhenFull', 'NewReport', 'Overwrite'

	// Validation: It's assumed that Period is parsed on ingress. Redfish
	// "Schedule" object is flexible, but we'll allow only period in seconds for
	// now When it gets to this struct, it needs to be expressed in Seconds.
	Period  int64       `db:"Period"` // period in seconds when type=periodic
	Metrics []MRDMetric `db:"Metrics" json:"Metrics"`
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

	MetricTSHWM SqlTimeInt            // high water mark for received metrics
	NextMRTS    map[string]SqlTimeInt // next timestamp where we need to generate a report
}

func NewMRDFactory(logger log.Logger, database *sqlx.DB) (ret *MRDFactory, err error) {
	ret = &MRDFactory{logger: logger, database: database, NextMRTS: map[string]SqlTimeInt{}}
	err = nil
	return
}

func (factory *MRDFactory) Delete(mrdEvData *MetricReportDefinitionData) (err error) {
	_, err = factory.database.Exec(`delete from MetricReportDefinition where name=?`, mrdEvData.Name)
	if err != nil {
		factory.logger.Crit("ERROR deleting MetricReportDefinition", "err", err, "Name", mrdEvData.Name)
		return
	}
	delete(factory.NextMRTS, mrdEvData.Name)
	return
}

func ValidateMRD(MRD *MetricReportDefinition) {
	switch MRD.Type {
	case "Periodic":
		if MRD.Period < 60 || MRD.Period > (60*60*24) {
			MRD.Period = 180 // period can be 60s to 24hrs. if outside that range, make it 3 minutes.
		}
	case "OnChange", "OnRequest":
		MRD.Period = 0
	default:
		MRD.Type = "invalid"
		MRD.Period = 0
	}

}

func (factory *MRDFactory) UpdateMRD(mrdEvData *MetricReportDefinitionData) (err error) {
	MRD := &MetricReportDefinition{
		MetricReportDefinitionData: mrdEvData,
		MRDFactory:                 factory,
		logger:                     factory.logger,
		AppendLimit:                1000,
	}

	ValidateMRD(MRD)
	// this record will be added back in when the report is created later
	delete(factory.NextMRTS, MRD.Name)

	// ===================================
	// Setup Transaction
	// ===================================
	tx, err := factory.database.Beginx()
	if err != nil {
		return xerrors.Errorf("Transaction create failed: %w", err)
	}

	// if we error out at all, roll back
	defer tx.Rollback()

	// Insert or update existing record
	_, err = tx.NamedExec(
		`INSERT INTO MetricReportDefinition
			( Name,  Enabled, AppendLimit, Type, SuppressDups, Actions, Updates, Period)
			VALUES (:Name, :Enabled, :AppendLimit, :Type, :SuppressDups, :Actions, :Updates, :Period)
		 on conflict(Name) do update set
		 	Enabled=:Enabled,
			AppendLimit=:AppendLimit,
			Type=:Type,
			SuppressDups=:SuppressDups,
			Actions=:Actions,
			Period=:Period,
			Updates=:Updates
			`, MRD)
	if err != nil {
		return xerrors.Errorf("Error inserting MRD(%s): %w", MRD, err)
	}

	// can't use LastInsertId() because it doesn't work on upserts in sqlite
	err = tx.Get(&MRD.ID, `SELECT ID FROM MetricReportDefinition where Name=?`, MRD.Name)
	if err != nil {
		return xerrors.Errorf("Error getting MRD ID for %s: %w", MRD.Name, err)
	}
	factory.logger.Info("Updated/Inserted Metric Report Definition", "Report Definition ID", MRD.ID, "MRD_NAME", MRD.Name)

	//=================================================
	// Update the list of metrics for this report
	//

	// First, just delete all the existing metric associations (not the actual MetricMeta, then we'll re-create
	_, err = tx.Exec(`delete from ReportDefinitionToMetricMeta where ReportDefinitionID=:id`, MRD.ID)
	if err != nil {
		return xerrors.Errorf("Error deleting rd2mm for MRD(%d): %w", MRD.ID, err)
	}

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
				PropertyPattern=:PropertyPattern and
				Wildcards=:Wildcards and
				CollectionFunction=:CollectionFunction and
				CollectionDuration=:CollectionDuration
		`)
		if err != nil {
			return xerrors.Errorf("Error getting MRD ID: %w", err)
		}

		err = statement.Get(&metaID, tempMetric)
		if err != nil {
			if !xerrors.Is(err, sql.ErrNoRows) {
				//factory.logger.Crit("Error getting MetricMeta ID", "err", err, "metric", tempMetric)
				return
			}
			// Insert new MetricMeta if it doesn't already exist per above
			res, err = tx.NamedExec(
				`INSERT INTO MetricMeta
			( Name, SuppressDups, PropertyPattern, Wildcards, CollectionFunction, CollectionDuration)
			VALUES (:Name, :SuppressDups, :PropertyPattern,  :Wildcards, :CollectionFunction, :CollectionDuration)
			`, tempMetric)
			if err != nil {
				return xerrors.Errorf("Error inserting MetricMeta(%s) for MRD(%s): %w", tempMetric, MRD, err)
			}

			metaID, err = res.LastInsertId()
			if err != nil {
				return xerrors.Errorf("Error from LastInsertID for MetricMeta(%s): %w", tempMetric, err)
			}
		}

		// Next cross link MetricMeta to ReportDefinition
		res, err = tx.Exec(`INSERT INTO ReportDefinitionToMetricMeta (ReportDefinitionID, MetricMetaID) VALUES (?, ?)`, MRD.ID, metaID)
		if err != nil {
			return xerrors.Errorf("Error while inserting MetricMeta(%s) association for MRD(%s): %w", metric, MRD, err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return xerrors.Errorf("Transaction failed commit for MRD(%d): %w", MRD.ID, err)
	}
	factory.logger.Debug("Transaction Committed for updates to Report Definition", "Report Definition ID", MRD.ID)

	return
}

var StopIter = xerrors.New("Stop Iteration")

func (factory *MRDFactory) IterMRD(checkFn func(MRD *MetricReportDefinition) bool, fn func(MRD *MetricReportDefinition) error) error {
	tx, err := factory.database.Beginx()
	if err != nil {
		return xerrors.Errorf("Error creating transaction to update MRD: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Preparex(`
		SELECT
			ID, Name,  Enabled, AppendLimit, Type, SuppressDups, Actions, Updates
		FROM MetricReportDefinition
	`)

	// First, Find the MetricMeta
	rows, err := stmt.Queryx()
	if err != nil {
		return xerrors.Errorf("Query error for MRD: %w", err)
	}

	for rows.Next() {
		MRD := &MetricReportDefinition{
			MetricReportDefinitionData: &MetricReportDefinitionData{},
			MRDFactory:                 factory,
			logger:                     factory.logger,
		}
		err = rows.StructScan(MRD)
		if err != nil {
			return xerrors.Errorf("scan error: %w", err)
		}
		if checkFn(MRD) {
			err = fn(MRD)
			if xerrors.Is(err, StopIter) {
				break
			}
			if err != nil {
				// this rolls back the transaction
				return xerrors.Errorf("STOP ITER WITH ERROR --> Iter FN returned error: %w", err)
			}
		}
	}

	err = tx.Commit()
	if err != nil {
		return xerrors.Errorf("Failed transaction commit: %w", err)
	}
	return nil
}

func (l *MRDFactory) FastCheckForNeededMRUpdates() error {
	for MRName, val := range l.NextMRTS {
		if l.MetricTSHWM.After(val.Time) {
			fmt.Println("GEN - CUR_HWM(", l.MetricTSHWM, "): MR(", MRName, ")")
			l.GenerateMetricReport(&MetricReportDefinitionData{Name: MRName})
		}
	}
	return nil
}

func (l *MRDFactory) SlowCheckForNeededMRUpdates() error {
	// TODO: not yet complete
	return l.IterMRD(
		func(MRD *MetricReportDefinition) bool { return true },
		func(MRD *MetricReportDefinition) error {
			fmt.Println("(FAKE) Check for MR Report Updates:", MRD.Name)
			switch MRD.Type {
			case "Periodic":
				switch MRD.Updates {
				case "AppendStopsWhenFull", "AppendWrapsWhenFull", "NewReport", "Overwrite":
				}
			case "OnRequest":
				switch MRD.Updates {
				case "AppendStopsWhenFull", "AppendWrapsWhenFull", "NewReport", "Overwrite":
				}
			case "OnChange":
				switch MRD.Updates {
				case "AppendStopsWhenFull", "AppendWrapsWhenFull", "NewReport", "Overwrite":
				}
			}
			return nil
		})
}

func loadReportDefinition(tx *sqlx.Tx, MRD *MetricReportDefinition) error {
	query := `
		SELECT
			ID, Name,  Enabled, AppendLimit, Type, SuppressDups, Actions, Updates, Period
		FROM MetricReportDefinition
	`
	if MRD.ID > 0 {
		query = query + "WHERE ID=:ID"
	} else if len(MRD.Name) > 0 {
		query = query + "WHERE Name=:Name"
	} else {
		return xerrors.Errorf("Require either an ID or Name to load a Report Definition, didn't get either")
	}

	stmt, err := tx.PrepareNamed(query)
	err = stmt.Get(MRD, MRD)
	if err != nil {
		return xerrors.Errorf("Error loading Metric Report Definition %d:(%s) %w", MRD.ID, MRD.Name, err)
	}
	return nil
}

func (factory *MRDFactory) GenerateMetricReport(mrdEvData *MetricReportDefinitionData) (err error) {
	// ===================================
	// Setup Transaction
	// ===================================
	tx, err := factory.database.Beginx()
	if err != nil {
		return xerrors.Errorf("Error creating transaction to update MRD: %w", err)
	}

	// if we error out at all, roll back
	defer tx.Rollback()

	MRD := &MetricReportDefinition{
		MetricReportDefinitionData: &MetricReportDefinitionData{Name: mrdEvData.Name},
		MRDFactory:                 factory,
		logger:                     factory.logger,
	}
	err = loadReportDefinition(tx, MRD)
	if err != nil {
		return xerrors.Errorf("Error getting MetricReportDefinition: ID(%s) NAME(%s) err: %w", MRD.ID, MRD.Name, err)
	}

	if MRD.ID == 0 {
		return xerrors.Errorf("Error loading Metric Report Definition %d:(%s) %w", MRD.ID, MRD.Name, err)
	}
	ID := MRD.ID

	if !MRD.Enabled {
		// Delete metric reports if MRD is disabled
		_, err = tx.Exec(`delete from MetricReport where ReportDefinitionID=?`, ID)
		if err != nil {
			return xerrors.Errorf("Error deleting MetricReport for ReportDefinitionID(%d): %w", ID, err)
		}
		return nil
	}

	sqlargs := map[string]interface{}{
		"Name":     MRD.Name,
		"MRDID":    MRD.ID,
		"Sequence": 0,
		"Start":    0,
		"End":      0,
	}
	SQL := ""

	switch MRD.Type {
	case "Periodic":
		//TODO: implement AppendLimit
		sqlargs["Start"] = factory.MetricTSHWM.Add(-time.Duration(MRD.Period) * time.Second).UnixNano()
		sqlargs["End"] = factory.MetricTSHWM.UnixNano()
		sqlargs["ReportTimestamp"] = factory.MetricTSHWM.UnixNano()
		factory.NextMRTS[MRD.Name] = SqlTimeInt{factory.MetricTSHWM.Add(time.Duration(MRD.Period) * time.Second)}

		switch MRD.Updates {
		case /*Periodic*/ "NewReport":
			sqlargs["Name"] = fmt.Sprintf("%s-%s", MRD.Name, factory.MetricTSHWM.Time.UTC().Format(time.RFC3339))
			// TODO for appendlimit: unclear on what to do for Periodic/NewReport that exceeds AppendLimit
			//    --> Proposed: *trigger* new report when AppendLimit exceeded. Would need to scan reports for appendlimit somehow
			//        This would happen outside of this code, so this code wouldn't need to change
			// NOTE: we should only have a maximum of 3 total reports per MetricReportDefinition, so delete any >3 after creating new report
			SQL = `INSERT INTO MetricReport (Name, ReportDefinitionID, Sequence, ReportTimestamp, StartTimestamp, EndTimestamp)
			values (:Name, :MRDID, ifnull((select max(sequence)+1 from MetricReport where ReportDefinitionID=:MRDID), 0), :ReportTimestamp, :Start, :End);
			delete from MetricReport where name in (select name from (select MR.name as Name, MR.ReportDefinitionID, MR.sequence as seq, max(MR2.Sequence) as ms from MetricReport as MR left join MetricReport as MR2 on MR.ReportDefinitionID = MR2.ReportDefinitionID group by MR.Name) where seq+2<ms)`

		case /*Periodic*/ "Overwrite":
			// TODO for appendlimit: unclear on what to do for Periodic/OverWrite that exceeds AppendLimit
			//    --> Proposed: *trigger* new report when AppendLimit exceeded. Would need to scan reports for appendlimit somehow
			//        This would happen outside of this code, so this code wouldn't need to change
			SQL = `INSERT INTO MetricReport (Name, ReportDefinitionID, Sequence, ReportTimestamp, StartTimestamp, EndTimestamp)
			values (:Name, :MRDID, :Sequence, :ReportTimestamp, :Start, :End)
				on conflict(Name) do update
				set Sequence=Sequence+1,
				  ReportTimestamp=:ReportTimestamp,
					StartTimestamp=EndTimestamp,
					EndTimestamp=:End`

		case /*Periodic*/ "AppendStopsWhenFull", "AppendWrapsWhenFull":
			// periodic/appendstops basically just periodically appends data to an existing report
			// The "STOPS"/"WRAPS" (*will be) implemented in the VIEW:
			//    order by timestamp ASC LIMIT :appendlimit ("STOPS")
			//    order by timestamp DESC LIMIT :appendlimit ("WRAPS")
			SQL = `INSERT INTO MetricReport (Name, ReportDefinitionID, Sequence, ReportTimestamp, StartTimestamp, EndTimestamp)
			values (:Name, :MRDID, :Sequence, :ReportTimestamp, :Start, :End)
				on conflict(Name) do update
				set Sequence=Sequence+1,
				  ReportTimestamp=:ReportTimestamp,
					EndTimestamp=:End`
		}

	case "OnChange", "OnRequest":
		SQL = `INSERT INTO MetricReport (Name, ReportDefinitionID, Sequence, ReportTimestamp, StartTimestamp, EndTimestamp)
				values (:Name, :MRDID, :Sequence, 0, NULL, NULL) on conflict(name) do nothing`
		switch MRD.Updates {
		case /*OnChange*/ "NewReport":
			SQL = ""
		case /*OnChange*/ "Overwrite":
			SQL = ""
		case /*OnChange*/ "AppendStopsWhenFull":
		case /*OnChange*/ "AppendWrapsWhenFull":
		}

	default:
		SQL = ""
	}

	// not a valid combo
	if len(SQL) == 0 {
		factory.logger.Crit("Report Definition Type not in allowed values", "Name", MRD.Name, "Type", MRD.Type, "Updates", MRD.Updates)
		MRD.Enabled = false
		tx.Exec(`delete from MetricReport where ReportDefinitionID=?`, ID)
		tx.Commit()
		return xerrors.Errorf("Report Definition Type not in allowed values: %s", MRD.Type)
	}

	_, err = tx.NamedExec(SQL, sqlargs)
	if err != nil {
		factory.logger.Crit("ERROR inserting MetricReport", "MetricReportDefinition", MRD, "err", err, "SQL", SQL, "sqlargs", sqlargs)
		return
	}

	err = tx.Commit()
	if err != nil {
		return xerrors.Errorf("Failed transaction commit for Metric Report Update(%d): %w", ID, err)
	}
	factory.logger.Debug("Transaction Committed for updates to Report Definition", "Report Definition ID", ID)
	return nil
}

type Scratch struct {
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
	ID                int64      `db:"ID"`
	InstanceID        int64      `db:"InstanceID"`
	CollectionScratch Scratch    `db:"CollectionScratch"`
	FlushTime         SqlTimeInt `db:"FlushTime"`
	SuppressDups      bool       `db:"SuppressDups"`
	LastTS            SqlTimeInt `db:"LastTS"`
	LastValue         string     `db:"LastValue"`
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
	rows, err := tx.Queryx(`select ID as MetaID, PropertyPattern, Wildcards, CollectionFunction, CollectionDuration from MetricMeta where ? Like Name`, ev.Name)
	if err != nil {
		factory.logger.Crit("Error querying for MetricMeta", "err", err)
		return
	}

	for rows.Next() {
		mm := &MetricMeta{MetricValueEventData: ev}
		err = rows.StructScan(mm)
		if err != nil {
			//factory.logger.Crit("Error scanning metric meta for event", "err", err, "metric", ev)
			continue
		}

		if len(mm.PropertyPattern) > 0 && mm.PropertyPattern != ev.Property {
			// TODO: IMPLEMENT WILDCARD CHECKS
			continue
		}

		// Construct label and Scratch space
		if mm.CollectionFunction != "" {
			mm.Label = fmt.Sprintf("%s - %s - %s", mm.Context, mm.Name, mm.CollectionFunction)
			mm.FlushTime = SqlTimeInt{mm.Timestamp.Add(mm.CollectionDuration * time.Second)}
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
				(MetaID, Name, Property, Context, Label, CollectionScratch, LastValue, LastTS, FlushTime)
				VALUES (:MetaID, :Name, :Property, :Context, :Label, :CollectionScratch, '', 0, :FlushTime)
				on conflict(MetaID, Name, Property, Context, Label) do nothing
			`, mm)
		if err != nil {
			return xerrors.Errorf("Error inserting MetricInstance(%s): %w", mm, err)
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
			MI.FlushTime,
			MM.SuppressDups,
			MM.CollectionFunction,
			MM.CollectionDuration
		from MetricInstance as MI
			inner join MetricMeta as MM on MI.MetaID = MM.ID
		where
		  :Name like MM.Name and
			MI.Property=:Property and
			MI.Context=:Context
		`, mm)
	if err != nil {
		factory.logger.Crit("Error querying for MetricMeta", "err", err)
		return
	}

	for rows.Next() {
		saveValue := true
		saveInstance := false

		mm := &MetricMeta{MetricValueEventData: ev}
		err = rows.StructScan(mm)
		if err != nil {
			//factory.logger.Crit("ERROR loading data from database into MetricMeta struct", "err", err, "MetricMeta", mm)
			continue
		}

		if mm.CollectionFunction == "" {
			mm.ValueToWrite = mm.Value
		} else {
			saveValue = false
			saveInstance = true
			// has the period expired?
			if mm.Timestamp.After(mm.FlushTime.Time) {
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
				// TODO: need a separate query to find all Metrics with HWM > FlushTime and flush them
				mm.FlushTime = SqlTimeInt{mm.Timestamp.Add(mm.CollectionDuration * time.Second)}
				mm.CollectionScratch.Sum = 0
				mm.CollectionScratch.Numvalues = 0
				mm.CollectionScratch.Maximum = -math.MaxFloat64
				mm.CollectionScratch.Minimum = math.MaxFloat64
			}

			val, err := strconv.ParseFloat(mm.Value, 64)
			if err != nil {
				//factory.logger.Warn("Collection failed on metric because Value couldn't be converted to float. Discarding this metric value from the result.",
				//	"Instance ID", mm.InstanceID, "CollectionFunction", mm.CollectionFunction, "Name", mm.Name, "Value", mm.Value, "err", err)
				continue
			}
			mm.CollectionScratch.Numvalues++
			mm.CollectionScratch.Sum += val
			mm.CollectionScratch.Maximum = math.Max(val, mm.CollectionScratch.Maximum)
			mm.CollectionScratch.Minimum = math.Min(val, mm.CollectionScratch.Minimum)
		}

		if mm.SuppressDups && mm.LastValue == mm.ValueToWrite {
			// No need to flush out new value, however instance may or may not need
			// flushing depending on above.
			saveValue = false
		}

		if saveValue {
			if mm.SuppressDups {
				mm.LastValue = mm.ValueToWrite
				mm.LastTS = mm.Timestamp
				saveInstance = true
			}

			args := []interface{}{mm.InstanceID, mm.Timestamp}
			tableName := "MetricValueText"
			for {
				// Put into optimized tables, if possible. Try INT first, as it will error out for a float(1.0) value, but not vice versa
				intVal, err := strconv.ParseInt(mm.Value, 10, 64)
				if err == nil {
					tableName = "MetricValueInt"
					args = append(args, intVal)
					break
				}

				floatVal, err := strconv.ParseFloat(mm.Value, 64)
				if err == nil {
					tableName = "MetricValueReal"
					args = append(args, floatVal)
					break
				}

				args = append(args, mm.Value)
				break
			}

			_, err = tx.Exec(fmt.Sprintf(`INSERT INTO %s (InstanceID, Timestamp, Value) VALUES (?, ?, ?)`, tableName), args...)
			if err != nil {
				return xerrors.Errorf("Error inserting MetricValue for MetricInstance(%d)/MetricMeta(%d): %w", mm.InstanceID, mm.MetaID, err)
			}
		}

		if saveInstance {
			_, err = tx.NamedExec(`
			UPDATE MetricInstance SET LastTS=:LastTS, LastValue=:LastValue, CollectionScratch=:CollectionScratch, FlushTime=:FlushTime
					WHERE ID=:InstanceID
					`, mm)
			if err != nil {
				return xerrors.Errorf("Failed to update MetricInstance(%d) with MetricMeta(%d): %w", mm.InstanceID, mm.MetaID, err)
			}
		}
	}

	err = tx.Commit()
	if err != nil {
		return xerrors.Errorf("Commit failed: %w", err)
	}
	factory.MetricTSHWM = mm.Timestamp

	return nil
}

var orphanOps = map[string]string{
	"Delete Orphan MetricMeta": `
			DELETE FROM MetricMeta WHERE id IN
			(
				select mm.ID from MetricMeta as mm
			  	LEFT JOIN ReportDefinitionToMetricMeta as rd2mm on mm.ID = rd2mm.MetricMetaID where rd2mm.MetricMetaID is null
			)`,
	"Delete Orphan MetricInstance": `
			DELETE FROM MetricInstance WHERE id IN
			(
				select MI.ID from MetricInstance as MI
			  	LEFT JOIN MetricMeta as MM on MM.ID = MI.MetaID where MM.ID is null
			)`,

	// delete orphans of all kinds.
	//   Metric Report Definitions (MRD) are the source of truth
	//    .. Delete any ReportDefinitionToMetricMeta that doesn't match an MRD
	//    .. Delete any ReportDefinitionToMetricMeta that doesn't match a MetricMeta
	//    XX Delete any MetricMeta that doesnt have a ReportDefinitionToMetricMeta entry
	//    oo Delete any MetricInstance without MetricMeta
	//    .. Delete any MetricValue without MetricInstance
	//
	// XX = complete
	// .. = Should be covered by foreign key cascade delete
	// oo = Should be covered by foreign key cascade delete, but double check
}

func (factory *MRDFactory) DeleteOrphans() (err error) {
	factory.logger.Info("Database Maintenance: delete orphans")

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

	// run all the delete ops
	for op, sql := range orphanOps {
		_, err = tx.Exec(sql)
		if err != nil {
			return xerrors.Errorf("Critical error performing orphan op '%s': %w", op, err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return xerrors.Errorf("Failed transaction commit: %w", err)
	}

	return nil
}

var deleteops = []string{
	// The metric value tables should hold ~500k entries.
	`delete from MetricValueInt where Timestamp > (select Timestamp from MetricValueInt order by Timestamp Limit 1 Offset 100000);`,
	`delete from MetricValueReal where Timestamp > (select Timestamp from MetricValueReal order by Timestamp Limit 1 Offset 100000);`,
	`delete from MetricValueText where Timestamp > (select Timestamp from MetricValueText order by Timestamp Limit 1 Offset 50000);`,

	// Only should have max 3 "new" reports per Metric Report Definition
	`delete from MetricReport where name in (select name from (select MR.name as Name, MR.ReportDefinitionID, MR.sequence as seq, max(MR2.Sequence) as ms from MetricReport as MR left join MetricReport as MR2 on MR.ReportDefinitionID = MR2.ReportDefinitionID group by MR.Name) where seq+2<ms)`,
}

func (factory *MRDFactory) DeleteOldestValues() (err error) {
	factory.logger.Info("Database Maintenance: Delete Oldest Metric Values")

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

	// run all the delete ops
	for _, sql := range deleteops {
		_, err = tx.Exec(sql)
		if err != nil {
			return xerrors.Errorf("Critical error performing delete from Metric Value table -> '%s': %w", sql, err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return xerrors.Errorf("Failed transaction commit: %w", err)
	}

	return nil
}
