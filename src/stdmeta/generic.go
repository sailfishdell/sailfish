package stdmeta

import (
	"context"
	eh "github.com/looplab/eventhorizon"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
	"strings"
)

var (
	GenericPlugin     = domain.PluginType("Generic")
	GenericBoolPlugin = domain.PluginType("GenericBool")
)

func GenericDefPlugin(ch eh.CommandHandler, d *domain.DomainObjects) {
	domain.RegisterPlugin(func() domain.Plugin { return &Generic{ch: ch, d: d} })
	domain.RegisterPlugin(func() domain.Plugin { return &GenericBool{ch: ch, d: d} })
}

type Generic struct {
	d  *domain.DomainObjects
	ch eh.CommandHandler
}

func (s *Generic) PluginType() domain.PluginType { return GenericPlugin }

// run per patch, can't run altogether.
func (s *Generic) PropertyPatch(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	auth *domain.RedfishAuthorizationProperty,
	rrp *domain.RedfishResourceProperty,
	encopts *domain.NuEncOpts,
	meta map[string]interface{},
) error {
	pathMapStr := map[string]interface{}{}
	domain.Map2Path(encopts.Request, pathMapStr, "")
	for pathStr, value := range pathMapStr {
		pathSlice := strings.Split(pathStr, "/")
		domain.UpdateAgg(agg, pathSlice, value, 0)
	}
	return nil
}

type GenericBool struct {
	d  *domain.DomainObjects
	ch eh.CommandHandler
}

func (s *GenericBool) PluginType() domain.PluginType { return GenericBoolPlugin }

// run per patch, can't run altogether.
func (s *GenericBool) PropertyPatch(
	ctx context.Context,
	agg *domain.RedfishResourceAggregate,
	auth *domain.RedfishAuthorizationProperty,
	rrp *domain.RedfishResourceProperty,
	encopts *domain.NuEncOpts,
	meta map[string]interface{},
) error {
	pathMapStr := map[string]interface{}{}
	domain.Map2Path(encopts.Request, pathMapStr, "")

	//TODO: ILLEGAL_RECURSIVE_COMMAND
	s.ch.HandleCommand(ctx,
		&domain.UpdateRedfishResourceProperties2{
			ID:         agg.ID,
			Properties: pathMapStr,
		})

	return nil
}
