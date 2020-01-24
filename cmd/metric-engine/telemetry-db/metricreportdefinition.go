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

type MetricReportDefinitionData struct {
	Name         string      `db:"Name"`
	Enabled      bool        `db:"Enabled"`
	Type         string      `db:"Type"` // 'Periodic', 'OnChange', 'OnRequest'
	SuppressDups bool        `db:"SuppressDups"`
	Actions      StringArray `db:"Actions"` // 	'LogToMetricReportsCollection', 'RedfishEvent'
	Updates      string      `db:"Updates"` // 'AppendStopsWhenFull', 'AppendWrapsWhenFull', 'NewReport', 'Overwrite'

	// Validation: It's assumed that TimeSpan is parsed on ingress. MRD Schema
	// specifies TimeSpan as a duration.
	// Represents number of seconds worth of metrics in a report. Metrics will be
	// reported from the Report generation as the "End" and metrics must have
	// timestamp > max(End-timespan, report start)
	TimeSpan int64 `db:"TimeSpan"`

	// Validation: It's assumed that Period is parsed on ingress. Redfish
	// "Schedule" object is flexible, but we'll allow only period in seconds for
	// now When it gets to this struct, it needs to be expressed in Seconds.
	Period  int64           `db:"Period"` // period in seconds when type=periodic
	Metrics []RawMetricMeta `db:"Metrics" json:"Metrics"`
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
	orphanops        []string
	optimizeops      []string
	vacuumops        []string

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
		deleteops:        cfg.GetStringSlice("sql_lists.deleteops"),
		orphanops:        cfg.GetStringSlice("sql_lists.orphanops"),
		optimizeops:      cfg.GetStringSlice("sql_lists.optimizeops"),
		vacuumops:        cfg.GetStringSlice("sql_lists.vacuumops"),
	}

	// SQLX can have SQL with '?' interpolation or ":Name" interpolation. They
	// are differnet datatypes, so instead of storing in an interface{} and type
	// assert, just have two differnt hashes to map prepared queries of each type

	// create prepared sql from yaml sql strings
	for name, sql := range cfg.GetStringMapString("internal.namedsql") {
		if strings.HasPrefix(name, "noop") {
			continue
		}
		err := factory.prepareNamed(name, sql)
		if err != nil {
			return nil, xerrors.Errorf("Failed to prepare sql query from config yaml. Section(internal.namedsql) Name(%s), SQL(%s). Err: %w", name, sql, err)
		}
	}
	// create prepared sql from yaml sql strings
	for name, sql := range cfg.GetStringMapString("internal.sqlx") {
		if strings.HasPrefix(name, "noop") {
			continue
		}
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

// getNamedSqlxTx will pull a prepared statement and add it to the current transaction
func (factory *MRDFactory) getNamedSqlxTx(tx *sqlx.Tx, name string) *sqlx.NamedStmt {
	return tx.NamedStmt(factory.getNamedSqlx(name))
}

// getNamedSqlxTx will return a prepared statement. It's prepared against the
// database directly. Don't use this if you have a currently active transaction
// or you will deadlock! (use getNamedSqlxTx())
func (factory *MRDFactory) getNamedSqlx(name string) *sqlx.NamedStmt {
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

// getNamedSqlxTx will pull a prepared statement and add it to the current transaction
func (factory *MRDFactory) getSqlxTx(tx *sqlx.Tx, name string) *sqlx.Stmt {
	return tx.Stmtx(factory.getSqlx(name))
}

// getSqlx will return a prepared statement. It was prepared directly against
// the databse. Don't use this if you have a currently active transaction or
// you will deadlock! (use getSqlxTx())
func (factory *MRDFactory) getSqlx(name string) *sqlx.Stmt {
	return factory.preparedSql[name]
}

// Delete will delete the requested MRD from the database
func (factory *MRDFactory) Delete(mrdEvData *MetricReportDefinitionData) (err error) {
	_, err = factory.getSqlx("delete_mrd").Exec(mrdEvData.Name)
	if err != nil {
		factory.logger.Crit("ERROR deleting MetricReportDefinition", "err", err, "Name", mrdEvData.Name)
	}
	delete(factory.NextMRTS, mrdEvData.Name)
	return
}

const (
	MinPeriod       = 5
	MaxPeriod       = 2 * 60 * 60
	DefaultPeriod   = 180
	MinTimeSpan     = 60
	MaxTimeSpan     = 4 * 60 * 60
	DefaultTimeSpan = MaxTimeSpan
)

// ValidateMRD: Validate Type, Period, and Timespan.
// - ensure the Type is valid enum
// - ensure Period is within allowed ranges for Periodic
// - ensure TimeSpan is set when required
func ValidateMRD(MRD *MetricReportDefinition) {
	PeriodMust := func(b bool) {
		if !b {
			MRD.Period = DefaultPeriod
		}
	}
	TimeSpanMust := func(b bool) {
		if !b {
			MRD.Period = DefaultPeriod
		}
	}

	// Globally: everything has to be within limits, or zero where allowed
	PeriodMust(MRD.Period <= MaxPeriod)
	PeriodMust(MRD.Period >= MinPeriod || MRD.Period == 0)
	TimeSpanMust(MRD.TimeSpan <= MaxTimeSpan)
	TimeSpanMust(MRD.TimeSpan >= MinTimeSpan || MRD.TimeSpan == 0)

	switch MRD.Type {
	case "Periodic":
		PeriodMust(MRD.Period > 0) // periodic reports must have nonzero period

		switch MRD.Updates {
		case "AppendWrapsWhenFull", "AppendStopsWhenFull":
			TimeSpanMust(MRD.TimeSpan > 0) // all Append* reports must have nonzero TimeSpan
		case "Overwrite":
		case "NewReport":
		}

	case "OnChange":
		MRD.Period = 0                            // Period must be zero for OnChange
		TimeSpanMust(MRD.TimeSpan >= MinTimeSpan) // OnChange requires nonzero TimeSpan

	case "OnRequest":
		// Implicitly force Updates/Actions/Period, validate TimeSpan
		MRD.Updates = "AppendWrapsWhenFull"
		MRD.Actions = []string{"LogToMetricReportsCollection"}
		MRD.Period = 0                            // Period must be zero for OnChange
		TimeSpanMust(MRD.TimeSpan >= MinTimeSpan) // OnRequest requires nonzero TimeSpan

	default:
		MRD.Type = "OnRequest"
		MRD.Enabled = false
		MRD.Period = 0
		MRD.TimeSpan = DefaultTimeSpan
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

func WrapWithTXOrPassIn(db *sqlx.DB, tx *sqlx.Tx, fn func(tx *sqlx.Tx) error) (err error) {
	if tx != nil {
		return fn(tx)
	}
	return WrapWithTX(db, fn)
}

func (factory *MRDFactory) WrapWithTX(fn func(tx *sqlx.Tx) error) error {
	return WrapWithTX(factory.database, fn)
}

func (factory *MRDFactory) WrapWithTXOrPassIn(tx *sqlx.Tx, fn func(tx *sqlx.Tx) error) error {
	return WrapWithTXOrPassIn(factory.database, tx, fn)
}

func (factory *MRDFactory) UpdateMRD(mrdEvData *MetricReportDefinitionData) (err error) {
	return factory.WrapWithTX(func(tx *sqlx.Tx) error {
		// TODO: Emit an error response message if the metric report definition does not exist

		newMRD := *mrdEvData
		MRD := &MetricReportDefinition{
			MetricReportDefinitionData: mrdEvData,
			AppendLimit:                24000,
		}

		ValidateMRD(MRD)

		// load the old values
		err = factory.loadReportDefinition(tx, MRD)
		if err != nil || MRD.ID == 0 {
			return xerrors.Errorf("Error getting MetricReportDefinition: ID(%s) NAME(%s) err: %w", MRD.ID, MRD.Name, err)
		}

		// if new type isn't periodic, delete nextmrts
		if newMRD.Type != "Periodic" {
			delete(factory.NextMRTS, MRD.Name)
		}

		_, err = factory.getNamedSqlxTx(tx, "mrd_update").Exec(MetricReportDefinition{MetricReportDefinitionData: &newMRD, AppendLimit: 3000})
		if err != nil {
			return xerrors.Errorf("Error updating MRD(%+v): %w", mrdEvData, err)
		}

		err = factory.UpdateMMList(tx, MRD)
		if err != nil {
			return xerrors.Errorf("Error Updating MetricMeta for MRD(%+v): %w", MRD, err)
		}

		if !MRD.Enabled {
			// we are done if report not enabled
			return nil
		}

		// insert the first (probably empty) report
		err = factory.InsertMetricReport(tx, MRD.Name)
		if err != nil {
			return xerrors.Errorf("Error Inserting Metric Report MRD(%+v): %w", MRD, err)
		}

		if MRD.Type == "Periodic" && MRD.Period != newMRD.Period {
			// If this is a periodic report, put it in the NextMRTS map so it'll get updated on the next report period
			factory.NextMRTS[MRD.Name] = metric.SqlTimeInt{Time: factory.MetricTSHWM.Add(time.Duration(newMRD.Period) * time.Second)}
		}

		return nil
	})
}

func (factory *MRDFactory) AddMRD(mrdEvData *MetricReportDefinitionData) (err error) {
	return factory.WrapWithTX(func(tx *sqlx.Tx) error {
		// TODO: Emit an error response message if the metric report definition already exists

		MRD := &MetricReportDefinition{
			MetricReportDefinitionData: mrdEvData,
			AppendLimit:                3000,
		}

		ValidateMRD(MRD)

		// stop processing any periodic report gen for this report. restart IFF report successfully added back
		delete(factory.NextMRTS, MRD.Name)

		res, err := factory.getNamedSqlxTx(tx, "mrd_insert").Exec(MRD)
		if err != nil {
			if strings.HasPrefix(err.Error(), "UNIQUE constraint failed") {
				// too verbose, but possibly uncomment for debugging
				//return xerrors.Errorf("Cannot ADD already existing MRD(%s).", MRD.Name)
				return nil
			}
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

		if !MRD.Enabled {
			return nil
		}

		// insert the first (probably empty) report
		err = factory.InsertMetricReport(tx, MRD.Name)
		if err != nil {
			return xerrors.Errorf("Error Inserting Metric Report MRD(%+v): %w", MRD, err)
		}

		// If this is a periodic report, put it in the NextMRTS map so it'll get updated on the next report period
		if MRD.Type == "Periodic" {
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

	// Set all metric instances dirty so we can pick up any new associations
	_, err = factory.getNamedSqlxTx(tx, "set_metric_instance_dirty").Exec(map[string]interface{}{})
	if err != nil {
		return xerrors.Errorf("Error setting metric instances dirty: %w", err)
	}

	// Then we will create each association one at a time
	for _, metric := range MRD.Metrics {
		var metaID int64
		var res sql.Result
		tempMetric := struct {
			*RawMetricMeta
			SuppressDups bool `db:"SuppressDups"`
		}{
			RawMetricMeta: &metric,
			SuppressDups:  MRD.SuppressDups,
		}

		// First, Find the MetricMeta
		err = factory.getNamedSqlxTx(tx, "find_mm").Get(&metaID, tempMetric)
		if err != nil && !xerrors.Is(err, sql.ErrNoRows) {
			return xerrors.Errorf("Error running query to find MetricMeta(%+v) for MRD(%s): %w", tempMetric, MRD, err)
		}

		if err != nil && xerrors.Is(err, sql.ErrNoRows) {
			// Insert new MetricMeta if it doesn't already exist per above
			res, err = factory.getNamedSqlxTx(tx, "insert_mm").Exec(tempMetric)
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

// IterMRD will run fn() for every MRD in the DB. Passes in a Transaction so function can update DB if needed
func (factory *MRDFactory) IterMRD(checkFn func(tx *sqlx.Tx, MRD *MetricReportDefinition) bool, fn func(tx *sqlx.Tx, MRD *MetricReportDefinition) error) error {
	return factory.WrapWithTX(func(tx *sqlx.Tx) error {
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
			if checkFn(tx, MRD) {
				err = fn(tx, MRD)
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

func (factory *MRDFactory) FastCheckForNeededMRUpdates() ([]string, error) {
	generatedList := []string{}
	for MRName, val := range factory.NextMRTS {
		if factory.MetricTSHWM.After(val.Time) {
			fmt.Printf("GEN - %s - ", MRName)
			err := factory.GenerateMetricReport(nil, MRName)
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

// SyncNextMRTSWithDB will clear the .NextMRTS cache and re-populate it
func (factory *MRDFactory) SyncNextMRTSWithDB() ([]string, error) {
	// scan through the database for enabled metric report definitions that are periodic and populate cache
	newMRTS := map[string]metric.SqlTimeInt{}
	factory.IterMRD(
		func(tx *sqlx.Tx, MRD *MetricReportDefinition) bool { return MRD.Type == "Periodic" && MRD.Enabled },
		func(tx *sqlx.Tx, MRD *MetricReportDefinition) error {
			if _, ok := newMRTS[MRD.Name]; !ok {
				newMRTS[MRD.Name] = metric.SqlTimeInt{}
			}
			return nil
		})
	// first, ensure that everything in current .NextMRTS is still alive. Delete if not.
	for k := range factory.NextMRTS {
		if _, ok := newMRTS[k]; ok {
			// delete from new if it already exists in old, simplifies next loop
			delete(newMRTS, k)
			continue
		}
		delete(factory.NextMRTS, k)
	}
	// next, pull in any new. newMRTS should only have new entries left
	for k, v := range newMRTS {
		factory.NextMRTS[k] = v
	}
	return factory.FastCheckForNeededMRUpdates()
}

func (factory *MRDFactory) loadReportDefinition(tx *sqlx.Tx, MRD *MetricReportDefinition) error {
	var err error

	if MRD.ID > 0 {
		err = factory.getSqlxTx(tx, "find_mrd_by_id").Get(MRD, MRD.ID)
	} else if len(MRD.Name) > 0 {
		err = factory.getSqlxTx(tx, "find_mrd_by_name").Get(MRD, MRD.Name)
	} else {
		return xerrors.Errorf("Require either an ID or Name to load a Report Definition, didn't get either")
	}

	if err != nil {
		return xerrors.Errorf("Error loading Metric Report Definition %d:(%s) %w", MRD.ID, MRD.Name, err)
	}
	return nil
}

func (factory *MRDFactory) InsertMetricReport(tx *sqlx.Tx, name string) (err error) {
	return factory.WrapWithTXOrPassIn(tx, func(tx *sqlx.Tx) error {
		MRD := &MetricReportDefinition{
			MetricReportDefinitionData: &MetricReportDefinitionData{Name: name},
		}
		err = factory.loadReportDefinition(tx, MRD)
		if err != nil || MRD.ID == 0 {
			return xerrors.Errorf("Error getting MetricReportDefinition: ID(%s) NAME(%s) err: %w", MRD.ID, MRD.Name, err)
		}

		sqlargs := map[string]interface{}{
			"Name":            MRD.Name,
			"MRDID":           MRD.ID,
			"ReportTimestamp": factory.MetricTSHWM.UnixNano(),
		}

		// Overwrite report name for NewReport
		if MRD.Updates == "NewReport" {
			// NO NEED TO INSERT INITIAL NULL REPORT, AS THIS WILL FIX ITSELF WHEN ITS GENERATED
			return nil
		}

		switch MRD.Type {
		case "Periodic":
			// FYI: using .Add() with a negative number, as ".Sub()" does something *completely different*.
			sqlargs["Start"] = factory.MetricTSHWM.Add(-time.Duration(MRD.Period) * time.Second).UnixNano()
			factory.NextMRTS[MRD.Name] = metric.SqlTimeInt{Time: factory.MetricTSHWM.Add(time.Duration(MRD.Period) * time.Second)}
		case "OnChange", "OnRequest":
			// FYI: using .Add() with a negative number, as ".Sub()" does something *completely different*.
			sqlargs["Start"] = factory.MetricTSHWM.Add(-time.Duration(MRD.TimeSpan) * time.Second).UnixNano()
		}

		// Delete all generated reports and reset everything
		_, err = factory.getNamedSqlxTx(tx, "delete_mr_by_id").Exec(sqlargs)
		if err != nil {
			return xerrors.Errorf("ERROR deleting MetricReport. MRD(%+v) args(%+v): %w", MRD, sqlargs, err)
		}

		// nothing left to do if it's not enabled
		if !MRD.Enabled {
			return nil
		}

		_, err = factory.getNamedSqlxTx(tx, "insert_report").Exec(sqlargs)
		if err != nil {
			return xerrors.Errorf("ERROR inserting MetricReport. MRD(%+v) args(%+v): %w", MRD, sqlargs, err)
		}

		return nil
	})
}

func (factory *MRDFactory) GenerateMetricReport(tx *sqlx.Tx, name string) (err error) {
	return factory.WrapWithTXOrPassIn(tx, func(tx *sqlx.Tx) error {
		MRD := &MetricReportDefinition{
			MetricReportDefinitionData: &MetricReportDefinitionData{Name: name},
		}
		err = factory.loadReportDefinition(tx, MRD)
		if err != nil || MRD.ID == 0 {
			return xerrors.Errorf("Error getting MetricReportDefinition: ID(%s) NAME(%s) err: %w", MRD.ID, MRD.Name, err)
		}

		// default to deleting all the reports... only actually does this if any params are invalid
		SQL := []string{"update_report_ts_seq"}
		sqlargs := map[string]interface{}{
			"Name":            MRD.Name,
			"MRDID":           MRD.ID,
			"ReportTimestamp": factory.MetricTSHWM.UnixNano(),
		}

		delete(factory.NextMRTS, MRD.Name)
		switch MRD.Type {
		case "Periodic":
			// FYI: using .Add() with a negative number, as ".Sub()" does something *completely different*.
			factory.NextMRTS[MRD.Name] = metric.SqlTimeInt{Time: factory.MetricTSHWM.Add(time.Duration(MRD.Period) * time.Second)}
			switch MRD.Updates {
			case "NewReport":
				SQL = []string{"insert_report", "keep_only_3_reports"}
				sqlargs["Start"] = factory.MetricTSHWM.Add(-time.Duration(MRD.Period) * time.Second).UnixNano()
				sqlargs["Name"] = fmt.Sprintf("%s-%s", MRD.Name, factory.MetricTSHWM.Time.UTC().Format(time.RFC3339))
			case "Overwrite":
				SQL = []string{"update_report_set_start_to_prev_timestamp", "update_report_ts_seq"}
			case "AppendWrapsWhenFull":
				SQL = []string{"update_report_ts_seq"}
			case "AppendStopsWhenFull":
				SQL = []string{"update_report_ts_seq"}
			}
		case "OnChange":
			switch MRD.Updates {
			case "NewReport":
				SQL = []string{"insert_report", "keep_only_3_reports"}
				sqlargs["Name"] = fmt.Sprintf("%s-%s", MRD.Name, factory.MetricTSHWM.Time.UTC().Format(time.RFC3339))
			case "Overwrite":
				SQL = []string{"update_report_start", "update_report_ts_seq"}
				// FYI: using .Add() with a negative number, as ".Sub()" does something *completely different*.
				sqlargs["Start"] = factory.MetricTSHWM.Add(-time.Duration(MRD.TimeSpan) * time.Second).UnixNano()
			case "AppendWrapsWhenFull":
				SQL = []string{"update_report_ts_seq"}
			case "AppendStopsWhenFull":
				SQL = []string{"update_report_ts_seq"}
			}
		default:
		}

		for _, sql := range SQL {
			fmt.Printf("SQL(%s) ", sql)
			_, err = factory.getNamedSqlxTx(tx, sql).Exec(sqlargs)
			if err != nil {
				return xerrors.Errorf("ERROR inserting MetricReport. MRD(%+v) sql(%s), args(%+v): %w", MRD, SQL, sqlargs, err)
			}
		}

		return nil
	})
}

func (factory *MRDFactory) CheckOnChangeReports(tx *sqlx.Tx, instancesUpdated map[int64]struct{}) error {
	err := factory.WrapWithTXOrPassIn(tx, func(tx *sqlx.Tx) error {
		for mmInstanceID := range instancesUpdated {
			instanceChangeList := []string{}
			err := factory.getSqlxTx(tx, "find_onchange_mrd_by_mm_instance").Select(&instanceChangeList, mmInstanceID)
			if err != nil {
				return xerrors.Errorf("Error getting changed reports by instance: %w", err)
			}
			for _, name := range instanceChangeList {
				if _, ok := factory.NextMRTS[name]; ok {
					continue
				}
				// ensure we generate report at most every 5 seconds by scheduling generation for 5s from now
				// Set up to generate report in 5
				factory.NextMRTS[name] = metric.SqlTimeInt{Time: factory.MetricTSHWM.Add(5 * time.Second)}
			}
		}
		return nil
	})

	return err
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

// Validation: It's assumed that Duration is parsed on ingress. The ingress
// format is (Redfish Duration): -?P(\d+D)?(T(\d+H)?(\d+M)?(\d+(.\d+)?S)?)?
// When it gets to this struct, it needs to be expressed in Seconds.
type RawMetricMeta struct {
	// Meta fields
	MetaID             int64         `db:"MetaID"`
	NamePattern        string        `db:"NamePattern" json:"MetricID"`
	FQDDPattern        string        `db:"FQDDPattern"`
	SourcePattern      string        `db:"SourcePattern"`
	PropertyPattern    string        `db:"PropertyPattern"`
	Wildcards          StringArray   `db:"Wildcards"`
	CollectionFunction string        `db:"CollectionFunction"`
	CollectionDuration time.Duration `db:"CollectionDuration"`
}

type RawMetricInstance struct {
	// Instance fields. Rest of the MetricInstance fields are in the MetricValue
	Label             string            `db:"Label"`
	InstanceID        int64             `db:"InstanceID"`
	CollectionScratch Scratch           `db:"CollectionScratch"`
	FlushTime         metric.SqlTimeInt `db:"FlushTime"`
	LastTS            metric.SqlTimeInt `db:"LastTS"`
	LastValue         string            `db:"LastValue"`
	Dirty             bool              `db:"Dirty"`
}

// Fusion structure: Meta + Instance + MetricValueEvent
type MetricMeta struct {
	*metric.MetricValueEventData
	*RawMetricMeta
	*RawMetricInstance
	ValueToWrite string `db:"Value"`
	SuppressDups bool   `db:"SuppressDups"`
}

func (factory *MRDFactory) InsertMetricInstance(tx *sqlx.Tx, ev *metric.MetricValueEventData) (instancesCreated int, err error) {
	err = factory.WrapWithTXOrPassIn(tx, func(tx *sqlx.Tx) error {
		rows, err := factory.getNamedSqlxTx(tx, "find_metric_meta").Queryx(ev)
		if err != nil {
			return xerrors.Errorf("Error querying for MetricMeta: %w", err)
		}

		// First, iterate over the MetricMeta to generate MetricInstance
		for rows.Next() {
			mm := &MetricMeta{MetricValueEventData: ev, RawMetricMeta: &RawMetricMeta{}, RawMetricInstance: &RawMetricInstance{}}
			err = rows.StructScan(mm)
			if err != nil {
				factory.logger.Crit("Error scanning metric meta for event", "err", err, "metric", ev)
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
			res, err := factory.getNamedSqlxTx(tx, "insert_metric_instance").Exec(mm)
			if err != nil {
				// It's ok if sqlite squawks about trying to insert dups here
				if !strings.HasPrefix(err.Error(), "UNIQUE constraint failed") {
					return xerrors.Errorf("Error inserting MetricInstance(%s): %w", mm, err)
				}
				err = factory.getNamedSqlxTx(tx, "find_metric_instance").Get(mm, mm)
				if err != nil {
					return xerrors.Errorf("Error getting MetricInstance(%s) InstanceID: %w", mm, err)
				}
			} else {
				mm.InstanceID, err = res.LastInsertId()
				if err != nil {
					return xerrors.Errorf("Error getting last insert ID for MetricInstance(%s): %w", mm, err)
				}
				instancesCreated++
			}
			_, err = factory.getSqlxTx(tx, "insert_mi_assoc").Exec(mm.MetaID, mm.InstanceID)
			if err != nil && !strings.HasPrefix(err.Error(), "UNIQUE constraint failed") {
				return xerrors.Errorf("Error inserting Association for MetricInstance(%s): %w", mm, err)
			}
			_, err = factory.getNamedSqlxTx(tx, "set_metric_instance_clean").Exec(mm)
			if err != nil {
				return xerrors.Errorf("Error setting metric instance (%d) clean: %w", mm.InstanceID, err)
			}
		}
		return nil
	})

	return instancesCreated, err
}

func (factory *MRDFactory) InsertMetricValue(tx *sqlx.Tx, ev *metric.MetricValueEventData, instancesUpdated map[int64]struct{}) (err error) {
	return factory.WrapWithTXOrPassIn(tx, func(tx *sqlx.Tx) error {
		// Optimized, so this logic isn't as straightforward as it could be.
		//
		// iterate over instances and insert
		// if we find an instance and it is clean, we know that we are done (ie. there are no reports with metric meta that we need to insert an instance for)
		// if we find an instance and it is dirty, we know a report has been added/updated and we potentially need to see if there is a metric meta we need to find and insert an instance for
		// if we do not find any instances, it could be because there is no report that contains this meta. Or, it could be because we simply haven't put in a metric instance for this value yet
		foundInstance, dirty, err := factory.doInsertMetricValue(tx, ev, instancesUpdated)
		if err != nil {
			return err
		}
		if dirty || !foundInstance {
			instancesCreated, err := factory.InsertMetricInstance(tx, ev)
			if err != nil {
				return err
			}
			if instancesCreated > 0 {
				_, _, err := factory.doInsertMetricValue(tx, ev, instancesUpdated)
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (factory *MRDFactory) doInsertMetricValue(tx *sqlx.Tx, ev *metric.MetricValueEventData, instancesUpdated map[int64]struct{}) (foundInstance, dirty bool, err error) {
	dirty = false
	foundInstance = false
	err = factory.WrapWithTXOrPassIn(tx, func(tx *sqlx.Tx) error {

		// And now, foreach MetricInstance, insert MetricValue
		mm := &MetricMeta{MetricValueEventData: ev, RawMetricMeta: &RawMetricMeta{}, RawMetricInstance: &RawMetricInstance{}}
		rows, err := factory.getNamedSqlxTx(tx, "iterate_metric_instance").Queryx(mm)
		if err != nil {
			return xerrors.Errorf("Error querying MetricInstance(%s): %w", mm, err)
		}

		for rows.Next() {
			saveValue := true
			saveInstance := false
			foundInstance = true

			mm := &MetricMeta{MetricValueEventData: ev, RawMetricMeta: &RawMetricMeta{}, RawMetricInstance: &RawMetricInstance{}}
			err = rows.StructScan(mm)
			if err != nil {
				factory.logger.Crit("Error scanning struct result for MetricInstance query", "err", err)
				continue
			}

			if mm.Dirty {
				dirty = true
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

				// report change hook. let caller know which instances were updated so they can look up reports
				instancesUpdated[mm.InstanceID] = struct{}{}

				var sql string
				var args []interface{}

				// Put into optimized tables, if possible. Try INT first, as it will error out for a float(1.0) value, but not vice versa
				intVal, err := strconv.ParseInt(mm.Value, 10, 64)
				if err == nil {
					sql = "insert_mv_int"
					args = []interface{}{mm.InstanceID, mm.Timestamp, intVal}

				} else if floatErr == nil { // re-use already parsed floatVal above
					sql = "insert_mv_real"
					args = []interface{}{mm.InstanceID, mm.Timestamp, floatVal}

				} else {
					sql = "insert_mv_text"
					args = []interface{}{mm.InstanceID, mm.Timestamp, mm.Value}
				}

				_, err = factory.getSqlxTx(tx, sql).Exec(args...)
				if err != nil {
					if !strings.HasPrefix(err.Error(), "UNIQUE constraint failed") {
						return xerrors.Errorf("Error inserting MetricValue for MetricInstance(%d)/MetricMeta(%d), ARGS: %+v: %w", mm.InstanceID, mm.MetaID, args, err)
					}
				}
			}

			if saveInstance {
				_, err = factory.getNamedSqlxTx(tx, "update_metric_instance").Exec(mm)
				if err != nil && !strings.HasPrefix(err.Error(), "UNIQUE constraint failed") {
					return xerrors.Errorf("Failed to update MetricInstance(%d) with MetricMeta(%d): %w", mm.InstanceID, mm.MetaID, err)
				}
			}
		}
		return nil
	})
	return foundInstance, dirty, err
}

func (factory *MRDFactory) runSQLFromList(sqllist []string, entrylog string, errorlog string) (err error) {
	factory.logger.Info(entrylog)
	return factory.WrapWithTX(func(tx *sqlx.Tx) error {
		for _, sql := range sqllist {
			_, err = factory.getSqlxTx(tx, sql).Exec()
			if err != nil {
				return xerrors.Errorf(errorlog, sql, err)
			}
		}
		return nil
	})
}

func (factory *MRDFactory) DeleteOrphans() (err error) {
	return factory.runSQLFromList(factory.orphanops, "Database Maintenance: Delete Orphans", "Orphan cleanup failed-> '%s': %w")
}

func (factory *MRDFactory) DeleteOldestValues() (err error) {
	return factory.runSQLFromList(factory.deleteops, "Database Maintenance: Delete Oldest Metric Values", "Value cleanup failed-> '%s': %w")
}

func (factory *MRDFactory) Vacuum() error {
	return factory.runSQLFromList(factory.vacuumops, "Database Maintenance: Vacuum", "Vacuum failed-> '%s': %w")
}

func (factory *MRDFactory) Optimize() error {
	return factory.runSQLFromList(factory.optimizeops, "Database Maintenance: Optimize", "Optimization failed-> '%s': %w")
}
