// +build redfish

package redfish

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"

	eh "github.com/looplab/eventhorizon"

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
	m.HandleFunc("/redfish/v1/TelemetryService/MetricReportDefinitions", rf.makeMRDPostHandleFunc("UNKNOWN", []string{"Unauthenticated"})).Methods("POST")
	m.HandleFunc("/redfish/v1/TelemetryService/MetricReportDefinitions/{ID}", rf.makeMRDPatchHandleFunc("UNKNOWN", []string{"Unauthenticated"})).Methods("PATCH")
	m.HandleFunc("/redfish/v1/TelemetryService/MetricReportDefinitions/{ID}", rf.makeMRDPutHandleFunc("UNKNOWN", []string{"Unauthenticated"})).Methods("PUT")
	m.HandleFunc("/redfish/v1/TelemetryService/MetricReportDefinitions/{ID}", rf.makeMRDDeleteHandleFunc("UNKNOWN", []string{"Unauthenticated"})).Methods("DELETE")
	m.HandleFunc("/redfish/v1/TelemetryService/MetricReports/{ID}", rf.makeMRDeleteHandleFunc("UNKNOWN", []string{"Unauthenticated"})).Methods("DELETE")
}

func (rf *RFServer) makeMRDPostHandleFunc(user string, privs []string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("HANDLE MRD POST\n")
	}
}

func (rf *RFServer) makeMRDPatchHandleFunc(user string, privs []string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("HANDLE MRD PATCH\n")
	}
}

func (rf *RFServer) makeMRDPutHandleFunc(user string, privs []string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("HANDLE MRD PUT\n")
	}
}

func (rf *RFServer) makeMRDDeleteHandleFunc(user string, privs []string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("HANDLE MRD DELETE\n")
	}
}

func (rf *RFServer) makeMRDeleteHandleFunc(user string, privs []string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("HANDLE MR DELETE\n")
	}
}
