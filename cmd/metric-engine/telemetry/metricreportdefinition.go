package telemetry

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"database/sql"

	"github.com/jmoiron/sqlx"
	"github.com/spf13/viper"
	"golang.org/x/xerrors"

	"github.com/superchalupa/sailfish/cmd/metric-engine/metric"
	log "github.com/superchalupa/sailfish/src/log"
)

// StopIter is the specific error to raise to cause iteration to stop without returning an error
const StopIter = constErr("Stop Iteration")

// configuration consts... can be overridden in config file
const (
	appendLimit             = 24000
	smallestReportInterval  = 5 * time.Second
	maxMetricExpandInterval = 1 * time.Hour
	maxAcceptableDrift      = 2 * time.Second
)

// Factory manages getting/putting into db
type telemetryManager struct {
	logger           log.Logger
	database         *sqlx.DB
	preparedNamedSQL map[string]*sqlx.NamedStmt
	preparedSQL      map[string]*sqlx.Stmt
	deleteops        []string
	orphanops        []string
	optimizeops      []string
	vacuumops        []string

	MetricTSHWM   time.Time            // high water mark for received metrics
	NextMRTS      map[string]time.Time // next timestamp where we need to generate a report
	LastMRTS      map[string]time.Time // last report timestamp
	PendingInsert map[string]bool

	// config items
	AppendLimit             int
	SmallestReportInterval  time.Duration
	MaxMetricExpandInterval time.Duration
	MaxAcceptableDrift      time.Duration
}

func setCfgDefaults(cfg *viper.Viper) {
	// set up programatic defaults based on the constants in this file that can be overridden in config file
	cfg.SetDefault("defaults.appendlimit", appendLimit)
	cfg.SetDefault("defaults.smallestreportinterval", int64(smallestReportInterval))
	cfg.SetDefault("defaults.maxmetricexpandinterval", int64(maxMetricExpandInterval))
	cfg.SetDefault("defaults.maxacceptabledrift", int64(maxAcceptableDrift))
	cfg.SetDefault("sql_lists.deleteops", []string{
		"delete_old_mr",
		"delete_old_mvi",
		"delete_old_mvr",
		"delete_old_mvt",
		"delete_mvi",
		"delete_mvr",
		"delete_mvt",
		"shrink",
	})
	cfg.SetDefault("sql_lists.orphanops", []string{"delete_orphan_metricinstance", "delete_orphan_metricmeta"})
	cfg.SetDefault("sql_lists.optimizeops", []string{"optimize", "shrink"})
	cfg.SetDefault("sql_lists.vacuumops", []string{"vacuum", "shrink"})
}

// newTelemetryManager is the constructor for the base telemetry service functionality
func newTelemetryManager(logger log.Logger, database *sqlx.DB, cfg *viper.Viper) (*telemetryManager, error) {
	// make sure not to store the 'cfg' passed in. That would be Bad.
	setCfgDefaults(cfg)

	factory := &telemetryManager{
		logger:                  logger,
		database:                database,
		NextMRTS:                map[string]time.Time{},
		LastMRTS:                map[string]time.Time{},
		PendingInsert:           map[string]bool{},
		preparedNamedSQL:        map[string]*sqlx.NamedStmt{},
		preparedSQL:             map[string]*sqlx.Stmt{},
		deleteops:               cfg.GetStringSlice("sql_lists.deleteops"),
		orphanops:               cfg.GetStringSlice("sql_lists.orphanops"),
		optimizeops:             cfg.GetStringSlice("sql_lists.optimizeops"),
		vacuumops:               cfg.GetStringSlice("sql_lists.vacuumops"),
		AppendLimit:             cfg.GetInt("defaults.appendlimit"),
		SmallestReportInterval:  time.Duration(cfg.GetInt64("defaults.smallestreportinterval")),
		MaxMetricExpandInterval: time.Duration(cfg.GetInt64("defaults.maxmetricexpandinterval")),
		MaxAcceptableDrift:      time.Duration(cfg.GetInt64("defaults.maxacceptabledrift")),
	}

	// SQLX can have SQL with '?' interpolation or ":Name" interpolation. They
	// are differnet datatypes, so instead of storing in an interface{} and type
	// assert, just have two different hashes to map prepared queries of each type

	// create prepared NAMED sql from yaml sql strings
	for name, sql := range cfg.GetStringMapString("internal.namedsql") {
		if strings.HasPrefix(name, "noop") {
			continue
		}
		err := factory.prepareNamed(name, sql)
		if err != nil {
			return nil, xerrors.Errorf(
				"Failed to prepare sql query from config yaml. Section(internal.namedsql) Name(%s), SQL(%s). Err: %w",
				name, sql, err)
		}
	}
	// create prepared sql from yaml sql strings
	for name, sql := range cfg.GetStringMapString("internal.sqlx") {
		if strings.HasPrefix(name, "noop") {
			continue
		}
		err := factory.prepareSQLX(name, sql)
		if err != nil {
			return nil, xerrors.Errorf(
				"Failed to prepare sql query from config yaml. Section(internal.sqlx) Name(%s), SQL(%s). Err: %w",
				name, sql, err)
		}
	}

	// sync internal state (.NextMRTS, mainly) from what may already be in DB
	err := factory.SyncInternalStateFromDB()
	if err != nil {
		return nil, xerrors.Errorf("telemetry sync from DB failed: %w", err)
	}

	return factory, nil
}

// prepareNamed will insert prepared sql statements into the namedstmt cache
//  Prefer using NamedStmts if there are a lot of variables to interpolate into
//  a query. Be aware that there is a very small overhead as sqlx uses
//  reflection to pull the names. For *very* performance critical code, use
//  regular sqlx.Stmt via telemetryManager.prepareSQLX()
func (factory *telemetryManager) prepareNamed(name, sql string) error {
	_, ok := factory.preparedNamedSQL[name]
	if !ok {
		insert, err := factory.database.PrepareNamed(sql)
		if err != nil {
			return xerrors.Errorf("Failed to prepare query(%s) with SQL (%s): %w", name, sql, err)
		}
		factory.preparedNamedSQL[name] = insert
	}
	return nil
}

