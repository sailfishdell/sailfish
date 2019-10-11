package domain

import (
	"context"
	"fmt"
	"sync"
	"time"

	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/sailfish/src/looplab/aggregatestore"
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

func (a *RedfishResourceAggregate) HandleCommand(ctx context.Context, command eh.Command) error {
	switch command := command.(type) {
	case RRCmdHandler:
		return command.Handle(ctx, a)
	}

	return nil
}

type CommandHandler struct {
	t       eh.AggregateType
	store   aggregatestore.AggregateStore // commands to manage stored aggregates
	aggChan chan aggregateStoreStatus
}

// NewCommandHandler creates a new CommandHandler for an aggregate type.
func NewCommandHandler(t eh.AggregateType, store aggregatestore.AggregateStore) (*CommandHandler, error) {

	h := &CommandHandler{
		t:       "RedfishResource",
		store:   store,
		aggChan: make(chan aggregateStoreStatus, 10),
	}
	return h, nil
}

type aggregateStoreStatus struct {
	aggStatus eh.Aggregate
	action    *string
}

// Saves aggregates in the order they enter the HandleCommand
func (h *CommandHandler) SaveorDelete(ctx context.Context) {

	chanLen := len(h.aggChan)
	if chanLen == 0 {
		return
	}
	for chanLen != 0 {
		chanLen = chanLen - 1

		if chanLen == 0 {
			return
		}

		aggS := <-h.aggChan

		agg := aggS.aggStatus
		action := *aggS.action
		aggS.action = nil

		if action == "save" {
			h.store.Save(ctx, agg)
		} else if action == "del" {
			h.store.Remove(ctx, agg)
		} else {
			fmt.Println("SaveorDelete: dropping", agg.EntityID())
		}
	}
	return
}

func (h *CommandHandler) HandleCommand(ctx context.Context, cmd eh.Command) error {
	aggStatus := aggregateStoreStatus{}
	action := ""
	aggStatus.action = &action
	isAction := false

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
	if cmdType == "internal:RedfishResource:Create" ||
		cmdType == "internal:RedfishResource:Remove" ||
		cmdType == "internal:RedfishResourceProperties:Update:2" ||
		cmdType == "internal:RedfishResourceProperties:Update" {
		aggStatus.aggStatus = a

		if cmd.CommandType() == "internal:RedfishResource:Remove" {
			*aggStatus.action = "del"
			h.aggChan <- aggStatus
		} else {
			*aggStatus.action = "save"
			h.aggChan <- aggStatus
		}
		isAction = true
	}

	if err = a.HandleCommand(ctx, cmd); err != nil {
		if isAction == true {
			*aggStatus.action = ""
		}
		return err
	}

	if isAction == true {
		h.SaveorDelete(ctx)
	}

	return nil
}
