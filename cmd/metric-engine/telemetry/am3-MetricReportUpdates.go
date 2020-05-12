package telemetry

import (
	"context"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"
	"golang.org/x/xerrors"

	"github.com/superchalupa/sailfish/cmd/metric-engine/eemi"
	"github.com/superchalupa/sailfish/cmd/metric-engine/metric"
	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/event"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
	"github.com/superchalupa/sailfish/src/ocp/am3"
)

// "configuration" -- TODO: need to move to config file
const (
	clockPeriod             = 1000 * time.Millisecond
	maxMetricTimestampDelta = 1 * time.Hour

	// NOTE: the numbers below are selected as PRIME numbers so that they run concurrently as infrequently as possible
	// With the default 73/3607/10831, they will run concurrently every ~90 years.
	cleanValuesTime = 307 * time.Second
	vacuumTime      = 3607 * time.Second
	optimizeTime    = 10831 * time.Second
)

const (
	// cant change
	mrdPath = "/redfish/v1/TelemetryService/MetricReportDefinitions/"
	trgPath = "/redfish/v1/TelemetryService/Triggers/"

	// often used strings
	optimize      = "optimize"
	vacuum        = "vacuum"
	cleanValues   = "clean values"
	deleteOrphans = "delete orphans"

	// error strings
	reportDefinition = "ReportDefinition"
	triggerDef       = "Trigger"
	typeAssertError  = "handler got event of incorrect format"
	maintfail        = "Maint failed"
	maintstart       = "Run DB Maintenance Op"
	respCreateError  = "create response event"
)

type urisetter interface {
	SetURI(string)
}

type busComponents interface {
	GetBus() eh.EventBus
	GetWaiter() *eventwaiter.EventWaiter
}

func backgroundTasks(logger log.Logger, bus eh.EventBus, shutdown chan struct{}) {
	ctx := context.Background()

	clockTicker := time.NewTicker(clockPeriod)
	cleanValuesTicker := time.NewTicker(cleanValuesTime)
	vacuumTicker := time.NewTicker(vacuumTime)
	optimizeTicker := time.NewTicker(optimizeTime)

	defer cleanValuesTicker.Stop()
	defer vacuumTicker.Stop()
	defer optimizeTicker.Stop()
	defer clockTicker.Stop()

	// all these are Sync Events, no real value in having >1 in flight at a time
	for {
		select {
		case <-cleanValuesTicker.C:
			event.PublishAndWait(ctx, bus, DatabaseMaintenance, cleanValues)
		case <-vacuumTicker.C:
			event.PublishAndWait(ctx, bus, DatabaseMaintenance, vacuum)
		case <-optimizeTicker.C:
			event.PublishAndWait(ctx, bus, DatabaseMaintenance, optimize)
			event.PublishAndWait(ctx, bus, DatabaseMaintenance, deleteOrphans)
		case <-clockTicker.C:
			event.PublishAndWait(ctx, bus, PublishClock, nil)
		case <-shutdown:
			return
		}
	}
}

// Startup registers event handlers with the awesome mapper and
// starts up timers and maintenance goroutines
//
// regarding: database.SetMaxOpenConns(1)
// If we run in WAL mode, you can only do one connection. Seems like a base
// library limitation that's reflected up into the golang implementation.
// SO: we will ensure that we have ONLY ONE GOROUTINE that does transactions.
// This isn't a terrible limitation as it is sort of what we want to do
// anyways.
//
// Don't leak cfg argument out of this function
func Startup(logger log.Logger, cfg *viper.Viper, am3Svc am3.Service, d busComponents) (func(), error) {
	database, err := sqlx.Open("sqlite3", cfg.GetString("main.databasepath"))
	if err != nil {
		return nil, xerrors.Errorf("could not open database(%s): %w", cfg.GetString("main.databasepath"))
	}

	database.SetMaxOpenConns(1) // see note above. WAL mode requires this

	// Create tables and views from sql stored in our YAML
	for _, sqltext := range cfg.GetStringSlice("createdb") {
		_, err = database.Exec(sqltext)
		if err != nil {
			// ignore drop errors. can happen if we have old telemetry db and are ok
			if strings.HasPrefix(err.Error(), "use DROP") {
				logger.Info("Ignoring SQL error dropping table/view", "err", err, "sql", sqltext)
				continue
			}
			database.Close()
			return nil, xerrors.Errorf("DB Error. SQL: %s: ERROR: %w", sqltext, err)
		}
	}

	telemetryMgr, err := newTelemetryManager(logger, database, cfg)
	if err != nil {
		database.Close()
		return nil, xerrors.Errorf("telemetry manager initialization failed: %w", err)
	}

	// can return a promise, but we'll wait here otherwise it could hang if we do it from a message handler
	msgreg := eemi.DeferredGetMsgreg(logger, d)()

	err = addEventHandlers(logger, am3Svc, telemetryMgr, msgreg, d)
	if err != nil {
		database.Close()
		return nil, xerrors.Errorf("error adding am3 event handlers: %w", err)
	}

	// start background thread publishing regular maintenance tasks
	shutdown := make(chan struct{}) // channel to help this goroutine shut down cleanly. close to exit
	go backgroundTasks(logger, d.GetBus(), shutdown)

	return func() { // return function that is called at shutdown to cleanly unwind things
		close(shutdown)
		database.Close()
	}, nil
}

