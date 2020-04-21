package udb

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

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
)

// format strings for JSON for update events
const (
	jsonEnableMRD              = `{"MetricReportDefinitionEnabled": true}`
	jsonDisableMRD             = `{"MetricReportDefinitionEnabled": false}`
	jsonReportTimespanMRD      = `{"Schedule": {"RecurrenceInterval": "PT%sS"}}`
	enableTelemetry            = "EnableTelemetry"
	reportInterval             = "ReportInterval"
	reportTriggers             = "ReportTriggers"
	maxPackedEvents            = 30
	ConfigDBSyncEnable         = "key=iDRAC.Embedded.1#Telemetry%s.1#EnableTelemetry"
	ConfigDBSyncReportInterval = "key=iDRAC.Embedded.1#Telemetry%s.1#ReportInterval"
	SyncValueTrue              = "value=Enabled"
	SyncValueFalse             = "value=Disabled"
	SyncValue                  = "value=%v"
)

type busComponents interface {
	GetBus() eh.EventBus
}

type eventHandlingService interface {
	AddEventHandler(string, eh.EventType, func(eh.Event)) error
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

func attachDB(database *sqlx.DB, dbfile string, as string) error {
	// attach UDB db
	attach := "" +
		"PRAGMA cache_size = 0; " +
		"PRAGMA mmap_size=65536; " +
		"PRAGMA  synchronous = NORMAL;" +
		"Attach '" + dbfile + "' as " + as + "; " +
		"PRAGMA " + as + ".cache_size = 0; " +
		"PRAGMA " + as + ".journal_mode = off; " +
		"PRAGMA cache=shared; " +
		""
	_, err := database.Exec(attach)
	if err != nil {
		database.Close()
		return xerrors.Errorf("Could not attach %s database(%s). sql(%s) err: %w", as, dbfile, attach, err)
	}
	return nil
}

// Startup will attach event handlers to handle import UDB import
func Startup(logger log.Logger, cfg *viper.Viper, am3Svc eventHandlingService, d busComponents) (func(), error) {
	// setup programatic defaults. can be overridden in config file
	cfg.SetDefault("udb.udbdatabasepath",
		"file:/run/unifieddatabase/DMLiveObjectDatabase.db?cache=shared&_foreign_keys=off&mode=ro&_busy_timeout=1000&nolock=1&cache=shared")
	cfg.SetDefault("udb.shmdatabasepath",
		"file:/run/unifieddatabase/SHM.db?cache=shared&_foreign_keys=off&mode=ro&_busy_timeout=1000&nolock=1&cache=shared")
	cfg.SetDefault("udb.udbnotifypipe", "/run/telemetryservice/udbtdbipcpipe")

	database, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, xerrors.Errorf("Could not create empty in-memory sqlite database: %w", err)
	}

	err = attachDB(database, cfg.GetString("udb.udbdatabasepath"), "udbdm")
	if err != nil {
		return nil, xerrors.Errorf("Error attaching UDB db file: %w", err)
	}

	err = attachDB(database, cfg.GetString("udb.shmdatabasepath"), "udbsm")
	if err != nil {
		return nil, xerrors.Errorf("Error attaching SHM db file: %w", err)
	}

	database.SetMaxOpenConns(1) // shouldn't need to run more than one query concurrently

	importMgr, err := newImportManager(logger, database, d, cfg)
	if err != nil {
		database.Close()
		return nil, xerrors.Errorf("Error creating udb integration: %w", err)
	}

	bus := d.GetBus()
	err = am3Svc.AddEventHandler("Import UDB Metric Values", telemetry.PublishClock, MakeHandlerUDBPeriodicImport(logger, importMgr, bus))
	if err != nil {
		return nil, xerrors.Errorf("Failed to attach event handler: %w", err)
	}
	err = am3Svc.AddEventHandler("UDB Change Notification", udbChangeEvent, MakeHandlerUDBChangeNotify(logger, importMgr, bus))
	if err != nil {
		return nil, xerrors.Errorf("Failed to attach event handler: %w", err)
	}

	// handle sync of legacy AR attributes for MRD enable/disable, etc.
	configSync, err := newConfigSync(logger, database, d)
	if err != nil {
		return nil, xerrors.Errorf("Failed to attach event handler: %w", err)
	}
	err = am3Svc.AddEventHandler(
		"UDB Cfg change Notification",
		udbChangeEvent,
		MakeHandlerLegacyAttributeSync(log.With(logger, "module", "LegacyARSync"), importMgr, bus, configSync))
	if err != nil {
		return nil, xerrors.Errorf("Failed to attach event handler: %w", err)
	}

	configSync.kickstartLegacyARConfigSync(logger, d)

	err = am3Svc.AddEventHandler("Update Metric Report Definition to Sync ConfigDB", telemetry.UpdateMRDResponseEvent, MakeHandlerUpdateMRDSyncConfigDB(logger, importMgr, bus))

	if err != nil {
		return nil, xerrors.Errorf("Failed to attach response event handler: %w", err)
	}

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

	err := GetRowID(logger, database, "Enum", cfgS.enumEntries)
	if err != nil {
		return nil, xerrors.Errorf("Failed to query legacy UDB AR values for Enum: %w", err)
	}

	err = GetRowID(logger, database, "Str", cfgS.strEntries)
	if err != nil {
		return nil, xerrors.Errorf("Failed to query legacy UDB AR values for Str: %w", err)
	}

	err = GetRowID(logger, database, "Int", cfgS.intEntries)
	if err != nil {
		return nil, xerrors.Errorf("Failed to query legacy UDB AR values for Int: %w", err)
	}

	return cfgS, err
}

