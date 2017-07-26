package domain

import (
	eh "github.com/looplab/eventhorizon"
)

const (
	// EVENTS
	HTTPCmdProcessedEvent eh.EventType = "HTTPCmdProcessed"
)

type HTTPCmdProcessedData struct {
	CommandID  eh.UUID
	Results    map[string]interface{}
	StatusCode int
	Headers    map[string]string
}

// END Events
//*****************************************************************************

func SetupHTTP(DDDFunctions) {
	// EVENT registration
	eh.RegisterEventData(HTTPCmdProcessedEvent, func() eh.EventData { return &HTTPCmdProcessedData{} })
}
