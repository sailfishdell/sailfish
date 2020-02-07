package domain

import (
	"fmt"
	eh "github.com/looplab/eventhorizon"
	"path"
)

const (
	RedfishResourceCreated              = eh.EventType("RedfishResource:created")
	RedfishResourceRemoved              = eh.EventType("RedfishResource:removed")
	HTTPCmdProcessed       eh.EventType = "HTTPCmdProcessed"

	RedfishResourcePropertiesUpdated   = eh.EventType("RedfishResourceProperty:updated")
	RedfishResourcePropertiesUpdated2  = eh.EventType("RedfishResourceProperty:updated2")
	RedfishResourcePropertyMetaUpdated = eh.EventType("RedfishResourcePropertyMeta:updated")
)

func init() {
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

type WC struct {
	Name string
	Values []string
}


type RedfishResourcePropertiesUpdatedData struct {
	ID            eh.UUID `json:"id"     bson:"id"`
	ResourceURI   string
	PropertyNames []string
}

// helper function to convert changes in aggregate format into "PropertyNames" format for RedfishResourcePropertiesUpdateData2
func Map2Path(pathMap interface{}, pathMapStr map[string]interface{}, pathStr string) {
	switch pathMapCast := pathMap.(type) {
	case map[string]interface{}:
		for keyS, vInterf := range pathMapCast {
			path2 := path.Join(pathStr, keyS)
			Map2Path(vInterf, pathMapStr, path2)
		}

	case []interface{}:
		for i, vInterf := range pathMapCast {
			path2 := fmt.Sprintf("%s/%d", pathStr, i)
			Map2Path(vInterf, pathMapStr, path2)
		}

	default:
		pathMapStr[pathStr] = pathMap
	}
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
