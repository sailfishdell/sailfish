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

func (d *DomainObjects) SetAggregateID(uri string, ID eh.UUID) {
	d.treeMu.Lock()
	defer d.treeMu.Unlock()
	d.Tree[uri] = ID
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

func (d *DomainObjects) DeleteResource(ctx context.Context, uri string) {
	d.treeMu.Lock()
	defer d.treeMu.Unlock()
	if UUID, ok := d.Tree[uri]; ok {
		d.Repo.Remove(ctx, UUID)
	}
	delete(d.Tree, uri)
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
			// First, delete any old resource at that URI
			d.DeleteResource(ctx, data.ResourceURI)
			// then attach the new resource
			d.SetAggregateID(data.ResourceURI, data.ID)
			// no need to worry about plugins as new plugin should automatically overwrite any old one.
		}
		return
	}
	if event.EventType() == RedfishResourceRemoved {
		if data, ok := event.Data().(*RedfishResourceRemovedData); ok {
			logger.Info("Delete URI", "URI", data.ResourceURI)
			d.DeleteResource(ctx, data.ResourceURI)
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
