package arbridge

import (
	"context"
	"fmt"
	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/go-redfish/domain"
	"strings"
)

const (
	ProcessAREventCommand eh.CommandType = "ProcessAREvent"
)

func SetupCommands(d domain.DDDFunctions) {
	eh.RegisterCommand(func() eh.Command { return &ProcessAREvent{} })
	d.GetAggregateCommandHandler().SetAggregate(domain.RedfishResourceAggregateType, ProcessAREventCommand)
	d.GetCommandBus().SetHandler(d.GetAggregateCommandHandler(), ProcessAREventCommand)
}

type ProcessAREvent struct {
	domain.RedfishResourceAggregateBaseCommand
	Name  string
	Value string
}

func (c ProcessAREvent) CommandType() eh.CommandType { return ProcessAREventCommand }
func (c ProcessAREvent) Handle(ctx context.Context, a *domain.RedfishResourceAggregate) error {
	// walk through all of the properties in this aggregate and see if any have a meta
	np := make(map[string]interface{})

	// TODO: need to recursively walk the properties. For now, just for testing only do one layer.
	for k, v := range a.Properties {
		if strings.HasSuffix(k, "@meta") {
			// check plugin type
			if meta, ok := v.(map[string]interface{}); ok {
				pluginType, ok := meta["plugin"]
				if ok && pluginType == "AR" {
					fmt.Printf("\tfound aggregate with matching @meta property: %s\n", k)
					data, ok := meta["data"]
					if !ok {
						continue
					}
					if c.Name == data {
						np[k[:len(k)-5]] = c.Value
					}
				}
			}
		}
	}

	a.StoreEvent(domain.RedfishResourcePropertiesUpdatedEvent,
		&domain.RedfishResourcePropertiesUpdatedData{
			Properties: np,
		},
	)

	return nil
}
