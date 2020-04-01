package udb

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"encoding/json"
	"github.com/jmoiron/sqlx"
	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"
	"golang.org/x/xerrors"

	"github.com/superchalupa/sailfish/cmd/metric-engine/fifocompat"
	"github.com/superchalupa/sailfish/cmd/metric-engine/telemetry"
	"github.com/superchalupa/sailfish/src/looplab/event"

	log "github.com/superchalupa/sailfish/src/log"
)

const (
	udbChangeEvent eh.EventType = "UDBChangeEvent"
	jsonEnableMRD               = `{"MetricReportDefinitionEnabled": true}`
	jsonDisableMRD              = `{"MetricReportDefinitionEnabled": false}`
)

type busComponents interface {
	GetBus() eh.EventBus
}

type eventHandlingService interface {
	AddEventHandler(string, eh.EventType, func(eh.Event))
}

/*
	  -- don't ever run sync() or friends
		-- PRAGMA synchronous = off;
		-- PRAGMA       journal_mode  = off;
		-- PRAGMA udbdm.journal_mode  = off;
		-- PRAGMA udbsm.journal_mode  = off;
	  -- these seem to increase memory usage, so disable until we get good values for these
		-- PRAGMA cache_size = 0;
		-- PRAGMA udbdm.cache_size = 0;
		-- PRAGMA udbsm.cache_size = 0;
		-- PRAGMA mmap_size = 0;
*/

type RowIdCurrentVal struct {
	CurrValue string
	Rowid     int64
	key       string
}

type UDBRowIdCurrentVal struct {
	RowId             int64
	EnableTeleRsyslog string
}

// StartupUDBImport will attach event handlers to handle import UDB import
func StartupUDBImport(logger log.Logger, cfg *viper.Viper, am3Svc eventHandlingService, d busComponents) error {
	// setup programatic defaults. can be overridden in config file
	cfg.SetDefault("udb.udbdatabasepath", "file:/run/unifieddatabase/DMLiveObjectDatabase.db?cache=shared&_foreign_keys=off&mode=ro&_busy_timeout=1000")
	cfg.SetDefault("udb.shmdatabasepath", "file:/run/unifieddatabase/SHM.db?cache=shared&_foreign_keys=off&mode=ro&_busy_timeout=1000")
	cfg.SetDefault("udb.udbnotifypipe", "/run/telemetryservice/udbtdbipcpipe")

	database, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		return xerrors.Errorf("Could not create empty in-memory sqlite database: %w", err)
	}

	// attach UDB db
	attach := "Attach '" + cfg.GetString("udb.udbdatabasepath") + "' as udbdm"
	fmt.Println(attach)
	_, err = database.Exec(attach)
	if err != nil {
		return xerrors.Errorf("Could not attach UDB database. sql(%s) err: %w", attach, err)
	}

	// attach SHM db
	attach = "Attach '" + cfg.GetString("udb.shmdatabasepath") + "' as udbsm"
	fmt.Println(attach)
	_, err = database.Exec(attach)
	if err != nil {
		return xerrors.Errorf("Could not attach SHM database. sql(%s) err: %w", attach, err)
	}

	// we have a separate goroutine for this, so we should be safe to busy-wait
	_, err = database.Exec(`
		-- ensure nothing we do will ever modify the source
		PRAGMA query_only = 1;
		-- should be set in connection string, but just in case:
		PRAGMA busy_timeout = 1000;
		`)
	if err != nil {
		return xerrors.Errorf("Could not set up initial UDB database parameters: %w", err)
	}

	// we have only one thread doing updates, so one connection should be
	// fine. keeps sqlite from opening new connections un-necessarily
	database.SetMaxOpenConns(1)

	importMgr, err := newImportManager(logger, database, d, cfg)
	if err != nil {
		database.Close()
		return xerrors.Errorf("Error creating udb integration: %w", err)
	}

	bus := d.GetBus()
	// set up the event handler that will do periodic imports every ~1s.
	am3Svc.AddEventHandler("Import UDB Metric Values", telemetry.PublishClock, MakeHandlerUDBPeriodicImport(logger, importMgr, bus))
	am3Svc.AddEventHandler("UDB Change Notification", udbChangeEvent, MakeHandlerUDBChangeNotify(logger, importMgr, bus))

	// handle UDB notifications
	go handleUDBNotifyPipe(logger, cfg.GetString("udb.udbnotifypipe"), d)

	// query UDB database for defined list of legacy reports, update the telemetry enable/disable status for each report.
	configSync, err := newConfigSync(logger, database, d)

	am3Svc.AddEventHandler("UDB Cfg change Notification", udbChangeEvent, MakeHandlerLegacyAttributeSync(logger, importMgr, bus, configSync))

	go configSync.GenerateStartupConfig(logger, d)

	fmt.Println("before sql query")
	return nil
}

