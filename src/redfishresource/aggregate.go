package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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

type RedfishResourceProperty struct {
	sync.Mutex
	Value interface{}
	Meta  map[string]interface{}
}

func (rrp RedfishResourceProperty) MarshalJSON() ([]byte, error) {
	rrp.Lock()
	defer rrp.Unlock()
	return json.Marshal(rrp.Value)
}

func (rrp *RedfishResourceProperty) Parse(thing interface{}) {
	rrp.Lock()
	defer rrp.Unlock()
	switch thing.(type) {
	case []interface{}:
		if _, ok := rrp.Value.([]interface{}); !ok || rrp.Value == nil {
			rrp.Value = []interface{}{}
		}
		rrp.Value = append(rrp.Value.([]interface{}), parse_array(thing.([]interface{}))...)
	case map[string]interface{}:
		v, ok := rrp.Value.(map[string]interface{})
		if !ok || v == nil {
			rrp.Value = map[string]interface{}{}
		}
		parse_map(rrp.Value.(map[string]interface{}), thing.(map[string]interface{}))
	default:
		rrp.Value = thing
	}
	return
}

func parse_array(props []interface{}) (ret []interface{}) {
	for _, v := range props {
		prop := &RedfishResourceProperty{}
		prop.Parse(v)
		ret = append(ret, prop)
	}
	return
}

