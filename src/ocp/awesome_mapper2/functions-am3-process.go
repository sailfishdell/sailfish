package awesome_mapper2

import (
	"errors"
	"sync"

	"github.com/Knetic/govaluate"

	"github.com/superchalupa/sailfish/src/log"
)

type setupProcessFunc func(log.Logger, interface{}) (processFunc, processSetupFunc, error)

type processFunc func(p *MapperParameters) error
type processSetupFunc func(p *MapperParameters) error

var setupProcessFuncsInit sync.Once
var setupProcessFuncsMu sync.RWMutex
var setupProcessFuncs map[string]setupProcessFunc

func InitAM3ProcessSetupFunctions() (map[string]setupProcessFunc, *sync.RWMutex) {
	setupProcessFuncsInit.Do(func() { setupProcessFuncs = map[string]setupProcessFunc{} })
	return setupProcessFuncs, &setupProcessFuncsMu
}

func AddAM3ProcessSetupFunction(name string, fn setupProcessFunc) {
	InitAM3ProcessSetupFunctions()
	setupProcessFuncsMu.Lock()
	setupProcessFuncs[name] = fn
	setupProcessFuncsMu.Unlock()
}

type ModelUpdate struct {
	property    string
	queryString string
	queryExpr   *govaluate.EvaluableExpression
	defaultVal  interface{}
}

func init() {
	AddAM3ProcessSetupFunction("govaluate_modelupdate", func(logger log.Logger, modelUpdates interface{}) (processFunc, processSetupFunc, error) {
		mus, ok := modelUpdates.([]*ConfigFileModelUpdate)
		if !ok {
			return nil, nil, errors.New("govaluate_modelupdate is missing []*ConfigFileModelUpdate parameter")

		}

		// initial parsing and setup
		m := []*ModelUpdate{}
		for _, modelUpdate := range mus {
			// this queryExpr would be handled the same as a exec expr
			queryExpr, err := govaluate.NewEvaluableExpressionWithFunctions(modelUpdate.Query, functions)
			if err != nil {
				logger.Crit("Query construction failed", "query", modelUpdate.Query, "err", err)
				continue
			}

			m = append(m, &ModelUpdate{
				property:    modelUpdate.Property,
				queryString: modelUpdate.Query,
				queryExpr:   queryExpr,
				defaultVal:  modelUpdate.Default,
			})
		}

		// Model Update Function
		modelUpdateFn := func(parameters *MapperParameters) error {

			for _, updates := range m {
				parameters.model.StopNotifications()
				// Note: LIFO order for defer
				defer parameters.model.NotifyObservers()
				defer parameters.model.StartNotifications()

				parameters.Params["propname"] = updates.property
				val, err := updates.queryExpr.Evaluate(parameters.Params)
				if err != nil {
					logger.Error("Expression failed to evaluate", "err", err, "parameters", parameters.Params, "val", val)
					continue
				}
				// comment out logging in the fast path. uncomment to enable.
				//ret.logger.Info("Updating property!", "property", updates.property, "value", val, "Event", event, "EventData", event.Data())
				parameters.model.UpdateProperty(updates.property, val)
			}

			delete(parameters.Params, "propname")
			return nil
		}

		modelDefaultSetupFn := func(parameters *MapperParameters) error {

			if parameters.model == nil {
				return errors.New("parameters model is nil")
			}

			parameters.model.StopNotifications()
			defer parameters.model.NotifyObservers()
			defer parameters.model.StartNotifications()

			for _, mapperUpdate := range m {
				if mapperUpdate.defaultVal != nil {
					parameters.model.UpdateProperty(mapperUpdate.property, mapperUpdate.defaultVal)
				} else {
					parameters.model.UpdateProperty(mapperUpdate.property, "")
				}
			}
			return nil
		}
		return modelUpdateFn, modelDefaultSetupFn, nil

	})
	AddAM3ProcessSetupFunction("govaluate_exec", func(logger log.Logger, execs interface{}) (processFunc, processSetupFunc, error) {
		// initial parsing and setup
		es, ok := execs.([]string)
		if !ok {
			return nil, nil, errors.New("govaluate_exec is missing []string parameter")

		}
		execExprSlice := []*govaluate.EvaluableExpression{}
		for _, exec := range es {
			execExpr, err := govaluate.NewEvaluableExpressionWithFunctions(exec, functions)
			if err != nil {
				logger.Crit("Query construction failed", "exec", exec, "err", err)
				continue
			}
			execExprSlice = append(execExprSlice, execExpr)
		}

		return func(parameters *MapperParameters) error {
			for _, ee := range execExprSlice {
				val, err := ee.Evaluate(parameters.Params)
				if err != nil {
					logger.Error("Expression failed to evaluate", "parameters", parameters.Params, "val", val)
					continue
				}
			}
			return nil
		}, nil, nil
	})

}
