package generic_dell_resource

import (
	"context"
	"sync"

	"github.com/spf13/viper"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
	"github.com/superchalupa/go-redfish/src/log"

	attr_prop "github.com/superchalupa/go-redfish/src/dell-resources/attribute-property"
	"github.com/superchalupa/go-redfish/src/ocp/event"
	"github.com/superchalupa/go-redfish/src/ocp/model"
)

type mapping struct {
	Property string
	FQDD     string
	Group    string
	Index    string
	Name     string
}

func AddController(ctx context.Context, logger log.Logger, s *model.Service, name string, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) (func(*viper.Viper), error) {
	mappings := []mapping{}
	mappingsMu := sync.RWMutex{}

	// stream processor for action events
	sp, err := event.NewEventStreamProcessor(ctx, ew, event.CustomFilter(SelectAttributeUpdate()))
	if err != nil {
		logger.Error("Failed to create event stream processor", "err", err)
		return nil, err
	}
	sp.RunForever(func(event eh.Event) {
		if data, ok := event.Data().(*attr_prop.AttributeUpdatedData); ok {
			mappingsMu.RLock()
			defer mappingsMu.RUnlock()
			logger.Debug("Process Event", "data", data)
			for _, mapping := range mappings {
				if data.FQDD != mapping.FQDD {
					continue
				}
				if data.Group != mapping.Group {
					continue
				}
				if data.Index != mapping.Index {
					continue
				}
				if data.Name != mapping.Name {
					continue
				}

				logger.Info("Updating Model", "mapping", mapping, "data", data)
				s.UpdateProperty(mapping.Property, data.Value)
			}
		} else {
			logger.Warn("Should never happen: got an invalid event in the event handler")
		}
	})

	return func(cfg *viper.Viper) {
		mappingsMu.Lock()
		defer mappingsMu.Unlock()

		k := cfg.Sub("mappings")
		err := k.UnmarshalKey(name, &mappings)
		if err != nil {
			logger.Warn("unamrshal failed", "err", err)
		}
		logger.Info("updating mappings", "mappings", mappings)
	}, nil
}

func SelectAttributeUpdate() func(eh.Event) bool {
	return func(event eh.Event) bool {
		if event.EventType() == attr_prop.AttributeUpdated {
			return true
		}
		return false
	}
}
