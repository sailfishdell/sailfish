#!/bin/sh

CURLCMD="curl --cacert ./ca.crt"
prot=https
user=root
pass=password
host=localhost
port=8443

URL=$proto://$user:$pass@$host

$CURLCMD http://localhost:8443/api/createresource  -d '
    {
        "ID": "49467bb4-5c1f-473b-af00-5d4fa209f7bf",
        "ResourceURI":"/redfish/v1/test",
        "Type": "footype",
        "Context": "foocontext",
        "Properties": { "Name": "TEST" },
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