type ConfigSync struct {
	db          *sqlx.DB
	enumEntries map[int64]string
	intEntries  map[int64]string
	strEntries  map[int64]string
	bus         eh.EventBus
	Temp        int
}

func newConfigSync(logger log.Logger, database *sqlx.DB, d busComponents) (*ConfigSync, error) {
	tempo := 10
	cfgS := &ConfigSync{
		db:          database,
		bus:         d.GetBus(),
		enumEntries: map[int64]string{},
		intEntries:  map[int64]string{},
		strEntries:  map[int64]string{},
		Temp:        tempo,
	}

	// TODO: Need to move this query out into the metric-engine.yaml file and prepare it on startup
	sqltextEnableTele := "select RowID,CurrentValue,Key from TblEnumAttribute where Key like '%Telemetry%';"

	rows, err := database.Queryx(sqltextEnableTele)
	fmt.Println("after sql query")
	if err != nil {
		fmt.Println("sql query failed")
	}

	for rows.Next() {
		var RowID int64
		var CurrentValue string
		var key string
		err = rows.Scan(&RowID, &CurrentValue, &key)
		if err != nil {
			// report errors out to caller, but safe to continue here and try the next
			fmt.Println("error with Scan() of row from query: %w", err)
			continue
		}

		cfgS.enumEntries[RowID] = key
	}

	fmt.Printf("DEBUG: got all these ENUM configuration settings: %+v\n", cfgS.enumEntries)
	fmt.Printf("DEBUG: got all these STR  configuration settings: %+v\n", cfgS.strEntries)
	fmt.Printf("DEBUG: got all these INT  configuration settings: %+v\n", cfgS.intEntries)

	return cfgS, err
}

func (cs ConfigSync) GenerateStartupConfig(logger log.Logger, d busComponents) {
	for _, s := range []struct {
		table  string
		mapref *map[int64]string
	}{
		{"TblEnumAttributes", &cs.enumEntries},
		{"TblStrAttributes", &cs.strEntries},
		{"TblIntAttributes", &cs.intEntries},
	} {
		for rowid := range *s.mapref {
			notify := changeNotify{Database: "DMLiveObjectDatabase.db", Table: s.table, Rowid: rowid, Operation: 0}
			publishAndWait(logger, d.GetBus(), udbChangeEvent, &notify)
		}
	}
}

