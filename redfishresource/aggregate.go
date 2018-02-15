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
		return &RedfishResourceAggregate{eventBus: eb}
	})
}

type RedfishResourceProperty struct {
	Value interface{}
	Meta  map[string]interface{}
}

func (rrp RedfishResourceProperty) MarshalJSON() ([]byte, error) {
	return json.Marshal(rrp.Value)
}

func (rrp *RedfishResourceProperty) Parse(thing interface{}) {
	//var val interface{}
	switch thing.(type) {
	// TODO: add array parse
	case []interface{}:
		rrp.Value = parse_array(thing.([]interface{}))
	case map[string]interface{}:
		rrp.Value = map[string]RedfishResourceProperty{}
		parse_map(rrp.Value.(map[string]RedfishResourceProperty), thing.(map[string]interface{}))
	default:
		rrp.Value = thing
	}
	return
}

func parse_array(props []interface{}) (ret []RedfishResourceProperty) {
	for _, v := range props {
		prop := RedfishResourceProperty{}
		prop.Parse(v)
		ret = append(ret, prop)
	}
	return
}

func parse_map(start map[string]RedfishResourceProperty, props map[string]interface{}) {
	for k, v := range props {
		if strings.HasSuffix(k, "@meta") {
			name := k[:len(k)-5]
			prop, ok := start[name]
			if !ok {
				prop = RedfishResourceProperty{}
			}
			prop.Meta = v.(map[string]interface{})
			start[name] = prop
		} else {
			prop, ok := start[k]
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
	// private
	eventBus eh.EventBus

	// public
	ID          eh.UUID
	ResourceURI string
	Plugin      string

	newPropertiesMu sync.RWMutex
	newProperties   map[string]RedfishResourceProperty

	// TODO: need accessor functions for all of these just like property stuff
	// above so that everything can be properly locked
	PrivilegeMap map[string]interface{}
	Headers      map[string]string
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

func (r *RedfishResourceAggregate) GetProperty(p string) interface{} {
	r.newPropertiesMu.RLock()
	defer r.newPropertiesMu.RUnlock()
	return r.newProperties[p].Value
}

func (r *RedfishResourceAggregate) SetProperty(p string, v interface{}) {
	r.newPropertiesMu.Lock()
	defer r.newPropertiesMu.Unlock()
	prop, ok := r.newProperties[p]
	if !ok {
		prop = RedfishResourceProperty{}
	}
	prop.Value = v
	r.newProperties[p] = prop
}

func (r *RedfishResourceAggregate) DeleteProperty(p string) {
	r.newPropertiesMu.Lock()
	defer r.newPropertiesMu.Unlock()
	delete(r.newProperties, p)
}

func (r *RedfishResourceAggregate) EnsureCollection() {
	r.newPropertiesMu.Lock()
	defer r.newPropertiesMu.Unlock()
	r.EnsureCollection_unlocked()
	r.UpdateCollectionMemberCount_unlocked()
}

func (r *RedfishResourceAggregate) EnsureCollection_unlocked() *RedfishResourceProperty {
	members, ok := r.newProperties["Members"]
	if !ok {
		members = RedfishResourceProperty{Value: []map[string]RedfishResourceProperty{}}
		r.newProperties["Members"] = members
	}

	if _, ok := members.Value.([]map[string]RedfishResourceProperty); !ok {
		members = RedfishResourceProperty{Value: []map[string]RedfishResourceProperty{}}
		r.newProperties["Members"] = members
	}

	return &members
}

func (r *RedfishResourceAggregate) AddCollectionMember(uri string) {
	r.newPropertiesMu.Lock()
	defer r.newPropertiesMu.Unlock()
	members := r.EnsureCollection_unlocked()
	members.Value = append(members.Value.([]map[string]RedfishResourceProperty), map[string]RedfishResourceProperty{"@odata.id": RedfishResourceProperty{Value: uri}})
	r.newProperties["Members"] = *members
	r.UpdateCollectionMemberCount_unlocked()
}

func (r *RedfishResourceAggregate) RemoveCollectionMember(uri string) {
	r.newPropertiesMu.Lock()
	defer r.newPropertiesMu.Unlock()
	members := r.EnsureCollection_unlocked()

	arr, ok := members.Value.([]map[string]RedfishResourceProperty)
	if !ok {
		return
	}

	for i, v := range arr {
		mem_uri, ok := v["@odata.id"].Value.(string)
		if !ok || mem_uri != uri {
			continue
		}
		arr[len(arr)-1], arr[i] = arr[i], arr[len(arr)-1]
		break
	}
	members.Value = arr[:len(arr)-1]

	r.newProperties["Members"] = *members
	r.UpdateCollectionMemberCount_unlocked()
}

func (r *RedfishResourceAggregate) UpdateCollectionMemberCount() {
	r.newPropertiesMu.Lock()
	defer r.newPropertiesMu.Unlock()
	r.UpdateCollectionMemberCount_unlocked()
}

func (r *RedfishResourceAggregate) UpdateCollectionMemberCount_unlocked() {
	l := len(r.newProperties["Members"].Value.([]map[string]RedfishResourceProperty))
	r.newProperties["Members@odata.count"] = RedfishResourceProperty{Value: l}
}

type PropertyRefresher interface {
	RefreshProperty(context.Context, *RedfishResourceAggregate, *RedfishResourceProperty, string, map[string]interface{}, interface{})
}

func (rrp *RedfishResourceProperty) Process(ctx context.Context, agg *RedfishResourceAggregate, property, method string, req interface{}) (ret RedfishResourceProperty) {
	// step 1: run the plugin to update rrp.Value based on the plugin.
	// Step 2: see if the rrp.Value is a recursable map or array and recurse down it

    // set up return copy. We are not going to modify our source
    ret = RedfishResourceProperty{ Meta: map[string]interface{}{} }
    for k, v := range rrp.Meta {
        ret.Meta[k] = v
    }
    ret.Value = rrp.Value

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
			fmt.Printf("ORPHAN PROPERTY(%s) - could not load plugin(%s): %s\n", property, pluginName, err.Error())
			break
		}

		if plugin, ok := plugin.(PropertyRefresher); ok {
			fmt.Printf("PROCESS PROPERTY(%s) with plugin(%s)\n", property, pluginName)
			plugin.RefreshProperty(ctx, agg, &ret, method, meta_t, req)
		}
	}

	switch t := ret.Value.(type) {
	case map[string]RedfishResourceProperty:
		type result struct {
			name   string
			result RedfishResourceProperty
		}
		reqmap, _ := req.(map[string]interface{})
		var promised []chan result
		for property, v := range t {
			resChan := make(chan result)
			promised = append(promised, resChan)

			go func(property string, v RedfishResourceProperty) {
				reqitem, _ := reqmap[property]
				retProp := v.Process(ctx, agg, property, method, reqitem)
				resChan <- result{property, retProp}
			}(property, v)
		}
        newMap := map[string]RedfishResourceProperty{}
		for _, resChan := range promised {
			res := <-resChan
			newMap[res.name] = res.result
		}
        ret.Value = newMap

	case []RedfishResourceProperty:
		// spawn off parallel goroutines to process each member of the array
		reqarr, _ := req.([]interface{})
		var promised []chan RedfishResourceProperty
		for index, v := range t {
			resChan := make(chan RedfishResourceProperty)
			promised = append(promised, resChan)
			go func(index int, v RedfishResourceProperty) {
				var reqitem interface{} = nil
				if index < len(reqarr) {
					reqitem = reqarr[index]
				}
				retProp := v.Process(ctx, agg, property, method, reqitem)
				resChan <- retProp
			}(index, v)
		}
		newArr := []RedfishResourceProperty{}
		for _, resChan := range promised {
			res := <-resChan
			newArr = append(newArr, res)
		}
        ret.Value = newArr
	default:
	}

    return
}

func (agg *RedfishResourceAggregate) ProcessMeta(ctx context.Context, method string, request map[string]interface{}) (results map[string]interface{}, err error) {
	agg.newPropertiesMu.Lock()
	defer agg.newPropertiesMu.Unlock()

    results = map[string]interface{}{}
    err = nil

	type result struct {
		name   string
		result RedfishResourceProperty
	}

	var promised []chan result
	for property, v := range agg.newProperties {
		resChan := make(chan result)
		promised = append(promised, resChan)
		go func(property string, v RedfishResourceProperty) {
			req, _ := request[property]
			ret := v.Process(ctx, agg, property, method, req)
			resChan <- result{name: property, result: ret}
		}(property, v)
	}

	// after everything has settled, copy it out (still under lock)
	for _, resChan := range promised {
		res := <-resChan
		agg.newProperties[res.name] = res.result
		results[res.name] = res.result
	}

	return
}