func parse_map(start map[string]interface{}, props map[string]interface{}) {
	for k, v := range props {
		if strings.HasSuffix(k, "@meta") {
			name := k[:len(k)-5]
			prop, ok := start[name].(*RedfishResourceProperty)
			if !ok {
				prop = &RedfishResourceProperty{}
			}
			prop.Meta = v.(map[string]interface{})
			start[name] = prop
		} else {
			prop, ok := start[k].(*RedfishResourceProperty)
			if !ok {
				prop = &RedfishResourceProperty{}
			}
			prop.Parse(v)
			start[k] = prop
		}
	}
	return
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

type PropertyGetter interface {
	PropertyGet(context.Context, *RedfishResourceAggregate, *RedfishResourceProperty, map[string]interface{})
}
type PropertyPatcher interface {
	PropertyPatch(context.Context, *RedfishResourceAggregate, *RedfishResourceProperty, map[string]interface{}, interface{}, bool)
}

func (rrp *RedfishResourceProperty) Process(ctx context.Context, agg *RedfishResourceAggregate, property, method string, req interface{}, present bool) {
	rrp.Lock()
	defer rrp.Unlock()

	// The purpose of this function is to recursively process the resource property, calling any plugins that are specified in the meta data.
	// step 1: run the plugin to update rrp.Value based on the plugin.
	// Step 2: see if the rrp.Value is a recursable map or array and recurse down it

	// equivalent to do{}while(1) to run once
	// if any of the intermediate steps fails, bail out on this part and continue by doing the next thing
	for ok := true; ok; ok = false {
		meta_t, ok := rrp.Meta[method].(map[string]interface{})
		if !ok {
			break
		}

		pluginName, ok := meta_t["plugin"].(string)
		if !ok {
			break
		}

		plugin, err := InstantiatePlugin(PluginType(pluginName))
		if err != nil {
			ContextLogger(ctx, "aggregate").Warn("Orphan property, could not load plugin", "property", property, "plugin", pluginName, "err", err)
			break
		}

		ContextLogger(ctx, "aggregate").Debug("getting property", "method", method, "value", fmt.Sprintf("%v", rrp.Value))
		switch method {
		case "GET":
			ContextLogger(ctx, "aggregate").Debug("getting property: GET", "method", method, "value", fmt.Sprintf("%v", rrp.Value))
			if plugin, ok := plugin.(PropertyGetter); ok {
				ContextLogger(ctx, "aggregate").Debug("getting property: GET - type assert success", "method", method, "value", fmt.Sprintf("%v", rrp.Value))
				plugin.PropertyGet(ctx, agg, rrp, meta_t)
				ContextLogger(ctx, "aggregate").Debug("AFTER getting property: GET - type assert success", "method", method, "value", fmt.Sprintf("%v", rrp.Value))
			}
		case "PATCH":
			ContextLogger(ctx, "aggregate").Debug("getting property: PATCH", "method", method, "value", fmt.Sprintf("%v", rrp.Value))
			if plugin, ok := plugin.(PropertyPatcher); ok {
				ContextLogger(ctx, "aggregate").Debug("getting property: PATCH - type assert success", "method", method, "value", fmt.Sprintf("%v", rrp.Value))
				plugin.PropertyPatch(ctx, agg, rrp, meta_t, req, present)
				ContextLogger(ctx, "aggregate").Debug("AFTER getting property: PATCH - type assert success", "method", method, "value", fmt.Sprintf("%v", rrp.Value))
			}
		}

		ContextLogger(ctx, "aggregate").Debug("GOT Property", "rrp", rrp, "TYPE", fmt.Sprintf("%T", rrp), "rrp.Value.TYPE", fmt.Sprintf("%T", rrp.Value))

	}

	switch t := rrp.Value.(type) {
	case map[string]interface{}:
		ContextLogger(ctx, "aggregate").Debug("Handle MAP", "rrp", rrp, "TYPE", fmt.Sprintf("%T", t))
		var wg sync.WaitGroup
		for property, v := range t {
			if vrr, ok := v.(*RedfishResourceProperty); ok {
				wg.Add(1)
				go func(property string, v *RedfishResourceProperty) {
					defer wg.Done()
					reqmap, ok := req.(map[string]interface{})
					var reqitem interface{}
					if ok {
						reqitem, ok = reqmap[property]
					}
					v.Process(ctx, agg, property, method, reqitem, ok)
				}(property, vrr)
			}
		}
		wg.Wait()
		ContextLogger(ctx, "aggregate").Debug("DONE Handle MAP", "rrp", rrp, "TYPE", fmt.Sprintf("%T", rrp.Value))

	case []interface{}:
		ContextLogger(ctx, "aggregate").Debug("Handle ARRAY", "rrp", rrp, "TYPE", fmt.Sprintf("%T", rrp.Value))
		// spawn off parallel goroutines to process each member of the array
		var wg sync.WaitGroup
		for index, v := range rrp.Value.([]interface{}) {
			if v, ok := v.(*RedfishResourceProperty); ok {
				wg.Add(1)
				go func(index int, v *RedfishResourceProperty) {
					defer wg.Done()
					var reqitem interface{} = nil
					var ok bool = false
					reqarr, ok := req.([]interface{})
					if ok {
						if index < len(reqarr) {
							reqitem = reqarr[index]
							ok = true
						}
					}
					v.Process(ctx, agg, property, method, reqitem, ok)
				}(index, v)
			}
		}
		wg.Wait()
		ContextLogger(ctx, "aggregate").Debug("DONE Handle ARRAY", "rrp", rrp, "TYPE", fmt.Sprintf("%T", rrp.Value))

	case *RedfishResourceProperty:
		ContextLogger(ctx, "aggregate").Debug("Handle single", "rrp", rrp, "TYPE", fmt.Sprintf("%T", rrp.Value))
		if v, ok := rrp.Value.(*RedfishResourceProperty); ok {
			v.Process(ctx, agg, property, method, req, true)
		}
		ContextLogger(ctx, "aggregate").Debug("DONE Handle single", "rrp", rrp, "TYPE", fmt.Sprintf("%T", rrp.Value))

	default:
		ContextLogger(ctx, "aggregate").Debug("CANT HANDLE", "rrp", rrp, "TYPE", fmt.Sprintf("%T", rrp.Value))

	}

	ContextLogger(ctx, "aggregate").Debug("ALL DONE: GOT Property", "rrp", rrp, "TYPE", fmt.Sprintf("%T", rrp.Value), "Value", fmt.Sprintf("%+v", rrp.Value))

	return
}

func (agg *RedfishResourceAggregate) ProcessMeta(ctx context.Context, method string, request map[string]interface{}) (results interface{}, err error) {
	agg.propertiesMu.Lock()
	defer agg.propertiesMu.Unlock()

	agg.properties.Process(ctx, agg, "", method, request, true)

	var dst RedfishResourceProperty
	Copy(&dst, &agg.properties)

	ContextLogger(ctx, "aggregate").Warn("ProcessMeta DONE", "dst", dst, "agg", agg)

	return dst, nil
}