func MakeHandlerLegacyAttributeSync(logger log.Logger, importMgr *importManager, bus eh.EventBus, configSync *ConfigSync) func(eh.Event) {
	return func(event eh.Event) {
		notify, ok := event.Data().(*changeNotify)
		if !ok {
			logger.Crit("UDB Change Notifier message handler got an invalid data event", "event", event, "eventdata", event.Data())
			return
		}

		//fmt.Printf("Receiving UDB Cfg ChangeEvent %d,Table:%s,db:%s\n",notify.Rowid,notify.Table,notify.Database)
		// Step 1: Is this a DMLiveObjectDatabase change
		if notify.Database != "DMLiveObjectDatabase.db" {
			return
		}

		// Step 2: Is this a tblEnumAttribute change
		keyname := ""
		ok = false
		switch notify.Table {
		case "TblEnumAttribute":
			keyname, ok = configSync.enumEntries[notify.Rowid]
		case "TblIntAttribute":
			keyname, ok = configSync.intEntries[notify.Rowid]
		case "TblStrAttribute":
			keyname, ok = configSync.strEntries[notify.Rowid]
		}

		if !ok {
			return
		}

		// Step 4: Generate a "UpdateMetricReportDefinition" event
		//	Ok, here first thing we need to do is do a UDB query to find the current value since UDB didn't actually send us the value

		//# tblEnumAttribute
		//# iDRAC.Embedded.1#TelemetryPSUMetrics.1#EnableTelemetry   (DONE)
		//# iDRAC.Embedded.1#TelemetryFPGASensor.1#RsyslogTarget

		//# tblIntAttribute
		//# iDRAC.Embedded.1#TelemetryFPGASensor.1#ReportInterval

		//# tblStrAttribute
		//# iDRAC.Embedded.1#TelemetryFPGASensor.1#ReportTriggers

		sqltextEnableTele := ""
		fmt.Printf("GOT KEY: %s\n", keyname)
		keys := strings.Split(keyname, "#")
		switch keys[2] {
		case "EnableTelemetry":
			sqltextEnableTele = "select CurrentValue from TblEnumAttribute where ROWID=?"
		//case "RsyslogTarget":
		//case "ReportInterval":
		//case "ReportTriggers":
		default:
			fmt.Printf("UNHANDLED TYPE OF KEY\n")
			return
		}

		var CurrentValue string
		err := configSync.db.Get(&CurrentValue, sqltextEnableTele, &notify.Rowid)
		if err != nil {
			logger.Crit("Error checking currentvalue of rowid in database ", "err", err)
		}
		fmt.Printf("CurrentValue:%s \n", CurrentValue)

		eventData, err := eh.CreateEventData(telemetry.UpdateMetricReportDefinition)
		if err != nil {
			fmt.Printf("Error trying to create update event: %s\n", err)
			return
		}

		updateEvent, ok := eventData.(*telemetry.UpdateMetricReportDefinitionData)
		if !ok {
			fmt.Printf("Internal error trying to type assert to update event")
		}

		updateEvent.ReportDefinitionName = keys[1][len("Telemetry") : len(keys[1])-len(".1")]

		switch keys[2] {
		case "EnableTelemetry":
			switch CurrentValue {
			case "Enabled":
				fmt.Printf("Enabling report: %s\n", updateEvent.ReportDefinitionName)
				json.Unmarshal([]byte(jsonEnableMRD), &(updateEvent.Patch))
			case "Disabled":
				fmt.Printf("Disabling report: %s\n", updateEvent.ReportDefinitionName)
				json.Unmarshal([]byte(jsonDisableMRD), &(updateEvent.Patch))
			}

		}
		fmt.Printf("About to send event for report(%s): %s\n", updateEvent.ReportDefinitionName, string(updateEvent.Patch))
		publishAndWait(logger, bus, telemetry.UpdateMetricReportDefinition, updateEvent)
	}
}

func MakeHandlerUDBPeriodicImport(logger log.Logger, importMgr *importManager, bus eh.EventBus) func(eh.Event) {
	// close over periodic... first iteration will do forced, nonperiodic import, rest will always do periodic import
	periodic := false
	return func(event eh.Event) {
		err := importMgr.runPeriodicImports(periodic)
		if err != nil {
			logger.Crit("Error running import", "err", err)
		}
		periodic = true
	}
}

func MakeHandlerUDBChangeNotify(logger log.Logger, importMgr *importManager, bus eh.EventBus) func(eh.Event) {
	return func(event eh.Event) {
		notify, ok := event.Data().(*changeNotify)
		if !ok {
			logger.Crit("UDB Change Notifier message handler got an invalid data event", "event", event, "eventdata", event.Data())
			return
		}
		err := importMgr.runUDBChangeImports(strings.ToLower(notify.Database), strings.ToLower(notify.Table))
		if err != nil {
			logger.Crit("Error checking if database changed", "err", err, "notify", notify)
		}
	}
}

type changeNotify struct {
	Database  string
	Table     string
	Rowid     int64
	Operation int64
}

// This is the number of '|' separated fields in a correct record
const numChangeFields = 4

