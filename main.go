package main

import (
	"context"
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"time"

	eh "github.com/looplab/eventhorizon"

	domain "github.com/superchalupa/go-redfish/internal/redfishresource"
)

func main() {
	log.Println("starting backend")

	domainObjs, _ := NewDomainObjects()
	domainObjs.EventPublisher.AddObserver(&Logger{})
	domain.RegisterRRA(domainObjs.EventBus)

	orighandler := domainObjs.CommandHandler

	// Create a tiny logging middleware for the command handler.
	loggingHandler := eh.CommandHandlerFunc(func(ctx context.Context, cmd eh.Command) error {
		log.Printf("CMD %#v", cmd)
		return orighandler.HandleCommand(ctx, cmd)
	})

	domainObjs.CommandHandler = loggingHandler

	// set up some basic stuff
    rootID := eh.NewUUID()
    domainObjs.Tree["/redfish/v1/"] = rootID
	loggingHandler.HandleCommand(context.Background(), &domain.CreateRedfishResource{ID: rootID, ResourceURI: "/redfish/v1/"})

	// Handle the API.
	m := mux.NewRouter()

	m.Path("/redfish").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {http.Redirect(w, r, "/redfish/", 301)})
	m.Path("/redfish/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("{\n\t\"v1\": \"/redfish/v1/\"\n}\n")) })
	m.Path("/redfish/v1").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {http.Redirect(w, r, "/redfish/v1/", 301)})

	m.PathPrefix("/redfish/v1/").Methods("GET").Handler(domainObjs.GetRedfishHandlerFunc())
	m.PathPrefix("/api/createresource").Handler(domainObjs.CreateHandler())
	m.PathPrefix("/api/removeresource").Handler(domainObjs.RemoveHandler())

	// Simple HTTP request logging.
	logger := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func(begin time.Time) {
			log.Println(
				"source", r.RemoteAddr,
				"method", r.Method,
				"url", r.URL,
				"business_logic_time", time.Since(begin),
				"args", fmt.Sprintf("%#v", mux.Vars(r)),
			)
		}(time.Now())
		m.ServeHTTP(w, r)
	})

	log.Println(http.ListenAndServe(":8080", logger))
}

// Logger is a simple event handler for logging all events.
type Logger struct{}

// Notify implements the Notify method of the EventObserver interface.
func (l *Logger) Notify(ctx context.Context, event eh.Event) {
	log.Printf("EVENT %s", event)
}
