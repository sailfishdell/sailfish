#!/bin/bash

set -e

unset HTTP_PROXY HTTPS_PROXY http_proxy https_proxy
prot=${prot:-https}
user=${user:-${1:-Administrator}}
pass=${pass:-${2:-password}}
host=${host:-localhost}
port=${port:-8443}
URL=$prot://$host:$port

if [ "${host}" = "localhost" ]; then
    cacert=${cacert:-./ca.crt}
else
    cacert=${cacert:-./${host}-ca.crt}
fi

CURLCMD="curl --cacert ${cacert} ${CURL_OPTS} "
headersfile=$(mktemp /tmp/headers-XXXXXX)
trap 'rm -f $headersfile' EXIT QUIT HUP INT ERR

LOGIN_URI=${LOGIN_URI:-$($CURLCMD -s -H "Content-Type: application/json" -D${headersfile} ${URL}/redfish/v1 | jq -r '.Links.Sessions[]' )}

RESPONSE_HEADERS=$($CURLCMD -H "Content-Type: application/json" -D${headersfile} ${URL}${LOGIN_URI} -X POST -d "{\"UserName\": \"${user}\", \"Password\": \"${pass}\"}" 2>&1)
X_AUTH_TOKEN=$(cat ${headersfile} | grep -i x-auth-token | cut -d: -f2 | perl -p -e 's/\r//g;')
SESSION_URI=$(cat ${headersfile} | grep -i location | cut -d: -f2 | perl -p -e 's/\r//g;')

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
