package http_sse

import (
	"encoding/json"
	"fmt"
	"net/http"

	eh "github.com/looplab/eventhorizon"
	log "github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
)

type busObjs interface {
	GetWaiter() *eventwaiter.EventWaiter
}

// NewSSEHandler constructs a new SSEHandler with the given username and privileges.
func NewSSEHandler(dobjs busObjs, logger log.Logger, u string, p []string) *SSEHandler {
	return &SSEHandler{UserName: u, Privileges: p, d: dobjs, logger: logger}
}

// SSEHandler struct holds authentication/authorization data as well as the domain variables
type SSEHandler struct {
	UserName   string
	Privileges []string
	d          busObjs
	logger     log.Logger
}

func (rh *SSEHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID := eh.NewUUID()
	ctx := WithRequestID(r.Context(), requestID)
	requestLogger := ContextLogger(ctx, "sse_handler")
	requestLogger.Info("Trying to start SSE Stream for request.")

	r.Body.Close()

	flusher, ok := w.(http.Flusher)
	if !ok {
		requestLogger.Crit("Streaming is not supported by the underlying http handler.")
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	l := eventwaiter.NewListener(ctx, requestLogger, rh.d.GetWaiter(), func(event eh.Event) bool {
		return true
	})
	l.Name = "SSE Listener"

	// set headers first
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

	notify := w.(http.CloseNotifier).CloseNotify()
	go func() {
		<-notify
		requestLogger.Debug("http session closed, closing down context")
		l.Close()
	}()

	flusher.Flush()

	l.ProcessEvents(ctx, func(event eh.Event) {
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
			return
		}
		fmt.Fprintf(w, "data: %s\n\n", d)

		flusher.Flush()
	})

	requestLogger.Debug("Closed session")
}
