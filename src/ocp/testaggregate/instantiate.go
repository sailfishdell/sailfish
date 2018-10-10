package testaggregate

import (
	"context"
	"errors"
	"sync"

	"github.com/Knetic/govaluate"
	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/awesome_mapper"
	am2 "github.com/superchalupa/sailfish/src/ocp/awesome_mapper2"
	"github.com/superchalupa/sailfish/src/ocp/model"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

type viewFunc func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, vw *view.View, cfg interface{}, parameters map[string]interface{}) error

var initViewRegistry sync.Once
var viewFunctionsRegistry map[string]viewFunc
var viewFunctionsRegistryMu sync.Mutex

func RegisterViewFunction(name string, fn viewFunc) {
	viewFunctionsRegistryMu.Lock()
	initViewRegistry.Do(func() { viewFunctionsRegistry = map[string]viewFunc{} })
	defer viewFunctionsRegistryMu.Unlock()
	viewFunctionsRegistry[name] = fn
}

func GetViewFunction(name string) viewFunc {
	viewFunctionsRegistryMu.Lock()
	initViewRegistry.Do(func() { viewFunctionsRegistry = map[string]viewFunc{} })
	defer viewFunctionsRegistryMu.Unlock()
	return viewFunctionsRegistry[name]
}

type EventService interface {
	PublishResourceUpdatedEventsForModel(context.Context, string) view.Option
}

func RunRegistryFunctions(evtsvc EventService) {
	// views
	RegisterWithURI()
	RegisterPublishEvents(evtsvc)

	// controller
	RegisterAwesomeMapper()
}

func RegisterWithURI() {
	RegisterViewFunction("with_URI", func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, vw *view.View, cfg interface{}, parameters map[string]interface{}) error {
		exprStr, ok := cfg.(string)
		if !ok {
			logger.Crit("Failed to type assert cfg to string", "cfg", cfg)
			return errors.New("Failed to type assert expression to string")
		}
		expr, err := govaluate.NewEvaluableExpressionWithFunctions(exprStr, functions)
		if err != nil {
			logger.Crit("Failed to create evaluable expression", "expr", expr, "err", err)
			return errors.New("Failed to create evaluable expression")
		}
		uri, err := expr.Evaluate(parameters)
		if err != nil {
			logger.Crit("expression evaluation failed", "expr", expr, "err", err)
			return errors.New("expression evaluation failed")
		}
		uriStr, ok := uri.(string)
		if !ok {
			logger.Crit("expression returned non-string", "exprStr", exprStr)
			return errors.New("expression returned non-string")
		}

		logger.Debug("Registering view with URI", "expr", exprStr, "uri", uriStr)
		vw.ApplyOption(view.WithURI(uriStr))

		return nil
	})
}

func RegisterPublishEvents(evtSvc EventService) {
	RegisterViewFunction("PublishResourceUpdatedEventsForModel", func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, vw *view.View, cfg interface{}, parameters map[string]interface{}) error {
		modelName, ok := cfg.(string)
		if !ok {
			logger.Crit("Failed to type assert cfg to string", "cfg", cfg)
			return errors.New("Failed to type assert expression to string")
		}

		logger.Debug("Running PublishResourceUpdatedEventsForModel", "modelName", modelName)
		vw.ApplyOption(evtSvc.PublishResourceUpdatedEventsForModel(ctx, modelName))

		return nil
	})
}

type controllerFunc func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, vw *view.View, cfg interface{}, parameters map[string]interface{}) error

var initControllerRegistry sync.Once
var controllerFunctionsRegistry map[string]controllerFunc
var controllerFunctionsRegistryMu sync.Mutex

func RegisterControllerFunction(name string, fn controllerFunc) {
	controllerFunctionsRegistryMu.Lock()
	initControllerRegistry.Do(func() { controllerFunctionsRegistry = map[string]controllerFunc{} })
	defer controllerFunctionsRegistryMu.Unlock()
	controllerFunctionsRegistry[name] = fn
}

