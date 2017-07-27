package provider

import (
    eh "github.com/looplab/eventhorizon"
)

const (
    ComputerSystemResetRequestEvent eh.EventType = "ComputerSystemResetRequest"
)

type ComputerSystemResetRequestData struct {
    ResetType   string
}
