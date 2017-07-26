package domain

import ()

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
