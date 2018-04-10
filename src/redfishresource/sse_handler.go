package domain

import (
	"encoding/json"
	"fmt"
	"net/http"

	eh "github.com/looplab/eventhorizon"
	log "github.com/superchalupa/go-redfish/src/log"
)

func NewSSEHandler(dobjs *DomainObjects, logger log.Logger, u string, p []string) *SSEHandler {
	return &SSEHandler{UserName: u, Privileges: p, d: dobjs, logger: logger}
}

type SSEHandler struct {
	UserName   string
	Privileges []string
	d          *DomainObjects
	logger     log.Logger
}

func (rh *SSEHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestId := eh.NewUUID()
	ctx := WithRequestID(r.Context(), requestId)
	requestLogger := ContextLogger(ctx, "sse_handler")

	flusher, ok := w.(http.Flusher)
	if !ok {
		requestLogger.Crit("Streaming is not supported by the underlying http handler.")
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	// TODO: need to worry about privileges: probably should do the privilege checks in the Listener
	// to avoid races, set up our listener first
	l, err := rh.d.EventWaiter.Listen(ctx, func(event eh.Event) bool {
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

		data := event.Data()
		d, err := json.Marshal(data)
		fmt.Fprintf(w, "data: %s\n\n", d)

		flusher.Flush()
	}

	requestLogger.Debug("Closed session")
}
