#!/bin/sh

 curl --noproxy '*' http://Administrator:password@localhost:8080/redfish/v1/EventService/Subscriptions -d '{"Context":"BoB", "Destination": "http://localhost:1234/", "Name": "foobar", "Protocol": "Redfish" }'