func GetRowID(logger log.Logger, database *sqlx.DB, dataType string, entries map[int64]string) error {
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

scan:
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

		keys := strings.Split(key, "#")
		switch keys[2] {
		// These are all attributes that we dont care about and will skip
		case "FQDD", "DevicePollFrequency":
			continue scan
		case "RSyslogServer1", "RSyslogServer2":
			continue scan
		case "TelemetrySubscription1", "TelemetrySubscription2":
			continue scan
		case "RSyslogServer1Port", "RSyslogServer2Port":
			continue scan

		// these are attributes we need to sync or TODO soon need to sync
		case "RsyslogTarget":
			// will need to handle rsyslog eventually (TODO:...)
			continue scan
			//case enableTelemetry, reportInterval, reportTriggers:
		case enableTelemetry, reportInterval, reportTriggers:
			// WE HANDLE THESE, add to map below
		default:
			logger.Crit("Internal error. Unhandled legacy AR key, code needs to be updated!", "keyname", keys[2])
			continue scan
		}

		entries[RowID] = key
	}
	return err
}

// cfgUtilSet is a helper to shell out to the cfgutil binary for setting AR
func cfgUtilSet(logger log.Logger, key, value string) error {
	cmd := exec.Command("/usr/bin/cfgutil", "command=setattr", key, value)
	output, err := cmd.Output()
	logger.Debug("cfgutil command=setattr", "key", key, "value", value, "output", string(output), "err", err, "module", "cfgutil")
	return err
}

func MakeHandlerUpdateMRDSyncConfigDB(logger log.Logger, importMgr *importManager, bus eh.EventBus) func(eh.Event) {
	return func(event eh.Event) {
		logger.Info("Sync update to MRD config to CfgDB")
		reportDef, ok := event.Data().(*telemetry.UpdateMRDResponseData)
		if !ok {
			logger.Crit("AddMRDCommand handler got event of incorrect format at configDBSync")
			return
		}

		switch reportDef.Enabled {
		case true:
			cfgUtilSet(logger, fmt.Sprintf(ConfigDBSyncEnable, reportDef.Name), SyncValueTrue)
		case false:
			cfgUtilSet(logger, fmt.Sprintf(ConfigDBSyncEnable, reportDef.Name), SyncValueFalse)
		}
		ReportInterval := time.Duration(reportDef.Period) / time.Second
		cfgUtilSet(logger, fmt.Sprintf(ConfigDBSyncReportInterval, reportDef.Name), fmt.Sprintf(SyncValue, uint64(ReportInterval)))
	}
}

