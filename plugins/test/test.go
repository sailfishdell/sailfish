package test

/* 
    STRATEGY 1:
    -- SAGA A: listen for meta updated events. maintain a list of matching plugin lists.
    -- SAGA B: listen to events and emit commands - for events that are relevant to the plugin, traverse list of aggregates affected and emit commands to update the relevant aggregates


    STRATEGY 2:
    -- SAGA A: listen for events of interest and directly update the aggregate that you know should be updated (cmd: updateredfishresourceproperties)


    STRATEGY 3:
    -- Output plugin: grabs data when we ask to output it (least efficient way: discourage this for the most part.)

    I think for many things it would be best if we tended towards strategy 1, because we could write more generic code
*/

import (
    "fmt"
    domain "github.com/superchalupa/go-redfish/redfishresource"
)

var (
    TestPlugin = domain.PluginType("test:plugin")
)

func init() {
    domain.RegisterPlugin( func() domain.Plugin { return &testplugin{} } )
}

type testplugin struct {
}

func (t *testplugin) PluginType() domain.PluginType { return TestPlugin }

func (t *testplugin) UpdateAggregate(a *domain.RedfishResourceAggregate) <-chan error {
    fmt.Printf("\n UPDATE AGGREGATE \n")
    foo := make(chan error)
    go func() {
        foo <- nil
        close(foo)
    }()
    return foo
}