func addEventHandlers(
	logger log.Logger,
	am3Svc am3.Service,
	telemetryMgr *telemetryManager,
	msgreg eemi.MessageRegistry,
	d busComponents,
) error {
	bus := d.GetBus()

	// use this to keep track of maintenance tasks to run on the next clock tick.
	// start out by priming for cleanup tasks on startup
	dbmaint := map[string]struct{}{
		deleteOrphans: {},
		optimize:      {},
		vacuum:        {},
		cleanValues:   {},
	}

	for _, h := range []struct {
		desc    string
		evtType eh.EventType
		fn      am3.EventHandler
	}{
		{"Generic GET Data", GenericGETCommandEvent, MakeHandlerGenericGET(logger, telemetryMgr, msgreg, bus)},
		{"Create Metric Report Definition", AddMRDCommandEvent, MakeHandlerCreateMRD(logger, telemetryMgr, msgreg, bus, dbmaint)},
		{"Update Metric Report Definition", UpdateMRDCommandEvent, MakeHandlerUpdateMRD(logger, telemetryMgr, msgreg, bus, dbmaint)},
		{"Delete Metric Report Definition", DeleteMRDCommandEvent, MakeHandlerDeleteMRD(logger, telemetryMgr, msgreg, bus, dbmaint)},
		{"Create Metric Definition", AddMDCommandEvent, MakeHandlerCreateMD(logger, telemetryMgr, msgreg, bus)},
		{"Delete Metric Report", DeleteMRCommandEvent, MakeHandlerDeleteMR(logger, telemetryMgr, msgreg, bus)},

		{"Add Trigger", AddTriggerCommandEvent, MakeHandlerAddTrigger(logger, telemetryMgr, msgreg, bus, dbmaint)},
		{"Update Trigger", UpdateTriggerCommandEvent, MakeHandlerUpdateTrigger(logger, telemetryMgr, msgreg, bus, dbmaint)},
		{"Delete Trigger", DeleteTriggerCommandEvent, MakeHandlerDeleteTrigger(logger, telemetryMgr, msgreg, bus, dbmaint)},

		{"Generate Metric Report", metric.GenerateReportCommandEvent, MakeHandlerGenReport(logger, telemetryMgr, msgreg, bus)},
		{"Clock", PublishClock, MakeHandlerClock(logger, telemetryMgr, bus, dbmaint)},
		{"Database Maintenance", DatabaseMaintenance, MakeHandlerMaintenance(logger, dbmaint)},
	} {
		err := am3Svc.AddEventHandler(h.desc, h.evtType, h.fn)
		if err != nil {
			return err
		}
	}

	err := am3Svc.AddMultiHandler("Store Metric Value(s)", metric.MetricValueEvent, MakeHandlerMV(logger, telemetryMgr, bus))
	if err != nil {
		return err
	}

	return nil
}

