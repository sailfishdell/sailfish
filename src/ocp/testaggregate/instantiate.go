package testaggregate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"sync"

	"github.com/Knetic/govaluate"
	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/model"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"

	"github.com/mitchellh/mapstructure"
)

type closer interface {
	Close()
}

type viewFunc func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, cfg interface{}, parameters map[string]interface{}) error
type controllerFunc func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, cfg interface{}, parameters map[string]interface{}) error
type aggregateFunc func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, cfg interface{}, parameters map[string]interface{}) ([]eh.Command, error)

type Service struct {
	sync.RWMutex
	logger                      log.Logger
	ctx                         context.Context
	cfgMgr                      *viper.Viper
	cfgMgrMu                    *sync.RWMutex
	ch                          eh.CommandHandler
	viewFunctionsRegistry       map[string]viewFunc
	controllerFunctionsRegistry map[string]controllerFunc
	aggregateFunctionsRegistry  map[string]aggregateFunc
	serviceGlobals              map[string]interface{}
	serviceGlobalsMu            sync.RWMutex
}

func New(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, ch eh.CommandHandler) *Service {
	return &Service{
		ctx:                         ctx,
		logger:                      logger,
		ch:                          ch,
		cfgMgr:                      cfgMgr,
		cfgMgrMu:                    cfgMgrMu,
		viewFunctionsRegistry:       map[string]viewFunc{},
		controllerFunctionsRegistry: map[string]controllerFunc{},
		aggregateFunctionsRegistry:  map[string]aggregateFunc{},
		serviceGlobals:              map[string]interface{}{},
		serviceGlobalsMu:            sync.RWMutex{},
	}
}

func (s *Service) RegisterViewFunction(name string, fn viewFunc) {
	s.Lock()
	defer s.Unlock()
	s.viewFunctionsRegistry[name] = fn
}

func (s *Service) GetViewFunction(name string) viewFunc {
	s.RLock()
	defer s.RUnlock()
	return s.viewFunctionsRegistry[name]
}

func (s *Service) RegisterControllerFunction(name string, fn controllerFunc) {
	s.Lock()
	defer s.Unlock()
	s.controllerFunctionsRegistry[name] = fn
}

func (s *Service) GetControllerFunction(name string) controllerFunc {
	s.RLock()
	defer s.RUnlock()
	return s.controllerFunctionsRegistry[name]
}

func (s *Service) RegisterAggregateFunction(name string, fn aggregateFunc) {
	s.Lock()
	defer s.Unlock()
	s.aggregateFunctionsRegistry[name] = fn
}

func (s *Service) GetAggregateFunction(name string) aggregateFunc {
	s.RLock()
	defer s.RUnlock()

	// if we find a registered function, return it
	fn, ok := s.aggregateFunctionsRegistry[name]
	if ok {
		return fn
	}

	// otherwise return function that will use json file to instantiate
	return func(ctx context.Context, logger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, vw *view.View, cfg interface{}, parameters map[string]interface{}) ([]eh.Command, error) {
		searchPath := cfgMgr.GetStringSlice("main.aggregatesearchpath")
		for _, p := range searchPath {
			filecontents, err := ioutil.ReadFile(p + "/" + name + ".json")
			if err != nil {
				continue
			}

			fmt.Printf("Read: %s\n", p+"/"+name+".json")
			rawjson := []interface{}{}
			err = json.Unmarshal([]byte(filecontents), &rawjson)
			if err != nil {
				fmt.Printf("Error unmarshalling: %s\n", err)
				continue
			}

			fmt.Printf("Got JSON: %s\n", rawjson)

			cmds := []eh.Command{}
			for i := range rawjson {
				cmdMeta, ok := rawjson[i].(map[string]interface{})
				if !ok {
					continue
				}
				var cmd eh.Command
				switch cmdMeta["cmd"] {
				case "CreateRedfishResource":
					fmt.Printf("doing a create command\n")
					createCmd := &domain.CreateRedfishResource{}
					err = mapstructure.Decode(cmdMeta["data"], createCmd)
					if err != nil {
						fmt.Printf("Error decoding: %s\n", err)
						continue
					}
					createCmd.ResourceURI = vw.GetURI()
					createCmd.Context = vw.GetURI() + createCmd.Context
					cmd = createCmd
					fmt.Printf("Added a command: %s\n", cmd)
				}
				cmds = append(cmds, cmd)
			}

			return cmds, nil
		}
		return nil, errors.New("Cannot find requested resource")
	}
}

type config struct {
	Logger      []interface{}
	Models      map[string]map[string]interface{}
	View        []map[string]interface{}
	Controllers []map[string]interface{}
	Aggregate   string
	ExecPost    []string
}

// InstantiateFromCfg will set up logger, model, view, controllers, aggregates from the config file
// 	- name should be a key in the Views section of cfgMgr
// 	- cfgMgr is the config file
// 	- parameters is a dictionary of key/value pairs that
// The following is needed in the Views[key]
//            key should have the same names as config struct above
//

func (s *Service) Instantiate(name string, parameters map[string]interface{}) (log.Logger, *view.View, error) {
	return s.InstantiateFromCfg(s.ctx, s.cfgMgr, s.cfgMgrMu, name, parameters)
}

