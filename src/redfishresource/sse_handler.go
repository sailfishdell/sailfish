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
	fmt.Printf("SSE SERVICE\n")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	// TODO: need to worry about privileges: probably should do the privilege checks in the Listener
	// to avoid races, set up our listener first
	l, err := rh.d.EventWaiter.Listen(r.Context(), func(event eh.Event) bool {
		return true
	})
	if err != nil {
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
		fmt.Printf("CLOSED SSE SESSION\n")
		l.Close()
	}()

	fmt.Fprintf(w, "TEST: 123\n\n")
	flusher.Flush()

	for {
		event, err := l.Wait(r.Context())
		fmt.Printf("GOT EVENT: %s\n", event)
		if err != nil {
			fmt.Printf("ERROR: %s\n", err.Error())
			break
		}

		data := event.Data()
		d, err := json.Marshal(data)
		fmt.Fprintf(w, "data: %s\n\n", d)

		flusher.Flush()
	}

	fmt.Printf("CLOSED\n")
}
