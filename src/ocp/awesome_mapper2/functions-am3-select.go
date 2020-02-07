package awesome_mapper2

import (
	"errors"
	"github.com/Knetic/govaluate"
	eh "github.com/looplab/eventhorizon"
	a "github.com/superchalupa/sailfish/src/dell-resources/attributedef"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/model"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
	"path"
	"strings"
	"sync"
)

type setupSelectFunc func(log.Logger, ConfigFileMappingEntry) (SelectFunc, error)
type SelectFunc func(p *MapperParameters) (bool, error)

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

func baseuri(resourceURI string) interface{} {
	return path.Dir(resourceURI)
}

func nohash(resourceURI string) bool {
	if strings.Contains(resourceURI, "#") {
		return false
	}
	return true
}

func makeRHS(cfgEntry *MapperParameters, cfgParams []string) string {

	var rhsURI string
	for _, v := range cfgParams {
		component, ok := cfgEntry.Params[v].(string)
		if !ok {
			component = v
		}
		rhsURI = rhsURI + component
	}
	return rhsURI
}

func init() {
	InitAM3SelectSetupFunctions()

	// debugging function
	AddAM3SelectSetupFunction("true", func(l log.Logger, c ConfigFileMappingEntry) (SelectFunc, error) {
		// comment for how to use SelectParams in a function
		// remove this comment whenever we get another user for it that can be used as an example
		//fmt.Printf("\n\nSETTING UP TRUE: %s\n\n", c.SelectParams)
		return func(*MapperParameters) (bool, error) {
			//fmt.Printf("TRUE!\n")
			return true, nil
		}, nil
	})

	AddAM3SelectSetupFunction("govaluate_select", func(logger log.Logger, cfgEntry ConfigFileMappingEntry) (SelectFunc, error) {
		selectExpr, err := govaluate.NewEvaluableExpressionWithFunctions(cfgEntry.Select, functions)
		// parse the expression
		if err != nil {
			logger.Crit("Select construction failed", "select", cfgEntry.Select, "err", err)
			return nil, err
		}
		return func(parameters *MapperParameters) (bool, error) {
			val, err := selectExpr.Evaluate(parameters.Params)
			if err != nil {
				logger.Error("expression failed to evaluate", "err", err, "select", cfgEntry.Select)
				return false, err
			}

			valBool, ok := val.(bool)
			if !ok {
				logger.Info("NOT A BOOL", "type", parameters.Params["event"].(eh.Event).EventType(), "select", cfgEntry.Select, "val", val)
				return false, err
			}
			return valBool, err
		}, nil
	})

	AddAM3SelectSetupFunction("uriCheckNoHash", func(logger log.Logger, cfgEntry ConfigFileMappingEntry) (SelectFunc, error) {
		return func(p *MapperParameters) (bool, error) {

			var str string
			switch data := p.Params["data"].(type) {
			case *domain.RedfishResourceCreatedData:
				str = data.ResourceURI
			case *domain.RedfishResourceRemovedData:
				str = data.ResourceURI
			default:
				return false, errors.New("unknown data type")
			}

			nohashresult := nohash(str)
			if !nohashresult {
				return false, nil
			}
			lhs := baseuri(str)
			cfg_params, ok := p.Params["cfg_params"].(*MapperParameters)
			if !ok {
				return false, errors.New("no mapperparameters found")
			}
			rhsParams := cfgEntry.SelectParams
			rhs := makeRHS(cfg_params, rhsParams)
			return (lhs == rhs), nil
		}, nil

	})

	AddAM3SelectSetupFunction("uriEquals", func(logger log.Logger, cfgEntry ConfigFileMappingEntry) (SelectFunc, error) {
		//with hash version
		return func(p *MapperParameters) (bool, error) {
			var str string
			switch data := p.Params["data"].(type) {
			case *domain.RedfishResourceCreatedData:
				str = data.ResourceURI
			case *domain.RedfishResourceRemovedData:
				str = data.ResourceURI
			default:
				return false, errors.New("unknown data type")
			}
			cfg_params, ok := p.Params["cfg_params"].(*MapperParameters)
			if !ok {
				return false, errors.New("No mapperparameters found")
			}
			rhsParams := cfgEntry.SelectParams
			rhs := makeRHS(cfg_params, rhsParams)
			return (str == rhs), nil
		}, nil
	})

	AddAM3SelectSetupFunction("uriCheckWithHash", func(logger log.Logger, cfgEntry ConfigFileMappingEntry) (SelectFunc, error) {
		return func(p *MapperParameters) (bool, error) {
			var str string
			switch data := p.Params["data"].(type) {
			case *domain.RedfishResourceCreatedData:
				str = data.ResourceURI
			case *domain.RedfishResourceRemovedData:
				str = data.ResourceURI
			default:
				return false, errors.New("unknown data type")
			}

			lhs := baseuri(str)

			cfg_params, ok := p.Params["cfg_params"].(*MapperParameters)
			if !ok {
				return false, errors.New("no mapperparameters found")
			}
			rhsParams := cfgEntry.SelectParams
			rhs := makeRHS(cfg_params, rhsParams)
			return (lhs == rhs), nil

		}, nil

	})

	AddAM3SelectSetupFunction("isSledProfile", func(logger log.Logger, cfgEntry ConfigFileMappingEntry) (SelectFunc, error) {
		return func(p *MapperParameters) (bool, error) {
			data := p.Params["data"].(*a.AttributeUpdatedData)
			model1 := p.Params["model"].(*model.Model)
			val, ok := model1.GetPropertyOk("slot_contains")
			if !ok {
				return false, nil
			}
			lhs := (data.FQDD == val)

			rhs := (data.Name == "SledProfile")
			return (lhs && rhs), nil
		}, nil

	})

	AddAM3SelectSetupFunction("isTaskState", func(logger log.Logger, cfgEntry ConfigFileMappingEntry) (SelectFunc, error) {
		return func(p *MapperParameters) (bool, error) {
			data := p.Params["data"].(*a.AttributeUpdatedData)
			group := p.Params["Group"]
			index := p.Params["Index"]
			return (data.Group == group && data.Index == index && data.Name == "TaskState"), nil
		}, nil

	})

	AddAM3SelectSetupFunction("isRowOrColumn", func(logger log.Logger, cfgEntry ConfigFileMappingEntry) (SelectFunc, error) {
		return func(p *MapperParameters) (bool, error) {
			data := p.Params["data"].(*a.AttributeUpdatedData)
			group := p.Params["Group"]
			index := p.Params["Index"]
			rowOrColumn := (cfgEntry.SelectParams[0])
			return (data.FQDD == "System.hassis.1" && data.Group == group && data.Index == index && data.Name == rowOrColumn), nil
		}, nil

	})

}
