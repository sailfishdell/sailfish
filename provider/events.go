package provider

import (
	eh "github.com/looplab/eventhorizon"
)

//
// Reset Request
//
const ComputerSystemResetRequestEvent eh.EventType = "ComputerSystemResetRequest"

type ComputerSystemResetRequestData struct {
	CorrelationID eh.UUID
	ResetType     string
}
