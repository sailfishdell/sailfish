package domain

import (
	"context"

	eh "github.com/looplab/eventhorizon"
)

func SetupAggregate(DDDFunctions) {
	eh.RegisterAggregate(func(id eh.UUID) eh.Aggregate {
		return NewRedfishResourceAggregate(id)
	})
}

const RedfishResourceAggregateType eh.AggregateType = "RedfishResource"

type RedfishResourceAggregate struct {
	// AggregateBase implements most of the eventhorizon.Aggregate interface.
	*eh.AggregateBase
	TreeID       eh.UUID
	ResourceURI  string
	Properties   map[string]interface{}
	PrivilegeMap map[string]interface{}
	Permissions  map[string]interface{}
	Headers      map[string]string
	Private      map[string]interface{}
}

func NewRedfishResourceAggregate(id eh.UUID) *RedfishResourceAggregate {
	return &RedfishResourceAggregate{
		AggregateBase: eh.NewAggregateBase(RedfishResourceAggregateType, id),
	}
}

type RRCmdHandler interface {
	Handle(ctx context.Context, a *RedfishResourceAggregate) error
}

func (a *RedfishResourceAggregate) HandleCommand(ctx context.Context, command eh.Command) error {
	switch command := command.(type) {
	case RRCmdHandler:
		return command.Handle(ctx, a)
	}

	return nil
}

type RREvtApplier interface {
	ApplyToAggregate(context.Context, *RedfishResourceAggregate, eh.Event) error
}

func (a *RedfishResourceAggregate) ApplyEvent(ctx context.Context, event eh.Event) error {
	d := event.Data()
	switch d := d.(type) {
	case RREvtApplier:
		return d.ApplyToAggregate(ctx, a, event)
	}

	return nil
}