// handle all HTTP requests for our URLs here. Need to handle all telemetry related requests.
func MakeHandlerGenericGET(
	logger log.Logger,
	telemetryMgr *telemetryManager,
	msgreg eemi.MessageRegistry,
	bus eh.EventBus,
) func(eh.Event) {
	return func(evt eh.Event) {
		getCmd, ok := evt.Data().(*GenericGETCommandData)
		if !ok {
			logger.Crit(typeAssertError)
			return
		}

		// Generate a "response" event that we use to respond
		respEvent, err := getCmd.NewStreamingResponse()
		if err != nil {
			logger.Crit(respCreateError, "err", err, "get", getCmd)
			return
		}

		resp, ok := respEvent.Data().(*GenericGETResponseData)
		if !ok {
			logger.Crit(typeAssertError)
			return
		}

		resp.WriteDefaultHeaders()

		go func() {
			// leverage automatic SetStatus(HTTPStatusOK) on first Write()
			err := telemetryMgr.get(getCmd.URI, resp)
			if err != nil {
				logger.Crit("telemetry get uri", "getCmd", getCmd, "err", err)
				resp.WriteStatus(metric.HTTPStatusNotFound)
				_, err = resp.Write([]byte("Resource not found. (FIXME: replace with redfish compliant error text.)"))
				if err != nil {
					logger.Crit("write", "err", err)
				}
			}
			// wont actually wait since event is not sync
			event.PublishEventAndWait(context.Background(), bus, respEvent)
		}()
	}
}

func MakeHandlerCreateMRD(logger log.Logger,
	telemetryMgr *telemetryManager,
	msgreg eemi.MessageRegistry,
	bus eh.EventBus,
	dbmaint map[string]struct{},
) func(eh.Event) {
	return func(evt eh.Event) {
		reportDef, ok := evt.Data().(*AddMRDCommandData)
		if !ok {
			logger.Crit(typeAssertError)
			return
		}

		// schedule cleanup next clock tick
		dbmaint[deleteOrphans] = struct{}{}

		addError := telemetryMgr.addMRD(reportDef.MetricReportDefinitionData)
		if addError != nil {
			logger.Crit("create report definition", "Name", reportDef.Name, "err", addError)
		}

		// Generate a "response" event that carries status back to initiator
		respEvent, err := reportDef.NewResponseEvent(msgreg, addError)
		if err != nil {
			logger.Crit(respCreateError, "err", err, reportDefinition, reportDef.Name)
			return
		}
		mrd := mrdPath + reportDef.Name

		r, ok := respEvent.Data().(urisetter)
		if ok {
			r.SetURI(mrd)
		}

		// Should add the populated metric report definition event as a response?
		// wont actually wait since event is not sync
		event.PublishEventAndWait(context.Background(), bus, respEvent)
	}
}

func MakeHandlerUpdateMRD(
	logger log.Logger,
	telemetryMgr *telemetryManager,
	msgreg eemi.MessageRegistry,
	bus eh.EventBus,
	dbmaint map[string]struct{},
) func(eh.Event) {
	return func(evt eh.Event) {
		update, ok := evt.Data().(*UpdateMRDCommandData)
		if !ok {
			logger.Crit(typeAssertError)
			return
		}

		// make a local by-value copy of the pointer passed in
		localUpdate := *update
		updatedMRD, updError := telemetryMgr.updateMRD(localUpdate.ReportDefinitionName, localUpdate.Patch)
		if updError != nil {
			logger.Crit("update report definition", "Name", update.ReportDefinitionName, "err", updError)
			return
		}

		// After we've done the adjustments to ReportDefinitionToMetricMeta, there might be orphan rows. schedule maintenance
		dbmaint[deleteOrphans] = struct{}{}

		// Generate a "response" event that carries status back to initiator
		respEvent, err := localUpdate.NewResponseEvent(msgreg, updError)
		if err != nil {
			logger.Crit(respCreateError, "err", err, reportDefinition, update.ReportDefinitionName)
			return
		}

		respData, ok := respEvent.Data().(*UpdateMRDResponseData)
		if ok {
			respData.MetricReportDefinitionData = updatedMRD.MetricReportDefinitionData
		}

		r, ok := respEvent.Data().(urisetter)
		if ok {
			r.SetURI(mrdPath + localUpdate.ReportDefinitionName)
		}

		// Should add the populated metric report definition event as a response?
		// wont actually wait since event is not sync
		event.PublishEventAndWait(context.Background(), bus, respEvent)
	}
}

