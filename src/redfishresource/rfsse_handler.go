package domain

import (
	"encoding/json"
	"fmt"
	"net/http"

	eh "github.com/looplab/eventhorizon"
	log "github.com/superchalupa/go-redfish/src/log"
)

// NewRedfishSSEHandler constructs a new RedfishSSEHandler with the given username and privileges.
func NewRedfishSSEHandler(dobjs *DomainObjects, logger log.Logger, u string, p []string) *RedfishSSEHandler {
	return &RedfishSSEHandler{UserName: u, Privileges: p, d: dobjs, logger: logger}
}

// RedfishSSEHandler struct holds authentication/authorization data as well as the domain variables
type RedfishSSEHandler struct {
	UserName   string
	Privileges []string
	d          *DomainObjects
	logger     log.Logger
}

func (rh *RedfishSSEHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID := eh.NewUUID()
	ctx := WithRequestID(r.Context(), requestID)
	requestLogger := ContextLogger(ctx, "redfish_sse_handler")
	requestLogger.Info("Trying to start RedfishSSE Stream for request.")

	flusher, ok := w.(http.Flusher)
	if !ok {
		requestLogger.Crit("Streaming is not supported by the underlying http handler.")
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	// TODO: need to worry about privileges: probably should do the privilege checks in the Listener
	// to avoid races, set up our listener first
	l, err := rh.d.EventWaiter.Listen(ctx, func(event eh.Event) bool {
//         if event.EventType() != eventservice.RedfishEvent {
            return false
 //        }
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

	for {
		event, err := l.Wait(ctx)
		if err != nil {
			requestLogger.Error("Wait exited", "err", err)
			break
		}

		d, err := json.Marshal(
			&struct {
				Name string      `json:"name"`
				Data interface{} `json:"data"`
			}{
				Name: string(event.EventType()),
				Data: event.Data(),
			},
		)
		if err != nil {
			requestLogger.Error("MARSHAL SSE FAILED", "err", err, "data", event.Data(), "event", event)
		}
		fmt.Fprintf(w, "data: %s\n\n", d)

		flusher.Flush()
	}

	requestLogger.Debug("Closed session")
}
