package stdmeta

import (
	"context"
	eh "github.com/looplab/eventhorizon"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
	"strings"
)

var (
	GenericPlugin = domain.PluginType("Generic")
)

func GenericDefPlugin(ch eh.CommandHandler, d *domain.DomainObjects) {
	domain.RegisterPlugin(func() domain.Plugin { return &Generic{ch: ch, d: d} })
}

type Generic struct {
	d  *domain.DomainObjects
	ch eh.CommandHandler
}

func (s *Generic) PluginType() domain.PluginType { return GenericPlugin }

// run per patch, can't run altogether.
func (s *Generic) PropertyPatch(
	ctx context.Context,
	resp map[string]interface{},
	agg *domain.RedfishResourceAggregate,
	auth *domain.RedfishAuthorizationProperty,
	rrp *domain.RedfishResourceProperty,
	encopts *domain.NuEncOpts,
	meta map[string]interface{},
) (error) {
	pathMapStr := map[string]interface{}{}
	domain.Map2Path(encopts.Request, pathMapStr, "")
	for pathStr, value := range pathMapStr {
		pathSlice := strings.Split(pathStr, "/")
		domain.UpdateAgg(agg, pathSlice, value)
	}
	return nil
}
