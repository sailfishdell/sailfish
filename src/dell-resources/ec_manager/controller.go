package ec_manager

import (
	"context"

	"github.com/superchalupa/go-redfish/src/log"
	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/utils"

	plugins "github.com/superchalupa/go-redfish/src/ocp"
    attr_prop "github.com/superchalupa/go-redfish/src/dell-resources/attribute-property"
)

func (s *service) AddController(ctx context.Context, ch eh.CommandHandler, eb eh.EventBus, ew *utils.EventWaiter) {

    type mapping struct {
        property string
        FQDD  string
        Group string
        Index string
        Name  string
    } 
    
    propMappings := []mapping{
        {property: "foo", FQDD: "CMC.Integrated.1", Group: "another_group", Index: "1", Name: "foo" },
        {property: "description", FQDD: "CMC.Integrated.1", Group: "another_group", Index: "1", Name: "foo" },
    }

	// stream processor for action events
	sp, err := plugins.NewEventStreamProcessor(ctx, ew, plugins.CustomFilter(SelectAttributeUpdate()))
	if err != nil {
		log.MustLogger("Managers/CMC.Integrated.1").Error("Failed to create event stream processor", "err", err)
		return
	}
	sp.RunForever(func(event eh.Event) {
		log.MustLogger("Managers/CMC.Integrated.1").Info("Got action event", "event", event)
		if data, ok := event.Data().(*attr_prop.AttributeUpdatedData); ok {
            for _, mapping := range(propMappings) {
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
                
			    s.UpdateProperty( mapping.property, data.Value )
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
