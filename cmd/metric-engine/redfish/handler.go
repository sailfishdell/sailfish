// +build redfish

package redfish

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/spf13/viper"
	"golang.org/x/xerrors"

	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/sailfish/cmd/metric-engine/metric"
	"github.com/superchalupa/sailfish/cmd/metric-engine/telemetry"
	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
)

const (
	// purely redfish centric events
	SubmitTestMetricReportCommandEvent  eh.EventType = "SubmitTestMetricReportCommandEvent"
	SubmitTestMetricReportResponseEvent eh.EventType = "SubmitTestMetricReportResponseEvent"

	defaultRequestTimeout = 5 * time.Second
)

type busComponents interface {
	GetBus() eh.EventBus
	GetWaiter() *eventwaiter.EventWaiter
}

type RFServer struct {
	logger log.Logger
	d      busComponents
}

type SubmitTestMetricReportCommandData struct {
	metric.Command
	MetricReport json.RawMessage
}

type SubmitTestMetricReportResponseData struct {
	metric.CommandResponse
}

func (u *SubmitTestMetricReportCommandData) UseInput(ctx context.Context, logger log.Logger, r io.Reader) error {
	decoder := json.NewDecoder(r)
	return decoder.Decode(&u.MetricReport)
}

func RegisterEvents() {
	// register events
	eh.RegisterEventData(SubmitTestMetricReportCommandEvent, func() eh.EventData {
		return &SubmitTestMetricReportCommandData{Command: metric.NewCommand(SubmitTestMetricReportResponseEvent)}
	})
	eh.RegisterEventData(SubmitTestMetricReportResponseEvent, func() eh.EventData { return &SubmitTestMetricReportResponseData{} })
}

func NewRedfishServer(logger log.Logger, d busComponents) *RFServer {
	return &RFServer{logger: logger, d: d}
}

func (rf *RFServer) AddHandlersToRouter(m *mux.Router) {
	// SubmitTestMetricReport
	m.HandleFunc("/redfish/v1/TelemetryService/Actions/TelemetryService.SubmitTestMetricReport",
		rf.makeCommand(SubmitTestMetricReportCommandEvent)).Methods("POST")
	// MetricReportDefinitions
	m.HandleFunc("/redfish/v1/TelemetryService/MetricReportDefinitions", rf.makeCommand(telemetry.AddMRDCommandEvent)).Methods("POST")
	m.HandleFunc("/redfish/v1/TelemetryService/MetricReportDefinitions/{ID}", rf.makeCommand(telemetry.UpdateMRDCommandEvent)).Methods("PATCH")
	m.HandleFunc("/redfish/v1/TelemetryService/MetricReportDefinitions/{ID}", rf.makeCommand(telemetry.UpdateMRDCommandEvent)).Methods("PUT")
	m.HandleFunc("/redfish/v1/TelemetryService/MetricReportDefinitions/{ID}", rf.makeCommand(telemetry.DeleteMRDCommandEvent)).Methods("DELETE")
	// MetricReports
	m.HandleFunc("/redfish/v1/TelemetryService/MetricReports/{ID}", rf.makeCommand(telemetry.DeleteMRCommandEvent)).Methods("DELETE")
	// Triggers
	m.HandleFunc("/redfish/v1/TelemetryService/Triggers", rf.makeCommand(telemetry.AddTriggerCommandEvent)).Methods("POST")
	m.HandleFunc("/redfish/v1/TelemetryService/Triggers/{ID}", rf.makeCommand(telemetry.UpdateTriggerCommandEvent)).Methods("PATCH")
	m.HandleFunc("/redfish/v1/TelemetryService/Triggers/{ID}", rf.makeCommand(telemetry.UpdateTriggerCommandEvent)).Methods("PUT")
	m.HandleFunc("/redfish/v1/TelemetryService/Triggers/{ID}", rf.makeCommand(telemetry.DeleteTriggerCommandEvent)).Methods("DELETE")
	// generic handler last
	m.PathPrefix("/redfish/v1/TelemetryService").HandlerFunc(rf.makeCommand(telemetry.GenericGETCommandEvent)).Methods("GET")
}

type eventHandler interface {
	AddEventHandler(string, eh.EventType, func(eh.Event)) error
}

