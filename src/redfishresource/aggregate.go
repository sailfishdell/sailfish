package domain

import (
	"context"
	"encoding/json"
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
		prop := RedfishResourceProperty{}
		prop.Parse(v)
		ret = append(ret, prop)
	}
	return
}

func parse_map(start map[string]interface{}, props map[string]interface{}) {
	for k, v := range props {
		if strings.HasSuffix(k, "@meta") {
			name := k[:len(k)-5]
			prop, ok := start[name].(RedfishResourceProperty)
			if !ok {
				prop = RedfishResourceProperty{}
			}
			prop.Meta = v.(map[string]interface{})
			start[name] = prop
		} else {
			prop, ok := start[k].(RedfishResourceProperty)
			if !ok {
				prop = RedfishResourceProperty{}
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

func (r *RedfishResourceAggregate) GetProperty(p string) (ret interface{}) {
	r.propertiesMu.RLock()
	defer r.propertiesMu.RUnlock()

	v := r.properties.Value.(map[string]interface{})
	rrp, ok := v[p].(RedfishResourceProperty)

	if ok {
		return rrp.Value
	}
	return nil
}

func (r *RedfishResourceAggregate) SetProperty(p string, n interface{}) {
	r.propertiesMu.Lock()
	defer r.propertiesMu.Unlock()

	// new hotness
	v := r.properties.Value.(map[string]interface{})
	rrp, ok := v[p].(RedfishResourceProperty)
	if !ok {
		rrp = RedfishResourceProperty{}
	}
	rrp.Value = n
	v[p] = rrp
}

func (r *RedfishResourceAggregate) DeleteProperty(p string) {
	r.propertiesMu.Lock()
	defer r.propertiesMu.Unlock()

	// new hotness
	v := r.properties.Value.(map[string]interface{})
	delete(v, p)
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

	membersRRP, ok := props["Members"].(RedfishResourceProperty)
	if !ok {
		props["Members"] = RedfishResourceProperty{Value: []map[string]interface{}{}}
		membersRRP = props["Members"].(RedfishResourceProperty)
	}

	if _, ok := membersRRP.Value.([]map[string]interface{}); !ok {
		props["Members"] = RedfishResourceProperty{Value: []map[string]interface{}{}}
		membersRRP = props["Members"].(RedfishResourceProperty)
	}

	return &membersRRP
}

func (r *RedfishResourceAggregate) AddCollectionMember(uri string) {
	r.propertiesMu.Lock()
	defer r.propertiesMu.Unlock()
	members := r.EnsureCollection_unlocked()
	members.Value = append(members.Value.([]map[string]interface{}), map[string]interface{}{"@odata.id": RedfishResourceProperty{Value: uri}})
	m := r.properties.Value.(map[string]interface{})
	m["Members"] = *members
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
		rrp, ok := v["@odata.id"].(RedfishResourceProperty)
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
	members.Value = arr[:len(arr)-1]

	m := r.properties.Value.(map[string]interface{})
	m["Members"] = *members
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
	m["Members@odata.count"] = RedfishResourceProperty{Value: l}
}

type PropertyGetter interface {
	PropertyGet(context.Context, *RedfishResourceAggregate, *RedfishResourceProperty, map[string]interface{})
}
type PropertyPatcher interface {
	PropertyPatch(context.Context, *RedfishResourceAggregate, *RedfishResourceProperty, map[string]interface{}, interface{}, bool)
}

func (rrp *RedfishResourceProperty) Process(ctx context.Context, agg *RedfishResourceAggregate, property, method string, req interface{}, present bool) (ret RedfishResourceProperty) {
	rrp.Lock()
	defer rrp.Unlock()

	// set up return copy. We are not going to modify our source
	ret = RedfishResourceProperty{Meta: map[string]interface{}{}}
	for k, v := range rrp.Meta {
		ret.Meta[k] = v
	}
	ret.Value = rrp.Value

	ret.Lock()
	defer ret.Unlock()

	// The purpose of this function is to recursively process the resource property, calling any plugins that are specified in the meta data.
	// step 1: run the plugin to update rrp.Value based on the plugin.
	// Step 2: see if the rrp.Value is a recursable map or array and recurse down it

	// equivalent to do{}while(1) to run once
	// if any of the intermediate steps fails, bail out on this part and continue by doing the next thing
	for ok := true; ok; ok = false {
		meta_t, ok := ret.Meta[method].(map[string]interface{})
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

		switch method {
		case "GET":
			if plugin, ok := plugin.(PropertyGetter); ok {
				plugin.PropertyGet(ctx, agg, &ret, meta_t)
			}
		case "PATCH":
			if plugin, ok := plugin.(PropertyPatcher); ok {
				plugin.PropertyPatch(ctx, agg, &ret, meta_t, req, present)
			}
		}
	}

	switch ret.Value.(type) {
	case map[string]interface{}:
		// somewhat complicated here, but not too bad: set up goroutines for
		// each sub object and process in parallel. Collect results via array
		// of channels.
		type result struct {
			name   string
			result interface{}
		}
		reqmap, _ := req.(map[string]interface{})
		var promised []chan result
		for property, v := range ret.Value.(map[string]interface{}) {
			resChan := make(chan result)
			promised = append(promised, resChan)
			if vrr, ok := v.(RedfishResourceProperty); ok {
				go func(property string, v RedfishResourceProperty) {
					reqitem, ok := reqmap[property]
					retProp := v.Process(ctx, agg, property, method, reqitem, ok)
					resChan <- result{property, retProp}
				}(property, vrr)
			} else {
				go func(property string, v interface{}) {
					resChan <- result{property, v}
				}(property, v)
			}
		}
		newMap := map[string]interface{}{}
		for _, resChan := range promised {
			res := <-resChan
			newMap[res.name] = res.result
		}
		ret.Value = newMap

	case []interface{}:
		// spawn off parallel goroutines to process each member of the array
		reqarr, _ := req.([]interface{})
		var promised []chan interface{}
		for index, v := range ret.Value.([]interface{}) {
			resChan := make(chan interface{})
			promised = append(promised, resChan)
			if v, ok := v.(RedfishResourceProperty); ok {
				go func(index int, v RedfishResourceProperty) {
					var reqitem interface{} = nil
					var ok bool = false
					if index < len(reqarr) {
						reqitem = reqarr[index]
						ok = true
					}
					retProp := v.Process(ctx, agg, property, method, reqitem, ok)
					resChan <- retProp
				}(index, v)
			} else {
				go func(property string, v interface{}) {
					resChan <- v
				}(property, v)
			}
		}

		// collect all the results together after processing.
		newArr := []interface{}{}
		for _, resChan := range promised {
			res := <-resChan
			newArr = append(newArr, res)
		}
		ret.Value = newArr
	}

	return
}

func (agg *RedfishResourceAggregate) ProcessMeta(ctx context.Context, method string, request map[string]interface{}) (results interface{}, err error) {
	agg.propertiesMu.Lock()
	defer agg.propertiesMu.Unlock()

	results = agg.properties.Process(ctx, agg, "", method, request, true)

	return
}