func GetControllerFunction(name string) controllerFunc {
	controllerFunctionsRegistryMu.Lock()
	initControllerRegistry.Do(func() { controllerFunctionsRegistry = map[string]controllerFunc{} })
	defer controllerFunctionsRegistryMu.Unlock()
	return controllerFunctionsRegistry[name]
}

func RegisterAwesomeMapper() {
	RegisterControllerFunction("AwesomeMapper", func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, vw *view.View, cfg interface{}, parameters map[string]interface{}) error {
		cfgParams, ok := cfg.(map[interface{}]interface{})
		if !ok {
			logger.Crit("Failed to type assert cfg to string", "cfg", cfg)
			return errors.New("Failed to type assert expression to string")
		}

		// ctx, logger, viper, *model, cfg_name, params
		modelName, ok := cfgParams["modelname"]
		if !ok {
			modelName = "default"
		}
		modelNameStr, ok := modelName.(string)
		if !ok {
			modelNameStr = "default"
		}

		cfgSection, ok := cfgParams["cfgsection"]
		if !ok {
			logger.Crit("Required parameter 'cfgsection' missing, cannot continue")
			return nil
		}
		cfgSectionStr, ok := cfgSection.(string)
		if !ok {
			logger.Crit("Required parameter 'cfgsection' could not be cast to string")
			return nil
		}

		logger.Debug("Creating awesome_mapper controller", "modelName", modelNameStr, "cfgSection", cfgSectionStr)
		awesome_mapper.New(ctx, logger, cfgMgr, vw.GetModel(modelNameStr), cfgSectionStr, parameters)

		return nil
	})
}

func RegisterAM2(s *am2.Service) {
	RegisterControllerFunction("AM2", func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, vw *view.View, cfg interface{}, parameters map[string]interface{}) error {
		cfgParams, ok := cfg.(map[interface{}]interface{})
		if !ok {
			logger.Crit("Failed to type assert cfg to string", "cfg", cfg)
			return errors.New("Failed to type assert expression to string")
		}

		// ctx, logger, viper, *model, cfg_name, params
		modelName, ok := cfgParams["modelname"]
		if !ok {
			modelName = "default"
		}
		modelNameStr, ok := modelName.(string)
		if !ok {
			modelNameStr = "default"
		}

		cfgSection, ok := cfgParams["cfgsection"]
		if !ok {
			logger.Crit("Required parameter 'cfgsection' missing, cannot continue")
			return errors.New("Required parameter 'cfgsection' missing, cannot continue")
		}
		cfgSectionStr, ok := cfgSection.(string)
		if !ok {
			logger.Crit("Required parameter 'cfgsection' could not be cast to string")
			return errors.New("Required parameter 'cfgsection' could not be cast to string")
		}

		uniqueName, ok := cfgParams["uniquename"]
		if !ok {
			logger.Crit("Required parameter 'uniquename' missing, cannot continue")
			return errors.New("Required parameter 'uniquename' missing, cannot continue")
		}
		uniqueNameStr, ok := uniqueName.(string)
		if !ok {
			logger.Crit("Required parameter 'uniquename' could not be cast to string")
			return errors.New("Required parameter 'uniquename' could not be cast to string")
		}

		expr, err := govaluate.NewEvaluableExpressionWithFunctions(uniqueNameStr, functions)
		if err != nil {
			logger.Crit("Failed to create evaluable expression", "uniqueNameStr", uniqueNameStr, "err", err)
			return err
		}
		uniqueName, err = expr.Evaluate(parameters)
		if err != nil {
			logger.Crit("expression evaluation failed", "expr", expr, "err", err)
			return err
		}

		uniqueNameStr, ok = uniqueName.(string)
		if err != nil {
			logger.Crit("could not cast result to string", "uniqueName", uniqueName)
			return errors.New("could not cast result to string")
		}

		logger.Debug("Creating awesome_mapper2 controller", "modelName", modelNameStr, "cfgSection", cfgSectionStr, "uniqueName", uniqueNameStr)
		s.NewMapping(ctx, logger, cfgMgr, vw.GetModel(modelNameStr), cfgSectionStr, uniqueNameStr, parameters)

		return nil
	})
}

