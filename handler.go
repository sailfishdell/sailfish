// Copyright (c) 2017 - Max Ekman <max@looplab.se>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/aggregatestore/model"
	"github.com/looplab/eventhorizon/commandhandler/aggregate"
	eventbus "github.com/looplab/eventhorizon/eventbus/local"
	"github.com/looplab/eventhorizon/httputils"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"
	repo "github.com/looplab/eventhorizon/repo/memory"

    domain "github.com/superchalupa/redfish/internal/redfishresource"
)

// Handler is a http.Handler for the TodoMVC app.
type Handler struct {
	http.Handler

	CommandHandler eh.CommandHandler
	Repo           eh.ReadWriteRepo
}

// Logger is a simple event handler for logging all events.
type Logger struct{}

// Notify implements the Notify method of the EventObserver interface.
func (l *Logger) Notify(ctx context.Context, event eh.Event) {
	log.Printf("EVENT %s", event)
}

// NewHandler sets up the full Event Horizon domain for the TodoMVC app and
// returns a handler exposing some of the components.
func NewHandler() (*Handler, error) {
	// Create the repository and wrap in a version repository.
	repo := repo.NewRepo()

	// Create the event bus that distributes events.
	eventBus := eventbus.NewEventBus()
	eventPublisher := eventpublisher.NewEventPublisher()
	eventPublisher.AddObserver(&Logger{})
	eventBus.SetPublisher(eventPublisher)

	// Create the aggregate repository.
	aggregateStore, err := model.NewAggregateStore(repo)
	if err != nil {
		return nil, fmt.Errorf("could not create aggregate store: %s", err)
	}

	// Create the aggregate command handler.
	commandHandler, err := aggregate.NewCommandHandler(domain.AggregateType, aggregateStore)
	if err != nil {
		return nil, fmt.Errorf("could not create command handler: %s", err)
	}

	// Create a tiny logging middleware for the command handler.
	loggingHandler := eh.CommandHandlerFunc(func(ctx context.Context, cmd eh.Command) error {
		log.Printf("CMD %#v", cmd)
		return commandHandler.HandleCommand(ctx, cmd)
	})

	// Handle the API.
	h := http.NewServeMux()
	h.Handle("/api/events/", httputils.EventBusHandler(eventPublisher))

	h.Handle("/api/test", httputils.CommandHandler(loggingHandler, domain.GETCommand))


	// Simple HTTP request logging.
	logger := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.URL)
		h.ServeHTTP(w, r)
	})

	return &Handler{
		Handler:        logger,
		CommandHandler: loggingHandler,
		Repo:           repo,
	}, nil
}
