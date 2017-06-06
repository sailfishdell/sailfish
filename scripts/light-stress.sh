#!/bin/sh

set -e

scriptdir=$(cd $(dirname $0); pwd)

while true
do
    for i in "" / /v1/ /v1/Systems /v1/Systems/437XR1138R2 /v1/Systems/dummy
    do
        $scriptdir/curl_timed.sh -o /dev/null http://localhost:8080/redfish${i}   
    done
done
