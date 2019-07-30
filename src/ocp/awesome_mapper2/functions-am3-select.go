package awesome_mapper2

import (
	"sync"

	"github.com/Knetic/govaluate"
	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/sailfish/src/log"
)

type setupSelectFunc func(log.Logger, ConfigFileMappingEntry) (selectFunc, error)
type selectFunc func(p *MapperParameters) (bool, error)

var setupSelectFuncsInit sync.Once
var setupSelectFuncsMu sync.RWMutex
var setupSelectFuncs map[string]setupSelectFunc

func InitAM3SelectSetupFunctions() (map[string]setupSelectFunc, *sync.RWMutex) {
	setupSelectFuncsInit.Do(func() { setupSelectFuncs = map[string]setupSelectFunc{} })
	return setupSelectFuncs, &setupSelectFuncsMu
}

func AddAM3SelectSetupFunction(name string, fn setupSelectFunc) {
	InitAM3SelectSetupFunctions()
	setupSelectFuncsMu.Lock()
	setupSelectFuncs[name] = fn
	setupSelectFuncsMu.Unlock()
}

func init() {
	InitAM3SelectSetupFunctions()

	// debugging function
	AddAM3SelectSetupFunction("true", func(l log.Logger, c ConfigFileMappingEntry) (selectFunc, error) {
		// comment for how to use SelectParams in a function
		// remove this comment whenever we get another user for it that can be used as an example
		//fmt.Printf("\n\nSETTING UP TRUE: %s\n\n", c.SelectParams)
		return func(*MapperParameters) (bool, error) {
			//fmt.Printf("TRUE!\n")
			return true, nil
		}, nil
	})

	AddAM3SelectSetupFunction("govaluate", func(logger log.Logger, cfgEntry ConfigFileMappingEntry) (selectFunc, error) {
		selectExpr, err := govaluate.NewEvaluableExpressionWithFunctions(cfgEntry.Select, functions)
		// parse the expression
		if err != nil {
			logger.Crit("Select construction failed", "select", cfgEntry.Select, "err", err)
			return nil, err
		}
		return func(parameters *MapperParameters) (bool, error) {
			val, err := selectExpr.Evaluate(parameters.params)
			if err != nil {
				logger.Error("expression failed to evaluate", "err", err, "select", cfgEntry.Select)
				return false, err
			}

			valBool, ok := val.(bool)
			if !ok {
				logger.Info("NOT A BOOL", "type", parameters.params["event"].(eh.Event).EventType(), "select", cfgEntry.Select, "val", val)
				return false, err
			}
			return valBool, err
		}, nil
	})

}
