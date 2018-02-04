package patch

import (
	"context"

	domain "github.com/superchalupa/go-redfish/redfishresource"
)

var (
	PatchPlugin = domain.PluginType("patch")
)

func init() {
	domain.RegisterPlugin(func() domain.Plugin { return &patch{} })
}

type patch struct{}

func (t *patch) PluginType() domain.PluginType { return PatchPlugin }

func (t *patch) DemandBasedUpdate(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	method string,
	meta map[string]interface{},
	body interface{},
) {
	// wow, how can it be this simple?
	// ... we need to add a way to add validation... so I guess it can't stay this simple for long.
	rrp.Value = body
}
