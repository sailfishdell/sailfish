#!/bin/bash

set -e

CURLCMD="curl --cacert ./ca.crt"
prot=${prot:-https}
user=${user:-Administrator}
pass=${pass:-password}
host=${host:-localhost}
port=${port:-8443}
URL=$prot://$user:$pass@$host:$port

headersfile=$(mktemp /tmp/headers-XXXXXX)
trap 'rm -f $headersfile' EXIT QUIT HUP INT ERR

RESPONSE_HEADERS=$($CURLCMD -D${headersfile} ${URL}/redfish/v1/SessionService/Sessions -X POST -d "{\"UserName\": \"${user}\", \"Password\": \"${pass}\"}" 2>&1)
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
