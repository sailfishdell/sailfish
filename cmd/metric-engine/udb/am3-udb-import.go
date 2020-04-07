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
	udbChangeEvent        eh.EventType = "UDBChangeEvent"
	jsonEnableMRD                      = `{"MetricReportDefinitionEnabled": true}`
	jsonDisableMRD                     = `{"MetricReportDefinitionEnabled": false}`
	jsonReportTimespanMRD              = `{"ReportTimespan": "PT%sS"}`
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

// Startup will attach event handlers to handle import UDB import
func Startup(logger log.Logger, cfg *viper.Viper, am3Svc eventHandlingService, d busComponents) (func(), error) {
	// setup programatic defaults. can be overridden in config file
	cfg.SetDefault("udb.udbdatabasepath", "file:/run/unifieddatabase/DMLiveObjectDatabase.db?cache=shared&_foreign_keys=off&mode=ro&_busy_timeout=1000")
	cfg.SetDefault("udb.shmdatabasepath", "file:/run/unifieddatabase/SHM.db?cache=shared&_foreign_keys=off&mode=ro&_busy_timeout=1000")
	cfg.SetDefault("udb.udbnotifypipe", "/run/telemetryservice/udbtdbipcpipe")

	database, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, xerrors.Errorf("Could not create empty in-memory sqlite database: %w", err)
	}

	// attach UDB db
	attach := "Attach '" + cfg.GetString("udb.udbdatabasepath") + "' as udbdm"
	fmt.Println(attach)
	_, err = database.Exec(attach)
	if err != nil {
		database.Close()
		return nil, xerrors.Errorf("Could not attach UDB database. sql(%s) err: %w", attach, err)
	}

	// attach SHM db
	attach = "Attach '" + cfg.GetString("udb.shmdatabasepath") + "' as udbsm"
	fmt.Println(attach)
	_, err = database.Exec(attach)
	if err != nil {
		database.Close()
		return nil, xerrors.Errorf("Could not attach SHM database. sql(%s) err: %w", attach, err)
	}

	// we have a separate goroutine for this, so we should be safe to busy-wait
	_, err = database.Exec(`
		-- ensure nothing we do will ever modify the source
		PRAGMA query_only = 1;
		-- should be set in connection string, but just in case:
		PRAGMA busy_timeout = 1000;
		`)
	if err != nil {
		database.Close()
		return nil, xerrors.Errorf("Could not set up initial UDB database parameters: %w", err)
	}

	// we have only one thread doing updates, so one connection should be
	// fine. keeps sqlite from opening new connections un-necessarily
	database.SetMaxOpenConns(1)

	importMgr, err := newImportManager(logger, database, d, cfg)
	if err != nil {
		database.Close()
		return nil, xerrors.Errorf("Error creating udb integration: %w", err)
	}

	bus := d.GetBus()
	// set up the event handler that will do periodic imports every ~1s.
	am3Svc.AddEventHandler("Import UDB Metric Values", telemetry.PublishClock, MakeHandlerUDBPeriodicImport(logger, importMgr, bus))
	am3Svc.AddEventHandler("UDB Change Notification", udbChangeEvent, MakeHandlerUDBChangeNotify(logger, importMgr, bus))

	// query UDB database for defined list of legacy reports, update the telemetry enable/disable status for each report.
	configSync, err := newConfigSync(logger, database, d)

	// check if a udb change is associated with an AR entry we care about
	am3Svc.AddEventHandler("UDB Cfg change Notification", udbChangeEvent, MakeHandlerLegacyAttributeSync(logger, importMgr, bus, configSync))

	// generate artificial change notifications to force read of existing AR enable/disable and sync to reports
	go configSync.kickstartLegacyARConfigSync(logger, d)

	// handle UDB notifications
	go handleUDBNotifyPipe(logger, cfg.GetString("udb.udbnotifypipe"), d)

	return func() { database.Close() }, nil
}

type ConfigSync struct {
	db          *sqlx.DB
	enumEntries map[int64]string
	intEntries  map[int64]string
	strEntries  map[int64]string
	bus         eh.EventBus
}

func newConfigSync(logger log.Logger, database *sqlx.DB, d busComponents) (*ConfigSync, error) {

	cfgS := &ConfigSync{
		db:          database,
		bus:         d.GetBus(),
		enumEntries: map[int64]string{},
		intEntries:  map[int64]string{},
		strEntries:  map[int64]string{},
	}

	err := GetRowId(logger, database, "Enum", cfgS.enumEntries)
	if err != nil {
		logger.Crit("GetRowId Enum failed\n")
	}

	err = GetRowId(logger, database, "Str", cfgS.strEntries)
	if err != nil {
		logger.Crit("GetRowId Str failed\n")
	}

	err = GetRowId(logger, database, "Int", cfgS.intEntries)
	if err != nil {
		logger.Crit("GetRowId Int failed\n")
	}

	fmt.Printf("Got all these ENUM configuration settings: %+v\n", cfgS.enumEntries)
	fmt.Printf("Got all these STR  configuration settings: %+v\n", cfgS.strEntries)
	fmt.Printf("Got all these INT  configuration settings: %+v\n", cfgS.intEntries)

	return cfgS, err
}

