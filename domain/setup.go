package domain

import (
	"log"

	eh "github.com/superchalupa/eventhorizon"
	"github.com/superchalupa/eventhorizon/eventhandler/projector"
)

// Setup configures the domain.
func Setup(
	eventStore eh.EventStore,
	eventBus eh.EventBus,
	eventPublisher eh.EventPublisher,
	commandBus eh.CommandBus,
	odataRepo eh.ReadWriteRepo,
	treeID eh.UUID) {

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

	// Odata
	handler.SetAggregate(OdataResourceAggregateType, CreateOdataResourceCommand)
	handler.SetAggregate(OdataResourceAggregateType, AddOdataResourcePropertyCommand)
	handler.SetAggregate(OdataResourceAggregateType, UpdateOdataResourcePropertyCommand)
	handler.SetAggregate(OdataResourceAggregateType, RemoveOdataResourcePropertyCommand)
	handler.SetAggregate(OdataResourceAggregateType, RemoveOdataResourceCommand)

	// OdataResourceCollection
	handler.SetAggregate(OdataResourceAggregateType, CreateOdataResourceCollectionCommand)
	handler.SetAggregate(OdataResourceAggregateType, AddOdataResourceCollectionMemberCommand)
	handler.SetAggregate(OdataResourceAggregateType, RemoveOdataResourceCollectionMemberCommand)

	// Create the command bus and register the handler for the commands.
	commandBus.SetHandler(handler, CreateOdataResourceCommand)
	commandBus.SetHandler(handler, AddOdataResourcePropertyCommand)
	commandBus.SetHandler(handler, UpdateOdataResourcePropertyCommand)
	commandBus.SetHandler(handler, RemoveOdataResourcePropertyCommand)
	commandBus.SetHandler(handler, RemoveOdataResourceCommand)

	commandBus.SetHandler(handler, CreateOdataResourceCollectionCommand)
	commandBus.SetHandler(handler, AddOdataResourceCollectionMemberCommand)
	commandBus.SetHandler(handler, RemoveOdataResourceCollectionMemberCommand)

	odataResourceProjector := projector.NewEventHandler(NewOdataProjector(), odataRepo)
	odataResourceProjector.SetModel(func() interface{} { return &OdataResource{} })
	eventBus.AddHandler(odataResourceProjector, OdataResourceCreatedEvent)
	eventBus.AddHandler(odataResourceProjector, OdataResourcePropertyAddedEvent)
	eventBus.AddHandler(odataResourceProjector, OdataResourcePropertyUpdatedEvent)
	eventBus.AddHandler(odataResourceProjector, OdataResourcePropertyRemovedEvent)
	eventBus.AddHandler(odataResourceProjector, OdataResourceRemovedEvent)

	// hook up tree rep
	odataTreeProjector := NewOdataTreeProjector(odataRepo, treeID)
	eventBus.AddHandler(odataTreeProjector, OdataResourceCreatedEvent)
	eventBus.AddHandler(odataTreeProjector, OdataResourcePropertyAddedEvent)
	eventBus.AddHandler(odataTreeProjector, OdataResourcePropertyUpdatedEvent)
	eventBus.AddHandler(odataTreeProjector, OdataResourcePropertyRemovedEvent)
	eventBus.AddHandler(odataTreeProjector, OdataResourceRemovedEvent)
}
