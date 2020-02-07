package testaggregate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/Knetic/govaluate"
	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
	"github.com/superchalupa/sailfish/src/ocp/model"
	"github.com/superchalupa/sailfish/src/ocp/view"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
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
	d                           *domain.DomainObjects
	ch                          eh.CommandHandler
	eb                          eh.EventBus
	ew                          *eventwaiter.EventWaiter
	cfgMgr                      *viper.Viper
	cfgMgrMu                    *sync.RWMutex
	viewFunctionsRegistry       map[string]viewFunc
	controllerFunctionsRegistry map[string]controllerFunc
	aggregateFunctionsRegistry  map[string]aggregateFunc
	serviceGlobals              map[string]interface{}
	serviceGlobalsMu            sync.RWMutex
}

type am3Service interface {
	AddEventHandler(name string, et eh.EventType, fn func(eh.Event))
}

const instantiate = eh.EventType("instantiate")
const instantiateResponse = eh.EventType("instantiate-response")

type InstantiateData struct {
	CmdID  eh.UUID
	Name   string
	Params map[string]interface{}
}

type InstantiateResponseData struct {
	CmdID eh.UUID
	Log   log.Logger
	View  *view.View
	Err   error
}

func New(logger log.Logger, cfgMgr *viper.Viper, cfgMgrMu *sync.RWMutex, d *domain.DomainObjects, am3Svc am3Service) *Service {
	ret := &Service{
		logger:                      logger,
		d:                           d,
		eb:                          d.GetBus(),
		ch:                          d.GetCommandHandler(),
		ew:                          d.GetWaiter(),
		cfgMgr:                      cfgMgr,
		cfgMgrMu:                    cfgMgrMu,
		viewFunctionsRegistry:       map[string]viewFunc{},
		controllerFunctionsRegistry: map[string]controllerFunc{},
		aggregateFunctionsRegistry:  map[string]aggregateFunc{},
		serviceGlobals:              map[string]interface{}{},
	}

	am3Svc.AddEventHandler("Instaniate", instantiate, func(event eh.Event) {
		idata, ok := event.Data().(*InstantiateData)
		if !ok {
			fmt.Printf("BAD INSTANTIATE: %+v\n", event.Data())
			return
		}
		resp := &InstantiateResponseData{CmdID: idata.CmdID}
		resp.Log, resp.View, resp.Err = ret.internalInstantiate(idata.Name, idata.Params)
		fmt.Println(resp.Err)
		ret.eb.PublishEvent(context.Background(), eh.NewEvent(instantiateResponse, resp, time.Now()))
	})

	return ret
}

func (s *Service) InstantiateNoRet(name string, parameters map[string]interface{}) {
	cmdId := eh.NewUUID()
	s.eb.PublishEvent(context.Background(), eh.NewEvent(instantiate, &InstantiateData{CmdID: cmdId, Name: name, Params: parameters}, time.Now()))
}