func Startup(logger log.Logger, cfg *viper.Viper, am3Svc eventHandler, d busComponents) error {
	// Important: don't leak 'cfg' outside the scope of this function!
	err := am3Svc.AddEventHandler("Submit Test Metric Report", SubmitTestMetricReportCommandEvent, MakeHandlerSubmitTestMR(logger, d.GetBus()))
	if err != nil {
		return xerrors.Errorf("could not add redfish am3 event handlers: %w", err)
	}
	return nil
}

func MakeHandlerSubmitTestMR(logger log.Logger, bus eh.EventBus) func(eh.Event) {
	// TODO: this function will need to open pipes and write out the MR
	return func(event eh.Event) {
		testMR, ok := event.Data().(*SubmitTestMetricReportCommandData)
		if !ok {
			logger.Crit("handler got event of incorrect format")
			return
		}

		fmt.Printf("\nSUBMIT TEST METRIC REPORT\n")

		// Generate a "response" event that carries status back to initiator
		respEvent, err := testMR.NewResponseEvent(nil)
		if err != nil {
			logger.Crit("Error creating response event", "err", err, "testmr", testMR)
			return
		}

		// Should add the populated metric report definition event as a response?
		err = bus.PublishEvent(context.Background(), respEvent)
		if err != nil {
			logger.Crit("Error publishing", "err", err)
		}
	}
}

// optional interface
type inputUser interface {
	UseInput(context.Context, log.Logger, io.Reader) error
}

// optional interface
type varUser interface {
	UseVars(context.Context, log.Logger, map[string]string) error
}

func forwardInputAndVars(timeoutCtx context.Context, requestLogger log.Logger, w http.ResponseWriter, r *http.Request, cmd interface{}) {
	if d, ok := cmd.(inputUser); ok {
		err := d.UseInput(timeoutCtx, requestLogger, r.Body)
		if err != nil {
			requestLogger.Warn("error from command UseInput()", "err", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	if d, ok := cmd.(varUser); ok {
		vars := mux.Vars(r)
		vars["uri"] = r.URL.Path
		err := d.UseVars(timeoutCtx, requestLogger, vars)
		if err != nil {
			requestLogger.Warn("error from command UseVars()", "err", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
}

func (rf *RFServer) makeContextAndLogger(r *http.Request, cmd metric.Commander) (context.Context, log.Logger, func()) {
	ctx := r.Context()
	ctx = log.WithRequestID(ctx, cmd.GetRequestID())
	timeoutCtx, cancel := context.WithTimeout(ctx, defaultRequestTimeout)
	cmd.SetContext(timeoutCtx)
	requestLogger := log.ContextLogger(timeoutCtx, "REDFISH_HANDLER")
	requestLogger.Debug("HANDLE", "Method", r.Method, "URI", r.URL.Path)
	return timeoutCtx, requestLogger, cancel
}

func (rf *RFServer) makeCommand(eventType eh.EventType) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		cmd, evt, err := metric.CommandFactory(eventType)()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// get request based context, attach request id to it, then set up timeout and logger
		tmCtx, rqLog, cancel := rf.makeContextAndLogger(r, cmd)
		defer cancel()

		forwardInputAndVars(tmCtx, rqLog, w, r, cmd) // let command pull anything out of read env that it needs

		// set up output paths
		cmd.SetResponseHandlers(w.Header().Set, w.WriteHeader, w)

		// set up listener before publishing to avoid races
		l := eventwaiter.NewListener(tmCtx, rqLog, rf.d.GetWaiter(), cmd.ResponseWaitFn())
		l.Name = "Redfish Response Listener"
		defer l.Close()

		// publish command, which kicks everything off. Note publish based on background context to avoid weirdness in publish
		err = rf.d.GetBus().PublishEvent(context.Background(), evt)
		if err != nil {
			rqLog.Crit("Error publishing event. This should never happen!", "err", err)
			http.Error(w, "internal error publishing", http.StatusInternalServerError)
			return
		}

		err = l.ProcessOneEvent(tmCtx, func(eh.Event) {}) // wait until we get response event or user cancels request
		if err != nil {
			rqLog.Info("Wait ERROR", "err", err)
			http.Error(w, "internal error waiting", http.StatusInternalServerError)
			return
		}
	}
}
