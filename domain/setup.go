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
	handler.SetAggregate(OdataAggregateType, CreateOdataCommand)
	handler.SetAggregate(OdataAggregateType, AddOdataPropertyCommand)
	handler.SetAggregate(OdataAggregateType, UpdateOdataPropertyCommand)
	handler.SetAggregate(OdataAggregateType, RemoveOdataPropertyCommand)
	handler.SetAggregate(OdataAggregateType, RemoveOdataCommand)

	// OdataCollection
	handler.SetAggregate(OdataAggregateType, CreateOdataCollectionCommand)
	handler.SetAggregate(OdataAggregateType, AddOdataCollectionMemberCommand)
	handler.SetAggregate(OdataAggregateType, RemoveOdataCollectionMemberCommand)

	// Create the command bus and register the handler for the commands.
	commandBus.SetHandler(handler, CreateOdataCommand)
	commandBus.SetHandler(handler, AddOdataPropertyCommand)
	commandBus.SetHandler(handler, UpdateOdataPropertyCommand)
	commandBus.SetHandler(handler, RemoveOdataPropertyCommand)
	commandBus.SetHandler(handler, RemoveOdataCommand)

	commandBus.SetHandler(handler, CreateOdataCollectionCommand)
	commandBus.SetHandler(handler, AddOdataCollectionMemberCommand)
	commandBus.SetHandler(handler, RemoveOdataCollectionMemberCommand)

	odataItemsProjector := projector.NewEventHandler(NewOdataProjector(), odataRepo)
	odataItemsProjector.SetModel(func() interface{} { return &OdataItem{} })
	eventBus.AddHandler(odataItemsProjector, OdataCreatedEvent)
	eventBus.AddHandler(odataItemsProjector, OdataPropertyAddedEvent)
	eventBus.AddHandler(odataItemsProjector, OdataPropertyUpdatedEvent)
	eventBus.AddHandler(odataItemsProjector, OdataPropertyRemovedEvent)
	eventBus.AddHandler(odataItemsProjector, OdataRemovedEvent)

	// hook up tree rep
	odataTreeProjector := NewOdataTreeProjector(odataRepo, treeID)
	eventBus.AddHandler(odataTreeProjector, OdataCreatedEvent)
	eventBus.AddHandler(odataTreeProjector, OdataPropertyAddedEvent)
	eventBus.AddHandler(odataTreeProjector, OdataPropertyUpdatedEvent)
	eventBus.AddHandler(odataTreeProjector, OdataPropertyRemovedEvent)
	eventBus.AddHandler(odataTreeProjector, OdataRemovedEvent)
}
