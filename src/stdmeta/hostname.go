package stdmeta

import (
	"context"
	"os"

	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

var (
	HostnamePlugin = domain.PluginType("hostname")
)

func init() {
	domain.RegisterPlugin(func() domain.Plugin { return &hostname{} })
}

type hostname struct{}

func (t *hostname) PluginType() domain.PluginType { return HostnamePlugin }

func (t *hostname) RefreshProperty(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	method string,
	meta map[string]interface{},
	body interface{},
) {
	hostname, err := os.Hostname()
	if err == nil {
		rrp.Value = hostname
	}
}
