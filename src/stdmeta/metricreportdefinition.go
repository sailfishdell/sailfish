package stdmeta

import (
	"context"
	eh "github.com/looplab/eventhorizon"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
	"strings"
)

var (
	MRDPlugin = domain.PluginType("MRD")
)

func MetricReportDefPlugin(ch eh.CommandHandler, d *domain.DomainObjects) {
	domain.RegisterPlugin(func() domain.Plugin { return &MRD{ch: ch, d: d} })
}

type MRD struct {
	d  *domain.DomainObjects
	ch eh.CommandHandler
}

func (s *MRD) PluginType() domain.PluginType { return MRDPlugin }

// run per patch, can't run altogether.
func (s *MRD) PropertyPatch(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	auth *domain.RedfishAuthorizationProperty,
	rrp *domain.RedfishResourceProperty,
	encopts interface{}, // returns patch json body
	meta map[string]interface{},
) error {
	pathMapStr := map[string]interface{}{}
	domain.Map2Path(encopts, pathMapStr, "")
	for pathStr, value := range pathMapStr {
		pathSlice := strings.Split(pathStr, "/")
		domain.UpdateAgg(agg, pathSlice, value)
	}
	return nil
}