func GetRowId(logger log.Logger, database *sqlx.DB, dataType string, Entries map[int64]string) error {

	var sqlForCurrentValue string

	switch dataType {
	case "Enum":
		// TODO: Need to move this query out into the metric-engine.yaml file and prepare it on startup
		sqlForCurrentValue = "select RowID,CurrentValue,Key from TblEnumAttribute where Key like '%Telemetry%';"
	case "Str":
		// TODO: Need to move this query out into the metric-engine.yaml file and prepare it on startup
		sqlForCurrentValue = "select RowID,CurrentValue,Key from TblStrAttribute where Key like '%Telemetry%';"
	case "Int":
		// TODO: Need to move this query out into the metric-engine.yaml file and prepare it on startup
		sqlForCurrentValue = "select RowID,CurrentValue,Key from TblIntAttribute where Key like '%Telemetry%';"
	}

	rows, err := database.Queryx(sqlForCurrentValue)
	if err != nil {
		logger.Crit("sql query failed for", "dataType", dataType)
	}

	for rows.Next() {
		var RowID int64
		var CurrentValue string
		var key string
		err = rows.Scan(&RowID, &CurrentValue, &key)
		if err != nil {
			// report errors out to caller, but safe to continue here and try the next
			logger.Crit("error with Scan() of row from query: ", "err", err)
			continue
		}

		Entries[RowID] = key
	}
	n := len(Entries)
	fmt.Printf("len of map:%d for Tbl%sAttribute\n", n, dataType)
	return err
}

func (cs ConfigSync) kickstartLegacyARConfigSync(logger log.Logger, d busComponents) {
	for _, s := range []struct {
		table  string
		mapref *map[int64]string
	}{
		{"TblEnumAttribute", &cs.enumEntries},
		{"TblStrAttribute", &cs.strEntries},
		{"TblIntAttribute", &cs.intEntries},
	} {
		for rowid := range *s.mapref {
			notify := changeNotify{Database: "DMLiveObjectDatabase.db", Table: s.table, Rowid: rowid, Operation: 0}
			publishAndWait(logger, d.GetBus(), udbChangeEvent, &notify)
			// something really odd, this should NOT be needed, but it hangs without it
			time.Sleep(time.Millisecond)
		}
	}
}

//# tblEnumAttribute
//# iDRAC.Embedded.1#TelemetryPSUMetrics.1#EnableTelemetry   (DONE)
//# iDRAC.Embedded.1#TelemetryFPGASensor.1#RsyslogTarget     (waiting for the MRD syntax for Rsyslog)

//# tblIntAttribute
//# iDRAC.Embedded.1#TelemetryFPGASensor.1#ReportInterval    (DONE)

//# tblStrAttribute
//# iDRAC.Embedded.1#TelemetryFPGASensor.1#ReportTriggers    (waiting for the MRD syntax for Rsyslog)

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

		// step 3: Is this rowId the interested one,i.e Config rowid
		if !ok {
			return
		}

		// Step 4: Generate a "UpdateMRDCommandEvent" event
		//	Ok, here first thing we need to do is do a UDB query to find the current value since UDB didn't actually send us the value

		sqlForCurrentValue := ""

		keys := strings.Split(keyname, "#")
		switch keys[2] {
		case "EnableTelemetry":
			sqlForCurrentValue = "select CurrentValue from TblEnumAttribute where ROWID=?"
		//case "RsyslogTarget":
		//    sqlForCurrentValue = "select CurrentValue from TblStrAttribute where ROWID=?"
		case "ReportInterval":
			sqlForCurrentValue = "select CurrentValue from TblIntAttribute where ROWID=?"
		//case "ReportTriggers":
		case "DevicePollFrequency":
			// unused: info only key. no need to process.
			return
		default:
			//logger.Crit("UNHANDLED TYPE OF KEY:","keys[2]",keys[2])
			fmt.Printf("UNHANDLED TYPE OF KEY:%s\n", keys[2])
			return
		}

		var CurrentValue string
		err := configSync.db.Get(&CurrentValue, sqlForCurrentValue, &notify.Rowid)
		if err != nil {
			logger.Crit("Error checking currentvalue of rowid in database ", "err", err)
		}

		eventData, err := eh.CreateEventData(telemetry.UpdateMRDCommandEvent)
		if err != nil {
			logger.Crit("Error trying to create update event:", "err", err)
			return
		}

		updateEvent, ok := eventData.(*telemetry.UpdateMRDCommandData)
		if !ok {
			logger.Crit("Internal error trying to type assert to update event")
			return
		}

		updateEvent.ReportDefinitionName = keys[1][len("Telemetry") : len(keys[1])-len(".1")]

		if updateEvent.ReportDefinitionName == "" {
			logger.Crit("Skipping report definition update for malformed keyname", "keyname", keys[1])
			return
		}
		switch keys[2] {
		case "EnableTelemetry":
			switch CurrentValue {
			case "Enabled":
				logger.Info("Enabling report:", "ReportName", updateEvent.ReportDefinitionName)
				json.Unmarshal([]byte(jsonEnableMRD), &(updateEvent.Patch))
			case "Disabled":
				logger.Info("Disabling report:", "ReportName", updateEvent.ReportDefinitionName)
				json.Unmarshal([]byte(jsonDisableMRD), &(updateEvent.Patch))
			}
		case "ReportInterval":
			json.Unmarshal([]byte(fmt.Sprintf(jsonReportTimespanMRD, CurrentValue)), &(updateEvent.Patch))
		}
		logger.Info("CRIT: about to send", "report", updateEvent.ReportDefinitionName, "PATCH", string(updateEvent.Patch))
		fmt.Printf("About to send event for report %s Patch %s\n", updateEvent.ReportDefinitionName, string(updateEvent.Patch))
		publishAndWait(logger, bus, telemetry.UpdateMRDCommandEvent, updateEvent)
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
