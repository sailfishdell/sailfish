#!/bin/sh

curl http://localhost:8080/api/createresource  -d '
    {
        "ID": "49467bb4-5c1f-473b-af00-5d4fa209f7bf",
        "ResourceURI":"/redfish/v1/test",
        "Type": "footype",
        "Context": "foocontext",
        "Properties": { "Name": "TEST" },
        "Privileges": { "GET": ["Unauthenticated"] }
    }'


echo "/redfish"
curl http://localhost:8080/redfish
echo "/redfish/"
curl http://localhost:8080/redfish/
echo "/redfish/v1"
curl http://localhost:8080/redfish/v1
echo "/redfish/v1/"
curl http://localhost:8080/redfish/v1/
echo "/redfish/v1/test"
curl http://root:password@localhost:8080/redfish/v1/test
