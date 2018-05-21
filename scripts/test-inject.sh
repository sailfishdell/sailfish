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

echo "Test internal event-injection command API"
$CURLCMD $URL/api/Event%3AInject  -d '
    {
        "ID": "49467bb4-5c1f-473b-a000-000000000011",
        "name":"AttributeUpdated",
        "data": {  "FQDD": "CMC.Integrated.1", "Group": "another_group", "Index": "1", "Name": "bar", "Value": "'$RANDOM'" }
    }'

echo "Test internal event-injection command API"
$CURLCMD $URL/api/Event%3AInject  -d '
    {
        "ID": "49467bb4-5c1f-473b-a000-000000000011",
        "name":"AttributeUpdated",
        "data": {  "FQDD": "CMC.Integrated.1", "Group": "another_group", "Index": "1", "Name": "foo", "Value": "'$RANDOM'" },
        "event_array": [
            {  "FQDD": "CMC.Integrated.1", "Group": "another_group", "Index": "1", "Name": "foo_A", "Value": "'$RANDOM'" },
            {  "FQDD": "CMC.Integrated.1", "Group": "another_group", "Index": "1", "Name": "foo_B", "Value": "'$RANDOM'" },
            {  "FQDD": "CMC.Integrated.1", "Group": "another_group", "Index": "1", "Name": "foo_C", "Value": "'$RANDOM'" },
            {  "FQDD": "CMC.Integrated.1", "Group": "another_group", "Index": "1", "Name": "foo_D", "Value": "'$RANDOM'" }
        ]
    }'

