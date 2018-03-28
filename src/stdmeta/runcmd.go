package stdmeta

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

var (
	RunCmdPlugin = domain.PluginType("runcmd")
)

func init() {
	domain.RegisterPlugin(func() domain.Plugin { return &runCmd{} })
}

type runCmd struct{}

func (t *runCmd) PluginType() domain.PluginType { return RunCmdPlugin }

func (t *runCmd) PropertyGet(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	method string,
	meta map[string]interface{},
) {

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
