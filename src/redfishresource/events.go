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
	RedfishResourcePropertiesUpdated2  = eh.EventType("RedfishResourceProperty:updated2")
	RedfishResourcePropertyMetaUpdated = eh.EventType("RedfishResourcePropertyMeta:updated")

	DroppedEvent = eh.EventType("DroppedEvent")
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

	eh.RegisterEventData(RedfishResourcePropertiesUpdated2, func() eh.EventData {
		return &RedfishResourcePropertiesUpdatedData2{}
	})

	eh.RegisterEventData(RedfishResourcePropertyMetaUpdated, func() eh.EventData {
		return &RedfishResourcePropertyMetaUpdatedData{}
	})
	eh.RegisterEventData(HTTPCmdProcessed, func() eh.EventData { return &HTTPCmdProcessedData{} })
	eh.RegisterEventData(DroppedEvent, func() eh.EventData { return &DroppedEventData{} })
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

type RedfishResourcePropertiesUpdatedData2 struct {
	ID            eh.UUID `json:"id"     bson:"id"`
	ResourceURI   string
	PropertyNames map[string]interface{}
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

type DroppedEventData struct {
	Name     eh.EventType
	EventSeq int64
}
