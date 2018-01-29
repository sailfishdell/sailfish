package test

import (
	"context"
	"fmt"
	domain "github.com/superchalupa/go-redfish/redfishresource"
	"sync"
	"time"
)

/*
   This is an example plugin that implements "strategy 3": it waits until a
   GET request is called and then pretends to make a backend call to fill in
   data. We add a small latency to prove that all the calls happen in
   parallel.
*/

var (
	TestPlugin_Strategy3 = domain.PluginType("test:strategy3")
)

func init() {
	domain.RegisterPlugin(func() domain.Plugin { return &testplugin_strategy3{} })
}

type testplugin_strategy3 struct{}

func (t *testplugin_strategy3) PluginType() domain.PluginType { return TestPlugin_Strategy3 }

func (t *testplugin_strategy3) UpdateAggregate(ctx context.Context, a *domain.RedfishResourceAggregate, wg *sync.WaitGroup, property string, method string) {
	fmt.Printf("UPDATE AGGREGATE: %s\n", property)
	defer wg.Done()

	plugin := a.GetPropertyPlugin(property, method)
	time.Sleep(1 * time.Second)

	a.SetProperty(property, fmt.Sprintf("method(%s)  time(%s) args(%s)", method, time.Now(), plugin["args"]))
}
