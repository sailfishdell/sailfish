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
	auth *domain.RedfishAuthorizationProperty,
	rrp *domain.RedfishResourceProperty,
	meta map[string]interface{},
) error {
	hostname, err := os.Hostname()
	if err == nil {
		rrp.Value = hostname
	}
	return err
}
