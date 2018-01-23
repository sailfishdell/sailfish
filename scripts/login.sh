#!/bin/bash

set -e

username=$1
password=$2
protocol=https
host=localhost
port=8443
headersfile=$(mktemp /tmp/headers-XXXXXX)
trap 'rm $headersfile' EXIT QUIT HUP INT ERR

RESPONSE_HEADERS=$(curl --cacert ./ca.crt -D${headersfile} ${protocol}://${host}:${port}/redfish/v1/SessionService/Sessions -X POST -d "{\"UserName\": \"${username}\", \"Password\": \"${password}\"}" 2>&1)
X_AUTH_TOKEN=$(cat ${headersfile} | grep X-Auth-Token | cut -d: -f2 | perl -p -e 's/\r//g;')
SESSION_URI=$(cat ${headersfile} | grep Location | cut -d: -f2 | perl -p -e 's/\r//g;')

for i in $X_AUTH_TOKEN
do
    export X_AUTH_TOKEN=$i
    break
done

for i in $SESSION_URI
do
    export SESSION_URI=$i
    break
done

export AUTH_HEADER="X-Auth-Token: $X_AUTH_TOKEN"

if [ -n "$X_AUTH_TOKEN" ]; then
    echo "export X_AUTH_TOKEN=$X_AUTH_TOKEN"
    echo "export AUTH_HEADER='X-Auth-Token: $X_AUTH_TOKEN'"
    echo "export SESSION_URI=$SESSION_URI"
else
    echo "export X_AUTH_TOKEN="
    echo "export AUTH_HEADER="
    echo "export SESSION_URI="
fi

rm $headersfile
trap - EXIT QUIT HUP INT ERR
