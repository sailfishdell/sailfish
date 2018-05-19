package ec_manager

import (
	"context"

	"github.com/spf13/viper"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"
	"github.com/superchalupa/go-redfish/src/log"

	attr_prop "github.com/superchalupa/go-redfish/src/dell-resources/attribute-property"
	plugins "github.com/superchalupa/go-redfish/src/ocp"
)

func (s *service) UpdateMappings(cfg *viper.Viper, name string) {
	s.armappingsMu.Lock()
	defer s.armappingsMu.Unlock()

	k := cfg.Sub("mappings")
	log.MustLogger("Managers/CMC.Integrated.1").Info("debug viper", "viper", k, "name", name)
	log.MustLogger("Managers/CMC.Integrated.1").Info("get name", "viper", k.Get(name), "name", name)

	err := k.UnmarshalKey(name, &s.armappings)
	if err != nil {
		log.MustLogger("Managers/CMC.Integrated.1").Warn("unamrshal failed", "err", err)
	}
	log.MustLogger("Managers/CMC.Integrated.1").Info("updating mappings", "mappings", s.armappings)
}

func (s *service) AddController(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {
	// stream processor for action events
	sp, err := plugins.NewEventStreamProcessor(ctx, ew, plugins.CustomFilter(SelectAttributeUpdate()))
	if err != nil {
		log.MustLogger("Managers/CMC.Integrated.1").Error("Failed to create event stream processor", "err", err)
		return
	}
	sp.RunForever(func(event eh.Event) {
		log.MustLogger("Managers/CMC.Integrated.1").Info("Got action event", "event", event)
		if data, ok := event.Data().(*attr_prop.AttributeUpdatedData); ok {
			s.armappingsMu.RLock()
			defer s.armappingsMu.RUnlock()
			for _, mapping := range s.armappings {
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

				s.UpdateProperty(mapping.property, data.Value)
			}
		} else {
			log.MustLogger("Managers/CMC.Integrated.1").Warn("Should never happen: got an invalid event in the event handler")
		}
	})
}

func SelectAttributeUpdate() func(eh.Event) bool {
	return func(event eh.Event) bool {
		if event.EventType() == attr_prop.AttributeUpdated {
			return true
		}
		return false
	}
}
