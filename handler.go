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

// Notify implements the Notify method of the EventObserver interface.
func (d *DomainObjects) Notify(ctx context.Context, event eh.Event) {
	if event.EventType() == domain.RedfishResourceCreated {
		if data, ok := event.Data().(*domain.RedfishResourceCreatedData); ok {
			// TODO: handle conflicts
			d.Tree[data.ResourceURI] = data.ID

			fmt.Printf("New resource: %s\n", data.ResourceURI)

			if data.Collection {
				fmt.Printf("A new collection: %s\n", data.ResourceURI)
				d.collections = append(d.collections, data.ResourceURI)
			}

			for _, v := range d.collections {
				collectionToTest := path.Dir(data.ResourceURI)
				fmt.Printf("check existing collections for %s = %s\n", collectionToTest, v)
				if v == path.Dir(data.ResourceURI) {
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
		if data, ok := event.Data().(*domain.RedfishResourceRemovedData); ok {
			// TODO: remove from aggregatestore?
			delete(d.Tree, data.ResourceURI)
		}
		return
	}
}

// CommandHandler is a HTTP handler for eventhorizon.Commands. Commands must be
// registered with eventhorizon.RegisterCommand(). It expects a POST with a JSON
// body that will be unmarshalled into the command.
func (d *DomainObjects) MakeHandler(command eh.CommandType) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "unsuported method: "+r.Method, http.StatusMethodNotAllowed)
			return
		}

		cmd, err := eh.CreateCommand(command)
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

type CmdIDSetter interface {
	SetCmdID(eh.UUID)
}
type AggIDSetter interface {
	SetAggID(eh.UUID)
}
type UserDetailsSetter interface {
	SetUserDetails(string, []string) string
	// return codes:
	//      "checkMaster" - command check passed, but also check master
	//      "authorized"  - command check passed, go right ahead, no master check
	//      "unauthorized" - failed auth check in command, no need to check further

}
type HTTPParser interface {
	ParseHTTPRequest(*http.Request) error
}

type RedfishHandler struct {
	UserName   string
	Privileges []string
	d          *DomainObjects
}

func (rh *RedfishHandler) IsAuthorized(requiredPrivs []string) (authorized string) {
	authorized = "unauthorized"
	if requiredPrivs == nil {
		requiredPrivs = []string{}
	}
    fmt.Printf("\tloop start\n")
outer:
	for _, p := range rh.Privileges {
        fmt.Printf("\t  --> %s\n", p)
		for _, q := range requiredPrivs {
            fmt.Printf("\t\tCheck %s == %s\n", p, q)
			if p == q {
				authorized = "authorized"
				break outer
			}
		}
	}
	return
}

// TODO: need to write middleware to check x-auth-token header
// TODO: need to write middleware that would allow different types of encoding on output
func (rh *RedfishHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // All operations have to be on URLs that exist, so look it up in the tree
	aggID, ok := rh.d.Tree[r.URL.Path]
	if !ok {
		http.Error(w, "Could not find URL: "+r.URL.Path, http.StatusNotFound)
		return
	}

	search := []eh.CommandType{
	}

    // load the aggregate for the URL we are operating on
	agg, err := rh.d.AggregateStore.Load(r.Context(), domain.AggregateType, aggID)
	redfishResource, ok := agg.(*domain.RedfishResourceAggregate)
	if ok {
		// prepend the plugins to the search path
		search = append(search, eh.CommandType(redfishResource.ResourceURI + ":" + r.Method))
		search = append(search, eh.CommandType(redfishResource.Properties["@odata.type"].(string) + ":" + r.Method))
		search = append(search, eh.CommandType(redfishResource.Properties["@odata.context"].(string) + ":" + r.Method))
	}
    search = append(search, eh.CommandType("RedfishResource:" + r.Method))

    // search through the commands until we find one that exists
	var cmd eh.Command
	for _, cmdType := range search {
		cmd, err = eh.CreateCommand(cmdType)
		if err == nil {
			break
		}
	}

    // with a proper error if we couldnt create a command of any kind
	if cmd == nil {
		http.Error(w, "could not create command", http.StatusBadRequest)
		return
	}

    // Each command needs a unique UUID. We'll use that to listen for the HTTPProcessed Event, which should have a matching UUID.
	cmdId := eh.NewUUID()

    // some optional interfaces that the commands might implement
	if t, ok := cmd.(CmdIDSetter); ok {
		t.SetCmdID(cmdId)
	}
	if t, ok := cmd.(AggIDSetter); ok {
		t.SetAggID(aggID)
	}

	if t, ok := cmd.(HTTPParser); ok {
		err := t.ParseHTTPRequest(r)
		if err != nil {
			http.Error(w, "Problems parsing http request: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	// Choices: command can process Authorization, or we can process authorization, or both
	// If command implements UserDetailsSetter interface, we'll go ahead and call that.
	// Return code from command determines if we also check privs here
	authAction := "checkMaster"
	var implementsAuthorization bool
	if t, implementsAuthorization := cmd.(UserDetailsSetter); implementsAuthorization {
        fmt.Printf("UserDetailsSetter\n")
		authAction = t.SetUserDetails(rh.UserName, rh.Privileges)
        fmt.Printf("\tauthAction: %s\n", authAction)
	}
	// if command does not implement userdetails setter, we always check privs here
	if !implementsAuthorization || authAction == "checkMaster" {
        fmt.Printf("checkMaster\n")
		privsToCheck := redfishResource.PrivilegeMap[r.Method]
        fmt.Printf("\tprivsToCheck: %s\n", privsToCheck)

        // convert Privileges from []interface{} to []string (way more code than there should be for something this simple)
        var s []interface{}
        var t []string
		s, ok := privsToCheck.([]interface{})
		if !ok {
			s = []interface{}{}
		}
        for _, v := range s {
            if a, ok := v.(string); ok {
                t = append(t, a)
            }
        }
        fmt.Printf("\tPrivs (s): %s\n", s)

		authAction = rh.IsAuthorized(t)
        fmt.Printf("\tauthAction: %s\n", authAction)
	}

	if authAction != "authorized" {
		http.Error(w, "Not authorized to access this resource: ", http.StatusUnauthorized)
		return
	}

    // to avoid races, set up our listener first
	l, err := rh.d.EventWaiter.Listen(r.Context(), func(event eh.Event) bool {
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
	if err := rh.d.CommandHandler.HandleCommand(ctx, cmd); err != nil {
		http.Error(w, "could not handle command: "+err.Error(), http.StatusBadRequest)
		return
	}

	event, err := l.Wait(r.Context())
	if err != nil {
		fmt.Printf("Error waiting for event: %s\n", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data, ok := event.Data().(*domain.HTTPCmdProcessedData)
	if !ok {
		fmt.Printf("Error waiting for event: %s\n", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// set headers first
	for k, v := range data.Headers {
		w.Header().Add(k, v)
	}

	// and then encode response
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(data.Results)
	return
}
