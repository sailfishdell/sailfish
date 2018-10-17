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
	selectExpr   []govaluate.ExpressionToken
	modelUpdates []*ModelUpdate
	cfg          *ConfigSection
}

type ModelUpdate struct {
	property    string
	queryString string
	queryExpr   []govaluate.ExpressionToken
	defaultVal  interface{}
}

func (s *Service) NewMapping(ctx context.Context, logger log.Logger, cfg *viper.Viper, mdl *model.Model, cfgName string, uniqueName string, parameters map[string]interface{}) error {
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
				selectExpr:   selectExpr.Tokens(),
				modelUpdates: []*ModelUpdate{},
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
					queryExpr:   queryExpr.Tokens(),
					defaultVal:  modelUpdate.Default,
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
	EventWaiter := eventwaiter.NewEventWaiter(eventwaiter.SetName("Awesome Mapper2"))
	EventPublisher.AddObserver(EventWaiter)

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
			ret.logger.Debug("am2 check event FILTER", "type", ev.EventType())
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

		ret.logger.Debug("am2 processing event", "type", event.EventType())
		for _, mapping := range ret.hash[event.EventType()] {
			expr, err := govaluate.NewEvaluableExpressionFromTokens(mapping.selectExpr)
			if err != nil {
				ret.logger.Error("failed to instantiate expression from tokens", "err", err, "select", mapping.selectStr)
				continue
			}

			for cfgName, parameters := range mapping.cfg.parameters {
				ret.logger.Debug("am2 check mapping", "type", event.EventType(), "select", mapping.selectStr, "for config", cfgName)
				parameters.params["type"] = string(event.EventType())
				parameters.params["data"] = event.Data()
				parameters.params["event"] = event
				parameters.params["model"] = parameters.model

				val, err := expr.Evaluate(parameters.params)

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

				ret.logger.Info("GOT A MATCH!!!!!")

				for _, updates := range mapping.modelUpdates {
					parameters.model.StopNotifications()
					defer parameters.model.StartNotifications()
					defer parameters.model.NotifyObservers()

					expr, err := govaluate.NewEvaluableExpressionFromTokens(updates.queryExpr)
					parameters.params["propname"] = updates.property
					val, err := expr.Evaluate(parameters.params)
					if err != nil {
						ret.logger.Error("Expression failed to evaluate", "err", err, "cfgName", cfgName, "type", event.EventType(), "queryString", updates.queryString, "parameters", parameters.params, "val", val)
						continue
					}
					ret.logger.Info("Updating property!", "property", updates.property, "value", val, "Event", event, "EventData", event.Data())
					parameters.model.UpdateProperty(updates.property, val)
				}
			}
		}
	})

	return ret, nil
}
