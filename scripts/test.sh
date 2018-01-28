#!/bin/sh

CURLCMD="curl --cacert ./ca.crt"
prot=https
user=root
pass=password
host=localhost
port=8443

URL=$prot://$user:$pass@$host:$port

echo "/redfish"
$CURLCMD $URL/redfish

echo "/redfish/"
$CURLCMD $URL/redfish/

echo "/redfish/v1"
$CURLCMD $URL/redfish/v1

echo "/redfish/v1/"
$CURLCMD $URL/redfish/v1/


echo "Test internal command API"
$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
        "ID": "49467bb4-5c1f-473b-af00-5d4fa209f7bf",
        "ResourceURI":"/redfish/v1/test",
        "Type": "footype",
        "Context": "foocontext",
        "Privileges": { "GET": ["Unauthenticated"] },
        "Properties": {
            "Name": "TEST",
            "testvalue1": 42,
            "testvalue1@meta": { "GET": {"plugin": "test:plugin", "args": "foobar1"} } ,
            "testvalue2": 42,
            "testvalue2@meta": { "GET": {"plugin": "test:plugin", "args": "foobar2"} } ,
            "testvalue3": 42,
            "testvalue3@meta": { "GET": {"plugin": "test:plugin", "args": "foobar3"} } ,
            "testvalue4": 42,
            "testvalue4@meta": { "GET": {"plugin": "test:plugin", "args": "foobar4"} } 
        }
    }'


echo "/redfish/v1/test"
$CURLCMD $URL/redfish/v1/test
