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
	expr     []govaluate.ExpressionToken
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

type OneMapperConfig struct {
	model  *model.Model
	params map[string]interface{}
}

type singleModelUpdate struct {
	property    string
	queryString string
	queryExpr   []govaluate.ExpressionToken
}

type AwesomeMapperConfig struct {
	configs      map[string]*OneMapperConfig
	selectStr    string
	selectExpr   []govaluate.ExpressionToken
	modelUpdates []*singleModelUpdate
}

type Service struct {
	eb     eh.EventBus
	logger log.Logger

	// map[ event type ] -> map[ yaml config name ]
	eventTypes   map[eh.EventType]map[string]*AwesomeMapperConfig
	eventTypesMu sync.RWMutex
}

func StartService(ctx context.Context, logger log.Logger, eb eh.EventBus) (*Service, error) {
	EventPublisher := eventpublisher.NewEventPublisher()
	eb.AddHandler(eh.MatchAny(), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter(eventwaiter.SetName("Awesome Mapper2"))
	EventPublisher.AddObserver(EventWaiter)

	ret := &Service{
		eb:         eb,
		logger:     logger.New("module", "am2"),
		eventTypes: map[eh.EventType]map[string]*AwesomeMapperConfig{},
	}

	// stream processor for action events
	sp, err := event.NewESP(ctx, event.CustomFilter(
		func(ev eh.Event) bool {
			ret.eventTypesMu.RLock()
			defer ret.eventTypesMu.RUnlock()

			// hash lookup to see if we process this event, should be the fastest way
			ret.logger.Debug("am2 checking for processor for event", "type", ev.EventType())
			if _, ok := ret.eventTypes[ev.EventType()]; ok {
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
		ret.eventTypesMu.RLock()
		defer ret.eventTypesMu.RUnlock()

		ret.logger.Debug("am2 processing event", "type", event.EventType())
		for configName, config := range ret.eventTypes[event.EventType()] {
			ret.logger.Debug("am2 found processor", "type", event.EventType(), "name", configName, "config", config)
			expr, err := govaluate.NewEvaluableExpressionFromTokens(config.selectExpr)
			if err != nil {
				ret.logger.Error("failed to instantiate expression from tokens", "err", err)
				continue
			}
			for cfgName, cfg := range config.configs {
				ret.logger.Debug("am2 running for config", "cfgName", cfgName, "type", event.EventType(), "name", configName, "config", config, "select", config.selectStr)
				// single threaded here, so can update the parameters struct. If this changes, have to update this
				cfg.params["type"] = string(event.EventType())
				cfg.params["data"] = event.Data()
				cfg.params["event"] = event
				cfg.params["model"] = cfg.model
				val, err := expr.Evaluate(cfg.params)
				if err != nil {
					ret.logger.Error("expression failed to evaluate", "err", err, "select", config.selectStr, "config", cfgName, "name", configName)
					continue
				}
				valBool, ok := val.(bool)
				if !ok {
					ret.logger.Info("NOT A BOOL", "cfgName", cfgName, "type", event.EventType(), "name", configName, "config", config, "select", config.selectStr)
					continue
				}
				if !valBool {
					ret.logger.Info("Select didnt match", "cfgName", cfgName, "type", event.EventType(), "name", configName, "config", config, "select", config.selectStr)
					continue
				}

				cfg.model.StopNotifications()
				defer cfg.model.StartNotifications()
				defer cfg.model.NotifyObservers()
				for _, m := range config.modelUpdates {
					expr, err := govaluate.NewEvaluableExpressionFromTokens(m.queryExpr)
					cfg.params["propname"] = m.property
					val, err := expr.Evaluate(cfg.params)
					if err != nil {
						ret.logger.Error("Expression failed to evaluate", "configName", configName, "queryString", m.queryString, "parameters", cfg.params, "err", err)
						continue
					}
					ret.logger.Info("Updating property!", "property", m.property, "value", val, "Event", event, "EventData", event.Data())
					cfg.model.UpdateProperty(m.property, val)
				}
			}
		}
	})

	return ret, nil
}

func (s *Service) NewMapping(ctx context.Context, logger log.Logger, cfg *viper.Viper, mdl *model.Model, cfgName string, uniqueName string, parameters map[string]interface{}) error {
	s.eventTypesMu.Lock()
	defer s.eventTypesMu.Unlock()

	logger = logger.New("module", "am2")

	newconfig := &OneMapperConfig{model: mdl, params: map[string]interface{}{}}
	for k, v := range parameters {
		newconfig.params[k] = v
	}

	c := []ConfigFileMappingEntry{}
	k := cfg.Sub("awesome_mapper")
	if k == nil {
		logger.Warn("missing config file section: 'awesome_mapper'")
		return errors.New("Missing config section 'awesome_mapper'")
	}
	err := k.UnmarshalKey(cfgName, &c)
	if err != nil {
		logger.Warn("unmarshal failed", "err", err)
		return errors.New("unmarshal failed")
	}
	logger.Info("updating mappings", "mappings", c)

	mdl.StopNotifications()
	for _, c := range c {
		m, ok := s.eventTypes[eh.EventType(c.SelectEventType)]
		if !ok {
			logger.Info("adding new EMPTY awesome mapper config for event type", "eventtype", c.SelectEventType)
			m = map[string]*AwesomeMapperConfig{}
			s.eventTypes[eh.EventType(c.SelectEventType)] = m
		}

		mapperConfig, ok := m[cfgName]
		if !ok {
			selectExpr, err := govaluate.NewEvaluableExpressionWithFunctions(c.Select, functions)
			if err != nil {
				logger.Crit("Query construction failed", "select", c.Select, "err", err)
				return errors.New("Query construction failed")
			}

			mapperConfig = &AwesomeMapperConfig{
				selectStr:    c.Select,
				configs:      map[string]*OneMapperConfig{},
				selectExpr:   selectExpr.Tokens(),
				modelUpdates: []*singleModelUpdate{},
			}
			for _, up := range c.ModelUpdate {
				queryExpr, err := govaluate.NewEvaluableExpressionWithFunctions(up.Query, functions)
				if err != nil {
					logger.Crit("Query construction failed", "query", up.Query, "err", err)
					return errors.New("Query construction failed")
				}

				newModelUpdate := &singleModelUpdate{
					property:    up.Property,
					queryString: up.Query,
					queryExpr:   queryExpr.Tokens(),
				}

				// set model default value if present
				if up.Default != nil {
					mdl.UpdateProperty(up.Property, up.Default)
				}
				mapperConfig.modelUpdates = append(mapperConfig.modelUpdates, newModelUpdate)
				logger.Info("adding new mapping", "eventtype", c.SelectEventType, "cfgName", cfgName, "query", up.Query)
			}

			m[cfgName] = mapperConfig
			logger.Info("adding new awesome mapper config for event type for config", "eventtype", c.SelectEventType, "cfgName", cfgName)
		}

		mapperConfig.configs[uniqueName] = newconfig

	}
	mdl.StartNotifications()
	mdl.NotifyObservers()

	return nil
}
