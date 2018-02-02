package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"sync"

	"github.com/gorilla/mux"
	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/aggregatestore/model"
	"github.com/looplab/eventhorizon/commandhandler/aggregate"
	eventbus "github.com/looplab/eventhorizon/eventbus/local"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"
	repo "github.com/looplab/eventhorizon/repo/memory"
	"github.com/looplab/eventhorizon/utils"
)

type DomainObjects struct {
	CommandHandler eh.CommandHandler
	Repo           eh.ReadWriteRepo
	EventBus       eh.EventBus
	EventWaiter    *utils.EventWaiter
	AggregateStore eh.AggregateStore
	EventPublisher eh.EventPublisher

	treeMu sync.RWMutex
	Tree   map[string]eh.UUID

	collectionsMu sync.RWMutex
	collections   []string
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
	d.CommandHandler, err = aggregate.NewCommandHandler(AggregateType, d.AggregateStore)
	if err != nil {
		return nil, fmt.Errorf("could not create command handler: %s", err)
	}

	return &d, nil
}

func (d *DomainObjects) HasAggregateID(uri string) bool {
	d.treeMu.RLock()
	defer d.treeMu.RUnlock()
	_, ok := d.Tree[uri]
	return ok
}

func (d *DomainObjects) GetAggregateID(uri string) eh.UUID {
	d.treeMu.RLock()
	defer d.treeMu.RUnlock()
	return d.Tree[uri]
}

func (d *DomainObjects) GetAggregateIDOK(uri string) (id eh.UUID, ok bool) {
	d.treeMu.RLock()
	defer d.treeMu.RUnlock()
	id, ok = d.Tree[uri]
	return
}

func (d *DomainObjects) SetAggregateID(uri string, ID eh.UUID) {
	d.treeMu.Lock()
	defer d.treeMu.Unlock()
	d.Tree[uri] = ID
}

func (d *DomainObjects) DeleteResource(uri string) {
	d.treeMu.Lock()
	defer d.treeMu.Unlock()
	delete(d.Tree, uri)
}

// Notify implements the Notify method of the EventObserver interface.
func (d *DomainObjects) Notify(ctx context.Context, event eh.Event) {
	fmt.Printf("Notify( event==%s )\n", event)
	if event.EventType() == RedfishResourceCreated {
		if data, ok := event.Data().(RedfishResourceCreatedData); ok {
			// TODO: handle conflicts (how?)
			d.SetAggregateID(data.ResourceURI, data.ID)

			// TODO: need to split out auto collection management into a plugin
			if data.Collection {
				fmt.Printf("A new collection: %s\n", data.ResourceURI)
				d.collectionsMu.Lock()
				d.collections = append(d.collections, data.ResourceURI)
				d.collectionsMu.Unlock()
			}

			collectionToTest := path.Dir(data.ResourceURI)
			d.collectionsMu.RLock()
			for _, v := range d.collections {
				if v == collectionToTest {
					fmt.Printf("Collection: ADD (%s) to collection (%s)\n", data.ResourceURI, v)
					d.CommandHandler.HandleCommand(
						context.Background(),
						&AddResourceToRedfishResourceCollection{
							ID:          d.GetAggregateID(collectionToTest),
							ResourceURI: data.ResourceURI,
						},
					)
				}
			}
			d.collectionsMu.RUnlock()
		}
		return
	}
	if event.EventType() == RedfishResourceRemoved {
		if data, ok := event.Data().(RedfishResourceRemovedData); ok {
			// Look to see if it is a member of a collection
			collectionToTest := path.Dir(data.ResourceURI)
			d.collectionsMu.RLock()
			for _, v := range d.collections {
				if v == collectionToTest {
					fmt.Printf("Collection: REMOVE (%s) from collection (%s)\n", data.ResourceURI, v)
					d.CommandHandler.HandleCommand(
						context.Background(),
						&RemoveResourceFromRedfishResourceCollection{
							ID:          d.GetAggregateID(collectionToTest),
							ResourceURI: data.ResourceURI,
						},
					)
				}
			}
			d.collectionsMu.RUnlock()

			// is this a collection? If so, remove it from our collections list
			d.collectionsMu.Lock()
			for i, c := range d.collections {
				if c == data.ResourceURI {
					fmt.Printf("Removing collection: %s\n", data.ResourceURI)
					// swap the collection we found to the end
					d.collections[len(d.collections)-1], d.collections[i] = d.collections[i], d.collections[len(d.collections)-1]
					// then slice it off
					d.collections = d.collections[:len(d.collections)-1]
					break
				}
			}
			d.collectionsMu.Unlock()

			// TODO: remove from aggregatestore?
			d.DeleteResource(data.ResourceURI)
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
