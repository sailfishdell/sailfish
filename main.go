package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"time"
    "context"

	"github.com/looplab/eventhorizon/httputils"
	eh "github.com/looplab/eventhorizon"

	domain "github.com/superchalupa/redfish/internal/redfishresource"
)

func main() {
	log.Println("starting backend")

	domainObjs, _ := SetupDomainObjects()

	// Create a tiny logging middleware for the command handler.
	loggingHandler := eh.CommandHandlerFunc(func(ctx context.Context, cmd eh.Command) error {
		log.Printf("CMD %#v", cmd)
		return domainObjs.CommandHandler.HandleCommand(ctx, cmd)
	})

	// Handle the API.
	m := mux.NewRouter()

	m.PathPrefix("/redfish/").Handler(httputils.CommandHandler(loggingHandler, domain.GETCommand))
	//m.Handle("/api/events/", httputils.EventBusHandler(eventPublisher))


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
