#!/bin/sh

set -x
set -e
# new default 8080 port for this for speed
port=${port:-8080}

scriptdir=$(cd $(dirname $0); pwd)
. ${scriptdir}/common-vars.sh

timelimit=${timelimit:-20}
concurrent=${concurrent:-5}

datafile=$(mktemp /tmp/inject-XXXXXX)
trap 'rm -f $datafile' EXIT INT QUIT ERR

echo '{"name": "FanEvent", "data": {"rotor2rpm": '$RANDOM', "fanpwm_int": 19,"fanhealth": 2, "ObjectHeader": {"refreshInterval": 0, "objSize": 137,"objType": 3330, "objFlags": 8, "objStatus": 2, "Struct":"thp_fan_data_object", "FQDD": "System.Chassis.1#Fan.Slot.6"}, "fanStateMask":1, "Key": "System.Chassis.1#Fan.Slot.6", "VendorName": "", "FanName": "RearFan 2", "DeviceName": "System.Chassis.1#Fan.Slot.6", "TachName": "","numrotors": 2, "fanpwm": 19.0, "rotor1rpm": '$RANDOM', "warningThreshold": 1050,"criticalThreshold": 800}}' > $datafile

$CURLCMD --fail -H "content-type: application/json" $BASE/api/Event%3AInject -d @${datafile}
ab -k -n 1500 -t ${timelimit} -c ${concurrent}  -H "content-type: application/json" -p $datafile $BASE/api/Event%3AInject