// getNamedSQLXTx will pull a prepared statement and add it to the current transaction
func (factory *telemetryManager) getNamedSQLXTx(tx *sqlx.Tx, name string) *sqlx.NamedStmt {
	return tx.NamedStmt(factory.getNamedSQLX(name))
}

// getNamedSQLXTx will return a prepared statement. It's prepared against the
// database directly. Don't use this if you have a currently active transaction
// or you will deadlock! (use getNamedSQLXTx())
func (factory *telemetryManager) getNamedSQLX(name string) *sqlx.NamedStmt {
	sql, ok := factory.preparedNamedSQL[name]
	if !ok {
		panic(fmt.Sprintf("getNamedSQLX(%s) -> nonexistent! Probably: typo or out of date config.", name))
	}
	return sql
}

// prepareNamed will insert prepared sql statements into the stmt cache
func (factory *telemetryManager) prepareSQLX(name, sql string) error {
	_, ok := factory.preparedSQL[name]
	if !ok {
		insert, err := factory.database.Preparex(sql)
		if err != nil {
			return xerrors.Errorf("Failed to prepare query(%s) with SQL (%s): %w", name, sql, err)
		}
		factory.preparedSQL[name] = insert
	}
	return nil
}

// getNamedSQLXTx will pull a prepared statement and add it to the current transaction
func (factory *telemetryManager) getSQLXTx(tx *sqlx.Tx, name string) *sqlx.Stmt {
	return tx.Stmtx(factory.getSQLX(name))
}

// getSQLX will return a prepared statement. It was prepared directly against
// the databse. Don't use this if you have a currently active transaction or
// you will deadlock! (use getSQLXTx())
func (factory *telemetryManager) getSQLX(name string) *sqlx.Stmt {
	sql, ok := factory.preparedSQL[name]
	if !ok {
		panic(fmt.Sprintf("getSQLX(%s) -> nonexistent! Probably: typo or out of date config.", name))
	}
	return sql
}

// deleteMRD will delete the requested MRD from the database
func (factory *telemetryManager) deleteMRD(reportDefName string) (err error) {
	_, err = factory.getSQLX("delete_mrd").Exec(reportDefName)
	if err != nil {
		factory.logger.Crit("ERROR deleting MetricReportDefinition", "err", err, "Name", reportDefName)
	}
	delete(factory.NextMRTS, reportDefName)
	delete(factory.LastMRTS, reportDefName)
	return
}

// deleteMR will delete the requested Report from the database
func (factory *telemetryManager) deleteMR(reportName string) (err error) {
	_, err = factory.getSQLX("delete_mr").Exec(reportName)
	if err != nil {
		factory.logger.Crit("ERROR deleting MetricReport", "err", err, "Name", reportName)
	}
	return
}

// "Configuration" constants that TODO need to move to be read from cfg file
const (
	MinPeriod       = 5 * time.Second
	MaxPeriod       = 2 * time.Hour
	DefaultPeriod   = 5 * time.Minute
	MinTimeSpan     = 60 * time.Second
	MaxTimeSpan     = 4 * time.Hour
	DefaultTimeSpan = (RedfishDuration)(MaxTimeSpan)
)

// validateMRD will validate (and fix/set defaults) for MRD -> Validate Type, Period, and Timespan.
// - ensure the Type is valid enum
// - ensure Period is within allowed ranges for Periodic
// - ensure TimeSpan is set when required
func validateMRD(mrd *MetricReportDefinition) error {
	if mrd.Name == "" {
		fmt.Printf("EMPTY NAME\n")
	}

	PeriodMust := func(b bool) {
		if !b {
			mrd.Period = RedfishDuration(DefaultPeriod)
		}
	}
	TimeSpanMust := func(b bool) {
		if !b {
			mrd.TimeSpan = DefaultTimeSpan
		}
	}

	// Globally: everything has to be within limits, or zero where allowed
	PeriodMust(mrd.Period.Duration() <= MaxPeriod)
	PeriodMust(mrd.Period.Duration() >= MinPeriod || mrd.Period.Duration() == 0)
	TimeSpanMust(mrd.GetTimeSpan() <= MaxTimeSpan)
	TimeSpanMust(mrd.GetTimeSpan() >= MinTimeSpan || mrd.GetTimeSpan() == 0)

	switch mrd.Type {
	case metric.Periodic:
		PeriodMust(mrd.Period.Duration() > 0) // periodic reports must have nonzero period

		switch mrd.Updates {
		case metric.AppendWrapsWhenFull, metric.AppendStopsWhenFull:
			TimeSpanMust(mrd.GetTimeSpan() > 0) // all Append* reports must have nonzero TimeSpan
		case metric.Overwrite, metric.NewReport:
			// all good
		default: //invalid 'ReportUpdates'
			mrd.Updates = metric.Overwrite
		}

	case metric.OnChange:
		mrd.Period = RedfishDuration(0)                // Period must be zero for OnChange
		TimeSpanMust(mrd.GetTimeSpan() >= MinTimeSpan) // OnChange requires nonzero TimeSpan
		switch mrd.Updates {
		case metric.AppendWrapsWhenFull, metric.AppendStopsWhenFull, metric.Overwrite, metric.NewReport:
			// all good
		default: //invalid 'ReportUpdates'
			mrd.Updates = metric.Overwrite
		}

	case metric.OnRequest:
		// Implicitly force Updates/Actions/Period, validate TimeSpan
		mrd.Updates = metric.AppendWrapsWhenFull
		mrd.Actions = []string{}
		mrd.Period = RedfishDuration(0)                // Period must be zero for OnChange
		TimeSpanMust(mrd.GetTimeSpan() >= MinTimeSpan) // OnRequest requires nonzero TimeSpan

	default:
		mrd.Type = metric.OnRequest
		mrd.Enabled = false
		mrd.Period = RedfishDuration(0)
		mrd.TimeSpan = DefaultTimeSpan
	}

	return nil
}

