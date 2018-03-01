#!/bin/sh

set -x
set -e

unset HTTP_PROXY HTTPS_PROXY http_proxy https_proxy

CURLCMD="curl --cacert ./ca.crt"
prot=${prot:-https}
user=${user:-Administrator}
pass=${pass:-password}
host=${host:-localhost}
port=${port:-8443}

URL=$prot://$user:$pass@$host:$port

echo "/redfish/v1"
$CURLCMD $URL/redfish/v1

$CURLCMD $URL/api/RedfishResourceProperties%3AUpdate  -d '
    {
        "ID": "49467bb4-5c1f-473b-af00-000000000001",
        "ResourceURI":"/redfish/v1/test",
        "Properties": { "NEWTHING": "NEWVALUE" }
    }'




