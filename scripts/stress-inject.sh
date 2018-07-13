#!/bin/sh

set -x
set -e

datafile=$(mktemp /tmp/inject-XXXXXX)
trap 'rm -f $datafile' EXIT INT QUIT ERR

echo '{"name": "FanEvent", "data": {"rotor2rpm": '$RANDOM', "fanpwm_int": 19,"fanhealth": 2, "ObjectHeader": {"refreshInterval": 0, "objSize": 137,"objType": 3330, "objFlags": 8, "objStatus": 2, "Struct":"thp_fan_data_object", "FQDD": "System.Chassis.1#Fan.Slot.6"}, "fanStateMask":1, "Key": "System.Chassis.1#Fan.Slot.6", "VendorName": "", "FanName": "RearFan 2", "DeviceName": "System.Chassis.1#Fan.Slot.6", "TachName": "","numrotors": 2, "fanpwm": 19.0, "rotor1rpm": '$RANDOM', "warningThreshold": 1050,"criticalThreshold": 800}}' > $datafile

host=localhost
ab -k -n 1500 -c 15 -g p1_output_gp -e p1_output.csv -H "content-type: application/json" -p $datafile http://localhost:8080/api/Event%3AInject 
