// +build redfish

package redfish

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/mux"

	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/sailfish/cmd/metric-engine/telemetry"
	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
)

type busComponents interface {
	GetBus() eh.EventBus
	GetWaiter() *eventwaiter.EventWaiter
}

type RFServer struct {
	logger log.Logger
	d      busComponents
}

func NewRedfishServer(logger log.Logger, d busComponents) *RFServer {
	return &RFServer{logger: logger, d: d}
}

func (rf *RFServer) AddHandlersToRouter(m *mux.Router) {
	m.HandleFunc("/redfish/v1/TelemetryService/MetricReportDefinitions", rf.makeCommand(telemetry.AddMetricReportDefinition)).Methods("POST")
	m.HandleFunc("/redfish/v1/TelemetryService/MetricReportDefinitions/{ID}", rf.makeCommand(telemetry.UpdateMetricReportDefinition)).Methods("PATCH")
	m.HandleFunc("/redfish/v1/TelemetryService/MetricReportDefinitions/{ID}", rf.makeCommand(telemetry.UpdateMetricReportDefinition)).Methods("PUT")
	m.HandleFunc("/redfish/v1/TelemetryService/MetricReportDefinitions/{ID}", rf.makeCommand(telemetry.DeleteMetricReportDefinition)).Methods("DELETE")
	m.HandleFunc("/redfish/v1/TelemetryService/MetricReports/{ID}", rf.makeCommand(telemetry.DeleteMetricReport)).Methods("DELETE")
}

// output a placeholder message
func (rf *RFServer) placeholder(message string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("%s\n", message)
	}
}

type Commander interface {
	GetRequestID() eh.UUID
	ResponseWaitFn() func(eh.Event) bool
}

type FromReader interface {
	DecodeFromReader(context.Context, log.Logger, io.Reader, map[string]string) error
}

func requestContextFromCommand(r *http.Request, cmd interface{}) (context.Context, Commander) {
	intCmd, ok := cmd.(Commander)
	if ok {
		return log.WithRequestID(r.Context(), intCmd.GetRequestID()), intCmd
	}
	return r.Context(), nil
}

type Response interface {
	GetError() error
}

func (rf *RFServer) makeCommand(eventType eh.EventType) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("IN %s HANDLER for %s\n", r.Method, r.URL.Path)
		fn := telemetry.Factory(eventType)
		evt, err := fn()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		cmd := evt.Data()
		reqCtx, intCmd := requestContextFromCommand(r, cmd)
		requestLogger := log.ContextLogger(reqCtx, "sse_handler")

		if d, ok := cmd.(FromReader); ok {
			err := d.DecodeFromReader(reqCtx, requestLogger, r.Body, mux.Vars(r))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}

		if intCmd == nil {
			http.Error(w, "internal error: not a command", http.StatusInternalServerError)
			return
		}

		l := eventwaiter.NewListener(reqCtx, requestLogger, rf.d.GetWaiter(), intCmd.ResponseWaitFn())
		l.Name = "Redfish Response Listener"
		defer l.Close()

		requestLogger.Crit("HANDLE", "Method", r.Method, "Event", fmt.Sprintf("%v", evt), "Command", fmt.Sprintf("%+v", cmd))

		err = rf.d.GetBus().PublishEvent(context.Background(), evt)
		if err != nil {
			requestLogger.Crit("Error publishing event. This should never happen!", "err", err)
			http.Error(w, "internal error publishing", http.StatusInternalServerError)
			return
		}

		ret, err := l.Wait(reqCtx)
		if err != nil {
			// most likely user disconnected before we sent response
			requestLogger.Info("Wait ERROR", "err", err)
			http.Error(w, "internal error waiting", http.StatusInternalServerError)
			return
		}
		requestLogger.Crit("RESPONSE", "Event", fmt.Sprintf("%v", ret), "RESPONSE", fmt.Sprintf("%+v", ret.Data()))
		// TODO: need to get this up to redfish standards for return
		// Need:
		// - HTTP headers. Location header for collection POST
		// - Return the created object. Is this optional?
		d := ret.Data()
		resp, ok := d.(Response)
		if !ok {
			requestLogger.Info("Got a non-response", "err", err)
			http.Error(w, "internal error with response", http.StatusInternalServerError)
			return
		}

		if resp.GetError() != nil {
			w.WriteHeader(http.StatusBadRequest)
		}

		fmt.Fprintf(w, "RET: %+v\n", ret.Data())

	}
}
