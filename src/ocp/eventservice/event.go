package eventservice

import (
	eh "github.com/looplab/eventhorizon"
)

const (
	RedfishEvent = eh.EventType("RedfishEvent")
)

type RedfishEventData struct {
    EventType string
    EventId   string
    EventTimestamp  string
    Severity string
    Message  string
    MessageId string
    MessageArgs []string
    OriginOfCondition map[string]interface{}
}


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