func wrapWithTX(db *sqlx.DB, fn func(tx *sqlx.Tx) error) (err error) {
	// ===================================
	// Setup Transaction
	// ===================================
	tx, err := db.Beginx()
	if err != nil {
		return xerrors.Errorf("Transaction create failed: %w", err)
	}

	// if we error out at all, roll back
	// if an error happens, that's really weird and there isn't much we can do
	// about it. Probably have some database inconsistency, so panic() seems like
	// the only real option. Telemetry service should restart in this case
	committed := false
	defer func() {
		if committed {
			return
		}
		err := tx.Rollback()
		if err != nil {
			panic(fmt.Sprintf("database rollback failed(%T), something is very wrong: %s", err, err))
		}
	}()

	err = fn(tx)
	if err != nil {
		return xerrors.Errorf("Rollback transaction: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return xerrors.Errorf("FAILED transaction commit: %w", err)
	}
	committed = true

	return nil
}

func wrapWithTXOrPassIn(db *sqlx.DB, tx *sqlx.Tx, fn func(tx *sqlx.Tx) error) (err error) {
	if tx != nil {
		return fn(tx)
	}
	return wrapWithTX(db, fn)
}

func (factory *telemetryManager) wrapWithTX(fn func(tx *sqlx.Tx) error) error {
	return wrapWithTX(factory.database, fn)
}

func (factory *telemetryManager) wrapWithTXOrPassIn(tx *sqlx.Tx, fn func(tx *sqlx.Tx) error) error {
	return wrapWithTXOrPassIn(factory.database, tx, fn)
}

func (factory *telemetryManager) get(cmd *GenericGETCommandData, resp *GenericGETResponseData) error {
	defer close(resp.dataChan)
	// this function seems too cozy with HTTP redfish interface, but no direct http dependencies

	data := []byte{}
	err := factory.getSQLX("generic_get").Get(&data, cmd.URI)
	if err != nil {
		factory.logger.Crit("error querying JSON table for URI", "err", err, "URI", cmd.URI)
		resp.SetStatus(metric.HTTPStatusNotFound)

		// TODO: need a body with eemi error here
		resp.dataChan <- []byte("Resource not found. (FIXME: replace with redfish compliant error text.)")
		return err
	}

	// on $expand, we'd need to parse all the return data and expand @odata.id's
	// kind of a pain in the backside

	resp.SetStatus(metric.HTTPStatusOk)
	resp.dataChan <- data
	return nil
}

func (factory *telemetryManager) updateMRD(reportDef string, updates json.RawMessage) (err error) {
	return factory.wrapWithTX(func(tx *sqlx.Tx) error {
		// TODO: Emit an error response message if the metric report definition does not exist

		// step 1: LOAD existing report definition
		mrd := MetricReportDefinition{
			MetricReportDefinitionData: &MetricReportDefinitionData{Name: reportDef},
			AppendLimit:                factory.AppendLimit,
		}
		err = factory.loadReportDefinition(tx, &mrd)
		if err != nil || mrd.ID == 0 {
			return xerrors.Errorf("error getting MetricReportDefinition: ID(%d) NAME(%s) err: %w", mrd.ID, mrd.Name, err)
		}
		var newData MetricReportDefinitionData = *mrd.MetricReportDefinitionData
		newMRD := MetricReportDefinition{MetricReportDefinitionData: &newData, AppendLimit: factory.AppendLimit, ID: mrd.ID}

		// Step 2: apply updates specified
		err = json.Unmarshal(updates, &newMRD)
		if err != nil {
			return err
		}
		newMRD.Name = reportDef // ensure name stays the same... should be no way this isn't the same, but lets be sure.

		// step 3: validate result
		err = validateMRD(&newMRD)
		if err != nil {
			return err
		}

		// step 4: update internal bookkeeping tables
		if newMRD.Type != metric.Periodic {
			delete(factory.NextMRTS, reportDef)
		}

		// step 5: save new report
		_, err = factory.getNamedSQLXTx(tx, "mrd_update").Exec(newMRD)
		if err != nil {
			return xerrors.Errorf("error updating MRD(%s): %s --ERR--> %w", reportDef, updates, err)
		}

		err = factory.updateMMList(tx, &newMRD)
		if err != nil {
			return xerrors.Errorf("error updating MetricMeta for MRD(%s): %s --ERR--> %w", reportDef, updates, err)
		}

		if !newMRD.Enabled {
			// we are done if report not enabled
			return nil
		}

		// let the next clock tick trigger generations and insert
		factory.PendingInsert[newMRD.Name] = true
		factory.NextMRTS[newMRD.Name] = time.Time{}

		if newMRD.Type == metric.Periodic && mrd.Period != newMRD.Period {
			// If this is a periodic report, put it in the NextMRTS map so it'll get updated on the next report period
			factory.NextMRTS[newMRD.Name] = factory.MetricTSHWM.Add(newMRD.Period.Duration())
		}

		return nil
	})
}

func (factory *telemetryManager) addMRD(mrdEvData *MetricReportDefinitionData) (err error) {
	return factory.wrapWithTX(func(tx *sqlx.Tx) error {
		// TODO: Emit an error response message if the metric report definition already exists
		mrd := &MetricReportDefinition{
			MetricReportDefinitionData: mrdEvData,
			AppendLimit:                factory.AppendLimit,
		}

		err = validateMRD(mrd)
		if err != nil {
			return err
		}

		res, err := factory.getNamedSQLXTx(tx, "mrd_insert").Exec(mrd)
		if err != nil {
			if strings.HasPrefix(err.Error(), "UNIQUE constraint failed") {
				// too verbose, but possibly uncomment for debugging
				return xerrors.Errorf("cannot ADD already existing MRD(%s).", mrd.Name)
			}
			return xerrors.Errorf("error inserting MRD(%s): %w", mrd, err)
		}
		mrd.ID, err = res.LastInsertId()
		if err != nil {
			return xerrors.Errorf("couldnt get ID for inserted MRD(%+v): %w", mrd, err)
		}

		err = factory.updateMMList(tx, mrd)
		if err != nil {
			return xerrors.Errorf("error Updating MetricMeta for MRD(%d): %w", mrd.ID, err)
		}

		// stop processing any periodic report gen for this report. restart IFF report successfully added back
		// do this after the mrd_insert sql query above so we dont stop processing reports just because of a constraint violation
		delete(factory.NextMRTS, mrd.Name)

		if !mrd.Enabled {
			return nil
		}

		// insert the first (probably empty) report
		factory.PendingInsert[mrd.Name] = true
		factory.NextMRTS[mrd.Name] = time.Time{}

		// If this is a periodic report, put it in the NextMRTS map so it'll get updated on the next report period
		if mrd.Type == metric.Periodic {
			factory.NextMRTS[mrd.Name] = factory.MetricTSHWM.Add(mrd.Period.Duration())
		}

		return nil
	})
}

func (factory *telemetryManager) addMD(mdEvData *MetricDefinitionData) (err error) {
	return factory.wrapWithTX(func(tx *sqlx.Tx) error {
		//TODO: valid MD input, for now all good input read from files
		//validateMD(md)

		_, err := factory.getNamedSQLXTx(tx, "md_insert").Exec(mdEvData)
		if err != nil {
			if strings.HasPrefix(err.Error(), "UNIQUE constraint failed") {
				return xerrors.Errorf("cannot ADD already existing MD(%s).", mdEvData.MetricID)
			}
			return xerrors.Errorf("error inserting MD(%s): %w", mdEvData.MetricID, err)
		}

		return nil
	})
}

func (factory *telemetryManager) updateMMList(tx *sqlx.Tx, mrd *MetricReportDefinition) (err error) {
	//=================================================
	// Update the list of metrics for this report
	// First, just delete all the existing metric associations (not the actual MetricMeta, then we'll re-create
	_, err = factory.getSQLXTx(tx, "delete_mm_assoc").Exec(mrd.ID)
	if err != nil {
		return xerrors.Errorf("error deleting rd2mm for MRD(%d): %w", mrd.ID, err)
	}

	// Set all metric instances dirty so we can pick up any new associations
	_, err = factory.getNamedSQLXTx(tx, "set_metric_instance_dirty").Exec(map[string]interface{}{})
	if err != nil {
		return xerrors.Errorf("error setting metric instances dirty: %w", err)
	}

	// Then we will create each association one at a time
	for _, metricToAdd := range mrd.Metrics {
		var metaID int64
		var res sql.Result
		tempMetric := struct {
			RawMetricMeta
			SuppressDups bool `db:"SuppressDups"`
		}{
			RawMetricMeta: metricToAdd,
			SuppressDups:  mrd.SuppressDups,
		}

		// First, Find the MetricMeta
		err = factory.getNamedSQLXTx(tx, "find_mm").Get(&metaID, tempMetric)
		if err != nil && !xerrors.Is(err, sql.ErrNoRows) {
			return xerrors.Errorf("error running query to find MetricMeta(%+v) for MRD(%s): %w", tempMetric, mrd, err)
		}

		if err != nil && xerrors.Is(err, sql.ErrNoRows) {
			// Insert new MetricMeta if it doesn't already exist per above
			res, err = factory.getNamedSQLXTx(tx, "insert_mm").Exec(tempMetric)
			if err != nil {
				return xerrors.Errorf("error inserting MetricMeta(%s) for MRD(%s): %w", tempMetric, mrd, err)
			}

			metaID, err = res.LastInsertId()
			if err != nil {
				return xerrors.Errorf("error from LastInsertID for MetricMeta(%s): %w", tempMetric, err)
			}
		}

		// Next cross link MetricMeta to ReportDefinition
		_, err = factory.getSQLXTx(tx, "insert_mm_assoc").Exec(mrd.ID, metaID)
		if err != nil {
			return xerrors.Errorf("error while inserting MetricMeta(%s) association for MRD(%s): %w", metricToAdd, mrd, err)
		}
	}
	return nil
}

// iterMRD will run fn() for every MRD in the DB. Passes in a Transaction so function can update DB if needed
func (factory *telemetryManager) iterMRD(
	checkFn func(tx *sqlx.Tx, mrd *MetricReportDefinition) bool,
	fn func(tx *sqlx.Tx, mrd *MetricReportDefinition) error) error {
	return factory.wrapWithTX(func(tx *sqlx.Tx) error {
		// set up query for the MRD
		rows, err := factory.getSQLXTx(tx, "query_mrds").Queryx()
		if err != nil {
			return xerrors.Errorf("query error for MRD: %w", err)
		}

		mrd := MetricReportDefinition{
			MetricReportDefinitionData: &MetricReportDefinitionData{},
		}

		// iterate over everything the query returns
		for rows.Next() {
			err = rows.StructScan(&mrd)
			if err != nil {
				return xerrors.Errorf("scan error: %w", err)
			}
			if checkFn(tx, &mrd) {
				err = fn(tx, &mrd)
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

func (factory *telemetryManager) FastCheckForNeededMRUpdates() ([]string, error) {
	generatedList := []string{}
	if drift := time.Since(factory.MetricTSHWM); drift > factory.MaxAcceptableDrift || drift < -factory.MaxAcceptableDrift {
		// TODO: FIXME - need a strategy to deal with this before we ship
		// here is where we could reset things... possible time change detected?
		factory.logger.Crit("Wall clock MetricTSHWM discrepancy", "drift", drift, "now", time.Now(), "MetricTSHWM", factory.MetricTSHWM)
	}
	for MRName, val := range factory.NextMRTS {
		if factory.MetricTSHWM.After(val) {
			err := factory.GenerateMetricReport(nil, MRName)
			if err != nil {
				factory.logger.Info("error generating metric report", "report-name", MRName, "err", err)
				continue
			}
			generatedList = append(generatedList, MRName)
			factory.logger.Info("Generated Metric Report", "report-name", MRName)
		}
	}
	return generatedList, nil
}

// SyncInternalStateFromDB will clear the .NextMRTS cache and re-populate it
func (factory *telemetryManager) SyncInternalStateFromDB() error {
	// scan through the database for enabled metric report definitions that are periodic and populate cache
	newMRTS := map[string]time.Time{}
	err := factory.iterMRD(
		func(tx *sqlx.Tx, mrd *MetricReportDefinition) bool { return mrd.Type == metric.Periodic && mrd.Enabled },
		func(tx *sqlx.Tx, mrd *MetricReportDefinition) error {
			if _, ok := newMRTS[mrd.Name]; !ok {
				newMRTS[mrd.Name] = time.Time{}
			}
			return nil
		})
	if err != nil {
		factory.logger.Crit("Error scanning through database metric report definitions", "err", err)
	}

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

	// TODO: need to set PendingInsert on any MRD that doesn't have an MR

	return nil
}

func (factory *telemetryManager) loadReportDefinition(tx *sqlx.Tx, mrd *MetricReportDefinition) error {
	var err error

	switch {
	case mrd.ID > 0:
		err = factory.getSQLXTx(tx, "find_mrd_by_id").Get(mrd, mrd.ID)
	case len(mrd.Name) > 0:
		err = factory.getSQLXTx(tx, "find_mrd_by_name").Get(mrd, mrd.Name)
	default:
		err = xerrors.Errorf("require either an ID or Name to load a Report Definition, didn't get either")
	}

	if err != nil {
		return xerrors.Errorf("error loading Metric Report Definition %d:(%s) %w", mrd.ID, mrd.Name, err)
	}
	return nil
}

// nolint: funlen,gocognit    //  this is all straight line code without a lot of
// conditionals. Splitting this up would likely impact readability so not
// splitting this up at this time
func (factory *telemetryManager) GenerateMetricReport(tx *sqlx.Tx, name string) (err error) {
	return factory.wrapWithTXOrPassIn(tx, func(tx *sqlx.Tx) error {
		MRD := &MetricReportDefinition{
			MetricReportDefinitionData: &MetricReportDefinitionData{Name: name},
		}
		err = factory.loadReportDefinition(tx, MRD)
		if err != nil || MRD.ID == 0 {
			return xerrors.Errorf("error getting MetricReportDefinition: ID(%s) NAME(%s) err: %w", MRD.ID, MRD.Name, err)
		}

		SQL := []string{}
		restorePendingInsert := false
		if factory.PendingInsert[MRD.Name] {
			restorePendingInsert = true
			delete(factory.PendingInsert, MRD.Name)
			// redo the report insert the next time around if we error out of this function
			defer func() {
				if restorePendingInsert {
					factory.PendingInsert[MRD.Name] = true
				}
			}()
			SQL = append(SQL, "delete_mr_by_id")
			SQL = append(SQL, "insert_report")
		}

		sqlargs := map[string]interface{}{
			"Name":            MRD.Name,
			"MRDID":           MRD.ID,
			"ReportTimestamp": factory.MetricTSHWM.UnixNano(),
		}

		delete(factory.NextMRTS, MRD.Name)
		switch MRD.Type {
		case metric.Periodic:
			factory.NextMRTS[MRD.Name] = factory.MetricTSHWM.Add(MRD.Period.Duration())
			sqlargs["Start"] = factory.MetricTSHWM.Add(-MRD.Period.Duration()).UnixNano()
			switch MRD.Updates {
			case metric.NewReport:
				sqlargs["Name"] = fmt.Sprintf("%s-%s", MRD.Name, factory.MetricTSHWM.UTC().Format(time.RFC3339))
				SQL = []string{"insert_report", "keep_only_3_reports"}
			case metric.Overwrite:
				SQL = append(SQL, "update_report_set_start_to_prev_timestamp")
				SQL = append(SQL, "update_report_ts_seq")
			case metric.AppendWrapsWhenFull: // default sql list is ok
				SQL = append(SQL, "update_report_ts_seq")
			case metric.AppendStopsWhenFull: // default sql list is ok
				SQL = append(SQL, "update_report_ts_seq")
			}
		case metric.OnChange, metric.OnRequest:
			sqlargs["Start"] = factory.MetricTSHWM.Add(-MRD.GetTimeSpan()).UnixNano()
			switch MRD.Updates {
			case metric.NewReport:
				sqlargs["Name"] = fmt.Sprintf("%s-%s", MRD.Name, factory.MetricTSHWM.UTC().Format(time.RFC3339))
				SQL = []string{"insert_report", "keep_only_3_reports"}
			case metric.Overwrite:
				SQL = append(SQL, "update_report_start")
				SQL = append(SQL, "update_report_ts_seq")
			case metric.AppendWrapsWhenFull: // default sql list is ok
				SQL = append(SQL, "update_report_ts_seq")
			case metric.AppendStopsWhenFull: // default sql list is ok
				SQL = append(SQL, "update_report_ts_seq")
			}
		default:
		}

		for _, sql := range SQL {
			_, err = factory.getNamedSQLXTx(tx, sql).Exec(sqlargs)
			if err != nil {
				return xerrors.Errorf("error generating MetricReport. MRD(%+v) sql(%s), args(%+v): %w", MRD, SQL, sqlargs, err)
			}
		}

		// after generation, iterate over MetricInstances and Expand
		totalRows := 0
		if !MRD.SuppressDups {
			rows, err := factory.getNamedSQLXTx(tx, "iterate_metric_instance_for_report").Queryx(MRD)
			if err != nil {
				return xerrors.Errorf("error querying MetricInstance for report MRD(%s): %w", MRD, err)
			}
			for rows.Next() {
				mm := MetricMeta{}
				err = rows.StructScan(&mm)
				if err != nil {
					factory.logger.Crit("Error scanning struct result for MetricInstance query", "err", err)
					continue
				}
				mm.Timestamp = metric.SQLTimeInt{Time: factory.MetricTSHWM}
				mm.Value = mm.LastValue

				mvRowsInserted, err := factory.doInsertMetricValueForInstance(tx, &mm, func(int64) {}, true)
				totalRows += mvRowsInserted
				if err != nil {
					factory.logger.Crit("Error query", "err", err)
					continue
				}
			}
		}

		// TODO:
		// Query the MetricValueByReport to see if there are any rows for this report. If there are no rows in
		// this report, error out to cancel transaction and pretend nothing happened
		if MRD.Type != metric.OnRequest {
			count := int64(0)
			err = factory.getNamedSQLXTx(tx, "count_report_records").Get(&count, sqlargs)
			if err != nil {
				return xerrors.Errorf("error executing query count_report_records: %w", err)
			}
			if count == 0 {
				return xerrors.Errorf("Report %s has no records, aborting generation.", MRD.Name)
			}
		}

		// do this after we are sure metric report is generated
		factory.LastMRTS[MRD.Name] = factory.MetricTSHWM
		restorePendingInsert = false

		return nil
	})
}

func (factory *telemetryManager) CheckOnChangeReports(tx *sqlx.Tx, instancesUpdated map[int64]struct{}) error {
	return factory.wrapWithTXOrPassIn(tx, func(tx *sqlx.Tx) error {
		query := factory.getSQLXTx(tx, "find_onchange_mrd_by_mm_instance")
		var err error
		for mmInstanceID := range instancesUpdated {
			rows, innerErr := query.Queryx(mmInstanceID)
			if innerErr != nil {
				return xerrors.Errorf("error getting changed reports by instance: %w", innerErr)
			}
			for rows.Next() {
				var name string
				innerErr = rows.Scan(&name)
				if innerErr != nil {
					// report errors out to caller, but safe to continue here and try the next
					err = xerrors.Errorf("error with Scan() of row from query (should never/can't happen): %w", innerErr)
					continue
				}

				if _, ok := factory.NextMRTS[name]; ok {
					continue
				}

				// ensure we generate report at most every 5 seconds by scheduling generation for 5s from previous report
				// this will immediately generate if the report is older than 5s
				factory.NextMRTS[name] = factory.LastMRTS[name].Add(factory.SmallestReportInterval)
			}
		}
		return err
	})
}

// RawMetricInstance is a sub structure to help serialize stuff to db. it
// containst the stuff we are putting in or taking out of DB for Instance.
type RawMetricInstance struct {
	// Instance fields. Rest of the MetricInstance fields are in the MetricValue
	Label             string            `db:"Label"`
	InstanceID        int64             `db:"InstanceID"`
	CollectionScratch Scratch           `db:"CollectionScratch"`
	FlushTime         metric.SQLTimeInt `db:"FlushTime"`
	LastTS            metric.SQLTimeInt `db:"LastTS"`
	LastValue         string            `db:"LastValue"`
	MISensorInterval  time.Duration     `db:"MISensorInterval"`
	MISensorSlack     time.Duration     `db:"MISensorSlack"`
	Dirty             bool              `db:"Dirty"`
	MIRequiresExpand  bool              `db:"MIRequiresExpand"`
	HasAssocMM        bool              `db:"HasAssocMM"`
}

// MetricMeta is a fusion structure: Meta + Instance + MetricValueEvent
type MetricMeta struct {
	metric.MetricValueEventData
	RawMetricMeta
	RawMetricInstance
	ValueToWrite string `db:"Value"`
	SuppressDups bool   `db:"SuppressDups"`
}

func resetMMAggregation(mm *MetricMeta) {
	// Construct label and Scratch space - (validate and audit this to make sure labels match legacy)
	mm.Label = fmt.Sprintf("%s %s", mm.Context, mm.Name)
	mm.FlushTime = metric.SQLTimeInt{Time: mm.Timestamp.Add(mm.CollectionDuration.Duration())}
	mm.CollectionScratch.Sum = 0
	mm.CollectionScratch.Numvalues = 0
	mm.CollectionScratch.Maximum = -math.MaxFloat64
	mm.CollectionScratch.Minimum = math.MaxFloat64
	if mm.CollectionFunction != "" {
		mm.Label += fmt.Sprintf("- %s (%v)", mm.CollectionFunction, time.Duration(mm.CollectionDuration))
	}
}

// TODO: Implement more specific wildcard matching (not required for halo+)
// TODO: Need to look up friendly fqdd (FOR LABEL)
//  Both of the above TODO should happen in the for rows.Next() {...} loop
func (factory *telemetryManager) InsertMetricInstance(tx *sqlx.Tx, ev *metric.MetricValueEventData) (instancesCreated int, err error) {
	err = factory.wrapWithTXOrPassIn(tx, func(tx *sqlx.Tx) error {
		// prepare sql for transaction outside the loop
		insertMetricInstance := factory.getNamedSQLXTx(tx, "insert_metric_instance")
		findMetricInstance := factory.getNamedSQLXTx(tx, "find_metric_instance")
		setMetricInstanceClean := factory.getNamedSQLXTx(tx, "set_metric_instance_clean")
		insertMIAssoc := factory.getSQLXTx(tx, "insert_mi_assoc")

		rows, err := factory.getNamedSQLXTx(tx, "find_metric_meta").Queryx(ev)
		if err != nil {
			return xerrors.Errorf("error querying for MetricMeta: %w", err)
		}

		// First, iterate over the MetricMeta to generate MetricInstance
		for rows.Next() {
			mm := &MetricMeta{MetricValueEventData: *ev}
			err = rows.StructScan(mm)
			if err != nil {
				factory.logger.Crit("Error scanning metric meta for event", "err", err, "metric", ev)
				continue
			}

			resetMMAggregation(mm)

			err = findMetricInstance.Get(mm, mm)
			if err != nil {
				res, err := insertMetricInstance.Exec(mm) // slow path, only ever happens once per instance
				if err != nil {
					return xerrors.Errorf("error inserting MetricInstance(%s): %w", mm, err)
				}
				mm.InstanceID, err = res.LastInsertId()
				if err != nil {
					return xerrors.Errorf("error getting last insert ID for MetricInstance(%s): %w", mm, err)
				}
				instancesCreated++
			}

			if !mm.HasAssocMM {
				_, err = insertMIAssoc.Exec(mm.MetaID, mm.InstanceID) // slow path: should be rare
				if err != nil && !strings.HasPrefix(err.Error(), "UNIQUE constraint failed") {
					return xerrors.Errorf("error inserting Association for MetricInstance(%s): %w", mm, err)
				}
			}
			if mm.Dirty {
				_, err = setMetricInstanceClean.Exec(mm) // slow path
				if err != nil {
					return xerrors.Errorf("error setting metric instance (%d) clean: %w", mm.InstanceID, err)
				}
			}
		}
		return nil
	})

	return instancesCreated, err
}

func (factory *telemetryManager) InsertMetricValue(tx *sqlx.Tx, ev *metric.MetricValueEventData, updatedInstance func(int64)) (err error) {
	return factory.wrapWithTXOrPassIn(tx, func(tx *sqlx.Tx) error {
		// Optimized, so this logic is slightly twisted from the straightforward
		// implementation
		//
		// - iterate over instances and insert
		// - if we find an instance and it is clean, we know that we are done (ie.
		// 		there are no reports with metric meta that we need to insert an instance
		// 		for)
		// - if we find an instance and it is dirty, we know a report has been
		// 		added/updated and we potentially need to see if there is a metric meta
		// 		we need to find and insert an instance for
		// - if we do not find any instances, it could be because there is no report
		// 		that contains this meta. Or, it could be because we simply haven't put
		// 		in a metric instance for this value yet
		foundInstance, dirty, err := factory.doInsertMetricValue(tx, ev, updatedInstance)
		if err != nil {
			return err
		}
		if dirty || !foundInstance {
			instancesCreated, err := factory.InsertMetricInstance(tx, ev)
			if err != nil {
				return err
			}
			if instancesCreated > 0 {
				_, _, err := factory.doInsertMetricValue(tx, ev, updatedInstance)
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (factory *telemetryManager) IterMetricInstance(tx *sqlx.Tx, mm *MetricMeta, fn func(*MetricMeta) error) error {
	return factory.wrapWithTXOrPassIn(tx, func(tx *sqlx.Tx) error {
		// And now, foreach MetricInstance, insert MetricValue
		rows, err := factory.getNamedSQLXTx(tx, "iterate_metric_instance").Queryx(mm)
		if err != nil {
			return xerrors.Errorf("Error querying MetricInstance(%s): %w", mm, err)
		}
		for rows.Next() {
			err = rows.StructScan(mm)
			if err != nil {
				factory.logger.Crit("Error scanning struct result for MetricInstance query", "err", err)
				continue
			}
			err := fn(mm)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// TODO: return an "aggregated" bool as well so caller knows its an aggregated value
func (factory *telemetryManager) handleAggregatedMV(mm *MetricMeta, floatVal float64, floatErr error) (aggregated, saveValue, saveInstance bool) {
	aggregated = false
	saveValue = true
	saveInstance = false
	if floatErr != nil && mm.CollectionFunction != "" { // handle aggregated: custom sum/min/max metrics
		saveValue = false // by default, we wont save values if aggregation is happening unless we pass the flush timeout
		saveInstance = true
		aggregated = true

		// We aggregate values in the MetricInstance until the collection duration expires for that metric
		if mm.Timestamp.After(mm.FlushTime.Time) {
			// Calculate what we should be dropping in the output
			saveValue = true // time to flush the value out now
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

			// now, reset everything to be ready for the next metric value
			mm.FlushTime = metric.SQLTimeInt{Time: mm.Timestamp.Add(mm.CollectionDuration.Duration())}
			mm.CollectionScratch.Sum = 0
			mm.CollectionScratch.Numvalues = 0
			mm.CollectionScratch.Maximum = -math.MaxFloat64
			mm.CollectionScratch.Minimum = math.MaxFloat64
		}

		mm.CollectionScratch.Numvalues++
		mm.CollectionScratch.Sum += floatVal // floatVal was saved, above.
		mm.CollectionScratch.Maximum = math.Max(floatVal, mm.CollectionScratch.Maximum)
		mm.CollectionScratch.Minimum = math.Min(floatVal, mm.CollectionScratch.Minimum)
	}
	return aggregated, saveValue, saveInstance
}

func (factory *telemetryManager) insertOneMVRow(tx *sqlx.Tx, mm *MetricMeta, updatedInstance func(int64), floatVal float64, floatErr error) error {
	var insertMV *sqlx.Stmt
	var args []interface{}

	// if the value changes, we change lastts/lastvalue so basically always have to update the instance.
	// This seems like an opportunity to optimize, but no immediately obvious way to do this.
	mm.LastTS = mm.Timestamp
	mm.LastValue = mm.Value

	// Put into optimized tables, if possible. Try INT first, as it will error out for a float(1.0) value, but not vice versa
	intVal, err := strconv.ParseInt(mm.Value, 10, 64)
	switch {
	case err == nil:
		insertMV = factory.getSQLXTx(tx, "insert_mv_int")
		args = []interface{}{mm.InstanceID, mm.Timestamp, intVal}
	case floatErr == nil: // re-use already parsed floatVal above
		insertMV = factory.getSQLXTx(tx, "insert_mv_real")
		args = []interface{}{mm.InstanceID, mm.Timestamp, floatVal}
	default:
		insertMV = factory.getSQLXTx(tx, "insert_mv_text")
		args = []interface{}{mm.InstanceID, mm.Timestamp, mm.Value}
	}

	_, err = insertMV.Exec(args...)
	if err != nil {
		if !strings.HasPrefix(err.Error(), "UNIQUE constraint failed") {
			return xerrors.Errorf(
				"Error inserting MetricValue for MetricInstance(%d)/MetricMeta(%d), ARGS: %+v: %w",
				mm.InstanceID, mm.MetaID, args, err)
		}
	}
	// report change hook. let caller know which instances were updated so they can look up reports for OnChange
	updatedInstance(mm.InstanceID)
	return nil
}

func (factory *telemetryManager) pumpMV(tx *sqlx.Tx, mm *MetricMeta, updatedInstance func(int64), expandOnly bool) (bool, bool, error) {
	// optimization: try to parse as float first, we can re-use this later in many cases
	floatVal, floatErr := strconv.ParseFloat(mm.Value, 64)

	// handle the aggregation functions, if requested by user for this metric instance (sum, avg, min, max)
	aggregated, saveValue, saveInstance := factory.handleAggregatedMV(mm, floatVal, floatErr)

	if mm.SuppressDups && mm.LastValue == mm.Value {
		// if value is same as previous report and this report is suppressing dups, no need to save
		saveValue = false
	}

	// if we are supposed to be doing only expansion, we'll only save the value if it's an aggregated value that has passed the flush time
	if expandOnly && !aggregated {
		return false, saveInstance, nil
	}

	if saveValue {
		// if we are saving the value, definitely need to stave the instance as well
		return true, true, factory.insertOneMVRow(tx, mm, updatedInstance, floatVal, floatErr)
	}
	return saveValue, saveInstance, nil
}

func (factory *telemetryManager) doInsertMetricValueForInstance(
	tx *sqlx.Tx, mm *MetricMeta,
	updatedInstance func(int64), expandOnly bool) (numInserted int, err error) {
	// Here we are going to expand any metrics that were skipped by upstream
	/// make sure that lastts is set and not the unix zero time
	saveInstance := false
	after := !mm.LastTS.IsZero()
	after = after && !mm.LastTS.Equal(time.Unix(0, 0))
	after = after && mm.Timestamp.After(mm.LastTS.Add(mm.MISensorInterval+mm.MISensorSlack))
	if mm.MIRequiresExpand && !mm.SuppressDups && after {
		missingInterval := mm.Timestamp.Sub(mm.LastTS.Time) // .Sub() returns a Duration!
		if missingInterval > factory.MaxMetricExpandInterval {
			// avoid disasters like filling in metrics since 1970...
			factory.logger.Warn(
				"Expand: missed interval too large, adjusted missingInterval to max supported",
				"missingInterval", missingInterval, "max", factory.MaxMetricExpandInterval)
			missingInterval = factory.MaxMetricExpandInterval // fill in a max of one hour of metrics
		}

		savedTS := mm.Timestamp
		savedValue := mm.Value
		mm.Value = mm.LastValue

		// loop over putting the same Metric Value in (ie. LastValue), but updating the timestamp
		mm.Timestamp = metric.SQLTimeInt{Time: mm.Timestamp.Add(-missingInterval + mm.MISensorInterval)}

		// TODO: we could use math to smooth this out a bit more rather than just jumping by sensorinterval
		for mm.Timestamp.Before(savedTS.Time.Add(-mm.MISensorSlack)) {
			inserted, _, err := factory.pumpMV(tx, mm, updatedInstance, false)
			if err != nil {
				factory.logger.Crit("Error inserting metric value", "err", err)
			}
			if inserted {
				numInserted++
			}
			mm.Timestamp = metric.SQLTimeInt{Time: mm.Timestamp.Add(mm.MISensorInterval)} // .Add() a negative to go backwards
		}
		mm.Timestamp = savedTS
		mm.Value = savedValue
	}

	inserted := false
	inserted, saveInstance, err = factory.pumpMV(tx, mm, updatedInstance, expandOnly)
	if err != nil {
		factory.logger.Crit("Error inserting metric value", "err", err)
	}
	if inserted {
		numInserted++
	}
	if saveInstance {
		_, err = factory.getNamedSQLXTx(tx, "update_metric_instance").Exec(mm)
		if err != nil && !strings.HasPrefix(err.Error(), "UNIQUE constraint failed") {
			return 0, xerrors.Errorf("Failed to update MetricInstance(%d) with MetricMeta(%d): %w", mm.InstanceID, mm.MetaID, err)
		}
	}

	return numInserted, nil
}

func (factory *telemetryManager) doInsertMetricValue(
	tx *sqlx.Tx, ev *metric.MetricValueEventData,
	updatedInstance func(int64)) (foundInstance, dirty bool, err error) {
	dirty = false
	foundInstance = false
	mm := MetricMeta{MetricValueEventData: *ev}
	err = factory.IterMetricInstance(tx, &mm, func(mm *MetricMeta) error {
		foundInstance = true
		if mm.Dirty {
			dirty = true
		}
		_, err := factory.doInsertMetricValueForInstance(tx, mm, updatedInstance, false)
		if err != nil {
			return xerrors.Errorf("Instance(%+v) insert failed: %w", mm, err)
		}
		return nil
	})
	return
}

func (factory *telemetryManager) runSQLFromList(sqllist []string, entrylog string, errorlog string) (err error) {
	factory.logger.Info(entrylog)
	for _, sqlName := range sqllist {
		sqlName := sqlName // make scopelint happy
		err := factory.wrapWithTX(func(tx *sqlx.Tx) error {
			res, err := factory.getSQLXTx(tx, sqlName).Exec()
			if err != nil {
				return xerrors.Errorf(errorlog, sqlName, err)
			}
			rows, err := res.RowsAffected()
			if err != nil {
				factory.logger.Warn("Run maintenance SQL. RowsAffected() returned error", "name", entrylog, "sql-name", sqlName, "rows", rows, "err", err)
			} else {
				factory.logger.Debug("Run maintenance SQL", "name", entrylog, "sql-name", sqlName, "rows", rows)
			}
			return nil
		})
		if err != nil {
			factory.logger.Warn("Error running maintenance sql", "name", sqlName, "err", err)
		}
	}
	return nil
}

func (factory *telemetryManager) DeleteOrphans() (err error) {
	return factory.runSQLFromList(factory.orphanops, "Database Maintenance: Delete Orphans", "Orphan cleanup failed-> '%s': %w")
}

func (factory *telemetryManager) DeleteOldestValues() (err error) {
	return factory.runSQLFromList(factory.deleteops, "Database Maintenance: Delete Oldest Metric Values", "Value cleanup failed-> '%s': %w")
}

func (factory *telemetryManager) Vacuum() error {
	// cant vacuum inside a transaction
	entrylog := "Database Maintenance: Vacuum"
	factory.logger.Info(entrylog)
	for _, sql := range factory.vacuumops {
		res, err := factory.getSQLX(sql).Exec()
		if err != nil {
			return xerrors.Errorf("Vacuum failed-> '%s': %w", sql, err)
		}
		rows, err := res.RowsAffected()
		if err != nil {
			factory.logger.Warn("Run maintenance SQL. RowsAffected() returned error", "name", entrylog, "sql-name", sql, "rows", rows, "err", err)
		} else {
			factory.logger.Debug("Run maintenance SQL", "name", entrylog, "sql-name", sql, "rows", rows)
		}
	}
	return nil
}

func (factory *telemetryManager) Optimize() error {
	return factory.runSQLFromList(factory.optimizeops, "Database Maintenance: Optimize", "Optimization failed-> '%s': %w")
}