// Instantiate publishes an event to request the instantiate and then waits for the respone message
func (s *Service) Instantiate(name string, parameters map[string]interface{}) (log.Logger, *view.View, error) {
	cmdId := eh.NewUUID()
	s.eb.PublishEvent(context.Background(), eh.NewEvent(instantiate, &InstantiateData{CmdID: cmdId, Name: name, Params: parameters}, time.Now()))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // ensure we always cancel
	listener := eventwaiter.NewListener(ctx, s.logger, s.ew, func(ev eh.Event) bool {
		if ev.EventType() != instantiateResponse {
			return false
		}
		if data, ok := ev.Data().(*InstantiateResponseData); ok {
			return cmdId == data.CmdID
		}
		return false
	})
	defer listener.Close()
	event, err := listener.Wait(ctx)
	cancel()
	if err != nil {
		return nil, nil, err
	}

	data, ok := event.Data().(*InstantiateResponseData)
	if !ok {
		return nil, nil, errors.New("bad event reception")
	}
	return data.Log, data.View, data.Err
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

// this is a pain in the butt
func walk(v reflect.Value, visitor func(string, interface{}, func(interface{}))) {
	// Indirect through pointers and interfaces
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		v = v.Elem()
	}

	recurseIt := func(name string, val reflect.Value, set func(nv reflect.Value)) {
		named := false
		nameit := func(n string) {
			if !named {
				named = true
				// fmt.Printf("\t%s", n)
			}
		}

		if val.Type().Kind() == reflect.Interface || val.Type().Kind() == reflect.Ptr {
			val = val.Elem()
		}

		switch val.Type().Kind() {
		case reflect.Ptr, reflect.UnsafePointer:
			// no-op
			nameit("Ptr-ish")

		case reflect.Chan, reflect.Func:
			// no-op
			nameit("chan/func")

		//
		// These are all recursive structures that we should walk(...)
		//
		case reflect.Array, reflect.Slice, reflect.Map, reflect.Struct:
			nameit("array/slice/map")
			walk(val, visitor)

		//
		// The following cases all are LEAF nodes that we should visit(...)
		//
		case reflect.String:
			nameit("String")
			fallthrough
		case reflect.Bool:
			nameit("Bool")
			fallthrough
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			nameit("Int-ish")
			fallthrough
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			nameit("Uint-ish")
			fallthrough
		case reflect.Uintptr:
			nameit("Uintptr")
			fallthrough
		case reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
			nameit("Float-ish")
			fallthrough
		case reflect.Interface:
			nameit(" NOT POSSIBLE - INTERFACE - ?? HOW DID THIS HAPPEN? ==============")
			visitor(name, val.Interface(), func(nv interface{}) {
				set(reflect.ValueOf(nv))
			})
		}
	}

	switch v.Kind() {
	case reflect.Array, reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			recurseIt(fmt.Sprintf("%d", i), v.Index(i), func(nv reflect.Value) {
				// TODO: test this!
				v.Index(i).Set(nv)
			})
		}

	case reflect.Map:
		for _, k := range v.MapKeys() {
			recurseIt(k.String(), v.MapIndex(k), func(nv reflect.Value) {
				v.SetMapIndex(k, nv)
			})
		}

	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			recurseIt(v.Type().Field(i).Name, v.Field(i), func(nv reflect.Value) {
				v.Field(i).Set(nv)
			})
		}

	case reflect.String:
		fallthrough
	case reflect.Bool:
		fallthrough
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		fallthrough
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		fallthrough
	case reflect.Uintptr:
		fallthrough
	case reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:

		if v.CanInterface() && v.CanSet() {
			visitor("<flat>", v.Interface(), func(nv interface{}) {
				if v.CanSet() {
					v.Set(reflect.ValueOf(nv))
				} else if !v.CanSet() {
					fmt.Printf("   request to mutate but CANT SET!\n")
				}
			})
		} else if !v.CanInterface() {
			fmt.Printf("   CANT INTERFACE!\n")
		}

	case reflect.Ptr, reflect.Interface:
		// CANT Happen
		panic("somehow got to reflect.Ptr or reflect.Interface")

	case reflect.UnsafePointer:
		// no-op
	case reflect.Chan, reflect.Func:
		// no-op
	default:
		fmt.Printf("TYPE: %+v\n", v)
		panic("DONT KNOW WHAT TO DO!")
	}
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

		rawjson := []struct {
			Cmd  string          `json:"cmd"`
			Data json.RawMessage `json:"data"`
		}{}

		for _, p := range searchPath {
			filecontents, err := ioutil.ReadFile(p + "/" + name + ".json")
			if err != nil {
				continue
			}

			if len(filecontents) == 0 {
				continue
			}

			err = json.Unmarshal([]byte(filecontents), &rawjson)
			if err != nil {
				logger.Crit("Error unmarshalling", "err", err)
				continue
			}

			// got something
			break
		}

		if len(rawjson) == 0 {
			return nil, errors.New("cannot find requested resource")
		}

		logger.Debug("Got JSON", "rawjson", rawjson)

		cmds := []eh.Command{}
		for i := range rawjson {
			cmdName := rawjson[i].Cmd
			cmdType := eh.CommandType("internal:" + cmdName)

			cmdData := rawjson[i].Data

			cmd, err := eh.CreateCommand(cmdType)
			if err != nil {
				logger.Crit("Error creating command", "command", cmdType, "err", err)
				continue
			}
			err = json.Unmarshal(cmdData, cmd)
			if err != nil {
				logger.Crit("Error decoding json", "err", err)
				continue
			}

			walk(reflect.ValueOf(cmd), func(name string, v interface{}, mutate func(interface{})) {
				s, ok := v.(string)
				if ok {
					t := strings.ReplaceAll(s, "##GETURI##", vw.GetURI())
					if s != t {
						mutate(t)
					}
				}
			})

			switch cmdType {
			case domain.CreateRedfishResourceCommand:
				logger.Debug("doing a create command")
				createCmd := cmd.(*domain.CreateRedfishResource)
				createCmd.ID = eh.NewUUID()
			case domain.UpdateRedfishResourcePropertiesCommand:
				logger.Debug("doing an update command")
				updateCmd := cmd.(*domain.UpdateRedfishResourceProperties)

				temp := map[string]interface{}{}
				err := json.Unmarshal(cmdData, &temp)
				if err != nil {
					logger.Crit("Error unmarshalling cmd json", "err", err)
					continue
				}

				uriR, ok := temp["URI"]
				if !ok {
					logger.Crit("no URI specified")
					continue // continue for loop
				}
				uri, ok := uriR.(string)
				if !ok {
					logger.Crit("URI not a string", "uri", uriR)
					continue
				}
				updateCmd.ID, ok = s.d.GetAggregateIDOK(uri)
				if !ok {
					logger.Crit("no such aggregate to update", "uri", uri)
					continue // continue for loop
				}
			default:
				panic(fmt.Sprintf("Unsupported command. Please add code to deal with this command in instantiate.go.  --> %+v", cmd))
			}

			logger.Info("Made a command", "command", cmd)
			cmds = append(cmds, cmd)
		}

		return cmds, nil
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

