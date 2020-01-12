package telemetry

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"database/sql"
	"database/sql/driver"
	"github.com/jmoiron/sqlx"
	"github.com/spf13/viper"
	"golang.org/x/xerrors"

	"github.com/superchalupa/sailfish/cmd/metric-engine/metric"
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
	FQDDPattern        string        `db:"FQDDPattern"`
	SourcePattern      string        `db:"SourcePattern"`
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

// MetricReportDefinition represents a DB record for a metric report
// definition. Basically adds ID and a static AppendLimit (for now, until we
// can figure out how to make this dynamic).
type MetricReportDefinition struct {
	*MetricReportDefinitionData
	AppendLimit int   `db:"AppendLimit"`
	ID          int64 `db:"ID"`
}

// Factory manages getting/putting into db
type MRDFactory struct {
	logger           log.Logger
	database         *sqlx.DB
	preparedNamedSql map[string]*sqlx.NamedStmt
	preparedSql      map[string]*sqlx.Stmt
	deleteops        []string

	MetricTSHWM metric.SqlTimeInt            // high water mark for received metrics
	NextMRTS    map[string]metric.SqlTimeInt // next timestamp where we need to generate a report
}

// NewMRDFactory is the constructor for the base telemetry service functionality
func NewMRDFactory(logger log.Logger, database *sqlx.DB, cfg *viper.Viper) (*MRDFactory, error) {
	// make sure not to store the 'cfg' passed in. That would be Bad.

	factory := &MRDFactory{
		logger:           logger,
		database:         database,
		NextMRTS:         map[string]metric.SqlTimeInt{},
		preparedNamedSql: map[string]*sqlx.NamedStmt{},
		preparedSql:      map[string]*sqlx.Stmt{},
		deleteops:        cfg.GetStringSlice("main.deleteops"),
	}

	// Create tables and views from sql stored in our YAML
	for name, sql := range cfg.GetStringMapString("internal.namedsql") {
		err := factory.prepareNamed(name, sql)
		if err != nil {
			return nil, xerrors.Errorf("Failed to prepare sql query from config yaml. Section(internal.namedsql) Name(%s), SQL(%s). Err: %w", name, sql, err)
		}
	}
	for name, sql := range cfg.GetStringMapString("internal.sqlx") {
		err := factory.prepareSqlx(name, sql)
		if err != nil {
			return nil, xerrors.Errorf("Failed to prepare sql query from config yaml. Section(internal.sqlx) Name(%s), SQL(%s). Err: %w", name, sql, err)
		}
	}

	return factory, nil
}

// prepareNamed will insert prepared sql statements into the namedstmt cache
//  Prefer using NamedStmts if there are a lot of variables to interpolate into
//  a query. Be aware that there is a very small overhead as sqlx uses
//  reflection to pull the names. For *very* performance critical code, use
//  regular sqlx.Stmt via MRDFactory.prepareSqlx()
func (factory *MRDFactory) prepareNamed(name, sql string) error {
	_, ok := factory.preparedNamedSql[name]
	if !ok {
		insert, err := factory.database.PrepareNamed(sql)
		if err != nil {
			return xerrors.Errorf("Failed to prepare query(%s) with SQL (%s): %w", name, sql, err)
		}
		factory.preparedNamedSql[name] = insert
	}
	return nil
}

// getNamedSqlTx will pull a prepared statement and add it to the current transaction
func (factory *MRDFactory) getNamedSqlTx(tx *sqlx.Tx, name string) *sqlx.NamedStmt {
	return tx.NamedStmt(factory.preparedNamedSql[name])
}

// getNamedSql will return a prepared statement. Don't use this if you have a currently active transaction or you will deadlock!
func (factory *MRDFactory) getNamedSql(name string) *sqlx.NamedStmt {
	return factory.preparedNamedSql[name]
}

// prepareNamed will insert prepared sql statements into the stmt cache
func (factory *MRDFactory) prepareSqlx(name, sql string) error {
	_, ok := factory.preparedSql[name]
	if !ok {
		insert, err := factory.database.Preparex(sql)
		if err != nil {
			return xerrors.Errorf("Failed to prepare query(%s) with SQL (%s): %w", name, sql, err)
		}
		factory.preparedSql[name] = insert
	}
	return nil
}

// getNamedSqlTx will pull a prepared statement and add it to the current transaction
func (factory *MRDFactory) getSqlxTx(tx *sqlx.Tx, name string) *sqlx.Stmt {
	return tx.Stmtx(factory.preparedSql[name])
}

