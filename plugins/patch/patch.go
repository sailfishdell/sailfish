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
	rrp.Value = body
}
