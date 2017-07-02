package domain

import (
	"context"
	"fmt"

	eh "github.com/superchalupa/eventhorizon"
)

var _ = fmt.Println

func init() {
	eh.RegisterAggregate(func(id eh.UUID) eh.Aggregate {
		return NewOdataAggregate(id)
	})
}

const OdataAggregateType eh.AggregateType = "Odata"

type OdataAggregate struct {
	// AggregateBase implements most of the eventhorizon.Aggregate interface.
	*eh.AggregateBase
	OdataURI   string
	Properties map[string]interface{}
}

func NewOdataAggregate(id eh.UUID) *OdataAggregate {
	return &OdataAggregate{
		AggregateBase: eh.NewAggregateBase(OdataAggregateType, id),
	}
}

type OACmdHandler interface {
	Handle(ctx context.Context, a *OdataAggregate) error
}

func (a *OdataAggregate) HandleCommand(ctx context.Context, command eh.Command) error {
	switch command := command.(type) {
	case OACmdHandler:
		return command.Handle(ctx, a)
	}

	return nil
}

func (a *OdataAggregate) ApplyEvent(ctx context.Context, event eh.Event) error {
	switch event.EventType() {
	case OdataCreatedEvent:
		if data, ok := event.Data().(*OdataCreatedData); ok {
			a.OdataURI = data.OdataURI
			a.Properties = map[string]interface{}{}
			for k, v := range data.Properties {
				a.Properties[k] = v
			}
		}
	case OdataPropertyAddedEvent:
		if data, ok := event.Data().(*OdataPropertyAddedData); ok {
			a.Properties[data.PropertyName] = data.PropertyValue
		}
	case OdataPropertyUpdatedEvent:
		if data, ok := event.Data().(*OdataPropertyUpdatedData); ok {
			a.Properties[data.PropertyName] = data.PropertyValue
		}
	case OdataPropertyRemovedEvent:
		if data, ok := event.Data().(*OdataPropertyRemovedData); ok {
			delete(a.Properties, data.PropertyName)
		}
	case OdataRemovedEvent:
		// no-op
	}

	return nil
}
