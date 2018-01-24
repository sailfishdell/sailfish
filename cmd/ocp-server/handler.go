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
	"fmt"
	"io/ioutil"
	"net/http"
	"path"

	"github.com/gorilla/mux"
	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/aggregatestore/model"
	"github.com/looplab/eventhorizon/commandhandler/aggregate"
	eventbus "github.com/looplab/eventhorizon/eventbus/local"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"
	repo "github.com/looplab/eventhorizon/repo/memory"
	"github.com/looplab/eventhorizon/utils"

	domain "github.com/superchalupa/go-redfish/redfishresource"
)

type DomainObjects struct {
	CommandHandler eh.CommandHandler
	Repo           eh.ReadWriteRepo
	EventBus       eh.EventBus
	EventWaiter    *utils.EventWaiter
	AggregateStore eh.AggregateStore
	EventPublisher eh.EventPublisher
	Tree           map[string]eh.UUID
	collections    []string
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

	// set up our built-in observer
	d.EventPublisher.AddObserver(&d)

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

func (d *DomainObjects) GetAggregateID(uri string) (eh.UUID, bool) {
	// All operations have to be on URLs that exist, so look it up in the tree
	aggID, ok := d.Tree[uri]
	return aggID, ok
}

// Notify implements the Notify method of the EventObserver interface.
func (d *DomainObjects) Notify(ctx context.Context, event eh.Event) {
	fmt.Printf("Notify( event==%s )\n", event)
	if event.EventType() == domain.RedfishResourceCreated {
		if data, ok := event.Data().(domain.RedfishResourceCreatedData); ok {
			// TODO: handle conflicts
			d.Tree[data.ResourceURI] = data.ID

			fmt.Printf("New resource: %s\n", data.ResourceURI)

			if data.Collection {
				fmt.Printf("A new collection: %s\n", data.ResourceURI)
				d.collections = append(d.collections, data.ResourceURI)
			}

			collectionToTest := path.Dir(data.ResourceURI)
			fmt.Printf("Searching for a collection named %s in our list: %s\n", collectionToTest, d.collections)
			for _, v := range d.collections {
				if v == collectionToTest {
					fmt.Printf("\tWe got a match: add to collection command\n")
					d.CommandHandler.HandleCommand(
						context.Background(),
						&domain.AddResourceToRedfishResourceCollection{
							ID:          d.Tree[collectionToTest],
							ResourceURI: data.ResourceURI,
						},
					)
				}
			}
		}
		return
	}
	if event.EventType() == domain.RedfishResourceRemoved {
		if data, ok := event.Data().(domain.RedfishResourceRemovedData); ok {
			// Look to see if it is a member of a collection
			collectionToTest := path.Dir(data.ResourceURI)
			fmt.Printf("Searching for a collection named %s in our list: %s\n", collectionToTest, d.collections)
			for _, v := range d.collections {
				if v == collectionToTest {
					fmt.Printf("\tWe got a match: remove from collection command\n")
					d.CommandHandler.HandleCommand(
						context.Background(),
						&domain.RemoveResourceFromRedfishResourceCollection{
							ID:          d.Tree[collectionToTest],
							ResourceURI: data.ResourceURI,
						},
					)
				}
			}

			// is this a collection? If so, remove it from our collections list
			for i, c := range d.collections {
				if c == data.ResourceURI {
					fmt.Printf("Removing collection: %s\n", data.ResourceURI)
					// swap the collection we found to the end
					d.collections[len(d.collections)-1], d.collections[i] = d.collections[i], d.collections[len(d.collections)-1]
					// then slice it off
					d.collections = d.collections[:len(d.collections)-1]
				}
			}

			// TODO: remove from aggregatestore?
			delete(d.Tree, data.ResourceURI)
		}
		return
	}
}

// CommandHandler is a HTTP handler for eventhorizon.Commands. Commands must be
// registered with eventhorizon.RegisterCommand(). It expects a POST with a JSON
// body that will be unmarshalled into the command.
func (d *DomainObjects) GetInternalCommandHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		if r.Method != "POST" {
			http.Error(w, "unsuported method: "+r.Method, http.StatusMethodNotAllowed)
			return
		}

		cmd, err := eh.CreateCommand(eh.CommandType("internal:" + vars["command"]))
		if err != nil {
			http.Error(w, "could not create command: "+err.Error(), http.StatusBadRequest)
			return
		}

		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "could not read command: "+err.Error(), http.StatusBadRequest)
			return
		}
		if err := json.Unmarshal(b, &cmd); err != nil {
			http.Error(w, "could not decode command: "+err.Error(), http.StatusBadRequest)
			return
		}

		// NOTE: Use a new context when handling, else it will be cancelled with
		// the HTTP request which will cause projectors etc to fail if they run
		// async in goroutines past the request.
		ctx := context.Background()
		if err := d.CommandHandler.HandleCommand(ctx, cmd); err != nil {
			http.Error(w, "could not handle command: "+err.Error(), http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}

