package awesome_mapper2

import (
	"context"
	"errors"
	"sync"

	eh "github.com/looplab/eventhorizon"
	"github.com/spf13/viper"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/looplab/eventwaiter"
	"github.com/superchalupa/sailfish/src/ocp/model"
)

// ##################################
// matches redfish.yaml config file
// ##################################

type ConfigFileModelUpdate struct {
	Property string
	Query    string
	Default  interface{}
}

type ConfigFileMappingEntry struct {
	SelectEventType string

	SelectFN     string
	SelectParams []string
	Select       string

	Process     []map[string]interface{}
	ModelUpdate []*ConfigFileModelUpdate
	Exec        []string
}

// ########################
// Internal data structures
// ########################

type Service struct {
	sync.RWMutex
	logger log.Logger

	// map [ config section ] -> array of mappings
	cfgSection map[string]*ConfigSection

	// optimized data structure to process incoming events
	// map[ event type ] -> map[ yaml config name ]
	hash map[eh.EventType][]*MapperConfig
}

type ConfigSection struct {
	sync.RWMutex
	onetimePer []processSetupFunc
	parameters map[string]*MapperParameters
	mappings   []*MapperConfig
}

type MapperParameters struct {
	uuid   eh.UUID
	model  *model.Model
	Params map[string]interface{}
	ctx    context.Context
}

type MapperConfig struct {
	sync.RWMutex
	eventType   eh.EventType
	selectFnStr string
	selectFn    SelectFunc
	processFn   []processFunc
	cfg         *ConfigSection
}

func (s *Service) NewMapping(ctx context.Context, logger log.Logger, cfg *viper.Viper, cfgMu *sync.RWMutex, mdl *model.Model, cfgName string, uniqueName string, parameters map[string]interface{}, UUID interface{}) error {
	var instanceParameters *MapperParameters
	functionsMu.RLock()
	defer functionsMu.RUnlock()

	logger = logger.New("module", "am2")

	// TODO: this is a candidate to push down into a closure for the actual functions instead of accounting for this at the global level
	if UUID == nil {
		instanceParameters = &MapperParameters{ctx: ctx, model: mdl, Params: map[string]interface{}{}}
	} else {
		instanceParameters = &MapperParameters{ctx: ctx, model: mdl, uuid: UUID.(eh.UUID), Params: map[string]interface{}{}}
	}
	for k, v := range parameters {
		instanceParameters.Params[k] = v
	}

	// ###############################################
	// Ensure that we have that section in our service
	// ###############################################
	s.RLock()
	mappingsForSection, ok := s.cfgSection[cfgName]
	s.RUnlock()

	if !ok {
		// ############################################
		// otherwise we need to load it from the config file
		logger.Info("Loading config for mapping from config file", "cfgName", cfgName)

		err := func() error {
			cfgMu.Lock()
			defer cfgMu.Unlock()
			s.Lock()
			defer s.Unlock()

			// Pull out the section from YAML config file
			fullSectionMappingList := []ConfigFileMappingEntry{}
			k := cfg.Sub("awesome_mapper")
			if k == nil {
				logger.Warn("missing config file section: 'awesome_mapper'")
				return errors.New("missing config section 'awesome_mapper'")
			}
			// we have a struct that mirrors what is supposed to be in the config, pull it directly in
			err := k.UnmarshalKey(cfgName, &fullSectionMappingList)
			if err != nil {
				logger.Warn("unmarshal failed", "err", err)
				return errors.New("unmarshal failed")
			}
			logger.Debug("updating mappings", "mappings", fullSectionMappingList)

			mappingsForSection = &ConfigSection{
				parameters: map[string]*MapperParameters{},
				mappings:   []*MapperConfig{},
			}
			s.cfgSection[cfgName] = mappingsForSection

			// transcribe each individual mapper in the config section into our config
			for _, cfgEntry := range fullSectionMappingList {
				logger.Info("Add one mapping row", "cfgEntry", cfgEntry, "select", cfgEntry.Select)
				setupSelectFuncsMu.RLock()
				setupSelectFn, ok := setupSelectFuncs[cfgEntry.SelectFN]
				if !ok {
					// fallback if choice not found
					setupSelectFn = setupSelectFuncs["govaluate_select"]
				}
				setupSelectFuncsMu.RUnlock()
				selectFn, err := setupSelectFn(logger.New("cfgName", cfgName), cfgEntry)

				if err != nil {
					logger.Crit("config setup failed", "err", err)
					continue
				}

				mappingsForSection.Lock()
				mc := &MapperConfig{
					eventType:   eh.EventType(cfgEntry.SelectEventType),
					selectFnStr: cfgEntry.Select,
					selectFn:    selectFn,
					processFn:   []processFunc{},
					cfg:         mappingsForSection,
				}
				mappingsForSection.mappings = append(mappingsForSection.mappings, mc)

				// default Process
				if len(cfgEntry.Process) == 0 {
					cfgEntry.Process = append(cfgEntry.Process, map[string]interface{}{"name": "govaluate_modelupdate", "params": cfgEntry.ModelUpdate})
					cfgEntry.Process = append(cfgEntry.Process, map[string]interface{}{"name": "govaluate_exec", "params": cfgEntry.Exec})
				}
				for _, processFnObj := range cfgEntry.Process {
					fnName, ok := processFnObj["name"].(string)
					if !ok {
						logger.Warn("Process Function name not found")
						continue
					}
					fnParams, ok := processFnObj["params"]
					if !ok {
						logger.Warn("Process Function params not found")
						continue
					}

					setupProcessFn, ok := setupProcessFuncs[fnName]
					if !ok {
						logger.Warn("SetupProcessFunc not found", "function name", fnName)
						continue
					}

					processFn, oneTimeFn, err := setupProcessFn(logger.New("cfgName", cfgName), fnParams)
					if !ok {
						logger.Warn("SetupProcessFn failed", "function name", fnName, "error", err)
						continue
					}

					mc.processFn = append(mc.processFn, processFn)

					if oneTimeFn != nil {
						mappingsForSection.onetimePer = append(mappingsForSection.onetimePer, oneTimeFn)
					}
				}
				mappingsForSection.Unlock()

			}

			// and then optimize by rebuilding the event indexed hash
			logger.Info("start Optimize hash", "s.cfgSection", s.cfgSection)
			for k := range s.hash {
				delete(s.hash, k)
			}

			for name, cfgSection := range s.cfgSection {
				cfgSection.RLock()
				logger.Info("Optimize hash", "LEN", len(cfgSection.mappings), "section", name, "config", cfgSection, "mappings", cfgSection.mappings)

				for index, singleMapping := range cfgSection.mappings {
					logger.Info("Add entry for event type", "eventType", singleMapping.eventType, "index", index)

					typeArray, ok := s.hash[singleMapping.eventType]
					if !ok {
						typeArray = []*MapperConfig{}
					}

					typeArray = append(typeArray, singleMapping)
					s.hash[singleMapping.eventType] = typeArray
				}
				cfgSection.RUnlock()
			}
			return nil
		}()
		if err != nil {
			return err
		}
	}

	logger.Debug("UPDATE MODEL")

	// Add our parameters to the pile
	mappingsForSection.Lock()
	mappingsForSection.parameters[uniqueName] = instanceParameters
	mappingsForSection.Unlock()

	for _, fn := range mappingsForSection.onetimePer {
		fn(instanceParameters)
	}

	return nil
}

