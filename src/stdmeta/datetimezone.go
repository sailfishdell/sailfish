package stdmeta

import (
	"context"
	"time"

	domain "github.com/superchalupa/go-redfish/src/redfishresource"
)

var (
	dateTimeZonePlugin = domain.PluginType("datetimezone")
)

func init() {
	domain.RegisterPlugin(func() domain.Plugin { return &dateTimeZone{} })
}

type dateTimeZone struct{}

func (t *dateTimeZone) PluginType() domain.PluginType { return dateTimeZonePlugin }

func (t *dateTimeZone) PropertyGet(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	meta map[string]interface{},
) {
	rrp.Value = time.Now().UTC().Format("-07:00")
}