// internalInstaniate will set up logger, model, view, controllers, aggregates from the config file
// 	- name should be a key in the Views section of cfgMgr
// 	- cfgMgr is the config file
// 	- parameters is a dictionary of key/value pairs that
// The following is needed in the Views[key]
//            key should have the same names as config struct above
//

func (s *Service) internalInstantiate(name string, parameters map[string]interface{}) (l log.Logger, v *view.View, e error) {
	s.logger.Info("Instantiate", "name", name)
	cfgMgr := s.cfgMgr
	cfgMgrMu := s.cfgMgrMu
	ctx := context.Background()

	newParams := map[string]interface{}{}
	for k, v := range parameters {
		newParams[k] = v
	}
	s.serviceGlobalsMu.RLock()
	for k, v := range s.serviceGlobals {
		newParams[k] = v
	}
	s.serviceGlobalsMu.RUnlock()
	newParams["serviceglobals"] = s.serviceGlobals
	newParams["serviceglobalsmu"] = &s.serviceGlobalsMu

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
	subLogger := s.logger
	if len(config.Logger) > 0 {
		subLogger = s.logger.New(config.Logger...)
	}
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

	fmt.Println("Instantiate before controller URI", vw.GetURI(), "UUID", vw.GetUUID())
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
	fmt.Println("Instantiate URI", vw.GetURI(), "UUID", vw.GetUUID())

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
		first:=true
		for _, cmd := range cmds {
			// if it's a resource create command, use the view ID for that
			if c, ok := cmd.(*domain.CreateRedfishResource); ok {
				if first {
					c.ID = vw.GetUUID()
					first = false
				} else {
					c.ID = eh.NewUUID()
				}
			
			}
			s.ch.HandleCommand(context.Background(), cmd)
		}
	}()
	fmt.Println("Instantiate before Exec URI", vw.GetURI(), "UUID", vw.GetUUID())

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
	fmt.Println("Instantiate Exec URI", vw.GetURI(), "UUID", vw.GetUUID())

	return subLogger, vw, nil
}
