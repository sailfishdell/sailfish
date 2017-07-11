package domain

import (
	"log"
	"sync"

	eh "github.com/superchalupa/eventhorizon"
	"github.com/superchalupa/eventhorizon/eventhandler/projector"
	"github.com/superchalupa/eventhorizon/eventhandler/saga"
	"github.com/superchalupa/eventhorizon/utils"
)

var dynamicCommands []eh.CommandType = []eh.CommandType{}
var dynamicCommandsMu sync.RWMutex

func RegisterDynamicCommand(cmd eh.CommandType) {
	dynamicCommandsMu.Lock()
	dynamicCommands = append(dynamicCommands, cmd)
	dynamicCommandsMu.Unlock()
}

// Setup configures the domain.
func Setup(
	eventStore eh.EventStore,
	eventBus eh.EventBus,
	eventPublisher eh.EventPublisher,
	commandBus eh.CommandBus,
	redfishRepo eh.ReadWriteRepo,
	treeID eh.UUID) (waiter *utils.EventWaiter) {

	SetupAggregate()
	SetupEvents()
	SetupCommands()
	SetupHTTP()

	// Add the logger as an observer.
	waiter = utils.NewEventWaiter()
	eventPublisher.AddObserver(&Logger{})
	eventPublisher.AddObserver(waiter)

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

	// redfish
	handler.SetAggregate(RedfishResourceAggregateType, CreateRedfishResourceCommand)
	handler.SetAggregate(RedfishResourceAggregateType, AddRedfishResourcePropertyCommand)
	handler.SetAggregate(RedfishResourceAggregateType, UpdateRedfishResourcePropertyCommand)
	handler.SetAggregate(RedfishResourceAggregateType, RemoveRedfishResourcePropertyCommand)
	handler.SetAggregate(RedfishResourceAggregateType, RemoveRedfishResourceCommand)

	// RedfishResourceCollection
	handler.SetAggregate(RedfishResourceAggregateType, CreateRedfishResourceCollectionCommand)
	handler.SetAggregate(RedfishResourceAggregateType, AddRedfishResourceCollectionMemberCommand)
	handler.SetAggregate(RedfishResourceAggregateType, RemoveRedfishResourceCollectionMemberCommand)

	handler.SetAggregate(RedfishResourceAggregateType, UpdateRedfishResourcePrivilegesCommand)
	handler.SetAggregate(RedfishResourceAggregateType, UpdateRedfishResourcePermissionsCommand)
	handler.SetAggregate(RedfishResourceAggregateType, AddRedfishResourceHeaderCommand)
	handler.SetAggregate(RedfishResourceAggregateType, UpdateRedfishResourceHeaderCommand)
	handler.SetAggregate(RedfishResourceAggregateType, RemoveRedfishResourceHeaderCommand)

	// HTTP commands...
	handler.SetAggregate(RedfishResourceAggregateType, HandleHTTPCommand)

	dynamicCommandsMu.RLock()
	for _, c := range dynamicCommands {
		handler.SetAggregate(RedfishResourceAggregateType, c)
		commandBus.SetHandler(handler, c)
	}
	dynamicCommandsMu.RUnlock()

	// Create the command bus and register the handler for the commands.
	// WARNING: If you miss adding a handler for a command, then all command processesing stops when that command is emitted!
	commandBus.SetHandler(handler, CreateRedfishResourceCommand)
	commandBus.SetHandler(handler, AddRedfishResourcePropertyCommand)
	commandBus.SetHandler(handler, UpdateRedfishResourcePropertyCommand)
	commandBus.SetHandler(handler, RemoveRedfishResourcePropertyCommand)
	commandBus.SetHandler(handler, RemoveRedfishResourceCommand)

	commandBus.SetHandler(handler, CreateRedfishResourceCollectionCommand)
	commandBus.SetHandler(handler, AddRedfishResourceCollectionMemberCommand)
	commandBus.SetHandler(handler, RemoveRedfishResourceCollectionMemberCommand)

	commandBus.SetHandler(handler, UpdateRedfishResourcePrivilegesCommand)
	commandBus.SetHandler(handler, UpdateRedfishResourcePermissionsCommand)
	commandBus.SetHandler(handler, AddRedfishResourceHeaderCommand)
	commandBus.SetHandler(handler, UpdateRedfishResourceHeaderCommand)
	commandBus.SetHandler(handler, RemoveRedfishResourceHeaderCommand)

	// HTTP
	commandBus.SetHandler(handler, HandleHTTPCommand)

	// read side projector
	redfishResourceProjector := projector.NewEventHandler(NewRedfishProjector(), redfishRepo)
	redfishResourceProjector.SetModel(func() interface{} { return &RedfishResource{} })
	eventBus.AddHandler(redfishResourceProjector, RedfishResourceCreatedEvent)
	eventBus.AddHandler(redfishResourceProjector, RedfishResourcePropertyAddedEvent)
	eventBus.AddHandler(redfishResourceProjector, RedfishResourcePropertyUpdatedEvent)
	eventBus.AddHandler(redfishResourceProjector, RedfishResourcePropertyRemovedEvent)
	eventBus.AddHandler(redfishResourceProjector, RedfishResourceRemovedEvent)
	eventBus.AddHandler(redfishResourceProjector, RedfishResourcePrivilegesUpdatedEvent)
	eventBus.AddHandler(redfishResourceProjector, RedfishResourcePermissionsUpdatedEvent)
	eventBus.AddHandler(redfishResourceProjector, RedfishResourceHeaderAddedEvent)
	eventBus.AddHandler(redfishResourceProjector, RedfishResourceHeaderUpdatedEvent)
	eventBus.AddHandler(redfishResourceProjector, RedfishResourceHeaderRemovedEvent)

	// hook up tree rep. this guy maintains the redfish dictionary that maps
	// URIs to read side projector UUIDs
	redfishTreeProjector := NewRedfishTreeProjector(redfishRepo, treeID)
	eventBus.AddHandler(redfishTreeProjector, RedfishResourceCreatedEvent)
	eventBus.AddHandler(redfishTreeProjector, RedfishResourceRemovedEvent)

	// Hook up the saga that sets privileges on all redfish resources based on privilege map
	privilegeSaga := saga.NewEventHandler(NewPrivilegeSaga(redfishRepo), commandBus)
	eventBus.AddHandler(privilegeSaga, RedfishResourceCreatedEvent)

	return
}