// getSqlx will return a prepared statement. Don't use this if you have a currently active transaction or you will deadlock!
func (factory *MRDFactory) getSqlx(name string) *sqlx.Stmt {
	return factory.preparedSql[name]
}

func (factory *MRDFactory) Delete(mrdEvData *MetricReportDefinitionData) (err error) {
	_, err = factory.getSqlx("delete_mrd").Exec(mrdEvData.Name)
	if err != nil {
		factory.logger.Crit("ERROR deleting MetricReportDefinition", "err", err, "Name", mrdEvData.Name)
	}
	delete(factory.NextMRTS, mrdEvData.Name)
	return
}

func ValidateMRD(MRD *MetricReportDefinition) {
	switch MRD.Type {
	case "Periodic":
		if MRD.Period < 5 || MRD.Period > (60*60*2) {
			MRD.Period = 180 // period can be 5s to 2hrs. if outside that range, make it 3 minutes.
		}
	case "OnChange", "OnRequest":
		MRD.Period = 0
	default:
		MRD.Type = "invalid"
		MRD.Period = 0
	}

}

func WrapWithTX(db *sqlx.DB, fn func(tx *sqlx.Tx) error) (err error) {
	// ===================================
	// Setup Transaction
	// ===================================
	tx, err := db.Beginx()
	if err != nil {
		return xerrors.Errorf("Transaction create failed: %w", err)
	}

	// if we error out at all, roll back
	defer tx.Rollback()

	err = fn(tx)
	if err != nil {
		return xerrors.Errorf("Failing Transaction because inner function returned error: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return xerrors.Errorf("FAILED transaction commit: %w", err)
	}

	return nil
}

func (factory *MRDFactory) UpdateMRD(mrdEvData *MetricReportDefinitionData) (err error) {
	return WrapWithTX(factory.database, func(tx *sqlx.Tx) error {
		newMRD := *mrdEvData
		MRD := &MetricReportDefinition{
			MetricReportDefinitionData: mrdEvData,
			AppendLimit:                3000,
		}

		ValidateMRD(MRD)

		// load the old values
		err = factory.loadReportDefinition(tx, MRD)
		if err != nil || MRD.ID == 0 {
			return xerrors.Errorf("Error getting MetricReportDefinition: ID(%s) NAME(%s) err: %w", MRD.ID, MRD.Name, err)
		}

		if newMRD.Type != "Periodic" && MRD.Enabled {
			delete(factory.NextMRTS, MRD.Name)
		} else if MRD.Period != newMRD.Period && MRD.Enabled {
			// periodic and report period changed, trigger report gen at new period
			factory.NextMRTS[MRD.Name] = metric.SqlTimeInt{Time: factory.MetricTSHWM.Add(time.Duration(newMRD.Period) * time.Second)}
		}

		_, err = factory.getNamedSqlTx(tx, "mrd_update").Exec(MetricReportDefinition{MetricReportDefinitionData: &newMRD, AppendLimit: 3000})
		if err != nil {
			return xerrors.Errorf("Error updating MRD(%+v): %w", mrdEvData, err)
		}

		err = factory.UpdateMMList(tx, MRD)
		if err != nil {
			return xerrors.Errorf("Error Updating MetricMeta for MRD(%+v): %w", MRD, err)
		}

		return nil
	})
}

func (factory *MRDFactory) AddMRD(mrdEvData *MetricReportDefinitionData) (err error) {
	return WrapWithTX(factory.database, func(tx *sqlx.Tx) error {
		MRD := &MetricReportDefinition{
			MetricReportDefinitionData: mrdEvData,
			AppendLimit:                3000,
		}

		ValidateMRD(MRD)

		// stop processing any periodic report gen for this report. restart IFF report successfully added back
		delete(factory.NextMRTS, MRD.Name)

		res, err := factory.getNamedSqlTx(tx, "mrd_insert").Exec(MRD)
		if err != nil {
			return xerrors.Errorf("Error inserting MRD(%s): %w", MRD, err)
		}
		MRD.ID, err = res.LastInsertId()
		if err != nil {
			return xerrors.Errorf("Couldnt get ID for inserted MRD(%+v): %w", MRD, err)
		}

		err = factory.UpdateMMList(tx, MRD)
		if err != nil {
			return xerrors.Errorf("Error Updating MetricMeta for MRD(%d): %w", MRD.ID, err)
		}

		// If this is a periodic report, put it in the NextMRTS map so it'll get updated on the next report period
		if MRD.Type == "Periodic" && MRD.Enabled {
			factory.NextMRTS[MRD.Name] = metric.SqlTimeInt{Time: factory.MetricTSHWM.Add(time.Duration(MRD.Period) * time.Second)}
		}

		return nil
	})
}

func (factory *MRDFactory) UpdateMMList(tx *sqlx.Tx, MRD *MetricReportDefinition) (err error) {
	//=================================================
	// Update the list of metrics for this report
	// First, just delete all the existing metric associations (not the actual MetricMeta, then we'll re-create
	_, err = factory.getSqlxTx(tx, "delete_mm_assoc").Exec(MRD.ID)
	if err != nil {
		return xerrors.Errorf("Error deleting rd2mm for MRD(%d): %w", MRD.ID, err)
	}

	// Then we will create each association one at a time
	for _, metric := range MRD.Metrics {
		var metaID int64
		var res sql.Result
		tempMetric := struct {
			*MRDMetric
			SuppressDups bool `db:"SuppressDups"`
		}{
			MRDMetric:    &metric,
			SuppressDups: MRD.SuppressDups,
		}

		// First, Find the MetricMeta
		err = factory.getNamedSqlTx(tx, "find_mm").Get(&metaID, tempMetric)
		if err != nil {
			if !xerrors.Is(err, sql.ErrNoRows) {
				return xerrors.Errorf("Error running query to find MetricMeta(%+v) for MRD(%s): %w", tempMetric, MRD, err)
			}
			// Insert new MetricMeta if it doesn't already exist per above
			res, err = factory.getNamedSqlTx(tx, "insert_mm").Exec(tempMetric)
			if err != nil {
				return xerrors.Errorf("Error inserting MetricMeta(%s) for MRD(%s): %w", tempMetric, MRD, err)
			}

			metaID, err = res.LastInsertId()
			if err != nil {
				return xerrors.Errorf("Error from LastInsertID for MetricMeta(%s): %w", tempMetric, err)
			}
		}

		// Next cross link MetricMeta to ReportDefinition
		_, err = factory.getSqlxTx(tx, "insert_mm_assoc").Exec(MRD.ID, metaID)
		if err != nil {
			return xerrors.Errorf("Error while inserting MetricMeta(%s) association for MRD(%s): %w", metric, MRD, err)
		}
	}
	return nil
}

var StopIter = xerrors.New("Stop Iteration")

// IterMRD will run fn() for every MRD in the DB.
//    TODO: pass in the TX to fn()
// 	  TO Consider: add an error return to allow rollback a single iter?
func (factory *MRDFactory) IterMRD(checkFn func(MRD *MetricReportDefinition) bool, fn func(MRD *MetricReportDefinition) error) error {
	return WrapWithTX(factory.database, func(tx *sqlx.Tx) error {
		// set up query for the MRD
		rows, err := factory.getSqlxTx(tx, "query_mrds").Queryx()
		if err != nil {
			return xerrors.Errorf("Query error for MRD: %w", err)
		}

		// iterate over everything the query returns
		for rows.Next() {
			MRD := &MetricReportDefinition{
				MetricReportDefinitionData: &MetricReportDefinitionData{},
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
		return nil
	})
}

func (l *MRDFactory) FastCheckForNeededMRUpdates() ([]string, error) {
	generatedList := []string{}
	for MRName, val := range l.NextMRTS {
		if l.MetricTSHWM.After(val.Time) {
			fmt.Printf("GEN - %s - ", MRName)
			err := l.GenerateMetricReport(MRName)
			if err != nil {
				fmt.Printf("ERROR: %s\n", err)
				continue
			}
			fmt.Printf("OK\n")
			generatedList = append(generatedList, MRName)
		}
	}
	return generatedList, nil
}

func (factory *MRDFactory) SlowCheckForNeededMRUpdates() ([]string, error) {
	factory.IterMRD(
		func(MRD *MetricReportDefinition) bool { return MRD.Type == "Periodic" },
		func(MRD *MetricReportDefinition) error {
			if _, ok := factory.NextMRTS[MRD.Name]; !ok {
				// setup nextmrts if it's not already in the list. Should never happen with current code structure
				fmt.Printf("Adding schedule for report. Shouldnt ever happen. Name(%s)\n", MRD.Name)
				factory.NextMRTS[MRD.Name] = metric.SqlTimeInt{}
			}
			return nil
		})
	return factory.FastCheckForNeededMRUpdates()
}

func (factory *MRDFactory) loadReportDefinition(tx *sqlx.Tx, MRD *MetricReportDefinition) error {
	var err error

	if MRD.ID > 0 {
		err = factory.getNamedSqlTx(tx, "find_mrd_by_id").Get(MRD, MRD)
	} else if len(MRD.Name) > 0 {
		err = factory.getNamedSqlTx(tx, "find_mrd_by_name").Get(MRD, MRD)
	} else {
		return xerrors.Errorf("Require either an ID or Name to load a Report Definition, didn't get either")
	}

	if err != nil {
		return xerrors.Errorf("Error loading Metric Report Definition %d:(%s) %w", MRD.ID, MRD.Name, err)
	}
	return nil
}

func (factory *MRDFactory) GenerateMetricReport(name string) (err error) {
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
		MetricReportDefinitionData: &MetricReportDefinitionData{Name: name},
	}
	err = factory.loadReportDefinition(tx, MRD)
	if err != nil || MRD.ID == 0 {
		return xerrors.Errorf("Error getting MetricReportDefinition: ID(%s) NAME(%s) err: %w", MRD.ID, MRD.Name, err)
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
		// FYI: adding a negative number, as ".Sub()" does something *completely different*.
		sqlargs["Start"] = factory.MetricTSHWM.Add(-time.Duration(MRD.Period) * time.Second).UnixNano()
		sqlargs["End"] = factory.MetricTSHWM.UnixNano()
		sqlargs["ReportTimestamp"] = factory.MetricTSHWM.UnixNano()
		factory.NextMRTS[MRD.Name] = metric.SqlTimeInt{Time: factory.MetricTSHWM.Add(time.Duration(MRD.Period) * time.Second)}

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
	*metric.MetricValueEventData
	ValueToWrite string `db:"Value"`

	// Meta fields
	Label              string        `db:"Label"`
	MetaID             int64         `db:"MetaID"`
	FQDDPattern        string        `db:"FQDDPattern"`
	SourcePattern      string        `db:"SourcePattern"`
	PropertyPattern    string        `db:"PropertyPattern"`
	Wildcards          string        `db:"Wildcards"`
	CollectionFunction string        `db:"CollectionFunction"`
	CollectionDuration time.Duration `db:"CollectionDuration"`

	// Instance fields
	ID                int64             `db:"ID"`
	InstanceID        int64             `db:"InstanceID"`
	CollectionScratch Scratch           `db:"CollectionScratch"`
	FlushTime         metric.SqlTimeInt `db:"FlushTime"`
	SuppressDups      bool              `db:"SuppressDups"`
	LastTS            metric.SqlTimeInt `db:"LastTS"`
	LastValue         string            `db:"LastValue"`
}

func (factory *MRDFactory) InsertMetricValue(ev *metric.MetricValueEventData) (err error) {
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
	rows, err := tx.NamedQuery(`
			select
				ID as MetaID, FQDDPattern, SourcePattern, PropertyPattern, Wildcards, CollectionFunction, CollectionDuration
			from MetricMeta
			where
				(:Name Like Name or Name is NULL or Name = '') and
				(:FQDD like FQDDPattern or FQDDPattern is NULL or FQDDPattern = '') and
				(:Source like SourcePattern or SourcePattern is NULL or SourcePattern = '') and
				(:Property like PropertyPattern or PropertyPattern is NULL or PropertyPattern = '')
				`, ev)
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

		// TODO: Implement more specific wildcard matching
		// TODO: Need to look up friendly fqdd (FOR LABEL)

		// Construct label and Scratch space
		if mm.CollectionFunction != "" {
			mm.Label = fmt.Sprintf("%s %s - %s", mm.Context, mm.Name, mm.CollectionFunction)
			mm.FlushTime = metric.SqlTimeInt{Time: mm.Timestamp.Add(mm.CollectionDuration * time.Second)}
			mm.CollectionScratch.Sum = 0
			mm.CollectionScratch.Numvalues = 0
			mm.CollectionScratch.Maximum = -math.MaxFloat64
			mm.CollectionScratch.Minimum = math.MaxFloat64
		} else {
			mm.Label = fmt.Sprintf("%s %s", mm.Context, mm.Name)
		}

		// create instances for each metric meta corresponding to this metric value
		_, err = tx.NamedExec(`
			INSERT INTO MetricInstance
				         ( MetaID,  Name,  FQDD,  Property,  Context,  Function,            Label,  CollectionScratch, LastValue, LastTS,  FlushTime)
				  VALUES (:MetaID, :Name, :FQDD, :Property, :Context, :CollectionFunction, :Label, :CollectionScratch, '',        0,      :FlushTime)
			`, mm)
		if err != nil {
			// It's ok if sqlite squawks about trying to insert dups here
			if !strings.HasPrefix(err.Error(), "UNIQUE constraint failed") {
				return xerrors.Errorf("Error inserting MetricInstance(%s): %w", mm, err)
			}
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
			MI.Name=:Name and
			MI.FQDD=:FQDD and
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
			factory.logger.Crit("Error scanning struct result for MetricInstance query", "err", err)
			continue
		}

		floatVal, floatErr := strconv.ParseFloat(mm.Value, 64)
		if floatErr != nil && mm.CollectionFunction != "" {
			saveValue = false
			saveInstance = true

			// has the period expired?
			if mm.Timestamp.After(mm.FlushTime.Time) {
				// Calculate what we should be dropping in the output
				saveValue = true
				factory.logger.Info("Collection period done Metric Instance", "Instance ID", mm.InstanceID, "CollectionFunction", mm.CollectionFunction)
				switch mm.CollectionFunction {
				case "Average":
					mm.Value = strconv.FormatFloat(mm.CollectionScratch.Sum/float64(mm.CollectionScratch.Numvalues), 'G', -1, 64)
				case "Maximum":
					mm.Value = strconv.FormatFloat(mm.CollectionScratch.Maximum, 'G', -1, 64)
				case "Minimum":
					mm.Value = strconv.FormatFloat(mm.CollectionScratch.Minimum, 'G', -1, 64)
				case "Summation":
					mm.Value = strconv.FormatFloat(mm.CollectionScratch.Sum, 'G', -1, 64)
				default:
					mm.Value = "Invalid or Unsupported CollectionFunction"
				}

				// now, reset everything
				// TODO: need a separate query to find all Metrics with HWM > FlushTime and flush them
				mm.FlushTime = metric.SqlTimeInt{Time: mm.Timestamp.Add(mm.CollectionDuration * time.Second)}
				mm.CollectionScratch.Sum = 0
				mm.CollectionScratch.Numvalues = 0
				mm.CollectionScratch.Maximum = -math.MaxFloat64
				mm.CollectionScratch.Minimum = math.MaxFloat64
			}

			// floatVal was saved, above.
			mm.CollectionScratch.Numvalues++
			mm.CollectionScratch.Sum += floatVal
			mm.CollectionScratch.Maximum = math.Max(floatVal, mm.CollectionScratch.Maximum)
			mm.CollectionScratch.Minimum = math.Min(floatVal, mm.CollectionScratch.Minimum)
		}

		if mm.SuppressDups && mm.LastValue == mm.Value {
			// No need to flush out new value, however instance may or may not need
			// flushing depending on above.
			saveValue = false
		}

		if saveValue {
			if mm.SuppressDups {
				mm.LastValue = mm.Value
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

				// re-use already parsed floatVal above
				if floatErr == nil {
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

func (factory *MRDFactory) DeleteOldestValues() (err error) {
	factory.logger.Info("Database Maintenance: Delete Oldest Metric Values")
	return WrapWithTX(factory.database, func(tx *sqlx.Tx) error {
		for _, sql := range factory.deleteops {
			_, err = factory.getSqlxTx(tx, sql).Exec()
			if err != nil {
				return xerrors.Errorf("Critical error performing delete from Metric Value table -> '%s': %w", sql, err)
			}
		}
		return nil
	})
}

func (factory *MRDFactory) Optimize() {
	factory.logger.Debug("Optimizing database - start")
	defer factory.logger.Debug("Optimizing database - done")
	_, err := factory.database.Exec("PRAGMA optimize")
	if err != nil {
		factory.logger.Crit("Problem optimizing database", "err", err)
	}
	_, err = factory.database.Exec("PRAGMA shrink_memory")
	if err != nil {
		factory.logger.Crit("Problem shrinking memory", "err", err)
	}
}

func (factory *MRDFactory) Vacuum() {
	factory.logger.Debug("Vacuuming database - start")
	defer factory.logger.Debug("Vacuuming database - done")
	_, err := factory.database.Exec("vacuum")
	if err != nil {
		factory.logger.Crit("Problem vacuuming database", "err", err)
	}
	_, err = factory.database.Exec("PRAGMA shrink_memory")
	if err != nil {
		factory.logger.Crit("Problem shrinking memory", "err", err)
	}
}