func (cs ConfigSync) kickstartLegacyARConfigSync(logger log.Logger, d busComponents) {
	events := make([]eh.EventData, 0, maxPackedEvents)
	for _, s := range []struct {
		table  string
		mapref *map[int64]string
	}{
		{"TblEnumAttribute", &cs.enumEntries},
		{"TblStrAttribute", &cs.strEntries},
		{"TblIntAttribute", &cs.intEntries},
	} {
		for rowid := range *s.mapref {
			notify := &changeNotify{Database: "DMLiveObjectDatabase.db", Table: s.table, Rowid: rowid, Operation: 0}
			events = append(events, notify)
			if len(events) > maxPackedEvents {
				publishAndWait(logger, d.GetBus(), udbChangeEvent, events)
				events = make([]eh.EventData, 0, maxPackedEvents)
			}
		}
	}
	if len(events) > 0 {
		publishAndWait(logger, d.GetBus(), udbChangeEvent, events)
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

		// Step 1: Is this a DMLiveObjectDatabase change
		if notify.Database != "DMLiveObjectDatabase.db" {
			return
		}

		// Step 2: Is this a tblEnumAttribute change, and does rowid match something we know about
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

		// step 3: exit if it's not something we found above
		if !ok {
			return
		}

		// Step 4: Generate a "UpdateMRDCommandEvent" event
		//	Ok, here first thing we need to do is do a UDB query to find the current value since UDB didn't actually send us the value
		sqlForCurrentValue := ""
		keys := strings.Split(keyname, "#")
		switch keys[2] {
		case enableTelemetry:
			sqlForCurrentValue = "select CurrentValue from TblEnumAttribute where ROWID=?"
		case reportInterval:
			sqlForCurrentValue = "select CurrentValue from TblIntAttribute where ROWID=?"

		case reportTriggers:
			sqlForCurrentValue = "select CurrentValue from TblStrAttribute where ROWID=?"

		default:
			// basically these should all be filtered out way up above
			logger.Crit("TODO: update legacy ar filter, we hit an unhandled key", "key", keys[2])
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

		// awkwardly pull out the name of the MRD to enable/disable
		updateEvent.ReportDefinitionName = keys[1][len("Telemetry") : len(keys[1])-len(".1")]

		if updateEvent.ReportDefinitionName == "" {
			logger.Crit("Skipping report definition update for malformed keyname", "keyname", keys[1])
			return
		}
		err = fmt.Errorf("unknown type of Legacy Config AR: %v", keys)
		switch keys[2] {
		case enableTelemetry:
			switch CurrentValue {
			case "Enabled":
				logger.Info("Enabling report:", "ReportName", updateEvent.ReportDefinitionName)
				err = json.Unmarshal([]byte(jsonEnableMRD), &(updateEvent.Patch))
			case "Disabled":
				logger.Info("Disabling report:", "ReportName", updateEvent.ReportDefinitionName)
				err = json.Unmarshal([]byte(jsonDisableMRD), &(updateEvent.Patch))
			default:
				logger.Crit("Got a weird value for EnableTelemetry from AR sync", "CurrentValue", CurrentValue)
			}
		case reportInterval:
			logger.Info("Set Report RecurrenceInterval", "ReportName", updateEvent.ReportDefinitionName, "Seconds", CurrentValue)
			err = json.Unmarshal([]byte(fmt.Sprintf(jsonReportTimespanMRD, CurrentValue)), &(updateEvent.Patch))

		case reportTriggers:
			logger.Info("Set ReportTriggers", "ReportName", updateEvent.ReportDefinitionName, "Triggers:", CurrentValue)
			err = json.Unmarshal([]byte(fmt.Sprintf(jsonReportTimespanMRD, CurrentValue)), &(updateEvent.Patch))

		default:
			logger.Crit("Asked to sync legacy AR attribute that I don't know about", "keyname", keys[1], "Attribute", keys[2])
		}
		if err != nil {
			logger.Crit("Legacy AR config sync error", "err", err)
			return
		}
		logger.Debug("CRIT: about to send", "report", updateEvent.ReportDefinitionName, "PATCH", string(updateEvent.Patch))
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
		return 0, nil, io.EOF
	}
	start := bytes.Index(data, []byte("||"))
	if start == -1 { // didnt find starting ||, skip over everything
		fmt.Printf("DEBUG (shouldnt happen): NO STARTING ||: len(%v), bytes(%+v), string(%v)\n", len(data), data, string(data))
		return len(data), data, nil
	}

	if len(data) < start+1 { // not enough data, read some more
		return 0, nil, nil
	}

	if start > 0 {
		fmt.Printf("DEBUG (shouldnt happen): JUNK start(%v): %v\n", start, string(data[0:start]))
		return start, data[0:0], nil
	}

	end := bytes.Index(data[start+1:], []byte("||"))
	if end == -1 { // didnt find ending ||, read some more
		return 0, nil, nil
	}

	if end == 0 { // got a ||| or ||||, consume 1 byte at a time
		fmt.Printf("DEBUG (shouldnt happen): GOT ||| or ||||, skip 2. len(%v), start(%v), end(%v): %v\n", len(data), start, end, data[start:end])
		return 1, data[0:0], nil
	}

	// consume everything between start and end markers
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
