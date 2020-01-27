package eventservice

import (
	eh "github.com/looplab/eventhorizon"
)

const (
	RedfishEvent         = eh.EventType("RedfishEvent")
	ExternalRedfishEvent = eh.EventType("ExternalRedfishEvent")
	ExternalMetricEvent  = eh.EventType("ExternalMetricEvent")
)

func init() {
	eh.RegisterEventData(RedfishEvent, func() eh.EventData { return &RedfishEventData{} })
	eh.RegisterEventData(ExternalRedfishEvent, func() eh.EventData { return &ExternalRedfishEventData{} })
	eh.RegisterEventData(ExternalMetricEvent, func() eh.EventData { return &MetricReportData{} })
}

type MetricReportData struct {
	Data map[string]interface{}
}

type RedfishEventData struct {
	EventType         string
	EventId           string                 `json:",omitempty"`
	EventTimestamp    string                 `json:",omitempty"`
	Severity          string                 `json:",omitempty"`
	Message           string                 `json:",omitempty"`
	MessageId         string                 `json:",omitempty"`
	MessageArgs       []string               `json:",omitempty"`
	OriginOfCondition string                 `json:",omitempty"`
	Oem               map[string]interface{} `json:",omitempty"`
} //TODO MSM BUG: OriginOfCondition for events has to be a string or will be rejected

type ExternalRedfishEventData struct {
	Id      int    `json:",string"`
	Context string `json:"@odata.context"`
	Type    string `json:"@odata.type"`
	Name    string
	Events  []*RedfishEventData
	// The SSE endpoint will need to add the Context parameter on a per subscriber basis
}

/*
// Redfish events design

The redfish event service spec says, "The value of the "Id" property should be
a positive integer value and should be generated in a sequential manner."

This is slightly problematic because many places in the code might be
generating redfish events. Instead of having a counter that we must lock, what
we'll do is have an event listener that listens for RedfishEvent and generates
ExternalRedfishEvent. This will be a single threaded listener so it can create a
monotonically increasing counter.

We'll create this event stream processor with the event service.

So, in general, any part of the code can PublishEvent(RedfishEvent...), and the
event service stream processor will consume it and emit an output event.

Views can do the following:
    - Set up a "observer" on all of its models
    - When a model change happens, "ProcessMeta" on the aggregate, and then
      check the resulting object. We will mark which fields generate
      ResourceUpdated events in the @meta, and when we process the meta, we can
      leave litter behind to tell us if something changed.


*/

// SSE Example, for reference
/*

id: 1
data:{
data:    "@odata.context": "/redfish/v1/$metadata#Event.Event",
data:    "@odata.type": "#Event.v1_1_0.Event",
data:    "Id": "1",
data:    "Name": "Event Array",
data:    "Context": "ABCDEFGH",
data:    "Events": [
data:        {
data:            "MemberId": "1",
data:            "EventType": "Alert",
data:            "EventId": "ABC132489713478812346",
data:            "Severity": "Warning",
data:            "EventTimestamp": "2017-11-23T17:17:42-0600",
data:            "Message": "The LAN has been disconnected",
data:            "MessageId": "Alert.1.0.LanDisconnect",
data:            "MessageArgs": [
data:                "EthernetInterface 1",
data:                "/redfish/v1/Systems/1"
data:            ],
data:            "OriginOfCondition": {
data:                "@odata.id": "/redfish/v1/Systems/1/EthernetInterfaces/1"
data:            },
data:            "Context": "ABCDEFGH"
data:        }
data:    ]
data:}


*/
