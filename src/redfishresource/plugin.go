package domain

import (
	"context"
	"errors"
	"fmt"
	"sync"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
)

var needsInit []func(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter)
var needsInitMu sync.Mutex

func InitDomain(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	for _, f := range needsInit {
		f(ctx, ch, eb, ew)
	}
}

// use this function in any plugin to register that it needs to be initialized with domain objects
func RegisterInitFN(fn func(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter)) {
	needsInitMu.Lock()
	defer needsInitMu.Unlock()
	needsInit = append(needsInit, fn)
}

// Dynamic plugins that help fill out aggregates during request processing
type PluginType string

type Plugin interface {
	PluginType() PluginType
}

type AggregatePlugin interface {
	UpdateAggregate(context.Context, *RedfishResourceAggregate, *sync.WaitGroup, string, string)
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