func (s *Service) InstantiateFromCfg(ctx context.Context, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, name string, parameters map[string]interface{}) (l log.Logger, v *view.View, e error) {
	newParams := map[string]interface{}{}
	for k, v := range parameters {
		newParams[k] = v
	}
	s.serviceGlobalsMu.RLock()
	for k, v := range s.serviceGlobals {
		newParams[k] = v
	}
	newParams["serviceglobals"] = s.serviceGlobals
	newParams["serviceglobalsmu"] = &s.serviceGlobalsMu
	s.serviceGlobalsMu.RUnlock()

	// be sure to unlock()
	cfgMgrMu.Lock()
	subCfg := cfgMgr.Sub("views")
	if subCfg == nil {
		cfgMgrMu.Unlock()
		s.RLock()
		s.logger.Crit("missing config file section: 'views'")
		s.RUnlock()
		return nil, nil, errors.New("invalid config section 'views'")
	}

	config := config{}

	err := subCfg.UnmarshalKey(name, &config)
	cfgMgrMu.Unlock()
	if err != nil {
		s.RLock()
		s.logger.Crit("unamrshal failed", "err", err, "name", name)
		s.RUnlock()
		return nil, nil, errors.New("unmarshal failed")
	}

	// Instantiate logger
	s.RLock()
	subLogger := s.logger.New(config.Logger...)
	s.RUnlock()
	subLogger.Debug("Instantiated new logger")

	// Instantiate view
	vw := view.New(view.WithDeferRegister())
	newParams["uuid"] = vw.GetUUID()
	newParams["view"] = vw

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
			functionsMu.RLock()
			expr, err := govaluate.NewEvaluableExpressionWithFunctions(propValueStr, functions)
			if err != nil {
				subLogger.Crit("Failed to create evaluable expression", "propValueStr", propValueStr, "err", err)
				functionsMu.RUnlock()
				continue
			}
			propValue, err := expr.Evaluate(newParams)
			functionsMu.RUnlock()
			if err != nil {
				subLogger.Crit("expression evaluation failed", "expr", expr, "err", err)
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
			subLogger.Crit("Missing function name", "name", name, "subsection", "View")
			continue
		}
		viewFnNameStr := viewFnName.(string)
		if !ok {
			subLogger.Crit("fn name isnt a string", "name", name, "subsection", "View")
			continue
		}
		viewFnParams, ok := viewFn["params"]
		if !ok {
			subLogger.Crit("Missing function parameters", "name", name, "subsection", "View")
			continue
		}
		fn := s.GetViewFunction(viewFnNameStr)
		if fn == nil {
			subLogger.Crit("Could not find registered view function", "function", viewFnNameStr)
			continue
		}
		fn(ctx, subLogger, cfgMgr, cfgMgrMu, vw, viewFnParams, newParams)
	}

	// Instantiate controllers
	for _, controllerFn := range config.Controllers {
		controllerFnName, ok := controllerFn["fn"]
		if !ok {
			subLogger.Crit("Missing function name", "name", name, "subsection", "Controllers")
			continue
		}
		controllerFnNameStr := controllerFnName.(string)
		if !ok {
			subLogger.Crit("fn name isnt a string", "name", name, "subsection", "Controllers")
			continue
		}
		controllerFnParams, ok := controllerFn["params"]
		if !ok {
			subLogger.Crit("Missing function parameters", "name", name, "subsection", "Controllers", "function", controllerFnNameStr)
			continue
		}
		fn := s.GetControllerFunction(controllerFnNameStr)
		if fn == nil {
			subLogger.Crit("Could not find registered controller function", "function", controllerFnNameStr)
			continue
		}
		fn(ctx, subLogger, cfgMgr, cfgMgrMu, vw, controllerFnParams, newParams)
	}

	// close any previous registrations
	p, err := domain.InstantiatePlugin(vw.PluginType())
	if err == nil && p != nil {
		if c, ok := p.(closer); ok {
			c.Close()
		}
	}

	// register the plugin
	domain.RegisterPlugin(func() domain.Plugin { return vw })
	vw.ApplyOption(view.AtClose(func() {
		subLogger.Info("Closing view", "URI", vw.GetURI(), "UUID", vw.GetUUID())
		domain.UnregisterPlugin(vw.PluginType())
	}))

	// Instantiate aggregate
	func() {
		if len(config.Aggregate) == 0 {
			subLogger.Debug("no aggregate specified in config file to instantiate.")
			return
		}
		fn := s.GetAggregateFunction(config.Aggregate)
		if fn == nil {
			subLogger.Crit("invalid aggregate function", "aggregate", config.Aggregate)
			return
		}
		cmds, err := fn(ctx, subLogger, cfgMgr, cfgMgrMu, vw, nil, newParams)
		if err != nil {
			subLogger.Crit("aggregate function returned nil")
			return
		}
		// We can get one or more commands back, handle them
		for _, cmd := range cmds {
			// if it's a resource create command, use the view ID for that
			if c, ok := cmd.(*domain.CreateRedfishResource); ok {
				c.ID = vw.GetUUID()
			}
			s.ch.HandleCommand(ctx, cmd)
		}
	}()

	// Run any POST commands
	for _, execStr := range config.ExecPost {
		subLogger.Debug("exec post", "execStr", execStr)

		functionsMu.RLock()
		expr, err := govaluate.NewEvaluableExpressionWithFunctions(execStr, functions)
		if err != nil {
			functionsMu.RUnlock()
			subLogger.Crit("Failed to create evaluable expression", "execStr", execStr, "err", err)
			continue
		}
		_, err = expr.Evaluate(newParams)
		functionsMu.RUnlock()
		if err != nil {
			subLogger.Crit("expression evaluation failed", "expr", expr, "err", err)
			continue
		}
	}

	return subLogger, vw, nil
}