func MakeHandlerDeleteMRD(
	logger log.Logger,
	telemetryMgr *telemetryManager,
	msgreg eemi.MessageRegistry,
	bus eh.EventBus,
	dbmaint map[string]struct{},
) func(eh.Event) {
	return func(evt eh.Event) {
		reportDef, ok := evt.Data().(*DeleteMRDCommandData)
		if !ok {
			logger.Crit(typeAssertError)
			return
		}

		delError := telemetryMgr.deleteMRD(reportDef.Name)
		if delError != nil {
			logger.Crit("delete metric report definition", "Name", reportDef.Name, "err", delError)
		}

		dbmaint[deleteOrphans] = struct{}{} // set bit to start orphan delete next clock tick

		// Generate a "response" event that carries status back to initiator
		respEvent, err := reportDef.NewResponseEvent(msgreg, delError)
		if err != nil {
			logger.Crit(respCreateError, "err", err, reportDefinition, reportDef.Name)
			return
		}

		mrd := mrdPath + reportDef.Name
		r, ok := respEvent.Data().(urisetter)
		if ok {
			r.SetURI(mrd)
		}

		// wont actually wait since event is not sync
		event.PublishEventAndWait(context.Background(), bus, respEvent)
	}
}

// MD event handlers
func MakeHandlerCreateMD(
	logger log.Logger,
	telemetryMgr *telemetryManager,
	msgreg eemi.MessageRegistry,
	bus eh.EventBus,
) func(eh.Event) {
	return func(evt eh.Event) {
		mdDef, ok := evt.Data().(*AddMDCommandData)
		if !ok {
			logger.Crit(typeAssertError)
			return
		}

		addError := telemetryMgr.addMD(mdDef.MetricDefinitionData)
		if addError != nil {
			logger.Crit("create metric definition", "MetricID", mdDef.MetricID, "err", addError)
		}

		// Generate a "response" event that carries status back to initiator
		respEvent, err := mdDef.NewResponseEvent(msgreg, addError)
		if err != nil {
			logger.Crit(respCreateError, "err", err, "MetricDefinition", mdDef.MetricID)
			return
		}

		// wont actually wait since event is not sync
		event.PublishEventAndWait(context.Background(), bus, respEvent)
	}
}

// Trigger event handlers
func MakeHandlerAddTrigger(
	logger log.Logger,
	telemetryMgr *telemetryManager,
	msgreg eemi.MessageRegistry,
	bus eh.EventBus,
	dbmaint map[string]struct{},
) func(eh.Event) {
	return func(evt eh.Event) {
		trigger, ok := evt.Data().(*AddTriggerCommandData)
		if !ok {
			logger.Crit(typeAssertError)
			return
		}

		// schedule cleanup next clock tick
		dbmaint[deleteOrphans] = struct{}{}

		addError := telemetryMgr.addTrigger(trigger.TriggerData)
		if addError != nil {
			logger.Crit("Failed to create the Trigger", "Id", trigger.RedfishID, "err", addError)
		}

		// Generate a "response" event that carries status back to initiator
		respEvent, err := trigger.NewResponseEvent(msgreg, addError)
		if err != nil {
			logger.Crit(respCreateError, "err", err, triggerDef, trigger.RedfishID)
			return
		}
		trg := trgPath + trigger.RedfishID

		r, ok := respEvent.Data().(urisetter)
		if ok {
			r.SetURI(trg)
		}

		// wont actually wait since event is not sync
		event.PublishEventAndWait(context.Background(), bus, respEvent)
	}
}

func MakeHandlerUpdateTrigger(
	logger log.Logger,
	telemetryMgr *telemetryManager,
	msgreg eemi.MessageRegistry,
	bus eh.EventBus,
	dbmaint map[string]struct{},
) func(eh.Event) {
	return func(evt eh.Event) {
		update, ok := evt.Data().(*UpdateTriggerCommandData)
		if !ok {
			logger.Crit(typeAssertError)
			return
		}

		updError := telemetryMgr.updateTrigger(update.TriggerName, update.Patch)
		if updError != nil {
			logger.Crit("Failed to update the Report Definition", "Name", update.TriggerName, "err", updError)
			return
		}

		// After we've done the adjustments to TriggerToMetricMeta, there might be orphan rows. schedule maintenance
		dbmaint[deleteOrphans] = struct{}{}

		// Generate a "response" event that carries status back to initiator
		respEvent, err := update.NewResponseEvent(msgreg, updError)
		if err != nil {
			logger.Crit("Error creating response event", "err", err, triggerDef, update.TriggerName)
			return
		}

		trg := trgPath + update.TriggerName
		r, ok := respEvent.Data().(urisetter)
		if ok {
			r.SetURI(trg)
		}

		// wont actually wait since event is not sync
		event.PublishEventAndWait(context.Background(), bus, respEvent)
	}
}