func StartService(ctx context.Context, logger log.Logger, eb eh.EventBus, ch eh.CommandHandler, d BusObjs) (*Service, error) {

	ret := &Service{
		logger:     logger.New("module", "am2"),
		cfgSection: map[string]*ConfigSection{},
		hash:       map[eh.EventType][]*MapperConfig{},
	}

	listener := eventwaiter.NewListener(ctx, logger, d.GetWaiter(), func(ev eh.Event) bool {
		ret.RLock()
		defer ret.RUnlock()

		// hash lookup to see if we process this event, should be the fastest way
		// comment out logging in the fast path. uncomment to enable.
		//ret.logger.Debug("am2 check event FILTER", "type", ev.EventType())
		_, ok := ret.hash[ev.EventType()]
		return ok
	})

	go listener.ProcessEvents(ctx, func(event eh.Event) {
		ret.RLock()
		mappings := ret.hash[event.EventType()]
		ret.RUnlock()

		postProcs := []func(){}

		// comment out logging in the fast path. uncomment to enable.
		//ret.logger.Debug("am2 processing event", "type", event.EventType())
		for _, mapping := range mappings {
			mapping.Lock()
			mapping.cfg.Lock()
			for cfgName, parameters := range mapping.cfg.parameters {
				// comment out logging in the fast path. uncomment to enable.
				//ret.logger.Debug("am2 check mapping", "type", event.EventType(), "select", mapping.selectStr, "for config", cfgName)

				// TODO: these lines should probably go...
				// for govaluate
				parameters.Params["cfg_params"] = parameters
				parameters.Params["type"] = string(event.EventType())
				parameters.Params["data"] = event.Data()
				parameters.Params["event"] = event
				parameters.Params["model"] = parameters.model
				parameters.Params["postprocs"] = &postProcs

				// delete these to save up mem before checking error conditions
				cleanup := func() {
					delete(parameters.Params, "data")
					delete(parameters.Params, "event")
					delete(parameters.Params, "model")
				}
				val, err := mapping.selectFn(parameters)

				if err != nil {
					ret.logger.Error("expression failed to evaluate", "err", err, "select", mapping.selectFnStr, "for config", cfgName)
					cleanup()
					continue
				}

				if !val {
					// comment out logging in the fast path. uncomment to enable.
					//ret.logger.Debug("Select did not match", "cfgName", cfgName, "type", event.EventType(), "select", mapping.selectStr, "val", val)
					cleanup()
					continue
				}

				for _, fn := range mapping.processFn {
					// Use Id to update aggregate. oh.. I need versioning now. ugh
					err := fn(parameters, event, ch, d)
					if err != nil {
						ret.logger.Error("expression failed to evaluate", "err", err, "select", mapping.selectFnStr, "for config", cfgName)
					}
				}

				// comment out logging in the fast path. uncomment to enable.
				//ret.logger.Info("GOT A MATCH!!!!!")

				cleanup()
			}
			mapping.cfg.Unlock()
			mapping.Unlock()
		}

		for _, fn := range postProcs {
			fn()
		}
	})

	return ret, nil
}
