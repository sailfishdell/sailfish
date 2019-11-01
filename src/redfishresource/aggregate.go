package domain

import (
	"context"
	"sync"
	"time"
	//"fmt"

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/sailfish/src/looplab/aggregatestore"
)

const AggregateType = eh.AggregateType("RedfishResource")

func init() {
	RegisterInitFN(RegisterRRA)
}

func (a *RedfishResourceAggregate) HandleCommand(ctx context.Context, command eh.Command) error {
	switch command := command.(type) {
	case RRCmdHandler:
		return command.Handle(ctx, a)
	}

	return nil
}

func RegisterRRA(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew waiter) {
	eh.RegisterAggregate(func(id eh.UUID) eh.Aggregate {
		return &RedfishResourceAggregate{}
	})
}

type RedfishResourceAggregate struct {
	events   []eh.Event
	eventsMu sync.RWMutex

	// public
	ID          eh.UUID
	ResourceURI string
	Plugin      string

	Properties     RedfishResourceProperty
	resultsCacheMu sync.RWMutex
	StatusCode     int // http status code for the current state of this object since the last time we've run the meta functions
	DefaultFilter  string

	// TODO: need accessor functions for all of these just like property stuff
	// above so that everything can be properly locked
	PrivilegeMap map[HTTPReqType]interface{}
	Headers      map[string]string

	// debug and beancounting
	checkcount int                       // watchdog process uses this to try to do race-free detection of orphan aggregates
	access     map[HTTPReqType]time.Time // store beancounting about when uri's were accessed
}

func (agg *RedfishResourceAggregate) Lock() {
	agg.resultsCacheMu.Lock()
}

func (agg *RedfishResourceAggregate) Unlock() {
	agg.resultsCacheMu.Unlock()
}

func (agg *RedfishResourceAggregate) RLock() {
	agg.resultsCacheMu.RLock()
}

func (agg *RedfishResourceAggregate) RUnlock() {
	agg.resultsCacheMu.RUnlock()
}

// PublishEvent registers an event to be published after the aggregate
// has been successfully saved.
func (a *RedfishResourceAggregate) PublishEvent(e eh.Event) {
	a.eventsMu.Lock()
	a.events = append(a.events, e)
	a.eventsMu.Unlock()
}

// EventsToPublish implements the EventsToPublish method of the EventPublisher interface.
func (a *RedfishResourceAggregate) EventsToPublish() (ret []eh.Event) {
	a.eventsMu.Lock()
	ret = a.events
	a.events = []eh.Event{}
	a.eventsMu.Unlock()
	return
}

// ClearEvents implements the ClearEvents method of the EventPublisher interface.
// no-op for now so we can avoid a race. EventsToPublish does a clear, so redundant here
func (a *RedfishResourceAggregate) ClearEvents() {
}

func (r *RedfishResourceAggregate) AggregateType() eh.AggregateType { return AggregateType }
func (r *RedfishResourceAggregate) EntityID() eh.UUID               { return r.ID }

func NewRedfishResourceAggregate(id eh.UUID) *RedfishResourceAggregate {
	return &RedfishResourceAggregate{}
}

// Two types of commands: HTTP commands, and Backend commands
//
// HTTP Commands: GET, PUT, PATCH, POST, DELETE, HEAD, OPTIONS
//      HTTP Commands get parameters (reqId, params) and emit an HTTPResponse Event with matching reqId
//      exposed via http redfish interface
//      These must be satisfied by base redfish resource aggregate
//      going to make this a pluggable system where we can extend how GET/etc works
//
// Backend Commands: CreateResource, DeleteResource, {Add|Update|Remove}Properties, UpdatePrivileges, UpdatePermissions, UpdateHeaders
//      exposed via dbus api
//      exposed via internal http interface
//
// Other commands? Other aggregates that might do other commands? Can we introspect and automatically register dbus commands?
//
// how do we get events into aggregates?
//      I think CreateResource (plugin="foo" ...) foo plugin registers with foo saga

