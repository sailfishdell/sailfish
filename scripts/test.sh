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

echo "/redfish"
$CURLCMD $URL/redfish

echo "/redfish/"
$CURLCMD $URL/redfish/

echo "/redfish/v1"
$CURLCMD $URL/redfish/v1

echo "/redfish/v1/"
$CURLCMD $URL/redfish/v1/

$CURLCMD $URL/api/RedfishResource%3ARemove  -d '
    {
        "ID": "49467bb4-5c1f-473b-af00-000000000001",
        "ResourceURI":"/redfish/v1/test"
    }'

echo "Test internal command API"
$CURLCMD $URL/api/RedfishResource%3ACreate  -d '
    {
        "ID": "49467bb4-5c1f-473b-af00-000000000001",
        "ResourceURI":"/redfish/v1/test",
        "Type": "footype",
        "Context": "foocontext",
        "Privileges": { "GET": ["Unauthenticated"], "PATCH":["ConfigureManager"] },
        "Properties": {
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

            "testvalue2@meta": {
                "GET": {
                    "plugin": "runcmd",
                    "CMD": "/bin/date",
                    "CMDARGS": ["+%Y-%m-%d %H:%M:%S"]
                }
            },

            "testvalue3": {
                "testvalue_embed@meta": {
                    "GET": {
                        "plugin": "runcmd",
                        "CMD": "/bin/date",
                        "CMDARGS": ["+%Y-%m-%d %H:%M:%S"]
                    }
                }
            },

            "testarray": [
                { "foobar@meta": {
                    "GET": {
                        "plugin": "runcmd",
                        "CMD": "/bin/date",
                        "CMDARGS": ["+%Y-%m-%d %H:%M:%S"]
                    }}},
                { "fofo@meta": {
                    "GET": {
                        "plugin": "runcmd",
                        "CMD": "/bin/date",
                        "CMDARGS": ["+%Y-%m-%d %H:%M:%S"]
                    }}},
                {"foobar1@meta": {
                    "GET": {
                        "plugin": "runcmd",
                        "CMD": "/bin/sleep",
                        "CMDARGS": ["1"]
                    }}},
                {"foobar1@meta": {
                    "GET": {
                        "plugin": "runcmd",
                        "CMD": "/bin/sleep",
                        "CMDARGS": ["1"]
                    }}},
                {"foobar1@meta": {
                    "GET": {
                        "plugin": "runcmd",
                        "CMD": "/bin/sleep",
                        "CMDARGS": ["1"]
                    }}},
                {"foobar1@meta": {
                    "GET": {
                        "plugin": "runcmd",
                        "CMD": "/bin/sleep",
                        "CMDARGS": ["1"]
                    }}},
                { "foobar@meta": {
                    "GET": {
                        "plugin": "runcmd",
                        "CMD": "/bin/sleep",
                        "CMDARGS": ["1"]
                    }}}
            ],

            "foobar1@meta": {
                "GET": {
                    "plugin": "runcmd",
                    "CMD": "/bin/sleep",
                    "CMDARGS": ["1"]
                }},
            "foobar2@meta": {
                "GET": {
                    "plugin": "runcmd",
                    "CMD": "/bin/sleep",
                    "CMDARGS": ["1"]
                }},
            "foobar3@meta": {
                "GET": {
                    "plugin": "runcmd",
                    "CMD": "/bin/sleep",
                    "CMDARGS": ["1"]
                }},
            "foobar4@meta": {
                "GET": {
                    "plugin": "runcmd",
                    "CMD": "/bin/sleep",
                    "CMDARGS": ["1"]
                }},

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
            "Name": "TEST",
            "Name@meta": {"PATCH": {"plugin": "patch"}}
        }
    }'

sleep 1
echo "/redfish/v1/test"
$CURLCMD $URL/redfish/v1/test

echo "Run a test action"
echo "/redfish/v1/Actions/Test"
$CURLCMD $URL/redfish/v1/Actions/Test -d '{"TestType": "FOO"}'

$CURLCMD $URL/redfish/v1/test -XPATCH -d '{"Name": "FOOBar"}'
$CURLCMD $URL/redfish/v1/SessionService -XPATCH -d '{"SessionTimeout": 35}'
