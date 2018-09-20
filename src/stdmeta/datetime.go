package stdmeta

import (
	"context"
	"time"

	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

var (
	dateTimePlugin = domain.PluginType("datetime")
)

func init() {
	domain.RegisterPlugin(func() domain.Plugin { return &dateTime{} })
}

type dateTime struct{}

func (t *dateTime) PluginType() domain.PluginType { return dateTimePlugin }

func (t *dateTime) PropertyGet(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	rrp *domain.RedfishResourceProperty,
	meta map[string]interface{},
) {
	// TODO: do we need to add options here to format different ways? Do we need to be able to format specific times instead of just current time?

	// time.RFC3339
	//rrp.Value = fmt.Sprintf(time.Now().UTC().Format("2006-01-02T15:04:05Z07:00"))
	rrp.Value = time.Now().UTC().Format("2006-01-02T15:04:05-07:00")
}
