package http_redfish_sse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	eh "github.com/looplab/eventhorizon"
	log "github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/eventservice"
	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

// NewRedfishSSEHandler constructs a new RedfishSSEHandler with the given username and privileges.
func NewRedfishSSEHandler(dobjs *domain.DomainObjects, logger log.Logger, u string, p []string) *RedfishSSEHandler {
	return &RedfishSSEHandler{UserName: u, Privileges: p, d: dobjs, logger: logger}
}

// RedfishSSEHandler struct holds authentication/authorization data as well as the domain variables
type RedfishSSEHandler struct {
	UserName   string
	Privileges []string
	d          *domain.DomainObjects
	logger     log.Logger
}

func (rh *RedfishSSEHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID := eh.NewUUID()
	ctx := domain.WithRequestID(r.Context(), requestID)
	requestLogger := domain.ContextLogger(ctx, "redfish_sse_handler")

	flusher, ok := w.(http.Flusher)
	if !ok {
		requestLogger.Crit("Streaming is not supported by the underlying http handler.")
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	rfSubContext := r.URL.Query().Get("context")

	requestLogger.Info("Trying to start RedfishSSE Stream for request.", "context", rfSubContext)

	l, err := rh.d.EventWaiter.Listen(ctx, func(event eh.Event) bool {
		if event.EventType() != eventservice.ExternalRedfishEvent {
			return false
		}
		return true
	})
	if err != nil {
		requestLogger.Crit("Could not create an event waiter.", "err", err)
		http.Error(w, "could not create waiter"+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Add("Strict-Transport-Security", "max-age=63072000; includeSubDomains") // for A+ SSL Labs score
	//w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("OData-Version", "4.0")
	w.Header().Set("Server", "go-redfish")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	notify := w.(http.CloseNotifier).CloseNotify()
	go func() {
		<-notify
		requestLogger.Debug("http session closed, closing down context")
		l.Close()
	}()

	flusher.Flush()

	sub := eventservice.Subscription{
		Protocol:    "SSE",
		Destination: "",
		EventTypes:  []string{"ResourceUpdated", "ResourceCreated", "ResourceRemoved", "Alert", "StateChanged"},
		Context:     rfSubContext,
	}
	sseContext, cancel := context.WithCancel(ctx)
	view := eventservice.CreateSubscription(sseContext, requestLogger, sub, cancel)
	_ = view

	for {
		event, err := l.Wait(sseContext)
		if err != nil {
			requestLogger.Error("Wait exited", "err", err)
			break
		}

		if evt, ok := event.Data().(*eventservice.ExternalRedfishEventData); ok {
			// Handle redfish events
			d, err := json.MarshalIndent(
				&struct {
					*eventservice.ExternalRedfishEventData
					Context string `json:",omitempty"`
				}{
					ExternalRedfishEventData: evt,
					Context:                  rfSubContext,
				},
				"data: ", "    ",
			)

			if err != nil {
				requestLogger.Error("MARSHAL SSE (event) FAILED", "err", err, "data", event.Data(), "event", event)
				return
			}
			fmt.Fprintf(w, "id: %d\n", evt.Id)
			fmt.Fprintf(w, "data: %s\n\n", d)
		} else {
			// Handle metric reports
			// sucks that we have to handle these two separately, but for now have to do it this way
			// TODO: find a better way to unify these
			d, err := json.MarshalIndent(event.Data(), "data: ", "    ")
			if err != nil {
				requestLogger.Error("MARSHAL SSE (metric report) FAILED", "err", err, "data", event.Data(), "event", event)
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", d)
		}

		flusher.Flush()
	}

	requestLogger.Debug("Closed session")
}
