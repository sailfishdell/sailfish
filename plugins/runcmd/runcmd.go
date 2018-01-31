package runcmd

import (
	"bytes"
	"os/exec"
	"context"
	"fmt"
	"sync"

	domain "github.com/superchalupa/go-redfish/redfishresource"
)

var (
	RunCmdPlugin = domain.PluginType("runcmd")
)

func init() {
	domain.RegisterPlugin(func() domain.Plugin { return &runCmd{} })
}

type runCmd struct{}

func (t *runCmd) PluginType() domain.PluginType { return RunCmdPlugin }


func (t *runCmd) UpdateValue(ctx context.Context, wg *sync.WaitGroup, agg *domain.RedfishResourceAggregate, property string, rrp *domain.RedfishResourceProperty, meta map[string]interface{}) {
	fmt.Printf("UPDATE AGGREGATE: %s  (Old: %s)\n", property, rrp.Value)
	defer wg.Done()

    cmd, ok := meta["CMD"].(string)
    if !ok {
        fmt.Printf("Misconfigured runcmd plugin: CMD not set\n")
        return
    }

    // convert args to something we can pass to command
    var args []string
    rawargs, ok := meta["CMDARGS"]
    if ok {
        for _, rs := range rawargs.([]interface{}) {
            s, ok := rs.(string)
            if ok {
                // TODO: search and replace args with properties. For example %{name} should be replace with the name property
                args = append(args, s)
            }
        }
    }

    out, err := exec.CommandContext(ctx, cmd, args...).Output()
    if err != nil {
        fmt.Printf("Command execution failure: %s\n", err.Error())
        return
    }

	fmt.Printf("Ran command (%s) with args (%s) and got output = %s\n", cmd, args, out)
    rrp.Value = fmt.Sprintf("%s", bytes.TrimSpace(out))
}