func publishAndWait(logger log.Logger, bus eh.EventBus, et eh.EventType, data eh.EventData) {
	evt := event.NewSyncEvent(et, data, time.Now())
	evt.Add(1)
	err := bus.PublishEvent(context.Background(), evt)
	if err != nil {
		logger.Crit("Error publishing event. This should never happen!", "err", err)
	}
	evt.Wait()
}

func splitUDBNotify(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF {
		fmt.Printf("EOF\n")
		return 0, nil, io.EOF
	}
	start := bytes.Index(data, []byte("||"))
	if start == -1 { // didnt find starting ||, skip over everything
		fmt.Printf("DEBUG (shouldnt happen): NO STARTING ||: len(%v), bytes(%+v), string(%v)\n", len(data), data, string(data))
		return len(data), data, nil
	}

	if len(data) < start+1 { // not enough data, read some more
		// this can happen in normal operations
		//fmt.Printf("DEBUG (can happen): NEED MORE DATA len(%v), start(%v)\n", len(data), start)
		return 0, nil, nil
	}

	if start > 0 {
		fmt.Printf("DEBUG (shouldnt happen): JUNK start(%v): %v\n", start, string(data[0:start]))
		return start, data[0:0], nil
	}

	end := bytes.Index(data[start+1:], []byte("||"))
	if end == -1 { // didnt find ending ||, read some more
		// this can happen in normal operations
		//fmt.Printf("DEBUG (can happen): NO ENDING ||, NEED MORE. len(%v), start(%v), end(%v): %+v\n", len(data), start, end, string(data))
		return 0, nil, nil
	}

	if end == 0 { // got a ||| or ||||, consume 1 byte at a time
		fmt.Printf("DEBUG (shouldnt happen): GOT ||| or ||||, skip 2. len(%v), start(%v), end(%v): %v\n", len(data), start, end, data[start:end])
		return 1, data[0:0], nil
	}

	// consume everything between start and end markers
	//fmt.Printf("CONSUME: %v - %v : %v\n", start, end, string(data[start:start+1+end+2]))
	return start + 1 + end + 2, data[start+2 : start+1+end], nil
}

// handleUDBNotifyPipe will handle the notification events from UDB on the
// notification pipe and turn them into event bus messages
//
// Data format we get:
//    DB                      TBL                  ROWID     operationid
// ||DMLiveObjectDatabase.db|TblNic_Port_Stats_Obj|167445167|23||
//
// The reader of the named pipe gets an EOF when the last writer exits. To
// avoid this, we'll simply open it ourselves for writing and never close it.
// This will ensure the pipe stays around forever without eof... That's what
// nullWriter is for, below.
func handleUDBNotifyPipe(logger log.Logger, pipePath string, d busComponents) {
	err := fifocompat.MakeFifo(pipePath, 0660)
	if err != nil && !os.IsExist(err) {
		logger.Warn("Error creating UDB pipe", "err", err)
	}

	file, err := os.OpenFile(pipePath, os.O_CREATE, os.ModeNamedPipe)
	if err != nil {
		logger.Crit("Error opening UDB pipe", "err", err)
	}

	defer file.Close()

	nullWriter, err := os.OpenFile(pipePath, os.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		logger.Crit("Error opening UDB pipe for (placeholder) write", "err", err)
	}

	// defer .Close() to keep linters happy. Inside we know we never exit...
	defer nullWriter.Close()

	s := bufio.NewScanner(file)
	s.Split(splitUDBNotify)
	for s.Scan() {
		fields := bytes.Split(s.Bytes(), []byte("|"))
		if len(fields) != numChangeFields {
			fmt.Printf("DEBUG (shouldnt happen): GOT MISMATCH(%v!=%v): %v\n", len(fields), numChangeFields, s.Text())
			continue
		}

		n := changeNotify{
			Database: string(fields[0]),
			Table:    string(fields[1]),
		}
		n.Rowid, _ = strconv.ParseInt(string(fields[2]), 10, 64)
		n.Operation, _ = strconv.ParseInt(string(fields[3]), 10, 64)

		publishAndWait(logger, d.GetBus(), udbChangeEvent, &n)
	}

	panic("should never finish handling UDB notify pipe")
}
