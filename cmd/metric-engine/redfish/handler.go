// +build redfish

package redfish

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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
	m.HandleFunc("/redfish/v1/TelemetryService/MetricReportDefinitions", rf.makeMRDPostHandleFunc()).Methods("POST")
	m.HandleFunc("/redfish/v1/TelemetryService/MetricReportDefinitions/{ID}", rf.makeMRDPatchHandleFunc()).Methods("PATCH")
	m.HandleFunc("/redfish/v1/TelemetryService/MetricReportDefinitions/{ID}", rf.makeMRDPutHandleFunc()).Methods("PUT")
	m.HandleFunc("/redfish/v1/TelemetryService/MetricReportDefinitions/{ID}", rf.makeMRDDeleteHandleFunc()).Methods("DELETE")
	m.HandleFunc("/redfish/v1/TelemetryService/MetricReports/{ID}", rf.makeMRDeleteHandleFunc()).Methods("DELETE")
}

type Commander interface {
	GetRequestID() eh.UUID
	ResponseWaitFn() func(eh.Event) bool
}

func (rf *RFServer) makeCommand(fn func() (eh.Event, error)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		evt, err := fn()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		cmd := evt.Data()
		intCmd, ok := cmd.(Commander)
		if !ok {
			rf.logger.Crit("Internal error")
		}

		reqCtx := log.WithRequestID(r.Context(), intCmd.GetRequestID())
		requestLogger := log.ContextLogger(reqCtx, "sse_handler")

		decoder := json.NewDecoder(r.Body)
		err = decoder.Decode(cmd)
		if err != nil {
			requestLogger.Crit("Error decoding", "err", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		l := eventwaiter.NewListener(reqCtx, requestLogger, rf.d.GetWaiter(), intCmd.ResponseWaitFn())
		l.Name = "Redfish Response Listener"
		defer l.Close()

		requestLogger.Crit("HANDLE", "Method", r.Method, "Event", fmt.Sprintf("%v", evt), "Command", fmt.Sprintf("%+v", cmd))

		err = rf.d.GetBus().PublishEvent(context.Background(), evt)
		if err != nil {
			requestLogger.Crit("Error publishing event. This should never happen!", "err", err)
		}

		ret, err := l.Wait(reqCtx)
		if err != nil {
			// most likely user disconnected before we sent response
			requestLogger.Info("Wait ERROR", "err", err)
			return
		}
		requestLogger.Crit("RESPONSE", "Event", fmt.Sprintf("%v", ret), "RESPONSE", fmt.Sprintf("%+v", ret.Data()))
		// TODO: need to get this up to redfish standards for return
		// Need:
		// - HTTP headers. Location header for collection POST
		// - Return the created object. Is this optional?
		fmt.Fprintf(w, "RET: %+v\n", ret.Data())
	}
}

func (rf *RFServer) makeMRDPostHandleFunc() func(w http.ResponseWriter, r *http.Request) {
	return rf.makeCommand(telemetry.Factory(telemetry.AddMetricReportDefinition))
}

// TODO
func (rf *RFServer) makeMRDPatchHandleFunc() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("HANDLE MRD PATCH\n")
	}
}

// TODO
func (rf *RFServer) makeMRDPutHandleFunc() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("HANDLE MRD PUT\n")
	}
}

func (rf *RFServer) makeMRDDeleteHandleFunc() func(w http.ResponseWriter, r *http.Request) {
	fn := telemetry.Factory(telemetry.DeleteMetricReportDefinition)
	return func(w http.ResponseWriter, r *http.Request) {
		evt, err := fn()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		cmd := evt.Data()
		intCmd, ok := cmd.(Commander)
		if !ok {
			rf.logger.Crit("Internal error")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		reqCtx := log.WithRequestID(r.Context(), intCmd.GetRequestID())
		requestLogger := log.ContextLogger(reqCtx, "sse_handler")

		// no body in delete
		// ==========================================
		// this could probably be abstracted?
		type deleter interface {
			SetPathToDelete(string)
		}
		d, ok := intCmd.(deleter)
		if !ok {
			// cant happen
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		pathStuff := strings.Split(r.URL.Path, "/")
		fmt.Printf("DEBUG: delete path(%s) components(%+v) with reportname(%s)\n", r.URL.Path, pathStuff, pathStuff[len(pathStuff)-1])

		d.SetPathToDelete(pathStuff[len(pathStuff)-1])
		// ==========================================

		l := eventwaiter.NewListener(reqCtx, requestLogger, rf.d.GetWaiter(), intCmd.ResponseWaitFn())
		l.Name = "Redfish Response Listener"
		defer l.Close()

		requestLogger.Crit("HANDLE", "Method", r.Method, "Event", fmt.Sprintf("%v", evt), "Command", fmt.Sprintf("%+v", cmd))

		err = rf.d.GetBus().PublishEvent(context.Background(), evt)
		if err != nil {
			requestLogger.Crit("Error publishing event. This should never happen!", "err", err)
		}

		ret, err := l.Wait(reqCtx)
		if err != nil {
			// most likely user disconnected before we sent response
			requestLogger.Info("Wait ERROR", "err", err)
			return
		}
		requestLogger.Crit("RESPONSE", "Event", fmt.Sprintf("%v", ret), "RESPONSE", fmt.Sprintf("%+v", ret.Data()))
		// TODO: need to get this up to redfish standards for return
		// Need:
		// - Return the deleted object. optional?
		fmt.Fprintf(w, "RET: %+v\n", ret.Data())
	}

}

func (rf *RFServer) makeMRDeleteHandleFunc() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("HANDLE MR DELETE\n")
	}
}
