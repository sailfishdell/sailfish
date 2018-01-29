package domain

import (
	"context"
	"fmt"
	"sync"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

const AggregateType = eh.AggregateType("RedfishResource")

func init() {
	RegisterInitFN(RegisterRRA)
}

func RegisterRRA(ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	eh.RegisterAggregate(func(id eh.UUID) eh.Aggregate {
		return &RedfishResourceAggregate{eventBus: eb}
	})
}

type RedfishResourceAggregate struct {
	// private
	eventBus eh.EventBus

	// public
	ID          eh.UUID
	ResourceURI string
	Plugin      string

	propertiesMu sync.RWMutex
	properties   map[string]interface{}

	// "prop": {"method": { "plugin": "foo", "args": "bar"}}
	propertyPluginMu sync.RWMutex
	propertyPlugin   map[string]map[string]map[string]interface{}

	// TODO: need accessor functions for all of these just like property stuff
	// above so that everything can be properly locked
	PrivilegeMap map[string]interface{}
	Permissions  map[string]interface{}
	Headers      map[string]string
	Private      map[string]interface{}
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

func (r *RedfishResourceAggregate) InitProperties() {
	r.properties = map[string]interface{}{}
}

func (r *RedfishResourceAggregate) HasProperty(p string) bool {
	r.propertiesMu.RLock()
	defer r.propertiesMu.RUnlock()
	_, ok := r.properties[p]
	return ok
}

func (r *RedfishResourceAggregate) GetProperty(p string) interface{} {
	r.propertiesMu.RLock()
	defer r.propertiesMu.RUnlock()
	return r.properties[p]
}

func (r *RedfishResourceAggregate) SetProperty(p string, v interface{}) {
	r.propertiesMu.Lock()
	defer r.propertiesMu.Unlock()
	r.properties[p] = v
}

func (r *RedfishResourceAggregate) DeleteProperty(p string) {
	r.propertiesMu.Lock()
	defer r.propertiesMu.Unlock()
	delete(r.properties, p)
}

// MutateProperty will run the function over Properties holding the properties RW lock
func (r *RedfishResourceAggregate) MutateProperty(mut func(map[string]interface{})) {
	r.propertiesMu.Lock()
	defer r.propertiesMu.Unlock()
	mut(r.properties)
}

func (r *RedfishResourceAggregate) RangeProperty(fn func(string, interface{})) {
	r.propertiesMu.RLock()
	defer r.propertiesMu.RUnlock()
	for k, v := range r.properties {
		fn(k, v)
	}
}

func (r *RedfishResourceAggregate) GetPropertyPlugin(p string, m string) (ret map[string]interface{}) {
	r.propertyPluginMu.RLock()
	defer r.propertyPluginMu.RUnlock()
	// propertyPlugin   map[string]map[string]map[string]interface{}
	v, ok := r.propertyPlugin[p]
	// v map[string]map[string]interface{}
	if ok {
		ret = v[m]
		// ret  map[string]interface{}
	}
	return
}

func (agg *RedfishResourceAggregate) ProcessMeta(ctx context.Context, method string) error {
	var wg sync.WaitGroup
	fmt.Printf("PROCESS META: %T\n", agg.propertyPlugin)
	agg.propertyPluginMu.RLock()
	defer agg.propertyPluginMu.RUnlock()
	for name, v := range agg.propertyPlugin {
		fmt.Printf("\tname(%s) = %s\n", name, v)

		get, ok := v[method]
		if !ok {
			continue
		}

		fmt.Printf("\tget: %s\n", get)
		plugin, ok := get["plugin"]
		if !ok {
			continue
		}

		pluginName, ok := plugin.(string)
		if !ok {
			continue
		}

		fmt.Printf("\tplugin: %s\n", pluginName)
		p, err := InstantiatePlugin(PluginType(pluginName))

		if err != nil {
			fmt.Printf("bummer, plugin(%s) not found: %s\n", plugins, err.Error())
			continue
		}

		// AggregatePlugin has free range to operate on the entire aggregate
		if aggPlug, ok := p.(AggregatePlugin); ok {
			// run all of the aggregate updates in parallel
			wg.Add(1)
			go aggPlug.UpdateAggregate(ctx, agg, &wg, name, method)
		}

		// TODO: make a streamable plugin that operates on a single property
	}
	wg.Wait()

	return nil
}
