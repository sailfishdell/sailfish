package awesome_mapper2

import (
	"context"
	"errors"
	"sync"

	"github.com/Knetic/govaluate"
	"github.com/spf13/viper"

	eh "github.com/looplab/eventhorizon"
	eventpublisher "github.com/looplab/eventhorizon/publisher/local"

	"github.com/superchalupa/sailfish/src/eventwaiter"
	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/event"
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
	Select          string
	SelectEventType string
	ModelUpdate     []*ConfigFileModelUpdate
	Exec            []string
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
	parameters map[string]*MapperParameters
	mappings   []*MapperConfig
}

type MapperParameters struct {
	model  *model.Model
	params map[string]interface{}
}

type MapperConfig struct {
	eventType    eh.EventType
	selectStr    string
	selectExpr   *govaluate.EvaluableExpression
	modelUpdates []*ModelUpdate
	exec         []*Exec
	cfg          *ConfigSection
}

type ModelUpdate struct {
	property    string
	queryString string
	queryExpr   *govaluate.EvaluableExpression
	defaultVal  interface{}
}

type Exec struct {
	execString string
	execExpr   *govaluate.EvaluableExpression
}

func (s *Service) NewMapping(ctx context.Context, logger log.Logger, cfg *viper.Viper, cfgMu *sync.RWMutex, mdl *model.Model, cfgName string, uniqueName string, parameters map[string]interface{}) error {
	s.Lock()
	defer s.Unlock()
	logger = logger.New("module", "am2")

	instanceParameters := &MapperParameters{model: mdl, params: map[string]interface{}{}}
	for k, v := range parameters {
		instanceParameters.params[k] = v
	}

	// ###############################################
	// Ensure that we have that section in our service
	// ###############################################
	mappingsForSection, ok := s.cfgSection[cfgName]

	if !ok {
		// ############################################
		// otherwise we need to load it from the config file
		logger.Info("Loading config for mapping from config file", "cfgName", cfgName)

		cfgMu.Lock()
		defer cfgMu.Unlock()

		// Pull out the section from YAML config file
		fullSectionMappingList := []ConfigFileMappingEntry{}
		k := cfg.Sub("awesome_mapper")
		if k == nil {
			logger.Warn("missing config file section: 'awesome_mapper'")
			return errors.New("Missing config section 'awesome_mapper'")
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

			// parse the expression
			selectExpr, err := govaluate.NewEvaluableExpressionWithFunctions(cfgEntry.Select, functions)
			if err != nil {
				logger.Crit("Select construction failed", "select", cfgEntry.Select, "err", err, "cfgName", cfgName)
				continue
			}

			mc := &MapperConfig{
				eventType:    eh.EventType(cfgEntry.SelectEventType),
				selectStr:    cfgEntry.Select,
				selectExpr:   selectExpr,
				modelUpdates: []*ModelUpdate{},
				exec:         []*Exec{},
				cfg:          mappingsForSection,
			}
			mappingsForSection.mappings = append(mappingsForSection.mappings, mc)

			for _, modelUpdate := range cfgEntry.ModelUpdate {
				queryExpr, err := govaluate.NewEvaluableExpressionWithFunctions(modelUpdate.Query, functions)
				if err != nil {
					logger.Crit("Query construction failed", "query", modelUpdate.Query, "err", err, "cfgName", cfgName, "select", cfgEntry.Select)
					continue
				}

				mc.modelUpdates = append(mc.modelUpdates, &ModelUpdate{
					property:    modelUpdate.Property,
					queryString: modelUpdate.Query,
					queryExpr:   queryExpr,
					defaultVal:  modelUpdate.Default,
				})
			}

			for _, exec := range cfgEntry.Exec {
				execExpr, err := govaluate.NewEvaluableExpressionWithFunctions(exec, functions)
				if err != nil {
					logger.Crit("Query construction failed", "exec", exec, "err", err, "cfgName", cfgName, "select", cfgEntry.Select)
					continue
				}

				mc.exec = append(mc.exec, &Exec{
					execString: exec,
					execExpr:   execExpr,
				})
			}

		}

		// and then optimize by rebuilding the event indexed hash
		logger.Info("start Optimize hash", "s.cfgSection", s.cfgSection)
		for k := range s.hash {
			delete(s.hash, k)
		}

		for name, cfgSection := range s.cfgSection {
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
		}
	}

	logger.Debug("UPDATE MODEL")

	// Add our parameters to the pile
	mappingsForSection.parameters[uniqueName] = instanceParameters

	// no need to set defaults if there is no model to put them in...
	if mdl == nil {
		return nil
	}

	// now set all of the model default values based on the mapper config
	for _, mapping := range mappingsForSection.mappings {
		mdl.StopNotifications()
		for _, mapperUpdate := range mapping.modelUpdates {
			//// set model default value if present
			if mapperUpdate.defaultVal != nil {
				mdl.UpdateProperty(mapperUpdate.property, mapperUpdate.defaultVal)
			}
		}
		mdl.StartNotifications()
		mdl.NotifyObservers()
	}

	return nil
}

