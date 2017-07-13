package domain

import (
	"log"
	"sync"

	eh "github.com/superchalupa/eventhorizon"
	"github.com/superchalupa/eventhorizon/eventhandler/projector"
	"github.com/superchalupa/eventhorizon/eventhandler/saga"
)

var dynamicCommands []eh.CommandType = []eh.CommandType{}
var dynamicCommandsMu sync.RWMutex

func RegisterDynamicCommand(cmd eh.CommandType) {
	dynamicCommandsMu.Lock()
	dynamicCommands = append(dynamicCommands, cmd)
	dynamicCommandsMu.Unlock()
}

// Setup configures the domain.
func Setup(ddd DDDFunctions) {
	SetupAggregate()
	SetupEvents()
	SetupCommands()
	SetupHTTP()

	// Add the logger as an observer.
	ddd.GetEventPublisher().AddObserver(&Logger{})

	// Create the aggregate repository.
	repository, err := eh.NewEventSourcingRepository(ddd.GetEventStore(), ddd.GetEventBus())
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
	handler.SetAggregate(RedfishResourceAggregateType, UpdateRedfishResourcePropertiesCommand)
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
		ddd.GetCommandBus().SetHandler(handler, c)
	}
	dynamicCommandsMu.RUnlock()

	// Create the command bus and register the handler for the commands.
	// WARNING: If you miss adding a handler for a command, then all command processesing stops when that command is emitted!
	ddd.GetCommandBus().SetHandler(handler, CreateRedfishResourceCommand)
	ddd.GetCommandBus().SetHandler(handler, UpdateRedfishResourcePropertiesCommand)
	ddd.GetCommandBus().SetHandler(handler, RemoveRedfishResourcePropertyCommand)
	ddd.GetCommandBus().SetHandler(handler, RemoveRedfishResourceCommand)

	ddd.GetCommandBus().SetHandler(handler, CreateRedfishResourceCollectionCommand)
	ddd.GetCommandBus().SetHandler(handler, AddRedfishResourceCollectionMemberCommand)
	ddd.GetCommandBus().SetHandler(handler, RemoveRedfishResourceCollectionMemberCommand)

	ddd.GetCommandBus().SetHandler(handler, UpdateRedfishResourcePrivilegesCommand)
	ddd.GetCommandBus().SetHandler(handler, UpdateRedfishResourcePermissionsCommand)
	ddd.GetCommandBus().SetHandler(handler, AddRedfishResourceHeaderCommand)
	ddd.GetCommandBus().SetHandler(handler, UpdateRedfishResourceHeaderCommand)
	ddd.GetCommandBus().SetHandler(handler, RemoveRedfishResourceHeaderCommand)

	// HTTP
	ddd.GetCommandBus().SetHandler(handler, HandleHTTPCommand)

	// read side projector
	redfishResourceProjector := projector.NewEventHandler(NewRedfishProjector(), ddd.GetReadWriteRepo())
	redfishResourceProjector.SetModel(func() interface{} { return &RedfishResource{} })
	ddd.GetEventBus().AddHandler(redfishResourceProjector, RedfishResourceCreatedEvent)
	ddd.GetEventBus().AddHandler(redfishResourceProjector, RedfishResourcePropertiesUpdatedEvent)
	ddd.GetEventBus().AddHandler(redfishResourceProjector, RedfishResourcePropertyRemovedEvent)
	ddd.GetEventBus().AddHandler(redfishResourceProjector, RedfishResourceRemovedEvent)
	ddd.GetEventBus().AddHandler(redfishResourceProjector, RedfishResourcePrivilegesUpdatedEvent)
	ddd.GetEventBus().AddHandler(redfishResourceProjector, RedfishResourcePermissionsUpdatedEvent)
	ddd.GetEventBus().AddHandler(redfishResourceProjector, RedfishResourceHeadersUpdatedEvent)
	ddd.GetEventBus().AddHandler(redfishResourceProjector, RedfishResourceHeaderRemovedEvent)

	// hook up tree rep. this guy maintains the redfish dictionary that maps
	// URIs to read side projector UUIDs
	redfishTreeProjector := NewRedfishTreeProjector(ddd.GetReadWriteRepo(), ddd.GetTreeID())
	ddd.GetEventBus().AddHandler(redfishTreeProjector, RedfishResourceCreatedEvent)
	ddd.GetEventBus().AddHandler(redfishTreeProjector, RedfishResourceRemovedEvent)

	// Hook up the saga that sets privileges on all redfish resources based on privilege map
	privilegeSaga := saga.NewEventHandler(NewPrivilegeSaga(ddd.GetReadRepo()), ddd.GetCommandBus())
	ddd.GetEventBus().AddHandler(privilegeSaga, RedfishResourceCreatedEvent)
}
