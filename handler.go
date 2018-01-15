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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/aggregatestore/model"
	"github.com/looplab/eventhorizon/commandhandler/aggregate"
	eventbus "github.com/looplab/eventhorizon/eventbus/local"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"
	repo "github.com/looplab/eventhorizon/repo/memory"
	"github.com/looplab/eventhorizon/utils"

	domain "github.com/superchalupa/go-redfish/internal/redfishresource"
)

type DomainObjects struct {
	CommandHandler eh.CommandHandler
	Repo           eh.ReadWriteRepo
	EventBus       eh.EventBus
	EventWaiter    *utils.EventWaiter
	AggregateStore eh.AggregateStore
	EventPublisher eh.EventPublisher
	Tree           map[string]eh.UUID
}

// SetupDDDFunctions sets up the full Event Horizon domain
// returns a handler exposing some of the components.
func NewDomainObjects() (*DomainObjects, error) {
	d := DomainObjects{}

	d.Tree = make(map[string]eh.UUID)

	// Create the repository and wrap in a version repository.
	d.Repo = repo.NewRepo()

	// Create the event bus that distributes events.
	d.EventBus = eventbus.NewEventBus()
	d.EventPublisher = eventpublisher.NewEventPublisher()
	d.EventBus.SetPublisher(d.EventPublisher)

	d.EventWaiter = utils.NewEventWaiter()
	d.EventPublisher.AddObserver(d.EventWaiter)

	// Create the aggregate repository.
	var err error
	d.AggregateStore, err = model.NewAggregateStore(d.Repo)
	if err != nil {
		return nil, fmt.Errorf("could not create aggregate store: %s", err)
	}

	// Create the aggregate command handler.
	d.CommandHandler, err = aggregate.NewCommandHandler(domain.AggregateType, d.AggregateStore)
	if err != nil {
		return nil, fmt.Errorf("could not create command handler: %s", err)
	}

	return &d, nil
}

func makeCommand(w http.ResponseWriter, r *http.Request, commandType eh.CommandType) (eh.Command, error) {
	if r.Method != "POST" {
		http.Error(w, "unsuported method: "+r.Method, http.StatusMethodNotAllowed)
		return nil, errors.New("unsupported method: " + r.Method)
	}

	cmd, err := eh.CreateCommand(commandType)
	if err != nil {
		http.Error(w, "could not create command: "+err.Error(), http.StatusBadRequest)
		return nil, errors.New("could not create command: " + err.Error())
	}

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "could not read command: "+err.Error(), http.StatusBadRequest)
		return nil, errors.New("could not read command: " + err.Error())
	}
	if err := json.Unmarshal(b, &cmd); err != nil {
		http.Error(w, "could not decode command: "+err.Error(), http.StatusBadRequest)
		return nil, errors.New("could not decode command: " + err.Error())
	}

	return cmd, nil
}

// CommandHandler is a HTTP handler for eventhorizon.Commands. Commands must be
// registered with eventhorizon.RegisterCommand(). It expects a POST with a JSON
// body that will be unmarshalled into the command.
func (d *DomainObjects) RemoveHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		fmt.Printf("HI %s: %s\n", r.URL, domain.RemoveRedfishResourceCommand)
		cmd, err := makeCommand(w, r, domain.RemoveRedfishResourceCommand)
		if err != nil {
			return
		}

		if c, ok := cmd.(*domain.CreateRedfishResource); ok {
			// TODO: remove from aggregatestore?
			delete(d.Tree, c.ResourceURI)

			// NOTE: Use a new context when handling, else it will be cancelled with
			// the HTTP request which will cause projectors etc to fail if they run
			// async in goroutines past the request.
			ctx := context.Background()
			if err := d.CommandHandler.HandleCommand(ctx, cmd); err != nil {
				http.Error(w, "could not handle command: "+err.Error(), http.StatusBadRequest)
				return
			}

		}

		fmt.Printf("OK!\n")

		w.WriteHeader(http.StatusOK)
	})
}