type RRCmdHandler interface {
	Handle(ctx context.Context, a *RedfishResourceAggregate) error
}

type CommandHandler struct {
	sync.RWMutex
	t        eh.AggregateType
	store    aggregatestore.AggregateStore // commands to manage stored aggregates
	cmdSlice []*aggregateStoreStatus
	bus      eh.EventBus
}

// NewCommandHandler creates a new CommandHandler for an aggregate type.
func NewCommandHandler(t eh.AggregateType, store aggregatestore.AggregateStore, bus eh.EventBus) (*CommandHandler, error) {

	h := &CommandHandler{
		t:        "RedfishResource",
		store:    store,
		cmdSlice: []*aggregateStoreStatus{},
		bus:      bus,
	}
	return h, nil
}

type aggregateStoreStatus struct {
	aggLock   sync.RWMutex
	aggStatus eh.Aggregate
	save2DB   int
}

func (as *aggregateStoreStatus) Lock() {
	as.aggLock.Lock()
}

func (as *aggregateStoreStatus) Unlock() {
	as.aggLock.Unlock()
}

// Saves aggregates in the order they enter the HandleCommand
func (h *CommandHandler) Save2DB(ctx context.Context, aPtr *aggregateStoreStatus) {

	h.RLock()
	defer h.RUnlock()
	if aPtr != h.cmdSlice[0] {
		return
	}

	cnt := 0

	for i, a := range h.cmdSlice {
		agg := a.aggStatus
		action := a.save2DB

		if action == 1 {
			h.store.Save(ctx, agg)
			h.cmdSlice[i] = nil
			cnt += 1
		} else if action == 2 {
			h.cmdSlice[i] = nil
			cnt += 1
		} else {
			break
		}
	}
	h.cmdSlice = h.cmdSlice[cnt:]
}

// EventPublisher is an optional event publisher that can be implemented by
// aggregates to allow for publishing of events on a successful save.
type EventPublisher interface {
	// EventsToPublish returns all events to publish.
	EventsToPublish() []eh.Event
	// ClearEvents clears all events after a publish.
	ClearEvents()
}

func (h *CommandHandler) HandleCommand(ctx context.Context, cmd eh.Command) error {
	aggStatus := aggregateStoreStatus{}
	// 2 - delete
	// 1 - save
	// 0   - skip

	err := eh.CheckCommand(cmd)
	if err != nil {
		return err
	}

	a, err := h.store.Load(ctx, h.t, cmd.AggregateID())
	if err != nil {
		return err
	} else if a == nil {
		return eh.ErrAggregateNotFound
	}

	cmdType := cmd.CommandType()
	if cmdType == CreateRedfishResourceCommand ||
		cmdType == UpdateRedfishResourcePropertiesCommand ||
		cmdType == UpdateRedfishResourcePropertiesCommand2 ||
		cmdType == UpdateMetricRedfishResourcePropertiesCommand ||
		cmdType == RemoveRedfishResourcePropertyCommand {

		aggStatus.aggStatus = a
		aggStatus.save2DB = 1
		h.Lock()
		h.cmdSlice = append(h.cmdSlice, &aggStatus)
		h.Unlock()
	}

	if err = a.HandleCommand(ctx, cmd); err != nil {
		if aggStatus.save2DB >= 1 {
			aggStatus.save2DB = 2
		}
		return err
	} else {
		var events []eh.Event
		publisher, ok := a.(EventPublisher)
		if ok && h.bus != nil {
			events = publisher.EventsToPublish()
		}

		if aggStatus.save2DB >= 1 {
			h.Save2DB(ctx, &aggStatus)
		}

		if ok && h.bus != nil {
			publisher.ClearEvents()
			for _, e := range events {
				h.bus.PublishEvent(ctx, e)
			}
		}

	}

	return nil
}
