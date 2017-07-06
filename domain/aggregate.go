package domain

import (
	"context"
	"fmt"

	eh "github.com/superchalupa/eventhorizon"
)

var _ = fmt.Println

func init() {
	eh.RegisterAggregate(func(id eh.UUID) eh.Aggregate {
		return NewOdataResourceAggregate(id)
	})
}

const OdataResourceAggregateType eh.AggregateType = "Odata"

type OdataResourceAggregate struct {
	// AggregateBase implements most of the eventhorizon.Aggregate interface.
	*eh.AggregateBase
	ResourceURI  string
	Properties   map[string]interface{}
	PrivilegeMap map[string]interface{}
	Permissions  map[string]interface{}
	Headers      map[string]string
	Methods      map[string]interface{}
}

func NewOdataResourceAggregate(id eh.UUID) *OdataResourceAggregate {
	return &OdataResourceAggregate{
		AggregateBase: eh.NewAggregateBase(OdataResourceAggregateType, id),
	}
}

type OACmdHandler interface {
	Handle(ctx context.Context, a *OdataResourceAggregate) error
}

func (a *OdataResourceAggregate) HandleCommand(ctx context.Context, command eh.Command) error {
	switch command := command.(type) {
	case OACmdHandler:
        fmt.Printf("Handle command OdataResourceAggregate: %t\n", command)
		return command.Handle(ctx, a)
	}

	return nil
}

func (a *OdataResourceAggregate) ApplyEvent(ctx context.Context, event eh.Event) error {
	switch event.EventType() {
	case OdataResourceCreatedEvent:
		if data, ok := event.Data().(*OdataResourceCreatedData); ok {
			a.ResourceURI = data.ResourceURI
			a.Properties = map[string]interface{}{}
			for k, v := range data.Properties {
				a.Properties[k] = v
			}
		}
	case OdataResourcePropertyAddedEvent:
		if data, ok := event.Data().(*OdataResourcePropertyAddedData); ok {
			a.Properties[data.PropertyName] = data.PropertyValue
		}
	case OdataResourcePropertyUpdatedEvent:
		if data, ok := event.Data().(*OdataResourcePropertyUpdatedData); ok {
			a.Properties[data.PropertyName] = data.PropertyValue
		}
	case OdataResourcePropertyRemovedEvent:
		if data, ok := event.Data().(*OdataResourcePropertyRemovedData); ok {
			delete(a.Properties, data.PropertyName)
		}
	case OdataResourceRemovedEvent:
		// no-op
	}

	return nil
}
