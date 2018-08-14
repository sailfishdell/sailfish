package ar_mapper

import (
	"context"
	"sync"
	"time"

	"github.com/spf13/viper"

	eh "github.com/looplab/eventhorizon"

	"github.com/superchalupa/go-redfish/src/log"
	"github.com/superchalupa/go-redfish/src/ocp/event"
	"github.com/superchalupa/go-redfish/src/ocp/model"

	"github.com/superchalupa/go-redfish/src/dell-resources/attributes"
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
	mdl        *model.Model

        requestedFQDD  string
        requestedGroup string
        requestedIndex string

	eb eh.EventBus
}

func New(ctx context.Context, logger log.Logger, m *model.Model, name string, fqdd string, group string, index string, ch eh.CommandHandler, eb eh.EventBus) (*ARMappingController, error) {
	c := &ARMappingController{
		mappings:       []mapping{},
		name:           name,
		logger:         logger,
		eb:             eb,
		mdl:            m,
		requestedFQDD:  fqdd,
		requestedGroup: group,
		requestedIndex:	index,
	}

	// stream processor for action events
	sp, err := event.NewESP(ctx, event.CustomFilter(selectAttributeUpdate()), event.SetListenerName("ar_mapper"))
	if err != nil {
		logger.Error("Failed to create event stream processor", "err", err)
		return nil, err
	}
	go sp.RunForever(func(event eh.Event) {
		fn := func(data *attributes.AttributeUpdatedData) {
			for _, mapping := range c.mappings {
				if data.Name != mapping.Name {
					continue
				}
				if data.Group != mapping.Group {
					// Check for group wildcard
					if mapping.Group != "{GROUP}" {
						continue
					}
					if data.Group != group {
						continue
					}
				}
				if data.Index != mapping.Index {
					// Check for index wildcard
					if mapping.Index != "{INDEX}" {
						continue
					}
					if data.Index != index {
						continue
					}
				}
				// check for direct fqdd match first
				if data.FQDD != mapping.FQDD {
					// Check for FQDD wildcard
					// mapping FQDD field equal to "{FQDD}" means use wildcard match
					if mapping.FQDD == "{FQDD}" {
						// if we get here, fqdd is wildcard, check against our passed in fqdd
						if data.FQDD != fqdd {
							continue
						}
					} else if mapping.FQDD != "{ANY}" {
						continue;
					}
				}

				logger.Info("Updating Model", "mapping", mapping, "property", mapping.Property, "data", data)
				m.UpdateProperty(mapping.Property, data.Value)
			}
		}

		if arr, ok := event.Data().(*attributes.AttributeArrayUpdatedData); ok {
			for _, data := range arr.Attributes {
				fn(&data)
			}
		} else if data, ok := event.Data().(*attributes.AttributeUpdatedData); ok {
			fn(data)
		} else {
			logger.Warn("Should never happen: got an invalid event in the event handler")
		}
	})

	return c, nil
}

func (c *ARMappingController) UpdateRequest(ctx context.Context, property string, value interface{}) (interface{}, error) {
	for _, mapping := range c.mappings {
		if property != mapping.Property {
			continue
		}

		c.logger.Info("Sending Update Request", "mapping", mapping, "value", value)
		reqUUID := eh.NewUUID()

		data := &attributes.AttributeUpdateRequestData{
			ReqID: reqUUID,
			FQDD:  mapping.FQDD,
			Group: mapping.Group,
			Index: mapping.Index,
			Name:  mapping.Name,
			Value: value,
		}
		c.eb.PublishEvent(ctx, eh.NewEvent(attributes.AttributeUpdateRequest, data, time.Now()))

		break
	}

	// TODO: wait for event to come back matching request

	return value, nil
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
		c.logger.Warn("unmarshal failed", "err", err)
	}

	for i, m := range c.mappings {
		if m.FQDD == "{FQDD}" {
			c.mappings[i].FQDD = c.requestedFQDD
			c.logger.Debug("Replacing {FQDD} with real fqdd", "fqdd", m.FQDD)
		}
		if m.Group == "{GROUP}" {
			c.mappings[i].Group = c.requestedGroup
			c.logger.Debug("Replacing {GROUP} with real group", "group", m.Group)
		}
		if m.Index == "{INDEX}" {
			c.mappings[i].Index = c.requestedIndex
			c.logger.Debug("Replacing {INDEX} with real index", "index", m.Index)
		}
	}

	c.logger.Info("updating mappings", "mappings", c.mappings)
	c.createModelProperties(ctx)
	go c.initialStartupBootstrap(ctx)
}

func (c *ARMappingController) createModelProperties(ctx context.Context) {
	for _, m := range c.mappings {
		if _, ok := c.mdl.GetPropertyOkUnlocked(m.Property); !ok {
			c.logger.Info("Model property does not exist, creating: "+m.Property, "property", m.Property, "FQDD", m.FQDD)
			c.mdl.UpdateProperty(m.Property, nil)
		}
	}
}

//
// background thread that sends messages to the data pump to ask for startup values
//
func (c *ARMappingController) initialStartupBootstrap(ctx context.Context) {
	// bypass for now
	return

	for {
		time.Sleep(120 * time.Second)
		for _, m := range c.mappings {
			c.logger.Info("SENDING ATTRIBUTE REQUEST", "mapping", m)
			data := &attributes.AttributeGetCurrentValueRequestData{
				FQDD:  m.FQDD,
				Group: m.Group,
				Index: m.Index,
				Name:  m.Name,
			}
			c.eb.PublishEvent(ctx, eh.NewEvent(attributes.AttributeGetCurrentValueRequest, data, time.Now()))
		}
	}
}

func selectAttributeUpdate() func(eh.Event) bool {
	return func(event eh.Event) bool {
		if event.EventType() == attributes.AttributeUpdated {
			return true
		}
		if event.EventType() == attributes.AttributeArrayUpdated {
			return true
		}
		return false
	}
}