func MakeHandlerDeleteTrigger(
	logger log.Logger,
	telemetryMgr *telemetryManager,
	msgreg eemi.MessageRegistry,
	bus eh.EventBus,
	dbmaint map[string]struct{},
) func(eh.Event) {
	return func(evt eh.Event) {
		trigger, ok := evt.Data().(*DeleteTriggerCommandData)
		if !ok {
			logger.Crit(typeAssertError)
			return
		}

		delError := telemetryMgr.deleteTrigger(trigger.Name)
		if delError != nil {
			logger.Crit("Error deleting Trigger", "Name", trigger.Name, "err", delError)
		}

		dbmaint[deleteOrphans] = struct{}{} // set bit to start orphan delete next clock tick

		// Generate a "response" event that carries status back to initiator
		respEvent, err := trigger.NewResponseEvent(msgreg, delError)
		if err != nil {
			logger.Crit(respCreateError, "err", err, triggerDef, trigger.Name)
			return
		}

		trg := trgPath + trigger.Name
		r, ok := respEvent.Data().(urisetter)
		if ok {
			r.SetURI(trg)
		}

		// wont actually wait since event is not sync
		event.PublishEventAndWait(context.Background(), bus, respEvent)
	}
}

func MakeHandlerDeleteMR(
	logger log.Logger,
	telemetryMgr *telemetryManager,
	msgreg eemi.MessageRegistry,
	bus eh.EventBus,
) func(eh.Event) {
	return func(evt eh.Event) {
		report, ok := evt.Data().(*DeleteMRCommandData)
		if !ok {
			logger.Crit(typeAssertError)
			return
		}

		// Handle the requested command
		delError := telemetryMgr.deleteMR(report.Name)
		if delError != nil {
			logger.Crit("delete metric report", "Name", report.Name, "err", delError)
		}

		// Generate a "response" event that carries status back to initiator
		respEvent, err := report.NewResponseEvent(msgreg, delError)
		if err != nil {
			logger.Crit(respCreateError, "err", err, "Report", report.Name)
			return
		}

		// wont actually wait since event is not sync
		event.PublishEventAndWait(context.Background(), bus, respEvent)
	}
}

func MakeHandlerGenReport(
	logger log.Logger,
	telemetryMgr *telemetryManager,
	msgreg eemi.MessageRegistry,
	bus eh.EventBus,
) func(eh.Event) {
	return func(evt eh.Event) {
		report, ok := evt.Data().(*metric.GenerateReportCommandData)
		if !ok {
			logger.Crit(typeAssertError)
			return
		}

		mrName, reportError := telemetryMgr.GenerateMetricReport(nil, report.MRDName)
		if reportError != nil {
			// dont return, because we are going to return the error to the caller
			logger.Crit("generate report", "err", reportError, reportDefinition, report.MRDName)
		}

		respEvent, err := report.NewResponseEvent(msgreg, reportError)
		if err != nil {
			logger.Crit(respCreateError, "err", err, reportDefinition, report.MRDName, "ReportName", mrName)
			return
		}

		// wont actually wait since event is not sync
		event.PublishEventAndWait(context.Background(), bus, respEvent)
		if reportError != nil {
			return
		}

		// Generate the generic "Report Generated" event that things like triggers
		// and such operate off. Only publish when there is no error generating report
		logger.Info("Generated Report", "MRD-Name", report.MRDName, "MR-Name", mrName, "module", "ReportGeneration")
		event.Publish(
			context.Background(),
			bus,
			metric.ReportGenerated,
			&metric.ReportGeneratedData{MRDName: report.MRDName, MRName: mrName})
	}
}

