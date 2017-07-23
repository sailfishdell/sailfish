package domain

import (
	"sync"

	eh "github.com/looplab/eventhorizon"
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
	SetupAggregate(ddd)
	SetupEvents(ddd)
	SetupCommands(ddd)
	SetupCollectionSaga(ddd)
	SetupHTTP(ddd)

	// read side projector
	SetupRedfishProjector(ddd)
	SetupRedfishTreeProjector(ddd)

	// sagas
	SetupPrivilegeSaga(ddd)
}
