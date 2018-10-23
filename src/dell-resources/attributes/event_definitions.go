package attributes

import (
	eh "github.com/looplab/eventhorizon"
	domain "github.com/superchalupa/sailfish/src/redfishresource"
)

const (
	AttributeUpdated                eh.EventType = "AttributeUpdated"
	AttributeUpdateRequest          eh.EventType = "AttributeUpdateRequest"
	AttributeGetCurrentValueRequest eh.EventType = "AttributeGetCurrentValueRequest"
)

func init() {
	eh.RegisterEventData(AttributeUpdated, func() eh.EventData { return &AttributeUpdatedData{} })
	eh.RegisterEventData(AttributeUpdateRequest, func() eh.EventData { return &AttributeUpdateRequestData{} })
	eh.RegisterEventData(AttributeGetCurrentValueRequest, func() eh.EventData { return &AttributeGetCurrentValueRequestData{} })
}

type AttributeUpdatedData domain.AttributeUpdatedData

type AttributeUpdateRequestData struct {
	ReqID eh.UUID
	FQDD  string
	Group string
	Index string
	Name  string
	Value interface{}
}

type AttributeGetCurrentValueRequestData struct {
	FQDD  string
	Group string
	Index string
	Name  string
}
