package event

import (
	"fmt"
	"github.com/Knetic/govaluate"
	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/go-redfish/src/log"
)

func Filter(logger log.Logger, expr string) (func(eh.Event) bool, error) {
	functions := map[string]govaluate.ExpressionFunction{
		"string": func(args ...interface{}) (interface{}, error) {
			return fmt.Sprint(args[0]), nil
		},
	}

	expression, err := govaluate.NewEvaluableExpressionWithFunctions(expr, functions)
	if err != nil {
		logger.Crit("Expression construction (lexing) failed.", "expression", expr)
		return nil, err
	}

	return func(ev eh.Event) bool {
		parameters := map[string]interface{}{
			"type":  string(ev.EventType()),
			"data":  ev.Data(),
			"event": ev,
		}
		result, err := expression.Evaluate(parameters)
		if err == nil {
			if ret, ok := result.(bool); ok {
				return ret
			}
			// LOG ERRROR: expression didn't return BOOL
			logger.Error("Expression did not return a bool.", "expression", expr, "parsed", expression.String())
		}
		// LOG ERRROR: expression evaluation failed
		logger.Crit("Expression evaluation failed.", "expression", expr, "parsed", expression.String())
		return false
	}, nil
}
