package test

import (
	"context"

	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

/*
   This is an example plugin that implements "strategy 3": it waits until a
   GET request is called and dynamically fills in the data for the full property
*/

var (
	TestPlugin_FullProperty = domain.PluginType("test:fullProperty")
)

func init() {
	domain.RegisterPlugin(func() domain.Plugin { return &testplugin_full_property{} })
}

type testplugin_full_property struct{}

func (t *testplugin_full_property) PluginType() domain.PluginType { return TestPlugin_FullProperty }

func (t *testplugin_full_property) RefreshProperty(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	method string,
	meta map[string]interface{},
	body interface{},
) {
	v, ok := rrp.Value.(map[string]interface{})
	if !ok {
		return
	}

	v["ITS"] = "ALIVE!"
	delete(v, "deleteme")
}
