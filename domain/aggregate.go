package domain

import (
	"context"
	"fmt"

	eh "github.com/superchalupa/eventhorizon"
)

var _ = fmt.Println

func init() {
	eh.RegisterAggregate(func(id eh.UUID) eh.Aggregate {
		return NewRedfishResourceAggregate(id)
	})
}

const RedfishResourceAggregateType eh.AggregateType = "RedfishResource"

type RedfishResourceAggregate struct {
	// AggregateBase implements most of the eventhorizon.Aggregate interface.
	*eh.AggregateBase
	ResourceURI  string
	Properties   map[string]interface{}
	PrivilegeMap map[string]interface{}
	Permissions  map[string]interface{}
	Headers      map[string]string
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
		fmt.Printf("Handle command RedfishResourceAggregate: %t\n", command)
		return command.Handle(ctx, a)
	}

	return nil
}

func (a *RedfishResourceAggregate) ApplyEvent(ctx context.Context, event eh.Event) error {
	switch event.EventType() {
	case RedfishResourceCreatedEvent:
		if data, ok := event.Data().(*RedfishResourceCreatedData); ok {
			a.ResourceURI = data.ResourceURI
			a.Properties = map[string]interface{}{}
			for k, v := range data.Properties {
				a.Properties[k] = v
			}
		}
	case RedfishResourcePropertyAddedEvent:
		if data, ok := event.Data().(*RedfishResourcePropertyAddedData); ok {
			a.Properties[data.PropertyName] = data.PropertyValue
		}
	case RedfishResourcePropertyUpdatedEvent:
		if data, ok := event.Data().(*RedfishResourcePropertyUpdatedData); ok {
			a.Properties[data.PropertyName] = data.PropertyValue
		}
	case RedfishResourcePropertyRemovedEvent:
		if data, ok := event.Data().(*RedfishResourcePropertyRemovedData); ok {
			delete(a.Properties, data.PropertyName)
		}
	case RedfishResourceRemovedEvent:
		// no-op
	}

	return nil
}
