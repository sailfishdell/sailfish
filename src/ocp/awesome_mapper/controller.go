package awesome_mapper

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"time"

	"github.com/Knetic/govaluate"
	"github.com/spf13/viper"

	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/event"
	"github.com/superchalupa/sailfish/src/ocp/model"
)

type Evaluable interface {
	Evaluate(map[string]interface{}) (interface{}, error)
}

type mapping struct {
	Property string
	Query    string
	expr     []govaluate.ExpressionToken
	Default  interface{}
}

type MappingEntry struct {
	Select      string
	ModelUpdate []*mapping
}

// TODO: need to implement a Close() method to clean everything up tidily

func New(ctx context.Context, logger log.Logger, cfg *viper.Viper, mdl *model.Model, name string, parameters map[string]interface{}) error {
	c := []MappingEntry{}

	k := cfg.Sub("awesome_mapper")
	if k == nil {
		logger.Warn("missing config file section: 'awesome_mapper'")
		return errors.New("Missing config section 'awesome_mapper'")
	}
	err := k.UnmarshalKey(name, &c)
	if err != nil {
		logger.Warn("unmarshal failed", "err", err)
	}
	logger.Info("updated mappings", "mappings", c)

	functions := map[string]govaluate.ExpressionFunction{
		"int": func(args ...interface{}) (interface{}, error) {
			switch t := args[0].(type) {
			case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, uintptr:
				return float64(reflect.ValueOf(t).Int()), nil
			case float32, float64:
				return float64(reflect.ValueOf(t).Float()), nil
			case string:
				float, err := strconv.ParseFloat(t, 64)
				return float, err
			default:
				return nil, errors.New("Can't parse non-string")
			}
		},
		"strlen": func(args ...interface{}) (interface{}, error) {
			length := len(args[0].(string))
			return (float64)(length), nil
		},
		"epoch_to_date": func(args ...interface{}) (interface{}, error) {
			return time.Unix(int64(args[0].(float64)), 0), nil
		},
    "string": func(args ...interface{}) (interface{}, error) {
      switch t := args[0].(type) {
      case int, int8, int16, int32, int64:
        str := strconv.FormatInt(reflect.ValueOf(t).Int(), 10)
        return str, nil
      case uint, uint8, uint16, uint32, uint64:
        str := strconv.FormatUint(reflect.ValueOf(t).Uint(), 10)
        return str, nil
      case float32, float64:
        str := strconv.FormatFloat(reflect.ValueOf(t).Float(), 'G', -1, 64)
        return str, nil
      case string:
        return t, nil
      default:
        return nil, errors.New("Not an int, float, or string")
      }
    },
		"map_health_value": func(args ...interface{}) (interface{}, error) {
			switch t := args[0].(float64); t {
			case 0, 1: //other, unknown
				return nil, nil
			case 2: //ok
				return "OK", nil
			case 3: //non-critical
				return "Warning", nil
			case 4, 5: //critical, non-recoverable
				return "Critical", nil
			default:
				return nil, errors.New("Invalid object status")
			}
		},
	}

outer:
	for _, entry := range c {
		loopvar := entry
		mdl.StopNotifications()
		for _, query := range loopvar.ModelUpdate {
			expr, err := govaluate.NewEvaluableExpressionWithFunctions(query.Query, functions)
			if err != nil {
				logger.Crit("Query construction failed", "query", query.Query, "err", err)
				continue outer
			}
			query.expr = expr.Tokens()
			if query.Default != nil {
				mdl.UpdateProperty(query.Property, query.Default)
			}
		}
		mdl.StartNotifications()
		mdl.NotifyObservers()

		// stream processor for action events
		sp, err := event.NewESP(ctx, event.ExpressionFilter(logger, loopvar.Select, parameters, functions), event.SetListenerName("awesome_mapper"))
		if err != nil {
			logger.Error("Failed to create event stream processor", "err", err, "select-string", loopvar.Select)
			continue
		}

		expressionParameters := map[string]interface{}{}
		for k, v := range parameters {
			expressionParameters[k] = v
		}

		go sp.RunForever(func(event eh.Event) {
				mdl.StopNotifications()
				for _, query := range loopvar.ModelUpdate {
					if query.expr == nil {
						logger.Crit("query is nil, that can't happen", "loopvar", loopvar)
						continue
					}

					expressionParameters["type"] = string(event.EventType())
					expressionParameters["data"] = event.Data()
					expressionParameters["event"] = event

					expr, err := govaluate.NewEvaluableExpressionFromTokens(query.expr)
					val, err := expr.Evaluate(expressionParameters)
					if err != nil {
						logger.Error("Expression failed to evaluate", "query.Query", query.Query, "parameters", expressionParameters, "err", err)
						continue
					}
					mdl.UpdateProperty(query.Property, val)
				}
				mdl.StartNotifications()
				mdl.NotifyObservers()
		})
	}

	return nil
}
