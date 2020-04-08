package telemetry

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"
	"golang.org/x/xerrors"

	"github.com/superchalupa/sailfish/cmd/metric-engine/metric"
	log "github.com/superchalupa/sailfish/src/log"
)

// "configuration" -- TODO: need to move to config file
const (
	clockPeriod             = 1000 * time.Millisecond
	maxMetricTimestampDelta = 1 * time.Hour

	// NOTE: the numbers below are selected as PRIME numbers so that they run concurrently as infrequently as possible
	// With the default 73/3607/10831, they will run concurrently every ~90 years.
	cleanMVTime  = 307 * time.Second
	vacuumTime   = 3607 * time.Second
	optimizeTime = 10831 * time.Second
)

type busComponents interface {
	GetBus() eh.EventBus
}

type eventHandler interface {
	AddEventHandler(string, eh.EventType, func(eh.Event)) error
	AddMultiHandler(string, eh.EventType, func(eh.Event)) error
}

// publishHelper will log/eat the error from PublishEvent since we can't do anything useful with it
func publishHelper(logger log.Logger, bus eh.EventBus, event eh.Event) {
	err := bus.PublishEvent(context.Background(), event)
	if err != nil {
		logger.Crit("Error publishing event. This should never happen!", "err", err)
	}
}

func backgroundTasks(logger log.Logger, bus eh.EventBus, shutdown chan struct{}) {
	clockTicker := time.NewTicker(clockPeriod)
	cleanValuesTicker := time.NewTicker(cleanMVTime)
	vacuumTicker := time.NewTicker(vacuumTime)
	optimizeTicker := time.NewTicker(optimizeTime)

	defer cleanValuesTicker.Stop()
	defer vacuumTicker.Stop()
	defer optimizeTicker.Stop()
	defer clockTicker.Stop()
	for {
		select {
		case <-cleanValuesTicker.C:
			publishHelper(logger, bus, eh.NewEvent(DatabaseMaintenance, "clean values", time.Now()))
		case <-vacuumTicker.C:
			publishHelper(logger, bus, eh.NewEvent(DatabaseMaintenance, "vacuum", time.Now()))
		case <-optimizeTicker.C:
			publishHelper(logger, bus, eh.NewEvent(DatabaseMaintenance, "optimize", time.Now()))
			publishHelper(logger, bus, eh.NewEvent(DatabaseMaintenance, "delete orphans", time.Now())) // belt and suspenders
		case <-clockTicker.C:
			publishHelper(logger, bus, eh.NewEvent(PublishClock, nil, time.Now()))
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
func Startup(logger log.Logger, cfg *viper.Viper, am3Svc eventHandler, d busComponents) (func(), error) {
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
			return nil, xerrors.Errorf("Error running DB Create statement. SQL: %s: ERROR: %w", sqltext, err)
		}
	}

	telemetryMgr, err := newTelemetryManager(logger, database, cfg)
	if err != nil {
		database.Close()
		return nil, xerrors.Errorf("telemetry manager initialization failed: %w", err)
	}

	cfg = nil // hint to runtime that we dont need cfg after this point. dont pass this into functions below here

	// schedule database cleanup on first clock tick
	dbmaint := map[string]struct{}{
		deleteOrphans: struct{}{},
		optimize:      struct{}{},
		vacuum:        struct{}{},
		cleanValues:   struct{}{},
	}
	bus := d.GetBus()
	am3Svc.AddEventHandler("Generic GET Data", GenericGETCommandEvent, MakeHandlerGenericGET(logger, telemetryMgr, bus))
	am3Svc.AddEventHandler("Create Metric Report Definition", AddMRDCommandEvent, MakeHandlerCreateMRD(logger, telemetryMgr, bus, dbmaint))
	am3Svc.AddEventHandler("Delete Metric Report Definition", DeleteMRDCommandEvent, MakeHandlerDeleteMRD(logger, telemetryMgr, bus))
	am3Svc.AddEventHandler("Delete Metric Report", DeleteMRCommandEvent, MakeHandlerDeleteMR(logger, telemetryMgr, bus))
	am3Svc.AddEventHandler("Update Metric Report Definition", UpdateMRDCommandEvent, MakeHandlerUpdateMRD(logger, telemetryMgr, bus, dbmaint))
	am3Svc.AddEventHandler("Create Metric Definition", AddMDCommandEvent, MakeHandlerCreateMD(logger, telemetryMgr, bus))
	am3Svc.AddEventHandler("Generate Metric Report", metric.GenerateReportCommandEvent, MakeHandlerGenReport(logger, telemetryMgr, bus))

	am3Svc.AddEventHandler("Clock", PublishClock, MakeHandlerClock(logger, telemetryMgr, bus, dbmaint))
	am3Svc.AddEventHandler("Database Maintenance", DatabaseMaintenance, MakeHandlerMaintenance(logger, telemetryMgr, bus, dbmaint))
	am3Svc.AddMultiHandler("Store Metric Value(s)", metric.MetricValueEvent, MakeHandlerMV(logger, telemetryMgr, bus))

	// start background thread publishing regular maintenance tasks
	shutdown := make(chan struct{}) // channel to help this goroutine shut down cleanly. close to exit
	go backgroundTasks(logger, bus, shutdown)

	return func() { // return function that is called at shutdown to cleanly unwind things
		close(shutdown)
		database.Close()
	}, nil
}

// handle all HTTP requests for our URLs here. Need to handle all telemetry related requests.
func MakeHandlerGenericGET(logger log.Logger, telemetryMgr *telemetryManager, bus eh.EventBus) func(eh.Event) {
	return func(event eh.Event) {
		getCmd, ok := event.Data().(*GenericGETCommandData)
		if !ok {
			logger.Crit("AddMRDCommand handler got event of incorrect format")
			return
		}

		// Generate a "response" event that carries status back to initiator
		respEvent, err := getCmd.NewResponseEvent(nil)
		if err != nil {
			logger.Crit("Error creating response event", "err", err, "get", getCmd)
			return
		}

		// yes, we really are going to publish the response event before actually doing the work
		publishHelper(logger, bus, respEvent)

		// async background process to actually read database to satisfy the request
		go func() {
			err = telemetryMgr.get(getCmd, respEvent.Data().(*GenericGETResponseData))
			if err != nil {
				logger.Crit("Failed to get", "getCmd", getCmd, "err", err)
			}
		}()
	}
}

func MakeHandlerCreateMRD(logger log.Logger, telemetryMgr *telemetryManager, bus eh.EventBus, dbmaint map[string]struct{}) func(eh.Event) {
	return func(event eh.Event) {
		reportDef, ok := event.Data().(*AddMRDCommandData)
		if !ok {
			logger.Crit("AddMRDCommand handler got event of incorrect format")
			return
		}

		// schedule cleanup next clock tick
		dbmaint[deleteOrphans] = struct{}{}

		// Can't write to event sent in, so make a local copy
		localReportDefCopy := *reportDef
		addError := telemetryMgr.addMRD(&localReportDefCopy.MetricReportDefinitionData)
		if addError != nil {
			logger.Crit("Failed to create or update the Report Definition", "Name", reportDef.Name, "err", addError)
		}

		// Generate a "response" event that carries status back to initiator
		respEvent, err := reportDef.NewResponseEvent(addError)
		if err != nil {
			logger.Crit("Error creating response event", "err", err, "ReportDefinition", reportDef.Name)
			return
		}

		// Should add the populated metric report definition event as a response?
		publishHelper(logger, bus, respEvent)
	}
}

func MakeHandlerUpdateMRD(logger log.Logger, telemetryMgr *telemetryManager, bus eh.EventBus, dbmaint map[string]struct{}) func(eh.Event) {
	return func(event eh.Event) {
		update, ok := event.Data().(*UpdateMRDCommandData)
		if !ok {
			return
		}

		// make a local by-value copy of the pointer passed in
		localUpdate := *update
		updError := telemetryMgr.updateMRD(localUpdate.ReportDefinitionName, localUpdate.Patch)
		if updError != nil {
			logger.Crit("Failed to create or update the Report Definition", "Name", update.ReportDefinitionName, "err", updError)
			return
		}

		// After we've done the adjustments to ReportDefinitionToMetricMeta, there might be orphan rows. schedule maintenance
		dbmaint[deleteOrphans] = struct{}{}

		// Generate a "response" event that carries status back to initiator
		respEvent, err := update.NewResponseEvent(updError)
		if err != nil {
			logger.Crit("Error creating response event", "err", err, "ReportDefinition", update.ReportDefinitionName)
			return
		}

		// Should add the populated metric report definition event as a response?
		publishHelper(logger, bus, respEvent)
	}
}

func MakeHandlerDeleteMRD(logger log.Logger, telemetryMgr *telemetryManager, bus eh.EventBus) func(eh.Event) {
	return func(event eh.Event) {
		reportDef, ok := event.Data().(*DeleteMRDCommandData)
		if !ok {
			return
		}

		// Handle the requested command
		delError := telemetryMgr.deleteMRD(reportDef.Name)
		if delError != nil {
			logger.Crit("Error deleting Metric Report Definition", "Name", reportDef.Name, "err", delError)
		}
		err := telemetryMgr.DeleteOrphans()
		if err != nil {
			logger.Crit("Orphan delete failed", "err", err)
		}

		// Generate a "response" event that carries status back to initiator
		respEvent, err := reportDef.NewResponseEvent(delError)
		if err != nil {
			logger.Crit("Error creating response event", "err", err, "ReportDefinition", reportDef.Name)
			return
		}

		publishHelper(logger, bus, respEvent)
	}
}

// MD event handlers
func MakeHandlerCreateMD(logger log.Logger, telemetryMgr *telemetryManager, bus eh.EventBus) func(eh.Event) {
	return func(event eh.Event) {
		mdDef, ok := event.Data().(*AddMDCommandData)
		if !ok {
			logger.Crit("AddMDCommandEvent handler got event of incorrect format")
			return
		}

		// Can't write to event sent in, so make a local copy
		locaMdDefCopy := *mdDef
		addError := telemetryMgr.addMD(&locaMdDefCopy.MetricDefinitionData)
		if addError != nil {
			logger.Crit("Failed to create or update the Metric Definition", "MetricID", mdDef.MetricDefinitionData.MetricID, "err", addError)
		}

		// Generate a "response" event that carries status back to initiator
		respEvent, err := mdDef.NewResponseEvent(addError)
		if err != nil {
			logger.Crit("Error creating response event", "err", err, "MetricDefinition", mdDef.MetricDefinitionData.MetricID)
			return
		}

		publishHelper(logger, bus, respEvent)
	}
}

func MakeHandlerDeleteMR(logger log.Logger, telemetryMgr *telemetryManager, bus eh.EventBus) func(eh.Event) {
	return func(event eh.Event) {
		report, ok := event.Data().(*DeleteMRCommandData)
		if !ok {
			return
		}

		// Handle the requested command
		delError := telemetryMgr.deleteMR(report.Name)
		if delError != nil {
			logger.Crit("Error deleting Metric Report", "Name", report.Name, "err", delError)
		}

		// Generate a "response" event that carries status back to initiator
		respEvent, err := report.NewResponseEvent(delError)
		if err != nil {
			logger.Crit("Error creating response event", "err", err, "Report", report.Name)
			return
		}

		publishHelper(logger, bus, respEvent)
	}
}

func MakeHandlerGenReport(logger log.Logger, telemetryMgr *telemetryManager, bus eh.EventBus) func(eh.Event) {
	return func(event eh.Event) {
		report, ok := event.Data().(*metric.GenerateReportCommandData)
		if !ok {
			return
		}

		mrName, reportError := telemetryMgr.GenerateMetricReport(nil, report.MRDName)
		if reportError != nil {
			// dont return, because we are going to return the error to the caller
			logger.Crit("Error generating metric report", "err", reportError, "ReportDefinition", report.MRDName)
		}

		respEvent, err := report.NewResponseEvent(reportError)
		if err != nil {
			logger.Crit("Error creating response event", "err", err, "ReportDefinition", report.MRDName, "ReportName", mrName)
			return
		}

		publishHelper(logger, bus, respEvent)
		if reportError != nil {
			return
		}

		// Generate the generic "Report Generated" event that things like triggers
		// and such operate off. Only publish when there is no error generating report
		logger.Info("Generated Report", "MRD-Name", report.MRDName, "MR-Name", mrName, "module", "ReportGeneration")
		publishHelper(logger, bus,
			eh.NewEvent(metric.ReportGenerated, &metric.ReportGeneratedData{MRDName: report.MRDName, MRName: mrName}, time.Now()))
	}
}

func MakeHandlerMV(logger log.Logger, telemetryMgr *telemetryManager, bus eh.EventBus) func(eh.Event) {
	return func(event eh.Event) {
		// This is a MULTI Handler! This function is called with an ARRAY of event
		// data, not the normal single event data.  This means we can wrap the
		// insert in a transaction and insert everything in the array in a single
		// transaction for a good performance boost.
		instancesUpdated := map[int64]struct{}{}
		err := telemetryMgr.wrapWithTX(func(tx *sqlx.Tx) error {
			dataArray, ok := event.Data().([]eh.EventData)
			if !ok {
				return nil
			}
			for _, eventData := range dataArray {
				metricValue, ok := eventData.(*metric.MetricValueEventData)
				if !ok {
					continue
				}

				err := telemetryMgr.InsertMetricValue(tx, metricValue, func(instanceid int64) { instancesUpdated[instanceid] = struct{}{} })
				if err != nil {
					logger.Crit("Error Inserting Metric Value", "Metric", metricValue, "err", err)
					continue
				}

				delta := telemetryMgr.MetricTSHWM.Sub(metricValue.Timestamp.Time)

				if (!telemetryMgr.MetricTSHWM.IsZero()) && (delta > maxMetricTimestampDelta || delta < -maxMetricTimestampDelta) {
					// if you see this warning consistently, check the import to ensure it's using UTC and not localtime
					fmt.Printf("Warning: Metric Value Event TIME OFF >1hr - (delta: %s)  Metric: %+v\n", delta, metricValue)
				}

				if telemetryMgr.MetricTSHWM.Before(metricValue.Timestamp.Time) {
					telemetryMgr.MetricTSHWM = metricValue.Timestamp.Time
				}
			}
			return nil
		})
		if err != nil {
			logger.Crit("critical error storing metric value", "err", err)
		}

		// this will set telemetryMgr.NextMRTS = telemetryMgr.LastMRTS+5s for any reports that have changes
		err = telemetryMgr.CheckOnChangeReports(nil, instancesUpdated)
		if err != nil {
			logger.Crit("Error Finding OnChange Reports for metrics", "instancesUpdated", instancesUpdated, "err", err)
		}
	}
}

func MakeHandlerClock(logger log.Logger, telemetryMgr *telemetryManager, bus eh.EventBus, dbmaint map[string]struct{}) func(eh.Event) {
	// close over lastHWM
	lastHWM := time.Time{}
	return func(event eh.Event) {
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
			publishHelper(logger, bus, eh.NewEvent(metric.ReportGenerated, &metric.ReportGeneratedData{MRDName: mrd, MRName: mr}, time.Now()))
		}

		// Generate any metric reports that need it
		telemetryMgr.FastCheckForNeededMRUpdates(pubReport)

		for k, _ := range dbmaint {
			runMaintenanceCommand(logger, telemetryMgr, k)
			delete(dbmaint, k)
		}
	}
}