type config struct {
	Logger      []interface{}
	Models      map[string]map[string]interface{}
	View        []map[string]interface{}
	Controllers []map[string]interface{}
}

func InstantiateFromCfg(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, name string, parameters map[string]interface{}) (log.Logger, *view.View, error) {
	subCfg := cfgMgr.Sub("views")
	if subCfg == nil {
		logger.Crit("missing config file section: 'views'")
		return nil, nil, errors.New("invalid config section 'views'")
	}

	config := config{}

	err := subCfg.UnmarshalKey(name, &config)
	if err != nil {
		logger.Crit("unamrshal failed", "err", err, "name", name)
		return nil, nil, errors.New("unmarshal failed")
	}

	// Instantiate logger
	subLogger := logger.New(config.Logger...)
	subLogger.Debug("Instantiated new logger")

	// Instantiate view
	vw := view.New(view.WithDeferRegister())

	// Instantiate Models
	for modelName, modelProperties := range config.Models {
		subLogger.Debug("creating model", "name", modelName)
		m := vw.GetModel(modelName)
		if m == nil {
			m = model.New()
		}
		for propName, propValue := range modelProperties {
			propValueStr, ok := propValue.(string)
			if !ok {
				continue
			}
			expr, err := govaluate.NewEvaluableExpressionWithFunctions(propValueStr, functions)
			if err != nil {
				logger.Crit("Failed to create evaluable expression", "propValueStr", propValueStr, "err", err)
				continue
			}
			propValue, err := expr.Evaluate(parameters)
			if err != nil {
				logger.Crit("expression evaluation failed", "expr", expr, "err", err)
				continue
			}

			subLogger.Debug("setting model property", "propname", propName, "propValue", propValue)
			m.UpdateProperty(propName, propValue)
		}
		vw.ApplyOption(view.WithModel(modelName, m))
	}

	// Run view functions
	for _, viewFn := range config.View {
		viewFnName, ok := viewFn["fn"]
		if !ok {
			subLogger.Crit("Missing function name")
			continue
		}
		viewFnNameStr := viewFnName.(string)
		if !ok {
			subLogger.Crit("fn name isnt a string")
			continue
		}
		viewFnParams, ok := viewFn["params"]
		if !ok {
			subLogger.Crit("Missing function parameters")
			continue
		}
		fn := GetViewFunction(viewFnNameStr)
		if fn == nil {
			subLogger.Crit("Could not find registered view function", "function", viewFnNameStr)
			continue
		}
		fn(ctx, logger, cfgMgr, vw, viewFnParams, parameters)
	}

	// Instantiate controllers
	for _, contollerFn := range config.Controllers {
		contollerFnName, ok := contollerFn["fn"]
		if !ok {
			subLogger.Crit("Missing function name")
			continue
		}
		controllerFnNameStr := contollerFnName.(string)
		if !ok {
			subLogger.Crit("fn name isnt a string")
			continue
		}
		contollerFnParams, ok := contollerFn["params"]
		if !ok {
			subLogger.Crit("Missing function parameters")
			continue
		}
		fn := GetControllerFunction(controllerFnNameStr)
		if fn == nil {
			subLogger.Crit("Could not find registered controller function", "function", controllerFnNameStr)
			continue
		}
		fn(ctx, logger, cfgMgr, vw, contollerFnParams, parameters)
	}

	// Instantiate aggregate

	// register the plugin
	domain.RegisterPlugin(func() domain.Plugin { return vw })

	return subLogger, vw, nil
}
