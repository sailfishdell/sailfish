package arbridge

import (
	"context"
	"fmt"
	eh "github.com/looplab/eventhorizon"
	"github.com/superchalupa/go-redfish/domain"
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
	fmt.Printf("Running command to process AR Event for aggregate.\n")
	return nil
}
