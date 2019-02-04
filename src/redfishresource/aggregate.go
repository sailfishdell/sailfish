package domain

import (
	"context"
	"sync"

	eh "github.com/looplab/eventhorizon"
)

const AggregateType = eh.AggregateType("RedfishResource")

func init() {
	RegisterInitFN(RegisterRRA)
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
	ResultsCache   interface{}
	ResultsCacheMu sync.RWMutex
	CacheTimeSec   int

	// TODO: need accessor functions for all of these just like property stuff
	// above so that everything can be properly locked
	PrivilegeMap map[string]interface{}
	Headers      map[string]string
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

func (a *RedfishResourceAggregate) HandleCommand(ctx context.Context, command eh.Command) error {
	switch command := command.(type) {
	case RRCmdHandler:
		return command.Handle(ctx, a)
	}

	return nil
}