func StartService(ctx context.Context, logger log.Logger, eb eh.EventBus) (*Service, error) {
	EventPublisher := eventpublisher.NewEventPublisher()
	eb.AddHandler(eh.MatchAny(), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter(eventwaiter.SetName("Awesome Mapper2"), eventwaiter.NoAutoRun)
	EventPublisher.AddObserver(EventWaiter)
	go EventWaiter.Run()

	ret := &Service{
		logger:     logger.New("module", "am2"),
		cfgSection: map[string]*ConfigSection{},
		hash:       map[eh.EventType][]*MapperConfig{},
	}

	// stream processor for action events
	sp, err := event.NewESP(ctx, event.CustomFilter(
		func(ev eh.Event) bool {
			ret.RLock()
			defer ret.RUnlock()

			// hash lookup to see if we process this event, should be the fastest way
			// comment out logging in the fast path. uncomment to enable.
			//ret.logger.Debug("am2 check event FILTER", "type", ev.EventType())
			if _, ok := ret.hash[ev.EventType()]; ok {
				return true
			}
			return false
		}),
		event.SetListenerName("awesome_mapper"))
	if err != nil {
		ret.logger.Error("Failed to create event stream processor", "err", err)
		return nil, errors.New("")
	}

	go sp.RunForever(func(event eh.Event) {
		ret.RLock()
		defer ret.RUnlock()

		// comment out logging in the fast path. uncomment to enable.
		//ret.logger.Debug("am2 processing event", "type", event.EventType())
		for _, mapping := range ret.hash[event.EventType()] {
			for cfgName, parameters := range mapping.cfg.parameters {
				// comment out logging in the fast path. uncomment to enable.
				//ret.logger.Debug("am2 check mapping", "type", event.EventType(), "select", mapping.selectStr, "for config", cfgName)
				parameters.params["type"] = string(event.EventType())
				parameters.params["data"] = event.Data()
				parameters.params["event"] = event
				parameters.params["model"] = parameters.model

				val, err := mapping.selectExpr.Evaluate(parameters.params)

				// delete these to save up mem before checking error conditions
				cleanup := func() {
					delete(parameters.params, "data")
					delete(parameters.params, "event")
					delete(parameters.params, "model")
				}

				if err != nil {
					ret.logger.Error("expression failed to evaluate", "err", err, "select", mapping.selectStr, "for config", cfgName)
					cleanup()
					continue
				}
				valBool, ok := val.(bool)
				if !ok {
					ret.logger.Info("NOT A BOOL", "cfgName", cfgName, "type", event.EventType(), "select", mapping.selectStr, "val", val)
					cleanup()
					continue
				}
				if !valBool {
					ret.logger.Debug("Select did not match", "cfgName", cfgName, "type", event.EventType(), "select", mapping.selectStr, "val", val)
					cleanup()
					continue
				}

				// comment out logging in the fast path. uncomment to enable.
				//ret.logger.Info("GOT A MATCH!!!!!")

				for _, updates := range mapping.modelUpdates {
					parameters.model.StopNotifications()
					// Note: LIFO order for defer
					defer parameters.model.NotifyObservers()
					defer parameters.model.StartNotifications()

					parameters.params["propname"] = updates.property
					val, err := updates.queryExpr.Evaluate(parameters.params)
					if err != nil {
						ret.logger.Error("Expression failed to evaluate", "err", err, "cfgName", cfgName, "type", event.EventType(), "queryString", updates.queryString, "parameters", parameters.params, "val", val)
						continue
					}
					ret.logger.Info("Updating property!", "property", updates.property, "value", val, "Event", event, "EventData", event.Data())
					parameters.model.UpdateProperty(updates.property, val)
				}

				delete(parameters.params, "propname")
				for _, updates := range mapping.exec {
					val, err := updates.execExpr.Evaluate(parameters.params)
					if err != nil {
						ret.logger.Error("Expression failed to evaluate", "err", err, "cfgName", cfgName, "type", event.EventType(), "execString", updates.execString, "parameters", parameters.params, "val", val)
						continue
					}
				}

				cleanup()
			}
		}
	})

	return ret, nil
}
