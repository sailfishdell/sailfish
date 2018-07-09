package awesome_mapper

import (
    "context"
    "errors"

	"github.com/spf13/viper"
    "github.com/Knetic/govaluate"

	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/event"
	"github.com/superchalupa/go-redfish/src/ocp/model"
)

type Evaluable interface {
    Evaluate(map[string]interface{}) (interface{}, error)
}

type mapping struct {
	Property   string
	Query string
    expr  Evaluable
}

type MappingEntry struct {
    Select      string
    ModelUpdate []*mapping
}

func New(ctx context.Context, logger log.Logger, cfg *viper.Viper, mdl *model.Model, name string, parameters map[string]interface{}) (error) {
	c := []MappingEntry{}

	k := cfg.Sub("mappings")
	if k == nil {
		logger.Warn("missing config file section: 'mappings'")
		return errors.New("Missing config section 'mappings'")
	}
	err := k.UnmarshalKey(name, &c)
	if err != nil {
		logger.Warn("unmarshal failed", "err", err)
	}
	logger.Warn("updated mappings", "mappings", c)

    functions := map[string]govaluate.ExpressionFunction{}

outer:
    for _, entry := range c {
        loopvar := entry
        for _, query := range loopvar.ModelUpdate {
            query.expr, err = govaluate.NewEvaluableExpressionWithFunctions(query.Query, functions)
            if err != nil {
                logger.Crit("Query construction failed", "query", query.Query)
                continue  outer
            }
        }

        // stream processor for action events
        sp, err := event.NewESP(ctx, event.ExpressionFilter(logger, loopvar.Select, parameters, functions))
        if err != nil {
            logger.Error("Failed to create event stream processor", "err", err, "select-string", loopvar.Select)
            continue
        }

        sp.RunForever(func(event eh.Event) {
            logger.Crit("GOT AN EVENT", "event", event)
            for _, query := range loopvar.ModelUpdate {
                if query.expr == nil {
                    logger.Error("query is nil", "loopvar", loopvar)
                    continue
                }
                val, err := query.expr.Evaluate(parameters)
                if err != nil {
                    logger.Error("Expression failed to evaluate", "query.expr", query.expr, "parameters", parameters)
                    continue
                }
                mdl.UpdateProperty(query.Property, val)
            }
        })
    }

	return nil
}
