package stdmeta

import (
	"context"
	"os"

	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

var (
	hostnamePlugin = domain.PluginType("hostname")
)

func init() {
	domain.RegisterPlugin(func() domain.Plugin { return &hostname{} })
}

type hostname struct{}

func (t *hostname) PluginType() domain.PluginType { return hostnamePlugin }

func (t *hostname) PropertyGet(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	meta map[string]interface{},
) {
	hostname, err := os.Hostname()
	if err == nil {
		rrp.Value = hostname
	}
}
