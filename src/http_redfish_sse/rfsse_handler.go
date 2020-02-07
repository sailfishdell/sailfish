package http_redfish_sse

import (
	"encoding/json"
	"fmt"
	"net/http"
	"context"

	eh "github.com/looplab/eventhorizon"
	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
	"github.com/superchalupa/sailfish/src/ocp/eventservice"
)

type busObjs interface {
	GetWaiter() *eventwaiter.EventWaiter
}

// NewRedfishSSEHandler constructs a new RedfishSSEHandler with the given username and privileges.
func NewRedfishSSEHandler(dobjs busObjs, logger log.Logger, u string, p []string) *RedfishSSEHandler {
	return &RedfishSSEHandler{UserName: u, Privileges: p, d: dobjs, logger: logger}
}

// RedfishSSEHandler struct holds authentication/authorization data as well as the domain variables
type RedfishSSEHandler struct {
	UserName   string
	Privileges []string
	d          busObjs
	logger     log.Logger
}

func (rh *RedfishSSEHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID := eh.NewUUID()
	ctx := log.WithRequestID(r.Context(), requestID)
	requestLogger := log.ContextLogger(ctx, "redfish_sse_handler")

	flusher, ok := w.(http.Flusher)
	if !ok {
		requestLogger.Crit("Streaming is not supported by the underlying http handler.")
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	rfSubContext := r.URL.Query().Get("context")

	requestLogger.Info("Trying to start RedfishSSE Stream for request.", "context", rfSubContext)

	l := eventwaiter.NewListener(ctx, requestLogger, rh.d.GetWaiter(), func(event eh.Event) bool {
		return event.EventType() == eventservice.ExternalRedfishEvent || event.EventType() == eventservice.ExternalMetricEvent
	})

	l.Name = "RF SSE Listener"
	defer l.Close()

	//w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("OData-Version", "4.0")
	w.Header().Set("Server", "sailfish")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Cache-Control", "no-Store,no-Cache")
	w.Header().Set("Pragma", "no-cache")

	// security headers
	w.Header().Add("Strict-Transport-Security", "max-age=63072000; includeSubDomains") // for A+ SSL Labs score
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	w.Header().Set("X-Content-Security-Policy", "default-src 'self'")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// compatibility headers
	w.Header().Set("X-UA-Compatible", "IE=11")

	r.Body.Close()
	flusher.Flush()

	sub := eventservice.Subscription{
		Protocol:    "SSE",
		Destination: "",
		EventTypes:  []string{"ResourceUpdated", "ResourceAdded", "ResourceRemoved", "Alert", "StateChanged"},
		Context:     rfSubContext,
	}
	sseContext, cancel := context.WithCancel(ctx)
	eventservice.GlobalEventService.CreateSubscription(sseContext, requestLogger, sub, cancel)

	// stream the output using an encoder
	outputEncoder := json.NewEncoder(w)
	outputEncoder.SetIndent("data: ", "    ")

	l.ProcessEvents(sseContext, func(event eh.Event) {
		var err error
		switch evt := event.Data().(type) {
		// TODO: find a better way to unify these
		// sucks that we have to handle these two separately, but for now have to do it this way
		case *eventservice.ExternalRedfishEventData: // regular redfish events
			// initial header
			fmt.Fprintf(w, "id: %d\ndata: ", evt.Id)
			err = outputEncoder.Encode(&struct {
				*eventservice.ExternalRedfishEventData
				Context string `json:",omitempty"`
			}{
				ExternalRedfishEventData: evt,
				Context:                  rfSubContext,
			})

		case eventservice.MetricReportData: // metric reports
			fmt.Fprintf(w, "data: ")
			err = outputEncoder.Encode(evt.Data)
		}

		// extra trailing newline for SSE protocol compliance
		fmt.Fprintf(w, "\n")
		if err != nil {
			requestLogger.Error("MARSHAL SSE (event) FAILED", "err", err, "data", event.Data(), "event", event)
			return
		}

		flusher.Flush()
	})

	requestLogger.Debug("Closed session")
}
