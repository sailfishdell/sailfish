#!/bin/sh

CURLCMD="curl --cacert ./ca.crt"
prot=https
user=root
pass=password
host=localhost
port=8443

URL=$prot://$user:$pass@$host:$port

$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
        "ID": "49467bb4-5c1f-473b-af00-5d4fa209f7bf",
        "ResourceURI":"/redfish/v1/test",
        "Type": "footype",
        "Context": "foocontext",
        "Properties": { "Name": "TEST", "testvalue": 42, "testvalue@meta": {"plugin": "command", "plugin_args": "foobar"} },
        "Privileges": { "GET": ["Unauthenticated"] }
    }'

echo "/redfish"
$CURLCMD $URL/redfish

echo "/redfish/"
$CURLCMD $URL/redfish/

echo "/redfish/v1"
$CURLCMD $URL/redfish/v1

echo "/redfish/v1/"
$CURLCMD $URL/redfish/v1/

echo "/redfish/v1/test"
$CURLCMD $URL/redfish/v1/test
