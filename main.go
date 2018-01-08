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

	// Create a tiny logging middleware for the command handler.
	loggingHandler := eh.CommandHandlerFunc(func(ctx context.Context, cmd eh.Command) error {
		log.Printf("CMD %#v", cmd)
		return domainObjs.CommandHandler.HandleCommand(ctx, cmd)
	})

	//domainObjs.CommandHandler = loggingHandler

	// set up some basic stuff
	loggingHandler.HandleCommand(
		context.Background(), &domain.CreateRedfishResource{ID: eh.NewUUID()},
	)

	// Handle the API.
	m := mux.NewRouter()

	m.PathPrefix("/redfish/").Handler(domainObjs.RedfishHandlerFunc())
	m.PathPrefix("/api/createresource").Handler(domain.CommandHandler(domainObjs.loggingHandler, domain.CreateRedfishResourceCommand))
	m.PathPrefix("/api/removeresource").Handler(domain.CommandHandler(domainObjs.loggingHandler, domain.RemoveRedfishResourceCommand))

	// Simple HTTP request logging.
	logger := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func(begin time.Time) {
			log.Println(
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