func MakeHandlerMV(logger log.Logger, telemetryMgr *telemetryManager, bus eh.EventBus) func(eh.Event) {
	return func(evt eh.Event) {
		// This is a MULTI Handler! This function is called with an ARRAY of event
		// data, not the normal single event data.  This means we can wrap the
		// insert in a transaction and insert everything in the array in a single
		// transaction for a good performance boost.
		instancesUpdated := map[int64]struct{}{}
		err := telemetryMgr.wrapWithTX(func(tx *sqlx.Tx) error {
			dataArray, ok := evt.Data().([]eh.EventData)
			if !ok {
				logger.Crit(typeAssertError)
				return nil
			}
			for _, eventData := range dataArray {
				metricValue, ok := eventData.(*metric.MetricValueEventData)
				if !ok {
					logger.Crit(typeAssertError)
					continue
				}

				err := telemetryMgr.InsertMetricValue(tx, *metricValue, func(instanceid int64) { instancesUpdated[instanceid] = struct{}{} })
				if err != nil {
					logger.Crit("inserting metric value", "Metric", *metricValue, "err", err)
					continue
				}

				delta := telemetryMgr.MetricTSHWM.Sub(metricValue.Timestamp.Time)

				if (!telemetryMgr.MetricTSHWM.IsZero()) && (delta > maxMetricTimestampDelta || delta < -maxMetricTimestampDelta) {
					// if you see this warning consistently, check the import to ensure it's using UTC and not localtime
					logger.Warn("metric value event time off", "MaxDelta", maxMetricTimestampDelta, "delta", delta, "Event", *metricValue)
				}

				if telemetryMgr.MetricTSHWM.Before(metricValue.Timestamp.Time) {
					telemetryMgr.MetricTSHWM = metricValue.Timestamp.Time
				}
			}
			return nil
		})
		if err != nil {
			logger.Crit("store metric value", "err", err)
		}

		// this will set telemetryMgr.NextMRTS = telemetryMgr.LastMRTS+5s for any reports that have changes
		err = telemetryMgr.CheckOnChangeReports(nil, instancesUpdated)
		if err != nil {
			logger.Crit("check onchange reports", "instancesUpdated", instancesUpdated, "err", err)
		}
	}
}

func MakeHandlerClock(logger log.Logger, telemetryMgr *telemetryManager, bus eh.EventBus, dbmaint map[string]struct{}) func(eh.Event) {
	// close over lastHWM
	lastHWM := time.Time{}
	return func(evt eh.Event) {
		// if no events have kickstarted the clock, bail
		if telemetryMgr.MetricTSHWM.IsZero() {
			return
		}

		// if no events come in during time between clock publishes, we'll artificially bump HWM forward.
		// if time is uninitialized, wait for an event to come in to set it
		if telemetryMgr.MetricTSHWM.Equal(lastHWM) {
			telemetryMgr.MetricTSHWM = telemetryMgr.MetricTSHWM.Add(clockPeriod)
		}
		lastHWM = telemetryMgr.MetricTSHWM

		pubReport := func(mrd string, mr string) {
			logger.Info("Generated Report", "MRD-Name", mrd, "MR-Name", mr, "module", "ReportGeneration")
			event.Publish(
				context.Background(),
				bus,
				metric.ReportGenerated,
				&metric.ReportGeneratedData{MRDName: mrd, MRName: mr})
		}

		// Generate any metric reports that need it
		telemetryMgr.FastCheckForNeededMRUpdates(pubReport)

		for k := range dbmaint {
			runMaintenanceCommand(logger, telemetryMgr, k)
			delete(dbmaint, k)
			break // only run one maintenance thing per tick
		}
	}
}

func MakeHandlerMaintenance(logger log.Logger, dbmaint map[string]struct{}) func(eh.Event) {
	return func(evt eh.Event) {
		command, ok := evt.Data().(string)
		if !ok {
			logger.Crit(typeAssertError)
			return
		}
		dbmaint[command] = struct{}{}
	}
}

func runMaintenanceCommand(logger log.Logger, telemetryMgr *telemetryManager, command string) {
	var err error
	switch command {
	case optimize:
		logger.Info(maintstart, "op", command)
		err = telemetryMgr.runSQLFromList(telemetryMgr.optimizeops, "Database Maintenance: Optimize", "Optimization failed-> '%s': %w")

	case vacuum:
		logger.Info(maintstart, "op", command)
		err = telemetryMgr.Vacuum()

	case cleanValues: // keep us under database size limits
		logger.Info(maintstart, "op", command)
		err = telemetryMgr.runSQLFromList(telemetryMgr.deleteops, "Database Maintenance: Delete Oldest Metric Values", "Value cleanup failed-> '%s': %w")

	case deleteOrphans: // see factory comment for details.
		logger.Info(maintstart, "op", command)
		err = telemetryMgr.runSQLFromList(telemetryMgr.orphanops, "Database Maintenance: Delete Orphans", "Orphan cleanup failed-> '%s': %w")

	default:
		logger.Warn("Unknown database maintenance command string received", "command", command)
	}

	if err != nil {
		logger.Crit(maintfail, "op", command, "err", err)
	}
}