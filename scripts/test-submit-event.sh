#!/bin/sh

curl  --noproxy '*' -X POST http://Administrator:password@localhost:8080/redfish/v1/EventService/Actions/EventService.SubmitTestEvent \
    -H 'content-type: application/json'  -d '{
    "EventType": "Alert",
    "EventId": "SomeEventString",
    "EventTimestamp": "2018-05-04T04:14:33",
    "Severity": "Warning",
    "Message": "The LAN has been disconnected",
    "MessageId": "Alert.1.0.LanDisconnect",
    "MessageArgs": [ "EthernetInterface 1", "/redfish/v1/Systems/1" ],
    "OriginOfCondition": "/redfish/v1/Systems/1/EthernetInterfaces/1"
}' 
