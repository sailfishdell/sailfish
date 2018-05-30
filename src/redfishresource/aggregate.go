package domain

import (
	"context"
	"sync"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

const AggregateType = eh.AggregateType("RedfishResource")

func init() {
	RegisterInitFN(RegisterRRA)
}

func RegisterRRA(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
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

	propertiesMu sync.RWMutex
	properties   RedfishResourceProperty

	// TODO: need accessor functions for all of these just like property stuff
	// above so that everything can be properly locked
	PrivilegeMap map[string]interface{}
	Headers      map[string]string
}

// PublishEvent registers an event to be published after the aggregate
// has been successfully saved.
func (a *RedfishResourceAggregate) PublishEvent(e eh.Event) {
	a.eventsMu.Lock()
	defer a.eventsMu.Unlock()
	a.events = append(a.events, e)
}

// EventsToPublish implements the EventsToPublish method of the EventPublisher interface.
func (a *RedfishResourceAggregate) EventsToPublish() []eh.Event {
	a.eventsMu.RLock()
	defer a.eventsMu.RUnlock()
	retArr := make([]eh.Event, len(a.events))
	copy(retArr, a.events)
	return retArr
}

// ClearEvents implements the ClearEvents method of the EventPublisher interface.
func (a *RedfishResourceAggregate) ClearEvents() {
	a.eventsMu.Lock()
	defer a.eventsMu.Unlock()
	a.events = []eh.Event{}
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

func (r *RedfishResourceAggregate) EnsureCollection() {
	r.propertiesMu.Lock()
	defer r.propertiesMu.Unlock()
	r.EnsureCollection_unlocked()
	r.UpdateCollectionMemberCount_unlocked()
}

func (r *RedfishResourceAggregate) EnsureCollection_unlocked() *RedfishResourceProperty {
	props, ok := r.properties.Value.(map[string]interface{})
	if !ok {
		r.properties.Value = map[string]interface{}{}
		props = r.properties.Value.(map[string]interface{})
	}

	membersRRP, ok := props["Members"].(*RedfishResourceProperty)
	if !ok {
		props["Members"] = &RedfishResourceProperty{Value: []map[string]interface{}{}}
		membersRRP = props["Members"].(*RedfishResourceProperty)
	}

	if _, ok := membersRRP.Value.([]map[string]interface{}); !ok {
		props["Members"] = &RedfishResourceProperty{Value: []map[string]interface{}{}}
		membersRRP = props["Members"].(*RedfishResourceProperty)
	}

	return membersRRP
}

func (r *RedfishResourceAggregate) AddCollectionMember(uri string) {
	r.propertiesMu.Lock()
	defer r.propertiesMu.Unlock()
	members := r.EnsureCollection_unlocked()
	members.Value = append(members.Value.([]map[string]interface{}), map[string]interface{}{"@odata.id": &RedfishResourceProperty{Value: uri}})
	m := r.properties.Value.(map[string]interface{})
	m["Members"] = members
	r.UpdateCollectionMemberCount_unlocked()
}

func (r *RedfishResourceAggregate) RemoveCollectionMember(uri string) {
	r.propertiesMu.Lock()
	defer r.propertiesMu.Unlock()
	members := r.EnsureCollection_unlocked()

	arr, ok := members.Value.([]map[string]interface{})
	if !ok {
		return
	}

	for i, v := range arr {
		rrp, ok := v["@odata.id"].(*RedfishResourceProperty)
		if !ok {
			continue
		}

		mem_uri, ok := rrp.Value.(string)
		if !ok || mem_uri != uri {
			continue
		}
		arr[len(arr)-1], arr[i] = arr[i], arr[len(arr)-1]
		break
	}

	l := len(arr) - 1
	if l > 0 {
		members.Value = arr[:l]
	} else {
		members.Value = []map[string]interface{}{}
	}

	m := r.properties.Value.(map[string]interface{})
	m["Members"] = members
	r.UpdateCollectionMemberCount_unlocked()
}

func (r *RedfishResourceAggregate) UpdateCollectionMemberCount() {
	r.propertiesMu.Lock()
	defer r.propertiesMu.Unlock()
	r.UpdateCollectionMemberCount_unlocked()
}

func (r *RedfishResourceAggregate) UpdateCollectionMemberCount_unlocked() {
	members := r.EnsureCollection_unlocked()
	l := len(members.Value.([]map[string]interface{}))
	m := r.properties.Value.(map[string]interface{})
	m["Members@odata.count"] = &RedfishResourceProperty{Value: l}
}

func (agg *RedfishResourceAggregate) ProcessMeta(ctx context.Context, method string, request map[string]interface{}) (results interface{}, err error) {
	agg.propertiesMu.Lock()
	defer agg.propertiesMu.Unlock()

	agg.properties.Process(ctx, agg, "", method, request, true)

	var dst RedfishResourceProperty
	Copy(&dst, &agg.properties)

	ContextLogger(ctx, "aggregate").Info("ProcessMeta DONE", "dst", dst, "agg", agg)

	return dst, nil
}
