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
	rrp *domain.RedfishResourceProperty,
	meta map[string]interface{},
) error {
	// TODO: do we need to add options here to format different ways? Do we need to be able to format specific times instead of just current time?
	// could do that with additional meta options...

	//odatalite format: 2000-08-12T18:27:01+00:00
	//rrp.Value = time.Now().UTC().Format(time.RFC3339) //2018-09-26T20:25:29Z
	rrp.Value = time.Now().UTC().Format("2006-01-02T15:04:05-07:00") //2018-09-26T20:26:05+00:00
	return nil
}
