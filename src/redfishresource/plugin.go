package domain

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

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

func UnregisterPlugin(pluginType PluginType) {
	pluginsMu.Lock()
	defer pluginsMu.Unlock()
	delete(plugins, pluginType)
}
