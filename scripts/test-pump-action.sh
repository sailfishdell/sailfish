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

$CURLCMD $URL/api/Event%3AInject  -d '
    {
        "name":"HTTPCmdProcessed",
        "data": {  "Results": "happy", "StatusCode": 200, "CommandID": "'$1'" }
    }'

