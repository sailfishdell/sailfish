package domain

import (
	"context"
	"fmt"
	"strings"
	"sync"
    "errors"

	eh "github.com/looplab/eventhorizon"
)

const AggregateType = eh.AggregateType("RedfishResource")

func RegisterRRA(eb eh.EventBus) {
	eh.RegisterAggregate(func(id eh.UUID) eh.Aggregate {
		return &RedfishResourceAggregate{eventBus: eb}
	})
}

type RedfishResourceAggregate struct {
	// private
	eventBus eh.EventBus

	// public
	ID           eh.UUID
	TreeID       eh.UUID
	ResourceURI  string
	Plugin       string
	Properties   map[string]interface{}
    PropertyPlugin map[string]interface{}
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

func (a *RedfishResourceAggregate) ProcessMeta(ctx context.Context) error {
	//    wg sync.WaitGroup
	for k, v := range a.Properties {
		if !strings.HasSuffix(k, "@meta") {
			continue
		}

		//        a.Properties[ k[:len(k)-5] ] =  InstantiatePlugin("test")
		_ = v
	}

	return nil
}

// Type of plugin to register
type PluginType string

type Plugin interface {
	UpdateAggregate(a *RedfishResourceAggregate) <-chan error
	PluginType() PluginType
}

var plugins = make(map[PluginType]func() Plugin)
var pluginsMu sync.RWMutex

// RegisterPlugin registers an plugin factory for a type. The factory is
// used to create concrete plugin types.
//
// An example would be:
//     RegisterPlugin(func() Plugin { return &MyPlugin{} })
func RegisterPlugin(factory func() Plugin) {
	plugin := factory()
	pluginType := plugin.PluginType()

	pluginsMu.Lock()
	defer pluginsMu.Unlock()
	if _, ok := plugins[pluginType]; ok {
		panic(fmt.Sprintf("eventhorizon: registering duplicate types for %q", pluginType))
	}
	plugins[pluginType] = factory
}

func InstantiatePlugin(pluginType PluginType) (Plugin, error) {
	pluginsMu.RLock()
	defer pluginsMu.RUnlock()
	if factory, ok := plugins[pluginType]; ok {
		return factory(), nil
	}
	return nil, errors.New("Plugin Type not registered")
}
