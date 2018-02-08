package test

import (
	"context"
	"fmt"
	"time"

	domain "github.com/superchalupa/go-redfish/redfishresource"
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

//     RefreshProperty(context.Context, *RedfishResourceAggregate, *RedfishResourceProperty, string, map[string]interface{}, interface{})
func (t *testplugin_strategy3) RefreshProperty(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	method string,
	meta map[string]interface{},
	body interface{},
) {
	time.Sleep(1 * time.Second)
	rrp.Value = fmt.Sprintf("time(%s) args(%s)", time.Now(), meta["args"])
}
