// +build redfish

package redfish

import (
	"context"
	"encoding/json"
	"fmt"
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
		requestLogger.Crit("RESPONSE", "Event", fmt.Sprintf("%v", ret), "Command", fmt.Sprintf("%+v", ret.Data()))
		fmt.Fprintf(w, "RET: %+v\n", ret.Data())
	}
}

func (rf *RFServer) makeMRDPostHandleFunc() func(w http.ResponseWriter, r *http.Request) {
	return rf.makeCommand(telemetry.NewAddMRDCommand)
}

func (rf *RFServer) makeMRDPatchHandleFunc() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("HANDLE MRD PATCH\n")
	}
}

func (rf *RFServer) makeMRDPutHandleFunc() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("HANDLE MRD PUT\n")
	}
}

func (rf *RFServer) makeMRDDeleteHandleFunc() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("HANDLE MRD DELETE\n")
	}
}

func (rf *RFServer) makeMRDeleteHandleFunc() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("HANDLE MR DELETE\n")
	}
}