func MakeHandlerMaintenance(logger log.Logger, telemetryMgr *telemetryManager, _ eh.EventBus, dbmaint map[string]struct{}) func(eh.Event) {
	return func(event eh.Event) {
		command, ok := event.Data().(string)
		if !ok {
			return
		}
		dbmaint[command] = struct{}{}
	}
}

const (
	optimize      = "optimize"
	vacuum        = "vacuum"
	cleanValues   = "clean values"
	deleteOrphans = "delete orphans"
	prunemv       = "prune unused metric values"
)

func runMaintenanceCommand(logger log.Logger, telemetryMgr *telemetryManager, command string) {
	var err error
	switch command {
	case optimize:
		logger.Info("Running scheduled database optimization")
		err = telemetryMgr.Optimize()
		if err != nil {
			logger.Crit("Optimize failed", "err", err)
		}

	case vacuum:
		logger.Info("Running scheduled database storage recovery")
		err = telemetryMgr.Vacuum()
		if err != nil {
			logger.Crit("Vacuum failed", "err", err)
		}

	case cleanValues: // keep us under database size limits
		logger.Info("Running scheduled cleanup of the stored Metric Values")
		err = telemetryMgr.DeleteOldestValues()
		if err != nil {
			logger.Crit("DeleteOldestValues failed.", "err", err)
		}

	case deleteOrphans: // see factory comment for details.
		logger.Info("Running scheduled database consistency cleanup")
		err = telemetryMgr.DeleteOrphans()
		if err != nil {
			logger.Crit("Orphan delete failed", "err", err)
		}

	case prunemv:
		logger.Info("Running scheduled cleanup of the stored Metric Values")
		err = telemetryMgr.DeleteOldestValues()
		if err != nil {
			logger.Crit("DeleteOldestValues failed.", "err", err)
		}
		logger.Info("Running scheduled database consistency cleanup")
		err = telemetryMgr.DeleteOrphans()
		if err != nil {
			logger.Crit("Orphan delete failed", "err", err)
		}

	default:
		logger.Warn("Unknown database maintenance command string received", "command", command)
	}
}
