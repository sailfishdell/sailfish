package dell_resource

import (
	"context"
	"sync"
	"time"

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

type ARMappingController struct {
	mappings   []mapping
	mappingsMu sync.RWMutex
	logger     log.Logger
	name       string

	eb eh.EventBus
}

func NewARMappingController(ctx context.Context, logger log.Logger, m *model.Model, name string, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) (*ARMappingController, error) {
	c := &ARMappingController{
		mappings: []mapping{},
		name:     name,
		logger:   logger,
		eb:       eb,
	}

	// stream processor for action events
	sp, err := event.NewEventStreamProcessor(ctx, ew, event.CustomFilter(SelectAttributeUpdate()))
	if err != nil {
		logger.Error("Failed to create event stream processor", "err", err)
		return nil, err
	}
	sp.RunForever(func(event eh.Event) {
		if data, ok := event.Data().(*attr_prop.AttributeUpdatedData); ok {
			c.mappingsMu.RLock()
			defer c.mappingsMu.RUnlock()
			logger.Debug("Process Event", "data", data)
			for _, mapping := range c.mappings {
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
				m.UpdateProperty(mapping.Property, data.Value)
			}
		} else {
			logger.Warn("Should never happen: got an invalid event in the event handler")
		}
	})

	return c, nil
}

func (c *ARMappingController) UpdateProperty(property string, value interface{}) {
	c.logger.Crit("GOT IT!")
}

// this is the function that viper will call whenever the configuration changes at runtime
func (c *ARMappingController) ConfigChangedFn(ctx context.Context, cfg *viper.Viper) {
	c.mappingsMu.Lock()
	defer c.mappingsMu.Unlock()

	k := cfg.Sub("mappings")
	if k == nil {
		c.logger.Warn("missing config file section: 'mappings'")
		return
	}
	err := k.UnmarshalKey(c.name, &c.mappings)
	if err != nil {
		c.logger.Warn("unamrshal failed", "err", err)
	}
	c.logger.Info("updating mappings", "mappings", c.mappings)

	go c.requestUpdates(ctx)
}

//
// background thread that sends messages to the data pump to ask for startup values
//
func (c *ARMappingController) requestUpdates(ctx context.Context) {
	for {
		for _, m := range c.mappings {
			c.logger.Crit("SENDING ATTRIBUTE REQUEST", "mapping", m)
			data := attr_prop.AttributeGetCurrentValueRequestData{
				FQDD:  m.FQDD,
				Group: m.Group,
				Index: m.Index,
				Name:  m.Name,
			}
			c.eb.PublishEvent(ctx, eh.NewEvent(attr_prop.AttributeGetCurrentValueRequest, data, time.Now()))
		}
		break
	}
}

func SelectAttributeUpdate() func(eh.Event) bool {
	return func(event eh.Event) bool {
		if event.EventType() == attr_prop.AttributeUpdated {
			return true
		}
		return false
	}
}
