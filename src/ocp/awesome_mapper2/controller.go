package awesome_mapper

import (
	"context"
	"errors"
	"eventwaiter"
	"sync"

	"github.com/Knetic/govaluate"

	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/sailfish/src/log"
	"github.com/superchalupa/sailfish/src/ocp/event"
	"github.com/superchalupa/sailfish/src/ocp/model"
)

type OneMapperConfig struct {
	model  *model.Model
	params map[string]interface{}
}

type QueryConfig struct {
	queryString string
	queryExpr   []govaluate.ExpressionToken
	property    string
}

type AwesomeMapperConfig struct {
	configs    map[string]OneMapperConfig
	selectStr  string
	selectExpr []govaluate.ExpressionToken
}

type Service struct {
	eb     eh.EventBus
	logger log.Logger

	// map[ event type ] -> map[ yaml config name ]
	eventTypes   map[eh.EventType]map[string]AwesomeMapperConfig
	eventTypesMu sync.RWMutex
}

func StartService(ctx context.Context, logger log.Logger, eb eh.EventBus) (*Service, error) {
	EventPublisher := eventpublisher.NewEventPublisher()
	eb.AddHandler(eh.MatchAny(), EventPublisher)
	EventWaiter := eventwaiter.NewEventWaiter(eventwaiter.SetName("Awesome Mapper2"))
	EventPublisher.AddObserver(EventWaiter)

	ret := &Service{
		eb:         eb,
		logger:     logger,
		eventTypes: map[eh.EventType]struct{}{},
	}

	// stream processor for action events
	sp, err := event.NewESP(ctx, event.CustomFilter(
		func(ev eh.Event) bool {
			ret.eventTypesMu.RLock()
			defer ret.eventTypesMu.RUnlock()

			// hash lookup to see if we process this event, should be the fastest way
			if _, ok := ret.eventTypes[ev.EventType]; ok {
				return true
			}
			return false
		}),
		event.SetListenerName("awesome_mapper"))
	if err != nil {
		logger.Error("Failed to create event stream processor", "err", err, "select-string", loopvar.Select)
		return nil, errors.New("")
	}

	go sp.RunForever(func(event eh.Event) {
		for configName, config := range ret.eventTypes[event.EventType()] {
			logger.Info("Running awesome mapper for config", "configName", configName)
			for name, individualMapperCfg := range config.configs {
				logger.Info("Running individual mapper config", "configName", configName, "name", name)
				individualMapperCfg.model.StopNotifications()
				defer func() { individualMapperCfg.model.StartNotifications(); individualMapperCfg.NotifyObservers() }()
				// config.expr

				// single threaded here, so can update the parameters struct. If this changes, have to update this
				individualMapperCfg.params["type"] = string(event.EventType())
				individualMapperCfg.params["data"] = event.Data()
				individualMapperCfg.params["event"] = event

				expr, err := govaluate.NewEvaluableExpressionFromTokens(config.expr)
				val, err := expr.Evaluate(individualMapperCfg.params)
				if err != nil {
					logger.Error("Expression failed to evaluate", "query.Query", query.Query, "parameters", expressionParameters, "err", err)
					continue
				}
				individualMapperCfg.model.UpdateProperty(config.property, val)
			}
		}

		mdl.StopNotifications()
		for _, query := range loopvar.ModelUpdate {
			if query.expr == nil {
				logger.Crit("query is nil, that can't happen", "loopvar", loopvar)
				continue
			}
		}
		mdl.StartNotifications()
		mdl.NotifyObservers()
	})

	return ret, nil
}

// Query    string
// Select      string
