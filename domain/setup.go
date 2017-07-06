package domain

import (
	"log"

	eh "github.com/superchalupa/eventhorizon"
	"github.com/superchalupa/eventhorizon/eventhandler/projector"
	"github.com/superchalupa/eventhorizon/eventhandler/saga"
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

	handler.SetAggregate(OdataResourceAggregateType, UpdateOdataResourcePrivilegesCommand)
	handler.SetAggregate(OdataResourceAggregateType, UpdateOdataResourcePermissionsCommand)
	handler.SetAggregate(OdataResourceAggregateType, UpdateOdataResourceMethodsCommand)
	handler.SetAggregate(OdataResourceAggregateType, AddOdataResourceHeaderCommand)
	handler.SetAggregate(OdataResourceAggregateType, UpdateOdataResourceHeaderCommand)
	handler.SetAggregate(OdataResourceAggregateType, RemoveOdataResourceHeaderCommand)

	// Create the command bus and register the handler for the commands.
    // WARNING: If you miss adding a handler for a command, then all command processesing stops when that command is emitted!
	commandBus.SetHandler(handler, CreateOdataResourceCommand)
	commandBus.SetHandler(handler, AddOdataResourcePropertyCommand)
	commandBus.SetHandler(handler, UpdateOdataResourcePropertyCommand)
	commandBus.SetHandler(handler, RemoveOdataResourcePropertyCommand)
	commandBus.SetHandler(handler, RemoveOdataResourceCommand)

	commandBus.SetHandler(handler, CreateOdataResourceCollectionCommand)
	commandBus.SetHandler(handler, AddOdataResourceCollectionMemberCommand)
	commandBus.SetHandler(handler, RemoveOdataResourceCollectionMemberCommand)

	commandBus.SetHandler(handler, UpdateOdataResourcePrivilegesCommand)
	commandBus.SetHandler(handler, UpdateOdataResourcePermissionsCommand)
	commandBus.SetHandler(handler, UpdateOdataResourceMethodsCommand)
	commandBus.SetHandler(handler, AddOdataResourceHeaderCommand)
	commandBus.SetHandler(handler, UpdateOdataResourceHeaderCommand)
	commandBus.SetHandler(handler, RemoveOdataResourceHeaderCommand)

	odataResourceProjector := projector.NewEventHandler(NewOdataProjector(), odataRepo)
	odataResourceProjector.SetModel(func() interface{} { return &OdataResource{} })
	eventBus.AddHandler(odataResourceProjector, OdataResourceCreatedEvent)
	eventBus.AddHandler(odataResourceProjector, OdataResourcePropertyAddedEvent)
	eventBus.AddHandler(odataResourceProjector, OdataResourcePropertyUpdatedEvent)
	eventBus.AddHandler(odataResourceProjector, OdataResourcePropertyRemovedEvent)
	eventBus.AddHandler(odataResourceProjector, OdataResourceRemovedEvent)
	eventBus.AddHandler(odataResourceProjector, OdataResourcePrivilegesUpdatedEvent)
	eventBus.AddHandler(odataResourceProjector, OdataResourcePermissionsUpdatedEvent)
	eventBus.AddHandler(odataResourceProjector, OdataResourceMethodsUpdatedEvent)
	eventBus.AddHandler(odataResourceProjector, OdataResourceHeaderAddedEvent)
	eventBus.AddHandler(odataResourceProjector, OdataResourceHeaderUpdatedEvent)
	eventBus.AddHandler(odataResourceProjector, OdataResourceHeaderRemovedEvent)

	// hook up tree rep
	odataTreeProjector := NewOdataTreeProjector(odataRepo, treeID)
	eventBus.AddHandler(odataTreeProjector, OdataResourceCreatedEvent)
	eventBus.AddHandler(odataTreeProjector, OdataResourceRemovedEvent)

	privilegeSaga := saga.NewEventHandler(NewPrivilegeSaga(odataRepo), commandBus)
	eventBus.AddHandler(privilegeSaga, OdataResourceCreatedEvent)

}