// CommandHandler is a HTTP handler for eventhorizon.Commands. Commands must be
// registered with eventhorizon.RegisterCommand(). It expects a POST with a JSON
// body that will be unmarshalled into the command.
func (d *DomainObjects) CreateHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("HI %s: %s\n", r.URL, domain.CreateRedfishResourceCommand)

		cmd, err := makeCommand(w, r, domain.CreateRedfishResourceCommand)
		if err != nil {
			return
		}

		if c, ok := cmd.(*domain.CreateRedfishResource); ok {
			// TODO: handle conflicts
			d.Tree[c.ResourceURI] = c.ID

			// NOTE: Use a new context when handling, else it will be cancelled with
			// the HTTP request which will cause projectors etc to fail if they run
			// async in goroutines past the request.
			ctx := context.Background()
			if err := d.CommandHandler.HandleCommand(ctx, cmd); err != nil {
				http.Error(w, "could not handle command: "+err.Error(), http.StatusBadRequest)
				return
			}

		}

		fmt.Printf("OK, tree is now: %s\n", d.Tree)

		w.WriteHeader(http.StatusOK)
	})
}

type CmdIDSetter interface {
	SetCmdID(eh.UUID)
}
type AggIDSetter interface {
	SetAggID(eh.UUID)
}
type HTTPParser interface {
	ParseHTTPRequest(*http.Request)
}

// TODO: need to write middleware to check x-auth-token header
// TODO: need to write middleware that would allow different types of encoding on output
func (d *DomainObjects) RedfishHandlerFunc() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("HAPPY\n")
		aggID, ok := d.Tree[r.URL.Path]
		if !ok {
			http.Error(w, "Could not find URL: "+r.URL.Path, http.StatusNotFound)
			return
		}

		search := []eh.CommandType{
			eh.CommandType("RedfishResource:" + r.Method),
		}

		agg, err := d.AggregateStore.Load(r.Context(), domain.AggregateType, aggID)
		redfishResource, ok := agg.(*domain.RedfishResourceAggregate)
		if ok {
			search = append([]eh.CommandType{eh.CommandType(redfishResource.Plugin + ":" + r.Method)}, search...)
		}

		var cmd eh.Command
		for _, cmdType := range search {
			cmd, err = eh.CreateCommand(cmdType)
			if err == nil {
				break
			}
		}

		if cmd == nil {
			http.Error(w, "could not create command", http.StatusBadRequest)
			return
		}

		cmdId := eh.NewUUID()

		if t, ok := cmd.(CmdIDSetter); ok {
			t.SetCmdID(cmdId)
		}
		if t, ok := cmd.(AggIDSetter); ok {
			t.SetAggID(aggID)
		}
		if t, ok := cmd.(HTTPParser); ok {
			t.ParseHTTPRequest(r)
		}

		l, err := d.EventWaiter.Listen(r.Context(), func(event eh.Event) bool {
			if event.EventType() != domain.HTTPCmdProcessed {
				return false
			}
			if data, ok := event.Data().(*domain.HTTPCmdProcessedData); ok {
				if data.CommandID == cmdId {
					return true
				}
			}
			return false
		})
		if err != nil {
			http.Error(w, "could not create waiter"+err.Error(), http.StatusInternalServerError)
			return
		}
		defer l.Close()

		ctx := context.Background()
		if err := d.CommandHandler.HandleCommand(ctx, cmd); err != nil {
			http.Error(w, "could not handle command: "+err.Error(), http.StatusBadRequest)
			return
		}

		event, err := l.Wait(r.Context())
		if err != nil {
			fmt.Printf("Error waiting for event: %s\n", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		d, ok := event.Data().(*domain.HTTPCmdProcessedData)
		if !ok {
			fmt.Printf("Error waiting for event: %s\n", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(d.Results)
		return
	}
}
