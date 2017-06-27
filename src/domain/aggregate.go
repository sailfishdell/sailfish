// Copyright (c) 2014 - Max Ekman <max@looplab.se>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package domain

import (
	"context"
	"fmt"
	"log"

	eh "github.com/superchalupa/eventhorizon"
)

func init() {
	eh.RegisterAggregate(func(id eh.UUID) eh.Aggregate {
		return NewOdataAggregate(id)
	})
}

// OdataAggregateType is the type name of the aggregate.
const OdataAggregateType eh.AggregateType = "Odata"

// OdataAggregate is the root aggregate.
//
// The aggregate root will guard that the Odata can only be accepted OR
// declined, but not both.
type OdataAggregate struct {
	// AggregateBase implements most of the eventhorizon.Aggregate interface.
	*eh.AggregateBase

	Properties map[string]interface{}
}

// NewOdataAggregate creates a new OdataAggregate with an ID.
func NewOdataAggregate(id eh.UUID) *OdataAggregate {
	return &OdataAggregate{
		AggregateBase: eh.NewAggregateBase(OdataAggregateType, id),
        Properties: make(map[string]interface{}),
	}
}

// HandleCommand implements the HandleCommand method of the Aggregate interface.
func (a *OdataAggregate) HandleCommand(ctx context.Context, command eh.Command) error {
	switch command := command.(type) {
	case *CreateOdata:
		a.StoreEvent(OdataCreatedEvent,
			&OdataCreatedData{
				command.OdataURI,
			},
		)
		return nil

	case *UpdateProperty:
		a.StoreEvent(OdataPropertyChangedEvent,
			&OdataPropertyChangedData{command.PropertyName, command.PropertyValue},
		)
		return nil

	}
	return fmt.Errorf("couldn't handle command")
}

// ApplyEvent implements the ApplyEvent method of the Aggregate interface.
func (a *OdataAggregate) ApplyEvent(ctx context.Context, event eh.Event) error {
	switch event.EventType() {
	case OdataCreatedEvent:
		if data, ok := event.Data().(*OdataCreatedData); ok {
			a.Properties["@odata.id"] = data.id
		} else {
			log.Println("invalid event data type:", event.Data())
		}

	case OdataPropertyChangedEvent:
		if data, ok := event.Data().(*OdataPropertyChangedData); ok {
			a.Properties[data.PropertyName] = data.PropertyValue
		} else {
			log.Println("invalid event data type:", event.Data())
		}
	}
	return nil
}
