package domain

import (
	eh "github.com/looplab/eventhorizon"
)

const (
	WatchdogEvent                       = eh.EventType("Watchdog")
	RedfishResourceCreated              = eh.EventType("RedfishResource:created")
	RedfishResourceRemoved              = eh.EventType("RedfishResource:removed")
	HTTPCmdProcessed       eh.EventType = "HTTPCmdProcessed"

	RedfishResourcePropertiesUpdated   = eh.EventType("RedfishResourceProperty:updated")
	RedfishResourcePropertyMetaUpdated = eh.EventType("RedfishResourcePropertyMeta:updated")
)

func init() {
	eh.RegisterEventData(WatchdogEvent, func() eh.EventData {
		return &struct{}{}
	})
	eh.RegisterEventData(RedfishResourceCreated, func() eh.EventData {
		return &RedfishResourceCreatedData{}
	})
	eh.RegisterEventData(RedfishResourceRemoved, func() eh.EventData {
		return &RedfishResourceRemovedData{}
	})
	eh.RegisterEventData(RedfishResourcePropertiesUpdated, func() eh.EventData {
		return &RedfishResourcePropertiesUpdatedData{}
	})
	eh.RegisterEventData(RedfishResourcePropertyMetaUpdated, func() eh.EventData {
		return &RedfishResourcePropertyMetaUpdatedData{}
	})
	eh.RegisterEventData(HTTPCmdProcessed, func() eh.EventData { return &HTTPCmdProcessedData{} })
}

// RedfishResourceCreatedData is the event data for the RedfishResourceCreated event.
type RedfishResourceCreatedData struct {
	ID          eh.UUID `json:"id"     bson:"id"`
	ResourceURI string
}

// RedfishResourceRemovedData is the event data for the RedfishResourceRemoved event.
type RedfishResourceRemovedData struct {
	ID          eh.UUID `json:"id"     bson:"id"`
	ResourceURI string
}

type RedfishResourcePropertiesUpdatedData struct {
	ID            eh.UUID `json:"id"     bson:"id"`
	ResourceURI   string
	PropertyNames []string
}

type RedfishResourcePropertyMetaUpdatedData struct {
	ID          eh.UUID `json:"id"     bson:"id"`
	ResourceURI string
	Meta        map[string]interface{}
}

type HTTPCmdProcessedData struct {
	CommandID  eh.UUID
	Results    interface{}
	StatusCode int
	Headers    map[string]string
}
