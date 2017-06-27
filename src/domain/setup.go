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
	"log"

	eh "github.com/superchalupa/eventhorizon"
	"github.com/superchalupa/eventhorizon/eventhandler/projector"
	// "github.com/superchalupa/eventhorizon/eventhandler/saga"
)

// Setup configures the domain.
func Setup(
	eventStore eh.EventStore,
	eventBus eh.EventBus,
	eventPublisher eh.EventPublisher,
	commandBus eh.CommandBus,
	odataRepo eh.ReadWriteRepo,
	eventID eh.UUID) {

	// Add the logger as an observer.
	eventPublisher.AddObserver(&Logger{})

	// Create the aggregate repository.
	repository, err := eh.NewEventSourcingRepository(eventStore, eventBus)
	if err != nil {
		log.Fatalf("could not create repository: %s", err)
	}

	// Create the aggregate command handler.
	handler, err := eh.NewAggregateCommandHandler(repository)
	if err != nil {
		log.Fatalf("could not create command handler: %s", err)
	}

	// Register the domain aggregates with the dispather. Remember to check for
	// errors here in a real app!
	handler.SetAggregate(OdataAggregateType, CreateOdataCommand)
	handler.SetAggregate(OdataAggregateType, UpdatePropertyCommand)

	// Create the command bus and register the handler for the commands.
	commandBus.SetHandler(handler, CreateOdataCommand)
	commandBus.SetHandler(handler, UpdatePropertyCommand)

	// Create and register a read model for individual invitations.
	odataProjector := projector.NewEventHandler(
		NewOdataProjector(), odataRepo)
	odataProjector.SetModel(func() interface{} { return &Odata{} })
	eventBus.AddHandler(odataProjector, OdataCreatedEvent)
	eventBus.AddHandler(odataProjector, OdataPropertyChangedEvent)

}
