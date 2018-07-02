#!/bin/sh

curl  --noproxy '*' -X POST http://Administrator:password@localhost:8080/redfish/v1/TelemetryService/Actions/TelemetryService.SubmitTestMetricReport \
    -H 'content-type: application/json'  -d '{ "TEST DATA": "foobar" }' 
