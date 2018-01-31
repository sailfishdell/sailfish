#!/bin/sh

CURLCMD="curl --cacert ./ca.crt"
prot=https
user=Administrator
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
        "ID": "49467bb4-5c1f-473b-af00-000000000001",
        "ResourceURI":"/redfish/v1/test",
        "Type": "footype",
        "Context": "foocontext",
        "Privileges": { "GET": ["Unauthenticated"] },
        "Properties": {
            "testvalue1": 41,
            "testvalue1@meta": {
                "GET": {
                    "plugin": "test:strategy3",
                    "args": "foobar1",
                    "cache": {
                        "min_age_ms": 10000,
                        "current_age_ms": 0
                    }
                }
            },

            "testvalue2": 42,
            "testvalue2@meta": {
                "GET": {
                    "plugin": "runcmd",
                    "CMD": "/bin/date",
                    "CMDARGS": ["+%Y-%m-%d %H:%M:%S"]
                }
            },

            "testvalue_invalid": 44,
            "testvalue_invalid@meta": { "GET": {"plugin": "test:invalid_plugin", "args": "foobar_invalid"} },

            "Actions": {
                "#Test.Action": {
                    "target": "/redfish/v1/Actions/Test",
                    "TestType@Redfish.AllowableValues": [
                        "TEST1",
                        "TEST2"
                    ]
                },
                "Oem": {},
                "Oem@meta": {"plugin": "nonexistent"}
            },
            "Name": "TEST"

            
        }
    }'

echo "/redfish/v1/test"
$CURLCMD $URL/redfish/v1/test


echo "Run a test action"
echo "/redfish/v1/Actions/Test"
$CURLCMD $URL/redfish/v1/Actions/Test -d '{"TestType": "FOO"}'


$CURLCMD $URL/redfish/v1/SessionService -XPATCH -d '{"SessionTimeout": 35}'

exit 1


