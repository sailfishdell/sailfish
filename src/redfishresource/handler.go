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
	"github.com/superchalupa/sailfish/src/looplab/aggregatestore"
	repo "github.com/superchalupa/sailfish/src/looplab/bbolt"
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

// define the starting capacity
//  	tuned to EC by default. about 330 default entries plus normal max 3k LCL entries
var INITIAL_CAPACITY int = 3500

// SetupDDDFunctions sets up the full Event Horizon domain
// returns a handler exposing some of the components.
func NewDomainObjects() (*DomainObjects, error) {
	d := DomainObjects{}

	d.Tree = make(map[string]eh.UUID, INITIAL_CAPACITY)

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
	d.EventBus.AddHandler(eh.MatchEvent(HTTPCmdProcessed), d.HTTPPublisher)

	// hook up http waiter to the other bus for back compat
	d.HTTPWaiter = eventwaiter.NewEventWaiter(eventwaiter.SetName("HTTP"), eventwaiter.NoAutoRun)
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
					continue
				} else {
					fmt.Printf("Validate %s\n", agg.EntityID())
					fmt.Printf("\tURI: %s", checkuri)

					rr.Lock()
					fmt.Printf("\n\tAggregate ID Mismatch! %s != %s  (count: %d)\n", id, agg.EntityID(), rr.checkcount)
					if rr.checkcount > 0 {
						fmt.Printf("\n\tCheck expired, assuming hanging aggregate and removing\n")
						d.Repo.Remove(context.Background(), rr.EntityID())
					} else {
						rr.checkcount++
					}
					rr.Unlock()

				}
			} else {
				if string(agg.EntityID()) == string(injectUUID) {
					// it's an inject command. that's ok
					injectCmds++
				} else {
					// aggregate isn't in the tree at that uri
					fmt.Printf("Validate %s\n", agg.EntityID())
					fmt.Printf("\tURI: %s", checkuri)
					rr.Lock()
					fmt.Printf("\n\tNOT IN TREE: %d\n", rr.checkcount)
					if rr.checkcount > 0 {
						fmt.Printf("\n\tCheck expired, assuming hanging aggregate and removing\n")

						d.Repo.Remove(context.Background(), rr.EntityID())
						// remove any plugins linked to the now unlinked agg. Careful here
						// because if a new aggregate is linked in we dont want to delete the
						// new plugins that may have already been instantiated
						p, err := InstantiatePlugin(PluginType(rr.ResourceURI))
						type closer interface {
							Close()
						}
						if err == nil && p != nil {
							if c, ok := p.(closer); ok {
								c.Close()
							}
						}
					} else {
						rr.checkcount++
					}
					rr.Unlock()
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

	// slower, but fewer allocations and less garbage created
	matchCount := 0
	for uri, _ := range d.Tree {
		if matcher(uri) {
			matchCount++
		}
	}

	ret := make([]string, 0, matchCount)
	// optimize no match count by not scanning tree again
	if matchCount > 0 {
		for uri, _ := range d.Tree {
			if matcher(uri) {
				ret = append(ret, uri) // preallocated
				// TODO: optimize here by decrementing matchcount and early out when we hit last match. be sure no off by one errors
			}
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

		}
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
		}
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

// DumpStatus is for debugging
func (d *DomainObjects) DumpStatus() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		// Adding a new aggregate to the tree
		d.treeMu.Lock()
		defer d.treeMu.Unlock()

		// Dump Tree
		injectCmds := 0
		orphans := 0
		fmt.Fprintf(w, "DUMP Aggregate Repository\n")
		aggs, _ := d.Repo.FindAll(context.Background())
		for _, agg := range aggs {
			fmt.Fprintf(w, "================================================\n")
			if rr, ok := agg.(*RedfishResourceAggregate); ok {
				fmt.Fprintf(w, "RedfishResourceAggregate: ")
				treeLookup, ok := d.Tree[rr.ResourceURI]
				if ok && treeLookup == rr.EntityID() {
					fmt.Fprintf(w, "OK")
				} else if ok {
					orphans++
					fmt.Fprintf(w, "MISMATCH(tree has id %s)", treeLookup)
				} else {
					if agg.EntityID() == injectUUID {
						fmt.Fprintf(w, "INJECTCMD")
						injectCmds++
					}
				}

				fmt.Fprintf(w, " %s: %s\n", rr.EntityID(), rr.ResourceURI)
				for k, v := range rr.access {
					fmt.Fprintf(w, "\t%s: %s\n", MapHTTPReqToString(k), v)
				}
			} else {
				fmt.Fprintf(w, "UNKNOWN ENTITY: %s\n", rr.EntityID())
			}
			fmt.Fprintf(w, "\n\n")
		}

		pluginsMu.Lock()
		defer pluginsMu.Unlock()
		fmt.Fprintf(w, "\nPLUGIN DUMP\n")
		for k, _ := range plugins {
			fmt.Fprintf(w, "Plugin: %s\n", k)
		}

		fmt.Fprintf(w, "\nSTATS DUMP\n")
		fmt.Fprintf(w, "Tree(%d) Aggregates(%d) InjectCmds(%d) Orphans(%d)\n", len(d.Tree), len(aggs), injectCmds, orphans)
		fmt.Fprintf(w, "InjectChan Q Len = %d\n", len(injectChan))
		fmt.Fprintf(w, "# PLUGINS = %d\n", len(plugins))

	})
}
