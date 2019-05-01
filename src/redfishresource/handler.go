package domain

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/commandhandler/aggregate"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"
	repo "github.com/looplab/eventhorizon/repo/memory"
	"github.com/superchalupa/sailfish/src/looplab/aggregatestore"
	"github.com/superchalupa/sailfish/src/looplab/eventbus"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
)

type waiter interface {
	Listen(context.Context, func(eh.Event) bool) (*eventwaiter.EventListener, error)
	Notify(context.Context, eh.Event)
	Run()
}

type DomainObjects struct {
	CommandHandler eh.CommandHandler
	Repo           eh.ReadWriteRepo
	EventBus       eh.EventBus
	EventWaiter    waiter
	AggregateStore eh.AggregateStore
	EventPublisher eh.EventPublisher

	// for http returns
	HTTPResultsBus eh.EventBus
	HTTPPublisher  eh.EventPublisher
	HTTPWaiter     waiter

	treeMu sync.RWMutex
	Tree   map[string]eh.UUID

	licensesMu sync.RWMutex
	licenses   []string
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
	d.EventBus.AddHandler(eh.MatchAny(), d.EventPublisher)

	ew := eventwaiter.NewEventWaiter(eventwaiter.SetName("Main"), eventwaiter.NoAutoRun)
	d.EventWaiter = ew
	d.EventPublisher.AddObserver(d.EventWaiter)
	go ew.Run()

	// specific event bus to handle returns from http
	d.HTTPResultsBus = eventbus.NewEventBus()
	d.HTTPPublisher = eventpublisher.NewEventPublisher()
	d.HTTPResultsBus.AddHandler(eh.MatchEvent(HTTPCmdProcessed), d.HTTPPublisher)

	// hook up http waiter to the other bus for back compat
	d.HTTPWaiter = eventwaiter.NewEventWaiter(eventwaiter.SetName("HTTP"), eventwaiter.NoAutoRun)
	d.EventPublisher.AddObserver(d.HTTPWaiter)
	d.HTTPPublisher.AddObserver(d.HTTPWaiter)
	go d.HTTPWaiter.Run()

	// set up commands so that they can directly publish to http bus
	eh.RegisterCommand(func() eh.Command {
		return &GET{
			HTTPEventBus: d.HTTPResultsBus,
		}
	})

	eh.RegisterCommand(func() eh.Command {
		return &PATCH{
			HTTPEventBus: d.HTTPResultsBus,
		}
	})

	// set up our built-in observer
	d.EventPublisher.AddObserver(&d)

	// Create the aggregate repository.
	var err error
	d.AggregateStore, err = aggregatestore.NewAggregateStore(d.Repo, d.EventBus)
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

func (d *DomainObjects) GetLicenses() []string {
	d.licensesMu.RLock()
	defer d.licensesMu.RUnlock()
	ret := make([]string, len(d.licenses))
	copy(ret, d.licenses)
	return ret
}

func (d *DomainObjects) HasAggregateID(uri string) bool {
	d.treeMu.RLock()
	defer d.treeMu.RUnlock()
	_, ok := d.Tree[uri]
	return ok
}

func (d *DomainObjects) GetAggregateID(uri string) (id eh.UUID) {
	id, _ = d.GetAggregateIDOK(uri)
	return
}

// VALIDATE TREE - DEBUG ONLY
func (d *DomainObjects) CheckTree() (id eh.UUID, ok bool) {
	d.treeMu.RLock()
	defer d.treeMu.RUnlock()

	injectCmds := 0
	aggs, _ := d.Repo.FindAll(context.Background())
	for _, agg := range aggs {
		if rr, ok := agg.(*RedfishResourceAggregate); ok {
			checkuri := rr.ResourceURI
			if id, ok := d.Tree[checkuri]; ok {
				if id == agg.EntityID() {
					// found good agg in tree
				} else {
					fmt.Printf("Validate %s\n", agg.EntityID())
					fmt.Printf("\tURI: %s", checkuri)
					fmt.Printf("\n\tAggregate ID Mismatch! %s != %s\n", id, agg.EntityID())
					//panic("Aggregate ID Mismatch!")
				}
			} else {
				if string(agg.EntityID()) == string(injectUUID) {
					injectCmds++
					// it's an inject command. that's ok
				} else {
					fmt.Printf("Validate %s\n", agg.EntityID())
					fmt.Printf("\tURI: %s", checkuri)
					fmt.Printf("\n\tNOT IN TREE\n")
					//panic("Found aggregate that isn't in the tree.")
				}
			}
		} else {
			fmt.Printf("Validate %s\n", agg.EntityID())
			fmt.Printf("NOT AN RRA!\n")
			//panic("Found aggregate in store that isn't a RedfishResourceAggregate")
		}
	}

	if len(aggs) != len(d.Tree)+injectCmds || injectCmds > 1 {
		fmt.Printf("MISMATCH Tree(%d) Aggregates(%d) InjectCmds(%d)\n", len(d.Tree), len(aggs), injectCmds)
	}
	//fmt.Printf("Number of inject commands: %d\n", injectCmds)
	//fmt.Printf("Number of tree objects: %d\n", len(d.Tree))
	//fmt.Printf("Number of aggregate objects: %d\n", len(aggs))

	return
}

func (d *DomainObjects) GetAggregateIDOK(uri string) (id eh.UUID, ok bool) {
	d.treeMu.RLock()
	defer d.treeMu.RUnlock()
	id, ok = d.Tree[uri]

	if !ok {
		// strip out any trailing slash and try again
		i := 0
		// start at the end and while the next char is '/', increment
		for i = 0; i < len(uri) && uri[len(uri)-(1+i)] == '/'; i++ {
		}
		// use all the way up to the last non-'/' char
		id, ok = d.Tree[uri[:len(uri)-(i)]]
	}

	return
}

func (d *DomainObjects) FindMatchingURIs(matcher func(string) bool) []string {
	d.treeMu.RLock()
	defer d.treeMu.RUnlock()
	ret := []string{}
	for uri, _ := range d.Tree {
		if matcher(uri) {
			ret = append(ret, uri)
		}
	}
	return ret
}

func (d *DomainObjects) ExpandURI(ctx context.Context, uri string) (*RedfishResourceProperty, error) {
	aggID, ok := d.GetAggregateIDOK(uri)
	if !ok {
		return nil, errors.New("URI does not exist: " + uri)
	}
	agg, _ := d.AggregateStore.Load(ctx, AggregateType, aggID)
	redfishResource, ok := agg.(*RedfishResourceAggregate)
	if !ok {
		return nil, errors.New("Problem loading URI from aggregate store: " + uri)
	}

	// TODO: check to see if .Meta of the properties is set and call process on it if so

	return &redfishResource.Properties, nil
}

// Notify implements the Notify method of the EventObserver interface.
func (d *DomainObjects) Notify(ctx context.Context, event eh.Event) {
	logger := ContextLogger(ctx, "domain")
	logger.Debug("EVENT", "event", event, "data", event.Data())
	if event.EventType() == RedfishResourceCreated {
		if data, ok := event.Data().(*RedfishResourceCreatedData); ok {
			logger.Info("Create URI", "URI", data.ResourceURI)

			// Adding a new aggregate to the tree
			d.treeMu.Lock()
			defer d.treeMu.Unlock()

			// First, remove any potential older aggregate that resides in the tree at this URI with different UUID
			if UUID, ok := d.Tree[data.ResourceURI]; ok && UUID != data.ID {
				// TODO: need to actually run the removeredfishresource command here instead of directly removing resource
				// TODO: Probably put this command into the inject queue?
				d.Repo.Remove(ctx, UUID)
			}

			// Next, attach this aggregate into the tree (possibly overwriting old def)
			d.Tree[data.ResourceURI] = data.ID

			// check for orphaned aggregates by iterating over all aggregates and finding any that claim to be this resource URI
			aggs, _ := d.Repo.FindAll(context.Background())
			for _, agg := range aggs {
				if rr, ok := agg.(*RedfishResourceAggregate); ok {
					if rr.ResourceURI == data.ResourceURI && rr.EntityID() != data.ID {
						fmt.Printf("FOUND ORPHAN, deleting\n")
						d.Repo.Remove(ctx, rr.EntityID())
					}
				}
			}

		}
		return
	} else if event.EventType() == RedfishResourceRemoved {
		if data, ok := event.Data().(*RedfishResourceRemovedData); ok {
			logger.Info("Delete URI", "URI", data.ResourceURI)
			d.treeMu.Lock()
			defer d.treeMu.Unlock()

			// directly remove the aggregate from the aggregate repo
			d.Repo.Remove(ctx, data.ID)

			UUID, ok := d.Tree[data.ResourceURI]

			// if it's *this* specific aggregate still in the tree, remove it from the tree
			if ok && UUID == data.ID {
				delete(d.Tree, data.ResourceURI)

				// remove any plugins linked to the now unlinked agg. Careful here
				// because if a new aggregate is linked in we dont want to delete the
				// new plugins that may have already been instantiated
				p, err := InstantiatePlugin(PluginType(data.ResourceURI))
				type closer interface {
					Close()
				}
				if err == nil && p != nil {
					if c, ok := p.(closer); ok {
						c.Close()
					}
				}
			}

			// check for orphaned aggregates by iterating over all aggregates and finding any that claim to be this resource URI
			aggs, _ := d.Repo.FindAll(context.Background())
			for _, agg := range aggs {
				if rr, ok := agg.(*RedfishResourceAggregate); ok {
					if rr.ResourceURI == data.ResourceURI && rr.EntityID() != UUID {
						fmt.Printf("FOUND ORPHAN, deleting\n")
						d.Repo.Remove(ctx, rr.EntityID())
					}
				}
			}
		}
		return
	}
}

// CommandHandler is a HTTP handler for eventhorizon.Commands. Commands must be
// registered with eventhorizon.RegisterCommand(). It expects a POST with a JSON
// body that will be unmarshalled into the command.
func (d *DomainObjects) GetInternalCommandHandler(backgroundCtx context.Context) http.Handler {
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
		r.Body.Close()
		if err != nil {
			http.Error(w, "could not read command: "+err.Error(), http.StatusBadRequest)
			return
		}
		r.Body.Close()

		contentType := r.Header.Get("Content-type")
		if contentType == "application/xml" {
			if err := xml.Unmarshal(b, &cmd); err != nil {
				http.Error(w, "could not decode command: "+err.Error(), http.StatusBadRequest)
				return
			}
		} else {
			if err := json.Unmarshal(b, &cmd); err != nil {
				http.Error(w, "could not decode command: "+err.Error(), http.StatusBadRequest)
				return
			}
		}

		// NOTE: Use a new context when handling, else it will be cancelled with
		// the HTTP request which will cause projectors etc to fail if they run
		// async in goroutines past the request.
		if err := d.CommandHandler.HandleCommand(backgroundCtx, cmd); err != nil {
			http.Error(w, "could not handle command: "+err.Error(), http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
